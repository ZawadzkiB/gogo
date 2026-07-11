# unattended-ops-input-signals - v0.14.0

- **shipped:** 2026-07-11
- **work:** [.gogo/work/feature-unattended-ops-input-signals/](../../work/feature-unattended-ops-input-signals/) (full audit trail: plan, decisions, review/test rounds, per-file changes)
- **review:** APPROVE (2 rounds) · **test:** all-green, zero code defects

Three linked fixes that make the gogo pipeline run **unattended** and make its
**waiting-on-you** states visible at a glance - shipped as three independently-valuable
slices at **v0.14.0** (12 -> **13** commands).

`/gogo:done` used to halt on a **false "dangerous rm" permission prompt**: gogo's own
mechanical file steps matched Claude Code's glob-`rm` / bare-variable-`rm` classifier, so
an "unattended" ship still nagged the user. **Slice A** rewrites that skill-bash to a
classifier-safe idiom so the mechanical steps run prompt-free. **Slice B** adds a single
`WaitingForInput()` predicate and surfaces it in three read-only places so it's obvious
which items block on the user. **Slice C** adds a board **accept** action so a
plan-pending card the board now *shows* can also be *cleared* from the board - completing
the control surface (`d` ships, `a` attaches, and now `m` accepts).

## What changed

- **Slice A - classifier-safe skill bash.** The two `rm` sites in `gogo-done` were
  rewritten to a **guarded scoped-`find ... -delete`** idiom (guard `$dst` non-empty *and*
  under `.gogo/changelog/`, then delete via `find` - no glob, no bare-variable `rm`); the
  `gogo-build` migration sites were similarly hardened. A **regression lint**
  (`cli/skills_lint_test.go` `TestSkillsBashNoUnsafeRm`) scans every `skills/*/SKILL.md`
  and fails if an unsafe `rm` shape reappears, so the fix can't erode.
- **Slice B - one predicate, three display sites.** `contract.Feature.WaitingForInput()`
  is true for exactly the three genuine user gates (`awaiting-plan-acceptance`,
  `waiting-for-user`, `awaiting-uat`). It drives a focus-independent **⏸** cue on the TUI
  board card, a dedicated **WAIT** column in `gogo status`, and **`│` borders** between the
  board's four columns. `awaiting-plan-acceptance` - which had *no* cue before - now reads
  as its own state.
- **Slice C - a thin launched `/gogo:accept`.** The board's `m` on an
  `awaiting-plan-acceptance` card now routes to a new `ActionAccept` -> a `/gogo:accept <slug>`
  session that presents the plan and records acceptance **through gogo-plan's existing
  single-owner recording**. The CLI never mutates pipeline state - only the launched
  session does - closing the dead end where `m` used to bounce into a `/gogo:go` that refuses.

## Decisions (one-liners)

- **Bash rewrite, not move-to-Go** (D1) - smallest change that removes both classifier triggers; keeps the skill dependency-free.
- **Per-card cue, not a 5th column** (D2) - a 5th column would break the frozen class->column 1:1 map; a waiting item still belongs to its phase column.
- **`awaiting-plan-acceptance` counts as waiting** (D3) - it is a genuine user gate that had no cue; that gap is what this closes.
- **Lint as a Go test** (D6) - lands in the `go test -race` gate the coding rules already require, so it can't be forgotten.
- **Reuse `m`, accept-only, via a launched session** (D7-D10) - a plan-pending card's legal move *is* accept; keeps the "CLI never mutates pipeline state" invariant and reuses the one acceptance recording.
- **Two live acceptance signals skipped** (D11/D12) - the user accepted the green lint + unit + harness/tmux evidence in lieu of a live prompt-free `/gogo:done` and a live board-accept follow-through; both are recorded `wontfix`, not silently dropped, and Slice A's prompt-free run confirms organically at ship time.

Full decision rationale in the [work folder](../../work/feature-unattended-ops-input-signals/decisions.md).

## Verdict

**Review: APPROVE** (2 rounds - round 1 found 1 major doc-sync miss + 1 nit, both
agent-fixed and re-verified in round 2). **Test: all-green** on every runnable level
(`gofmt`/`go vet`/`go test -race` across all packages incl. 8 new tests and the
regenerated `status.golden`; built binary reported `0.14.0`; the live tmux TUI confirmed
the ⏸ cue, the `│` separators, and `m`'s accept-vs-go routing). Zero code defects.

## Diagrams

The as-built UML set (rendered by the interactive viewer):

- **flow** - one `WaitingForInput()` predicate read by three read-only display sites.
- **flow** (control-surface) - the board move-guard routing each card class/status to a delegated launch; `ActionAccept` fills the plan-acceptance gap.
- **activity** - the pipeline status lifecycle: the three ⏸USER gates vs the auto transitions.
- **sequence** - the board-accept interaction end to end: `m` -> `ActionAccept` -> launched `/gogo:accept` -> gogo-plan recording -> `plan-accepted`.

A **before/** flow ships alongside (compare mode): the old two-separate-checks render path
with no waiting-for-input union and no column borders.

## Follow-ups (deferred)

CLI hooks firing desktop/OS notifications on a waiting state (`WaitingForInput()` is the
seam); accept parity on the SKILL-side fallback board (`board.py`); chaining accept into
`/gogo:go` (accept-then-go).
