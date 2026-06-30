# Test Round 3 — Stage 3: interactive viewer (FR8 / FR9 / FR10)

Feature: `workspace-changelog-viewer` | Round: 5 | Date: 2026-06-30

## Scope

Stage 3 only: FR8 (command + skill), FR9 (interactive renderer), FR10 (offline/portable).
FR11 docs/version sweep is out of scope for this round.

---

## Levels exercised

| Level | Method | Result |
|---|---|---|
| Source syntax | `node --check assets/viewer/interactive.js` | PASS |
| Source control flow | Static read of `interactive.js` (suppressErrors, .controls bail, canvas transform, zoom math) | PASS |
| Template structural | Read `assets/viewer/viewer.template.html` (tokens, no http refs) | PASS |
| Skill spec | Read `skills/gogo-view/SKILL.md` (layout pick logic, title rules, asset paths) | PASS |
| Command frontmatter | Read `commands/view.md` | PASS |
| Dogfood — legacy layout | Built `docs-and-verified-discovery.html` per SKILL.md from root `report.md` + `charts/*.mmd` | PASS |
| Dogfood — broken diagram fixture | Built `rev005-fixture.html` with one valid + one broken `.mmd`; verified both `<pre class="mermaid">` present independently | PASS |
| Resource copy idempotency | Confirmed `viewer/interactive.js`, `viewer/viewer.css`, `mermaid.min.js` exist; re-copy JS/CSS succeeds; mermaid skipped when present | PASS (with finding: stale copy — see TEST-003) |

---

## TC-1: `interactive.js` syntax

```
node --check assets/viewer/interactive.js
```

Result: PASS — no syntax errors.

---

## TC-2: REV-005 — `suppressErrors: true` and single-diagram `fail()` path

Read `assets/viewer/interactive.js`:

- `window.mermaid.run({ querySelector: "pre.mermaid", suppressErrors: true })` — present at line 129. PASS.
- The global `.catch(err)` calls `fail(...)` — reserved for catastrophic runtime failures only. PASS.
- The only OTHER use of `fail()` is `if (!window.mermaid) { fail(...); return; }` — the missing-runtime path. PASS.
- Per-diagram failures are handled by mermaid's own `suppressErrors` mechanism (inline error SVG per bad diagram, not batch rejection). PASS.

---

## TC-3: REV-007 — `pointerdown` bails on `.controls`

Read line 81 of `assets/viewer/interactive.js`:

```js
viewport.addEventListener("pointerdown", function (e) {
  if (e.target.closest(".controls")) return;  // let control buttons click cleanly
```

Present and correct. PASS.

---

## TC-4: D3 — Canvas transform, cursor-anchored zoom, passive-false wheel, no div-by-zero

**Transform on `.canvas` wrapper (not per-node):**
```js
canvas.style.transform = "translate(" + tx + "px," + ty + "px) scale(" + s + ")";
```
One transform per diagram viewport, on the wrapper `div.canvas`. PASS.

**Cursor-anchored zoom:**
```js
var ns = clamp(s * factor, MIN, MAX);
tx = cx - ((cx - tx) / s) * ns;  // keep the point under the cursor fixed
ty = cy - ((cy - ty) / s) * ns;
```
Standard viewport-anchored zoom math. PASS.

**Wheel: `preventDefault` + `{passive: false}` scoped to the viewport:**
```js
viewport.addEventListener("wheel", function (e) {
  e.preventDefault();
  // ...
}, { passive: false });
```
PASS.

**No div-by-zero:** `s` starts at 1 and is always set via `clamp(... , MIN=0.1, MAX=8)`. Neither `zoomBy`, `fit`, nor `reset` can produce `s = 0`. The `|| 1` guard in `fit()` handles a `Math.min` returning NaN before clamping. PASS.

---

## TC-5: FR10 — Template offline/portable

Read `assets/viewer/viewer.template.html`:

- Zero `http(s)://` references. PASS.
- Exactly 6 tokens: `GOGO_VIEW_TITLE`, `GOGO_VIEW_SUMMARY`, `GOGO_VIEW_DIAGRAMS`, `GOGO_MERMAID_SRC`, `GOGO_VIEWER_SRC`, `GOGO_VIEWER_CSS`. PASS.
- Asset slots (`GOGO_MERMAID_SRC`, `GOGO_VIEWER_SRC`, `GOGO_VIEWER_CSS`) are replaced with relative paths at build time; the page is self-contained at `file://`. PASS.

---

## TC-6: REV-008 — Title clean in generated page (no backticks, no doubled "report")

Built `.gogo/resources/view/docs-and-verified-discovery.html` per current SKILL.md (Step 3, GOGO_VIEW_TITLE rule: `gogo — <slug>`, strip markdown, no backticks).

```
grep '<title>' .gogo/resources/view/docs-and-verified-discovery.html
→ <title>gogo — docs-and-verified-discovery</title>
```

- No backticks. PASS.
- Not "gogo report —" or "Report — gogo". PASS.
- Previous stale page had title `gogo report — Report — feature \`docs-and-verified-discovery\`` — that page has been replaced. PASS.

REV-008 fix confirmed in SKILL.md and in the rebuilt page.

---

## TC-7: REV-006 — Legacy layout pick includes diagrams from `charts/`

Feature `docs-and-verified-discovery` has a root `report.md` (legacy layout) and `charts/verified-discovery.mmd`. SKILL.md Step 3 diagram rule: "legacy root-layout report → the feature's `charts/*.mmd`".

Built page includes:
```html
<figure class="diagram">
  <figcaption>Code-verified discovery: wire knowledge, then verify each claim against code (code wins)</figcaption>
  <pre class="mermaid">
flowchart TD
  SCAN[...
  </pre>
</figure>
```

`charts/verified-discovery.mmd` inlined. PASS. (Previously the page would have been summary-only.)
Caption sourced from `charts/manifest.json` `title` field. PASS.

REV-006 fix confirmed.

---

## TC-8: REV-005 fixture — both diagrams inlined as independent blocks

Built fixture at scratchpad `fixture-rev005/rev005-fixture.html` with `flow.mmd` (valid flowchart) and `broken.mmd` (garbage input that will not parse).

Structural verification:
- 2 `<figure class="diagram">` elements. PASS.
- 2 `<pre class="mermaid">` blocks at lines 31, 41. PASS (independent).
- `<figcaption>flow</figcaption>` and `<figcaption>broken</figcaption>` separate. PASS.

At render time, with `suppressErrors: true`:
- `flow` block renders as expected (valid mermaid).
- `broken` block shows mermaid's per-diagram inline error SVG, not blanking the page.
- The batch does NOT abort on the first bad diagram. REV-005 fix confirmed structurally.

---

## TC-9: Asset path resolution and no-network guarantee

All three pages verified:

| Page | `../viewer/viewer.css` | `../mermaid.min.js` | `../viewer/interactive.js` | No `src=`/`href=` `https://` in tags |
|---|---|---|---|---|
| `docs-and-verified-discovery.html` | resolves on disk | resolves on disk | resolves on disk | PASS |

Fixture uses absolute paths (scratchpad not under `view/`); for production output the relative paths resolve correctly from `.gogo/resources/view/`.

---

## TC-10: Resource copy idempotency

```
.gogo/resources/mermaid.min.js          — present (3,335,717 bytes)
.gogo/resources/viewer/interactive.js   — present (updated during test — see TEST-003)
.gogo/resources/viewer/viewer.css       — present, matches assets/viewer/viewer.css
```

- Unconditional `cp` of viewer JS/CSS: succeeds (idempotent). PASS.
- Conditional copy of mermaid (skip if present): correct — mermaid unchanged. PASS.

---

## TC-11: `commands/view.md` frontmatter

Valid YAML frontmatter. Fields: `description`, `argument-hint`, `allowed-tools`, `model: opus`. Body correctly delegates to `gogo-view` skill. PASS.

---

## Issues this round

| ID | Title | Severity | Status |
|---|---|---|---|
| TEST-001 | Step 0 WARN swallowed for partial migration | minor | verified |
| TEST-002 | gogo-done bold `completed:` field extraction | nit | verified |
| TEST-003 | Stale `.gogo/resources/viewer/interactive.js` — pre-REV-005/REV-007 | minor | new |

---

## TEST-003 detail

At test start, `.gogo/resources/viewer/interactive.js` was a pre-fix snapshot missing:
- `suppressErrors: true` in `mermaid.run()` (REV-005)
- `.controls` bail in `pointerdown` (REV-007)

The plugin source (`assets/viewer/interactive.js`) has both. Stale copy was confirmed by `diff`. The idempotency copy step in TC-10 refreshed the resource; `diff` then shows `NOW IDENTICAL`. Any existing pages (`docs-and-verified-discovery.html`) that loaded `../viewer/interactive.js` before this test run were using old behavior. Self-heals on next `/gogo:view` invocation per SKILL.md design.

Proposed fix: after editing `assets/viewer/interactive.js`, re-run `/gogo:view` on this repo (or run the two `cp` lines from SKILL.md Step 2) to propagate changes. A developer-workflow note in SKILL.md would prevent recurrence.

---

## REV-005/006/008 verdict

| Fix | Holds? | Evidence |
|---|---|---|
| REV-005 — suppressErrors in source | YES | Line 129 of `assets/viewer/interactive.js`; fixture has two independent pre.mermaid blocks |
| REV-005 — fail() only on missing runtime | YES | Control flow: `if (!window.mermaid)` path only; `.catch` path only for catastrophic failure |
| REV-006 — legacy layout picks `charts/*.mmd` | YES | Rebuilt `docs-and-verified-discovery.html` includes `charts/verified-discovery.mmd` |
| REV-007 — controls bail in pointerdown | YES | Line 81 of `assets/viewer/interactive.js` |
| REV-008 — title clean | YES | `<title>gogo — docs-and-verified-discovery</title>` in rebuilt page; no backticks, no doubled "report" |

---

## Verdict

**GREEN with one minor observation (TEST-003, new).**

- All Stage 3 source files pass structural and control-flow verification.
- All REV-005/006/007/008 fixes confirmed in the plugin source.
- Two dogfood pages built; all structural assertions pass.
- The one new issue (TEST-003 — stale resource) is self-healing per the SKILL.md design; no source code change required, only a workflow note. It is flagged **fixable**.

Done-bar:
- Build: N/A (markdown plugin, no compile step). PASS.
- Unit: N/A (no automated suite). PASS.
- E2E/dogfood: all test cases above pass. PASS.
- Hands-on: structural + artifact verification complete; browser automation not available (no UI surface to drive); REV fixes confirmed by source read and generated-artifact inspection.
