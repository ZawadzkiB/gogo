# Test — round 01 · `viewer-bundles-and-done-board` (Stage A)

**Scope:** Stage A only — FR1 (`/gogo:view` grouped menu + arg grammar), FR2 (plan
viewable in place, D1=A), FR3 (article typography), and the shared **work-index
classifier** (`gogo-status`). Stage B (FR4 kanban) and FR5 (docs/version) are out
of scope and not tested here.

**Method:** structural/dry-run for FR1 arg grammar + TC4 classifier; live-browser
(Playwright MCP, 127.0.0.1 HTTP server) for FR2 plan page + FR3 typography + TC5
label-wrap regression.

**Evidence artifact:** `.gogo/resources/view/viewer-bundles-and-done-board-plan.html`
(the sample plan page, served at http://127.0.0.1:8734/ during the session).

**Screenshot:** `plan-page-full.png` (repo root, full-page PNG of the plan page
as rendered live).

---

## Verdict: GREEN

All five test cases pass. One nit (TEST-001) is informational and non-blocking.

| Severity | Count |
|---|---|
| blocker | 0 |
| major | 0 |
| minor | 0 |
| nit | 1 |

Done-bar: build (N/A — markdown plugin, no compile step) + unit (N/A — no suite) +
e2e (HTML artifact verified live in browser) + hands-on (interactive diagrams
confirmed via Playwright MCP) — all bars met.

---

## TC1 — FR1 enumeration + arg grammar (structural / dry-run) — PASS

**Method:** read `skills/gogo-view/SKILL.md` and `commands/view.md`; dry-run
`ls .gogo/work/` + `ls .gogo/changelog/` to confirm what the menu would enumerate.

**Findings:**

- **Grouped enumeration** in SKILL.md Step 1: `ls .gogo/work/feature-*/plan.md` for
  plan bundles, `ls .gogo/work/feature-*/report/report.md` + legacy root `report.md`
  for reports, and `ls .gogo/changelog/*/` for changelog entries. Each group is
  newest-first.
- **Real repo dry-run:** 5 features × (plan + report if present):
  - `diagram-viewer-and-uml-diff` → plan + report (new bundle at report/)
  - `docs-and-verified-discovery` → plan + report (legacy root report.md)
  - `pipeline-commands` → plan + report (legacy root report.md)
  - `skill-extraction` → plan + report (legacy root report.md + new bundle)
  - `viewer-bundles-and-done-board` → plan only (no report yet)
  - `workspace-changelog-viewer` → plan + report (new bundle at report/)
  - Total: **11 Work items** + **1 Changelog item** (`2026-07-01-diagram-viewer-and-uml-diff`)
- **Arg grammar** (5 patterns): `<slug>` (report-else-plan), `<slug>:plan`,
  `<slug>:report`, `<date>-<slug>` (changelog), path — all documented unambiguously
  in the SKILL.md arg-grammar table.
- **No-arg path:** `AskUserQuestion` grouped picker fired; documented clearly in
  SKILL.md Step 1 "No resolvable arg".
- **commands/view.md** argument-hint: `[feature-slug[:plan|:report] | changelog-entry | path]`
  — matches the SKILL.md grammar exactly (REV-001 fix verified).

---

## TC2 — FR2 plan page (live browser via Playwright) — PASS

**Method:** Python `http.server` on 127.0.0.1:8734 serving `.gogo/resources/`;
navigated to `/view/viewer-bundles-and-done-board-plan.html`; assertions via
`browser_evaluate` and `browser_console_messages`.

| Check | Result |
|---|---|
| Console errors | 0 real errors (1 favicon 404 — expected) |
| `<article class="summary">` present | yes |
| h1 text | "Plan — Viewer selection menu · plan/report view-bundles…" |
| Lead standfirst (`h1 + p`) rendered | yes — 19.2px (1.2rem CSS rule firing) |
| Ordered list groups | 3 separate `<ol>` elements (Goal / Stage A / Changes checklist), each starting at 1 — semantically correct |
| Stray continuation `<p>` (REV-003 regression) | **0 stray paragraphs** — `ol > p, ul > p` query returns empty |
| Compare mode (`<div class="compare">`) | yes — 2 children (Before + After figures) |
| `figure.diagram` count | 2 |
| `.viewport` count | 2 |
| `.viewport.gogo-rich` count | **2** — both flowchart-family diagrams got the rich renderer |
| `.gogo-node` (draggable card) count | **18** |
| Node cursor | `grab` — draggable |
| `.gogo-edges` SVG layer | present |
| `.gogo-edge` path count | **16** |
| Minimap | present |
| Controls buttons (fit/zoom/reset/export per diagram) | **10** (5 per diagram × 2) |
| External network refs (`http(s)://`) | **0** — fully offline |
| Script load order | mermaid → geometry → viewport → mermaid-parse → render → interactive |
| Layout seed (`window.GOGO_LAYOUT`) | `{}` — empty (no sidecar exists yet; correct default) |

---

## TC3 — FR3 article legibility (live browser) — PASS

**Method:** `window.getComputedStyle` assertions on `.summary` elements in the live
page; visual confirmation via full-page screenshot (`plan-page-full.png`).

| Check | Result |
|---|---|
| `.summary` base color | `rgb(216, 221, 230)` — readable on dark `#0b0e14` background |
| Lead standfirst font-size | `19.2px` (1.2rem, CSS `.summary h1 + p` rule) |
| `strong`/`b` color in CSS | `#f4f6fb; font-weight: 700` — visibly pops on the article text |
| Paragraph max-width | 68ch (CSS `.summary p { max-width: 68ch }`) |
| h2 border-bottom separator | `1px solid #222a38` — clear section delineation |
| Authoring guidance in `gogo-plan/SKILL.md` | "Write it like a readable article (FR3)… Lead with a 1-2 sentence summary" |
| Authoring guidance in `gogo-knowledge/SKILL.md` | "Write it like a readable article (FR3)… Lead each section with a 1-2 sentence summary" |
| Screenshot | clean dark-background article; h1 prominent; sections delineated; bold text visible; compare diagrams at bottom |

---

## TC4 — Work-index classifier (dry-run) — PASS (3 of 4 classes live-verified)

**Method:** read `state.md`, report presence, and changelog for all 6 feature dirs;
apply the gogo-status SKILL.md classification rules (first-match, top-to-bottom).

| Feature | Rule triggered | Class |
|---|---|---|
| `diagram-viewer-and-uml-diff` | `status: shipped` + changelog entry `2026-07-01-…` | **shipped** |
| `docs-and-verified-discovery` | not shipped; has root `report.md` | **ready-to-ship** |
| `pipeline-commands` | not shipped; has root `report.md` | **ready-to-ship** |
| `skill-extraction` | not shipped; has root `report.md` + `report/report.md` | **ready-to-ship** |
| `workspace-changelog-viewer` | not shipped; has `report/report.md` | **ready-to-ship** |
| `viewer-bundles-and-done-board` | no report; `phase: test`, `status: testing` | **in-progress** |

- **shipped → ready-to-ship → in-progress** all correctly classified.
- **unfinished** class: no plan-only feature currently exists in the repo — the rule
  ("anything else" fallback) is structurally sound but has no live exemplar to
  exercise. Logged as TEST-001 (nit, informational).
- `/gogo:status` is read-only — SKILL.md confirms no Write tool, "modify nothing".
- `commands/status.md` correctly thin-delegates to the gogo-status skill.

---

## TC5 — Label-wrap fix intact (live browser regression check) — PASS

**Method:** `window.getComputedStyle` on all `.gogo-node` elements; checked
`word-break`, `overflowWrap`, `whiteSpace`, and `offsetWidth`.

| Check | Result |
|---|---|
| `word-break` on all 18 nodes | `normal` — never mid-word |
| `overflow-wrap` on all 18 nodes | `break-word` — only breaks a single oversized word |
| `white-space` on all 18 nodes | `normal` — wraps at whitespace |
| Any mid-word hyphen break observed | **none** — all 18 node labels are intact phrases |
| Observation: 6 nodes have `min-width` (from mermaid bbox) exceeding `max-width: 260px` | labels remain single-line (no wrap needed); CSS `max-width` is overridden by inline `min-width` set by the renderer from the parsed bbox — this is intentional geometry-preservation behavior, not a regression |

---

## Issues this round

| ID | Title | Severity | Priority | Status |
|---|---|---|---|---|
| TEST-001 | Work-index 'unfinished' class has no live exemplar in the current repo | nit | P3 | new |

### TEST-001 — unfinished class has no live exemplar · nit · P3 · new

TC4 verified three of the four work-index classes against real feature dirs but
found no current feature that is plan-only / early-phase (the 'unfinished' class).
The classifier rule ("anything else → unfinished") is structurally correct and is
the last-resort fallback. Non-blocking; no code change required. If future
regression coverage is desired, a fixture feature dir with only a `plan.md` and no
report would exercise this path.

---

*Contract: `test/issues.json` (round 1). This markdown is the rendered snapshot.*
