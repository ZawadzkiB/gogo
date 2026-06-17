---
name: gogo-review
description: >-
  Phase ③ of the gogo pipeline — review the implementation against the project's
  code-review standards and non-functional requirements; loop fixable findings
  back to implement, or stop for a user decision. Delegates to the gogo-reviewer
  agent for fresh-eyes review.
---

# gogo-review — phase ③ (review, then route)

The orchestrator runs this as the **router**; the actual review is done by the
**`gogo-reviewer`** agent (fresh context = unbiased eyes — it didn't write the code).

## Steps
1. **Delegate** to `gogo-reviewer` via `Task`, passing:
   - the diff scope (changed files / `git diff` against the base branch),
   - the feature's `plan.md` (so review is against intent),
   - the output path `review-NN.md` (the next round number).

   The reviewer reads `code-review-standards.md`, `coding-rules.md`, and
   `non-functional-requirements.md` and writes its findings.
2. **Read `review-NN.md`.** Each finding is tagged `AGENT-FIXABLE` or
   `NEEDS-USER-DECISION`, with a severity (blocker/major/minor/nit) and a verdict.
3. **Route:**
   - Any `AGENT-FIXABLE` blockers/majors (and batch the minors) → back to
     **② implement** with those findings, then **re-review** (new round). Bound:
     if the same finding survives ~3 rounds, escalate it as a decision.
   - Any `NEEDS-USER-DECISION` → **decision gate**: log to `decisions.md`, set
     `state.md` `waiting-for-user` (resume: review), stop and ask the user.
   - **Clean verdict** (no blockers/majors) → set `state.md` review done; advance
     to **④ test**.
4. Update `state.md`: phase=review, status=reviewing, bump
   `iterations: review=<n+1>` each round.

## If browser/agent delegation is unavailable
Run the `gogo-reviewer` review steps yourself in-context against the same
standards, write `review-NN.md`, then route as above.
