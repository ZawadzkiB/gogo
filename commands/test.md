---
description: Run phase ④ test standalone — e2e/UI/CLI/API testing per the project's strategy; emit the living test issues.json + a test-NN.md snapshot. Validates inputs and outputs.
argument-hint: "[feature-slug]"
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Skill, Task, TodoWrite, AskUserQuestion
model: opus
---

Run **phase ④ (test)** standalone for a feature, via the `gogo-test` skill, with
**validate-in → work → validate-out** (using `gogo-contracts`). Re-running it
after fixes updates the same living `test/issues.json` in place.

Target: $ARGUMENTS  (if no slug, pick the most recent `.gogo/plans/feature-*/`
that has passed review; if several, ask which.)

Documents it accepts: `plan.md` (required, its Tests section), `testing-tools.md` /
`test-strategy.md` / `tech-stack.md` / `non-functional-requirements.md` (required
knowledge), the as-built `charts/manifest.json` (optional input), and any existing
`test/issues.json` (optional — updated in place).

Load `gogo-test` and follow it:

1. **validate-in** — `plan.md` present and review done; validate
   `charts/manifest.json` and any prior `test/issues.json` against their schemas.
   Invalid/missing required input → STOP with a contract error.
2. **Work** — delegate to `gogo-tester` (run suites, exercise UI/CLI/API, extend
   e2e); update the living `test/issues.json`; render the `test-NN.md` snapshot.
3. **validate-out** — validate `test/issues.json` against
   `issues-list.schema.json`; write `test/result.json`; route on issues-list
   emptiness (open issues → implement with `--issues`; all-green → report). Update
   `state.md`.
