# Review round 01 — feature `diagram-viewer-and-uml-diff` (Stage 1 of 3)

Scope: **Stage 1 only** (FR1-FR6, the xplan-style interactive renderer). Stage 2
(before/after compare), Stage 3 (done-link), and the 0.5.0 -> 0.6.0 version bump
are later stages and were **not** reviewed / not flagged missing.

Fresh-eyes review by `gogo-reviewer`. Reviewed against `plan.md`, `decisions.md`
(D1 hybrid, **D2=A no label editing**, D3 flowchart-family rich + fallback, D6
sidecar), and `.gogo/knowledge/{code-review-standards,coding-rules,non-functional-requirements}.md`.

Files reviewed:
- New: `assets/viewer/{geometry.js, viewport.js, mermaid-parse.js, render.js}`
- Modified: `assets/viewer/{interactive.js, viewer.css, viewer.template.html}`, `skills/gogo-view/SKILL.md`
- Evidence artifact: `.gogo/resources/view/sample-flow.html`

## Verdict: **APPROVE**

No open blockers or majors. Stage 1 is a faithful, offline-safe port. Four
findings, all minor/nit; one (REV-002) carries a **needs-user-decision** on how
far FR4 persistence should go in v1 — the orchestrator may route it to a decision
gate before advancing, but it does not block Stage 1 approval.

## What was verified clean

- **Offline / portability (FR6):** all five JS modules are plain IIFEs attaching to
  `window.gogoViewer`; no `type=module`, no ES `import/export`, no `require`, no
  `fetch()` (the word "import" appears only in comments). Zero `http(s)://`
  references except the required SVG namespace URI in `render.js:24` (not network).
  Template loads the 6 scripts in the correct order (mermaid, geometry, viewport,
  mermaid-parse, render, interactive) as plain `<script src>`. `node --check`
  passes on all five. The owned SVG edge layer uses only `createElementNS` /
  DOM APIs — nothing network/module.
- **Geometry port (geometry.js):** `borderAnchor` / `routeEdge` / `contentBounds` /
  `clamp` are line-for-line faithful to xplan's `diagramGeometry.ts` (aspect-aware
  anchor side; all four orthogonal elbow branches match; correct `mid` per branch).
  No NaN on zero-size boxes (parser drops zero-area nodes; `contentBounds` always
  pads so `fit` never divides by zero).
- **Viewport port (viewport.js):** drag delta / zoom, `CLICK_SLOP=4`, zoom-toward-
  cursor (`pan = c - (c - pan) * k`), fit (clamp to `<= 1` then center),
  `ZOOM_MIN/MAX = 0.25/2.5` all match `useCanvasViewport.ts`.
- **Parser (mermaid-parse.js):** selectors match the **vendored mermaid v10.9.1**
  (confirmed it emits `data-id` on node groups and `LS-`/`LE-`/`flowchart-link`
  edge classes). Returns `null` for non-flowchart kinds -> fallback. Guards missing
  bbox (`try/getBBox`), unknown ids (endpoints validated against the node set), and
  malformed transforms. `querySelectorAll` de-dupes multi-selector matches, so no
  duplicate edges.
- **Drag -> re-route wiring (render.js + viewport.js):** dragging a node updates
  `state.pos`; `draw()` re-routes every edge via `routeEdge(boxOf(from), boxOf(to))`
  each frame, so edges follow live. Zoom/fit/reset-layout controls, minimap
  inverse-transform + click/drag-to-recenter all correct.
- **Listener hygiene:** window `pointermove`/`pointerup` are added **once per rich
  diagram** (per `render()` call), **not per drag** — mirroring xplan's add-once
  pattern. Bounded by diagram count; no per-drag leak. The naive "added per drag,
  never removed" concern does **not** apply here.
- **D2=A honored:** no `contentEditable` / `designMode` / `execCommand` anywhere;
  node cards use `textContent`. No label editing.
- **Orchestration / no regression (interactive.js):** flowchart-family -> rich;
  other kinds / empty model -> preserved fallback; a rich-render throw is caught and
  routed to fallback (never blank); parse throw -> fallback; `suppressErrors:true`
  kept; missing-mermaid -> inline error, summary still reads.
- **gogo-view skill:** copies all `assets/viewer/*.js` (+ css) idempotently; token
  table lists the 6 ordered scripts; `<name>.layout.json` sidecar (D6) +
  `data-diagram` keys documented; `${CLAUDE_PLUGIN_ROOT}` used; 199 lines (<=200).
- **Style/ASCII:** the only non-ASCII in the skill are em-dashes / arrows / phase
  glyphs, consistent with the established house style across the repo (coding-rules
  allows the glyphs and "plain ASCII where practical"). Not flagged.

## Findings

| id | sev | pri | status | fix owner | title |
|---|---|---|---|---|---|
| REV-001 | minor | P2 | new | AGENT-FIXABLE | Emptied `<pre class="mermaid">` left in DOM — not byte-for-byte with 0.5.0 |
| REV-002 | minor | P1 | new | NEEDS-USER-DECISION | Layout persistence is in-memory only; `onPersist` unwired, file:// can't write sidecar |
| REV-003 | minor | P2 | new | AGENT-FIXABLE | Rich drag listeners omit `pointercancel` — cancelled touch/pen leaves drag stuck |
| REV-004 | nit | P3 | new | AGENT-FIXABLE | Edge-label index desyncs when an edge is dropped during parse |

### REV-001 — stray empty `<pre>` (minor, P2, AGENT-FIXABLE)
0.5.0's `setupViewport` removed the emptied `<pre>` via `pre.replaceWith(viewport)`.
`fallbackViewport` (`interactive.js:42-123`) now ends with `figure.appendChild(viewport)`
and never removes the `<pre>`; the rich path (`render.js:51`) only hides the svg and
leaves its `<pre>`. Result: a default-styled empty `<pre>` (~1em margin) sits above
each diagram — a small cosmetic gap that diverges from the prior behaviour.
**Fix:** after extracting the svg, `pre.remove()` (fallback, svg moved out) or
`pre.style.display='none'` (rich, svg hidden inside).

### REV-002 — persistence is read-only in practice (minor, P1, NEEDS-USER-DECISION)
`render.js:236-250` debounces positions into `window.gogoViewer.layouts[name]` and
would call `opts.onPersist(...)`, but `interactive.js:165` never passes `onPersist`
(dead branch), and a `file://` page cannot write the `<name>.layout.json` sidecar
from JS. Seeding from `window.GOGO_LAYOUT` works, so a **manually** saved sidecar is
honored on reload — but simply dragging then reopening loses the layout. Documented
as best-effort v1 (skill) and consistent with D6, yet FR4 / the Stage-1 test
"positions persist ... and reload" is only half-delivered (read half only).
**Decision needed:** accept manual/in-memory persistence for v1 (and drop or clearly
mark the unused `onPersist` seam), OR add a `localStorage` seed/save fallback (works
on file://), OR add an export-layout affordance so writing the sidecar is one step —
then align `plan.md` / `test-strategy.md` so the persistence test is satisfiable.

### REV-003 — missing `pointercancel` on the rich path (minor, P2, AGENT-FIXABLE)
`render.js:266-275` handles window `pointermove`/`pointerup` but not `pointercancel`;
a cancelled touch/pen pointer never calls `endDrag()`, so `drag` stays set and the
node follows a bare hover afterwards. The fallback path handles `pointercancel`
(`interactive.js:101`). **Fix:** register the pointerup handler for `pointercancel`
too.

### REV-004 — edge-label desync on dropped edges (nit, P3, AGENT-FIXABLE)
`mermaid-parse.js:143-157`: the label cursor `idx` only advances for accepted edges,
so a dropped edge (unknown endpoint) shifts every later label by one. Low impact
(labels are best-effort; valid gogo flowcharts don't drop edges). **Fix:** index
labels by the source edge position rather than a running accepted-edge counter.

## Judgment on the three focus areas
- **(a) Offline-safety:** SOLID. Genuinely zero-dep, module-free, network-free;
  works over `file://`; the only URL is the SVG namespace constant.
- **(b) Geometry-port correctness:** FAITHFUL. Math matches xplan exactly across all
  elbow branches; edge-case (zero-size / empty) handling is safe.
- **(c) Drag -> re-route wiring + listener hygiene:** CORRECT and leak-free. Edges
  re-route live from `routeEdge`; window listeners are per-render (bounded), not
  per-drag. The only interaction gap is the missing `pointercancel` (REV-003).
