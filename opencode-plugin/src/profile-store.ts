/**
 * Persistent stores for the ADO plugin.
 *
 * Four stores at ~/.azure-devops-cli/:
 *   - active-profile  — currently active profile name
 *   - selected-pr     — currently selected PR (format: repo:prId)
 *   - selected-wi     — currently selected WI (format: profileName:wiId)
 *   - sidebar-view    — current sidebar view mode (prs | wis | qa)
 *
 * All operations are synchronous and defensive — they never throw.
 */

import { existsSync, readFileSync, writeFileSync, unlinkSync, mkdirSync } from "node:fs";
import { join } from "node:path";
import { homedir } from "node:os";

const STORE_DIR = join(homedir(), ".azure-devops-cli");
const PROFILE_FILE = join(STORE_DIR, "active-profile");
const SELECTED_PR_FILE = join(STORE_DIR, "selected-pr");
const SELECTED_WI_FILE = join(STORE_DIR, "selected-wi");
const SIDEBAR_VIEW_FILE = join(STORE_DIR, "sidebar-view");

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

// ─── Selected WI ────────────────────────────────────────────────────────────

/**
 * Read the selected WI from disk.
 * Returns null if the file is missing, empty, or unreadable (never throws).
 */
export function getSelectedWi(): { profileName: string; wiId: number } | null {
  try {
    if (!existsSync(SELECTED_WI_FILE)) return null;
    const content = readFileSync(SELECTED_WI_FILE, "utf-8").trim();
    if (!content) return null;
    const [profileName, idStr] = content.split(":");
    const wiId = parseInt(idStr, 10);
    if (!profileName || isNaN(wiId)) return null;
    return { profileName, wiId };
  } catch {
    return null;
  }
}

/**
 * Persist the selected WI to disk.
 * Creates the store directory if it doesn't exist.
 */
export function setSelectedWi(profileName: string, wiId: number): void {
  try {
    mkdirSync(STORE_DIR, { recursive: true });
    writeFileSync(SELECTED_WI_FILE, `${profileName}:${wiId}`, "utf-8");
  } catch { /* defensive */ }
}

/** Clear the selected WI file. Silently ignores errors. */
export function clearSelectedWi(): void {
  try {
    if (existsSync(SELECTED_WI_FILE)) writeFileSync(SELECTED_WI_FILE, "", "utf-8");
  } catch { /* defensive */ }
}

// ─── Sidebar View ───────────────────────────────────────────────────────────

/**
 * Read the current sidebar view mode from disk.
 * Returns "prs" if the file is missing, empty, or unreadable (never throws).
 */
export function getViewMode(): "prs" | "wis" | "qa" {
  try {
    if (!existsSync(SIDEBAR_VIEW_FILE)) return "prs";
    const content = readFileSync(SIDEBAR_VIEW_FILE, "utf-8").trim();
    if (!content) return "prs";
    if (content === "wis" || content === "qa") return content;
    return "prs";
  } catch {
    return "prs";
  }
}

/**
 * Persist the sidebar view mode to disk.
 * Creates the store directory if it doesn't exist.
 */
export function setViewMode(view: string): void {
  try {
    mkdirSync(STORE_DIR, { recursive: true });
    writeFileSync(SIDEBAR_VIEW_FILE, view, "utf-8");
  } catch { /* defensive */ }
}

// ─── Collapsed States ────────────────────────────────────────────────────────

const COLLAPSED_STATES_FILE = join(STORE_DIR, "collapsed-states");

/**
 * Read the collapsed states from disk.
 * Returns {} if the file is missing, empty, or unreadable (never throws).
 * A state NOT in the record defaults to collapsed (true).
 */
export function getCollapsedStates(): Record<string, boolean> {
  try {
    if (!existsSync(COLLAPSED_STATES_FILE)) return {};
    const content = readFileSync(COLLAPSED_STATES_FILE, "utf-8").trim();
    if (!content) return {};
    return JSON.parse(content) as Record<string, boolean>;
  } catch {
    return {};
  }
}

/**
 * Persist the collapsed states to disk.
 * Creates the store directory if it doesn't exist.
 */
export function setCollapsedStates(states: Record<string, boolean>): void {
  try {
    mkdirSync(STORE_DIR, { recursive: true });
    writeFileSync(COLLAPSED_STATES_FILE, JSON.stringify(states), "utf-8");
  } catch { /* defensive */ }
}
