/**
 * Shared types, config helpers, and formatting utilities for the ADO plugin.
 *
 * Used by both the server module (index.ts) and the TUI module (tui.tsx).
 * Import from here instead of duplicating definitions.
 */

import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { homedir } from "node:os";

// ─── Types ────────────────────────────────────────────────────────────────

export interface AdoProfile {
  org: string;
  project: string;
  patEnvVar: string;
  repos: string[];
  default?: boolean;
}

export interface AdoConfig {
  defaultProfile?: string;
  profiles: Record<string, AdoProfile>;
}

export interface PRSummary {
  id: number;
  title: string;
  repo: string;
  source: string;
  target: string;
  author: string;
  isDraft: boolean;
}

// ─── Config helpers ───────────────────────────────────────────────────────

/** Coerce an unknown plugin config value into an AdoConfig, or return undefined. */
export function asAdoConfig(value: unknown): AdoConfig | undefined {
  if (!value || typeof value !== "object") return undefined;
  const maybe = value as { ado?: unknown; profiles?: unknown };
  if (maybe.profiles && typeof maybe.profiles === "object") return maybe as AdoConfig;
  if (maybe.ado && typeof maybe.ado === "object") return asAdoConfig(maybe.ado);
  return undefined;
}

/** Resolve the active profile from config using defaultProfile > default:true > first. */
export function resolveActiveProfile(config: AdoConfig): { name: string; profile: AdoProfile } {
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

/** Build the organisation URL from a short name or full URL. */
export function resolveOrgUrl(org: string): string {
  if (org.startsWith("http://") || org.startsWith("https://")) return org.replace(/\/$/, "");
  return `https://dev.azure.com/${org}`;
}

// ─── PAT helpers ──────────────────────────────────────────────────────────

/**
 * Get the PAT for the given env var name. Throws if not found.
 * Used by the server module which requires a PAT to function.
 */
export function getPAT(envVarName: string): string {
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

/**
 * Get the PAT for the given env var name. Returns undefined if not found.
 * Used by the TUI module which handles missing PATs gracefully.
 */
export function getPATOptional(envVarName: string): string | undefined {
  const fromEnv = process.env[envVarName];
  if (fromEnv) return fromEnv;
  try {
    const patPath = join(homedir(), ".azure-devops-cli", "pat");
    if (existsSync(patPath)) return readFileSync(patPath, "utf-8").trim() || undefined;
  } catch { /* ignore */ }
  return undefined;
}

// ─── Formatting helpers ──────────────────────────────────────────────────

/** Strip refs/heads/ or refs/tags/ prefix from a ref string. */
export function shortBranch(ref?: string): string {
  if (!ref) return "?";
  return ref.replace("refs/heads/", "").replace("refs/tags/", "");
}

/** Check if a reviewer object matches the given user ID. Handles undefined userId gracefully. */
export function reviewerMatchesUser(reviewer: any, userId: string | undefined): boolean {
  if (!userId) return false;
  return reviewer?.id === userId
    || reviewer?.votedBy?.id === userId
    || reviewer?.uniqueName === userId;
}

/** Format a PR as a single-line list entry. */
export function fmtPR(pr: any): string {
  const repo = pr.repository?.name ?? "?";
  const src = shortBranch(pr.sourceRefName);
  const tgt = shortBranch(pr.targetRefName);
  const author = pr.createdBy?.displayName ?? "?";
  const draft = pr.isDraft ? " [DRAFT]" : "";
  return `- #${pr.pullRequestId} ${pr.title}${draft}\n  ${repo}: ${src} → ${tgt} by ${author}`;
}

/** Format a PR as a detailed multi-line block. */
export function fmtPRDetail(pr: any): string {
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

/** Format a thread as a concise summary line. */
export function fmtThread(t: any): string {
  const file = t.threadContext?.filePath ?? "";
  const status = t.status ?? "?";
  const firstComment = t.comments?.[0]?.content ?? "";
  return `- [${status}] ${file ? file + ": " : ""}${firstComment.slice(0, 120)}\n  (${t.comments?.length ?? 0} comments)`;
}
