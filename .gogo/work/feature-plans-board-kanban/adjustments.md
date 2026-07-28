# Adjustments — feature `plans-board-kanban`

Log of changes / clarifications requested during planning (and, later, at the UAT
gate). Each entry: date · what changed · why. The plan above is kept current; this
is the running history.

## 2026-07-23 — scope: build the whole vision in one plan (D1=B) + resolve D2/D3/D4

At the plan-acceptance gate the user chose to build **all of FR1-FR6 in THIS one
plan/build** (not the recommended Slice-1-only cut), shipping as **0.26.0**. The four
forks were resolved (see `decisions.md`); the plan is revised so **FR5 and FR6 carry
the same depth as FR1-FR4** (full BDD, changes-checklist entries in build order,
tests). Summary of what changed in `plan.md`:

- **D1=B** — dropped the "Slice 1 now, 2/3 later" framing; the "Recommended slice
  cut" section is now **Build order** (Phase A = kanban + re-sequence · Phase B =
  done-archive · Phase C = auto-pickup), all in scope, built + tested together.
- **D2=A** — the manual move is a single board-style **`m`**; today's `r`/`D` keys
  fold into it (their tests are adapted). CLI verbs `gogo plan ready`/`go`/`done`
  kept.
- **D3=A** — **FR5 fully specified**: `MarkDone` also writes a deterministic,
  LLM-free project-changelog entry under
  `~/.gogo/projects/<name>/.gogo/changelog/<date>-<id>/` (mirrors `plans.Dir`) and
  keeps the plan in the store as `status=done` so the kanban's `done` column still
  shows it (archive = additional record, not a move-out).
- **D4=B** — **FR6 fully specified**: auto-pickup fires from the cockpit's
  fsnotify `reloadMsg` path for a **skip-source** member at **`plan-accepted`**, with
  **no live session**, **fire-once** (composite `Root\x00Slug` key). Cap two-branch:
  UNDER the source's integer `ConcurrentWorkItems` cap → auto-fire `/gogo:go`; AT cap
  → **no fire + a visible "needs manual trigger" cue on BOTH the work-item card and
  its plan card** (transient — cap-skips are NOT added to the fire-once set, so a
  freed slot on a later reload auto-fires then). Real mid-pipeline gates still park
  `waiting-for-user`.

Invariants unchanged: CLI writes only `~/.gogo/`; every spawn/pickup is a launched
`claude -p`; heap-stable `*formBinding`; no em-dash; version → 0.26.0.
