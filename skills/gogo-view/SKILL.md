---
name: gogo-view
user-invocable: false
description: >-
  Build and open a self-contained, offline interactive webpage for a gogo plan or
  report — the plan.md / report.md summary pre-rendered to readable HTML plus its
  mermaid diagrams made interactive by the vendored very-nice-mermaid renderer:
  every kind (flow, use-case, sequence, class, activity) gets draggable nodes,
  live-re-routing edges, a minimap, fit / zoom / reset and SVG-PNG export, plus a
  ⛶ expand-to-modal control (vendored runtime, no network, no build). A bundle
  that carries a before/ set renders before | after side by side (compare mode).
  Use when the user runs /gogo:view or asks to view / browse a plan, a changelog
  entry, or a feature's report. Enumerates both plans and reports grouped Work
  (each feature's plan + report) / Changelog (shipped reports), lets the user
  pick, builds the page under .gogo/resources/view/, and opens it.
---

# gogo-view — interactive viewer for gogo plans & reports (FR8 / FR9 / FR10)

Turns a plan or report bundle (`plan.md` / `report.md` + its `.mmd` diagrams) into
one self-contained HTML page: the summary rendered as a clean centered article,
and each diagram made interactive by the vendored
[**very-nice-mermaid**](https://www.npmjs.com/package/very-nice-mermaid) renderer
(`.gogo/resources/vnm-browser.js`). **Every** gogo kind - `flow`, `use-case`,
`sequence`, `class`, `activity` - gets the same first-class treatment: node
**cards you can drag**, an edge layer that **re-routes live**, a **minimap**,
per-diagram fit / zoom / reset, **SVG / PNG export**, and gogo's **⛶
expand-to-modal**. Dragged positions auto-persist to `localStorage` per diagram
(D7=A) and can be exported as a portable `<name>.layout.json` sidecar. Opens over
`file://` with **no network, no build, no runtime deps**. Pure
Glob / Grep / Read / Write / Bash; only ever writes under `.gogo/`.

**Plans view in place too (D1=A).** A feature's **plan** is a first-class viewable
bundle: `plan.md` (its summary) + its `charts/*.mmd` (its intended-design diagrams,
plus `charts/before/*` as the as-is baseline) render with the *same* interactive
renderer, from the feature root — `plan.md` is **not** moved (it stays the contract
path every phase reads). Reports still live in `report/`.

## Inputs (declared) and outputs

| Direction | Artifact | Contract |
|---|---|---|
| in (one required) | a chosen `report.md` (+ its sibling `*.mmd`) | the report bundle |
| in (one required) | a feature's `plan.md` (+ its `charts/*.mmd`) | the plan bundle (D1=A, in place) |
| in (optional) | a `before/` set beside the report (`report/before/*.mmd`, FR8) or a plan's `charts/before/*.mmd` | triggers compare mode (FR9) |
| in (assets) | `${CLAUDE_PLUGIN_ROOT}/assets/vnm/{viewer.template.html,viewer.js,viewer.css}` | the gogo page + orchestrator |
| in (assets) | `${CLAUDE_PLUGIN_ROOT}/assets/vnm/vnm-browser.js` | vendored very-nice-mermaid renderer |
| in (optional) | the bundle's `layouts.json` (`charts/` for a plan, `report/` for a report) | prebuilt models - required for sequence/class/state |
| in (optional) | `.gogo/resources/view/<name>.layout.json` | saved node positions (sidecar, D6) |
| out | `.gogo/resources/{vnm-browser.js, viewer/viewer.js, viewer/viewer.css}` | shared, idempotent copies |
| out | `.gogo/resources/view/<date-or-slug>.html` (report) or `<slug>-plan.html` (plan) | the self-contained page |

## ① validate-in (gate)

At least one **viewable item** must exist — a feature `plan.md` (a plan bundle), a
`report.md` under `.gogo/changelog/*/` or `.gogo/work/feature-*/report/`, or a
legacy root `report.md` (report bundles). None of any → **STOP** and tell the user
to run `/gogo:plan` first (then `/gogo:report`, `/gogo:done`). An explicit slug/path
arg that resolves to nothing (e.g. `<slug>:report` when no report exists, or
`<slug>` with neither a plan nor a report) → STOP with the path(s) tried.

## ② Steps

### 1. Enumerate + pick (grouped Work / Changelog — plans AND reports)

Gather every selectable item, in **two groups**, newest first:

- **Work** — for each `.gogo/work/feature-*/`:
  - its **plan** — whenever `plan.md` exists (bundle = `plan.md` + its `charts/`);
    label `<slug> — plan`.
  - its **report** — when `report/report.md` exists (or a legacy root `report.md`);
    label `<slug> — report`.
- **Changelog** — each `.gogo/changelog/<date>-<name>/` that contains a `report.md`;
  label `<date>-<name> — changelog`.

```bash
ls -d .gogo/work/feature-*/          2>/dev/null  # each feature → plan (plan.md + charts/) + report if present
ls    .gogo/work/feature-*/plan.md   2>/dev/null  # the plan bundles (viewable in place, D1=A)
ls -d .gogo/work/feature-*/report/   2>/dev/null  # in-progress work reports (report/report.md)
ls    .gogo/work/feature-*/report.md 2>/dev/null  # legacy root-layout reports
ls -d .gogo/changelog/*/             2>/dev/null  # shipped entries (report.md inside)
```
Keep only entries that actually resolve — a `plan.md` for a plan, a `report.md` for
a report. Sort each group newest-first (by dir/file mtime, or the changelog date).

**Explicit arg (`$ARGUMENTS`) — resolve directly, no menu.** Arg grammar:

| Arg | Resolves to |
|---|---|
| `<slug>` | that feature's **report if it exists, else its plan** |
| `<slug>:plan` | that feature's **plan** bundle (`plan.md` + `charts/`) |
| `<slug>:report` | that feature's **report** bundle (`report/`, else legacy root `report.md`) |
| `<date>-<name>` (a changelog entry; `<name>` = the feature slug for a single entry, the release name for a merged one) | that changelog **report** |
| a path | the `plan.md` / `report.md` it names |

A `<slug>:report` (or bare `<slug>`) that resolves to no report, or a `<slug>:plan`
with no `plan.md`, → STOP with the path tried (validate-in).

**No resolvable arg** → present the **grouped picker** via `AskUserQuestion`: the
Work items (each plan / report) and the Changelog items as options, each labeled
`<slug> — plan` / `<slug> — report` / `<date>-<name> — changelog`, newest first.
The pick builds + opens its page. Default highlight: the most recent changelog
entry, else the newest work item.

Record the chosen **kind** (`plan` | `report`), the source markdown (`plan.md` or
`report.md`), its bundle dir, and a short **name** for the output file:
`<slug>-plan` for a plan; the changelog `<date>-<name>`, else the feature `<slug>`,
for a report.

### 2. Ensure shared resources (idempotent)

Copy the vendored renderer into `.gogo/resources/` (re-runs are no-ops).
Use `${CLAUDE_PLUGIN_ROOT}` — never hard-code plugin paths:
```bash
set -euo pipefail
mkdir -p .gogo/resources/viewer .gogo/resources/view
[ -f .gogo/resources/vnm-browser.js ] || \
  cp "${CLAUDE_PLUGIN_ROOT}/assets/vnm/vnm-browser.js" .gogo/resources/vnm-browser.js
cp "${CLAUDE_PLUGIN_ROOT}/assets/vnm/viewer.js"  .gogo/resources/viewer/viewer.js
cp "${CLAUDE_PLUGIN_ROOT}/assets/vnm/viewer.css" .gogo/resources/viewer/viewer.css
```
(The viewer JS/CSS are small - copy them every run so updates propagate;
`vnm-browser.js` is ~450 KB, so copy it once.) Two files, not six:
`vnm-browser.js` is the vendored **very-nice-mermaid** build (parser, dagre
layout, and interactive renderer for every diagram kind), and `viewer.js` is the
thin gogo orchestrator that mounts each figure and owns the expand-to-modal
control. gogo no longer ships a renderer of its own.

### 3. Build the page (D7 — pre-render, no JS markdown lib)

**Bundle kind — same page, two sources (FR2).** The build path is identical for a
**report** and a **plan**; only the source files differ:

| Bundle | Summary source | Diagrams source | Output page |
|---|---|---|---|
| report | the chosen `report.md` | the `*.mmd` beside it (or legacy `charts/*.mmd`) | `<date-or-slug>.html` |
| plan (D1=A, in place) | the feature's `plan.md` (at the feature root — **never moved**) | the feature's `charts/*.mmd` (+ `charts/before/*.mmd`) | `<slug>-plan.html` |

A plan bundle reuses the **exact same renderer** as a report (interactive cards
for every kind, compare mode, layout sidecar) - it is only a different markdown +
diagram source rendered into the same template.

Start from `${CLAUDE_PLUGIN_ROOT}/assets/vnm/viewer.template.html` and replace
its tokens:

| Token | Value |
|---|---|
| `GOGO_VIEW_TITLE` | a clean plain-text tab title — `gogo — <name>` for a report, `gogo — <slug> (plan)` for a plan; strip markdown (no backticks/`#`/`**`), don't duplicate "report" |
| `GOGO_VIEW_SUMMARY` | the source markdown **converted to HTML by you** — `report.md` for a report, the feature's `plan.md` for a plan (see below) |
| `GOGO_VIEW_DIAGRAMS` | one `<figure>` per diagram (see below) |
| `GOGO_VIEW_LAYOUTS` | the bundle's prebuilt `layouts.json` inlined verbatim (see below); use `{}` when there is none |
| `GOGO_VIEW_LAYOUT` | saved node positions as an inline JSON object (see "Layout sidecar" below); use `{}` when there is none |
| `GOGO_VNM_SRC` | `../vnm-browser.js` |
| `GOGO_VIEWER_SRC` | `../viewer/viewer.js` |
| `GOGO_VIEWER_CSS` | `../viewer/viewer.css` |

The two script tags load in this order - **vnm-browser, viewer** - as plain
`<script src>` at the **end of `<body>`** (never `type=module`): `file://` blocks
ES-module loading and `fetch()`, so the bundle attaches to `window.vnm` instead
of exporting.

**`GOGO_VIEW_LAYOUTS` - the prebuilt models.** Read the bundle's `layouts.json`
(written by gogo-mermaid beside the `.mmd` set: `charts/layouts.json` for a plan,
`report/layouts.json` for a report) and inline it verbatim. If it is missing,
first try to build it:

```bash
node "${CLAUDE_PLUGIN_ROOT}/assets/vnm/layout.mjs" <bundle>/layouts.json <bundle>/*.mmd
```

Exit **3** means `very-nice-mermaid` isn't installed - use `{}` and carry on (see
degradation). In **compare mode** merge the before set under `before-<stem>` keys
so they match the `data-diagram` attributes:

```bash
node "${CLAUDE_PLUGIN_ROOT}/assets/vnm/layout.mjs" /tmp/before.json <bundle>/before/*.mmd
```
then re-key each entry `<stem>` → `before-<stem>` and merge into the same object.

**How the viewer renders each figure**, in order, falling through on failure:
1. **Prebuilt layout** - `window.GOGO_LAYOUTS[<data-diagram>]` exists → mount it.
   This is the only tier that can render **sequence / class / state**, because
   routing raw DSL for those needs mermaid's `detectType` (a dynamic import
   `file://` blocks) - without a layout they would silently render as a garbage
   flowchart.
2. **Inline DSL** - a `flowchart` / `graph` source parses in-browser with vnm's
   own dependency-free parser. Covers the majority with zero authoring deps.
3. **Static SVG** - an `<svg>` inlined in the figure is shown as-is.

Anything left renders an inline `.err` box naming the diagram, with the summary
still readable - a figure never silently blanks. Every interactive figure gets
drag / zoom / fit / reset / minimap / SVG-PNG export from vnm itself, plus gogo's
**⛶ expand-to-modal** control for reading a cramped compare-row diagram big.

**Summary → HTML (you pre-render it; no runtime markdown dependency).** Convert the
chosen source markdown — `report.md` for a report, `plan.md` for a plan — to clean,
semantic HTML yourself: `#/##/###` → `<h1..3>`,
paragraphs → `<p>`, `-`/`1.` → `<ul>/<ol><li>`, GFM tables → `<table>`, fenced
code → `<pre><code>`, inline `` `code` `` → `<code>`, `[text](url)` → `<a>`, `---`
→ `<hr>`, `> ` → `<blockquote>`. **Escape** `&`, `<`, `>` in text so the page is
well-formed. Strip HTML-comment blocks. Do **not** include any `report.md` mermaid
fences in the summary — diagrams are rendered separately (next).

**Coalesce soft-wrapped continuation lines first (before emitting).** The source
markdown is soft-wrapped, so one list item or paragraph often spans several lines.
Join each continuation line into the block it belongs to: a non-blank line that is
**not** a line-leading list marker (`-` / `1.`), heading (`#`), table row (`|`),
fence (` ``` `), or blank line **continues the current `<li>`/`<p>`** — append it
with a single space and collapse runs of whitespace to one. Only a **blank line**
starts a new block; only a **line-leading marker** starts a new list item — so
consecutive `1.`/`-` lines stay in **one** `<ol>`/`<ul>` (ordered numbering never
restarts) and a wrapped line never leaks out as a stray `<p>` carrying the source's
literal indentation/double-spaces. Plain-text pass, offline — no JS markdown lib.

**Diagrams.** Gather the `.mmd` sources for the chosen bundle, **by layout** (always
skip the non-diagram files `diagrams.html` and `manifest.json`):
- new `report/` bundle (or a `<date>-<name>/` changelog entry) → the `*.mmd`
  beside `report.md`;
- legacy root-layout report (`.gogo/work/feature-<slug>/report.md`) → the
  feature's `charts/*.mmd` (older features keep diagrams under `charts/`);
- **plan** bundle → the feature's `charts/*.mmd` (the plan's intended-design set),
  with `charts/before/*.mmd` as the **before** set for compare mode (see below).

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
set alongside the after `.mmd`, build the diagrams as **side-by-side pairs** instead
of a single column. The before/after sources depend on the bundle:
- **report** → after = the `*.mmd` beside `report.md`; before = its `before/*.mmd`
  sub-folder (copied in by phase ⑤).
- **plan** → after = the feature's `charts/*.mmd` (the intended design); before =
  `charts/before/*.mmd` (the as-is baseline plan ① drew). Same pairing rules below.

Then:

- **Pair by filename stem.** Match each `before/<stem>.mmd` to the after
  `<stem>.mmd` by full basename — `<stem>` is the filename without extension
  (`flow`, or a changelog entry's slug-prefixed `<slug>-flow`), not the manifest
  `kind` enum. For a stem present in **both**, emit a `<div class="compare">`
  wrapping **two** `figure.diagram` elements — **Before** (left) then **After**
  (right) — each with its `.mmd` source inlined **verbatim** (never `fetch()`).
  Give the after figure the normal `data-diagram="<basename>"` and the before
  figure `data-diagram="before-<basename>"` so their saved layouts never collide.
  Mark them `class="diagram compare-before"` / `class="diagram compare-after"`
  and caption "Before — <title>" / "After — <title>".
- **Unmatched stems.** A stem present on only one side is a single full-width figure
  inside its own `.compare` row with `class="diagram compare-solo"`, captioned
  "Added — <title>" (after only) or "Removed — <title>" (before only).
- **Still fully interactive.** `viewer.js` mounts **every** `figure.diagram` on
  the page, so compare mode is **pure markup** - two figures per row, no renderer
  change. Give the before set its own `before-<stem>` keys in `GOGO_VIEW_LAYOUTS`
  so both sides get prebuilt models. The `.compare` CSS is two columns that fall
  back to stacked on narrow widths (`viewer.css`).

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
  (this committed sidecar) → then `localStorage` → then the prebuilt positions.
  So a committed sidecar wins first-open; thereafter local drags persist. **No label
  editing** (decision D2=A) — modify == reposition + restyle + persist only.

**Write** the finished page to `.gogo/resources/view/<name>.html`. Writing it
there fixes the relative paths: `../vnm-browser.js` and `../viewer/...` resolve on
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

One line: which plan or report was viewed, the generated page path, and the
`file://` URL.

## Degradation

- No `open`/`xdg-open` → print the path (above).
- Renderer missing at view time → the page shows an inline error per diagram
  (handled by `viewer.js`) and the summary still reads; re-running the skill
  restores `.gogo/resources/vnm-browser.js`.
- `very-nice-mermaid` not installed → no `layouts.json`: **flowcharts still
  render** (parsed in-browser), while sequence / class / state show an inline
  `.err` naming the diagram and the fix (`npm i -g very-nice-mermaid`). Never a
  silent blank, never a garbage flowchart.
- A bundle with no diagrams → a summary-only page (valid).
- Only ever writes under `.gogo/`; offline throughout (no network, no build).
