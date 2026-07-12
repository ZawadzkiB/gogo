# UAT — feature `immediate-kill-at-ship`

<!-- The UAT gate log — the plan-gate symmetry at the END of the pipeline.
  Phase ⑤ (report) no longer ends at `done`; it ends at status: awaiting-uat, and you verify the work.
  There are exactly two ways forward, and both are recorded here (append-only, newest round at the bottom):

  1. ACCEPT — running `/gogo:done` IS the acceptance (no extra confirmation question, mirroring how
     accepting a plan unlocks `/gogo:go`). `/gogo:done` first appends the one-line accept verdict below,
     then ships as usual.
  2. ISSUES / QUESTIONS — you describe what's wrong or ask a question instead of shipping. The
     orchestrator hands your input to `gogo-analyst`, which analyses it against the current plan.md +
     decisions.md + THE CODE (code = source of truth) and appends an issues round below; adjustments.md
     logs the plan delta and plan.md is updated. You RE-ACCEPT the adjusted plan, then `/gogo:go` reruns
     ②→⑤ — the SAME work item, never a new one — landing back at awaiting-uat.
-->

## UAT round 1 — accepted (user, 2026-07-12) — via /gogo:done
Shipped 0.17.0: targeted, self-guarded `/gogo:done` ship→reap + dropped board `remain-on-exit`. Review APPROVE (2 rounds), test GREEN (live scratch-tmux hands-on). Accepted at the UAT gate.
