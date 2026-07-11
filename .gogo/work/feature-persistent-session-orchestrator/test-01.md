# Test round 1 — persistent-session orchestrator (CLI)

**Scope:** Go CLI-only change (`cli/`). No web UI surface — Playwright not applicable.
Per the tasking, real `gogo go`/`gogo plan` against a runnable slug (incl. this
very feature) was explicitly NOT run — that would spawn a real, billable
`claude -p` session.

## 1. Go suite (the primary gate)

From `cli/`, fresh (`-count=1`), all three commands from `tech-stack.md` /
`testing-tools.md`:

- `gofmt -l .` — **clean**, zero files listed.
- `go vet ./...` — **clean**, zero warnings.
- `go test -race -count=1 ./...` — **green**, 149 test functions passed, 0
  failed, across all 9 packages (`cli`, `internal/contract`, `internal/diagram`,
  `internal/diagram/mermaidascii`, `internal/launch`, `internal/orchestrator`,
  `internal/pages`, `internal/trash`, `internal/tui`).

Confirmed present and passing:
- **Hermetic stub-claude e2e** — `TestGoE2EStubClaude` (`cli/go_e2e_test.go`),
  4 sub-tests: first-launch argv (`--session-id`, `-p "/gogo:go <slug>"`,
  `--permission-mode auto`, `--output-format json`, no `--resume`), warm
  `--resume` on re-run (same UUID, no fresh `--session-id`), an `is_error`
  envelope halting `cmdGo` with exit 1 (never a false green), and `gogo run`
  forwarding to `gogo go` with a printed deprecation notice.
- **REV-001 regression guards** — `TestValidSlug` (valid/invalid table incl.
  `../foo`, `..`, `a/b`) and `TestCmdRejectsTraversalSlug` (`cmdGo`/`cmdPlan`
  reject a traversal slug before `findRoot()`/any path build).
- **Lock/registry/reap/sweep** (`cli/internal/orchestrator/`) — `TestLockRefusesLiveOwner`,
  `TestLockReclaimsStaleOwner`, `TestLockTakeoverSeizesAndReaps`,
  `TestLockRefusesUntrackedBoardSession`, `TestTakeoverReapsBoardSessionBySlug`
  (REV-002 regression), `TestRegistryRoundTrip`, `TestReapKillsTrackedTmux`,
  `TestSweepReapsOrphansAndTerminal`, `TestSweepDryRunKillsNothing`,
  `TestResolveInvocation` (fresh-vs-resume resolver), `TestExitClassifyAwaitingUAT`,
  `TestExitClassifyWaitingForUser`, `TestExitIsErrorHalts`.

## 2. Hands-on CLI exercise (SAFE paths only)

Built `cd cli && go build -o /tmp/gogo-test .` — clean build.

| Command | Observed |
|---|---|
| `gogo --version` | `gogo 0.15.0` — matches `plugin.json` and `cli/main.go Version` (FR12 sync confirmed by grep across all three). |
| `gogo --help` | Lists `go`/`plan`/`sweep`/`status`/`view`/`events`/`trash`/`run` (deprecated), flags, exit codes. Clean. |
| `gogo go --help` | Describes launch-or-resume, lock, `--attach`/`--takeover`, exit code meanings. Clean. |
| `gogo plan --help` | Describes the same lifecycle machinery for `/gogo:plan`. Clean. |
| `gogo sweep --help` | Describes terminal-feature reap + orphan reap + `--dry-run`, exact-match attribution, lockfile self-heal. Clean. |
| `gogo sweep --dry-run` (repo root) | `nothing to reap — no orphaned or terminal-feature sessions.` exit 0. **Verified the live tmux session `gogo-go-persistent-session-orchestrator` (this feature, `state.md` status mid-pipeline/non-terminal) was correctly spared** — `tmux ls` before/after shows it untouched; sweep did not list or kill it. No issue. |
| `gogo go this-slug-does-not-exist-xyz` | `gogo go: no feature "this-slug-does-not-exist-xyz" under .gogo/work/`, exit 1. Matches expected "no feature" style. |
| `gogo go ../../pwn` | `gogo go: invalid slug "../../pwn" — expected kebab-case [a-z0-9-] (no path separators)`, exit 1. |
| `gogo plan ../../pwn` | Same invalid-slug message, exit 1. Confirmed REV-001's fix holds live: verified no `pwn` file/dir was created anywhere outside the repo (checked both candidate escape targets) and `git status` shows no stray paths. |
| `gogo run some-slug` (nonexistent) | Prints `gogo run is deprecated — use \`gogo go\` ...` to stderr, then forwards and prints `gogo go: no feature "some-slug" under .gogo/work/`, exit 1. FR11 confirmed: deprecation notice + forward to the same gate logic, safely refusing without spawning claude. |

### Status-gate fixture (throwaway, under scratchpad — real repo untouched)

Built a minimal `.gogo/work/feature-<slug>/state.md` + `plan.md` fixture for
three statuses and ran `gogo go <slug>` with cwd set to the fixture root
(no `--root` flag exists; root is resolved from cwd via `findRoot()`):

| Status | Observed | Exit |
|---|---|---|
| `awaiting-uat` | `feature "uat-fixture" is "awaiting-uat" — not runnable here. it's at the UAT gate — run /gogo:done to ship, or give feedback to loop it back.` | 1 |
| `waiting-for-user` | `feature "waiting-fixture" is "waiting-for-user" — not runnable here. it's paused on a decision — resolve it and re-accept (→ plan-accepted) first.` | 1 |
| `shipped` | `shipped-fixture is shipped — nothing to run; reaped any tracked session.` | **0** |

All refusal/hint text is correct per `runnableHint`. One observation (not filed
as an issue): the plan's FR3 prose says a terminal status is "refused... with
the existing guidance," but the actual code path (`go.go` `cmdGo`, terminal
branch) treats `shipped`/terminal as a friendly no-op — opportunistic reap +
exit **0**, not a hard refusal (exit 1) like `awaiting-uat`/`waiting-for-user`.
This matches FR8's "opportunistic reap... when gogo go/gogo plan/the board see
the feature is terminal" wording and is a reasonable, arguably better,
reconciliation of two FRs that pull slightly different ways in the plan text —
not a bug. Confirmed all fixture writes stayed under the fixture's own
`.gogo/` (registry file for the reaped shipped session, no writes elsewhere) —
safety NFR intact. Fixture removed after the check.

## 3. True end-to-end (Step 3) — BLOCKED by design, user-decision gate

Not run, per the explicit instruction not to run `gogo go` against any
runnable slug (including this feature). This is expected: per `plan.md`'s
Tests section and `test-strategy.md`, a real `claude -p` drive is
non-deterministic/billable and must be a hands-on user-decision gate, never a
silent skip. Filed as **TEST-001** (major/P1, needs-user-decision, status
open) in `test/issues.json`. The CI-runnable proxy (`TestGoE2EStubClaude`) is
green and covers the argv/classification contract; what it cannot reach is the
real skill's nested Task review/test + report behaviour inside a live model
session.

## 4. Coverage check against plan.md's 8 test areas

All 8 areas have a corresponding, passing test function — no gap found, so no
test file was added or extended this round (adding tests without a real gap
would be padding, which the tasking explicitly says not to do):

1. Launch-or-resume resolver → `TestResolveInvocation`
2. Lock refusal/reclaim/takeover → `TestLockRefusesLiveOwner`, `TestLockReclaimsStaleOwner`, `TestLockTakeoverSeizesAndReaps`, `TestLockRefusesUntrackedBoardSession`, `TestTakeoverReapsBoardSessionBySlug`
3. Registry round-trip → `TestRegistryRoundTrip`
4. Reap kills tracked session + tmux → `TestReapKillsTrackedTmux`
5. Orphan-sweep (exact-slug attribution) → `TestSweepReapsOrphansAndTerminal`, `TestSweepDryRunKillsNothing`
6. Exit classification → `TestExitClassifyAwaitingUAT`, `TestExitClassifyWaitingForUser`, `TestExitIsErrorHalts`
7. `gogo run` alias → covered inside `TestGoE2EStubClaude`'s 4th sub-test
8. Hermetic stub-claude e2e → `TestGoE2EStubClaude`

## Issues this round

| ID | Title | Severity | Priority | Status | Fixable / decision |
|---|---|---|---|---|---|
| TEST-001 | True end-to-end (real claude -p driving /gogo:go) not run — hands-on user-decision gate | major | P1 | open | needs-user-decision |

## Verdict

**Build:** clean. **Unit (`go test -race`):** green, 149/149. **Hands-on CLI:**
done for every safe path (version/help, gate refusals, path-traversal guard,
status-gate fixture, sweep --dry-run correctly sparing the live in-flight
session). **Hermetic e2e (the CI-runnable proxy for the true e2e):** green.

**Done-bar is NOT fully met.** The one blocker is TEST-001 — the true
end-to-end (a real `claude -p` driving `/gogo:go` through
implement→review→test→report) has not been run by design, and only the user
can decide to run it by hand or explicitly skip it for this round. Everything
that CAN run unattended is green with zero regressions found.
