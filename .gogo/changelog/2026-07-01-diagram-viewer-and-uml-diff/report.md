# Report — feature `diagram-viewer-and-uml-diff`

- **feature:** xplan-style interactive diagram viewer + before/after UML compare
- **status:** done
- **completed:** 2026-07-01
- **branch / commits:** main · uncommitted (0.6.0 release pending)

## Run status / gaps
Clean (strict) run — all phases completed green. Review **APPROVE** (REV-001..007
all `verified`, 6 rounds); test **GREEN** (Stage 1 driven **live in Chromium** via
Playwright; Stages 2/final green). No open issues. (No `charts/before/` set for
this feature — the before/after convention it *introduces* didn't exist at its own
plan ①, so the comparison here shows the after set only, exercising the graceful
no-before path.)

## Summary
Upgraded `/gogo:view` from a flat "mermaid image in a pan/zoom box" to an
**xplan-style interactive renderer**: mermaid still does layout, but its rendered
SVG is parsed into a `{nodes, edges}` model and re-rendered as **custom-styled node
cards with an owned edge layer** — you **drag nodes and their edges re-route live**,
with zoom/fit/**minimap** and a **persisted layout** (localStorage + export). Added
**before/after UML compare** (plan ① draws the as-is "before"; report ⑤ draws the
after + a side-by-side comparison; `/gogo:view` compare mode) and made **`/gogo:done`
print an interactive viewer `file://` link**. All offline / vanilla-JS / zero-dep.
Version **0.5.0 → 0.6.0**.

## Planned vs shipped
Shipped **as planned** (FR1–FR11) across three stages. Two decisions were tightened
during the build: **D2=A** (v1 "modify" = move + restyle + persist; **label editing
deferred**) and **D7=A** (persistence = **localStorage + Export button**, because a
`file://` page can't write the sidecar from JS — raised by review REV-002). The
structural node-diff (D4-B) stayed a deferred stretch; the comparison is
side-by-side + prose.

## Implementation
- **Stage 1 — renderer (`assets/viewer/`):** new vanilla-JS modules loaded as
  ordered plain `<script>`s (no ES modules — `file://` blocks them): `geometry.js`
  (xplan `borderAnchor`+`routeEdge` orthogonal router, ported), `viewport.js`
  (pan/zoom-to-cursor/fit/per-node drag), `mermaid-parse.js` (v10 SVG →
  `{nodes,edges}`; null → fallback), `render.js` (token-styled cards + owned edge
  layer + minimap + localStorage persist + Export). `interactive.js` orchestrates:
  flowchart-family → rich; other kinds → the preserved 0.5.0 pan/zoom canvas.
- **Stage 2 — before/after:** plan ① draws `charts/before/` (as-is); report ⑤
  copies it into `report/before/` (paths rewritten so the archive doesn't dangle)
  and adds a Before/after section; `/gogo:view` compare mode = pure markup
  (`.compare` grid) reusing the Stage-1 renderer per pane (no JS change).
- **Final — FR10 + FR11:** `/gogo:done` reuses the `gogo-view` build to emit
  `.gogo/resources/view/<date>-<slug>.html` + prints its `file://` link (graceful
  fallback); docs/README/enumeration sweep + version 0.6.0.

### Changes (as-built, by area)
| Area | Files |
|---|---|
| Renderer (new) | `assets/viewer/{geometry,viewport,mermaid-parse,render}.js` |
| Renderer (mod) | `assets/viewer/{interactive.js, viewer.css, viewer.template.html}` |
| Before/after + compare | `skills/{gogo-mermaid,gogo-plan,gogo-knowledge,gogo-view}/SKILL.md`, `templates/report.template.md`, `assets/viewer/viewer.css` (`.compare`) |
| Done→link | `skills/gogo-done/SKILL.md`, `commands/done.md` |
| Docs/version | `docs/{architecture,commands,flow}.md`, `README.md`, `skills/gogo/SKILL.md`, `templates/state.template.md`, `.claude-plugin/plugin.json` (0.6.0) |

## Decisions & rationale
| Decision | Choice | Reason |
|---|---|---|
| D1 Renderer approach | **Hybrid** (mermaid layout → parse SVG → custom render/interact) | ~xplan interaction without an auto-layout engine or a pipeline rewrite; keeps `.mmd` |
| D2 "Modify" scope | Move + restyle + **persist**; no label edit | xplan itself doesn't edit labels; a real edit needs a `.mmd` writer + re-layout |
| D3 Rich kinds | Flowchart-family rich; others fall back | Flowcharts are the common case + cleanly parseable |
| D4 Compare | Side-by-side + prose (node-diff deferred) | Delivers the value; a computed diff is a bigger, later lift |
| D5 Before storage | `charts/before/` → copied to `report/before/` | Self-contained archive/viewer |
| D6 Persistence store | Sidecar `<name>.layout.json` | Inspectable/portable — but see D7 |
| D7 Write-back | **localStorage + Export button** | `file://` can't write files from JS; localStorage auto-persists, Export writes the portable sidecar |
See [decisions.md](../decisions.md) · [adjustments.md](../adjustments.md).

## Review outcome
**APPROVE** (6 rounds). REV-001 (leftover `<pre>`), REV-002 (persistence → D7),
REV-003 (`pointercancel`), REV-004 (edge-label index), REV-005 (`report/before`
manifest path rewrite), REV-006/007 (doc polish) — all **verified**. The reviewer
confirmed the geometry port is faithful, offline-safe, drag→re-route is correct and
leak-free, and gogo-done reuses gogo-view with a graceful fallback.
See [review/issues.json](../review/issues.json) · snapshots `review-01..03.md`.

## Test outcome
**GREEN.** **Stage 1 was driven live in Chromium (Playwright):** dragging a node
**re-routed its edges** (edge `d` changed to follow) and **positions persisted
across reload** (localStorage seed); plus fit/zoom/minimap, the sequence-diagram
**fallback**, no network, no console errors, no label-edit. Stage 2 (compare mode
side-by-side, independent panes, responsive stacking) and the final pass
(`/gogo:done` build+link + graceful no-diagram fallback; FR11 structural) also
green. See [test/issues.json](../test/issues.json) · snapshots `test-01..03.md` +
`test/*.png` screenshots.

## Diagrams
As-built — open [report/diagrams.html](./diagrams.html), or the fully interactive
[`/gogo:view`](../../../..) page:
- **Flow** (`flow.mmd`) — the renderer pipeline (layout → parse → custom render +
  owned edges + drag/persist; fallback for non-flowchart kinds).
- **Sequence** (`sequence.mmd`) — `/gogo:done` reuses the gogo-view build → a
  self-contained page → offline browser interaction.
- **Use-case** (`use-case.mmd`) — the new capabilities (interactive view, drag +
  re-route, persisted layout, before/after compare, done→viewer link).

## Before / after comparison
No `charts/before/` baseline exists for this feature (it introduces the convention),
so there is nothing to compare against — the after set above stands alone. Future
features that touch an existing flow will show a real before↔after here.

## Knowledge updates
- `.gogo/knowledge/project-knowledge.md` — gogo-overrides note for 0.6.0 (the
  interactive viewer + before/after compare + `charts/before`/`report/before` +
  done→link). No upstream/proxied file edited.
- Shipped templates updated so new projects inherit it: `templates/report.template.md`
  (Before/after section), `templates/state.template.md` (before/ folders).

## Follow-ups & known limitations
- **Live-browser test of the `/gogo:done`-built page:** verified structurally +
  manual steps (Playwright/claude-in-chrome block `file://` on this box; Stage 1
  proved the same renderer live via a local http server). A quick manual open
  confirms compare panes.
- **Deferred (as planned):** label editing (D2), computed structural node-diff
  (D4-B), rich interaction for sequence/class/state kinds (fallback only).
- **Release 0.6.0:** commit + push + tag (working tree uncommitted by design).
- **Roadmap (in memory):** pre/post per-phase agent extensions; deeper xplan
  integration.
