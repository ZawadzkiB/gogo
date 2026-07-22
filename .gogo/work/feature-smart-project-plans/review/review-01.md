# Review — smart-project-plans (round 01)

Fresh-eyes review of 0.25.0: the plans-tab `A` becomes a cross-repo analyst
(`gogo-project-plan` skill) that auto-selects target sources and writes per-source
briefs, and accepting a plan (`r` / `gogo plan ready`) auto-spawns a `/gogo:plan`
work item into each target.

**Gates:** `gofmt -l .` clean · `go vet ./...` clean · `go test -race ./...` green
(cli + all internal packages). Version 0.25.0 in `.claude-plugin/plugin.json` and
`cli/main.go` (version_test pinned). Skill-bash `TestSkillsBashNoUnsafeRm` green;
`skills_lint`/`cli_enum` green.

## What I verified holds (the high-risk surface)

- **Auto-spawn fires each un-spawned target EXACTLY once through the launcher seam.**
  `finishPlanSpawn` (plans_tab.go) builds one `spawn{}` per `edit.targets` entry and
  the returned `tea.Cmd` calls `launcher(root, intent)` once each; the CLI mirror
  (`planReady`, plan.go) loops `planLauncher(src.Path, intent)` once per un-spawned
  target. Tests assert 3 fires / 1 fire.
- **REV-005 discipline (no phantom member).** Both paths record `AddMember` +
  `SetStatus(active)` ONLY on `err == nil`; a failed launch `continue`s un-recorded.
  Covered by `TestPlansTabAcceptLaunchErrorRecordsNoMember`.
- **Idempotent.** `unspawnedTargets`/`targetSpawned` skip a target with a recorded
  member OR a board feature carrying the plan id; a re-`r` fans out only the remainder
  (`TestPlansTabAcceptSkipsAlreadySpawned`, `TestCmdPlanReadyIdempotent`).
- **Confirm gates the multi-launch.** `r` on a targeted plan opens a huh confirm bound
  through a heap-stable `*formBinding` (TEST-001); it never launches silently.
- **Targetless → plain `MarkReady`, zero launches** (both TUI and CLI, tested).
- **Correct anchoring + correlation + skip.** Each spawn anchors at its OWN target
  source root (`src.Path`), not the focused one, and carries `--correlation plan-ID`;
  the per-source `--skip-acceptance` rides only where the source opted in (api gets it,
  web does not — asserted).
- **`AuthorPlanIntent` is one injection-safe argv.** Plain prompt (no slash command,
  no `--correlation` flag), directs the session to load `gogo-project-plan`, and
  carries each source LABEL + absolute PATH + `.knowledge/` + planPath, unsplit even
  with a space in a path (`TestAuthorPlanIntentCarriesSkillAndSourcePaths`).
- **`plans.BriefFor` is a robust pure extractor** — right `### <name>` under
  `## Source briefs`, case/heading-level tolerant, stops at the next same-or-higher
  heading, `""` when absent (→ falls back to body/title). No cross-section bleed
  (`TestBriefFor`).
- **New skill safety contract is strict.** `skills/gogo-project-plan/SKILL.md`
  (`user-invocable: false`) bars writing ANY source `.gogo/`, bars scaffolding
  `.gogo/work/`, bars running `gogo-plan`, and states the `targets:` + `## Source
  briefs`/`### <name>` output contract exactly enough for `BriefFor` + the front-matter
  parse. No bash / no unsafe `rm`.
- **Additive invariants.** `n`/`+`/`c` untouched; write-scope stays `~/.gogo/` (members
  + status only); work-item creation is always the launched `/gogo:plan`; the 0.24.0
  project-UAT still gates `done`.
- **Doc-sync.** Plans-tab `A`/`r` and `gogo plan ready` updated in `README.md`,
  `skills/gogo-cli/SKILL.md`, `docs/cli-contract.md`; the new skill is
  `user-invocable: false` (no command surface), so the phase-skill/command enumerations
  are legitimately untouched. No stale "r = mark ready" wording remains.

## Findings (all minor / nit — no open blockers or majors)

| id | sev | title | fix |
|---|---|---|---|
| REV-001 | minor | TUI auto-spawn resolves the per-source skip via `SkipForSource(AllSources(...))` instead of the `src` it already holds — cross-project shared-source path collision can bleed the wrong project's flag; the CLI mirror correctly scopes to `proj.Sources`. `plans_tab.go` finishPlanSpawn. | AGENT-FIXABLE — use `src.PlanAcceptanceSkip` (or scope to `m.project.Sources`). |
| REV-002 | minor | `gogo plan ready` idempotency checks only recorded members (`planHasMember`), not board features — weaker than the TUI's member-OR-spawnedFeature guard; an out-of-band-spawned target could be re-launched (duplicate). Within the plan's member-based contract, but asymmetric. `plan.go` planReady. | AGENT-FIXABLE — accept + document member-scope, or also consult contract features. |
| REV-003 | minor | `gogo plan ready` prints "all N target(s) already spawned - nothing to do" when spawned==0 was actually caused by invalid/unresolved targets (stderr warns, stdout misreports, exit 0); also swallows the `projects.Load` error. `plan.go` planReady. | AGENT-FIXABLE — distinguish already-membered vs invalid; handle the Load error. |
| REV-004 | nit | No test drives the `updateForm -> finishPlanSpawn` completion routing through a real form message (tests call `finishPlanSpawn` directly); the shipped dispatch line + cancel path are unasserted end-to-end. `update.go`. | AGENT-FIXABLE — add a message-driven confirm/cancel test. |

## Verdict

**APPROVE** — no open blockers or majors (0 blocker · 0 major · 3 minor · 1 nit).
The auto-spawn and the new analyst skill are correct, idempotent, injection-safe, and
write-scope-clean; the four findings are small robustness/UX/coverage polish that can
land as follow-ups without blocking the merge.
