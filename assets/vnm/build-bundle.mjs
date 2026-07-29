#!/usr/bin/env node
// Regenerates the vendored browser bundle `vnm-browser.js` from an installed
// `very-nice-mermaid`. DEV-TIME ONLY - gogo users never run this; they get the
// committed bundle. Re-run after bumping very-nice-mermaid:
//
//   npm i -g very-nice-mermaid && node assets/vnm/build-bundle.mjs
//
// Why a transform at all: very-nice-mermaid ships ESM (`dist/index.js`), but a
// gogo viewer page opens over `file://`, where the browser blocks BOTH ES-module
// `src` loading and `fetch()`. A classic <script src> is the only thing that
// loads there. The bundle is already self-contained (dagre inlined; the only
// dynamic imports are the mermaid/jsdom fallback tier, which gogo never hits -
// its five diagram kinds are all native), so the sole change needed is swapping
// the single trailing `export { … }` for a `window.vnm = { … }` assignment.

import { execSync } from "node:child_process";
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const HERE = dirname(fileURLToPath(import.meta.url));
const OUT = join(HERE, "vnm-browser.js");

/** Locate `very-nice-mermaid`'s ESM entry: local resolve, then npm roots. */
function findEntry() {
  const candidates = [];
  for (const cmd of ["npm root", "npm root -g"]) {
    try {
      const root = execSync(cmd, { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] }).trim();
      if (root) candidates.push(join(root, "very-nice-mermaid", "dist", "index.js"));
    } catch {}
  }
  return candidates.find(existsSync) ?? null;
}

const entry = findEntry();
if (!entry) {
  console.error("build-bundle: very-nice-mermaid not found. Install it first:\n  npm i -g very-nice-mermaid");
  process.exit(1);
}

const pkg = JSON.parse(readFileSync(join(entry, "..", "..", "package.json"), "utf8"));
const src = readFileSync(entry, "utf8");

// The bundle ends with exactly one top-level `export { a, b as c, … };`.
const marks = [...src.matchAll(/^export\s*\{/gm)];
if (marks.length !== 1) {
  console.error(`build-bundle: expected 1 top-level export statement, found ${marks.length}. ` +
    "very-nice-mermaid's bundle shape changed - re-check this transform.");
  process.exit(1);
}
const start = marks[0].index;
const end = src.indexOf("};", start) + 2;

const pairs = src
  .slice(start, end)
  .replace(/^export\s*\{/, "")
  .replace(/\};$/, "")
  .split(",")
  .map((s) => s.trim())
  .filter(Boolean)
  .map((n) => {
    const [local, exported] = n.includes(" as ") ? n.split(" as ").map((x) => x.trim()) : [n, n];
    return `  ${exported}: ${local}`;
  });

const banner = `/* GENERATED FILE - DO NOT EDIT.
 * very-nice-mermaid v${pkg.version} (${pkg.license}) - https://github.com/ZawadzkiB/very-nice-mermaid
 * Rebuilt by assets/vnm/build-bundle.mjs from dist/index.js: the single trailing
 * ESM \`export { … }\` is replaced by \`window.vnm = { … }\` so the file loads as a
 * classic <script src> over file://, where ES-module src and fetch are blocked.
 */
`;

writeFileSync(OUT, banner + src.slice(0, start) + "window.vnm = {\n" + pairs.join(",\n") + "\n};\n");
console.log(`build-bundle: wrote vnm-browser.js - very-nice-mermaid v${pkg.version}, ${pairs.length} exports on window.vnm`);
