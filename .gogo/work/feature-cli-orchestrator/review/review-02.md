# Review 02 — CLI process-orchestrator (Slice 1)

Round 2 · fresh-eyes re-review verifying the round-1 fixes (REV-001…005) and hunting
for regressions the fixes introduced. Scope: `cli/internal/orchestrator/*`,
`cli/internal/contract/{route,result}.go`, `cli/run.go`, `cli/internal/launch/*`,
plus the plugin-side `--in-session` docs, checked against `plan.md` (FR1-FR9/FR11)
and the `gogo-review`/`gogo-test` §④ routing rules.

**Gates (re-run fresh, `-count=1 -race`):** `gofmt -l .` clean · `go vet ./...` clean ·
`go test -race ./...` green. The round-2 tests assert the fixed behaviour, not just
compile: `TestReviewBatchesMinor`, `TestPhaseErrorHalts`, `TestNoOutputGates`,
`TestPreflightCostGateNoSpend`, `TestNeedsUserDecisionScansAllFields`, and the route
table's minor/nit/severity + needs-decision cases all pass.

## Round-1 findings — verification

| id | sev | round-1 | verdict |
|----|-----|---------|---------|
| REV-001 | major | routing drift on minors | **verified** |
| REV-002 | major | false green on failed/no-output phase | **verified** |
| REV-003 | major | sticky bounds / cost re-gate money sink | **verified** |
| REV-004 | minor | needs-user-decision scan too narrow | **verified** |
| REV-005 | minor | global vs per-finding round bound | **verified (accept-as-documented)** |

### REV-001 — track-aware Route (verified)
`Route(track, result, issues)` now routes through `routableOpen`: **review** counts
only open/new `blocker|major` and batches `minor`/`nit`; **test** counts any open/new.
The severity strings match the schema enum exactly (`blocker/major/minor/nit`), the
status guard matches (`open/new`), and both call sites in `orchestrator.go` pass the
correct track. The doc-comment now states the per-track difference and cites the skills
(no false parity claim); plan FR6 is corrected. This matches `gogo-review` §④ ("batch
the minors") and `gogo-test` §④ ("any open/new"). The deterministic resolution of the
round-1 NEEDS-USER-DECISION is sound — the plan's constraint 3 makes the skills the
authority. `TestReviewBatchesMinor` proves a lone open minor advances (no re-implement).

### REV-002 — no more false green (verified)
`exec` now returns an error on `RunResult.IsError`, halting the run
(`TestPhaseErrorHalts` stops after the failed phase). Before Advance, both the review
and test legs gate (`gateDecision`) when **both** `result.json` and `issues.json` are
absent (`TestNoOutputGates`) — and, importantly, a legitimately clean phase that writes
`result.json` but no `issues.json` is **not** mis-gated, because the guard requires both
to be nil. `implementGate` is re-checked after every warm fix round inside `reImplement`,
not just the first build.

### REV-003 — bounds pre-flighted, gates disambiguated (verified)
A pre-flight `overBudget()` at the top of `Run()` gates an already-over-ceiling re-run
**spawning nothing** (`TestPreflightCostGateNoSpend` asserts zero phase calls). Gates are
split: `gateDecision` ("resolve, then re-run the warm session") vs `gateBudget` ("raise
`GOGO_RUN_MAX_ROUNDS` / `GOGO_RUN_COST_CEILING`, or resolve the findings") — the
misleading "re-run to continue" no longer appears on a budget gate. Not resetting `Round`
is a sound call: a re-run still re-runs review, so a feature whose findings the human
resolved completes without more fixes; only when more fixes are needed does the exhausted
budget re-gate. The one review a re-run spends is necessary fresh-eyes re-verification,
not the money sink round 1 flagged.

### REV-004 — decision-marker scan widened (verified)
`hasOpenNeedsUserDecision` uppercases `title + description + proposed_solution` and
substring-matches the marker. `TestNeedsUserDecisionScansAllFields` covers the tag in
title and in description; the route table covers the marker inside an otherwise-batched
minor (→ gate).

### REV-005 — round bound documented (verified, accept-as-documented)
The global per-feature counter is kept and documented plainly as a **total** fix-round
budget in the `reImplement` doc-comment, `gogo run --help` (`GOGO_RUN_MAX_ROUNDS`), and
plan FR7. A legitimate accept-as-documented resolution for a minor; per-id tracking is
deferred to a later slice. (Trivia: the fix note's mention of a "route.go doc-comment"
is inaccurate — route.go carries no round-bound text — but the docs are present where
they matter.)

## New finding (introduced by the round-2 fixes)

### REV-006 — is_error halt discards the run's cost (minor)
The `is_error` early-return added for REV-002 (`orchestrator.go` exec, ~line 297) returns
**before** `Reg.record()`/`save()`, so the failed run's `res.CostUSD` is never booked.
A phase that finishes `is_error=true` still cost real money; because the run halts with
exit 1 (not a gate), a user who re-runs a deterministically-erroring phase spends an
unaccounted session each time, and REV-003's cost pre-flight never trips (the spend was
never persisted). Low impact — it halts loudly and is user-driven, not an automatic loop
— but it undercuts the cost-accounting invariant the fixes strengthen. **Agent-fixable:**
book the `SessionInfo` telemetry before returning the halt error; keep the halt.

## Verdict

**APPROVE** — all three round-1 majors and both minors are verified fixed, gates are
green, and the new tests assert the fixed behaviour. The one new finding (REV-006) is a
minor cost-accounting gap, non-blocking; it can be batched into the next fix round.
