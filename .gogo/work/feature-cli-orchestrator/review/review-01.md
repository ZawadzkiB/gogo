# Review 01 — CLI process-orchestrator (Slice 1)

Round 1 · fresh-eyes review of the uncommitted `gogo run` / `cli/internal/orchestrator`
change set against `plan.md` (FR1-FR9 + FR11) and `.gogo/knowledge/*`.

**Gates:** `gofmt -l .` clean · `go vet ./...` clean · `go test -race ./...` green
(orchestrator, launch, contract all pass). Version bumped in both mirrors
(`.claude-plugin/plugin.json` and `cli/main.go` → 0.13.0).

## What is solid (verified, no action)

- **Write-scope invariant holds.** The orchestrator's only production write is
  `Registry.Save` under `.gogo/resources/cli/sessions/<slug>.json`
  (`registry.go:63,70`). Nothing writes `state.md` / `issues.json` / `events.jsonl` /
  the feature folder. Registry degrades to first-run on a missing/garbled file.
- **Warm/fresh session identity (FR3/FR4) is correct.** The dev keeps one UUID and
  switches `--session-id`→`--resume`; review/test each get a fresh UUID with no
  resume; report is a flagless one-shot. `PhaseArgs` keeps flag+value as separate
  argv elements and makes resume win over session-id (injection-safe; tested).
- **Acceptance gate (FR1) is a safe whitelist.** `RunnableStatus` admits only
  `plan-accepted/implementing/reviewing/testing`; every unaccepted/parked status
  (`awaiting-plan-acceptance`, `awaiting-uat`, `waiting-for-user`, `done`, `shipped`)
  is refused. Verified against the real state.md status vocabulary.
- **Loop shape is correct and terminating.** A test fix warm-resumes the dev, then
  re-enters the review sub-loop (fresh review) before re-testing; the round/cost
  gates sit where they gate rather than loop unbounded.
- **`--in-session` (FR11) is documented consistently** across `commands/implement.md`
  (arg-hint + body), `skills/gogo-implement/SKILL.md`, and is passed on *both* the
  initial build and every warm fix round.
- **Enumeration sync:** new subcommand in `main.go` help, `README.md`, `gogo run
  --help`, and `docs/cli-contract.md`; `/gogo:resume` (the `--attach` path) exists.
  No `docs/*.md` quick-reference enumerates the CLI subcommands, so none was missed.

## Findings

| id | sev | title |
|----|-----|-------|
| REV-001 | major | `Route` re-implements on ANY open/new; gogo-review batches minors → drift on the anti-drift constraint (FR6) |
| REV-002 | major | Failed / blocked / no-output phase is a false green: `is_error` ignored, `Route(nil,nil)=Advance`, fix-round result unchecked |
| REV-003 | major | Persisted `Round`/`CostUSD` never reset → re-run after a bound/cost gate re-gates (cost re-gate first spends a paid session) |
| REV-004 | minor | needs-user-decision scan is `proposed_solution`-only; the tester agent doesn't pin the tag there |
| REV-005 | minor | Round bound is a global per-feature counter, not the "same finding ~3 rounds" bound FR7 specifies |

### REV-001 — routing drift on minors (major)
`route.go` `OpenIssueCount` counts every open/new issue, so `Route` → ReImplement on
any finding. Its doc-comment claims parity with the skills' "④ Route" and names
`gogo-review/SKILL.md` as authority — but that skill routes only on open/new
**blockers/majors** and "batch[es] the minors" (`gogo-review/SKILL.md:87-94`).
`gogo-test` *does* route on any open/new, so the one shared function matches test but
contradicts review. A lone open minor advances in-chat yet loops back (and can burn to
a spurious round-bound gate) under `gogo run`. The plan's FR6 *summary* ("count > 0")
matches the code but contradicts the skill it cites, so **which semantics is canonical
is a NEEDS-USER-DECISION.**

### REV-002 — false green on a failed phase (major)
`RunResult.IsError` (`launch.go:170`) is parsed but never checked. A `claude -p` phase
that finishes with `is_error:true` (or is killed) but exits 0 returns cleanly; if it
wrote no `result.json`/`issues.json`, `readTrack` returns `(nil,nil)` and
`Route(nil,nil)` → **Advance**. So a failed review marches to test and a failed test to
report→awaiting-uat, silently. Also `implementGate` (blocked/waiting-for-user check) is
called only on the first build, never on warm fix rounds. The fake runner always writes
the files, so this path is untested.

### REV-003 — sticky bounds block the documented recovery (major)
`Round` and `CostUSD` persist in the registry and are never reset. After a round/cost
gate, a re-run reloads the maxed values; `overBudget` is only checked *after* a phase
runs, so a cost re-gate first spawns and **pays for** another review before gating, and
a round re-gate gates on the first `reImplement` with no work done. The gate footer's
"the feature is parked … re-run `gogo run` to continue" is misleading here — the
orchestrator (correctly) doesn't touch `state.md`, so the feature isn't parked and the
re-run can't progress. Escaping needs an env bump or deleting the registry — undocumented.

### REV-004 — decision-marker scan is too narrow (minor)
`hasOpenNeedsUserDecision` scans only `proposed_solution`. The reviewer pins the tag
there (`gogo-reviewer.md:38-39`); the tester merely says findings are "tagged
needs-user-decision" (`gogo-tester.md:40,50`). A test decision tagged in title/description
would misroute to ReImplement (backstopped only by the `waiting-for-user` result status).
Scan title+description too, or pin the tester's tag to `proposed_solution`.

### REV-005 — global vs per-finding round bound (minor)
`Reg.Round` is one counter shared across review and test; FR7 and `gogo-review:90` mean
"the same id survives ~3 rounds". Progress on distinct findings can gate at the cap
though no single finding survived three rounds. Errs safe, but diverges from the stated
semantics — fix per-id, or document the per-feature-total behaviour.

## Verdict

**CHANGES** — 3 open majors (REV-001 routing drift, REV-002 false-green on failed
phase, REV-003 sticky bounds block recovery). REV-001 carries a NEEDS-USER-DECISION on
the canonical minor-routing semantics; REV-002/003 are agent-fixable.
