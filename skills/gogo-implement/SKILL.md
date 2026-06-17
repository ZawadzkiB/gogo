---
name: gogo-implement
description: >-
  Phase ② of the gogo pipeline — implement an accepted plan, and re-enter to
  apply review/test fixes. The operating manual for the gogo-developer agent.
  Builds only what the accepted plan describes; keeps the build and tests green.
---

# gogo-implement — phase ② (build the accepted plan)

This skill is the operating manual for the **`gogo-developer`** agent (and for
the orchestrator when it implements in-context). You are the *implementer* — the
coordination and the decision gates belong to the orchestrator.

## Preconditions
- `state.md` status is `plan-accepted` (first pass) or you're applying fixes
  (re-entry from review/test).
- Read: `plan.md` (the contract), the latest `review-NN.md`/`test-NN.md` when
  fixing, `coding-rules.md`, `tech-stack.md`.

## Steps
1. Work the `plan.md` **Changes checklist** in order, scoped to the plan. When
   applying fixes, address exactly the findings in the latest review/test report.
2. Follow `coding-rules.md`; match surrounding code. Smallest correct change; no
   opportunistic refactors outside the plan.
3. Keep it green: run build / typecheck / unit (commands from `tech-stack.md`)
   and fix what you break. Don't leave the tree broken.
4. Small, obvious plan corrections → make them and note in `plan.md`. A
   **material** change, a new fork, or anything destructive/irreversible →
   **don't decide it**: return it as a decision for the orchestrator (it owns the
   gate), with enough context to log to `decisions.md`.
5. Commit only if the user has asked for commits (gogo defers to the user on
   commits). If committing, use small safe increments.
6. Update `state.md`: phase=implement, status=implementing, bump
   `iterations: implement=<n+1>`.

## Return
A concise summary: what you changed (files), what's green, and anything you
couldn't decide (forks to escalate). Hand back to the orchestrator → review.

## Degradation
If `git` is unavailable, track touched files via the plan's Changes checklist so
the review phase still has a scope to work from.
