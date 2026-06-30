---
name: gogo-view
description: >-
  Build and open a self-contained, offline interactive webpage for a gogo report
  — the report.md summary pre-rendered to readable HTML plus its mermaid diagrams
  wrapped in a pan / zoom / drag canvas (vendored runtime, no network, no build).
  Use when the user runs /gogo:view or asks to view / browse a changelog entry or
  a feature's report. Lists reports from .gogo/changelog/ and .gogo/work/*/report/,
  lets the user pick, builds the page under .gogo/resources/view/, and opens it.
---

# gogo-view — interactive viewer for gogo reports (FR8 / FR9 / FR10)

Turns a report bundle (`report.md` + its `.mmd` diagrams) into one self-contained
HTML page: the summary rendered as a clean centered article, and each diagram in a
**pan / zoom / drag canvas** with per-diagram reset / fit / zoom controls. Renders
mermaid client-side from the **vendored** `.gogo/resources/mermaid.min.js` — opens
over `file://` with **no network, no build, no runtime deps**. Pure
Glob / Grep / Read / Write / Bash; only ever writes under `.gogo/`.

## Inputs (declared) and outputs

| Direction | Artifact | Contract |
|---|---|---|
| in (required) | a chosen `report.md` (+ its sibling `*.mmd`) | the report bundle |
| in (assets) | `${CLAUDE_PLUGIN_ROOT}/assets/viewer/{viewer.template.html,interactive.js,viewer.css}` | vendored renderer |
| in (assets) | `${CLAUDE_PLUGIN_ROOT}/assets/mermaid/mermaid.min.js` | vendored mermaid |
| out | `.gogo/resources/{mermaid.min.js, viewer/interactive.js, viewer/viewer.css}` | shared, idempotent copies |
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
(re-runs are no-ops). Use `${CLAUDE_PLUGIN_ROOT}` — never hard-code plugin paths:
```bash
set -euo pipefail
mkdir -p .gogo/resources/viewer .gogo/resources/view
[ -f .gogo/resources/mermaid.min.js ] || \
  cp "${CLAUDE_PLUGIN_ROOT}/assets/mermaid/mermaid.min.js" .gogo/resources/mermaid.min.js
cp "${CLAUDE_PLUGIN_ROOT}/assets/viewer/interactive.js" .gogo/resources/viewer/interactive.js
cp "${CLAUDE_PLUGIN_ROOT}/assets/viewer/viewer.css"      .gogo/resources/viewer/viewer.css
```
(The viewer JS/CSS are small — copy them every run so updates propagate; mermaid is
large, so copy it once.)

### 3. Build the page (D7 — pre-render, no JS markdown lib)

Start from `${CLAUDE_PLUGIN_ROOT}/assets/viewer/viewer.template.html` and replace
its tokens:

| Token | Value |
|---|---|
| `GOGO_VIEW_TITLE` | a clean plain-text tab title `gogo — <name>` (the changelog `<date>-<slug>` or feature `<slug>`); strip markdown — no backticks/`#`/`**`, and don't duplicate the word "report" |
| `GOGO_VIEW_SUMMARY` | the **report.md, converted to HTML by you** (see below) |
| `GOGO_VIEW_DIAGRAMS` | one `<figure>` per diagram (see below) |
| `GOGO_MERMAID_SRC` | `../mermaid.min.js` |
| `GOGO_VIEWER_SRC` | `../viewer/interactive.js` |
| `GOGO_VIEWER_CSS` | `../viewer/viewer.css` |

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
filename:
```html
<figure class="diagram">
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
