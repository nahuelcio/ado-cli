/**
 * Persistent stores for the ADO plugin.
 *
 * Two stores at ~/.azure-devops-cli/:
 *   - active-profile  — currently active profile name
 *   - selected-pr     — currently selected PR (format: repo:prId)
 *
 * All operations are synchronous and defensive — they never throw.
 */

import { existsSync, readFileSync, writeFileSync, unlinkSync, mkdirSync } from "node:fs";
import { join } from "node:path";
import { homedir } from "node:os";

const STORE_DIR = join(homedir(), ".azure-devops-cli");
const PROFILE_FILE = join(STORE_DIR, "active-profile");
const SELECTED_PR_FILE = join(STORE_DIR, "selected-pr");

// ─── Active Profile ────────────────────────────────────────────────────────

/** Return the file path used for the active profile store. */
export function getActiveProfilePath(): string {
  return PROFILE_FILE;
}

/**
 * Read the active profile name from disk.
 * Returns null if the file is missing, empty, or unreadable (never throws).
 */
export function getActiveProfile(): string | null {
  try {
    if (!existsSync(PROFILE_FILE)) return null;
    const content = readFileSync(PROFILE_FILE, "utf-8").trim();
    return content || null;
  } catch {
    return null;
  }
}

/**
 * Persist the active profile name to disk.
 * Creates the store directory if it doesn't exist.
 */
export function setActiveProfile(name: string): void {
  mkdirSync(STORE_DIR, { recursive: true });
  writeFileSync(PROFILE_FILE, name, "utf-8");
}

/** Delete the active profile file from disk. Silently ignores errors. */
export function clearActiveProfile(): void {
  try {
    if (existsSync(PROFILE_FILE)) unlinkSync(PROFILE_FILE);
  } catch { /* ignore */ }
}

// ─── Selected PR ───────────────────────────────────────────────────────────

/**
 * Read the selected PR from disk.
 * Returns null if the file is missing, empty, or unreadable (never throws).
 */
export function getSelectedPr(): { repo: string; prId: number } | null {
  try {
    if (!existsSync(SELECTED_PR_FILE)) return null;
    const content = readFileSync(SELECTED_PR_FILE, "utf-8").trim();
    if (!content) return null;
    const [repo, idStr] = content.split(":");
    const prId = parseInt(idStr, 10);
    if (!repo || isNaN(prId)) return null;
    return { repo, prId };
  } catch {
    return null;
  }
}

/**
 * Persist the selected PR to disk.
 * Creates the store directory if it doesn't exist.
 */
export function setSelectedPr(repo: string, prId: number): void {
  try {
    mkdirSync(STORE_DIR, { recursive: true });
    writeFileSync(SELECTED_PR_FILE, `${repo}:${prId}`, "utf-8");
  } catch { /* defensive */ }
}

/** Clear the selected PR file. Silently ignores errors. */
export function clearSelectedPr(): void {
  try {
    if (existsSync(SELECTED_PR_FILE)) writeFileSync(SELECTED_PR_FILE, "", "utf-8");
  } catch { /* defensive */ }
}
