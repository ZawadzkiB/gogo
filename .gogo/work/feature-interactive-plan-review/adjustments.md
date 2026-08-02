# Adjustments — feature `interactive-plan-review`

Log of changes / clarifications requested during planning (and, later, at the UAT
gate). Each entry: date · what changed · why. The plan above is kept current; this
is the running history.

## 2026-07-31 — first review round (plan stays awaiting acceptance)

- **The plan is deliberately parked at `awaiting-plan-acceptance`.** The user reviewed
  it and answered every decision, but wants to start implementation only after the
  in-progress gogo item (`feature-notify-only-at-user-gates`) is finished. No
  acceptance recorded yet; accept later via `/gogo:accept interactive-plan-review` or
  the board.
- **Tool renamed `planpad` → `xplan`** (user's choice, D3): `skills/xplan/` across
  `plan.md`, `decisions.md`, and all `charts/` files (labels + node ids + layouts kept
  in sync).
- **D1 overridden — direct save, not download + drop-in:** "Send review" writes
  `plan-comments.json` straight into the plan's own folder (work folder for work
  items, project folder for `~/.gogo` project plans) via a one-time File System
  Access folder grant; the named Blob download + printed destination becomes the
  fallback, and the local server stays a follow-up. Revised: FR8, Approach §2, the
  alternatives table, Slice 3 item 19, the TL;DR, the plan flowchart edge, and
  `charts/sequence.mmd`.
- **D2, D4–D8 resolved as recommended** (sibling `plan-detail.md` · per-diagram +
  per-node comments · hash+quote anchoring · one comments file · no new slash
  command · open comments advisory-only). All eight decisions now carry RESOLVED
  blocks in `decisions.md`.
