# Report — `smart-project-plans` (0.25.0)

**Planning becomes a project-level, analyst-driven act: pick the project, and Claude finds the
sources.** `A` now loads a cross-repo analyst that reads the project's `.knowledge/` + analyzes the
actual source repos, auto-selects the sources the plan needs, and writes a per-source brief for
each — and accepting the plan (`r`) auto-spawns a work item into each chosen source. Review APPROVE,
test PASS. Ships **0.25.0**.

## Run status

Plan accepted 2026-07-22 (D1=a new skill, D2=briefs-in-body, D3=a auto-spawn on `r`, D4=additive,
D5=a anchor first source). Implement 1 round. Review 1 round APPROVE (0 blocker/major; 3 minor +
1 nit fixed). Test 1 round PASS (real binary + stub `claude`; 1 minor TEST-001 fixed). Gate green;
`gogo --version` → 0.25.0.

## Planned vs shipped

| FR | Shipped |
|---|---|
| FR1 smart `A` (analyst-loaded, analyzes sources, auto-targets) | New `skills/gogo-project-plan/SKILL.md` (`user-invocable: false`) — reads project `.knowledge/` + the CLI-seeded source paths, analyzes each source repo read-only, writes the `~/.gogo/` plan with `targets:` front-matter + a `## Source briefs`/`### <name>` per target. `launch.AuthorPlanIntent` now carries `[]SourceRef{Label,Path}` + directs the session to load that skill (one injection-safe argv, plain prompt). |
| FR2 auto-spawn on accept | `r` / `gogo plan ready` → for each un-spawned target, `PlanIntent(title, BriefFor||body, planID)` + per-source `--skip-acceptance`, fire-once through the launcher seam, record member + `active` on success only. huh-confirm gates the fan-out; targetless → plain `MarkReady`; idempotent (member OR correlated feature). `plans.BriefFor` extracts each target's brief. |
| FR3/FR4 additive + gate-respecting | `n`/`+`/`c` byte-for-byte; a targetless/hand-authored plan spawns from body/title; the 0.24.0 project-UAT still gates `done`; per-source `planAcceptanceSkip` rides the spawned `/gogo:plan`. |
| version | 0.25.0; no new command verb (reuses `A`/`plan ready`); enum-sync + version-sync + no-unsafe-rm green. |

## Decisions (D1–D5 + phasing)

D1=a a new `gogo-project-plan` skill (strict parseable `targets:` + briefs — only this guarantees the
shape auto-spawn parses). D2=briefs in the plan body under `## Source briefs` (no schema change).
D3=a auto-spawn bound to the accept step (`r`). D4=additive. D5=a anchor at the first (trusted)
source, read the others by absolute path, write only `~/.gogo`. Shipped FR1+FR2 as one `0.25.0`.

## Review + test outcomes

- **Review:** APPROVE. The two high-risk areas held — the multi-launch auto-spawn (fire-once,
  member-on-success-only, idempotent, confirm-gated, right source/correlation/skip) and the new
  skill's read-only-sources / write-only-`~/.gogo` contract. 3 minor (REV-001 cross-project skip
  bleed · REV-002 CLI idempotency vs board features · REV-003 misreport/swallowed error) + 1 nit
  (form-completion test) — all fixed.
- **Test:** PASS. The fan-out verified with the REAL binary + a stub `claude`: one `/gogo:plan
  --correlation` per un-spawned target at its own source root, `--skip-acceptance` only where opted
  in, idempotent, zero when targetless, project-UAT still refusing. TEST-001 (exit-0 on launch
  failure) fixed — `gogo plan ready` now exits non-zero + names the failed targets.

## Invariants held

CLI writes ONLY `~/.gogo/` (the plan file + member/status); work-item creation is ALWAYS a launched
`claude -p /gogo:plan` (skill writes the source's `.gogo/work/`), never the CLI; the analyst session
reads sources read-only + writes only the project plan. Additive; heap-stable `*formBinding`
confirm; LLM-free read path.

## Follow-ups

The live analyst quality (the session actually choosing good targets/briefs) is a runtime judgment
you'll tune by driving `A`. P5 worktrees · `gogo project build` · project-UAT re-plan loop remain.

## TL;DR

`A` = a cross-repo analyst that picks the target sources for you and writes per-source briefs;
accepting (`r`) fans out a correlated work item into each. Ships 0.25.0. Review APPROVE, test PASS.
Full audit: `.gogo/work/feature-smart-project-plans/`.
