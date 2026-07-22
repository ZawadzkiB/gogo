# Test round 1 — feature `smart-project-plans` (→ 0.25.0)

**Verdict: PASS with 1 open finding (minor, agent-fixable, CLI-only).** No blocked
hands-on checks (tmux + a compiled `claude` were both available; every check the
plan/review called out was exercised live). Gate green, version confirmed, the
`r`/`gogo plan ready` auto-spawn fan-out verified correct end-to-end with the real
compiled binary. Safe to report; the one finding (TEST-001) is small and does not
block shipping, but should be picked up in a follow-up loop.

## Gate

```
cd cli && go build -o /tmp/gogo-spp .   → clean
gofmt -l .                              → clean
go vet ./...                            → clean
go test -race ./...                     → ok, all 12 packages (internal/textfmt has no tests)
/tmp/gogo-spp --version                 → gogo 0.25.0  (matches .claude-plugin/plugin.json)
```
Re-ran after adding one durable regression test (below); still clean/green.

## What I read first
`plan.md` (FR1-FR4, the design), `decisions.md` (D1-D5 resolutions), `review/review-01.md`
(APPROVE, 3 minor + 1 nit — all marked `fixed` in `review/issues.json`), `.gogo/knowledge/
test-strategy.md` + `testing-tools.md` (the "stubbed claude on PATH" + `TMUX_TMPDIR`-style
tmux isolation this round used), `tech-stack.md`, `non-functional-requirements.md`.

## Level 1 — existing unit suite (read + spot-run, all green)
Read every new/changed test in `internal/plans/plans_test.go` (`TestBriefFor`),
`internal/launch/launch_test.go` (`TestAuthorPlanIntentCarriesSkillAndSourcePaths`,
`TestPlanIntentCorrelation`), `internal/tui/plans_tab_test.go` (17 plans-tab tests
incl. `TestPlansTabAcceptSpawnsPerTarget`, `TestPlansTabAcceptSkipsAlreadySpawned`,
`TestPlansTabAcceptLaunchErrorRecordsNoMember`, `TestPlansTabAcceptSpawnFormMessageDriven`),
and `plan_test.go` (`TestCmdPlanReadyFansOut`, `TestCmdPlanReadyIdempotentOnBoardFeature`,
`TestCmdPlanReadyInvalidTargetsReported`, `TestCmdPlanReadyMissingProjectSources`). These
map 1:1 onto REV-001..004 from the review and all genuinely assert what their names claim
(fire counts, per-source brief isolation, `--skip-acceptance` scoping, phantom-member
avoidance, message-driven huh completion). No rubber-stamped/vacuous assertions found.

## Level 2 — hands-on with the REAL compiled binary (isolated, no real claude spawned)

Per `testing-tools.md`'s "stubbed claude on PATH" technique, extended with an isolated
`TMUX_TMPDIR` (a short path under `/tmp` — the deep scratchpad path breaks tmux's
unix-socket length limit, a harness-only snag, fixed and noted, not a product bug) so a
**real** `tmux new-session` actually fires per spawn without ever touching the user's real
default tmux server or `~/.gogo`. Verified afterward: the real `~/.gogo/projects/` has no
trace of the test projects, and `tmux list-sessions` on the real default socket shows only
the user's own pre-existing session, untouched.

**Setup:** `GOGO_DATA_HOME` → a fresh temp dir; `project add` / `source add` run through
the real binary against two throwaway repo dirs (`.gogo/` stub only); one source's
`planAcceptanceSkip` flipped to `true` in `config.json` (no CLI verb sets it — TUI
config-tab only, matches `projects.Source` schema); a plan file **hand-written** to disk
(front-matter `targets: srcA, srcB` + a `## Source briefs` / `### srcA` / `### srcB` body)
to simulate exactly what the `gogo-project-plan` analyst session would produce.

1. **`gogo plan ready plan-hand0001 --project app` — the core fan-out.** Fired exactly 2
   launches (one per target), captured via a PATH-stub `claude` that records argv (no real
   claude ever ran). Confirmed:
   - `srcA`'s call carries `ALPHA-UNIQUE-MARKER...` only (not `srcB`'s brief) +
     `--correlation plan-hand0001` + `--skip-acceptance` (its `planAcceptanceSkip: true`).
   - `srcB`'s call carries `BETA-UNIQUE-MARKER...` only, `--correlation`, **no**
     `--skip-acceptance`.
   - Each session anchored (`tmux -c <root>`) at that target's OWN source root.
   - Plan file after: `status: active`, `members: srcA:cross-repo-rollout,
     srcB:cross-repo-rollout`.
2. **Idempotent re-run.** Ran `gogo plan ready` again on the same plan: 0 new claude-stub
   calls, stdout `"all 2 target(s) already spawned - nothing to do"`, exit 0.
3. **Project-UAT gate after auto-spawn (FR4).** `gogo plan done plan-hand0001 --project
   app` **refused**, exit 1, naming both unshipped members
   (`srcA:cross-repo-rollout, srcB:cross-repo-rollout`) — the 0.24.0 gate holds after an
   auto-spawned plan exactly as it does after a manually-spawned one.
4. **Targetless plan → zero launches.** A plan with no `targets:` → `"marked plan ...
   ready"`, exit 0, `status: ready`; zero launcher calls (verified via a launch-refusing
   PATH with no `claude` at all — a launch attempt would have surfaced as a stderr error,
   and none appeared).
5. **`BriefFor` fallback (D2).** A plan with a target but **no** `## Source briefs`
   section spawned using the plain body as the goal (`BODY-FALLBACK-MARKER...` appeared
   verbatim in the captured argv) — confirms the hand-authored/`n`-drafted degrade path.
6. **Malformed plan files don't crash the CLI.** Dropped three adversarial files into the
   plans dir: no front-matter fence at all, a totally empty file, and a fence that never
   closes. `gogo plan list` and `gogo plan show <id>` on each: no crash, non-zero never
   forced, sensible defaults (`(untitled)`, `draft`, whole content folded into the body).
7. **REV-002/REV-003 CLI robustness** — verified by direct unit-test re-read (item above)
   rather than re-hands-on (the existing tests already drive the exact code paths:
   no-sources project → `"no sources"` + non-zero; an unresolved `targets:` entry → named
   on stderr + non-zero, not a false "already spawned"; `projects.Load` error path is
   explicit, not swallowed).

### TEST-001 (new, minor, agent-fixable) — found via the hands-on run, not by inspection
While reproducing item 4 I first hit this by accident (a tmux harness issue caused every
launch to fail) and then reproduced it **deterministically** with a clean repro: an
isolated env with **no `claude` anywhere on PATH**, so every target's `planLauncher` call
fails with a real error. `gogo plan ready` printed the correct per-target
`"spawn into srcC/srcD failed: claude CLI not found on PATH..."` stderr lines, but then
**still printed** `"plan plan-fail0001: all 2 target(s) already spawned - nothing to do"`
on stdout and **exited 0** — a false-success summary that directly contradicts the errors
just printed above it. The plan file itself was confirmed byte-for-byte unchanged
(REV-005 discipline holds — no phantom member), so this is not a data-integrity bug, but
it is the same misreport class REV-003 fixed for the *invalid-target* case, just not
extended to the *launch-failure* case — `cli/plan.go` `planReady` has no `failed` counter,
so `spawned==0 && len(invalid)==0` unconditionally reads as "already spawned" regardless
of *why* nothing spawned. The TUI mirror (`plans_tab.go` `finishPlanSpawn`) does **not**
have this bug — it separately tracks `launched`/`failed` and reports `"spawned %d work
item(s) (%d failed)"`. Full repro + proposed fix in `test/issues.json` TEST-001.

## Level 3 — BriefFor / AuthorPlanIntent (unit-verified, table-driven, thorough)
Read `TestBriefFor` (present/absent/multi-section/case-insensitive/no-bleed-into-neighbour
cases) and `TestAuthorPlanIntentCarriesSkillAndSourcePaths` (single injection-safe argv
even with a space in a source path; plain prompt, no leading `/`, no `--correlation` flag;
carries the skill directive, plan path, `.knowledge/` path, every source label+path, the
correlation id in prose). Both genuinely exercise what they claim; independently confirmed
item 5 above (the fallback) live with the real binary.

## Level 4 — the new skill (doc verification — LLM behavior can't run under `go test`)
Read `skills/gogo-project-plan/SKILL.md` in full: `user-invocable: false` frontmatter;
explicit "Write ONLY the given `~/.gogo/` project-plan file... never write, edit, or
scaffold anything under a source repo (no source `.gogo/`, no `.gogo/work/`)"; explicit
"Do NOT run `gogo-plan`... Stop" step; no bash/rm anywhere in the file (it's pure
prose/markdown, grepped for `rm ` — zero hits). `TestSkillsBashNoUnsafeRm` (which globs
`skills/*/SKILL.md`, including this new one) ran green as part of the full suite.
**Verified-by-inspection**, not runnable — flagged per the skill's own rule.

## Level 5 — additive/edges
- `n` / `+` / `c` byte-for-byte: unchanged per `TestPlanListNewAndDelete`,
  `TestPlanDetailCreateWorkItemFiresLauncherOnce`, `planAddTarget` — re-read, still exactly
  today's mechanics (fire-once, single target, no brief/skip logic bleeding in).
- REV-001 (TUI cross-project skip scoping) — the review's finding was already fixed in the
  diff (`finishPlanSpawn` now reads `src.PlanAcceptanceSkip` off the source it already
  resolved via `m.sourceByName`, scoped to `m.project.Sources`). No existing test proved
  the specific "two projects share a source path with opposite flags" scenario the task
  called out, so I added one: **`TestPlansTabAcceptSpawnSkipScopedToFocusedProject`**
  (`cli/internal/tui/plans_tab_cross_project_skip_test.go`, new, durable, kept — not a
  throwaway) — two projects `alpha`/`beta` both link a source at the identical path with
  opposite `PlanAcceptanceSkip`; focusing each in turn and firing `finishPlanSpawn` proves
  the FOCUSED project's flag always wins, never a cross-project bleed. Green under
  `-race`. The CLI mirror (`plan.go` `sourceInProject`) is structurally immune to the same
  bleed (it resolves sources via `projects.Load(project)` for the named project only, so
  it can never see another project's `Source` slice) — verified by code-read, no test
  added (nothing to regress).
- Malformed plan file degrades gracefully — see hands-on item 6 above.
- Post-auto-spawn project-UAT still gates `done` — see hands-on item 3 above (also unit
  `TestPlansTabAcceptProjectUAT`, `TestCmdPlanReadyFansOut` + `planDone`'s own refusal
  tests).

## Doc-sync spot-check
`README.md`, `skills/gogo-cli/SKILL.md`, `docs/cli-contract.md` all describe the new `A`
analyst behavior and `r`/`gogo plan ready` auto-spawn accurately (grepped, matches the
shipped code); `docs/commands.md`/`docs/architecture.md` correctly omit the new skill (it
is `user-invocable: false`, no command-surface change, per the plan's own note).

## Cleanup / isolation confirmation
Every hands-on run used `GOGO_DATA_HOME` + a short isolated `TMUX_TMPDIR` (never the
default tmux socket) + a PATH-stub `claude` (no real `claude` process ever spawned).
Confirmed after the fact: the real `~/.gogo/projects/` carries none of the test projects
(`app`, `app2`, `app3`, `alpha`, `beta`), and `tmux list-sessions` on the real default
socket shows only the user's own pre-existing session — the run left no trace outside the
scratchpad and `/tmp/gogo-spp-tmux-*` (cleaned up via `tmux kill-server` + `rm -rf` at the
end of each script).

## Files touched by this test round
- **New (durable):** `cli/internal/tui/plans_tab_cross_project_skip_test.go` — kept as a
  regression guard, not deleted (see Level 5).
- **New (artifacts, this round):** `test/issues.json`, `test/test-01.md`.
- **Scratch (not part of the repo):** `/private/tmp/.../scratchpad/handson/{run.sh,
  repro2.sh}` — the hands-on scripts, safe to leave or discard.

## Verdict vs the done-bar
Build green · unit green (`-race`) · version pinned · hands-on done at every level the
plan named (CLI end-to-end with the real binary; TUI logic covered by the existing +1 new
unit test; the new skill verified by inspection, as designed) · zero blocked checks.
**PASS** — 1 open minor/agent-fixable finding (TEST-001) does not block report; recommend
picking it up in a follow-up loop (or immediately, at the orchestrator's discretion) before
`/gogo:report`.
