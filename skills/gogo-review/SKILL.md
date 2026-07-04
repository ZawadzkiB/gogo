---
name: gogo-review
user-invocable: false
description: >-
  Phase ③ of the gogo pipeline — review the implementation against the project's
  code-review standards and non-functional requirements; emit the living, typed
  issues.json (the contract) and render a review-NN.md snapshot; loop fixable
  findings back to implement, or stop for a user decision. Delegates to the
  gogo-reviewer agent for fresh-eyes review.
---

# gogo-review — phase ③ (review, then route)

The orchestrator runs this as the **router**; the actual review is done by the
**`gogo-reviewer`** agent (fresh context = unbiased eyes — it didn't write the code).
This phase is **idempotent**: re-running it after fixes updates the same living
`issues.json` in place — "review after fixes" is just re-running `review`.

## Inputs (declared) and outputs (typed)

| Direction | Artifact | Contract |
|---|---|---|
| in (required) | `plan.md` | prose contract (accepted) |
| in (required) | `code-review-standards.md`, `coding-rules.md`, `non-functional-requirements.md` | knowledge docs |
| in (optional) | `charts/manifest.json` + the `.mmd`s | `charts-manifest.schema.json` |
| in (optional) | existing `review/issues.json` | `issues-list.schema.json` |
| out | `review/issues.json` (living) | `issues-list.schema.json` |
| out | `review-NN.md` (snapshot) | rendered markdown |
| out | `review/result.json` (per run) | `phase-result.schema.json` |

## ① validate-in (gate — FR2)

Via `gogo-contracts`: confirm `plan.md` exists and `state.md` is past
plan-acceptance; if `charts/manifest.json` or a prior `review/issues.json` is
present, validate each against its schema (right slug, real paths, unique ids,
valid enums). Any required input missing/invalid → **STOP** with a precise
contract error; do not review on bad input.

## ② Steps

1. **Delegate** to `gogo-reviewer` via `Task`, passing:
   - the diff scope (changed files / `git diff` against the base branch),
   - the feature's `plan.md` (so review is against intent),
   - the as-built `charts/` (the diagram set implement emitted, when present),
   - the current `review/issues.json` (so prior findings are tracked, not
     re-raised), and the next round number `NN`.

   The reviewer reads `code-review-standards.md`, `coding-rules.md`, and
   `non-functional-requirements.md` and produces its findings.

   **Append the round-open event (telemetry).** As round `NN` opens, append one
   compact JSON line to `.gogo/work/feature-<slug>/events.jsonl` per
   `events.schema.json` (`${CLAUDE_PLUGIN_ROOT}/templates/contracts/`):
   `{"ts":"<RFC3339>","event":"round-opened","phase":"review","status":"reviewing","round":NN,"slug":"<slug>"}`.
   Create the file if absent; **best-effort** — never fail the phase if the append
   fails (append-only telemetry; `state.md` stays the human resume file).
2. **Update the living `review/issues.json`** (the contract — D1/D2). For this round:
   - **New finding** → append an issue with a fresh stable `id` (e.g. `REV-007`),
     `origin: review`, `found_in_round: NN`, `status: new`, and all FR4 fields
     (title, description, proposed_solution, severity, priority).
   - **Prior `fixed` issue that the fix resolved** → set `status: verified`.
   - **Prior `fixed` issue that the fix did NOT resolve** → set back to `open`
     (this counts toward its ~3-round bound).
   - **Prior `open`/`new` still unaddressed** → leave `open`.
   - Never renumber or reuse an id; resolved issues stay for the audit trail.
   - Bump the file's top-level `round` to `NN` and `updated` to today.

   **If this round has any `open`/`new` findings, append the findings event**
   (best-effort, per `events.schema.json`):
   `{"ts":"<RFC3339>","event":"issues-found","phase":"review","status":"reviewing","round":NN,"note":"<e.g. 2 blockers, 1 minor>","slug":"<slug>"}`.
3. **Render the human snapshot** `review-NN.md` from this round's issues (the
   audit view): per finding, its id, severity/priority, status, the finding and
   proposed fix; plus the verdict (clean vs has-open). The JSON is the contract;
   the markdown is the readable companion.

## ③ validate-out (gate — FR3)

Via `gogo-contracts`: validate `review/issues.json` against
`issues-list.schema.json` (structural + semantic). Repair once on failure; if it
still fails, write `review/result.json` with `status: blocked`,
`validated_out: false` and stop. On success, write `review/result.json`
(`phase: review`, `status: ok`, `inputs`, `outputs`, `validated_in: true`,
`validated_out: true`, `open_issues: <count of open/new>`, `summary`).

## ④ Route

Decide purely on the **issues list** (count of `open`/`new`):
- Any `open`/`new` blockers/majors (batch the minors) → back to **② implement**
  with `--issues review/issues.json`, then **re-review** (new round, same living
  list). Bound: if the same `id` survives ~3 rounds, escalate it as a decision.
- Any finding tagged needs-user-decision → **decision gate**: log to
  `decisions.md`, set `state.md` `waiting-for-user` (resume: review), stop and ask.
- **Clean** (no `open`/`new` blockers/majors) → set `state.md` review done;
  advance to **④ test**.

Update `state.md`: phase=review, status=reviewing, bump `iterations: review=<n+1>`
each round. (`issues.json`/`result.json` are the machine state; `state.md` stays
the human-facing file.)

**Append the terminal event (telemetry).** Only when this round is **clean** (no
`open`/`new` blockers/majors — review is done and advancing to ④ test), append one
compact JSON line to `.gogo/work/feature-<slug>/events.jsonl` per
`events.schema.json` (`${CLAUDE_PLUGIN_ROOT}/templates/contracts/`) — this skill
owns `phase-done`/review (the orchestrator no longer emits it):
`{"ts":"<RFC3339>","event":"phase-done","phase":"review","status":"reviewing","slug":"<slug>"}`.
A round that loops back to implement or opens a decision gate is **not** a
`phase-done`. Best-effort — never fail the phase if the append fails.

## If browser/agent delegation is unavailable

Run the `gogo-reviewer` review steps yourself in-context against the same
standards, then update `review/issues.json` + render `review-NN.md` and route as
above. The contract and the gates are identical whether delegated or in-context.
