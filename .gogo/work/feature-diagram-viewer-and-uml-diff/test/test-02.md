# Test round 02 — Stage 2 (FR7 / FR8 / FR9)

**Feature:** diagram-viewer-and-uml-diff  
**Stage tested:** Stage 2 — before/after UML compare (FR7 before-set, FR8 report copy + path rewrite, FR9 compare mode)  
**Date:** 2026-07-01  
**Tester:** gogo-tester (automated + live browser)  
**Verdict:** GREEN — all four test cases pass; no open issues

---

## Levels exercised

- **Structural (artifact)** — skill docs, schema, and template inspected for contract compliance.
- **CLI / Bash** — path-depth math verified; schema validation via Python; `node --check` on all JS.
- **UI / Browser (live)** — Playwright MCP navigated `sample-compare.html` over a local HTTP server (port 7421, `http://127.0.0.1`). Screenshots captured.

> Note: `file://` is blocked by the gogo-playwright MCP; a `python3 -m http.server` workaround (rooted at `.gogo/resources/`) was used. This is equivalent — all assets served from disk with no external network.

---

## TC1 — FR7 before-set conventions (structural)

**What was checked:**
- `skills/gogo-mermaid/SKILL.md` specifies the "before" baseline at `charts/before/<kind>.mmd` + `charts/before/manifest.json`, reusing `charts-manifest.schema.json` with no added fields (D5).
- `skills/gogo-plan/SKILL.md` step 4 explicitly invokes the before-set at plan phase ①.
- `charts/before/diagrams.html` path depth: `GOGO_MERMAID_SRC` = `../../../../resources/mermaid.min.js` (4× `../`). Path math verified: from `.gogo/work/feature-<slug>/charts/before/` go up 4 levels to reach `.gogo/`, then `resources/mermaid.min.js`. Correct.
- `templates/contracts/charts-manifest.schema.json` git diff against HEAD: empty (schema unchanged). No `role` or `before` fields added.

**Result: PASS** — all conventions correct; schema D5-compliant.

---

## TC2 — FR8 report copy + path rewrite (REV-005 fix dogfood)

**What was checked:**

1. `skills/gogo-knowledge/SKILL.md` step 2 documents the copy step: "Copy the plan-time 'before' set into the bundle (FR8)" — copy `*.mmd` + `manifest.json` from `charts/before/` into `report/before/`, then **rewrite each `file` to point at the copied location** (`report/before/<kind>.mmd`). Confirmed on lines 95–106.
2. A scratch fixture was built:
   - `charts/before/manifest.json` → `file: "charts/before/flow.mmd"` (the dangling reference form)
   - After copy+rewrite → `report/before/manifest.json` → `file: "report/before/flow.mmd"`
3. Validation:
   - `grep -i "charts/" report/before/manifest.json` → no output (PASS: no dangling refs)
   - Manual field check against `charts-manifest.schema.json`: `slug`, `diagrams[]`, `kind` ∈ enum, `file` ends `.mmd`, `title` non-empty — all pass.
4. `templates/report.template.md` has a "Before / after comparison" section at line 65. It explicitly says "side-by-side + prose only — no computed node-diff (decision D4)."
5. No-before case: `gogo-knowledge/SKILL.md` line 104–106 says "If there is **no** `charts/before/`... note that and produce only the after set." Graceful handling confirmed.

**Verdict on REV-005 fix:** PASS — the skill correctly documents path rewriting; the archived `report/before/manifest.json` no longer references `charts/`; the schema is unchanged.

---

## TC3 — FR9 compare mode (live Playwright — browser-driven)

**Server:** `python3 -m http.server 7421` rooted at `.gogo/resources/`  
**URL:** `http://127.0.0.1:7421/view/sample-compare.html`

### Layout checks (wide viewport — default browser width)

```
.compare row 0:  grid-template-columns: 394px 394px  (2 columns, 1fr 1fr)
  figure.diagram.compare-before  data-diagram="before-flow"  left=194 width=394
  figure.diagram.compare-after   data-diagram="flow"         left=612 width=394

.compare row 1:  grid-template-columns: 394px 394px
  figure.diagram.compare-solo    data-diagram="class"        left=194 width=812  (full-width)
```

Two-column Before | After side-by-side for matched kind (flow): PASS  
Full-width `compare-solo` for unmatched kind (class, after-only): PASS

### Rendering per pane

| pane | data-diagram | rich (.gogo-world) | fallback (.canvas) | nodes | edges |
|---|---|---|---|---|---|
| Before — Flow | `before-flow` | yes | no | 2 | 2 |
| After — Flow | `flow` | yes | no | 4 | 4 |
| Added — Class | `class` | no | yes | — | — |

Flowchart panes → rich renderer; classDiagram → fallback canvas. Correct per D1/D3.

### Distinct persistence keys

`before-flow` and `flow` are separate `data-diagram` values → separate `gogo-view:layout:before-flow` and `gogo-view:layout:flow` localStorage keys. Each pane has its own independent `.gogo-world` DOM subtree (confirmed: node counts differ, 2 vs 4). Moving a node in one pane does not affect the other's DOM or position state.

### Narrow viewport (480px — stacked)

```
grid-template-columns: 432px  (1fr — single column)
All figures:  left=24, width=432  (stacked, same horizontal position)
```

Stacks to one column at < 720px: PASS (media query `@media (max-width: 720px)` fires).

### Network requests

All 8 requests served from `127.0.0.1:7421` only:
1. `/view/sample-compare.html` → 200
2. `/viewer/viewer.css` → 200
3. `/mermaid.min.js` → 200
4. `/viewer/geometry.js` → 200
5. `/viewer/viewport.js` → 200
6. `/viewer/mermaid-parse.js` → 200
7. `/viewer/render.js` → 200
8. `/viewer/interactive.js` → 200

Zero external (`http(s)://`) requests: PASS

### Console errors

One error only: `favicon.ico` 404 — browser auto-request, not a page error. Zero application errors.

### Screenshots

- Wide two-column: `.gogo/work/feature-diagram-viewer-and-uml-diff/test/compare-wide.png`
- Narrow stacked: `.gogo/work/feature-diagram-viewer-and-uml-diff/test/compare-narrow.png`

---

## TC4 — No node-diff / no JS change (D4=A)

- `grep` over all `assets/viewer/*.js` for `nodeDiff`, `diffNode`, `computeDiff`, `addedNode`, `removedNode`, `changedNode` → zero matches. PASS.
- `node --check` on all five viewer JS files:
  - `geometry.js` → PASS
  - `interactive.js` → PASS
  - `mermaid-parse.js` → PASS
  - `render.js` → PASS
  - `viewport.js` → PASS
- `interactive.js` processes each `figure.diagram` independently (no cross-figure state); compare mode is "pure markup — two figures per row, no renderer change" (confirmed by code inspection).

---

## Issues this round

None. `test/issues.json` round bumped to 2; `issues: []` (zero open/new).

---

## Done-bar verdict

| Bar | Status |
|---|---|
| Build (no compile step — `node --check` all JS) | PASS |
| Unit / integration suite (none — markdown plugin; dogfood only) | N/A |
| Structural: skill docs specify FR7/FR8 conventions correctly | PASS |
| FR7: before-set at `charts/before/`, 4× `../` depth, schema unchanged | PASS |
| FR8: path rewrite confirmed; no dangling `charts/` refs in `report/before/manifest.json` | PASS |
| FR8: `templates/report.template.md` has "Before / after comparison" section | PASS |
| FR8: no-before case handled gracefully | PASS |
| FR9: live browser — two-column `.compare` side-by-side (wide) | PASS |
| FR9: live browser — `compare-solo` full-width for unmatched kind | PASS |
| FR9: live browser — stacked single-column at < 720px | PASS |
| FR9: independent pane interaction (distinct `data-diagram`, separate DOM trees) | PASS |
| FR9: zero external network requests | PASS |
| FR9: zero real console errors | PASS |
| D4=A: no computed node-diff logic | PASS |
| D5: `charts-manifest.schema.json` unchanged | PASS |

**Overall: GREEN. Stage 2 implementation is complete and correct.**
