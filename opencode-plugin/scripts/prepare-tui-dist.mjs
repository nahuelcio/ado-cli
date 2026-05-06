import { copyFileSync, mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const source = join(root, "src", "tui.tsx");
const target = join(root, "dist", "tui.tsx");

mkdirSync(dirname(target), { recursive: true });
copyFileSync(source, target);
