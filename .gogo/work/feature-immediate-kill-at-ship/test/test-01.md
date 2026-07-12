# Test round 1 — feature `immediate-kill-at-ship`

**Scope:** 0.17.0 — targeted `gogo sweep <slug>` (D4=B), FR3 self-guard, FR4
`remain-on-exit` removal, `/gogo:done` ship-reap. CLI/artifact levels are primary
per test-strategy.md (no UI surface for this repo).

## 1. Go suite (required gate) — PASS

```
cd cli && go build -o /tmp/gogo-kastest . && /tmp/gogo-kastest --version   → gogo 0.17.0
gofmt -l .        → clean, no output
go vet ./...      → clean
go test -race ./...  → all packages ok (contract, diagram, launch, orchestrator, pages, trash, tui, root)
```

The 5 named tests all exist and pass (`go test ./... -run 'TestSweep|TestSkillsBashNoUnsafeRm' -v -race`):
`TestSweepTargetedOnlyNamedSlug`, `TestSweepSparesSelf`, `TestSweepReapsOrphansAndTerminal`,
`TestSweepDryRunKillsNothing`, `TestSkillsBashNoUnsafeRm` — all PASS. Read (not just
trusted by name): each genuinely asserts the claimed behaviour — e.g.
`TestSweepTargetedOnlyNamedSlug` proves a `gogo sweep x` with `Only:["x"]` reaps only
`gogo-go-x` and spares `gogo-done-z` (a different terminal feature's concurrent host),
an orphan, and the self session.

## 2. Targeted-reap hands-on (real tmux) — PASS, NOT BLOCKED

Built a throwaway scratch repo (`.gogo/work/feature-kastest-scratch-1` `state.md`
`status: shipped`, `feature-kastest-scratch-2` `status: testing`). Baseline
`tmux list-sessions` recorded first (3 real host sessions: `gogo-done-cockpit-cards-and-cli-awareness`,
`gogo-go-cockpit-cards-and-cli-awareness`, `gogo-go-immediate-kill-at-ship`, attached).

- Created `gogo-go-kastest-scratch-1` (simulated driving session, terminal feature) +
  `gogo-done-kastest-scratch-2` (simulated concurrent DIFFERENT feature's `/gogo:done`, REV-002 scenario).
- `/tmp/gogo-kastest sweep kastest-scratch-1` (targeted, real kill) from the scratch root:
  reaped `gogo-go-kastest-scratch-1`; **`gogo-done-kastest-scratch-2` still alive**
  (spared — proves the D4=B/REV-002 fix); all 3 real host sessions untouched.
- `--dry-run` variant: recreated `gogo-go-kastest-scratch-1`, ran
  `sweep --dry-run kastest-scratch-1` → printed `would reap gogo-go-kastest-scratch-1 (...)`,
  session still alive afterward. Confirmed dry-run never kills.
- Cleaned up every scratch session; final `tmux list-sessions` matched the original
  baseline exactly.

## 3. Self-guard hands-on — PASS, live-demoed (not just the unit test)

Ran the targeted sweep **from inside** a real tmux session named
`gogo-done-kastest-scratch-3` (a 3rd scratch feature, `state.md` `status: shipped`),
via `tmux new-session -d -s gogo-done-kastest-scratch-3 '/tmp/gogo-kastest sweep kastest-scratch-3 ...'`
so `$TMUX`/`launch.CurrentSession()` resolved to that session name inside the child.
Output: `nothing to reap — no orphaned or terminal-feature sessions.` — the self-guard
skipped the hosting session before `shouldReap` even though its feature is terminal,
exactly as `TestSweepSparesSelf` predicts. Cleaned up.

## 4. FR4 remain-on-exit — PASS (code inspection + live demo)

`grep -n "remain-on-exit" cli/internal/launch/launch.go` → 5 hits, **all doc-comment
prose** (`Launch()`, `KillSession`, `LaunchPersistent`, the two other launch paths'
comments) — **no `set-option ... remain-on-exit` command** remains in `Launch()`'s
body. Live demo: `tmux new-session -d -s kastest-remaintest 'sleep 1'` → after it
finished, `kastest-remaintest` was gone from `tmux list-sessions` (pane closed on
exit by construction, no remain-on-exit needed to demonstrate the contrast).

## 5. Artifact/contract conformance — PASS, 1 doc-drift issue found

- `implement/result.json`, `review/issues.json`, `charts/manifest.json` structurally
  validated by hand against `templates/contracts/{phase-result,issues-list,charts-manifest}.schema.json`
  (repo's own current schema copies) — all required fields present, all enums valid
  (`status: ok`, `track: review`, diagram `kind`s `flow`/`activity`/`class`), no
  `additionalProperties` violations, `fixed` issues carry `fixed_in_round`+`fix_summary`.
- `state.md` transitions read coherently: plan-accepted → implementing → reviewing
  (round 1) → gate D4 (waiting-for-user) → implementing (round 2, fix) → reviewing
  (round 2, APPROVE) → testing (current). `iterations` line matches round history.
- `events.jsonl`: every line is valid JSON with the required `events.schema.json`
  fields and valid enums (**TEST-002**, nit — append order isn't strictly
  chronological by `ts`; not caused by this feature's code, see issues.json).
- **TEST-001** (open, minor): `plan.md`'s Approach/Alternatives/Tests/Out-of-scope
  sections still describe the superseded D1=A (plain sweep) design; `decisions.md`'s
  D4=B override and the actual shipped code (targeted `Only`-scoped sweep) were never
  back-ported into `plan.md`, contradicting `adjustments.md`'s claim that they were.
- `sweep --help` / `-h`: printed usage (`gogo sweep [--dry-run] [<slug>...]`, targeted
  vs whole-board explained) literally matches `cmdSweep`'s actual parsing.
- `sweep 'bad/slug'` (parse-only, safe anywhere): exit 1, stderr
  `gogo sweep: invalid slug "bad/slug"` — rejected before any tmux/file action.
- `docs/cli-contract.md` and `skills/gogo-done/SKILL.md` step 6 both correctly
  describe/invoke the targeted ship-reap (`gogo sweep <member-slug>...`) — in sync
  with the shipped code, unlike `plan.md` (TEST-001).

## 6. Whole-board regression — DRY-RUN ONLY — PASS

From this repo's real root: `gogo sweep --dry-run` (no slug) printed
`would reap gogo-done-cockpit-cards-and-cli-awareness (...)` and
`would reap gogo-go-cockpit-cards-and-cli-awareness (...)` (that feature already
shipped — real, pre-existing candidates) — correctly **spared** the currently
attached `gogo-go-immediate-kill-at-ship` (non-terminal). `tmux list-sessions`
before/after the dry-run was byte-identical — nothing was killed. (Never ran a real
whole-board sweep, per the safety brief.)

## Hands-on checks — none blocked

tmux was present and fully usable throughout. Every hands-on check in the plan (targeted
reap, dry-run, self-guard, FR4 pane-close, whole-board dry-run) was run for real, live,
against real or scratch tmux sessions — none were blocked or skipped.

## Acceptance signal vs. observed behaviour

Plan's acceptance signal: "no live `gogo-*` session for the shipped slug's driving
session, and the board badge is truthful." The scratch harness (step 2) is the
accepted equivalent proof: the targeted sweep reaped the shipped slug's driving
session while sparing a different feature's concurrent ship. **REV-001** (own
`/gogo:done` host session lingers by design, self-guard) is consistent with what was
observed live in step 3 — not re-flagged, per instruction.

## Issues this round

| id | severity | priority | status | origin | summary |
|---|---|---|---|---|---|
| TEST-001 | minor | P3 | open | test | `plan.md` still documents the superseded D1=A plain-sweep design, not the shipped D4=B targeted sweep |
| TEST-002 | nit | P3 | open | test | `events.jsonl` lines individually valid but not in strict chronological append order (pre-existing, not this feature's code) |

No blockers, no majors, no needs-user-decision issues, no blocked hands-on checks.

## Verdict

**Build:** green. **Unit (`go test -race ./...`):** green, including all 5 named
regression tests. **Hands-on/e2e:** fully run, nothing blocked (tmux present and
used live for targeted reap, self-guard, FR4, and whole-board dry-run). **Artifacts:**
conform to their schemas; 1 minor doc-drift + 1 nit telemetry-ordering issue found,
both fixable, neither a blocker to the shipped behaviour.

Per the done-bar (build + unit + e2e green + hands-on done): the **mechanical**
done-bar is met — no hands-on check was blocked or skipped. Two open minor/nit test
issues remain (TEST-001, TEST-002); routing them (fix now vs. accept) is the
orchestrator's call per the phase-④ routing rule.
