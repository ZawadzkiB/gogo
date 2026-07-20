# Test round 01 — project-scope-and-gates (→ 0.24.0)

Fresh-eyes hands-on test. Go CLI + Bubble Tea TUI (no web/Playwright — N/A to this
plugin). Every run isolated via `GOGO_DATA_HOME`/`GOGO_CONFIG_HOME` pointed at scratch
temp dirs (`/private/tmp/claude-502/.../scratchpad/gogo-test-psg/`); the real `~/.gogo`
and this repo's own `.gogo/` were never touched (verified: no new/modified files under
`/Users/bartlomiej.zawadzki/repos/gogo/.gogo/` beyond this feature folder, no stray
`gogo-*` tmux sessions).

## Gate (build + suites)
- `cd cli && go build -o /tmp/gogo-psg .` — clean.
- `gofmt -l .` — clean.
- `go vet ./...` — clean.
- `go test -race -count=1 ./...` — **green**, every package (`cli`, `config`,
  `contract`, `diagram`, `diagram/mermaidascii`, `launch`, `orchestrator`, `pages`,
  `plans`, `projects`, `trash`, `tui`).
- `TestCLICommandEnumerationInSync`, `TestSkillsBashNoUnsafeRm`,
  `TestVersionMirrorsPlugin` — individually re-run, all **PASS**.
- `/tmp/gogo-psg --version` → `gogo 0.24.0`. `.claude-plugin/plugin.json` version →
  `0.24.0`. Both match.

## Review findings (round 1) — verified contained
All 5 findings in `review/issues.json` are `status: fixed`. Re-verified by direct
inspection (not just trusting the fix_summary):
- **REV-001** (major — FR4 had no direct test coverage): confirmed the promised
  regression net now exists and passes — `TestSkipForSource` +
  `TestSkipFlagsOmitemptyRoundTrip` (`internal/projects/projects_test.go`),
  `TestSkipParams` (`internal/launch/launch_test.go`), `TestGoSkipSuffix`
  (`internal/orchestrator/skip_test.go`), `TestLaunchedCommandCarriesSkipParams`
  (`internal/orchestrator/orchestrator_test.go`), `TestResolveSourceSkip` +
  `TestGoPathCarriesSkipParams` (`cli/skip_test.go`),
  `TestBoardGoIntentCarriesSkipParams` (`internal/tui/skip_test.go`). I additionally
  re-proved the CLI-path claim independently outside `go test` (see FR4 below).
- **REV-002** (minor — `memberFeature` cross-project guard): confirmed
  `plans.memberFeature`/`MembersShippedIn` (`internal/plans/plans.go:498-522`) now
  takes a `project` param and skips a feature whose `Project` is set and differs;
  `TestMembersShippedInCrossProjectGuard` (`internal/plans/plans_test.go:200-238`)
  covers the same-source-name-in-two-projects case, the plan's-own-project case, and
  the project-less fallback.
- **REV-003** (minor — `gogo/SKILL.md` wording risk): confirmed
  `skills/gogo/SKILL.md:227-231` now reads "**invoke the `gogo-done` skill**...
  `gogo-done` stays the SOLE owner of the ship... you do **not** emit `uat-passed`
  yourself" — matches `skills/gogo-done/SKILL.md:83-88`'s "the gogo orchestrator
  auto-invokes this ship". No room left for a second emitter.
- **REV-004** (minor — unreachable `gogo-plan --skip-acceptance` branch): confirmed
  `skills/gogo-plan/SKILL.md:47-58` now documents it as intentional
  belt-and-suspenders coverage for a *direct* `/gogo:plan <goal> --skip-acceptance`
  invocation, with the correct routing statement ("the cockpit... appends
  `--skip-acceptance` to the launched `/gogo:go`, **not** to `/gogo:plan`").
- **REV-005** (nit — misleading "auto-skipped" wording): confirmed `cli/go.go:184-190`
  now prints "source X has planAcceptanceSkip — /gogo:go auto-accepts the plan if it
  hits the plan gate this run" (no over-claim); reproduced live in my own hands-on run
  below (no "plan-acceptance auto-skipped" string; the opt-in note is still printed).

## FR1 — dual-mode `gogo project add` (hands-on, real binary)
All exercised against `/tmp/gogo-psg` in the isolated scratch data home:
- `gogo project add sanoma` → empty project: `~/.gogo/projects/sanoma/config.json`
  (`sources: []`) + `.knowledge/project-knowledge.md` + `.gogo/plans/`. No repo, no
  source, no error.
- Editing the seeded knowledge file then re-running `gogo project add sanoma` →
  idempotent: prints "already exists", the edit is **preserved** (never clobbered).
- `gogo project add <repo-with-.gogo>` (path mode) → project + source #1, byte-for-byte
  today's flow (branch detected, cap 1, color assigned).
- `gogo project add <name-no-.gogo-repo>` (path mode, error case) → the exact error
  "... has no .gogo/ - not a gogo source (run /gogo:build there first)", exit 1.
- `gogo project add sanoma2 --source <repo>` (one-shot) → project + source #1 +
  `.knowledge/` scaffold, all in one call.
- `gogo project add <path> --source <repo>` (PATH positional + `--source` together) →
  correctly **rejected**: "--source is for the bare-NAME form...", exit 2.
- Bare token that resolves (via cwd) to a real `.gogo/` dir → correctly read as PATH
  (idempotent "already registered" note), not re-created as a name.
- Bare token with a leading `.` (`.hidden-name`) → correctly read as PATH per the
  documented rule, clean error (no `.gogo/`), no crash.
- Bare token that IS an existing plain directory with **no** `.gogo/` → correctly read
  as NAME (creates an empty project), not misclassified as a broken path.
- `gogo project rm`/`gogo project list` — reads back correctly; a malformed
  `config.json` (`{not valid json!!!`) degrades to an empty project (0 sources), no
  crash.

## FR2 — project knowledge (hands-on + durable TUI test)
- `.knowledge/project-knowledge.md` content confirmed headed exactly as planned:
  `## Domain`, `## How the sources connect`, `## Glossary`, `## Integration
  contracts`; title line `# Project knowledge - sanoma` uses a plain dash.
  Idempotent/non-clobbering confirmed above.
- Config-tab explorer split: `internal/tui/config_tab.go:427-436` renders two labelled
  groups (`project knowledge` / `source knowledge`) from two distinct dirs. Durable
  test `TestViewConfigKnowledgeGroupsSplit` (`internal/tui/config_screen_test.go:454-489`)
  drives `Update`/`viewConfigRight()` directly and asserts the groups are ordered and
  NOT mixed (a source file never appears before the source-knowledge header) — this
  already covers exactly what I would have written as a throwaway; no new test needed.
- `AuthorPlanIntent` (`internal/launch/launch.go:395-422`) confirmed to splice in a
  "READ the project's cross-repo domain knowledge under `<path>`" instruction when
  `knowledgePath` is non-empty, injection-safe (single trailing argv element). Caller
  wiring confirmed: `internal/tui/plans_tab.go:503` passes
  `projects.KnowledgeDir(m.project.Name)`.

## FR3 — project-plan lifecycle + project-UAT gate (hands-on, real binary)
Built two real cross-repo plan fixtures (`sanoma2` project, sources `sanoma-web` +
`sanoma-api`, each a scratch repo with a hand-built `.gogo/work/feature-.../state.md`
carrying the plan's correlation id):
- `gogo plan done <nonexistent-id>` → clean error, exit 1.
- `gogo plan done <id>` on a plan with **zero members** → refuses: "plan ... has no
  work items yet...", exit 1.
- Linked 2 members via `gogo plan add <id> <source>:<slug>`; both members' state.md at
  `awaiting-uat` (not shipped) → `gogo plan done` refuses, **naming both**: "2 of 2
  member(s) not shipped yet: sanoma-web:web-migrate, sanoma-api:api-migrate".
- Shipped **one** member (`status: shipped`) → `gogo plan done` still refuses, now
  **naming only the one remaining**: "1 of 2 member(s) not shipped yet:
  sanoma-api:api-migrate". Proves the guard re-evaluates precisely, not just
  all-or-nothing.
- Shipped **both** members → `gogo plan done` **succeeds**: "accepted project-UAT for
  plan ... all 2 member(s) shipped; plan is now done". Verified the plan `.md` file:
  a `## Project UAT` / `## UAT round 1 - accepted (user, 2026-07-20) - via gogo plan
  done` block was appended, and `status: done` was persisted.
- Re-running `gogo plan done` on the now-done plan → idempotent friendly message
  ("plan ... is already done"), exit 0, no duplicate round.
- Confirmed **read-only**: `find <source>/.gogo -type f` before/after showed only the
  hand-created `state.md` fixtures — `MembersShipped`/`MembersShippedIn` never wrote
  into a source's `.gogo/`.
- `DerivedStatus` (`internal/plans/plans.go:529-534`) and the plans-tab `D` accept flow
  are covered by durable, rigorous `Update`/`View`-driven tests already in the tree:
  `TestPlansTabDerivedAwaitingProjectUAT` and `TestPlansTabAcceptProjectUAT`
  (`internal/tui/plans_tab_test.go:369-456`) — both walk the refuse-then-accept
  sequence through the real huh confirm (`m.binding.confirm` + `finishPlanDone()`),
  asserting the card/detail render `awaiting-project-uat`, the `D` key refuses with a
  named-member status line and does NOT open a confirm while unshipped, and once
  shipped it opens the confirm and completing it flips the plan to `done` with the
  round appended. No new throwaway test needed — this already IS the hands-on/render
  assertion the task called for.
- Cross-project guard (REV-002): covered by the durable
  `TestMembersShippedInCrossProjectGuard` (`internal/plans/plans_test.go:200-238`),
  re-read and confirmed correct (a same-named source's shipped feature in ANOTHER
  project does not satisfy a member; the plan's own project's does; a project-less
  feature still matches by (source, correlation) for the single-repo fallback).

## FR4 — per-source gate-skip flags (hands-on, real binary + stub `claude`)
This is the safety-sensitive one, so I independently reproduced it **outside** the Go
test suite, driving the actual compiled `/tmp/gogo-psg` against a hand-written stub
`claude` on `PATH` that logs its full argv:
- **Flagged source** (`planAcceptanceSkip: true, uatAcceptanceSkip: true` hand-set in
  `config.json`): `gogo go <slug>` printed both opt-in notes (the corrected REV-005
  wording, not "auto-skipped") and launched
  `claude -p "/gogo:go <slug> --skip-acceptance --skip-uat"` — confirmed the whole
  slash command is a **single trailing argv element** (`<<...>>` in the stub's argv
  log), never split across shell-interpretable tokens.
- **Unflagged source** in the same run pattern → launched `/gogo:go <slug>` with **no**
  skip tokens and **no** skip-related print lines — byte-for-byte the base command.
- **Unregistered / lone repo** (no project at all) → `gogo go` still ran cleanly, no
  skip params, no crash — the fallback behaves.
- **Malformed `config.json`** (`{not valid json!!!`) in the data home → `gogo project
  list` degraded to an empty project list (no crash); a `gogo go` against a repo not
  registered there ran with no skip params (graceful degrade).
- **Plan-leg invariant**: ran `gogo plan <new-slug>` from **inside the flagged
  source's repo** (same source that carries both skip flags) — the launched
  `/gogo:plan <slug>` argv carried **no** `--skip-*` tokens at all, confirming
  `goSkipSuffix()` returns `""` for `Kind=="plan"` live, not just in the unit test.
- **Board vs CLI parity**: `internal/tui/move.go:94-102` (`intentFor`) and
  `cli/go.go:178-196` (`cmdGo`/`resolveSourceSkip`) both resolve through the identical
  `projects.SkipForSource` — confirmed by code read (no drift) and by the durable
  `TestBoardGoIntentCarriesSkipParams` (`internal/tui/skip_test.go`), which proves the
  board's go-launch carries the same two tokens for a flagged source, none for
  unflagged, and never for a non-go action (a ship).
- `omitempty` round-trip: a source with both flags false/default writes NO
  `planAcceptanceSkip`/`uatAcceptanceSkip` keys at all in `config.json` (confirmed by
  reading the raw file); a flagged source round-trips both `true` keys; schema stays
  `1` in both cases.
- **Skill docs — verified BY INSPECTION, not a live run** (the LLM behaviour they
  describe cannot execute under `go test` or a stub): read
  `skills/gogo-plan/SKILL.md` (Step 0 parse + Step 6 accept), `skills/gogo/SKILL.md`
  (① plan-acceptance skip block + ⑤/UAT skip block), and `skills/gogo-done/SKILL.md`
  (its own `--skip-uat` note). All three explicitly gate the auto-advance on the
  param being **present** ("Absent the flag → the gate is the hard stop above,
  byte-for-byte" / "Absent → ⑤ stops at `awaiting-uat`, byte-for-byte"), and the
  `--skip-uat` path in `gogo/SKILL.md` explicitly **invokes** `gogo-done` (the single
  recording/emission owner) rather than performing the ship inline — this is the
  REV-003 fix, re-confirmed in place. This satisfies the doc-verification bar the
  task set; it is not, and cannot be, a live gate-skip test.

## Edges / fallbacks
- Unregistered/lone repo `gogo go` → no skip params, no crash (above).
- Malformed `config.json` (both at the project-store level and mid-`gogo go`) →
  degrades gracefully, never a crash (above).
- `gogo plan done` on a nonexistent plan id → clean error (above).
- `gogo plan done` with no `--project` when several projects exist → clean "several
  projects exist... pass --project <name>" error, exit 1 (no silent wrong-project
  pick).
- `gogo plan done --project <nonexistent>` → clean "no plan ... in ..." error.

## Cleanup
All testing ran against `GOGO_DATA_HOME`/`GOGO_CONFIG_HOME` pointed at
`/private/tmp/claude-502/.../scratchpad/gogo-test-psg/` and scratch repos under the
same tree; the real `~/.gogo` was never referenced. No tmux sessions were started
(every launch used the headless `-p` stub path). Verified no new/modified files under
the real `/Users/bartlomiej.zawadzki/repos/gogo/.gogo/` beyond this feature's own
folder. No throwaway `zz_*` test files were needed — durable coverage was already
adequate for every scenario the task asked me to drive through `Update`/`View`.

## Issues found
**None.** Zero new `test/issues.json` entries this round.

## Verdict: PASS

- Gate: **green** (build, `gofmt`, `go vet`, `go test -race ./...`, the three named
  regression tests, version = `0.24.0` everywhere).
- All 5 round-1 review findings: **verified fixed** by direct inspection (not just
  trusting fix_summary).
- FR1-FR4 hands-on: **all pass**, including the safety-sensitive FR4 skip-param
  wiring, independently reproduced outside the Go test suite with a real stub
  `claude` and real argv capture.
- **Confirmed**: FR4's `--skip-acceptance`/`--skip-uat` fire **only** for a flagged
  source's `gogo go` (`/gogo:go`) leg, **never** for an unflagged/unregistered source,
  and **never** on the plan leg (`/gogo:plan`) — verified live, not just by unit test.
- No hands-on/e2e check was blocked; nothing needs a user decision. Done-bar (build +
  unit + e2e green + hands-on done) is **met**. Ready to advance to ⑤ report.
