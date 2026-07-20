# Report — `project-scope-and-gates` (0.24.0)

**Projects become first-class multi-repo containers, with a project-level UAT and per-source gate
skips.** A project no longer needs a repo — `gogo project add <name>` creates an empty container;
sources (which need `.gogo/`) are added separately. Projects carry their own cross-repo
`.knowledge/`; a plan spanning repos gets a project-level UAT before it's `done`; and a source can
opt out of the plan-acceptance and/or UAT gates. Review APPROVE (after fixes), test PASS. Ships **0.24.0**.

## Run status

Plan accepted 2026-07-20 (FR1=A, FR2=rec, FR3=rec, FR4=B). Implement across two passes + a
completion (the second pass was interrupted; a fresh pass finished the skill change, config UI,
plans-tab `D`, version, docs). Review 1 round → CHANGES-REQUESTED (1 major test-gap + 3 minor +
1 nit) → fixed + re-verified. Test 1 round PASS (real binary, isolated; FR4 reproduced with a stub
`claude`). Gate green; `gogo --version` → 0.24.0.

## Planned vs shipped

| FR | Shipped |
|---|---|
| FR1 empty projects (dual-mode) | `gogo project add <name>` → empty project (`config.json sources:[]` + `.knowledge/` + `.gogo/plans/`); a repo path → today's project+source-#1 flow byte-for-byte; `--source` one-shot. Name-vs-path disambiguation. |
| FR2 project knowledge | seeded `.knowledge/project-knowledge.md` (domain · how sources connect · glossary · contracts), idempotent/non-clobber; config-tab splits PROJECT vs SOURCE knowledge; `AuthorPlanIntent` reads the project `.knowledge/` so domain context flows into spawned work items. |
| FR3 project-UAT | `plans.MembersShipped`/`DerivedStatus`/`MarkDone`; `gogo plan done <id>` (+ plans-tab `D`, huh-confirm) REFUSES unless every member work item is shipped (naming unshipped), then appends a `## Project UAT` round + flips `done`. `awaiting-project-uat` derived at read. Accept-only v1. Member check is READ-only (never writes a source's `.gogo/`). |
| FR4 per-source gate skips (D=B, skill change) | `Source.PlanAcceptanceSkip`/`UatAcceptanceSkip` (config-tab toggles); `SkipForSource` resolves by root; `gogo go` appends `--skip-acceptance`/`--skip-uat` to `/gogo:go` (never `/gogo:plan`); the skills (`gogo-plan`/`gogo`/`gogo-done`) auto-advance the gate on the param via the single-owner accept path. Default false, per-source, printed. |
| version | 0.24.0; no new verb; enum-sync + no-unsafe-rm + version-sync green. |

## Decisions

FR1=A dual-mode. FR2=seeded template + author-read (deeper `gogo project build` deferred). FR3=derive
+ `gogo plan done` + accept-only. **FR4=B (user chose the skill honoring `--skip-*` over the CLI
auto-firing rec)** — the one skill change this feature makes (additive optional params; absent →
today's hard gate byte-for-byte).

## Review + test outcomes

- **Review:** CHANGES-REQUESTED → resolved. The skill change was scrutinized hardest and verified
  SOUND (injection-safe token, only the `go` leg, single-owner `plan-accepted`/`uat-passed` paths,
  absent → byte-for-byte). REV-001 (the major — no test on the gate-skip enforcement) + REV-002
  (cross-project guard) + REV-003 (single-owner UAT wording) + REV-004 (doc the direct-invoke path)
  + REV-005 (skip-note over-print) all fixed.
- **Test:** PASS, no issues. Dual-mode, idempotent scaffold, the project-UAT refuse/accept guard,
  and — with a stub `claude` — the skip params firing ONLY for a flagged source and NEVER on the
  plan leg, with the CLI + board sharing one resolver.

## Invariants held

CLI writes ONLY `~/.gogo/` (project config/knowledge/plans; the plan-done round is a CLI-owned plan
write; member-shipped is a READ). The skip is explicit, per-source, default-false, printed. The
skill change is additive optional params; the single event owners (`plan-accepted`, `uat-passed`)
are unchanged. LLM-free read path.

## Follow-ups

`gogo project build` (LLM-populate project knowledge) · a project-UAT re-plan loop (v1 is
accept-only) · plans-tab grouped-by-project. P5 worktrees + the earlier fast-follows remain.

## TL;DR

Empty (name-only) projects with their own cross-repo `.knowledge/`; a project-level UAT (`gogo plan
done` — refuses until all members ship); per-source `planAcceptanceSkip`/`uatAcceptanceSkip` that
the gogo skills honor via `--skip-*`. Ships 0.24.0. Full audit: `.gogo/work/feature-project-scope-and-gates/`.
