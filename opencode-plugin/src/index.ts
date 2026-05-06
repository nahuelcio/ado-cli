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
} from "./shared.js";
import { getActiveProfile, setActiveProfile, getSelectedPr, setSelectedPr, clearSelectedPr } from "./profile-store.js";

// ─── ADO HTTP Client (minimal, inline) ────────────────────────────────────

const API_VERSION = "7.1-preview.1";

class AdoClient {
  private authHeader: string;

  constructor(
    private orgUrl: string,
    private project: string,
    pat: string,
  ) {
    this.authHeader = "Basic " + Buffer.from(":" + pat).toString("base64");
  }

  private buildUrl(endpoint: string, scope: "org" | "project" = "project"): string {
    if (endpoint.startsWith("http://") || endpoint.startsWith("https://")) {
      const url = new URL(endpoint);
      url.searchParams.set("api-version", API_VERSION);
      return url.toString();
    }

    const root = scope === "org"
      ? this.orgUrl
      : `${this.orgUrl}/${encodeURIComponent(this.project)}`;
    const apiPath = endpoint.startsWith("/_apis/")
      ? endpoint
      : `/_apis${endpoint.startsWith("/") ? endpoint : `/${endpoint}`}`;
    const url = new URL(root + apiPath);
    url.searchParams.set("api-version", API_VERSION);
    return url.toString();
  }

  private async request<T>(
    endpoint: string,
    init?: RequestInit,
    scope: "org" | "project" = "project",
  ): Promise<T> {
    const url = this.buildUrl(endpoint, scope);
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
      throw new Error(`ADO API ${res.status}: ${body.slice(0, 200)}`);
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
      throw new Error(`ADO API ${res.status}: ${body.slice(0, 200)}`);
    }
    return res.text();
  }

  async getUserIdentity(): Promise<{ id: string; displayName: string }> {
    const data = await this.request<{ authenticatedUser: { id: string; displayName: string } }>(
      "/connectionData",
      undefined,
      "org",
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

  async createThread(repo: string, prId: number, comment: string) {
    return this.request<any>(
      `/git/repositories/${encodeURIComponent(repo)}/pullrequests/${prId}/threads`,
      {
        method: "POST",
        body: JSON.stringify({
          comments: [{ content: comment, commentType: "text" }],
          status: "active",
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
    },
  };
};

const pluginModule: PluginModule & { id: string } = {
  id: "@nahuelcio/opencode-ado",
  server,
};

export default pluginModule;
export { server };
