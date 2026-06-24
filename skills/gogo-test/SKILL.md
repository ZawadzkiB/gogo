---
name: gogo-test
description: >-
  Phase ④ of the gogo pipeline — e2e test and explore the change at every
  relevant level (UI/CLI/API) per the project's testing tools and strategy; emit
  the living test issues.json (the contract) and render a test-NN.md snapshot;
  loop issues back to implement, or escalate to re-planning. Delegates to the
  gogo-tester agent (bundled Playwright MCP).
---

# gogo-test — phase ④ (test + explore, then route)

The orchestrator runs this as the **router**; testing is done by the
**`gogo-tester`** agent. This phase is **idempotent**: re-running it after fixes
updates the same living `test/issues.json` in place.

## Inputs (declared) and outputs (typed)

| Direction | Artifact | Contract |
|---|---|---|
| in (required) | `plan.md` (its Tests section) | prose contract |
| in (required) | `testing-tools.md`, `test-strategy.md`, `tech-stack.md`, `non-functional-requirements.md` | knowledge docs |
| in (optional) | `charts/manifest.json` + the `.mmd`s | `charts-manifest.schema.json` |
| in (optional) | existing `test/issues.json` | `issues-list.schema.json` |
| out | `test/issues.json` (living) | `issues-list.schema.json` |
| out | `test-NN.md` (snapshot) | rendered markdown |
| out | `test/result.json` (per run) | `phase-result.schema.json` |

## ① validate-in (gate — FR2)

Via `gogo-contracts`: confirm `plan.md` exists and review is done; if
`charts/manifest.json` or a prior `test/issues.json` is present, validate each
against its schema (right slug, real paths, unique ids, valid enums). Any required
input missing/invalid → **STOP** with a precise contract error; do not test on
bad input.

## ② Steps

1. Read `testing-tools.md`, `test-strategy.md`, `tech-stack.md`, `plan.md`'s
   Tests section, `non-functional-requirements.md` (the bars to verify), and the
   as-built `charts/` (what to exercise).
2. **Delegate** to `gogo-tester` via `Task`, passing the plan, the test
   strategy/tools, the as-built charts, the current `test/issues.json`, and the
   round number `NN`. The tester:
   - runs existing suites first (build, unit, then e2e — require green),
   - exercises the change hands-on: **UI** via the bundled `gogo-playwright`
     MCP (`browser_*` tools — real flows + exploration + screenshots), **CLI**
     via shell, **API** via HTTP,
   - adds/extends e2e tests for the new behaviour.
3. **Update the living `test/issues.json`** (the contract — D1/D2, same shape as
   review's). For this round:
   - **New issue** → append with a fresh stable `id` (e.g. `TEST-004`),
     `origin: test`, `found_in_round: NN`, `status: new`, all FR4 fields.
   - **Prior `fixed` issue the re-test confirmed** → `status: verified`.
   - **Prior `fixed` issue still failing** → back to `open`.
   - **Prior `open`/`new` still failing** → leave `open`.
   - Never renumber/reuse ids; resolved issues stay for the audit trail. Bump the
     file's `round` to `NN` and `updated` to today.
4. **Render the human snapshot** `test-NN.md`: what was exercised (UI/CLI/API),
   results, new/extended tests, and this round's issues with id/severity/priority/
   status. The JSON is the contract; the markdown is the readable companion.

## ③ validate-out (gate — FR3)

Via `gogo-contracts`: validate `test/issues.json` against
`issues-list.schema.json` (structural + semantic). Repair once on failure; if it
still fails, write `test/result.json` with `status: blocked`,
`validated_out: false` and stop. On success, write `test/result.json`
(`phase: test`, `status: ok`, `inputs`, `outputs`, `validated_in: true`,
`validated_out: true`, `open_issues: <count of open/new>`, `summary`).

## ④ Route

Decide purely on the **issues list** (count of `open`/`new`):
- Any `open`/`new` issue, fixable → back to **② implement** with
  `--issues test/issues.json` → ③ review → back here (re-test, same living list).
- Any issue needing a user decision → back to **① plan** (re-plan how to handle
  it, re-accept) via a decision gate (`decisions.md` + waiting-for-user).
- **All green** (build + unit + e2e + hands-on, per the done-bar in
  `test-strategy.md`; no `open`/`new` issues) → advance to **⑤ report**
  (`gogo-knowledge`).

Update `state.md`: phase=test, status=testing, bump `iterations: test=<n+1>` each
round. (`issues.json`/`result.json` are the machine state; `state.md` stays the
human-facing file.)

## Degradation (portability)

If the Playwright MCP / Node is unavailable: skip browser automation, run the
project's own test commands, exercise API/CLI directly, and write **manual
UI-check steps** into `test-NN.md` (and raise a `test/issues.json` issue if a
check can't be run). Never fail the phase for missing browser tooling — note the
gap so the user can run those checks.
