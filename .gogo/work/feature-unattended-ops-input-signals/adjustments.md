# Adjustments — feature `unattended-ops-input-signals`

Log of changes / clarifications requested during planning (and any UAT re-plan
deltas later). Newest at the bottom.

## 2026-07-11 — add a board "accept plan" action (Slice C)

**Requested (user, at the plan-acceptance gate):** the cockpit can *display* a plan
waiting on the user (Slice B) but there is **no way to accept a plan from the board** —
`m` on an `awaiting-plan-acceptance` card launches `/gogo:go`, which refuses (needs
`plan-accepted`), a dead end. Fold the missing action into this feature so the board
both **shows** and **lets you clear** the plan-acceptance gate — completing the control
surface (UAT already has `d` ship; decisions have `a` attach; plan-acceptance had nothing).

**Delta:** add **Slice C** — a board action to accept an `awaiting-plan-acceptance` card.
Respect the "CLI never mutates pipeline state" invariant: the board **launches** the
acceptance in a claude session (likely a thin new `/gogo:accept <slug>` command that
presents the plan and flips `state.md` → `plan-accepted`), not a blind state-flip from a
keypress. Analyst to design the exact mechanism + key + whether it chains into `/gogo:go`.
Deferred notification hooks unchanged.
