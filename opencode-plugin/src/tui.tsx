/** @jsxImportSource @opentui/solid */
/**
 * OpenCode TUI Plugin — Azure DevOps sidebar panel.
 *
 * Renders a sidebar section showing:
 *   - Active profile name
 *   - Pending PR reviews (where you haven't voted)
 *   - Your active PRs
 *
 * Uses SolidJS signals for reactive updates. Polls every 60 seconds
 * and refreshes on session/message update events.
 */

import type { TuiPlugin, TuiPluginApi } from "@opencode-ai/plugin/tui";
import { createSignal, onCleanup } from "solid-js";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { homedir } from "node:os";

// ─── Types ────────────────────────────────────────────────────────────────

interface AdoProfile {
  org: string;
  project: string;
  patEnvVar: string;
  repos: string[];
  default?: boolean;
}

interface AdoConfig {
  defaultProfile?: string;
  profiles: Record<string, AdoProfile>;
}

interface SidebarData {
  status: "loading" | "ready" | "error";
  profileName: string;
  pendingReviews: PRSummary[];
  myPRs: PRSummary[];
  error?: string;
}

interface PRSummary {
  id: number;
  title: string;
  repo: string;
  source: string;
  target: string;
  author: string;
  isDraft: boolean;
}

// ─── Config & Client (lightweight, self-contained) ────────────────────────

function resolveActiveProfile(config: AdoConfig): { name: string; profile: AdoProfile } {
  if (config.defaultProfile && config.profiles[config.defaultProfile])
    return { name: config.defaultProfile, profile: config.profiles[config.defaultProfile] };
  for (const [name, p] of Object.entries(config.profiles))
    if (p.default) return { name, profile: p };
  const entries = Object.entries(config.profiles);
  if (!entries.length) throw new Error("No ADO profiles configured");
  return { name: entries[0][0], profile: entries[0][1] };
}

function resolveOrgUrl(org: string): string {
  if (org.startsWith("http")) return org.replace(/\/$/, "");
  return `https://dev.azure.com/${org}`;
}

function getPAT(envVarName: string): string | undefined {
  const fromEnv = process.env[envVarName];
  if (fromEnv) return fromEnv;
  try {
    const patPath = join(homedir(), ".azure-devops-cli", "pat");
    if (existsSync(patPath)) return readFileSync(patPath, "utf-8").trim() || undefined;
  } catch { /* ignore */ }
  return undefined;
}

async function readConfig(client: TuiPluginApi["client"]): Promise<AdoConfig> {
  const resp = await client.config.get().catch(() => ({ data: {} }));
  const data = (resp.data ?? {}) as Record<string, unknown>;
  const ado = data["ado"] as AdoConfig | undefined;
  if (!ado?.profiles) throw new Error("No ADO config in opencode.json");
  return ado;
}

async function fetchSidebarData(client: TuiPluginApi["client"]): Promise<SidebarData> {
  const config = await readConfig(client);
  const { name, profile } = resolveActiveProfile(config);
  const pat = getPAT(profile.patEnvVar);
  if (!pat) return { status: "error", profileName: name, pendingReviews: [], myPRs: [], error: `Set env var ${profile.patEnvVar}` };

  const orgUrl = resolveOrgUrl(profile.org);
  const authHeader = "Basic " + Buffer.from(":" + pat).toString("base64");

  const doReq = async (endpoint: string) => {
    const url = `${orgUrl}/${profile.project}/_apis${endpoint}?api-version=7.1`;
    const res = await fetch(url, { headers: { Authorization: authHeader, Accept: "application/json" } });
    if (!res.ok) throw new Error(`ADO ${res.status}`);
    return res.json() as Promise<{ value: any[] }>;
  };

  // Get user identity
  const connData = await doReq("/_apis/connectionData") as any;
  const userId: string | undefined = connData?.authenticatedUser?.id;

  const pending: PRSummary[] = [];
  const mine: PRSummary[] = [];

  for (const repo of profile.repos) {
    try {
      const data = await doReq(`/git/repositories/${encodeURIComponent(repo)}/pullrequests&searchCriteria.status=active`);
      for (const pr of data.value) {
        const summary: PRSummary = {
          id: pr.pullRequestId,
          title: pr.title,
          repo,
          source: (pr.sourceRefName ?? "").replace("refs/heads/", ""),
          target: (pr.targetRefName ?? "").replace("refs/heads/", ""),
          author: pr.createdBy?.displayName ?? "?",
          isDraft: !!pr.isDraft,
        };
        const myReview = pr.reviewers?.find((r: any) => r.votedBy?.id === userId);
        if (myReview && myReview.vote === 0) pending.push(summary);
        if (pr.createdBy?.id === userId) mine.push(summary);
      }
    } catch { /* skip */ }
  }

  return { status: "ready", profileName: name, pendingReviews: pending, myPRs: mine };
}

// ─── Helper ───────────────────────────────────────────────────────────────

function shortRef(ref: string | undefined): string {
  if (!ref) return "?";
  return ref.replace("refs/heads/", "").replace("refs/tags/", "");
}

// ─── TUI Plugin Export ────────────────────────────────────────────────────

export const tui: TuiPlugin = async (api: TuiPluginApi) => {
  const ID = "@nahuelcio/opencode-ado";

  api.slots.register({
    order: 200,
    slots: {
      sidebar_content(ctx) {
        // ─── Reactive State ──────────────────────────────────────
        const [data, setData] = createSignal<SidebarData>({
          status: "loading",
          profileName: "",
          pendingReviews: [],
          myPRs: [],
        });

        let disposed = false;
        const refresh = async () => {
          if (disposed) return;
          try {
            const result = await fetchSidebarData(api.client);
            if (!disposed) setData(result);
          } catch (err) {
            if (!disposed) setData({
              status: "error",
              profileName: "",
              pendingReviews: [],
              myPRs: [],
              error: err instanceof Error ? err.message : String(err),
            });
          }
        };

        // Initial load
        refresh();

        // Polling every 60s
        const interval = setInterval(refresh, 60_000);
        const unsubEvent = api.event.on("session.updated", () => {
          setTimeout(refresh, 150);
        });
        const unsubMsg = api.event.on("message.updated", () => {
          setTimeout(refresh, 150);
        });

        onCleanup(() => {
          disposed = true;
          clearInterval(interval);
          unsubEvent();
          unsubMsg();
        });

        // ─── Render ──────────────────────────────────────────────
        const d = data;
        if (d().status === "loading") return <text fg="gray">Loading PRs...</text>;
        if (d().status === "error") return <text fg="red">ADO: {d().error ?? "Unknown error"}</text>;

        const pending = d().pendingReviews;
        const mine = d().myPRs;

        return (
          <box gap={0}>
            <text fg={api.theme.current.text}>
              <b>Azure DevOps ({d().profileName})</b>
            </text>
            {pending.length > 0 && (
              <box gap={0}>
                <text fg="yellow">Pending Review ({pending.length})</text>
                {pending.map((pr) => (
                  <text wrapMode="none">
                    {"  "}#{pr.id} {pr.repo}/{shortRef(pr.source)} → {shortRef(pr.target)}{"  "}
                    <text fg="gray">{pr.author} — {pr.title}</text>
                  </text>
                ))}
              </box>
            )}
            {mine.length > 0 && (
              <box gap={0}>
                <text fg="green">Your PRs ({mine.length})</text>
                {mine.map((pr) => (
                  <text wrapMode="none">
                    {"  "}#{pr.id} {pr.repo}/{shortRef(pr.source)} → {shortRef(pr.target)}
                    {pr.isDraft && <text fg="gray"> [DRAFT]</text>}
                    {"  "}<text fg="gray">{pr.title}</text>
                  </text>
                ))}
              </box>
            )}
            {pending.length === 0 && mine.length === 0 && (
              <text fg="gray">No active PRs</text>
            )}
          </box>
        );
      },
    },
  });
};
