# Test round 1 — feature `diagram-viewer-and-uml-diff`

**Stage:** 1 of 3 (FR1–FR6, the interactive renderer)  
**Date:** 2026-07-01  
**Method:** Live browser via Playwright MCP (all 7 cases driven in a real Chromium session served over HTTP from the built `.gogo/resources/` directory)  
**Result: GREEN — 0 open issues, all test cases passed**

---

## Setup

- HTTP server started at `http://127.0.0.1:8765` serving `.gogo/resources/` (simulating `file://`; Playwright MCP blocks the `file://` protocol directly).
- Two test pages used:
  - `sample-flow.html` (flowchart diagram, rich renderer path) — already existed
  - `sample-seq.html` (sequence diagram, fallback path) — created for this test round at `.gogo/resources/view/sample-seq.html`
- All 10 JS files checked with `node --check` before browser testing (see below).

---

## JS syntax check (node --check)

Ran on both the source files in `assets/viewer/` and the deployed copies in `.gogo/resources/viewer/`. All 10 files passed.

| File | Result |
|---|---|
| `assets/viewer/geometry.js` | OK |
| `assets/viewer/interactive.js` | OK |
| `assets/viewer/mermaid-parse.js` | OK |
| `assets/viewer/render.js` | OK |
| `assets/viewer/viewport.js` | OK |
| `.gogo/resources/viewer/geometry.js` | OK |
| `.gogo/resources/viewer/interactive.js` | OK |
| `.gogo/resources/viewer/mermaid-parse.js` | OK |
| `.gogo/resources/viewer/render.js` | OK |
| `.gogo/resources/viewer/viewport.js` | OK |

---

## Test cases (live browser)

### TC-1 — Rich render (flowchart)

**FR tested:** FR1 (structured model seeded from mermaid), FR2 (custom node/edge styling + owned edge layer)

Navigated to `sample-flow.html`. Evaluated DOM structure via `browser_evaluate`.

**Findings:**
- 10 node cards rendered as `.gogo-node` HTML divs (`WORK`, `REPORT`, `RDIR`, `RBROKEN`, `DONE`, `STOP`, `CL`, `RES`, `VIEW`, `PAGE`)
- Each card has the correct role class from the mermaid `classDef` (`role-art`, `role-proc`, `role-gate`, `role-io`, `role-out`) — visibly not mermaid's default SVG look
- 10 owned edge `<path class="gogo-edge">` elements in a `<svg class="gogo-edges">` overlay
- The original mermaid SVG removed from the DOM (the `<pre class="mermaid">` was removed by interactive.js, taking the mermaid SVG with it)
- Minimap (`div.gogo-minimap` + `div.gogo-mm-view`) present and positioned
- Rich viewport (`div.viewport.gogo-rich`) present
- Controls: −, +, fit, reset, export
- `window.gogoViewer` namespace contains: `geometry`, `createViewport`, `isFlowchart`, `parseMermaid`, `render`, `layouts`

**Result: PASS**  
Screenshot: `test/screenshots/flowchart-initial.png`

---

### TC-2 — Drag → live edge re-route

**FR tested:** FR3 (per-node drag with live edge re-routing)

Recorded edge path before drag:
```
edge[0] before: M 206 37 L 206 63.796875 L 394.25390625 63.796875 L 394.25390625 90.59375
```

Dispatched pointer events (pointerdown on WORK card, pointermove +200/+200 screen-px, pointerup) to simulate a drag. After drag:

- WORK node position: `left: 365.384px, top: 365.384px` (world-space ~365,365, up from 0,0)
- Edge path 0 (WORK → REPORT route) changed to:
```
M 571.38 365.38 L 571.38 246.49 L 394.25 246.49 L 394.25 127.59
```

The route re-calculated from the new WORK position and re-entered the REPORT node from a different side (top vs. the original left-to-top). The orthogonal router (`routeEdge`) re-ran live on every `pointermove`.

**Result: PASS — drag triggers live edge re-routing as required**  
Screenshot: `test/screenshots/flowchart-after-drag.png`

---

### TC-3 — Viewport controls (zoom, fit, reset-layout) + minimap

**FR tested:** FR3 (viewport: pan, zoom-to-cursor, fit), FR4 (reset-layout restores mermaid positions)

| Control | Before | After | Result |
|---|---|---|---|
| Zoom in (+) | scale 0.547 | scale 0.657 | PASS |
| Fit | scale 0.657 | scale 0.676, re-centered | PASS |
| Reset-layout | WORK at (365,365) | WORK at (0,0) — original mermaid position | PASS |

Minimap: `div.gogo-mm-view` has `left`, `top`, `width`, `height` CSS properties set (reflects the current viewport window within the world). Clicking/dragging the minimap calls `minimapPanTo` and recenters the viewport.

**Result: PASS**

---

### TC-4 — Persistence (D7=A): localStorage + export

**FR tested:** FR4 (persist layout, reload keeps the arrangement; export button)

**localStorage auto-persist:**
- Dragged WORK node to screen delta +150/+100 (world ~222,148)
- Waited 500ms for debounced persist (PERSIST_MS = 400ms)
- Checked `localStorage.getItem('gogo-view:layout:flow')`:
  ```json
  {"WORK":{"x":222,"y":148},"REPORT":{"x":224,"y":91},...}
  ```
- Full positions for all 10 nodes saved

**Persist on reload:**
- Reloaded `sample-flow.html` (full page navigate)
- After two rAF frames: WORK node was at `left: 222px, top: 148px` — the dragged position, seeded from localStorage

**Reset clears localStorage:**
- Before reset: `localStorage.getItem('gogo-view:layout:flow')` returned the saved JSON
- After clicking "reset": `localStorage.getItem('gogo-view:layout:flow')` returned `null`

**Export layout button:**
- Intercepted `document.body.appendChild` to capture the download `<a>` element
- `a.download = "layout.json"`, `a.href` is a `blob:` URL
- Playwright recorded: "Downloaded file layout.json"
- Downloaded file shape:
  ```json
  {
    "flow": {
      "WORK": {"x": 222, "y": 148},
      "REPORT": {"x": 224, "y": 91},
      ...
    }
  }
  ```
  This is the D6 shape: `{<diagram-name>: {<node-id>: {x, y}}}` — valid as a `window.GOGO_LAYOUT` seed.

**Result: PASS — localStorage auto-persist, reload-keeps-arrangement, and Export button all work**

---

### TC-5 — Fallback (non-flowchart: sequence diagram)

**FR tested:** FR1 (non-flowchart kinds fall back to 0.5.0 pan/zoom canvas, graceful)

Navigated to `sample-seq.html`. Evaluated DOM:

| Check | Value | Expected |
|---|---|---|
| Has `.viewport.gogo-rich` | false | false (no rich renderer) |
| Has `.viewport:not(.gogo-rich)` | true | true (fallback) |
| `.gogo-node` count | 0 | 0 (no card renderer) |
| `svg.gogo-edges` | false | false (no owned edge layer) |
| `.gogo-minimap` | false | false (no minimap) |
| `.canvas svg` present | true | true (mermaid SVG in canvas) |
| Canvas transform set | `translate(12px, 21.3232px) scale(1.03684)` | non-default (fit applied) |
| Control buttons | −, +, fit, reset | correct (no "export" — that's rich-only) |

Pan/zoom tested:
- Zoom: scale 1.037 → 1.244 after clicking "+"
- Pan: translate changed by +50/+50px after pointer drag — correct

No errors, no blank page.

**Result: PASS — sequence diagram falls back cleanly to the 0.5.0 pan/zoom canvas**  
Screenshot: `test/screenshots/sequence-fallback.png`

---

### TC-6 — Offline + console clean

**FR tested:** FR6 (offline/portable, no network, no uncaught errors)

**Network requests (flowchart page, full session):**

All 8 requests were to `127.0.0.1:8765` (the local server, equivalent to `file://` access). No requests to any external host.

```
GET /view/sample-flow.html    200
GET /viewer/viewer.css        200
GET /mermaid.min.js           200
GET /viewer/geometry.js       200
GET /viewer/viewport.js       200
GET /viewer/mermaid-parse.js  200
GET /viewer/render.js         200
GET /viewer/interactive.js    200
```

**Console messages (entire session, all levels):**

Only one message across the full session:
```
[ERROR] Failed to load resource: 404 File not found @ /favicon.ico
```

This is a browser default behavior (browser automatically requests favicon). It is not generated by the app code and is irrelevant to offline operation.

Zero application errors, zero warnings across the flowchart page, the after-drag state, the reload, and the sequence page.

**Result: PASS — fully offline, console clean**

---

### TC-7 — No label editing (D2=A)

**FR tested:** FR5 (D2=A: label editing deferred/out of scope for v1)

Fired `dblclick` event on the WORK node card:

| Check | Before | After dblclick | Result |
|---|---|---|---|
| `contenteditable` attribute | null | null | PASS |
| `input`/`textarea` inside node | false | false | PASS |
| Any editable descendant | false | false | PASS |
| Node text content | unchanged | unchanged | PASS |

Double-clicking a node does not make it editable.

**Result: PASS**

---

## Issues raised this round

None. All 7 test cases passed. The 4 review issues (REV-001..REV-004) were already marked `verified` in `review/issues.json` before this test round and are not re-examined here (they were verified in review round 3).

**`test/issues.json`**: 0 entries, track=test, round=1.

---

## Done-bar check (Stage 1)

Per `test-strategy.md` / `plan.md`:

| Bar | Status |
|---|---|
| `node --check` on every JS | PASS (10/10) |
| Rich render: token-styled node cards, owned edge SVG layer | PASS |
| Drag a node → edges re-route live | PASS |
| Minimap present + viewport indicator | PASS |
| Zoom / fit / reset-layout work | PASS |
| Positions persist to localStorage, reload keeps arrangement | PASS |
| Export layout button → valid D6 JSON download | PASS |
| sequenceDiagram → falls back cleanly, pan/zoom works | PASS |
| All offline (no external network requests) | PASS |
| Console clean (no uncaught errors) | PASS |
| No label editing on double-click | PASS |

**Verdict: GREEN. Stage 1 (FR1–FR6) is complete. No open or new issues. Ready to advance to phase ⑤ (report).**
