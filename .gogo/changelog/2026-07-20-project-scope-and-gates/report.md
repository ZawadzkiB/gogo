# first-class projects, project-UAT & per-source gate skips — 0.24.0

**A project is now a first-class multi-repo container — no repo required — with its own knowledge, a
project-level UAT, and per-source control over the pipeline gates.** Ships **0.24.0**.

## What changed

- **Empty projects.** `gogo project add <name>` creates an empty project (`~/.gogo/projects/<name>/`
  with `config.json`, `.knowledge/`, `.gogo/plans/`) — NO repo or `.gogo/` needed. Sources (which do
  need `.gogo/`) are added with `gogo source add <repo> --project <name>`. `project add` is dual-mode:
  a bare name → empty project; a repo path → today's project+source-#1 flow, byte-for-byte; `--source`
  does both at once.
- **Project-level knowledge.** Each project gets a seeded `~/.gogo/projects/<name>/.knowledge/
  project-knowledge.md` (domain · how the sources connect · glossary · integration contracts) —
  cross-repo domain knowledge, distinct from each source's per-repo `.gogo/knowledge/`. The config
  tab splits PROJECT vs SOURCE knowledge, and the plan-with-claude author session reads the project
  knowledge so the whole-domain context flows into each spawned work item.
- **Project-level UAT.** When a plan fans work items across repos, `gogo plan done <id>` (or `D` on
  the plans tab) is the project-UAT accept — it **refuses until every member work item is shipped**
  (naming the unshipped ones), then records a Project-UAT round and flips the plan to `done`. A plan
  whose members are all shipped shows `awaiting-project-uat` (derived). Accept-only in v1.
- **Per-source gate skips.** A source's config gains `planAcceptanceSkip` / `uatAcceptanceSkip`
  (default false, toggled in the config tab). When set, the CLI appends `--skip-acceptance` /
  `--skip-uat` to the launched `/gogo:go`, and the gogo skills (`gogo-plan` / `gogo` / `gogo-done`)
  auto-advance that gate via the normal single-owner accept path — pre-declared consent from your
  per-source opt-in, printed every time, never a global default. (The plan leg never carries them.)

## Key outcomes

- Multi-repo work has a real home: a project you can create first and grow with sources, with shared
  domain knowledge and a whole-plan sign-off — plus the ability to turn off gates for repos where you
  don't want them.
- Additive: no `.gogo` state-contract change; the CLI still writes only `~/.gogo/`; the one skill
  change is additive optional params (absent → today's hard gates byte-for-byte).

## Decisions

Dual-mode `project add` · seeded project-knowledge template + author-read · derive
`awaiting-project-uat` + `gogo plan done` (accept-only) · **FR4 = the SKILL honors `--skip-*`** (the
user's choice over CLI auto-fire — the one skill change).

## Review / test

- **Review:** CHANGES-REQUESTED → resolved. The skill change was scrutinized hardest and verified
  sound (injection-safe token, only the `go` leg, single-owner `plan-accepted`/`uat-passed`, absent →
  byte-for-byte). 1 major (a missing test on the gate-skip enforcement) + 3 minor + 1 nit, all fixed.
- **Test:** PASS, no issues. The gate-skip was reproduced with a stub `claude`: the params fire ONLY
  for a flagged source and NEVER on the plan leg; CLI + board share one resolver.
- **Gates:** `gofmt`/`go vet`/`go test -race ./...` green; `gogo --version` → 0.24.0.

## Follow-ups

`gogo project build` (LLM-populate project knowledge) · a project-UAT re-plan loop · plans-tab
grouped-by-project · P5 worktrees.

Full audit trail: `.gogo/work/feature-project-scope-and-gates/`.
