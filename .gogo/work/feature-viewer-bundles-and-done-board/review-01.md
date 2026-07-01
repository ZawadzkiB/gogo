# Review — round 01 · `viewer-bundles-and-done-board` (Stage A)

**Scope reviewed:** Stage A only — FR1 (`/gogo:view` grouped menu), FR2 (plan
viewable **in place**, D1=A), FR3 (legibility: authoring guidance + viewer CSS,
D4=A), and the shared **work-index classifier** (`gogo-status`). Stage B (FR4
kanban) and FR5 (docs/version sync) are out of scope and not flagged as missing.

**Files:** `skills/gogo-view/SKILL.md`, `skills/gogo-plan/SKILL.md`,
`skills/gogo-knowledge/SKILL.md`, `assets/viewer/viewer.css`,
`skills/gogo-status/SKILL.md` (new), `commands/status.md`, `.gitignore`.
Evidence artifact: `.gogo/resources/view/viewer-bundles-and-done-board-plan.html`.
(`assets/viewer/mermaid-parse.js` is the pre-existing label-wrap fix — confirmed
intact, not re-reviewed.)

## Verdict: **CHANGES** (blocking: REV-001)

Core Stage A is correct and verified — no blockers. One **major** (a stale
command surface) plus minors/nits keep it from a clean approve.

| Severity | Count |
|---|---|
| blocker | 0 |
| major | 1 |
| minor | 2 |
| nit | 2 |

All 5 findings are **AGENT-FIXABLE**; none need a user decision.

## What was verified good

- **Arg grammar & enumeration (FR1).** Grouped Work (plan + report per feature) /
  Changelog (report) enumeration, newest-first; the `AskUserQuestion` menu fires
  only when no arg resolves; the grammar (`<slug>` = report-else-plan,
  `<slug>:plan`, `<slug>:report`, `<date>-<slug>`, path) is unambiguous and its
  STOP cases match validate-in, which now accepts a `plan.md`-only feature.
- **Plan-in-place rendering (FR2 / D1=A).** `plan.md` is not moved; the evidence
  page renders `plan.md` + `charts/*.mmd` + `charts/before/*.mmd` (compare mode)
  through the same renderer, output `…-plan.html` at the same depth as a report
  page (`../mermaid.min.js`, `../viewer/*`), title `gogo — <slug> (plan)`, no
  `http(s)://` (offline). Inline plan mermaid fence correctly excluded from the
  summary.
- **Work-index classifier (`gogo-status`).** The four classes derive correctly and
  unambiguously with first-match precedence (shipped → ready-to-ship → in-progress
  → unfinished): `status: shipped`/changelog ⇒ shipped; report-not-shipped ⇒
  ready-to-ship; mid-loop no-report ⇒ in-progress; plan-only ⇒ unfinished;
  `aborted` ⇒ unfinished (flagged). Output shape is well-defined and reusable by
  Stage B; `/gogo:status` stays read-only (no `Write` tool, explicit "modify
  nothing"); `commands/status.md` is thin and delegates.
- **Invariants.** `${CLAUDE_PLUGIN_ROOT}` for assets; only writes under `.gogo/`;
  new `gogo-status` frontmatter valid; `.gogo-node` still `width:max-content` +
  `overflow-wrap:break-word` (label-wrap fix intact); `.summary` typography is
  sound and doesn't collide with the rich-renderer rules; FR3 changes are
  legibility-only (no section add/remove).

## Findings

### REV-001 — commands/view.md out of sync with new gogo-view behavior · **major** · P1 · new · AGENT-FIXABLE
The skill now offers a grouped plans+reports menu and `<slug>[:plan|:report]`
grammar, but `commands/view.md` still says "a gogo report", has
`argument-hint: "[changelog-entry | feature-slug]"` (no plans, no `:plan`/`:report`),
and embeds a stale reports-only 4-step flow (also re-introducing flow logic into a
command, against the thin-command rule). This is the FR1 command surface, not an
FR5 doc — and its sibling `commands/status.md` **was** updated this stage, so the
omission reads as an oversight. Per code-review-standards, an enumeration left out
of sync is Major.
**Fix:** broaden the description to "plan or report"; set argument-hint to
`[feature-slug[:plan|:report] | changelog-entry | path]`; slim the body to a thin
delegation mirroring the new `commands/status.md`, without restating the
enumeration logic.

### REV-002 — skills/gogo/SKILL.md "View" section still reports-only · **minor** · P2 · new · AGENT-FIXABLE
The orchestrator's `### View` blurb (~line 172) still says "Lists reports from
`.gogo/changelog/` and `.gogo/work/*/report/`" with no plans / grouped menu.
coding-rules names `skills/gogo/SKILL.md` a mandatory sync target (distinct from
the FR5-deferred README/docs).
**Fix:** add one or two sentences noting the grouped Work/Changelog picker and
plan-in-place viewing (D1=A).

### REV-003 — plan page renders multi-line list items as broken lists · **minor** · P2 · new · AGENT-FIXABLE
In the evidence page, each soft-wrapped Markdown list item is split: ordered lists
restart numbering per item, and wrapped continuation lines become `<p>` with the
source's literal leading/double spaces — degrading the flagship FR3 page. Root
cause: gogo-view Step 3 "Summary → HTML" gives no guidance on coalescing
soft-wrapped continuation lines. (CSS typography itself is fine.)
**Fix:** add guidance to join soft-wrapped continuation lines into their `<li>`/`<p>`
(collapse whitespace; blank line starts a new block; keep consecutive markers in
one `<ol>`/`<ul>`), then regenerate the sample page.

### REV-004 — unplanned `.gitignore` change (ignore `roadmap.md`) · **nit** · P3 · new · AGENT-FIXABLE
Adds `roadmap.md` to `.gitignore`; unrelated to Stage A and not in the plan.
Brushes the "don't auto-edit .gitignore" bar (that bar targets user projects, so
not a hard violation) but hurts diff hygiene / plan fidelity.
**Fix:** drop from this diff and land as a separate housekeeping commit, or note it
in the plan/report.

### REV-005 — gogo-view H1 / opening line still say "reports" only · **nit** · P3 · new · AGENT-FIXABLE
The skill's H1 (line 16) and first sentence (line 18) still frame it as report-only,
contradicting the new "Plans view in place too (D1=A)" paragraph just below.
**Fix:** reword the H1 to "plans & reports" and generalize the opening sentence.

---
*Contract: `review/issues.json` (round 1). This markdown is the rendered snapshot.*
