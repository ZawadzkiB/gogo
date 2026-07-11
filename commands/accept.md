---
description: Accept a plan from the board — present feature-<slug>'s plan and record acceptance (state.md → plan-accepted) so /gogo:go can run it. Only valid at the plan-acceptance gate.
argument-hint: "<feature-slug>"
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Skill, AskUserQuestion
model: opus
---

Accept a feature's plan at the **plan-acceptance gate**, via the `gogo-accept`
skill. This is the board's `m`-on-a-plan-pending-card path (Slice C): a thin,
launched acceptance that reuses gogo-plan's existing recording — the CLI itself
never mutates pipeline state.

Target: $ARGUMENTS  (the feature slug; required.)

Load `gogo-accept` and follow it:

1. **Resolve + gate** — the feature's `state.md` status MUST be
   `awaiting-plan-acceptance`. Anything else → STOP with guidance (already
   accepted → run `/gogo:go`; mid-pipeline → nothing to accept here).
2. **Present the plan** — show `plan.md`'s summary + any open decisions for the
   user to eyeball before they accept.
3. **Record acceptance on the user's confirmation** — exactly as `gogo-plan`
   does: `state.md` → `plan-accepted`, add the `Status: **accepted** (user,
   <today>)` line to `plan.md`, clear `open-decision`, and emit the single-owner
   `plan-accepted` event. Then tell the user to run `/gogo:go`. **Accept-only** —
   it does not chain into `/gogo:go` (the board's `m` on the now-accepted card is
   the natural second step).
