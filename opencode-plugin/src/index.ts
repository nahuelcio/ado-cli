/**
 * OpenCode ADO Plugin — V1 Module Entry Point.
 *
 * This package registers TWO plugin surfaces:
 *   - Server (index.ts) — tools for PR workflow
 *   - TUI (tui.tsx) — sidebar panel
 *
 * OpenCode loads each surface via the exports map in package.json:
 *   - `"."` → this file (server module)
 *   - `"./tui"` → tui.tsx (tui module)
 *
 * Config in opencode.json:
 * ```jsonc
 * {
 *   "ado": {
 *     "defaultProfile": "work",
 *     "profiles": {
 *       "work": {
 *         "org": "myorg",
 *         "project": "myproject",
 *         "patEnvVar": "ADO_PAT",
 *         "repos": ["backend", "frontend"]
 *       }
 *     }
 *   }
 * }
 * ```
 */

import type { Plugin, PluginInput, Hooks, PluginModule, PluginOptions } from "@opencode-ai/plugin";
import { z } from "zod/v4";
import type { AdoConfig, AdoProfile } from "./shared.js";
import {
  asAdoConfig,
  resolveActiveProfile,
  resolveOrgUrl,
  getPAT,
  shortBranch,
  reviewerMatchesUser,
  fmtPR,
  fmtPRDetail,
  fmtThread,
  fmtWorkItem,
  fmtWorkItemDetail,
  fmtQaFeedback,
  fmtQaFeedbackDetail,
} from "./shared.js";
import { getActiveProfile, setActiveProfile, getSelectedPr, setSelectedPr, clearSelectedPr, getSelectedWi, setSelectedWi, clearSelectedWi } from "./profile-store.js";

// ─── ADO HTTP Client (minimal, inline) ────────────────────────────────────

const API_VERSION = "7.1";

class AdoClient {
  private authHeader: string;

  constructor(
    private orgUrl: string,
    private project: string,
    pat: string,
  ) {
    this.authHeader = "Basic " + Buffer.from(":" + pat).toString("base64");
  }

  private buildUrl(
    endpoint: string,
    scope: "org" | "project" = "project",
    apiVersion: string = API_VERSION,
  ): string {
    if (endpoint.startsWith("http://") || endpoint.startsWith("https://")) {
      const url = new URL(endpoint);
      url.searchParams.set("api-version", apiVersion);
      return url.toString();
    }

    const root = scope === "org"
      ? this.orgUrl
      : `${this.orgUrl}/${encodeURIComponent(this.project)}`;
    const apiPath = endpoint.startsWith("/_apis/")
      ? endpoint
      : `/_apis${endpoint.startsWith("/") ? endpoint : `/${endpoint}`}`;
    const url = new URL(root + apiPath);
    url.searchParams.set("api-version", apiVersion);
    return url.toString();
  }

  private async request<T>(
    endpoint: string,
    init?: RequestInit,
    scope: "org" | "project" = "project",
    apiVersion?: string,
  ): Promise<T> {
    const url = this.buildUrl(endpoint, scope, apiVersion);
    const res = await fetch(url, {
      ...init,
      headers: {
        Authorization: this.authHeader,
        Accept: "application/json",
        "Content-Type": "application/json",
        ...init?.headers,
      },
    });
    if (!res.ok) {
      const body = await res.text().catch(() => "");
      const truncated = body.length > 200 ? `${body.slice(0, 200)}...` : body;
      throw new Error(`ADO API ${res.status} from ${url}: ${truncated}`);
    }
    return res.json() as Promise<T>;
  }

  private async requestRaw(
    endpoint: string,
    scope: "org" | "project" = "project",
  ): Promise<string> {
    const url = this.buildUrl(endpoint, scope);
    const res = await fetch(url, {
      headers: {
        Authorization: this.authHeader,
        Accept: "text/plain",
      },
    });
    if (!res.ok) {
      const body = await res.text().catch(() => "");
      const truncated = body.length > 200 ? `${body.slice(0, 200)}...` : body;
      throw new Error(`ADO API ${res.status} from ${url}: ${truncated}`);
    }
    return res.text();
  }

  async getUserIdentity(): Promise<{ id: string; displayName: string }> {
    const data = await this.request<{ authenticatedUser: { id: string; displayName: string } }>(
      "/connectionData",
      undefined,
      "org",
      "7.1-preview.1",
    );
    return data.authenticatedUser;
  }

  async listPullRequests(repo: string, options?: { status?: string; reviewerId?: string; creatorId?: string }) {
    const params = new URLSearchParams({ "searchCriteria.status": options?.status ?? "active" });
    if (options?.reviewerId) params.set("searchCriteria.reviewerId", options.reviewerId);
    if (options?.creatorId) params.set("searchCriteria.creatorId", options.creatorId);
    const data = await this.request<{ value: any[] }>(
      `/git/repositories/${encodeURIComponent(repo)}/pullrequests?${params.toString()}`,
    );
    return data.value;
  }

  async getPullRequest(repo: string, prId: number) {
    return this.request<any>(`/git/repositories/${encodeURIComponent(repo)}/pullrequests/${prId}`);
  }

  async getThreads(repo: string, prId: number) {
    const data = await this.request<{ value: any[] }>(
      `/git/repositories/${encodeURIComponent(repo)}/pullrequests/${prId}/threads`,
    );
    return data.value;
  }

  async voteReviewer(repo: string, prId: number, userId: string, vote: number) {
    await this.request(
      `/git/repositories/${encodeURIComponent(repo)}/pullrequests/${prId}/reviewers/${encodeURIComponent(userId)}`,
      { method: "PUT", body: JSON.stringify({ vote }) },
    );
  }

  async createThread(repo: string, prId: number, comment: string, context?: { filePath?: string; line?: number }) {
    const threadContext = context?.filePath
      ? {
          filePath: context.filePath,
          ...(context.line
            ? {
                rightFileStart: { line: context.line, offset: 1 },
                rightFileEnd: { line: context.line, offset: 1 },
              }
            : {}),
        }
      : undefined;
    return this.request<any>(
      `/git/repositories/${encodeURIComponent(repo)}/pullrequests/${prId}/threads`,
      {
        method: "POST",
        body: JSON.stringify({
          comments: [{ content: comment, commentType: "text" }],
          status: "active",
          ...(threadContext ? { threadContext } : {}),
        }),
      },
    );
  }

  // ─── Diff & iteration methods ────────────────────────────────────

  async getIterations(repo: string, prId: number) {
    const data = await this.request<{ value: any[] }>(
      `/git/repositories/${encodeURIComponent(repo)}/pullrequests/${prId}/iterations`,
    );
    return data.value;
  }

  async getIterationChanges(repo: string, prId: number, iterationId: number) {
    const data = await this.request<{ changeEntries: any[] }>(
      `/git/repositories/${encodeURIComponent(repo)}/pullrequests/${prId}/iterations/${iterationId}/changes`,
    );
    return data.changeEntries ?? [];
  }

  async getCommits(repo: string, prId: number) {
    const data = await this.request<{ value: any[] }>(
      `/git/repositories/${encodeURIComponent(repo)}/pullrequests/${prId}/commits`,
    );
    return data.value ?? [];
  }

  async getFileContent(repo: string, path: string, branch: string): Promise<string> {
    const url = `/_apis/git/repositories/${encodeURIComponent(repo)}/items`
      + `?path=${encodeURIComponent(path)}`
      + `&versionDescriptor.version=${encodeURIComponent(branch)}`
      + `&versionDescriptor.versionType=branch`
      + `&$format=text`;
    const res = await this.requestRaw(url);
    return res;
  }

  async getPrSourceBranch(repo: string, prId: number): Promise<string> {
    const pr = await this.getPullRequest(repo, prId);
    return (pr.sourceRefName ?? "").replace("refs/heads/", "");
  }

  // ─── Work Item Tracking (WIT) methods ────────────────

  async updateWorkItem(id: number, patchOps: Array<{ op: string; path: string; value: any }>): Promise<any> {
    const url = this.buildUrl(`/_apis/wit/workitems/${id}`, "org");
    const res = await fetch(url, {
      method: "PATCH",
      headers: {
        Authorization: this.authHeader,
        Accept: "application/json",
        "Content-Type": "application/json-patch+json",
      },
      body: JSON.stringify(patchOps),
    });
    if (!res.ok) {
      const body = await res.text().catch(() => "");
      const truncated = body.length > 200 ? `${body.slice(0, 200)}...` : body;
      throw new Error(`ADO API ${res.status} from ${url}: ${truncated}`);
    }
    return res.json();
  }

  async getWorkItemComments(id: number): Promise<any> {
    const data = await this.request<{ value: any[] }>(
      `/_apis/wit/workitems/${id}/comments`,
      undefined,
      "project",
      "7.1-preview.2",
    );
    return data;
  }

  async addWorkItemComment(id: number, text: string): Promise<any> {
    return this.request<any>(
      `/_apis/wit/workitems/${id}/comments`,
      {
        method: "POST",
        body: JSON.stringify({ text }),
      },
      "project",
      "7.1-preview.2",
    );
  }

  async getWorkItemTypes(): Promise<any[]> {
    const data = await this.request<{ value: any[] }>(
      "/wit/workitemtypes",
      undefined,
      "project",
    );
    return data.value ?? [];
  }

  async queryWiql(wiql: string): Promise<{ workItems?: Array<{ id: number }>; workItemRelations?: Array<{ source?: { id: number }; target?: { id: number } }> }> {
    const data = await this.request<{ workItems?: Array<{ id: number }>; workItemRelations?: Array<{ source?: { id: number }; target?: { id: number } }> }>(
      "/wit/wiql",
      {
        method: "POST",
        body: JSON.stringify({ query: wiql }),
      },
      "project",
      "7.1-preview.2",
    );
    return data;
  }

  async getWorkItemsByIds(ids: number[], fields?: string[]): Promise<any[]> {
    if (ids.length === 0) return [];
    const defaultFields = [
      "System.Id", "System.Title", "System.State", "System.WorkItemType",
      "System.AssignedTo", "Microsoft.VSTS.Common.Priority", "System.ChangedDate",
    ];
    const fieldsParam = (fields ?? defaultFields).join(",");
    const idsParam = ids.slice(0, 200).join(",");
    const data = await this.request<{ value: any[] }>(
      `/_apis/wit/workitems?ids=${idsParam}&fields=${fieldsParam}`,
      undefined,
      "org",
    );
    return data.value ?? [];
  }

  async getWorkItem(id: number, options?: { expandRelations?: boolean }): Promise<any> {
    const expand = options?.expandRelations ? "?$expand=relations" : "";
    return this.request<any>(
      `/_apis/wit/workitems/${id}${expand}`,
      undefined,
      "org",
    );
  }
}

// ─── Helpers ──────────────────────────────────────────────────────────────

function guessLang(path: string): string {
  const ext = path.split(".").pop()?.toLowerCase() ?? "";
  const map: Record<string, string> = {
    ts: "typescript", tsx: "typescript", js: "javascript", jsx: "javascript",
    py: "python", rs: "rust", go: "go", java: "java", kt: "kotlin",
    cs: "csharp", rb: "ruby", php: "php", swift: "swift", c: "c", cpp: "cpp",
    scss: "scss", css: "css", html: "html", sql: "sql", sh: "bash",
    yaml: "yaml", yml: "yaml", json: "json", xml: "xml", md: "markdown",
  };
  return map[ext] ?? "";
}

function wiqlLiteral(value: string): string {
  return `'${value.replace(/'/g, "''")}'`;
}

function assignedToCondition(assignedTo?: string): string {
  const normalized = assignedTo?.trim();
  if (!normalized || normalized.toLowerCase() === "me" || normalized.toLowerCase() === "@me") {
    return "[System.AssignedTo] = @Me";
  }
  return `[System.AssignedTo] = ${wiqlLiteral(normalized)}`;
}

function filterLabel(value?: string): string {
  if (!value || value.trim().toLowerCase() === "me" || value.trim().toLowerCase() === "@me") return "@Me";
  return value.trim();
}

function workItemIdFromUrl(url?: string): number | undefined {
  const match = url?.match(/\/workItems\/(\d+)$/i);
  return match ? Number(match[1]) : undefined;
}

function relationLabel(rel: any): string {
  return rel?.attributes?.name ?? rel?.rel ?? "Related";
}

async function formatWorkItemRelations(ado: AdoClient, wi: any): Promise<string> {
  const relations = (wi.relations ?? [])
    .map((rel: any) => ({ rel, id: workItemIdFromUrl(rel.url) }))
    .filter((item: { id?: number }) => item.id !== undefined) as Array<{ rel: any; id: number }>;
  if (!relations.length) return "\n## Related Work Items\n\nNo related work items found.\n";

  const relatedIds = [...new Set(relations.map((r) => r.id))];
  const relatedItems = await ado.getWorkItemsByIds(relatedIds, [
    "System.Id", "System.Title", "System.State", "System.WorkItemType", "System.AssignedTo",
  ]).catch(() => []);
  const byId = new Map(relatedItems.map((item: any) => [item.id, item]));

  let out = "\n## Related Work Items\n";
  for (const relation of relations) {
    const related = byId.get(relation.id);
    if (related) {
      const title = related.fields?.["System.Title"] ?? "?";
      const type = related.fields?.["System.WorkItemType"] ?? "?";
      const state = related.fields?.["System.State"] ?? "?";
      out += `- #${relation.id} ${title} [${type}] — State: ${state}, Relation: ${relationLabel(relation.rel)}\n`;
    } else {
      out += `- #${relation.id} — Relation: ${relationLabel(relation.rel)}\n`;
    }
  }
  return out;
}

function commentList(commentsData: any): any[] {
  return commentsData?.comments ?? commentsData?.value ?? [];
}

function plainText(value: unknown): string {
  if (value === undefined || value === null) return "";
  return String(value)
    .replace(/<br\s*\/?>/gi, "\n")
    .replace(/<\/(p|div|li)>/gi, "\n")
    .replace(/<[^>]+>/g, "")
    .replace(/&nbsp;/g, " ")
    .trim();
}

function isQaFeedbackWorkItem(wi: any): boolean {
  const type = String(wi?.fields?.["System.WorkItemType"] ?? "").toLowerCase();
  return type.includes("qa") || type.includes("feedback") || type.includes("test feedback");
}

async function formatComments(ado: AdoClient, id: number): Promise<string> {
  const commentsData = await ado.getWorkItemComments(id).catch(() => ({ value: [] }));
  const comments = commentList(commentsData);
  if (!comments.length) return "\n## Comments\n\nNo comments found.\n";
  let out = `\n## Comments (${comments.length})\n`;
  for (const c of comments.slice(0, 20)) {
    const author = c.createdBy?.displayName ?? c.author?.displayName ?? "?";
    const date = c.createdDate ?? c.date ?? "";
    const text = plainText(c.text ?? c.renderedText).slice(0, 1000).replace(/\n/g, "\n  ");
    out += `- **${author}**${date ? ` (${date})` : ""}: ${text}\n`;
  }
  return out;
}

async function formatWorkItemFullDetail(ado: AdoClient, wi: any, title: string): Promise<string> {
  let out = `${title}\n\n${fmtWorkItemDetail(wi)}`;
  out += await formatWorkItemRelations(ado, wi);
  out += await formatComments(ado, wi.id);
  return out;
}

// ─── Server Plugin ────────────────────────────────────────────────────────

const server: Plugin = async (input: PluginInput, options?: PluginOptions): Promise<Hooks> => {
  const { client } = input;

  // ─── Config loader ───────────────────────────────────────────────
  async function loadConfig() {
    const fromOptions = asAdoConfig(options);
    if (fromOptions) return fromOptions;

    const resp = await client.config.get().catch(() => ({ data: {} }));
    const data = (resp.data ?? {}) as Record<string, unknown>;
    const ado = asAdoConfig(data["ado"]);
    if (!ado || !ado.profiles || Object.keys(ado.profiles).length === 0) {
      throw new Error("No ADO config found. Add an 'ado' section to opencode.json with profiles.");
    }
    return ado;
  }

  async function createClient(profileOverride?: string) {
    const config = await loadConfig();

    // 1. Explicit profile override (from tool arg)
    // 2. Persisted active profile (from profile-store)
    // 3. Config defaultProfile / default:true / first profile

    let profileName = profileOverride;
    if (!profileName) {
      const persisted = getActiveProfile();
      if (persisted && config.profiles[persisted]) {
        profileName = persisted;
      }
    }

    const { name, profile } = profileName && config.profiles[profileName]
      ? { name: profileName, profile: config.profiles[profileName] }
      : resolveActiveProfile(config);

    const pat = getPAT(profile.patEnvVar);
    const orgUrl = resolveOrgUrl(profile.org);
    const ado = new AdoClient(orgUrl, profile.project, pat);
    const userId = await ado.getUserIdentity();
    return { client: ado, profile, name, userId };
  }

  /** Resolve repo+prId from args, falling back to the selected PR in the sidebar store. */
  function resolvePrArgs(args: { repo?: string; prId?: number }): { repo: string; prId: number } {
    if (args.repo && args.prId) return { repo: args.repo, prId: args.prId };
    const selected = getSelectedPr();
    if (!selected) throw new Error("No repo/prId provided and no PR selected in the sidebar. Use ado_select_pr first or pass repo and prId.");
    return selected;
  }

  // ─── Tools ───────────────────────────────────────────────────────

  return {
    tool: {
      ado_prs: {
        description: "List active pull requests from configured Azure DevOps repos. Shows pending reviews and your own PRs.",
        args: { profile: z.string().optional().describe("Optional profile name override") },
        async execute({ profile }: { profile?: string }) {
          const { client: ado, profile: prof, name, userId } = await createClient(profile);
          const allPRs: any[] = [];
          for (const repo of prof.repos) {
            try {
              const prs = await ado.listPullRequests(repo, { status: "active" });
              allPRs.push(...prs);
            } catch { /* skip failing repos */ }
          }

          const pending = allPRs.filter(pr => pr.reviewers?.some((r: any) => reviewerMatchesUser(r, userId.id) && r.vote === 0));
          const mine = allPRs.filter(pr => pr.createdBy?.id === userId.id);

          let out = `## Azure DevOps — Active PRs (${name})\n\n`;
          if (pending.length) { out += `### Pending Your Review (${pending.length})\n\n${pending.map(fmtPR).join("\n")}\n\n`; }
          if (mine.length) { out += `### Your PRs (${mine.length})\n\n${mine.map(fmtPR).join("\n")}\n\n`; }
          if (!pending.length && !mine.length) out += "No active PRs requiring your attention.\n";
          out += `\n---\nTotal active: ${allPRs.length} across ${prof.repos.length} repos`;
          return out;
        },
      },

      ado_pr: {
        description: "Show details for a specific Azure DevOps pull request. If repo/prId are omitted, uses the PR selected in the sidebar.",
        args: {
          repo: z.string().optional().describe("Repository name (omit to use sidebar selection)"),
          prId: z.number().optional().describe("Pull request ID (omit to use sidebar selection)"),
          profile: z.string().optional().describe("Optional profile name override"),
        },
        async execute({ repo, prId, profile }: { repo?: string; prId?: number; profile?: string }) {
          const resolved = resolvePrArgs({ repo, prId });
          const { client: ado, name } = await createClient(profile);
          const pr = await ado.getPullRequest(resolved.repo, resolved.prId);
          return `## PR #${resolved.prId} (${resolved.repo}) — ${name}\n\n${fmtPRDetail(pr)}`;
        },
      },

      ado_pr_threads: {
        description: "Show comment threads for an Azure DevOps pull request. If repo/prId are omitted, uses the PR selected in the sidebar.",
        args: {
          repo: z.string().optional().describe("Repository name (omit to use sidebar selection)"),
          prId: z.number().optional().describe("Pull request ID (omit to use sidebar selection)"),
          profile: z.string().optional().describe("Optional profile name override"),
        },
        async execute({ repo, prId, profile }: { repo?: string; prId?: number; profile?: string }) {
          const resolved = resolvePrArgs({ repo, prId });
          const { client: ado } = await createClient(profile);
          const threads = await ado.getThreads(resolved.repo, resolved.prId);
          if (!threads.length) return `No threads found for PR #${resolved.prId}.`;
          return `## Threads for PR #${resolved.prId} (${resolved.repo})\n\n${threads.map(fmtThread).join("\n")}`;
        },
      },

      ado_pr_comment: {
        description: "Add a standalone comment thread to an Azure DevOps pull request. If repo/prId are omitted, uses the PR selected in the sidebar. Optionally attach it to a file and line.",
        args: {
          repo: z.string().optional().describe("Repository name (omit to use sidebar selection)"),
          prId: z.number().optional().describe("Pull request ID (omit to use sidebar selection)"),
          comment: z.string().describe("Comment text"),
          filePath: z.string().optional().describe("Optional file path to attach the comment to, e.g. /src/app.ts"),
          line: z.number().optional().describe("Optional 1-based line number. Requires filePath."),
          profile: z.string().optional().describe("Optional profile name override"),
        },
        async execute({ repo, prId, comment, filePath, line, profile }: { repo?: string; prId?: number; comment: string; filePath?: string; line?: number; profile?: string }) {
          if (line !== undefined && !filePath) return "Provide filePath when specifying line.";
          const resolved = resolvePrArgs({ repo, prId });
          const { client: ado } = await createClient(profile);
          await ado.createThread(resolved.repo, resolved.prId, comment, { filePath, line });
          let out = `## PR Comment Added\n\nPR: #${resolved.prId} in ${resolved.repo}\n`;
          if (filePath) out += `File: ${filePath}\n`;
          if (line !== undefined) out += `Line: ${line}\n`;
          out += `Comment: ${comment}\n`;
          return out;
        },
      },

      ado_review: {
        description: "Vote on an Azure DevOps pull request with an optional comment. If repo/prId are omitted, uses the PR selected in the sidebar.",
        args: {
          repo: z.string().optional().describe("Repository name (omit to use sidebar selection)"),
          prId: z.number().optional().describe("Pull request ID (omit to use sidebar selection)"),
          vote: z.enum(["approve", "reject", "wait", "suggestions"]).describe("Vote: approve, reject, wait, or suggestions"),
          comment: z.string().optional().describe("Optional review comment"),
          profile: z.string().optional().describe("Optional profile name override"),
        },
        async execute({ repo, prId, vote: voteStr, comment, profile }: { repo?: string; prId?: number; vote: string; comment?: string; profile?: string }) {
          const resolved = resolvePrArgs({ repo, prId });
          const { client: ado, userId } = await createClient(profile);
          const voteMap: Record<string, number> = { approve: 10, suggestions: 5, wait: -5, reject: -10 };
          const voteValue = voteMap[voteStr];
          if (voteValue === undefined) return `Invalid vote: ${voteStr}. Use: approve, reject, wait, suggestions`;

          await ado.voteReviewer(resolved.repo, resolved.prId, userId.id, voteValue);
          if (comment) await ado.createThread(resolved.repo, resolved.prId, comment);

          const labels: Record<number, string> = { 10: "Approved", 5: "Approved with suggestions", "-5": "Waiting for author", "-10": "Rejected" };
          let out = `## Review Submitted\n\nPR: #${resolved.prId} in ${resolved.repo}\nVote: ${labels[voteValue]}\n`;
          if (comment) out += `Comment: ${comment}\n`;
          return out;
        },
      },

      ado_profile: {
        description: "Show the active Azure DevOps profile configuration.",
        args: { profile: z.string().optional().describe("Optional profile name override") },
        async execute({ profile }: { profile?: string }) {
          const { profile: prof, name } = await createClient(profile);
          return [
            `## Azure DevOps Profile: ${name}`,
            ``,
            `org: ${prof.org}`,
            `project: ${prof.project}`,
            `repos: ${prof.repos.join(", ")}`,
            `patEnvVar: ${prof.patEnvVar}`,
          ].join("\n");
        },
      },

      // ─── New tools: profiles ────────────────────────────────────

      ado_profiles: {
        description: "List available Azure DevOps profiles.",
        args: {},
        async execute() {
          const config = await loadConfig();
          const active = getActiveProfile();
          const lines = ["## Azure DevOps Profiles\n"];
          for (const [name, p] of Object.entries(config.profiles)) {
            const marker = name === active || (!active && name === config.defaultProfile) ? " ← active" : "";
            lines.push(`- **${name}**${marker}: org=${p.org}, project=${p.project}, repos=${p.repos.join(",")}`);
          }
          if (active) lines.push(`\nActive profile: ${active}`);
          else if (config.defaultProfile) lines.push(`\nActive profile: ${config.defaultProfile} (from config)`);
          return lines.join("\n");
        },
      },

      ado_profile_use: {
        description: "Switch the active Azure DevOps profile. Persists across restarts.",
        args: { name: z.string().describe("Profile name to activate") },
        async execute({ name }: { name: string }) {
          const config = await loadConfig();
          if (!config.profiles[name]) {
            const available = Object.keys(config.profiles).join(", ");
            return `Profile "${name}" not found. Available: ${available}`;
          }
          setActiveProfile(name);
          return `Active profile set to: **${name}**\nOrg: ${config.profiles[name].org}, Project: ${config.profiles[name].project}`;
        },
      },

      // ─── New tools: selection ──────────────────────────────────────

      ado_select_pr: {
        description: "Select a PR in the sidebar. The selected PR is highlighted and persisted across refreshes.",
        args: {
          repo: z.string().describe("Repository name"),
          prId: z.number().describe("Pull request ID"),
        },
        async execute({ repo, prId }: { repo: string; prId: number }) {
          setSelectedPr(repo, prId);
          return `Selected PR **#${prId}** in **${repo}**. The sidebar will highlight this PR on the next refresh.`;
        },
      },

      // ─── New tools: diff & review context ──────────────────────

      ado_pr_diff: {
        description: "Get the list of changed files and their change types for an Azure DevOps pull request. If repo/prId are omitted, uses the PR selected in the sidebar.",
        args: {
          repo: z.string().optional().describe("Repository name (omit to use sidebar selection)"),
          prId: z.number().optional().describe("Pull request ID (omit to use sidebar selection)"),
          profile: z.string().optional().describe("Optional profile name override"),
        },
        async execute({ repo, prId, profile }: { repo?: string; prId?: number; profile?: string }) {
          const resolved = resolvePrArgs({ repo, prId });
          const { client: ado, name } = await createClient(profile);

          // Get latest iteration
          const iterations = await ado.getIterations(resolved.repo, resolved.prId);
          if (!iterations?.length) return `No iterations found for PR #${resolved.prId}`;

          const latest = iterations[iterations.length - 1];
          const changes = await ado.getIterationChanges(resolved.repo, resolved.prId, latest.id);

          if (!changes?.length) return `No file changes found for PR #${resolved.prId}`;

          const files = changes
            .filter((c: any) => c.item && !c.item.isFolder)
            .map((c: any) => {
              const type = c.changeType ?? "?";
              const path = c.item.path ?? "?";
              return `  [${type}] ${path}`;
            });

          let out = `## Changed Files — PR #${resolved.prId} in ${resolved.repo} (${name})\n`;
          out += `Iteration: ${latest.id} (${latest.sourceRefCommit?.commitId?.slice(0, 8) ?? "?"})\n`;
          out += `Files changed: ${files.length}\n\n`;
          out += files.join("\n");
          return out;
        },
      },

      // ─── New tools: file content ───────────────────────────────────

      ado_pr_file: {
        description: "Get the content of a file from a PR's source branch. Use this to review actual code changes. Supports optional line range.",
        args: {
          path: z.string().describe("File path in the repository (e.g. /src/app/config.ts)"),
          repo: z.string().optional().describe("Repository name (omit to use sidebar selection)"),
          prId: z.number().optional().describe("Pull request ID (omit to use sidebar selection)"),
          startLine: z.number().optional().describe("Start line (1-based, optional)"),
          endLine: z.number().optional().describe("End line (1-based, optional)"),
          profile: z.string().optional().describe("Optional profile name override"),
        },
        async execute({ path, repo, prId, startLine, endLine, profile }: { path: string; repo?: string; prId?: number; startLine?: number; endLine?: number; profile?: string }) {
          const resolved = resolvePrArgs({ repo, prId });
          const { client: ado, name } = await createClient(profile);

          const branch = await ado.getPrSourceBranch(resolved.repo, resolved.prId);
          const content = await ado.getFileContent(resolved.repo, path, branch);

          const MAX_FILE_SIZE = 15000;
          let out = `## File: ${path} (PR #${resolved.prId}, branch: ${branch})\n\n`;

          if (startLine || endLine) {
            const lines = content.split("\n");
            const start = (startLine ?? 1) - 1;
            const end = endLine ?? lines.length;
            const slice = lines.slice(start, end);
            out += `Lines ${(start + 1)}-${Math.min(end, lines.length)} of ${lines.length}:\n\n`;
            out += "```" + guessLang(path) + "\n";
            for (let i = 0; i < slice.length; i++) {
              out += `${String(start + 1 + i).padStart(4)} | ${slice[i]}\n`;
            }
            out += "```";
          } else {
            if (content.length > MAX_FILE_SIZE) {
              out += `⚠ File truncated (${content.length} chars, showing first ${MAX_FILE_SIZE})\n\n`;
              out += "```" + guessLang(path) + "\n";
              out += content.slice(0, MAX_FILE_SIZE);
              out += "\n```";
            } else {
              out += "```" + guessLang(path) + "\n";
              out += content;
              out += "\n```";
            }
          }

          return out;
        },
      },

      // ─── New tools: review context ─────────────────────────────────

      ado_pr_review_context: {
        description: "Get a complete review context bundle for an Azure DevOps PR: metadata, threads, changed files, and commit info. Optimized for AI code review. If repo/prId are omitted, uses the PR selected in the sidebar.",
        args: {
          repo: z.string().optional().describe("Repository name (omit to use sidebar selection)"),
          prId: z.number().optional().describe("Pull request ID (omit to use sidebar selection)"),
          profile: z.string().optional().describe("Optional profile name override"),
        },
        async execute({ repo, prId, profile }: { repo?: string; prId?: number; profile?: string }) {
          const resolved = resolvePrArgs({ repo, prId });
          const { client: ado, name } = await createClient(profile);

          // Parallel fetch
          const [pr, threads, iterations, commits] = await Promise.all([
            ado.getPullRequest(resolved.repo, resolved.prId),
            ado.getThreads(resolved.repo, resolved.prId).catch(() => []),
            ado.getIterations(resolved.repo, resolved.prId).catch(() => []),
            ado.getCommits(resolved.repo, resolved.prId).catch(() => []),
          ]);

          // Changed files from latest iteration
          let changedFiles: string[] = [];
          if (iterations.length) {
            const latest = iterations[iterations.length - 1];
            const changes = await ado.getIterationChanges(resolved.repo, resolved.prId, latest.id).catch(() => []);
            const entries = Array.isArray(changes) ? changes : [];
            changedFiles = entries
              .filter((c: any) => c.item && !c.item.isFolder)
              .map((c: any) => `  [${c.changeType ?? "?"}] ${c.item.path ?? "?"}`);
          }

          // Truncation budget
          const MAX_TOTAL = 30000;

          let out = `# Review Context — PR #${resolved.prId} in ${resolved.repo} (${name})\n\n`;

          // Metadata
          out += `## Metadata\n`;
          out += `- **Title**: ${pr.title}\n`;
          out += `- **Author**: ${pr.createdBy?.displayName ?? "?"}\n`;
          out += `- **Status**: ${pr.status}\n`;
          out += `- **Source**: ${shortBranch(pr.sourceRefName)}\n`;
          out += `- **Target**: ${shortBranch(pr.targetRefName)}\n`;
          out += `- **Draft**: ${pr.isDraft ? "Yes" : "No"}\n`;
          out += `- **Created**: ${pr.creationDate ?? "?"}\n`;
          if (pr.description) {
            const desc = pr.description.length > 500 ? pr.description.slice(0, 500) + "..." : pr.description;
            out += `- **Description**: ${desc}\n`;
          }
          out += `\n`;

          // Reviewers
          if (pr.reviewers?.length) {
            out += `## Reviewers\n`;
            for (const r of pr.reviewers) {
              const vote = r.vote === 10 ? "✓" : r.vote === -10 ? "✗" : r.vote === -5 ? "⏳" : "—";
              out += `- ${vote} ${r.votedBy?.displayName ?? r.displayName ?? "?"}\n`;
            }
            out += `\n`;
          }

          // Commits
          if (commits.length) {
            out += `## Commits (${commits.length})\n`;
            for (const c of commits.slice(0, 20)) {
              out += `- ${c.commitId?.slice(0, 8) ?? "?"} ${c.comment?.slice(0, 80) ?? ""}\n`;
            }
            if (commits.length > 20) out += `  ... and ${commits.length - 20} more\n`;
            out += `\n`;
          }

          // Changed files
          if (changedFiles.length) {
            out += `## Changed Files (${changedFiles.length})\n`;
            out += changedFiles.join("\n");
            out += `\n\n`;
          }

          // Threads
          if (threads.length) {
            out += `## Threads (${threads.length})\n`;
            for (const t of threads) {
              const file = t.threadContext?.filePath ?? "";
              const status = t.status ?? "?";
              const firstComment = t.comments?.[0]?.content ?? "";
              out += `### [${status}] ${file ? file + ": " : ""}${firstComment.slice(0, 120)}\n`;
              out += `(${t.comments?.length ?? 0} comments)\n`;
              for (const c of (t.comments ?? []).slice(0, 5)) {
                const author = c.author?.displayName ?? "?";
                out += `- **${author}**: ${c.content?.slice(0, 200) ?? ""}\n`;
              }
              out += `\n`;
            }
          }

          // Truncation warning
          if (out.length > MAX_TOTAL) {
            out = out.slice(0, MAX_TOTAL) + "\n\n⚠ **Output truncated to fit context window. Use ado_pr_diff for file details or ado_pr_threads for full thread content.**\n";
          }

          return out;
        },
      },

      // ─── Work Item tools ──────────────────────────────────────────────

      ado_work_items: {
        description: "List work items from Azure DevOps. Supports filtering by state, assignedTo, and tag.",
        args: {
          state: z.string().optional().describe("Filter by work item state (e.g. Active, New, Resolved)"),
          assignedTo: z.string().optional().describe("Filter by assigned user (defaults to @Me)"),
          tag: z.string().optional().describe("Filter by tag (WIQL CONTAINS match, e.g. 'bug', 'backend')"),
          profile: z.string().optional().describe("Optional profile name override"),
        },
        async execute({ state, assignedTo, tag, profile }: { state?: string; assignedTo?: string; tag?: string; profile?: string }) {
          const { client: ado, name } = await createClient(profile);

          // Build WIQL dynamically based on provided filters
          const conditions = [`[System.State] <> 'Closed'`];
          conditions.push(assignedToCondition(assignedTo));
          if (state) conditions.push(`[System.State] = ${wiqlLiteral(state)}`);
          if (tag) conditions.push(`[System.Tags] CONTAINS ${wiqlLiteral(tag)}`);

          const wiql = `SELECT [System.Id] FROM WorkItems WHERE ${conditions.join(" AND ")} ORDER BY [System.ChangedDate] DESC`;
          const wiqlResult = await ado.queryWiql(wiql);
          const ids = (wiqlResult.workItems ?? []).map((wi: any) => wi.id);

          // Build active filter label for header
          const activeFilters: string[] = [];
          activeFilters.push(`assignedTo=${filterLabel(assignedTo)}`);
          if (state) activeFilters.push(`state=${state}`);
          if (tag) activeFilters.push(`tag=${tag}`);
          const filterSuffix = activeFilters.length ? ` — ${activeFilters.join(", ")}` : "";

          if (ids.length === 0) return `## Work Items (${name})${filterSuffix}\n\nNo work items found.`;
          const workItems = await ado.getWorkItemsByIds(ids);
          let out = `## Work Items (${name})${filterSuffix}\n\n`;
          out += workItems.map(fmtWorkItem).join("\n");
          out += `\n\n---\nTotal: ${workItems.length}`;
          return out;
        },
      },

      ado_work_item: {
        description: "Show details and comments for a specific Azure DevOps work item.",
        args: {
          id: z.number().describe("Work item ID"),
          profile: z.string().optional().describe("Optional profile name override"),
        },
        async execute({ id, profile }: { id: number; profile?: string }) {
          const { client: ado, name } = await createClient(profile);
          const wi = await ado.getWorkItem(id, { expandRelations: true });
          return formatWorkItemFullDetail(ado, wi, `## Work Item #${id} (${name})`);
        },
      },

      ado_work_item_update: {
        description: "Update an Azure DevOps work item. Supports changing state, priority, and other fields via JSON Patch.",
        args: {
          id: z.number().describe("Work item ID"),
          state: z.string().optional().describe("New state value (e.g. Active, Resolved, Closed)"),
          priority: z.number().optional().describe("New priority value"),
          comment: z.string().optional().describe("Optional comment to add with the update"),
          profile: z.string().optional().describe("Optional profile name override"),
        },
        async execute({ id, state, priority, comment, profile }: { id: number; state?: string; priority?: number; comment?: string; profile?: string }) {
          const { client: ado, name } = await createClient(profile);
          const patchOps: Array<{ op: string; path: string; value: any }> = [];
          if (state) patchOps.push({ op: "replace", path: "/fields/System.State", value: state });
          if (priority !== undefined) patchOps.push({ op: "replace", path: "/fields/Microsoft.VSTS.Common.Priority", value: priority });
          if (patchOps.length === 0 && !comment) return "No changes specified. Provide --state, --priority, or --comment.";
          if (patchOps.length > 0) await ado.updateWorkItem(id, patchOps);
          if (comment) await ado.addWorkItemComment(id, comment);
          let out = `## Work Item Updated\n\nWork Item: #${id}\n`;
          if (state) out += `State: ${state}\n`;
          if (priority !== undefined) out += `Priority: ${priority}\n`;
          if (comment) out += `Comment added\n`;
          return out;
        },
      },

      ado_work_item_comment: {
        description: "Add a comment to an Azure DevOps work item.",
        args: {
          id: z.number().describe("Work item ID"),
          comment: z.string().describe("Comment text"),
          profile: z.string().optional().describe("Optional profile name override"),
        },
        async execute({ id, comment, profile }: { id: number; comment: string; profile?: string }) {
          const { client: ado } = await createClient(profile);
          await ado.addWorkItemComment(id, comment);
          return `Comment added to work item #${id}.`;
        },
      },

      ado_work_item_types: {
        description: "List available work item types in the Azure DevOps project. Used to discover custom types like QA Feedback.",
        args: { profile: z.string().optional().describe("Optional profile name override") },
        async execute({ profile }: { profile?: string }) {
          const { client: ado, name } = await createClient(profile);
          const types = await ado.getWorkItemTypes();
          let out = `## Work Item Types (${name})\n\n`;
          for (const t of types) {
            out += `- **${t.name}**: ${t.description ?? "No description"}\n`;
          }
          out += `\n---\nTotal: ${types.length} types`;
          return out;
        },
      },

      // ─── QA Feedback tools ────────────────────────────────────────────

      ado_qa_feedbacks: {
        description: "List QA Feedback work items from Azure DevOps. Discovers the QA Feedback type name dynamically. Supports filtering by state and assignedTo.",
        args: {
          state: z.string().optional().describe("Filter by work item state (e.g. Active, New, Resolved)"),
          assignedTo: z.string().optional().describe("Filter by assigned user (defaults to @Me)"),
          profile: z.string().optional().describe("Optional profile name override"),
        },
        async execute({ state, assignedTo, profile }: { state?: string; assignedTo?: string; profile?: string }) {
          const { client: ado, name } = await createClient(profile);
          // First discover QA feedback type name(s)
          const types = await ado.getWorkItemTypes();
          const qaTypes = types.filter((t: any) =>
            t.name?.toLowerCase().includes("qa") ||
            t.name?.toLowerCase().includes("feedback") ||
            t.name?.toLowerCase().includes("test feedback"),
          );
          if (qaTypes.length === 0) return `## QA Feedbacks (${name})\n\nNo QA Feedback work item type found in this project.`;

          // Build WIQL dynamically based on provided filters
          const typeNames = qaTypes.map((t: any) => `'${t.name}'`).join(", ");
          const conditions = [
            `[System.WorkItemType] IN (${typeNames})`,
            `[System.State] <> 'Closed'`,
          ];
          conditions.push(assignedToCondition(assignedTo));
          if (state) conditions.push(`[System.State] = ${wiqlLiteral(state)}`);

          const wiql = `SELECT [System.Id] FROM WorkItems WHERE ${conditions.join(" AND ")} ORDER BY [System.ChangedDate] DESC`;
          const wiqlResult = await ado.queryWiql(wiql);
          const ids = (wiqlResult.workItems ?? []).map((wi: any) => wi.id);

          // Build active filter label for header
          const activeFilters: string[] = [];
          activeFilters.push(`assignedTo=${filterLabel(assignedTo)}`);
          if (state) activeFilters.push(`state=${state}`);
          const filterSuffix = activeFilters.length ? ` — ${activeFilters.join(", ")}` : "";

          if (ids.length === 0) return `## QA Feedbacks (${name})${filterSuffix}\n\nNo QA Feedbacks found.`;
          const workItems = await ado.getWorkItemsByIds(ids);
          let out = `## QA Feedbacks (${name})${filterSuffix}\n\n`;
          out += `Types: ${typeNames}\n\n`;
          out += workItems.map(fmtQaFeedback).join("\n");
          out += `\n\n---\nTotal: ${workItems.length}`;
          return out;
        },
      },

      ado_qa_feedback: {
        description: "Show details and comments for a QA Feedback work item.",
        args: {
          id: z.number().describe("Work item ID"),
          profile: z.string().optional().describe("Optional profile name override"),
        },
        async execute({ id, profile }: { id: number; profile?: string }) {
          const { client: ado, name } = await createClient(profile);
          const wi = await ado.getWorkItem(id, { expandRelations: true });
          return formatWorkItemFullDetail(ado, wi, `## QA Feedback #${id} (${name})`);
        },
      },

      ado_qa_feedbacks_for_work_item: {
        description: "List QA Feedbacks associated with a parent/related work item, including each feedback's full detail, comments, and relations. Use this before writing QA plan markdown files.",
        args: {
          id: z.number().describe("Parent or related work item ID, e.g. a User Story like 13494"),
          state: z.string().optional().describe("Optional QA Feedback state filter (e.g. New, Active, Resolved)"),
          profile: z.string().optional().describe("Optional profile name override"),
        },
        async execute({ id, state, profile }: { id: number; state?: string; profile?: string }) {
          const { client: ado, name } = await createClient(profile);
          const parent = await ado.getWorkItem(id, { expandRelations: true });
          const relationIds = [
            ...new Set((parent.relations ?? [])
              .map((rel: any) => workItemIdFromUrl(rel.url))
              .filter((relatedId: number | undefined) => relatedId !== undefined)),
          ] as number[];

          const relatedItems = relationIds.length
            ? await ado.getWorkItemsByIds(relationIds, [
              "System.Id", "System.Title", "System.State", "System.WorkItemType",
              "System.AssignedTo", "Microsoft.VSTS.Common.Priority", "System.ChangedDate",
            ])
            : [];

          let feedbacks = relatedItems.filter(isQaFeedbackWorkItem);
          if (state) feedbacks = feedbacks.filter((wi: any) => wi.fields?.["System.State"] === state);

          let out = `# QA Feedbacks associated with Work Item #${id} (${name})\n\n`;
          out += `${fmtWorkItemDetail(parent)}\n`;
          if (state) out += `Filter: state=${state}\n`;
          out += `Total QA Feedbacks: ${feedbacks.length}\n\n`;
          if (!feedbacks.length) return `${out}No QA Feedbacks found for this work item.\n`;

          out += "## Summary\n\n";
          for (const fb of feedbacks) out += fmtQaFeedback(fb) + "\n";

          out += "\n## Details\n";
          for (const fb of feedbacks) {
            const full = await ado.getWorkItem(fb.id, { expandRelations: true });
            out += "\n---\n\n";
            out += await formatWorkItemFullDetail(ado, full, `### QA Feedback #${fb.id}`);
          }
          return out;
        },
      },

      ado_qa_feedback_update: {
        description: "Update a QA Feedback work item. Supports changing state, priority, and adding an optional comment.",
        args: {
          id: z.number().describe("QA Feedback work item ID"),
          state: z.string().optional().describe("New state value (e.g. Active, Resolved, Closed)"),
          priority: z.number().optional().describe("New priority value"),
          comment: z.string().optional().describe("Optional comment to add with the update"),
          profile: z.string().optional().describe("Optional profile name override"),
        },
        async execute({ id, state, priority, comment, profile }: { id: number; state?: string; priority?: number; comment?: string; profile?: string }) {
          const { client: ado } = await createClient(profile);
          const patchOps: Array<{ op: string; path: string; value: any }> = [];
          if (state) patchOps.push({ op: "replace", path: "/fields/System.State", value: state });
          if (priority !== undefined) patchOps.push({ op: "replace", path: "/fields/Microsoft.VSTS.Common.Priority", value: priority });
          if (patchOps.length === 0 && !comment) return "No changes specified. Provide state, priority, or comment.";
          if (patchOps.length > 0) await ado.updateWorkItem(id, patchOps);
          if (comment) await ado.addWorkItemComment(id, comment);
          let out = `## QA Feedback Updated\n\nQA Feedback: #${id}\n`;
          if (state) out += `State: ${state}\n`;
          if (priority !== undefined) out += `Priority: ${priority}\n`;
          if (comment) out += `Comment added\n`;
          return out;
        },
      },

      ado_qa_feedback_comment: {
        description: "Add a comment to a QA Feedback work item.",
        args: {
          id: z.number().describe("QA Feedback work item ID"),
          comment: z.string().describe("Comment text"),
          profile: z.string().optional().describe("Optional profile name override"),
        },
        async execute({ id, comment, profile }: { id: number; comment: string; profile?: string }) {
          const { client: ado } = await createClient(profile);
          await ado.addWorkItemComment(id, comment);
          return `Comment added to QA Feedback #${id}.`;
        },
      },
    },
  };
};

const pluginModule: PluginModule & { id: string } = {
  id: "@nahuelcio/opencode-ado",
  server,
};

export default pluginModule;
export { server };
