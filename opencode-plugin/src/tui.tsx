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
import type { AdoConfig, AdoProfile, PRSummary, WorkItemSummary, QaFeedbackSummary } from "./shared.js";
import {
  asAdoConfig,
  resolveActiveProfile,
  resolveOrgUrl,
  getPATOptional,
  shortBranch,
  reviewerMatchesUser,
} from "./shared.js";

// Persistence stores (ESM — import from .js)
import { getActiveProfile, setActiveProfile, getSelectedPr, setSelectedPr, clearSelectedPr, getSelectedWi, setSelectedWi, clearSelectedWi, getViewMode, setViewMode } from "./profile-store.js";

// ─── Constants ─────────────────────────────────────────────────────────────

const API_VERSION = "7.1";
const CONNECTION_DATA_API_VERSION = "7.1-preview.1";
const WIQL_API_VERSION = "7.1-preview.2";
const REQUEST_TIMEOUT_MS = 10_000;
const SIDEBAR_LOAD_TIMEOUT_MS = 15_000;
const POLL_INTERVAL_MS = 60_000;

// ─── Types ─────────────────────────────────────────────────────────────────

interface SidebarData {
  status: "loading" | "ready" | "error";
  profileName: string;
  profileCount: number;
  profileNames: string[];
  assignedToMe: PRSummary[];
  myPRs: PRSummary[];
  selectedPr: PRSummary | null;
  workItems: WorkItemSummary[];
  qaFeedbacks: QaFeedbackSummary[];
  selectedWi: WorkItemSummary | null;
  sidebarView: "prs" | "wis" | "qa";
  view: "list" | "detail";
  focusIndex: number;
  filters?: { state?: string; assignedTo?: string };
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
  filters?: { state?: string; assignedTo?: string },
): Promise<Omit<SidebarData, "selectedPr" | "selectedWi" | "sidebarView" | "view" | "filters" | "focusIndex">> {
  const config = await readConfig(client, options);
  const { name, profile, count, names } = resolveProfile(config);
  const pat = getPATOptional(profile.patEnvVar);

  if (!pat) {
    return {
      status: "error",
      profileName: name,
      profileCount: count,
      profileNames: names,
      assignedToMe: [],
      myPRs: [],
      workItems: [],
      qaFeedbacks: [],
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

  const assigned: PRSummary[] = [];
  const mine: PRSummary[] = [];
  const seenIds = new Set<string>();

  for (const repo of profile.repos) {
    try {
      const data = await doReq(
        `/git/repositories/${encodeURIComponent(repo)}/pullrequests?searchCriteria.status=active`,
      );
      for (const pr of data.value) {
        const key = `${repo}:${pr.pullRequestId}`;
        if (seenIds.has(key)) continue;
        seenIds.add(key);

        const summary: PRSummary = {
          id: pr.pullRequestId,
          title: pr.title,
          repo,
          source: shortBranch(pr.sourceRefName),
          target: shortBranch(pr.targetRefName),
          author: pr.createdBy?.displayName ?? "?",
          isDraft: !!pr.isDraft,
          myVote: undefined,
        };

        const myReview = pr.reviewers?.find((r: any) => reviewerMatchesUser(r, userId));
        if (myReview) {
          summary.myVote = myReview.vote ?? 0;
          assigned.push(summary);
        }
        if (pr.createdBy?.id === userId) {
          mine.push(summary);
        }
      }
    } catch { /* skip repo */ }
  }

  // ── Work Items ──
  const workItems: WorkItemSummary[] = [];
  const qaFeedbacks: QaFeedbackSummary[] = [];

  // Try fetching WIs (will fail gracefully for profiles without WI access)
  try {
    // Build WIQL query from filters or use default
    let wiqlQuery: string;
    if (filters) {
      const clauses: string[] = [];
      if (filters.assignedTo) {
        clauses.push(`[System.AssignedTo] = '${filters.assignedTo.replace(/'/g, "''")}'`);
      } else {
        clauses.push("[System.AssignedTo] = @Me");
      }
      if (filters.state) {
        clauses.push(`[System.State] = '${filters.state.replace(/'/g, "''")}'`);
      }
      wiqlQuery = `SELECT [System.Id] FROM WorkItems WHERE ${clauses.join(" AND ")} ORDER BY [System.ChangedDate] DESC`;
    } else {
      wiqlQuery = "SELECT [System.Id] FROM WorkItems WHERE [System.AssignedTo] = @Me AND [System.State] <> 'Closed' ORDER BY [System.ChangedDate] DESC";
    }
    const wiqlUrl = new URL(`${orgUrl}/${encodeURIComponent(profile.project)}/_apis/wit/wiql`);
    wiqlUrl.searchParams.set("api-version", WIQL_API_VERSION);
    const wiqlRes = await fetch(wiqlUrl.toString(), {
      method: "POST",
      headers: { Authorization: authHeader, "Content-Type": "application/json", Accept: "application/json" },
      body: JSON.stringify({ query: wiqlQuery }),
      signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS),
    });
    if (wiqlRes.ok) {
      const wiqlData = (await wiqlRes.json()) as { workItems?: Array<{ id: number }> };
      const wiIds = (wiqlData.workItems ?? []).map((wi) => wi.id);
      if (wiIds.length > 0) {
        const fields = "System.Id,System.Title,System.State,System.WorkItemType,System.AssignedTo,Microsoft.VSTS.Common.Priority,System.ChangedDate";
        const batchUrl = new URL(`${orgUrl}/_apis/wit/workitems`);
        batchUrl.searchParams.set("ids", wiIds.slice(0, 200).join(","));
        batchUrl.searchParams.set("fields", fields);
        batchUrl.searchParams.set("api-version", API_VERSION);
        const batchRes = await fetch(batchUrl.toString(), {
          headers: { Authorization: authHeader, Accept: "application/json" },
          signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS),
        });
        if (batchRes.ok) {
          const batchData = (await batchRes.json()) as { value: any[] };
          for (const wi of batchData.value ?? []) {
            const summary: WorkItemSummary = {
              id: wi.id,
              title: wi.fields?.["System.Title"] ?? "?",
              state: wi.fields?.["System.State"] ?? "?",
              type: wi.fields?.["System.WorkItemType"] ?? "?",
              assignedTo: wi.fields?.["System.AssignedTo"]?.displayName ?? "Unassigned",
              priority: wi.fields?.["Microsoft.VSTS.Common.Priority"] ?? 0,
              changedDate: wi.fields?.["System.ChangedDate"],
            };
            workItems.push(summary);
            // Check if this is a QA-type WI
            const typeLower = summary.type.toLowerCase();
            if (typeLower.includes("qa") || typeLower.includes("feedback") || typeLower.includes("test feedback")) {
              qaFeedbacks.push(summary);
            }
          }
        }
      }
    }
  } catch { /* skip WI fetching for profiles without access */ }

  return {
    status: "ready",
    profileName: name,
    profileCount: count,
    profileNames: names,
    assignedToMe: assigned,
    myPRs: mine,
    workItems,
    qaFeedbacks,
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
  return [...data.assignedToMe, ...data.myPRs];
}

/** Auto-select: try persisted, then first PR, or null if empty. */
function resolveSelection(
  fetched: Omit<SidebarData, "selectedPr" | "selectedWi" | "sidebarView" | "view" | "focusIndex">,
): PRSummary | null {
  const all = [...fetched.assignedToMe, ...fetched.myPRs];
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

/** Auto-select WI: try persisted, then first WI, or null if empty. */
function resolveWiSelection(
  fetched: Omit<SidebarData, "selectedPr" | "selectedWi" | "sidebarView" | "view" | "focusIndex">,
  profileName: string,
): WorkItemSummary | null {
  if (fetched.workItems.length === 0) return null;

  // 1. Try persisted selection
  const persisted = getSelectedWi();
  if (persisted && persisted.profileName === profileName) {
    const found = fetched.workItems.find((w) => w.id === persisted.wiId);
    if (found) return found;
  }

  // 2. Auto-select first WI
  return fetched.workItems[0];
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
        <text fg="gray">Loading...</text>
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

          {/* View indicator */}
          <text wrapMode="none" fg="gray">
            {(() => {
              let line = `View: ${d().sidebarView === "prs" ? "PRs" : d().sidebarView === "wis" ? "Work Items" : "QA Feedbacks"}`;
              if (d().filters) {
                const parts: string[] = [];
                if (d().filters!.state) parts.push(`state=${d().filters!.state}`);
                if (d().filters!.assignedTo) parts.push(`assignedTo=${d().filters!.assignedTo}`);
                line += ` | Filters: ${parts.join(", ")}`;
              }
              return `${line} | cmd/ctrl+P → ADO: Switch View`;
            })()}
          </text>

          {/* Keyboard hints for WI/QA navigation */}
          {(d().sidebarView === "wis" || d().sidebarView === "qa") && (
            <text fg="gray">cmd/ctrl+P → "Next Item" / "Previous Item" to move cursor</text>
          )}

          {/* ── PRs View ── */}
          {d().sidebarView === "prs" && (
            <>
              {/* Assigned to Me */}
              {d().assignedToMe.length > 0 && (
                <box gap={0}>
                  <text fg="yellow">{`Assigned to You (${String(d().assignedToMe.length)})`}</text>
                  {d().assignedToMe.map((pr) => {
                    const sel = d().selectedPr?.id === pr.id && d().selectedPr?.repo === pr.repo;
                    const voteIcon = pr.myVote === 10 ? "✓" : pr.myVote === -10 ? "✗" : pr.myVote === -5 ? "⏳" : pr.myVote === 5 ? "✓?" : "—";
                    const voteColor = pr.myVote === 10 ? "green" : pr.myVote === -10 ? "red" : pr.myVote === -5 ? "yellow" : "gray";
                    return (
                      <text wrapMode="none" fg={sel ? "cyan" : undefined}>
                        {sel ? "► " : "  "}
                        {`#${String(pr.id)} ${pr.repo}/${pr.source} → ${pr.target}`}
                        {" "}<span style={{ fg: voteColor }}>{voteIcon}</span>
                        {" "}<span style={{ fg: "gray" }}>{`${pr.author} — ${pr.title}`}</span>
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
              {d().assignedToMe.length === 0 && d().myPRs.length === 0 && (
                <text fg="gray">No active PRs</text>
              )}
            </>
          )}

          {/* ── Work Items View ── */}
          {d().sidebarView === "wis" && (
            <box gap={0}>
              <text fg="yellow">{`Work Items Assigned to You (${String(d().workItems.length)})`}</text>
              {d().workItems.length === 0 && <text fg="gray">No work items assigned</text>}
              {d().workItems.map((wi, idx) => {
                const sel = d().selectedWi?.id === wi.id;
                const focused = idx === d().focusIndex;
                const marker = sel ? "► " : focused ? "> " : "  ";
                const fg = sel ? "cyan" : focused ? "yellow" : undefined;
                return (
                  <text wrapMode="none" fg={fg}>
                    {marker}
                    {`#${String(wi.id)} [${wi.type}] ${wi.state} — ${wi.title}`}
                    {" "}<span style={{ fg: "gray" }}>{`P${String(wi.priority)} | ${wi.assignedTo}`}</span>
                  </text>
                );
              })}
            </box>
          )}

          {/* ── QA Feedbacks View ── */}
          {d().sidebarView === "qa" && (
            <box gap={0}>
              <text fg="magenta">{`QA Feedbacks (${String(d().qaFeedbacks.length)})`}</text>
              {d().qaFeedbacks.length === 0 && <text fg="gray">No QA Feedbacks</text>}
              {d().qaFeedbacks.map((fb, idx) => {
                const sel = d().selectedWi?.id === fb.id;
                const focused = idx === d().focusIndex;
                const marker = sel ? "► " : focused ? "> " : "  ";
                const fg = sel ? "cyan" : focused ? "yellow" : undefined;
                return (
                  <text wrapMode="none" fg={fg}>
                    {marker}
                    {`#${String(fb.id)} [${fb.type}] ${fb.state} — ${fb.title}`}
                    {" "}<span style={{ fg: "gray" }}>{`P${String(fb.priority)} | ${fb.assignedTo}`}</span>
                  </text>
                );
              })}
            </box>
          )}

          {/* ── Selected Detail (PR or WI/QA) ── */}
          {d().sidebarView === "prs" && d().selectedPr && (() => {
            const pr = d().selectedPr!;
            return (
              <box gap={0}>
                <text fg="cyan">{"── Selected PR ──"}</text>
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

          {(d().sidebarView === "wis" || d().sidebarView === "qa") && d().selectedWi && (() => {
            const wi = d().selectedWi!;
            return (
              <box gap={0}>
                <text fg="cyan">{"── Selected WI ──"}</text>
                <text wrapMode="none" fg={props.api.theme.current.text}>
                  {`#${String(wi.id)} ${wi.title}`}
                </text>
                <text wrapMode="none" fg="gray">
                  {`[${wi.type}] State: ${wi.state} | Priority: ${String(wi.priority)}`}
                </text>
                <text wrapMode="none" fg="gray">
                  {`Assigned to: ${wi.assignedTo}`}
                </text>
                {wi.changedDate && (
                  <text wrapMode="none" fg="gray">
                    {`Changed: ${wi.changedDate}`}
                  </text>
                )}
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
      title: "ADO: Refresh",
      value: "ado:refresh",
      description: "Refresh data from Azure DevOps",
      category: "Azure DevOps",
      onSelect: () => { void refresh(); },
    });

    // ── Switch View ──
    commands.push({
      title: "ADO: Switch View",
      value: "ado:switch-view",
      description: `Current: ${d.sidebarView === "prs" ? "PRs" : d.sidebarView === "wis" ? "Work Items" : "QA Feedbacks"}`,
      category: "Azure DevOps",
      suggested: true,
      onSelect: () => {
        api.ui.dialog.replace(() =>
          <api.ui.DialogSelect
            title="Switch ADO View"
            options={[
              { title: "Pull Requests", value: "prs", description: `${String(d.assignedToMe.length + d.myPRs.length)} PRs` },
              { title: "Work Items", value: "wis", description: `${String(d.workItems.length)} assigned` },
              { title: "QA Feedbacks", value: "qa", description: `${String(d.qaFeedbacks.length)} feedbacks` },
            ]}
            current={d.sidebarView}
            onSelect={(opt: { value: string }) => {
              setViewMode(opt.value);
              setData((prev) => ({ ...prev, sidebarView: opt.value as "prs" | "wis" | "qa", focusIndex: 0 }));
              api.ui.dialog.clear();
            }}
          />
        );
      },
    });

    // ── Filter Work Items (only when sidebarView is "wis" or "qa") ──
    if (d.status === "ready" && (d.sidebarView === "wis" || d.sidebarView === "qa")) {
      commands.push({
        title: "ADO: Filter Work Items",
        value: "ado:filter-workitems",
        description: "Filter work items by state or assignment",
        category: "Azure DevOps",
        suggested: true,
        onSelect: () => {
          const STATE_OPTIONS = [
            { title: "None (no filter)", value: "", description: "Show all states" },
            { title: "New", value: "New" },
            { title: "Active", value: "Active" },
            { title: "Resolved", value: "Resolved" },
            { title: "Closed", value: "Closed" },
            { title: "Removed", value: "Removed" },
            { title: "Committed", value: "Committed" },
            { title: "In Progress", value: "In Progress" },
          ];
          // Step 1: choose state filter
          const currentState = d.filters?.state ?? "";
          api.ui.dialog.replace(() =>
            <api.ui.DialogSelect
              title="Filter by State"
              placeholder="Choose state filter..."
              options={STATE_OPTIONS}
              current={currentState}
              onSelect={(stateOpt: { value: string }) => {
                const chosenState = stateOpt.value;
                // Step 2: optionally input assignedTo
                api.ui.dialog.replace(() =>
                  <api.ui.DialogPrompt
                    title="Filter by Assigned To (optional)"
                    placeholder="e.g., John Doe (leave empty for @Me)"
                    value={d.filters?.assignedTo ?? ""}
                    onConfirm={(assignedTo: string) => {
                      const trimmed = assignedTo.trim();
                      const newFilters: { state?: string; assignedTo?: string } = {};
                      if (chosenState) newFilters.state = chosenState;
                      if (trimmed) newFilters.assignedTo = trimmed;
                      const finalFilters = Object.keys(newFilters).length > 0 ? newFilters : undefined;
                      setData((prev) => ({ ...prev, filters: finalFilters }));
                      api.ui.dialog.clear();
                      void refresh();
                    }}
                    onCancel={() => {
                      api.ui.dialog.clear();
                    }}
                  />
                );
              }}
            />
          );
        },
      });
    }

    // ── Select PR (only when sidebarView === "prs") ──
    if (d.status === "ready" && d.sidebarView === "prs") {
      const prList = allPrs(d);
      if (prList.length > 0) {
        commands.push({
          title: "ADO: Select PR",
          value: "ado:select-pr",
          description: `${String(prList.length)} PRs available`,
          category: "Azure DevOps",
          suggested: true,
          onSelect: () => {
            const currentKey = d.selectedPr ? `${d.selectedPr.repo}:${d.selectedPr.id}` : undefined;
            const options = prList.map((pr) => {
              const key = `${pr.repo}:${pr.id}`;
              const isAssigned = d.assignedToMe.some((p) => p.id === pr.id && p.repo === pr.repo);
              const voteIcon = pr.myVote === 10 ? "✓" : pr.myVote === -10 ? "✗" : pr.myVote === -5 ? "⏳" : pr.myVote === 5 ? "✓?" : "—";
              return {
                title: `#${String(pr.id)} ${pr.title}`,
                value: key,
                description: `${pr.repo}: ${pr.source} → ${pr.target} by ${pr.author}${pr.isDraft ? " [DRAFT]" : ""}`,
                category: isAssigned ? "Assigned to You" : "Your PRs",
                footer: isAssigned ? `Your vote: ${voteIcon}` : undefined,
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

    // ── Select Work Item (only when sidebarView === "wis") ──
    if (d.status === "ready" && d.sidebarView === "wis" && d.workItems.length > 0) {
      commands.push({
        title: "ADO: Select Work Item",
        value: "ado:select-wi",
        description: `${String(d.workItems.length)} work items`,
        category: "Azure DevOps",
        suggested: true,
        onSelect: () => {
          const currentKey = d.selectedWi ? `${d.selectedWi.id}` : undefined;
          const options = d.workItems.map((wi) => ({
            title: `#${String(wi.id)} ${wi.title}`,
            value: String(wi.id),
            description: `[${wi.type}] ${wi.state} — P${String(wi.priority)}`,
            category: wi.state,
          }));
          api.ui.dialog.replace(() =>
            <api.ui.DialogSelect
              title="Select Work Item"
              placeholder="Search work items..."
              options={options}
              current={currentKey}
              onSelect={(opt: { value: string }) => {
                const wiId = parseInt(opt.value, 10);
                const found = d.workItems.find((w) => w.id === wiId);
                if (found) {
                  setSelectedWi(d.profileName, found.id);
                  setData((prev) => ({ ...prev, selectedWi: found }));
                }
                api.ui.dialog.clear();
              }}
            />
          );
        },
      });
    }

    // ── Select QA Feedback (only when sidebarView === "qa") ──
    if (d.status === "ready" && d.sidebarView === "qa" && d.qaFeedbacks.length > 0) {
      commands.push({
        title: "ADO: Select QA Feedback",
        value: "ado:select-qa",
        description: `${String(d.qaFeedbacks.length)} feedbacks`,
        category: "Azure DevOps",
        suggested: true,
        onSelect: () => {
          const currentKey = d.selectedWi ? `${d.selectedWi.id}` : undefined;
          const options = d.qaFeedbacks.map((fb) => ({
            title: `#${String(fb.id)} ${fb.title}`,
            value: String(fb.id),
            description: `[${fb.type}] ${fb.state} — P${String(fb.priority)}`,
            category: fb.state,
          }));
          api.ui.dialog.replace(() =>
            <api.ui.DialogSelect
              title="Select QA Feedback"
              placeholder="Search QA feedbacks..."
              options={options}
              current={currentKey}
              onSelect={(opt: { value: string }) => {
                const fbId = parseInt(opt.value, 10);
                const found = d.qaFeedbacks.find((f) => f.id === fbId);
                if (found) {
                  setSelectedWi(d.profileName, found.id);
                  setData((prev) => ({ ...prev, selectedWi: found }));
                }
                api.ui.dialog.clear();
              }}
            />
          );
        },
      });
    }

    // ── Keyboard Navigation (WI/QA views) ──
    commands.push({
      title: "Next Item",
      value: "ado:nav-down",
      category: "ADO",
      enabled: d.sidebarView === "wis" || d.sidebarView === "qa",
      onSelect: () => {
        setData((prev) => {
          const items = prev.sidebarView === "wis" ? prev.workItems : prev.qaFeedbacks;
          const count = items.length;
          if (count === 0) return prev;
          return { ...prev, focusIndex: prev.focusIndex + 1 >= count ? 0 : prev.focusIndex + 1 };
        });
      },
    });

    commands.push({
      title: "Previous Item",
      value: "ado:nav-up",
      category: "ADO",
      enabled: d.sidebarView === "wis" || d.sidebarView === "qa",
      onSelect: () => {
        setData((prev) => {
          const items = prev.sidebarView === "wis" ? prev.workItems : prev.qaFeedbacks;
          const count = items.length;
          if (count === 0) return prev;
          return { ...prev, focusIndex: prev.focusIndex - 1 < 0 ? count - 1 : prev.focusIndex - 1 };
        });
      },
    });

    commands.push({
      title: "Select Highlighted Item",
      value: "ado:nav-select",
      category: "ADO",
      enabled: d.sidebarView === "wis" || d.sidebarView === "qa",
      onSelect: () => {
        const current = data();
        const items = current.sidebarView === "wis" ? current.workItems : current.qaFeedbacks;
        if (items.length === 0) return;
        const idx = current.focusIndex;
        if (idx >= 0 && idx < items.length) {
          const focused = items[idx];
          setSelectedWi(current.profileName, focused.id);
          setData((prev) => ({ ...prev, selectedWi: focused }));
        }
      },
    });

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
    assignedToMe: [],
    myPRs: [],
    selectedPr: null,
    workItems: [],
    qaFeedbacks: [],
    selectedWi: null,
    sidebarView: getViewMode(),
    view: "list",
    focusIndex: 0,
    filters: undefined,
  });

  let disposed = false;
  let inFlight: Promise<void> | undefined;

  // ── Refresh with selection preservation ──

  const refresh = async () => {
    if (disposed || inFlight) return inFlight;

    inFlight = (async () => {
      try {
        const currentFilters = data().filters;
        const fetched = await withTimeout(
          fetchPRData(api.client, options, currentFilters),
          SIDEBAR_LOAD_TIMEOUT_MS,
          "Sidebar load timed out",
        );
        if (disposed) return;

        // Preserve or resolve selections
        const prSelected = resolveSelection(fetched);
        if (prSelected) setSelectedPr(prSelected.repo, prSelected.id);
        else clearSelectedPr();

        const wiSelected = resolveWiSelection(fetched, fetched.profileName);
        if (wiSelected) setSelectedWi(fetched.profileName, wiSelected.id);
        else clearSelectedWi();

        // Persist sidebar view
        const persistedView = getViewMode();

        // Cap focusIndex to new item count
        const currentFocus = data().focusIndex;
        const itemCount = persistedView === "wis"
          ? fetched.workItems.length
          : persistedView === "qa"
            ? fetched.qaFeedbacks.length
            : 0;
        const focusIndex = itemCount > 0 ? Math.min(currentFocus, itemCount - 1) : 0;

        setData({
          ...fetched,
          selectedPr: prSelected,
          selectedWi: wiSelected,
          sidebarView: persistedView,
          view: "list",
          focusIndex,
          filters: currentFilters,
        });
      } catch (err) {
        if (!disposed) {
          // If filters were active, keep previous data to avoid blanking the sidebar
          const prev = data();
          if (prev.status === "ready" && prev.filters) {
            setData((prevData) => ({ ...prevData, status: "ready" }));
          } else {
            setData({
              status: "error",
              profileName: "",
              profileCount: 0,
              profileNames: [],
              assignedToMe: [],
              myPRs: [],
              selectedPr: null,
              workItems: [],
              qaFeedbacks: [],
              selectedWi: null,
              sidebarView: getViewMode(),
              view: "list",
              focusIndex: 0,
              filters: prev.filters,
              error: err instanceof Error ? err.message : String(err),
            });
          }
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
