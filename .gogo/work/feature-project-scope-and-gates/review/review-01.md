# Review round 01 — project-scope-and-gates (→ 0.24.0)

Fresh-eyes review of the two implement passes + completion. Reviewer did not write the
code. Base: `git diff HEAD` on `main` (26 files, +1380/-61). Standards read:
`code-review-standards.md`, `non-functional-requirements.md`, `coding-rules.md`.

## Gates (all green)
- `gofmt -l .` clean · `go vet ./...` clean · `go build ./...` clean.
- `go test -race ./...` green (every package).
- `TestCLICommandEnumerationInSync`, `TestSkillsBashNoUnsafeRm`, `TestVersionMirrorsPlugin` green.
- Version bumped everywhere: `cli/main.go` + `plugin.json` = `0.24.0`; docs/cli-contract.md
  + README + skills/gogo-cli enum-synced (the new `gogo plan done` verb is in every list).
- Write-scope invariant holds: every new write path targets `~/.gogo/` only
  (`EnsureProjectHome`/`SeedProjectKnowledge`/`MarkDone`); member-shipped checks READ a
  source's `state.md` via the contract reader, never write a source's `.gogo/`.

## The skill change (FR4=B — scrutinised hardest)
The user-chosen option (b): the CLI appends `--skip-acceptance`/`--skip-uat` to `/gogo:go`
and the skills honor them. Verified:
- **Injection-safe capture.** Both tokens are fixed `[a-z-]` literals appended INSIDE the
  single trailing argv element (`launch.SkipParams`), never a shell string — same
  discipline as `--correlation`. The plan leg never carries them (`goSkipSuffix` returns
  `""` for `Kind=="plan"`; `PlanIntent`/promote don't append), so `--correlation` stays the
  final token on `/gogo:plan`. No double-append across the headless vs `--attach` paths.
- **Pre-declared consent, not a global default.** The param only reaches a skill because
  `resolveSourceSkip`→`projects.SkipForSource` read the SOURCE's opt-in
  `planAcceptanceSkip`/`uatAcceptanceSkip` (default false; unregistered root → no skip, no
  note). Both go-launch paths (CLI `cmdGo` and board `intentFor`) share the one resolver.
- **Absent param → today's hard gate byte-for-byte.** Every skill block states this
  explicitly; `SkipParams`/`goSkipSuffix` return `""`.
- **Single recording path.** `--skip-acceptance` records acceptance via gogo-plan's ONE
  path (`state.md`→`plan-accepted` + the single-owner `plan-accepted` event). See REV-003
  for a wording risk on the `--skip-uat` side.

## Findings
| id | sev | pri | status | title |
|----|-----|-----|--------|-------|
| REV-001 | **major** | P1 | open | FR4 gate-skip enforcement wiring has no direct test coverage |
| REV-002 | minor | P2 | open | `memberFeature` lacks the REV-002 cross-project scope guard `spawnedFeature` has |
| REV-003 | minor | P2 | open | `gogo/SKILL.md` `--skip-uat` wording risks a divergent second `uat-passed` emitter |
| REV-004 | minor | P3 | open | `gogo-plan` `--skip-acceptance` branch is unreachable via current CLI wiring |
| REV-005 | nit | P3 | open | `gogo go` prints "auto-skipped" even when no gate is skipped this run |

### REV-001 (major, AGENT-FIXABLE) — FR4 enforcement untested
The riskiest FR (auto-skips user gates) ships with no test around the source→param
resolution. Nothing covers `projects.SkipForSource`, `launch.SkipParams`,
`orchestrator.goSkipSuffix` (incl. the plan-leg `""` invariant), `resolveSourceSkip`
(`cli/go.go`), or the board `intentFor` append. The one FR4 test only proves the huh form
persists the two bools. The omitempty round-trip is not directly asserted either. Behaviour
was traced and is correct today, but `plan.md` Phase-C promised a go-path test that the skip
fires exactly under the flag and never otherwise — the exact regression net a gate-skip
feature needs. Add unit + go-path tests (both launch paths).

### REV-002 (minor, AGENT-FIXABLE) — matcher divergence
`plans.memberFeature` matches only by `(Source, correlationID)`; the sibling
`tui.spawnedFeature` also requires `f.Project == m.project.Name` (REV-002). `MembersShippedIn`'s
own comment says callers spanning projects must pass a project-scoped repo, but the TUI passes
the workspace-spanning `m.repo` in unified/global mode. Safe in practice (plan ids are unique
hashes) but relies on id-uniqueness instead of an explicit scope guard. Mirror the guard or
pass `LoadProject(*m.project)`.

### REV-003 (minor, AGENT-FIXABLE) — single-owner wording
`skills/gogo/SKILL.md` tells the orchestrator to "perform the ship exactly as `/gogo:done`
does … emit the single-owner `uat-passed` event" — read literally, an inline second emitter,
which would break "only gogo-done owns `uat-passed`." `skills/gogo-done/SKILL.md` correctly
frames it as "the orchestrator auto-invokes this ship." Reword gogo/SKILL.md to say it
INVOKES the gogo-done skill.

### REV-004 (minor, AGENT-FIXABLE) — unreachable skill branch
`skills/gogo-plan` parses+honors `--skip-acceptance`, but the CLI never appends it to a
`/gogo:plan` command (only `/gogo:go`, handled by the gogo orchestrator gate). Harmless (no
double-accept) but dead relative to the wiring, and decisions.md FR4=B implied the plan leg
would carry it too. Either document it as direct-invocation coverage or remove the branch.

### REV-005 (nit, AGENT-FIXABLE) — misleading note
`cmdGo` prints "plan-acceptance auto-skipped for source X" on every `gogo go` for a flagged
source, even when the feature is already past the plan gate (nothing skipped this run).
Soften the wording or gate it on the classified status.

## What is solid
- FR1 dual-mode disambiguation is well-covered (`isPathArg`, bare-name/path/`--source`/
  no-`.gogo/` error, idempotent re-add) and `--source`+path-positional is correctly rejected.
- FR2 scaffold is idempotent + non-clobbering; `AuthorPlanIntent` reads the project
  `.knowledge/` injection-safely; config-tab knowledge split is tested.
- FR3 derive-at-read (`DerivedStatus`), refuse-with-names guard (`MembersShipped`/`MarkDone`),
  the CLI-owned plan write, and the plans-tab `D` huh-confirm (heap-stable `*formBinding`,
  re-guard in `finishPlanDone`, `pendingPlanDone` wired through `updateForm`/`cancelForm`) are
  correct and tested. `Feature.Shipped()` keys on `status` (TEST-004), not artifact presence.
- Contract/schema untouched (`omitempty`, schema stays 1); no unsafe skill-bash; the huh
  bools follow the TEST-001 heap-stable-pointer rule.

## Verdict
CHANGES-REQUESTED (1 open major: REV-001 — add the FR4 gate-skip enforcement tests).
