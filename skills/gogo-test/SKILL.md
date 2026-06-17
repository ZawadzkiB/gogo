---
name: gogo-test
description: >-
  Phase ④ of the gogo pipeline — e2e test and explore the change at every
  relevant level (UI/CLI/API) per the project's testing tools and strategy; loop
  issues back to implement, or escalate to re-planning. Delegates to the
  gogo-tester agent (bundled Playwright MCP).
---

# gogo-test — phase ④ (test + explore, then route)

The orchestrator runs this as the **router**; testing is done by the
**`gogo-tester`** agent.

## Steps
1. Read `testing-tools.md`, `test-strategy.md`, `tech-stack.md`, `plan.md`'s
   Tests section, and `non-functional-requirements.md` (the bars to verify).
2. **Delegate** to `gogo-tester` via `Task`, passing the plan, the test
   strategy/tools, and the output path `test-NN.md`. The tester:
   - runs existing suites first (build, unit, then e2e — require green),
   - exercises the change hands-on: **UI** via the bundled `gogo-playwright`
     MCP (`browser_*` tools — real flows + exploration + screenshots), **CLI**
     via shell, **API** via HTTP,
   - adds/extends e2e tests for the new behaviour,
   - writes `test-NN.md` (what was exercised, results, new tests, issues +
     severity + fixable?).
3. **Route:**
   - **Issue, fixable** → back to **② implement** → ③ review → back here.
   - **Issue needing a user decision** → back to **① plan** (re-plan how to
     handle it, re-accept) via a decision gate (`decisions.md` + waiting-for-user).
   - **All green** (build + unit + e2e + hands-on, per the done-bar in
     `test-strategy.md`) → advance to **⑤ report** (`gogo-knowledge`).
4. Update `state.md`: phase=test, status=testing, bump `iterations: test=<n+1>`
   each round.

## Degradation (portability)
If the Playwright MCP / Node is unavailable: skip browser automation, run the
project's own test commands, exercise API/CLI directly, and write **manual
UI-check steps** into `test-NN.md`. Never fail the phase for missing browser
tooling — note the gap so the user can run those checks.
