#!/usr/bin/env node
// Pre-computes very-nice-mermaid layouts for a set of .mmd files so the offline
// viewer can mount EVERY diagram kind interactively.
//
//   node assets/vnm/layout.mjs <out.json> <file.mmd>...
//
// Why this exists (and why the browser can't do it): mounting a sequence /
// class / state diagram from raw DSL needs mermaid's `detectType` router, i.e.
// a dynamic `import("mermaid")`. A gogo page opens over `file://`, where every
// module fetch is blocked - so in-browser routing silently degrades those kinds
// to a garbage flowchart parse. Doing the parse+layout HERE (Node, at authoring
// time, where mermaid resolves) and shipping the positioned model as JSON keeps
// the viewer dependency-free and correct for all five gogo kinds.
//
// Exit codes: 0 = wrote layouts (possibly partial), 3 = very-nice-mermaid not
// installed (caller degrades gracefully - see gogo-mermaid SKILL.md), 2 = usage.
// Per-file parse failures are reported on stderr and skipped, never fatal: an
// invalid diagram must not sink the whole bundle.

import { execSync } from "node:child_process";
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { basename, join } from "node:path";

const [out, ...files] = process.argv.slice(2);
if (!out || files.length === 0) {
  console.error("usage: node layout.mjs <out.json> <file.mmd>...");
  process.exit(2);
}

/** Resolve very-nice-mermaid: bare import first, then local/global npm roots. */
async function loadVnm() {
  try {
    return await import("very-nice-mermaid");
  } catch {}
  for (const cmd of ["npm root", "npm root -g"]) {
    try {
      const root = execSync(cmd, { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] }).trim();
      const p = join(root, "very-nice-mermaid", "dist", "index.js");
      if (existsSync(p)) return await import(p);
    } catch {}
  }
  return null;
}

const vnm = await loadVnm();
if (!vnm) {
  console.error("layout: very-nice-mermaid not installed - skipping prebuilt layouts (viewer degrades to inline DSL / SVG)");
  process.exit(3);
}

const { readStateModel, layoutState, readSequenceModel, layoutSequence,
        readClassModel, layoutClass, parse, layout, resolveTheme, serializeModel } = vnm;

const THEME = process.env.GOGO_VNM_THEME || "arch-light";

// gogo diagram sources escape angle brackets (`A["/gogo:done &lt;slug&gt;"]`)
// because the same DSL also ships as a ```mermaid fence, and mermaid draws
// labels as HTML. very-nice-mermaid draws SVG <text>, which would show the
// entity literally - so decode here to keep both renderings identical. Ampersand
// LAST so an escaped `&amp;lt;` survives as the literal `&lt;`.
function decodeEntities(s) {
  return s.replace(/&lt;/g, "<").replace(/&gt;/g, ">").replace(/&amp;/g, "&");
}
const result = {};
let ok = 0, failed = 0;

for (const file of files) {
  const stem = basename(file).replace(/\.mmd$/, "");
  try {
    const dsl = decodeEntities(readFileSync(file, "utf8"));
    const head = dsl.trim().split("\n")[0].trim();
    let built;
    if (/^stateDiagram/.test(head)) built = layoutState(await readStateModel(dsl));
    else if (/^sequenceDiagram/.test(head)) built = layoutSequence(await readSequenceModel(dsl));
    else if (/^classDiagram/.test(head)) built = layoutClass(await readClassModel(dsl));
    else {
      const parsed = parse(dsl);
      built = layout(parsed.model ?? parsed, { theme: resolveTheme(THEME) });
    }
    // `serializeModel` is required for anything carrying a PositionedModel: it
    // holds Map/Set values that plain JSON.stringify silently flattens (the
    // viewer then dies on "model.classDefs is not iterable"). A sequence layout
    // has no `.model` and is already plain.
    result[stem] = built?.model
      ? { ...built, model: serializeModel(built.model), _model: true }
      : built?.kind
        ? built
        : { _bare: serializeModel(built) };
    ok++;
  } catch (e) {
    failed++;
    // Record WHY, keyed like a layout, so the viewer can show the real parse
    // error instead of wrongly blaming a missing install. (These are almost
    // always genuinely invalid Mermaid - mermaid.js rejects them identically.)
    // Keep the source excerpt + caret, not just the first line - that is the
    // part that actually locates the problem. The .err box renders pre-wrap.
    const why = String(e.message || e).trim().slice(0, 400);
    result[stem] = { _error: why };
    console.error(`layout: skip ${stem} - ${why.split("\n")[0].slice(0, 120)}`);
  }
}

writeFileSync(out, JSON.stringify(result));
console.error(`layout: ${ok} laid out, ${failed} skipped -> ${out}`);
