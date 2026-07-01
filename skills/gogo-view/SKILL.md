---
name: gogo-view
description: >-
  Build and open a self-contained, offline interactive webpage for a gogo report
  — the report.md summary pre-rendered to readable HTML plus its mermaid diagrams
  made interactive: flowchart-family kinds get an xplan-style rich renderer
  (draggable, token-styled node cards with a live-re-routing edge layer + minimap),
  other kinds fall back to a pan / zoom / drag canvas (vendored runtime, no
  network, no build). A report that carries a before/ set renders before | after
  side by side (compare mode). Use when the user runs /gogo:view or asks to view / browse a
  changelog entry or a feature's report. Lists reports from .gogo/changelog/ and
  .gogo/work/*/report/, lets the user pick, builds the page under
  .gogo/resources/view/, and opens it.
---

# gogo-view — interactive viewer for gogo reports (FR8 / FR9 / FR10)

Turns a report bundle (`report.md` + its `.mmd` diagrams) into one self-contained
HTML page: the summary rendered as a clean centered article, and each diagram made
interactive. **Flowchart-family** diagrams (`flow` + `use-case`) get an
**xplan-style rich renderer** — mermaid lays them out, the viewer parses that SVG
into a `{nodes,edges}` model and owns interaction: token-styled node **cards you
can drag**, an **owned edge layer** that re-routes live, a **minimap**, and
per-diagram fit / zoom / reset-layout controls (dragged positions auto-persist to
`localStorage`, plus an **export** control that downloads the portable
`<name>.layout.json` sidecar; D7=A). **Other kinds** (`sequence` / `class` /
`stateDiagram`) fall back to the **pan / zoom / drag canvas**. Renders mermaid
client-side from the **vendored** `.gogo/resources/mermaid.min.js` — opens over
`file://` with **no network, no build, no runtime deps**. Pure
Glob / Grep / Read / Write / Bash; only ever writes under `.gogo/`.

## Inputs (declared) and outputs

| Direction | Artifact | Contract |
|---|---|---|
| in (required) | a chosen `report.md` (+ its sibling `*.mmd`) | the report bundle |
| in (optional) | a `before/` set beside the report (`report/before/*.mmd`, FR8) | triggers compare mode (FR9) |
| in (assets) | `${CLAUDE_PLUGIN_ROOT}/assets/viewer/{viewer.template.html,viewer.css}` + all `assets/viewer/*.js` | vendored renderer (modular) |
| in (assets) | `${CLAUDE_PLUGIN_ROOT}/assets/mermaid/mermaid.min.js` | vendored mermaid |
| in (optional) | `.gogo/resources/view/<name>.layout.json` | saved node positions (sidecar, D6) |
| out | `.gogo/resources/{mermaid.min.js, viewer/*.js, viewer/viewer.css}` | shared, idempotent copies |
| out | `.gogo/resources/view/<date-or-slug>.html` | the self-contained page |

## ① validate-in (gate)

At least one report bundle must exist (a `report.md` under `.gogo/changelog/*/`
or `.gogo/work/feature-*/report/`). None found → **STOP** and tell the user to
run `/gogo:report` (and `/gogo:done`) first. An explicit slug/path arg that
resolves to no `report.md` → STOP with the path tried.

## ② Steps

### 1. Enumerate + pick

Gather every selectable report:
```bash
ls -d .gogo/changelog/*/ 2>/dev/null            # shipped entries (report.md inside)
ls -d .gogo/work/feature-*/report/ 2>/dev/null  # in-progress work reports
# older features keep report.md at the feature root — include those too:
ls .gogo/work/feature-*/report.md 2>/dev/null
```
Keep only paths that actually contain a `report.md`. Label each (changelog entries
by `<date>-<slug>`; work reports by slug + "in progress"). Present the list.

- If `$ARGUMENTS` names a changelog entry, a feature slug, or a path, resolve it
  directly (no prompt).
- Otherwise, when interactive and the choice is unclear, ask with
  `AskUserQuestion` (one row per report). Default to the most recent changelog
  entry, else the newest work report.

Record the chosen `report.md`, its bundle dir, and a short **name** for the output
file: the changelog `<date>-<slug>`, else the feature `<slug>`.

### 2. Ensure shared resources (idempotent)

Copy the vendored runtime + renderer into `.gogo/resources/` only if missing
(re-runs are no-ops). The viewer is now **modular** — copy every
`assets/viewer/*.js` (they load as ordered plain `<script>` tags, no bundler).
Use `${CLAUDE_PLUGIN_ROOT}` — never hard-code plugin paths:
```bash
set -euo pipefail
mkdir -p .gogo/resources/viewer .gogo/resources/view
[ -f .gogo/resources/mermaid.min.js ] || \
  cp "${CLAUDE_PLUGIN_ROOT}/assets/mermaid/mermaid.min.js" .gogo/resources/mermaid.min.js
cp "${CLAUDE_PLUGIN_ROOT}"/assets/viewer/*.js  .gogo/resources/viewer/
cp "${CLAUDE_PLUGIN_ROOT}/assets/viewer/viewer.css" .gogo/resources/viewer/viewer.css
```
(The viewer JS/CSS are small — copy them every run so updates propagate; mermaid is
large, so copy it once.) The modules are: `geometry.js` (pure edge/anchor math),
`viewport.js` (pan/zoom/fit/drag controller), `mermaid-parse.js` (rendered SVG →
`{nodes,edges}` model), `render.js` (rich node-card + owned-edge renderer +
minimap), and `interactive.js` (the orchestrator + fallback).

### 3. Build the page (D7 — pre-render, no JS markdown lib)

Start from `${CLAUDE_PLUGIN_ROOT}/assets/viewer/viewer.template.html` and replace
its tokens:

| Token | Value |
|---|---|
| `GOGO_VIEW_TITLE` | a clean plain-text tab title `gogo — <name>` (the changelog `<date>-<slug>` or feature `<slug>`); strip markdown — no backticks/`#`/`**`, and don't duplicate the word "report" |
| `GOGO_VIEW_SUMMARY` | the **report.md, converted to HTML by you** (see below) |
| `GOGO_VIEW_DIAGRAMS` | one `<figure>` per diagram (see below) |
| `GOGO_VIEW_LAYOUT` | saved node positions as an inline JSON object (see "Layout sidecar" below); use `{}` when there is none |
| `GOGO_MERMAID_SRC` | `../mermaid.min.js` |
| `GOGO_GEOMETRY_SRC` | `../viewer/geometry.js` |
| `GOGO_VIEWPORT_SRC` | `../viewer/viewport.js` |
| `GOGO_MERMAID_PARSE_SRC` | `../viewer/mermaid-parse.js` |
| `GOGO_RENDER_SRC` | `../viewer/render.js` |
| `GOGO_VIEWER_SRC` | `../viewer/interactive.js` |
| `GOGO_VIEWER_CSS` | `../viewer/viewer.css` |

The six script tags load in this order — **mermaid, geometry, viewport,
mermaid-parse, render, interactive** — as plain `<script src>` (never
`type=module`): `file://` blocks ES-module loading and `fetch()`, so each module
attaches to the shared `window.gogoViewer` namespace instead of importing.

**Rich vs fallback rendering (what the viewer does at runtime).** For each
diagram the orchestrator lets mermaid lay it out to SVG, then `mermaid-parse.js`
tries to read a `{nodes,edges}` model from it. **Flowchart-family** diagrams
(`flowchart` / `graph` — gogo `flow` + `use-case`) parse successfully and get the
**rich renderer**: token-styled node cards you can drag, an owned edge layer that
re-routes live, a minimap, and fit/zoom/reset-layout. **Other kinds**
(`sequence` / `class` / `stateDiagram`) return `null` and **fall back** to the
0.5.0 pan/zoom/drag canvas — no regression, never a blank page. A missing mermaid
runtime still degrades to an inline error per diagram with the summary readable.

**Summary → HTML (you pre-render it; no runtime markdown dependency).** Convert the
chosen `report.md` to clean, semantic HTML yourself: `#/##/###` → `<h1..3>`,
paragraphs → `<p>`, `-`/`1.` → `<ul>/<ol><li>`, GFM tables → `<table>`, fenced
code → `<pre><code>`, inline `` `code` `` → `<code>`, `[text](url)` → `<a>`, `---`
→ `<hr>`, `> ` → `<blockquote>`. **Escape** `&`, `<`, `>` in text so the page is
well-formed. Strip HTML-comment blocks. Do **not** include any `report.md` mermaid
fences in the summary — diagrams are rendered separately (next).

**Diagrams.** Gather the `.mmd` sources for the chosen report, **by layout**:
- new `report/` bundle (or a `<date>-<slug>/` changelog entry) → the `*.mmd`
  beside `report.md` (skip `diagrams.html`);
- legacy root-layout report (`.gogo/work/feature-<slug>/report.md`) → the
  feature's `charts/*.mmd` (older features keep diagrams under `charts/`).

For each, emit one block, **inlining the source verbatim** (do **not** `fetch()` —
`file://` forbids it). Caption from `manifest.json` `title` if present, else the
filename. Set `data-diagram` to the `.mmd` **basename** (no extension) — the
renderer uses it as the stable key for saved node positions:
```html
<figure class="diagram" data-diagram="flow">
  <figcaption>Flow — code-verified discovery</figcaption>
  <pre class="mermaid">
flowchart TD
  A[...] --> B[...]
  </pre>
</figure>
```
If **no** `.mmd` is found for the pick, leave `GOGO_VIEW_DIAGRAMS` empty AND tell
the user "no diagrams found for <name> — showing the summary only" (so a missing
diagram set is never silent). A genuine pure-process report legitimately has none.

**Compare mode (before / after — FR9).** When the chosen bundle carries a `before/`
set alongside the after `.mmd` (i.e. a `before/*.mmd` sub-folder beside `report.md`,
copied there by phase ⑤), build the diagrams as **side-by-side pairs** instead of a
single column:

- **Pair by kind.** Match each `before/<kind>.mmd` to the after `<kind>.mmd`. For a
  kind present in **both**, emit a `<div class="compare">` wrapping **two**
  `figure.diagram` elements — **Before** (left) then **After** (right) — each with
  its `.mmd` source inlined **verbatim** (never `fetch()`). Give the after figure the
  normal `data-diagram="<basename>"` and the before figure
  `data-diagram="before-<basename>"` so their saved layouts never collide. Mark them
  `class="diagram compare-before"` / `class="diagram compare-after"` and caption
  "Before — <title>" / "After — <title>".
- **Unmatched kinds.** A kind present on only one side is a single full-width figure
  inside its own `.compare` row with `class="diagram compare-solo"`, captioned
  "Added — <title>" (after only) or "Removed — <title>" (before only).
- **Still fully interactive.** `interactive.js` renders **every** `figure.diagram`
  on the page (rich for flowchart-family, pan/zoom fallback otherwise), so compare
  mode is **pure markup** — two figures per row, no renderer change. The `.compare`
  CSS is two columns that fall back to stacked on narrow widths (`viewer.css`).

This is **side-by-side + labeled pairs only** (decision D4=A) — do **not** compute a
structural node-diff. A bundle with **no** `before/` set builds the normal
single-column layout, unchanged.

**Layout sidecar + persistence (D6 / D7=A — saved node positions).** The rich
renderer persists dragged positions per diagram, keyed by `data-diagram`. A
`file://` page can't write files from JS, so persistence works in two layers:
- **Auto-persist to `localStorage`.** On drag-end (debounced) the renderer writes
  the diagram's `{ "<nodeId>": {"x":N,"y":N} }` positions to `localStorage` under
  the key `gogo-view:layout:<data-diagram>`. localStorage works over `file://`, so
  simply re-opening the same page in the same browser keeps the arrangement with
  **zero manual steps**. **Reset-layout** restores mermaid's original positions and
  clears that diagram's localStorage entry.
- **Export layout (portable sidecar).** Each diagram's controls include an
  **"export"** button that downloads a `layout.json` — the full
  `window.gogoViewer.layouts` map in the D6 shape
  `{ "<diagram-basename>": { "<nodeId>": {"x":N,"y":N}, ... }, ... }` — via an
  offline `Blob` download (no network). Save it as
  `.gogo/resources/view/<name>.layout.json` to commit / share the arrangement.
- **Seed on load** (this skill's job) — read `.gogo/resources/view/<name>.layout.json`
  if it exists (the map above) and inline it into the `GOGO_VIEW_LAYOUT` token; if
  it doesn't, use `{}` (optionally write an empty `{}` sidecar so the path is
  obvious). The renderer's seed order is: injected `window.GOGO_LAYOUT[<data-diagram>]`
  (this committed sidecar) → then `localStorage` → then mermaid's parsed positions.
  So a committed sidecar wins first-open; thereafter local drags persist. **No label
  editing** (decision D2=A) — modify == reposition + restyle + persist only.

**Write** the finished page to `.gogo/resources/view/<name>.html`. Writing it
there fixes the relative paths: `../mermaid.min.js` and `../viewer/...` resolve on
disk, so the page is self-contained and offline. The page must contain **no**
`http(s)://` CDN/network references.

### 4. Open it (best-effort, FR10 graceful)

```bash
page=".gogo/resources/view/<name>.html"
abs="$(cd "$(dirname "$page")" && pwd)/$(basename "$page")"
if command -v open >/dev/null 2>&1; then open "$abs"
elif command -v xdg-open >/dev/null 2>&1; then xdg-open "$abs"
fi
echo "Open it manually if it didn't launch: file://$abs"
```
Auto-open is best-effort: on any failure (or a headless box), **print the absolute
`file://` path** so the user can open it themselves. Never fail the command
because a browser couldn't be launched.

## ③ Return

One line: which report was viewed, the generated page path, and the `file://` URL.

## Degradation

- No `open`/`xdg-open` → print the path (above).
- mermaid runtime missing at view time → the page shows an inline error per
  diagram (handled by `interactive.js`) and the summary still reads; re-running
  the skill restores `.gogo/resources/mermaid.min.js`.
- A bundle with no diagrams → a summary-only page (valid).
- Only ever writes under `.gogo/`; offline throughout (no network, no build).
