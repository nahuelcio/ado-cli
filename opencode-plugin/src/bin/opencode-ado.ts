#!/usr/bin/env node

/**
 * CLI entry point for @nahuelcio/opencode-ado
 *
 * Usage:
 *   npx @nahuelcio/opencode-ado init    — Interactive setup wizard
 *   npx @nahuelcio/opencode-ado show    — Show current config
 *   npx @nahuelcio/opencode-ado --help
 */

import { realpathSync, existsSync, readFileSync, writeFileSync, mkdirSync, chmodSync } from "node:fs";
import { resolve, dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { homedir, platform } from "node:os";
import { createInterface } from "node:readline";
import { execSync } from "node:child_process";

const PLUGIN_SPEC = "@nahuelcio/opencode-ado";
const SCHEMA_URL = "https://opencode.ai/config.json";

// ─── Colors ───────────────────────────────────────────────────────────────

function cyan(t: string) { return `\x1b[36m${t}\x1b[0m`; }
function green(t: string) { return `\x1b[32m${t}\x1b[0m`; }
function yellow(t: string) { return `\x1b[33m${t}\x1b[0m`; }
function bold(t: string) { return `\x1b[1m${t}\x1b[0m`; }

// ─── Prompt helpers ───────────────────────────────────────────────────────

function ask(prompt: string, defaultValue?: string): Promise<string> {
  const rl = createInterface({ input: process.stdin, output: process.stdout });
  return new Promise((resolve) => {
    const hint = defaultValue ? ` (${defaultValue})` : "";
    rl.question(`  ${cyan("❯")} ${prompt}${hint}: `, (answer) => {
      rl.close();
      resolve((answer.trim() || (defaultValue ?? "")).trim());
    });
  });
}

function yesNo(prompt: string, def = true): Promise<boolean> {
  const rl = createInterface({ input: process.stdin, output: process.stdout });
  return new Promise((resolve) => {
    const hint = def ? "Y/n" : "y/N";
    rl.question(`  ${cyan("❯")} ${prompt} (${hint}): `, (answer) => {
      rl.close();
      const a = answer.trim().toLowerCase();
      resolve(a === "" ? def : a === "y" || a === "yes");
    });
  });
}

// ─── Config paths ─────────────────────────────────────────────────────────

function getOpenCodeConfigDir(): string {
  // 1. XDG_CONFIG_HOME
  const xdg = process.env.XDG_CONFIG_HOME;
  if (xdg) return join(xdg, "opencode");

  // 2. Platform default
  if (platform() === "win32") {
    const appData = process.env.APPDATA;
    if (appData) return join(appData, "opencode");
  }

  // 3. ~/.config/opencode
  return join(homedir(), ".config", "opencode");
}

function getAdoCredentialsDir(): string {
  return join(homedir(), ".azure-devops-cli");
}

function getAdoCredentialsPath(): string {
  return join(getAdoCredentialsDir(), "pat");
}

// ─── Config read/write ────────────────────────────────────────────────────

function findConfigFile(dir: string): string | null {
  for (const name of ["opencode.jsonc", "opencode.json"]) {
    const p = join(dir, name);
    if (existsSync(p)) return p;
  }
  return null;
}

function readConfig(path: string): Record<string, unknown> {
  if (!existsSync(path)) return {};
  const raw = readFileSync(path, "utf-8");
  const stripped = raw.replace(/\/\/.*$/gm, "").replace(/\/\*[\s\S]*?\*\//g, "");
  return JSON.parse(stripped);
}

function writeConfig(path: string, data: Record<string, unknown>): void {
  const dir = dirname(path);
  if (!existsSync(dir)) mkdirSync(dir, { recursive: true });
  if (!data["$schema"]) data["$schema"] = SCHEMA_URL;
  writeFileSync(path, JSON.stringify(data, null, 2) + "\n", "utf-8");
}

// ─── PAT storage ──────────────────────────────────────────────────────────

function storePAT(pat: string): void {
  const dir = getAdoCredentialsDir();
  if (!existsSync(dir)) mkdirSync(dir, { recursive: true });

  const path = getAdoCredentialsPath();
  writeFileSync(path, pat.trim(), "utf-8");

  // Restrict permissions (Unix only — Windows uses ACL)
  if (platform() !== "win32") {
    try { chmodSync(path, 0o600); } catch { /* best effort */ }
  }

  // Also try setx on Windows for persistent env var
  if (platform() === "win32") {
    try {
      execSync(`setx AZURE_DEVOPS_PAT "${pat.trim()}"`, { stdio: "pipe" });
      console.log(`  ${green("✓")} Set AZURE_DEVOPS_PAT via setx (restart terminal to apply)`);
    } catch {
      // setx not available or failed — file-based fallback still works
    }
  }
}

function loadStoredPAT(): string | null {
  try {
    const path = getAdoCredentialsPath();
    if (existsSync(path)) return readFileSync(path, "utf-8").trim();
  } catch { /* ignore */ }
  return null;
}

// ─── Init Command ─────────────────────────────────────────────────────────

interface ProfileConfig {
  org: string;
  project: string;
  patEnvVar: string;
  repos: string[];
  default?: boolean;
}

async function runInit(_cwd: string): Promise<number> {
  console.log();
  console.log(bold("  Azure DevOps Plugin for OpenCode"));
  console.log("  ─────────────────────────────────────────");
  console.log();

  // ── Step 1: Organization ──────────────────────────────────────────
  console.log(`  ${bold("Organization")}`);
  console.log("  Your Azure DevOps org URL or name.");
  console.log("  Examples: yoizen, https://dev.azure.com/yoizen, https://yoizen.visualstudio.com");
  const org = await ask("Organization URL or name");
  if (!org) { console.log(yellow("  ✗ Organization is required")); return 1; }

  // ── Step 2: PAT ───────────────────────────────────────────────────
  console.log();
  console.log(`  ${bold("Personal Access Token (PAT)")}`);

  const existingPAT = process.env.AZURE_DEVOPS_PAT ?? loadStoredPAT();
  if (existingPAT) {
    console.log(`  ${green("✓")} Found existing PAT — will reuse it`);
  } else {
    console.log("  Your PAT is stored securely in ~/.azure-devops-cli/pat (never in opencode.json)");
    const pat = await ask("Enter your PAT");
    if (!pat) { console.log(yellow("  ✗ PAT is required")); return 1; }
    storePAT(pat);
    console.log(`  ${green("✓")} PAT saved to ~/.azure-devops-cli/pat`);
  }

  const patEnvVar = "AZURE_DEVOPS_PAT";

  // ── Step 3: Profiles (loop) ───────────────────────────────────────
  const profiles: Record<string, ProfileConfig> = {};
  let isFirstProfile = true;
  let defaultProfileName = "";

  while (true) {
    console.log();
    if (isFirstProfile) {
      console.log(`  ${bold("── Profile ──")}`);
    } else {
      console.log(`  ${bold("── Add another profile ──")}`);
    }

    // Project
    const project = await ask("Project name");
    if (!project) {
      if (isFirstProfile) { console.log(yellow("  ✗ Project name is required")); return 1; }
      break; // No more profiles
    }

    // Repos
    console.log();
    console.log("  Comma-separated repos to monitor for PRs.");
    console.log("  Example: web-api,web-executor,frontend");
    const reposStr = await ask("Repositories");
    const repos = reposStr.split(",").map((s) => s.trim()).filter(Boolean);
    if (repos.length === 0) {
      console.log(yellow("  ✗ At least one repo is required"));
      continue;
    }

    // Default?
    const isDefault = isFirstProfile || await yesNo("Set as default profile?", false);

    const profileName = project.toLowerCase().replace(/[^a-z0-9-]/g, "-");

    profiles[profileName] = {
      org,
      project,
      patEnvVar,
      repos,
      ...(isDefault ? { default: true } : {}),
    };

    if (isDefault) defaultProfileName = profileName;

    console.log(`  ${green("✓")} Profile "${profileName}" added (project: ${project}, repos: ${repos.join(", ")})`);

    // Ask if more
    const addMore = await yesNo("Add another project for this organization?", false);
    if (!addMore) break;
    isFirstProfile = false;
  }

  if (Object.keys(profiles).length === 0) {
    console.log(yellow("  No profiles configured. Exiting."));
    return 1;
  }

  // ── Step 4: Summary ──────────────────────────────────────────────
  console.log();
  console.log(bold("  ── Summary ──"));
  console.log(`  Organization: ${green(org)}`);
  console.log(`  PAT:          ${green("stored in ~/.azure-devops-cli/pat")}`);
  console.log();
  for (const [name, p] of Object.entries(profiles)) {
    const marker = p.default ? green(" (default)") : "";
    console.log(`  Profile: ${bold(name)}${marker}`);
    console.log(`    project: ${p.project}`);
    console.log(`    repos:   ${p.repos.join(", ")}`);
  }
  console.log();

  const ok = await yesNo("Apply this configuration?");
  if (!ok) { console.log(yellow("  Cancelled")); return 0; }

  // ── Step 5: Write config ──────────────────────────────────────────
  const configDir = getOpenCodeConfigDir();
  const configPath = findConfigFile(configDir) ?? join(configDir, "opencode.json");
  const config = readConfig(configPath);

  // Ensure plugin array
  if (!Array.isArray(config["plugin"])) config["plugin"] = [];
  const plugins = config["plugin"] as (string | unknown[])[];

  const alreadyHas = plugins.some((p) => {
    if (typeof p === "string") return p === PLUGIN_SPEC || p.startsWith(PLUGIN_SPEC + "@");
    if (Array.isArray(p)) return p[0] === PLUGIN_SPEC;
    return false;
  });
  if (!alreadyHas) plugins.push(PLUGIN_SPEC);

  // Build ado config — merge with existing profiles
  if (!config["ado"] || typeof config["ado"] !== "object") config["ado"] = {};
  const ado = config["ado"] as Record<string, unknown>;

  if (!ado["profiles"] || typeof ado["profiles"] !== "object") ado["profiles"] = {};
  const existingProfiles = ado["profiles"] as Record<string, ProfileConfig>;

  for (const [name, p] of Object.entries(profiles)) {
    existingProfiles[name] = p;
  }

  if (defaultProfileName) ado["defaultProfile"] = defaultProfileName;

  writeConfig(configPath, config);
  console.log();
  console.log(`  ${green("✓")} Plugin added to config`);
  console.log(`  ${green("✓")} Wrote ${configPath}`);

  // ── Step 6: Next steps ────────────────────────────────────────────
  console.log();
  console.log(bold("  Done! Restart OpenCode to activate the plugin."));
  console.log();
  console.log(`  The LLM can now use: ${cyan("ado_prs")}, ${cyan("ado_pr <repo> <id>")}, ${cyan("ado_review <repo> <id> approve")}`);
  console.log(`  The sidebar will show PRs pending your review.`);
  if (platform() === "win32") {
    console.log();
    console.log(yellow("  ⚠ Restart your terminal for the AZURE_DEVOPS_PAT env var to take effect."));
  }
  console.log();

  return 0;
}

// ─── Show Command ─────────────────────────────────────────────────────────

async function runShow(): Promise<number> {
  const configDir = getOpenCodeConfigDir();
  const configPath = findConfigFile(configDir);
  if (!configPath) {
    console.log(yellow("  No opencode.json found in " + configDir));
    return 1;
  }

  const config = readConfig(configPath);
  const ado = config["ado"] as Record<string, unknown> | undefined;
  if (!ado?.["profiles"]) {
    console.log(yellow("  No ado.profiles configured"));
    return 1;
  }

  const profiles = ado["profiles"] as Record<string, ProfileConfig>;
  const defaultName = ado["defaultProfile"] as string | undefined;

  console.log();
  console.log(`  Config: ${configPath}`);
  console.log();
  for (const [name, p] of Object.entries(profiles)) {
    const marker = (name === defaultName || p.default) ? green(" (default)") : "";
    console.log(`  ${bold(name)}${marker}`);
    console.log(`    org:     ${p.org}`);
    console.log(`    project: ${p.project}`);
    console.log(`    repos:   ${p.repos.join(", ")}`);
    console.log();
  }

  // Check PAT
  const pat = process.env.AZURE_DEVOPS_PAT ?? loadStoredPAT();
  if (pat) {
    console.log(green("  ✓ PAT available"));
  } else {
    console.log(yellow("  ⚠ No PAT found"));
    console.log("    Run: npx @nahuelcio/opencode-ado init");
  }
  console.log();

  return 0;
}

// ─── Main ─────────────────────────────────────────────────────────────────

const USAGE = [
  "",
  bold("  @nahuelcio/opencode-ado"),
  "  Azure DevOps plugin for OpenCode",
  "",
  "  Usage:",
  `    ${cyan("npx @nahuelcio/opencode-ado init")}    Interactive setup wizard`,
  `    ${cyan("npx @nahuelcio/opencode-ado show")}    Show current config`,
  `    ${cyan("npx @nahuelcio/opencode-ado --help")}  Show this help`,
  "",
].join("\n");

export async function main(argv = process.argv.slice(2)): Promise<number> {
  const [command] = argv;
  if (!command || command === "--help" || command === "-h") {
    console.log(USAGE);
    return 0;
  }
  if (command === "init") return runInit(process.cwd());
  if (command === "show") return runShow();
  console.log(`Unknown command: ${command}`);
  console.log(USAGE);
  return 1;
}

const __filename = fileURLToPath(import.meta.url);
const __argv1 = process.argv[1];
try {
  if (__argv1 && realpathSync.native(__filename) === realpathSync.native(__argv1)) {
    void main().then((code) => { process.exitCode = code; });
  }
} catch { /* not executed directly */ }
