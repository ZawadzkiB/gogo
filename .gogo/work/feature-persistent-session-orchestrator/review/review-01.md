# Review — round 01 · `persistent-session-orchestrator`

Fresh-eyes review (phase ③). Reviewed against `plan.md`, `decisions.md` (D1–D6),
and `.gogo/knowledge/{code-review-standards,coding-rules,non-functional-requirements}.md`.

## Scope reviewed
- **New:** `cli/go.go`, `cli/go_e2e_test.go`, `cli/internal/orchestrator/lock.go`,
  `cli/internal/orchestrator/sweep.go`.
- **Modified:** `cli/internal/orchestrator/{orchestrator,registry,orchestrator_test}.go`,
  `cli/internal/launch/launch.go`, `cli/main.go`, `.claude-plugin/plugin.json`,
  `docs/cli-contract.md`, `README.md`.
- **Deleted (D3=A):** `cli/internal/contract/route{,_test}.go`, `cli/run.go`,
  `cli/run_e2e_test.go` — confirmed gone; no dangling `contract.Route` / per-phase-loop callers.

## Gates (re-verified)
- `gofmt -l .` clean · `go vet ./...` clean · `go test -race ./...` green (incl. the
  hermetic stub-claude e2e `TestGoE2EStubClaude` crossing the real `exec.Command` boundary).

## What holds up (spot checks against the plan)
- **FR1/FR2/FR4** — `ResolveInvocation` is a pure fresh(`--session-id`)-vs-`--resume` resolver;
  exit classification reads `state.md` (awaiting-uat → done hint, waiting-for-user → parked,
  `is_error` → halt with no false green; verified the cost is still booked before the halt).
- **FR3** — `RunnableStatus` unchanged rule for `gogo go`; `PlannableStatus` for `gogo plan`;
  terminal → opportunistic reap, friendly exit.
- **FR5/FR6 lock (D1=C, D6=A)** — lockfile + PID(signal-0) **AND** live `gogo-*` tmux cross-check;
  refuse-by-default, `--takeover` seizes, stale silently reclaimed; attribution via exact
  `SessionMatchesSlug` (TEST-005) — never substring. Sweep spares `oauth` vs `auth`,
  `waiting-card` vs `awaiting-card` (test-verified).
- **FR7 registry** — per-leg (`go`|`plan`) persistent session; missing/garbled/legacy `gogo run`
  file degrades to fresh, never crashes; writes stay under `.gogo/resources/` (no pipeline-state mutation).
- **FR8/FR9 reap/sweep (D5=A)** — kill-at-ship + orphan `gogo-*` reaping; `--dry-run` kills nothing;
  the `--attach` path (`LaunchPersistent`) never sets `remain-on-exit` — the leak is gone by construction.
- **Injection safety** — single-argv everywhere (`PhaseArgs`, `TmuxPersistentArgs`, `KillSession`);
  no shell; the slug/command is always a separate final argv element.
- **FR10/FR11/FR12** — skills/commands/templates untouched (out of scope by the accepted plan);
  `gogo run` is a deprecating alias; version bumped 0.14.0 → 0.15.0 in both `plugin.json` and
  `main.go`; `docs/cli-contract.md` extended additively and `README.md`/help updated.

## Findings (4 — none blocking)

| id | sev | pri | one-line | fix |
|----|-----|-----|----------|-----|
| REV-001 | minor | P2 | `gogo plan` derives lock/registry paths from an unvalidated slug — `..` can escape `.gogo/resources/` (write-scope invariant) | AGENT-FIXABLE |
| REV-002 | minor | P2 | `--takeover`/stale-reclaim reaps the prior by the lockfile's pre-collision base tmux name, not the real (suffixed) session | AGENT-FIXABLE |
| REV-003 | nit | P3 | owner-lock `Acquire` is a non-atomic read-check-write (TOCTOU) — narrow simultaneous double-launch window | AGENT-FIXABLE |
| REV-004 | minor | P3 | dead reader (`contract.PhaseResult`/`ReadResult`) + stale comments still describing the deleted per-phase loop / `Route` | AGENT-FIXABLE |

### REV-001 — unvalidated slug reaches the filesystem (minor, P2)
`cmdPlan` (go.go:137-172) accepts a brand-new slug (`PlannableStatus("")==true`); it flows into
`LockPath`/`RegistryPath` and `writeLock` (lock.go:59-61,120-129 · registry.go:64-66) with no
validation, so `gogo plan ../../../../foo` writes outside `.gogo/`. `gogo go` is safe (refuses when
`repo.Feature(slug)==nil`); `gogo plan` is the exposed verb. **Fix:** validate the slug against
`^[a-z0-9]+(-[a-z0-9]+)*$` (the schema's own pattern) before building any path/command.

### REV-002 — takeover reaps the wrong tmux name on a collision (minor, P2)
The prior-owner reap uses `s.Killer(prior.Tmux)` (orchestrator.go:156-158) where `prior.Tmux` is the
**pre-collision base name** recorded at orchestrator.go:145 (`me.Tmux = s.intent().Session`), while the
real session may be `uniqueSession`-suffixed and is only stored in the registry (orchestrator.go:220).
If the base name collided at launch (e.g. a board-launched `gogo-go-<slug>` already existed), takeover
kills the wrong/nonexistent name and orphans the session it meant to seize (the inline comment "reap
uses the registry" is untrue on this path). Narrow + `gogo sweep` backstops. **Fix:** reap by slug via
`ListSessions` + `SessionMatchesSlug` (TEST-005), or use the prior's registry tmux name.

### REV-003 — non-atomic lock create (nit, P3)
`Acquire` (lock.go:76-96) reads-checks-writes without `O_CREATE|O_EXCL`; two same-instant first-launches
of one fresh slug can both pass and launch. The over-time racer the feature targets is still caught by
the PID/tmux cross-check; this is only the microsecond simultaneous-start gap. **Fix:** create with
`O_CREATE|O_EXCL` and treat `EEXIST` as "prior present → re-read + liveness-check".

### REV-004 — dead reader + stale comments (minor, P3)
After the D3 deletion, `contract/result.go`'s `PhaseResult`/`ReadResult` have zero callers (grep) and
its comment still cites the removed `gogo run` per-phase chainer and `Route`; `launch.go:128-181`
(RunPhase/PhaseOpts) still describes the deleted "developer warm across fix rounds, review/test fresh"
per-phase model. Trips code-review-standard #1. **Fix:** delete the dead reader (or drop the `Route`/
`gogo run` references — `result.json` is still a written artifact) and refresh the `launch.go` comments
to the single persistent-session model.

## Plan fidelity
Nothing unplanned crept in; every FR and every accepted decision (D1–D6) is reflected in code.
Out-of-scope items (board drill-in, `gogo done`/`accept` CLI, multi-model, board `remain-on-exit`,
`/gogo:done` reap hook) are correctly deferred.

## Verdict
**APPROVE** — no open blockers or majors; 4 non-blocking findings (2 minor, 1 minor doc, 1 nit), all AGENT-FIXABLE.
