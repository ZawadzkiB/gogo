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

1. **Record occupancy - FIRST, before you read the diff (FR11).** Write `state.md`
   `phase: review` + `status: reviewing`, and append
   `{"ts":"<RFC3339>","event":"phase-started","phase":"review","status":"reviewing","slug":"<slug>"}`
   to `events.jsonl`. Only then continue to step 2. §④ writes `phase`/`status` **again** at the
   end and bumps `iterations` there - deliberate belt-and-braces, not redundancy to tidy away
   (see §④). Do **not** bump `iterations` here - that is a completion count.

   `state.md` is what every reader believes (the board's column, `gogo status`, the cap,
   `pages`). Written ONLY at §④, after the work, the line records that reviewing just
   **finished** - so for the whole round the disk still describes the PREVIOUS phase and the
   board narrates the past. Idempotent on a resume: the values are already correct.

   **This is step 1 of the numbered flow on purpose** - it shipped once as a separate `①b`
   section, which was worse. Be honest about the track record though: this write has been
   skipped on **all three** of its live runs so far, twice AFTER the move into the numbered
   steps, so the move helped less than hoped. That is exactly why §④ writes `phase`/`status`
   again and why the board carries a detector: skip this and a card whose telemetry contradicts
   its phase line, with a build session still live, reads **`· state lags`**. Visible, and still
   wrong - §④'s write will eventually move the line, but every reader describes the previous
   phase until it does.

2. **Delegate** to `gogo-reviewer` via `Task`, passing:
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
3. **Update the living `review/issues.json`** (the contract - D1/D2). For this round:
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
4. **Render the human snapshot** `review-NN.md` from this round's issues (the
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

Update `state.md`: **`phase: review`, `status: reviewing`**, and bump
`iterations: review=<n+1>` - the *completion* count - each round.

**Scope: only on the routes that CONTINUE.** A route that parks the item at a **user gate**
(a decision gate → `waiting-for-user`, and for ④ a blocked hands-on check → the same) has
already written the status that matters, and it is the one status a reader must not lose:
it feeds the `⏸ K need you` count, the card's gate stripe, and the `/gogo:go` refusal that
stops an unattended rerun. **Never overwrite a gate status with the working status** - write
`phase`/`status` here only when the round loops back to ② or advances to the next phase.
(`issues.json`/`result.json` are the machine state; `state.md` stays
the human-facing file.)

**Write phase/status here EVEN THOUGH §② step 1 already did. The redundancy IS the design -
do not "clean it up".** 0.29.0 briefly dropped this exit write on the theory that the entry
write covers it. It does not: the entry write is **prose an LLM follows**, and it was skipped
on all three of its first live runs. With only the entry write, `state.md` stops advancing at
all - it sticks at whatever phase last actually wrote it, which is *worse* than the
one-phase-behind lag this release set out to fix. Two writers, one at each end, means the
floor is that old one-phase lag and the ceiling is the entry write's accuracy. One of the two
is an LLM following prose, so the other one is what makes the line move at all.


**Append the terminal event (telemetry).** The `phase-started` line was appended at §② step 1 -
do not re-emit it. Only when this round is **clean** (no
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
