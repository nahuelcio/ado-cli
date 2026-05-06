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
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { homedir } from "node:os";

// ─── Config helpers (inline to avoid circular deps in module resolution) ──

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

function asAdoConfig(value: unknown): AdoConfig | undefined {
  if (!value || typeof value !== "object") return undefined;
  const maybe = value as { ado?: unknown; profiles?: unknown };
  if (maybe.profiles && typeof maybe.profiles === "object") return maybe as AdoConfig;
  if (maybe.ado && typeof maybe.ado === "object") return asAdoConfig(maybe.ado);
  return undefined;
}

function resolveActiveProfile(config: AdoConfig): { name: string; profile: AdoProfile } {
  // 1. Explicit defaultProfile name
  if (config.defaultProfile && config.profiles[config.defaultProfile]) {
    return { name: config.defaultProfile, profile: config.profiles[config.defaultProfile] };
  }
  // 2. Profile marked default:true
  for (const [name, profile] of Object.entries(config.profiles)) {
    if (profile.default) return { name, profile };
  }
  // 3. First profile
  const entries = Object.entries(config.profiles);
  if (entries.length === 0) throw new Error("No ADO profiles configured in opencode.json under 'ado.profiles'");
  const [name, profile] = entries[0];
  return { name, profile };
}

function resolveOrgUrl(org: string): string {
  if (org.startsWith("http://") || org.startsWith("https://")) return org.replace(/\/$/, "");
  return `https://dev.azure.com/${org}`;
}

function getPAT(envVarName: string): string {
  // 1. Try env var
  const fromEnv = process.env[envVarName];
  if (fromEnv) return fromEnv;

  // 2. Fallback: ~/.azure-devops-cli/pat (set by npx init)
  try {
    const patPath = join(homedir(), ".azure-devops-cli", "pat");
    if (existsSync(patPath)) {
      const pat = readFileSync(patPath, "utf-8").trim();
      if (pat) return pat;
    }
  } catch { /* ignore */ }

  throw new Error(
    `PAT not found. Either set env var ${envVarName} or run: npx @nahuelcio/opencode-ado init`,
  );
}

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
}

// ─── Formatting helpers ───────────────────────────────────────────────────

function shortBranch(ref?: string): string {
  if (!ref) return "?";
  return ref.replace("refs/heads/", "").replace("refs/tags/", "");
}

function fmtPR(pr: any): string {
  const repo = pr.repository?.name ?? "?";
  const src = shortBranch(pr.sourceRefName);
  const tgt = shortBranch(pr.targetRefName);
  const author = pr.createdBy?.displayName ?? "?";
  const draft = pr.isDraft ? " [DRAFT]" : "";
  return `- #${pr.pullRequestId} ${pr.title}${draft}\n  ${repo}: ${src} → ${tgt} by ${author}`;
}

function fmtPRDetail(pr: any): string {
  let out = `id: ${pr.pullRequestId}\ntitle: ${pr.title}\nstatus: ${pr.status}\n`;
  out += `source: ${shortBranch(pr.sourceRefName)}\ntarget: ${shortBranch(pr.targetRefName)}\n`;
  if (pr.createdBy) out += `author: ${pr.createdBy.displayName}\n`;
  if (pr.isDraft) out += `draft: true\n`;
  if (pr.description) out += `\ndescription: |\n  ${pr.description.slice(0, 500)}\n`;
  if (pr.reviewers?.length) {
    out += `\nreviewers:\n`;
    for (const r of pr.reviewers) {
      const vote = r.vote === 10 ? "✓" : r.vote === -10 ? "✗" : r.vote === -5 ? "⏳" : "—";
      out += `  ${vote} ${r.votedBy?.displayName ?? "?"}\n`;
    }
  }
  return out;
}

function fmtThread(t: any): string {
  const file = t.threadContext?.filePath ?? "";
  const status = t.status ?? "?";
  const firstComment = t.comments?.[0]?.content ?? "";
  return `- [${status}] ${file ? file + ": " : ""}${firstComment.slice(0, 120)}\n  (${t.comments?.length ?? 0} comments)`;
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

  async function createClient() {
    const config = await loadConfig();
    const { name, profile } = resolveActiveProfile(config);
    const pat = getPAT(profile.patEnvVar);
    const orgUrl = resolveOrgUrl(profile.org);
    const ado = new AdoClient(orgUrl, profile.project, pat);
    const userId = await ado.getUserIdentity();
    return { client: ado, profile, name, userId };
  }

  // ─── Tools ───────────────────────────────────────────────────────

  return {
    tool: {
      ado_prs: {
        description: "List active pull requests from configured Azure DevOps repos. Shows pending reviews and your own PRs.",
        args: {},
        async execute(_args: {}, ctx) {
          const { client: ado, profile, name, userId } = await createClient();
          const allPRs: any[] = [];
          for (const repo of profile.repos) {
            try {
              const prs = await ado.listPullRequests(repo, { status: "active" });
              allPRs.push(...prs);
            } catch { /* skip failing repos */ }
          }

          const pending = allPRs.filter(pr => pr.reviewers?.some((r: any) => r.votedBy?.id === userId.id && r.vote === 0));
          const mine = allPRs.filter(pr => pr.createdBy?.id === userId.id);

          let out = `## Azure DevOps — Active PRs (${name})\n\n`;
          if (pending.length) { out += `### Pending Your Review (${pending.length})\n\n${pending.map(fmtPR).join("\n")}\n\n`; }
          if (mine.length) { out += `### Your PRs (${mine.length})\n\n${mine.map(fmtPR).join("\n")}\n\n`; }
          if (!pending.length && !mine.length) out += "No active PRs requiring your attention.\n";
          out += `\n---\nTotal active: ${allPRs.length} across ${profile.repos.length} repos`;
          return out;
        },
      },

      ado_pr: {
        description: "Show details for a specific Azure DevOps pull request.",
        args: { repo: z.string().describe("Repository name"), prId: z.number().describe("Pull request ID") },
        async execute({ repo, prId }: { repo: string; prId: number }) {
          const { client: ado, name } = await createClient();
          const pr = await ado.getPullRequest(repo, prId);
          return `## PR #${prId} — ${name}\n\n${fmtPRDetail(pr)}`;
        },
      },

      ado_pr_threads: {
        description: "Show comment threads for an Azure DevOps pull request.",
        args: { repo: z.string().describe("Repository name"), prId: z.number().describe("Pull request ID") },
        async execute({ repo, prId }: { repo: string; prId: number }) {
          const { client: ado } = await createClient();
          const threads = await ado.getThreads(repo, prId);
          if (!threads.length) return `No threads found for PR #${prId}.`;
          return `## Threads for PR #${prId}\n\n${threads.map(fmtThread).join("\n")}`;
        },
      },

      ado_review: {
        description: "Vote on an Azure DevOps pull request with an optional comment.",
        args: {
          repo: z.string().describe("Repository name"),
          prId: z.number().describe("Pull request ID"),
          vote: z.enum(["approve", "reject", "wait", "suggestions"]).describe("Vote: approve, reject, wait, or suggestions"),
          comment: z.string().optional().describe("Optional review comment"),
        },
        async execute({ repo, prId, vote: voteStr, comment }: { repo: string; prId: number; vote: string; comment?: string }) {
          const { client: ado, userId } = await createClient();
          const voteMap: Record<string, number> = { approve: 10, suggestions: 5, wait: -5, reject: -10 };
          const voteValue = voteMap[voteStr];
          if (voteValue === undefined) return `Invalid vote: ${voteStr}. Use: approve, reject, wait, suggestions`;

          await ado.voteReviewer(repo, prId, userId.id, voteValue);
          if (comment) await ado.createThread(repo, prId, comment);

          const labels: Record<number, string> = { 10: "Approved", 5: "Approved with suggestions", "-5": "Waiting for author", "-10": "Rejected" };
          let out = `## Review Submitted\n\nPR: #${prId} in ${repo}\nVote: ${labels[voteValue]}\n`;
          if (comment) out += `Comment: ${comment}\n`;
          return out;
        },
      },

      ado_profile: {
        description: "Show the active Azure DevOps profile configuration.",
        args: {},
        async execute() {
          const { profile, name } = await createClient();
          return [
            `## Azure DevOps Profile: ${name}`,
            ``,
            `org: ${profile.org}`,
            `project: ${profile.project}`,
            `repos: ${profile.repos.join(", ")}`,
            `patEnvVar: ${profile.patEnvVar}`,
          ].join("\n");
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
