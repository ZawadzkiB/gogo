# Test round 1 — feature `cli-orchestrator` (Slice 1: CLI process-orchestrator)

**Verdict: ALL GREEN — 0 open findings. Done-bar MET** (build + unit green, hands-on
done, no hands-on check blocked). Advance to ⑤ report.

This is a CLI/Go feature (no UI), so the bundled Playwright MCP was skipped per the
task brief — testing focused on the Go suite plus hands-on CLI/subprocess exercise.

## 1 — Existing suites (required green before exploring)

```
cd cli && gofmt -l . && go vet ./... && go build ./... && go test -race -count=1 ./...
```

All clean:

| check | result |
|---|---|
| `gofmt -l .` | clean (no files) |
| `go vet ./...` | clean |
| `go build ./...` | clean |
| `go test -race -count=1 ./...` | **green**, 9 packages (`cli`, `internal/contract`, `internal/diagram`, `internal/diagram/mermaidascii`, `internal/launch`, `internal/orchestrator`, `internal/pages`, `internal/trash`, `internal/tui`; `internal/textfmt` has no test files) |

Individually re-ran (by name, `-v`) every test review-02.md cited as proof of the
round-2 fixes — all pass: `TestReviewBatchesMinor`, `TestPhaseErrorHalts`,
`TestNoOutputGates`, `TestPreflightCostGateNoSpend`,
`TestNeedsUserDecisionScansAllFields`, `TestRouteTable`, plus the rest of the
orchestrator suite (`TestHappyPath`, `TestWarmResumeOnFix`,
`TestGateOnNeedsUserDecision`, `TestRoundBoundGates`, `TestCostCeilingGates`,
`TestRegistryRoundTrip`, `TestRunnableStatus`).

## 2 — CLI hands-on (real binary, `cd cli && go build -o /tmp/gogo-test .`)

| check | command | result |
|---|---|---|
| Help | `gogo run --help` | prints full usage: warm dev / fresh review-test, env knobs (`GOGO_RUN_MAX_ROUNDS`, `GOGO_RUN_COST_CEILING`, `GOGO_CLAUDE_PERMISSION_MODE`), exit-code legend. exit 0. |
| Unknown slug | `gogo run some-nonexistent-slug` (repo root) | `gogo run: no feature "some-nonexistent-slug" under .gogo/work/`, exit 1. |
| Acceptance gate — `awaiting-uat` | scratch fixture (temp dir, **not** the real repo), `feature-ztest-notrunnable/state.md` status `awaiting-uat` | `gogo run: feature "ztest-notrunnable" is "awaiting-uat" — not runnable here. it's at the UAT gate — run /gogo:done to ship, or give feedback to loop it back.`, exit 1. |
| Acceptance gate — `waiting-for-user` | same fixture, status flipped | `... it's paused on a decision — resolve it and re-accept (→ plan-accepted) first.`, exit 1. |
| No `.gogo/` anywhere on path | scratch empty dir | `gogo: no .gogo/ found from here up — run inside a gogo project`, exit 1. *(extra FR1 guard, checked for completeness.)* |
| `claude` not on PATH | `env -i PATH=/usr/bin:/bin ... gogo run cli-orchestrator` | `gogo run: claude CLI not on PATH — the orchestrator spawns \`claude -p\` sessions`, exit 1. *(extra FR1 guard, checked for completeness.)* |

Per the task brief, the real loop was **never** run against the live
`cli-orchestrator` feature itself (it is `status: testing`, currently runnable,
and `claude` is genuinely on PATH in this environment — running it for real would
re-drive the pipeline and spend real money). All guard paths above were exercised
against throwaway scratch fixtures, cleaned up afterward.

## 3 — THE KEY HANDS-ON E2E: the stub-`claude` dry run (plan Tests §, D2)

Put a scripted stub `claude` first on `PATH` — a bash script that logs its full
argv, writes the canned contract files a real phase session would, and prints a
minimal `--output-format json` envelope
(`{"session_id":...,"total_cost_usd":0.01,"num_turns":1,"duration_ms":10,"is_error":...}`)
— then ran the **real `gogo` binary** (`/tmp/gogo-test run <scratch-slug>`)
against three scratch scenarios, each in its own throwaway `.gogo/` tree. All
three were run manually first, then captured as a hermetic, repeatable Go test
(`cli/run_e2e_test.go`, `TestRunE2EStubClaude`) that calls the real `cmdRun`
dispatch — the exact function the compiled binary's `main()` calls for `gogo
run` — so `launch.RunPhase`'s actual `exec.Command("claude", ...)` boundary is
exercised, not just the injected-fake-`PhaseRunner` unit tests in
`orchestrator_test.go` (which never cross that boundary).

**Happy path.**
```
→ /gogo:implement <slug> --in-session
→ /gogo:review <slug>
→ /gogo:test <slug>
→ /gogo:report <slug>
✓ <slug> — pipeline green; report written, stopped at awaiting-uat.
```
Exit 0. The exact call order (implement → review → test → report), confirmed
both by the printed trace and by the stub's own `calls.log`. The session
registry `.gogo/resources/cli/sessions/<slug>.json` was written with a **stable
`dev_uuid`**, 4 recorded sessions (implement/review/test/report), each with
`cost_usd`/`num_turns`/`duration_ms` telemetry > 0.

**Warm resume.** Stub's review round 1 writes one open `major` finding →
`contract.Route` correctly returns `ReImplement` (review batches minors, so a
`major` is the right severity to trigger it) → orchestrator re-implements via
`--resume`. Grepped the stub's own recorded argv, call by call:

```
call 0 (build):     --session-id <dev-uuid>          (fresh)
call 1 (review 1):  --session-id <fresh-A>            (fresh, never --resume)
call 2 (fix round): --resume <dev-uuid>               (SAME dev uuid — warm)
call 3 (review 2):  --session-id <fresh-B>, B != A     (fresh, never --resume)
call 4 (test):      --session-id <fresh-C>
call 5 (report):    (one-shot, no session flag)
```

Exit 0. The dev session kept **exactly one** uuid across the whole run
(`--session-id` once, `--resume` on the fix round); each review round got a
**brand-new** uuid with **no** `--resume` ever on a review call — proving
FR3/FR4 hold under a real subprocess boundary, not just in the fake-runner unit
tests. The registry records the fix-round `implement` session with
`resumed: true` under the dev uuid.

**`is_error` halt.** Stub sets `is_error:true` on the review call (after
implement succeeds) →
```
→ /gogo:implement <slug> --in-session
→ /gogo:review <slug>
gogo run: phase "review" reported an error (claude is_error) — halting; not advancing on a failed phase
```
Exit **1** — the loop halts immediately, never reaches test/report, never a
false green (REV-002's fix holds). The registry still records **2** sessions
(implement + the failed review) with the review's `cost_usd` booked **before**
the halt — confirms REV-006's fix (book telemetry before returning the halt
error) holds under a real subprocess, not just the fake-runner unit test.

All three scenarios: hermetic (temp dirs only), no network, no real claude, `git
status` on the real repo clean of stray artifacts after every run.

## 4 — New test added

**`cli/run_e2e_test.go`** — `TestRunE2EStubClaude` (3 subtests: happy path /
warm-resume / is_error-halt). Drives the real `cmdRun` → `orchestrator.New` →
`ClaudeRunner` → `launch.RunPhase` → `exec.Command("claude", ...)` chain against
an embedded stub `claude` script (written to `t.TempDir()`), a scratch `.gogo/`
tree (`t.TempDir()` + `t.Chdir()`, auto-restored), and env vars set via
`t.Setenv` (auto-restored). No `t.Parallel()` anywhere in `package main`'s tests,
so the process-global `Chdir`/env mutation is safe. Confirmed clean on a 3x
repeat with `-race`; `gofmt`/`go vet` clean.

This closes the one gap the existing `orchestrator_test.go` fake-`PhaseRunner`
suite structurally cannot reach: the real argv construction and process-spawn
boundary. It is the automated, repeatable form of plan Tests §'s "one scripted
end-to-end dry run" item.

## Issues found

**None.** 0 open findings this round — `test/issues.json` carries an empty
`issues: []` array (round 1).

## Verdict against the done-bar (`test-strategy.md`)

- Build + unit green — **MET** (gofmt/vet/build/test -race all clean, 9
  packages).
- Hands-on/e2e done — **MET**. Every relevant hands-on check ran: CLI guards
  (help, unknown slug, both acceptance-gate refusals, no-`.gogo/`,
  no-`claude`-on-PATH) and, critically, the stub-`claude` dry run across all
  three scenarios the task specified (happy / warm-resume / is_error), now also
  captured as a repeatable Go test.
- **No hands-on check was blocked.** `claude` and `tmux` are both genuinely on
  `PATH` in this environment, so no `needs-user-decision` gate is raised this
  round.

**Advance to ⑤ report.**
