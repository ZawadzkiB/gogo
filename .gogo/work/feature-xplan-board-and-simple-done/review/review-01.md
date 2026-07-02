# Review — round 1 (Stage A) — feature `xplan-board-and-simple-done`

**Scope:** Stage A only (FR1/FR2/FR3). Reviewed the working-tree diff on top of
`47d872f` (v0.9.0): `skills/gogo-done/SKILL.md` (heavy rewrite), `skills/gogo-view/SKILL.md`,
`commands/done.md`, `commands/view.md`, and the deletion `D assets/kanban/board.py`.
Stage B (the `/gogo:xplan` React board + the FR8 doc/version sweep) has intentionally
not run yet.

Reviewed against: `plan.md` (accepted), `decisions.md`, `code-review-standards.md`,
`non-functional-requirements.md`.

## Verdict: **APPROVE** (Stage A) — no open blockers or majors

Two minor findings, both agent-fixable; neither blocks Stage A. Plus one info-level
reminder about the Stage-B-owned doc sweep (not a Stage A finding — see below).

## What checks out

- **Writer is byte-identical (0.8.0 regression guard holds).** Extracted the whole
  `## Write changelog entry (1..N members)` section from both `47d872f` and the working
  tree — **122 lines each, `diff` reports no changes**. The 1-vs-N members shape, the
  date/name derivation, the slim `report.md`+`.mmd`+`manifest.json`+`before/` assembly,
  and `members[]` semantics are untouched, exactly as the plan requires. (The two bare
  "the board" mentions inside it — lines 99 and 188 — are inside that untouched block;
  they are forward-references satisfied when Stage B reintroduces the browser board, so
  leaving them is correct and should NOT be edited.)
- **The separate-vs-merged gate is completely gone.** `grep` for
  `separate-vs-merged` across the four Stage-A files → no matches. List mode Step 4 is
  explicit: 0 picks → stop, 1 → single entry, ≥2 → one merged entry (release-name
  suggest+confirm via the writer), "no extra merge-or-split question — multi-select IS
  the merge signal." No orphaned references to the old gate, the old Step 5, `s`/`m`
  keys, or intents.
- **No TUI residue in the Stage-A files.** `grep -Ei
  'tmux|board\.py|board-intent|curses|cockpit|resources/kanban'` across all four files →
  no matches. The only `python3`/`tty` mention left is the degradation line asserting
  none are required. The `Inputs/outputs` table dropped every kanban row; only the
  in-memory work-index (list mode) + viewer-asset rows remain.
- **List mode is walkable end-to-end.** classify (Step 1) → four-class context table
  (Step 2) → filter (Step 3) → multi-select (Step 4) → writer. Args route correctly:
  `<slug>` → single, `a+b+c` (all real) → merge, empty → list, bare non-resolving →
  filter. Degradation is now only zero-features / zero-ready — both stop cleanly with
  actionable guidance, no silent no-op.
- **gogo-view filter.** Explicit target that resolves to nothing (`<slug>:plan` /
  `<slug>:report` / `<date>-<name>` / path) → STOP preserved; a bare non-resolving word
  → filter (not stop), applied consistently in validate-in and © Step 1; the pre-existing
  arg-grammar table is unchanged; >4 enumerated items → filter question first. The
  explicit-vs-bare split is crisp enough for an agent to apply (a `:`-selector,
  date-prefixed entry, or path = explicit → STOP; a plain word = bare → filter).
- **Thin commands match their skills.** `commands/done.md` and `commands/view.md`
  descriptions/bodies mirror the list + filter flow; no board/cockpit/tmux wording.
- **Deletion hygiene.** `assets/kanban/` is gone (`D assets/kanban/board.py`); nothing
  in the four Stage-A files references the deleted path.
- **NFR.** Stage A adds no dependencies (it removes the `python3`+`tmux` soft-dep usage
  from gogo-done); list mode writes nothing until the `.gogo/`-only writer; the shared
  classifier (`gogo-status`) is not in the diff.

## Findings

| id | sev | pri | status | title |
|---|---|---|---|---|
| REV-001 | minor | P2 | new | `+`-joined arg with a bad/unknown part silently degrades to a no-match filter instead of a clear STOP (error-handling regression) — **AGENT-FIXABLE** |
| REV-002 | minor | P3 | new | Text filter narrows only once — >4 items still matching leaves the AskUserQuestion over its stated 4-option capacity (gogo-done + gogo-view) — **AGENT-FIXABLE** |

### REV-001 — partial `+`-merge regresses to a confusing empty filter (minor, P2)
`/gogo:done foo+bar` where `foo` is real but `bar` is a typo/unknown slug is
underspecified: validate-in now guards the merge STOP with "(each part naming a real
feature)" and Resolve mode only merges when parts "ALL name real features", so a partial
`+` string falls through to List mode as a filter — matching nothing and reporting "no
matches for foo+bar". `47d872f` gave a clean STOP naming `bar`. **Fix:** if `$ARGUMENTS`
contains a `+`, treat it as an explicit merge list and STOP on any part that names no
feature / lacks a report (the existing message); only a bare no-`+` non-resolving arg
becomes the filter.

### REV-002 — filter doesn't guarantee it fits one question (minor, P3)
Both skills say ">4 items → ask a text filter first" (premised on a 4-option
AskUserQuestion) but only handle the zero-match outcome; a filter term still matching 5+
items leaves an over-capacity multi-select — the exact condition the filter was meant to
prevent. Partly a plan-level gap (FR2/FR3 describe one filter step). **Fix:** loop the
narrow — if the filtered set still exceeds one question, re-ask for a tighter filter (or
show the first N and offer to refine). Apply symmetrically in gogo-done Step 3 and
gogo-view © Step 1.

## Info reminder (NOT a Stage A finding — Stage B owns it)

The FR8 sweep is deferred to Stage B by design, so the following still describe the
removed TUI and are **out of Stage-A scope** — flagged here once so the Stage B review
verifies the sweep, per `code-review-standards.md` #1 (all of `docs/*.md` must be
enumerated) and #2 (version bump):
- `.claude-plugin/plugin.json` still `0.9.0` (plan bumps to `0.10.0` at Stage B).
- Stale cockpit / `board.py` / tmux / `resources/kanban` wording in `README.md`,
  `docs/architecture.md`, `docs/commands.md`, `docs/flow.md`, `docs/index.md`,
  `skills/gogo/SKILL.md`, `skills/gogo-status/SKILL.md`, `.gitignore` (the board.py
  bytecode comment), and `.gogo/knowledge/non-functional-requirements.md`.

These are expected interim state for a Stage-A-only round; the two stages ship together
as `0.10.0` and this review re-runs after Stage B.
