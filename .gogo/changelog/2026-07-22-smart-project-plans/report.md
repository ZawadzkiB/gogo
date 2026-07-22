# analyst-driven project plans + auto-spawn on accept — 0.25.0

**Planning becomes a project-level, Claude-driven act: you pick the project, and the analyst finds
the sources.** The plans-tab `A` trigger now loads a new cross-repo analyst that reads the project's
`.knowledge/` and analyzes the actual source repos, auto-selects the sources the plan needs, and
writes a per-source brief for each — and accepting the plan (`r`) fans out a work item into each
chosen source. Ships **0.25.0**.

## What changed

- **`A` is now an analyst, not a blank editor.** A new `gogo-project-plan` skill
  (`user-invocable: false`) is loaded by the `A` session: it reads the project `.knowledge/` + the
  source repo paths, analyzes each source **read-only**, decides **which sources** the plan needs,
  and writes the `~/.gogo/` plan with a `targets:` list + a `## Source briefs` section (a `### <name>`
  brief per target). You only pick the project — the sources fall out of the analysis. (`AuthorPlanIntent`
  now seeds the session with each source's absolute path + the skill directive, one injection-safe
  argv, a plain prompt.)
- **Accept = create the work items.** `r` (or `gogo plan ready`) now, for each un-spawned target,
  fires one `claude -p /gogo:plan --correlation plan-XXXX` (seeded with that target's brief) — a
  confirm lists the targets, each launch fires once, a member is recorded only on success, and it's
  idempotent (a re-accept or an already-spawned target doesn't re-fire). Per-source `planAcceptanceSkip`
  rides the spawned plan. A targetless plan just marks ready.
- **Additive.** `n` (quick draft), `+` (add target), `c` (spawn one) are unchanged; a hand-authored
  or targetless plan spawns from the body/title as before; the 0.24.0 project-UAT still gates the plan
  `done`.

## Key outcomes

- Multi-repo planning is now a single act: "plan this for the project," and Claude figures out the
  repos + what each needs, then one accept spawns them all — correlated, tracked, and gate-aware.
- No `.gogo` state-contract change; one new additive skill; the CLI still only launches (the skill
  writes the source `.gogo/work/`, never the CLI).

## Decisions

D1 a NEW `gogo-project-plan` skill (the only path giving a strict, parseable `targets:` + briefs
output). D2 briefs in the plan body (`## Source briefs`, no schema change). D3 auto-spawn bound to
the accept step (`r`). D4 additive. D5 anchor at the first trusted source, read the others by
absolute path, write only `~/.gogo`.

## Review / test

- **Review:** APPROVE. The multi-launch auto-spawn (fire-once, member-on-success-only, idempotent,
  confirm-gated, right source/correlation/skip) and the new skill's read-only/write-only contract
  both held. 3 minor + 1 nit fixed.
- **Test:** PASS. The fan-out was verified with the REAL binary + a stub `claude` (right root,
  `--correlation`, per-source `--skip-acceptance`, idempotent, zero when targetless). 1 minor
  (`gogo plan ready` exit-0-on-launch-failure) fixed.
- **Gates:** `gofmt`/`go vet`/`go test -race ./...` green; `gogo --version` → 0.25.0.

## Follow-ups

Tune the analyst skill from live drives · P5 worktrees · `gogo project build` · project-UAT re-plan loop.

Full audit trail: `.gogo/work/feature-smart-project-plans/`.
