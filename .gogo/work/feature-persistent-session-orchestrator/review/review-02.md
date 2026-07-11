# Review — round 02 · `persistent-session-orchestrator`

Fresh-eyes fix-verification pass (phase ③, round 2). Round 1 was **APPROVE** with
4 agent-fixable findings (REV-001..004). This round **verifies each fix is real**
(not merely claimed) and re-scans the touched code for any regression the fixes
introduced. Reviewed against `plan.md`, `decisions.md` (D1–D6), and
`.gogo/knowledge/{code-review-standards,coding-rules,non-functional-requirements}.md`.

## Scope re-reviewed (the fix round)
- `cli/go.go` — new `validSlug` (`^[a-z0-9]+(-[a-z0-9]+)*$`) + guards in `cmdGo`/`cmdPlan`.
- `cli/internal/orchestrator/lock.go` — `Acquire` reworked (atomic `O_CREATE|O_EXCL`
  create + new refuse-over-live-untracked-session path); new `Lock.SetTmux`,
  `writeOwner`, `untrackedOwner`.
- `cli/internal/orchestrator/orchestrator.go` — `reapMatchingSessions(slug)` +
  best-effort `Killer(prior.Tmux)`; `launchAttached` records the real post-collision
  name via `lock.SetTmux`; injectable `Lister` seam; `printRefusal` untracked path.
- `cli/internal/launch/launch.go` — refreshed persistent-session doc-comments.
- **Deleted** `cli/internal/contract/result.go` (dead `PhaseResult`/`ReadResult`).
- Tests added: `TestLockRefusesUntrackedBoardSession`,
  `TestTakeoverReapsBoardSessionBySlug`, `TestValidSlug`, `TestCmdRejectsTraversalSlug`.

## Gates (re-run this round)
- `gofmt -l .` **clean** · `go vet ./...` **clean** · `go test -race ./...` **green**
  (whole module). The four new tests re-run fresh (`-count=1 -race`) — all pass.
- Version still in sync: `plugin.json` **0.15.0** == `cli/main.go` `Version 0.15.0`.

## Fix verification

| id | sev | verdict | evidence |
|----|-----|---------|----------|
| REV-001 | minor | **VERIFIED** | `validSlug` guard sits BEFORE `findRoot()`/any path build in both `cmdGo` (go.go:100) and `cmdPlan` (go.go:161); anchored `^…$` regexp rejects `..`, `/`, empty; `cmdRun` forwards through `cmdGo` so the alias is covered. TestValidSlug + TestCmdRejectsTraversalSlug pass. |
| REV-002 | minor | **VERIFIED** | `reapMatchingSessions` uses **exact** `SessionMatchesSlug` (no substring cross-attribution); `Killer(prior.Tmux)` fires only when `prior.Tmux != ""` (untracked owner has `Tmux=""` → no spurious kill); `SetTmux` records the real post-collision name as a secondary remedy. TestTakeoverReapsBoardSessionBySlug spares `gogo-done-other`, reaps `gogo-go-feat-2`. |
| REV-003 | nit | **VERIFIED** | Atomic `O_CREATE\|O_EXCL\|O_WRONLY` create; loser sees `EEXIST` → read+liveness branch; non-EEXIST errors surface. New untracked-refuse does **not** break a legit headless resume (see below). Refused untracked-acquire `os.Remove`s its just-made lockfile. TestLockRefusesUntrackedBoardSession + existing refuse/reclaim/takeover green. |
| REV-004 | minor | **VERIFIED** | `result.go` deleted in git (` D`); repo-wide grep for `PhaseResult`/`ReadResult` across `*.go` = **zero** hits; `IssuesList`/`ReadIssues` remain in `artifacts.go`. launch.go doc-comments now describe the persistent-session model. The lone `contract.Route` string is a comment describing what was *removed* — legitimate history, not stale live code. |

### The two riskiest fixes, checked explicitly
- **New untracked-refuse does not break a legitimate headless resume.**
  `DefaultLiveness(Owner{}, slug)` → `PidAlive(0)` is **false** (launch.go:509-515
  guards `pid<=0`), and a headless `-p` run opens **no** tmux session (`RunPhase`
  execs `claude` directly). The bare `live(Owner{}, slug)` check only runs on a
  **clean create** (no prior lockfile); the `--attach` path keeps its lockfile, so
  its re-run hits the read+liveness branch instead — the untracked check never
  fires against our own session. With no tmux, `ListSessions` → nil → the check is
  false (portability preserved).
- **No double-kill / cross-slug kill.** Reap attribution is exact
  `SessionMatchesSlug` everywhere; killing an exact tmux name can't cross-attribute
  to a substring; a redundant best-effort `Killer` call on an already-dead name is
  a harmless no-op (errors ignored by design).

## New findings
**None.** No regression introduced by the fixes; no new dead code or comment drift.
The one theoretical edge (`writeOwner` ignores a write error on the just-created
handle) is consistent with the file's best-effort discipline and negligible
(marshal of a ~150-byte struct to a fresh file) — not worth raising.

## Plan fidelity
Unchanged from round 1: every FR (FR1–FR12) and every accepted decision (D1–D6) is
reflected in code; out-of-scope items stay deferred. The fixes are internal
hardening (slug validation, lock atomicity, reap-by-slug, dead-code removal) — no
new user-facing surface, so no doc/enumeration sync was required.

## Verdict
**APPROVE** — all four round-1 findings (REV-001..004) VERIFIED; zero open
blockers/majors; no new findings. Gates green.
