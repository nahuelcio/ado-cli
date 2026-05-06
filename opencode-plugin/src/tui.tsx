/** @jsxImportSource @opentui/solid */
/**
 * OpenCode TUI Plugin — Azure DevOps interactive sidebar panel.
 *
 * Renders an interactive sidebar with:
 *   - Active profile name with profile count
 *   - Navigable PR list (► marker for selected PR, persisted to disk)
 *   - Selected PR detail section
 *   - Selection survives refreshes (~/.azure-devops-cli/selected-pr)
 *   - Profile switching via persisted profile-store
 *   - Progressive keyboard enhancement (up/down/r/p if api.onKey available)
 *
 * Uses SolidJS signals for reactive updates. Polls every 60 seconds
 * and refreshes on session/message update events.
 */

import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from "@opencode-ai/plugin/tui";
import { createSignal, Match, Switch } from "solid-js";
import { existsSync } from "node:fs";
import { join } from "node:path";

// Shared types and helpers (ESM — import from .js)
import type { AdoConfig, AdoProfile, PRSummary } from "./shared.js";
import {
  asAdoConfig,
  resolveActiveProfile,
  resolveOrgUrl,
  getPATOptional,
  shortBranch,
  reviewerMatchesUser,
} from "./shared.js";

// Persistence stores (ESM — import from .js)
import { getActiveProfile, setActiveProfile, getSelectedPr, setSelectedPr, clearSelectedPr } from "./profile-store.js";

// ─── Constants ─────────────────────────────────────────────────────────────

const API_VERSION = "7.1";
const CONNECTION_DATA_API_VERSION = "7.1-preview.1";
const REQUEST_TIMEOUT_MS = 10_000;
const SIDEBAR_LOAD_TIMEOUT_MS = 15_000;
const POLL_INTERVAL_MS = 60_000;

// ─── Types ─────────────────────────────────────────────────────────────────

interface SidebarData {
  status: "loading" | "ready" | "error";
  profileName: string;
  profileCount: number;
  profileNames: string[];
  pendingReviews: PRSummary[];
  myPRs: PRSummary[];
  selectedPr: PRSummary | null;
  view: "list" | "detail";
  error?: string;
}

// ─── Config reading ────────────────────────────────────────────────────────

async function readConfig(client: TuiPluginApi["client"], options?: unknown): Promise<AdoConfig> {
  const fromOptions = asAdoConfig(options);
  if (fromOptions) return fromOptions;

  const resp = await client.config.get().catch(() => ({ data: {} }));
  const data = (resp.data ?? {}) as Record<string, unknown>;
  const ado = asAdoConfig(data["ado"]);
  if (!ado?.profiles) throw new Error("No ADO config in opencode.json");
  return ado;
}

/** Resolve profile: persisted > defaultProfile > default:true > first. */
function resolveProfile(config: AdoConfig): { name: string; profile: AdoProfile; count: number; names: string[] } {
  const names = Object.keys(config.profiles);
  const count = names.length;

  // Try persisted profile first
  const persisted = getActiveProfile();
  if (persisted && config.profiles[persisted]) {
    return { name: persisted, profile: config.profiles[persisted], count, names };
  }

  // Fallback to config defaults
  const resolved = resolveActiveProfile(config);
  return { name: resolved.name, profile: resolved.profile, count, names };
}

// ─── Data fetching ─────────────────────────────────────────────────────────

async function fetchPRData(
  client: TuiPluginApi["client"],
  options?: unknown,
): Promise<Omit<SidebarData, "selectedPr" | "view">> {
  const config = await readConfig(client, options);
  const { name, profile, count, names } = resolveProfile(config);
  const pat = getPATOptional(profile.patEnvVar);

  if (!pat) {
    return {
      status: "error",
      profileName: name,
      profileCount: count,
      profileNames: names,
      pendingReviews: [],
      myPRs: [],
      error: `Set env var ${profile.patEnvVar} or run init`,
    };
  }

  const orgUrl = resolveOrgUrl(profile.org);
  const authHeader = "Basic " + Buffer.from(":" + pat).toString("base64");

  const doReq = async (
    endpoint: string,
    scope: "org" | "project" = "project",
    apiVersion = API_VERSION,
  ) => {
    const root = scope === "org" ? orgUrl : `${orgUrl}/${encodeURIComponent(profile.project)}`;
    const apiPath = endpoint.startsWith("/_apis/")
      ? endpoint
      : `/_apis${endpoint.startsWith("/") ? endpoint : `/${endpoint}`}`;
    const url = new URL(root + apiPath);
    url.searchParams.set("api-version", apiVersion);
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS);
    const res = await fetch(url, {
      headers: { Authorization: authHeader, Accept: "application/json" },
      signal: controller.signal,
    }).finally(() => clearTimeout(timeout));
    if (!res.ok) {
      const body = await res.text().catch(() => "");
      throw new Error(`ADO ${res.status}: ${body.slice(0, 120)}`);
    }
    return res.json() as Promise<{ value: any[] }>;
  };

  // Get user identity
  const connData = await doReq("/connectionData", "org", CONNECTION_DATA_API_VERSION) as any;
  const userId: string | undefined = connData?.authenticatedUser?.id;

  const pending: PRSummary[] = [];
  const mine: PRSummary[] = [];

  for (const repo of profile.repos) {
    try {
      const data = await doReq(
        `/git/repositories/${encodeURIComponent(repo)}/pullrequests?searchCriteria.status=active`,
      );
      for (const pr of data.value) {
        const summary: PRSummary = {
          id: pr.pullRequestId,
          title: pr.title,
          repo,
          source: shortBranch(pr.sourceRefName),
          target: shortBranch(pr.targetRefName),
          author: pr.createdBy?.displayName ?? "?",
          isDraft: !!pr.isDraft,
        };
        const myReview = pr.reviewers?.find((r: any) => reviewerMatchesUser(r, userId));
        if (myReview && myReview.vote === 0) pending.push(summary);
        if (pr.createdBy?.id === userId) mine.push(summary);
      }
    } catch { /* skip repo */ }
  }

  return {
    status: "ready",
    profileName: name,
    profileCount: count,
    profileNames: names,
    pendingReviews: pending,
    myPRs: mine,
  };
}

async function withTimeout<T>(promise: Promise<T>, timeoutMs: number, message: string): Promise<T> {
  return await Promise.race([
    promise,
    new Promise<T>((_, reject) => {
      const timer = setTimeout(() => {
        clearTimeout(timer);
        reject(new Error(message));
      }, timeoutMs);
    }),
  ]);
}

// ─── Selection helpers ─────────────────────────────────────────────────────

/** Get all PRs in display order (pending first, then mine). */
function allPrs(data: SidebarData): PRSummary[] {
  return [...data.pendingReviews, ...data.myPRs];
}

/** Auto-select: try persisted, then first PR, or null if empty. */
function resolveSelection(
  fetched: Omit<SidebarData, "selectedPr" | "view">,
): PRSummary | null {
  const all = [...fetched.pendingReviews, ...fetched.myPRs];
  if (all.length === 0) return null;

  // 1. Try persisted selection
  const persisted = getSelectedPr();
  if (persisted) {
    const found = all.find((p) => p.repo === persisted.repo && p.id === persisted.prId);
    if (found) return found;
  }

  // 2. Auto-select first PR
  return all[0];
}

// ─── Sidebar Content View ──────────────────────────────────────────────────

function SidebarContentView(props: {
  api: TuiPluginApi;
  data: () => SidebarData;
}) {
  const d = props.data;

  return (
    <Switch>
      <Match when={d().status === "loading"}>
        <text fg="gray">Loading PRs...</text>
      </Match>
      <Match when={d().status === "error"}>
        <box gap={0}>
          <text fg="red">ADO: {d().error ?? "Unknown error"}</text>
          <text fg="gray">{"cmd/ctrl+P → ADO: Refresh"}</text>
        </box>
      </Match>
      <Match when={d().status === "ready"}>
        <box gap={0}>
          {/* Header */}
          <text wrapMode="none" fg={props.api.theme.current.text}>
            <b>{`Azure DevOps (${d().profileName})`}</b>
            {d().profileCount > 1
              ? <span style={{ fg: "gray" }}>{` [${String(d().profileCount)} profiles]`}</span>
              : ""}
          </text>

          {/* Pending Reviews */}
          {d().pendingReviews.length > 0 && (
            <box gap={0}>
              <text fg="yellow">{`Pending Review (${String(d().pendingReviews.length)})`}</text>
              {d().pendingReviews.map((pr) => {
                const sel = d().selectedPr?.id === pr.id && d().selectedPr?.repo === pr.repo;
                return (
                  <text wrapMode="none" fg={sel ? "cyan" : undefined}>
                    {sel ? "► " : "  "}
                    {`#${String(pr.id)} ${pr.repo}/${pr.source} → ${pr.target}`}
                    {"  "}<span style={{ fg: "gray" }}>{`${pr.author} — ${pr.title}`}</span>
                  </text>
                );
              })}
            </box>
          )}

          {/* My PRs */}
          {d().myPRs.length > 0 && (
            <box gap={0}>
              <text fg="green">{`Your PRs (${String(d().myPRs.length)})`}</text>
              {d().myPRs.map((pr) => {
                const sel = d().selectedPr?.id === pr.id && d().selectedPr?.repo === pr.repo;
                return (
                  <text wrapMode="none" fg={sel ? "cyan" : undefined}>
                    {sel ? "► " : "  "}
                    {`#${String(pr.id)} ${pr.repo}/${pr.source} → ${pr.target}`}
                    {pr.isDraft ? <span style={{ fg: "gray" }}>{" [DRAFT]"}</span> : ""}
                    {"  "}<span style={{ fg: "gray" }}>{pr.title}</span>
                  </text>
                );
              })}
            </box>
          )}

          {/* Empty state */}
          {d().pendingReviews.length === 0 && d().myPRs.length === 0 && (
            <text fg="gray">No active PRs</text>
          )}

          {/* Selected PR Detail */}
          {d().selectedPr && (() => {
            const pr = d().selectedPr!;
            return (
              <box gap={0}>
                <text fg="cyan">{"── Selected ──"}</text>
                <text wrapMode="none" fg={props.api.theme.current.text}>
                  {`#${String(pr.id)} ${pr.title}`}
                </text>
                <text wrapMode="none" fg="gray">
                  {`${pr.repo}: ${pr.source} → ${pr.target}`}
                </text>
                <text wrapMode="none" fg="gray">
                  {`by ${pr.author}${pr.isDraft ? " [DRAFT]" : ""}`}
                </text>
              </box>
            );
          })()}

          {/* Footer hints */}
          <text fg="gray">
            {"cmd/ctrl+P → ADO commands"}
          </text>
        </box>
      </Match>
    </Switch>
  );
}

// ─── Commands & Dialogs (native OpenCode interaction) ──────────────────────

/**
 * Register commands that appear in the OpenCode command palette.
 * Uses api.ui.DialogSelect for interactive keyboard-navigable selection.
 */
function registerCommands(
  api: TuiPluginApi,
  data: () => SidebarData,
  setData: (fn: (prev: SidebarData) => SidebarData) => void,
  refresh: () => Promise<void>,
): () => void {
  const unsub = api.command.register(() => {
    const d = data();
    const commands: Array<{
      title: string;
      value: string;
      description?: string;
      category?: string;
      keybind?: string;
      suggested?: boolean;
      hidden?: boolean;
      enabled?: boolean;
      onSelect?: () => void;
    }> = [];

    // ── Refresh ──
    commands.push({
      title: "ADO: Refresh PRs",
      value: "ado:refresh",
      description: "Refresh the PR list from Azure DevOps",
      category: "Azure DevOps",
      onSelect: () => { void refresh(); },
    });

    // ── Select PR ──
    if (d.status === "ready") {
      const prList = allPrs(d);
      if (prList.length > 0) {
        commands.push({
          title: "ADO: Select PR",
          value: "ado:select-pr",
          description: `${prList.length} PRs available`,
          category: "Azure DevOps",
          suggested: true,
          onSelect: () => {
            const currentKey = d.selectedPr ? `${d.selectedPr.repo}:${d.selectedPr.id}` : undefined;
            const options = prList.map((pr) => {
              const key = `${pr.repo}:${pr.id}`;
              const isPending = d.pendingReviews.some((p) => p.id === pr.id && p.repo === pr.repo);
              return {
                title: `#${pr.id} ${pr.title}`,
                value: key,
                description: `${pr.repo}: ${pr.source} → ${pr.target} by ${pr.author}${pr.isDraft ? " [DRAFT]" : ""}`,
                category: isPending ? "Pending Review" : "Your PRs",
              };
            });
            api.ui.dialog.replace(() =>
              <api.ui.DialogSelect
                title="Select a PR"
                placeholder="Search PRs..."
                options={options}
                current={currentKey}
                onSelect={(option: { value: string }) => {
                  const [repo, idStr] = option.value.split(":");
                  const prId = parseInt(idStr, 10);
                  const found = prList.find((p) => p.repo === repo && p.id === prId);
                  if (found) {
                    setSelectedPr(found.repo, found.id);
                    setData((prev) => ({ ...prev, selectedPr: found }));
                  }
                  api.ui.dialog.clear();
                }}
              />
            );
          },
        });
      }
    }

    // ── Switch Profile ──
    if (d.profileCount > 1) {
      commands.push({
        title: "ADO: Switch Profile",
        value: "ado:switch-profile",
        description: `Current: ${d.profileName}`,
        category: "Azure DevOps",
        suggested: true,
        onSelect: () => {
          const options = d.profileNames.map((name) => ({
            title: name,
            value: name,
            description: name === d.profileName ? "← active" : undefined,
          }));
          api.ui.dialog.replace(() =>
            <api.ui.DialogSelect
              title="Switch Profile"
              options={options}
              current={d.profileName}
              onSelect={(option: { value: string }) => {
                setActiveProfile(option.value);
                api.ui.dialog.clear();
                void refresh();
              }}
            />
          );
        },
      });
    }

    return commands;
  });

  return unsub;
}

// ─── TUI Plugin Export ─────────────────────────────────────────────────────

export const tui: TuiPlugin = async (api: TuiPluginApi, options) => {
  const [data, setData] = createSignal<SidebarData>({
    status: "loading",
    profileName: "",
    profileCount: 0,
    profileNames: [],
    pendingReviews: [],
    myPRs: [],
    selectedPr: null,
    view: "list",
  });

  let disposed = false;
  let inFlight: Promise<void> | undefined;

  // ── Refresh with selection preservation ──

  const refresh = async () => {
    if (disposed || inFlight) return inFlight;

    inFlight = (async () => {
      try {
        const fetched = await withTimeout(
          fetchPRData(api.client, options),
          SIDEBAR_LOAD_TIMEOUT_MS,
          "Sidebar load timed out",
        );
        if (disposed) return;

        // Preserve or resolve selection
        const selected = resolveSelection(fetched);
        if (selected) setSelectedPr(selected.repo, selected.id);
        else clearSelectedPr();

        setData({
          ...fetched,
          selectedPr: selected,
          view: "list",
        });
      } catch (err) {
        if (!disposed) {
          setData({
            status: "error",
            profileName: "",
            profileCount: 0,
            profileNames: [],
            pendingReviews: [],
            myPRs: [],
            selectedPr: null,
            view: "list",
            error: err instanceof Error ? err.message : String(err),
          });
        }
      } finally {
        inFlight = undefined;
      }
    })();

    return inFlight;
  };

  // ── Initial load ──
  void refresh();

  // ── Polling ──
  const interval = setInterval(() => {
    void refresh();
  }, POLL_INTERVAL_MS);

  // ── Event-driven refresh ──
  const unsubEvent = api.event.on("session.updated", () => {
    setTimeout(() => { void refresh(); }, 150);
  });
  const unsubMsg = api.event.on("message.updated", () => {
    setTimeout(() => { void refresh(); }, 150);
  });

  // ── Commands (command palette + DialogSelect) ──
  const unsubCommands = registerCommands(api, data, setData, refresh);

  // ── Lifecycle ──
  api.lifecycle.onDispose(() => {
    disposed = true;
    clearInterval(interval);
    unsubEvent();
    unsubMsg();
    unsubCommands();
  });

  // ── Register sidebar slot ──
  api.slots.register({
    order: 200,
    slots: {
      sidebar_content() {
        return <SidebarContentView api={api} data={data} />;
      },
    },
  });
};

// ─── Module export ─────────────────────────────────────────────────────────

const pluginModule: TuiPluginModule & { id: string } = {
  id: "@nahuelcio/opencode-ado",
  tui,
};

export default pluginModule;
