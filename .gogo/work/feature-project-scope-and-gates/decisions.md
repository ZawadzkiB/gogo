# Decisions — feature `project-scope-and-gates`

Each fork lists the options, gogo's recommendation, and the reason. The orchestrator
owns the user's call; record the answer here at acceptance.

## FR1 — `gogo project add` semantics (empty vs path)
**Fork:** does `project add` take a NAME only (always empty), or still accept a repo
path convenience?
- **(A) Dual-mode (RECOMMENDED).** Bare name (`validName`, no separator) → empty project;
  a path (separator / `~` / `.` / resolves to a `.gogo/` dir) → today's project+source
  #1 flow byte-for-byte; a path with no `.gogo/` → today's error. Optional `--source
  <repo>` does both in one shot.
- (B) Name-only + `--source` for the convenience. Cleaner model but silently reinterprets
  the current `add <repo>` positional (PATH→NAME) — the "breaks the single-arg flow
  confusingly" the brief warns against.
- (C) Two subcommands (`project add <name>` empty · `project add-repo <repo>`). More
  surface, more enum-sync, no real gain.

**Recommendation:** **A.** Preserves every existing invocation, adds only the bare-name
case (the user's primary flow), and biases the ambiguous bare token to *name* because
"create empty, add sources later" is the stated intent. Document the name-vs-path rule
in help.
**Decision:** RESOLVED (see Resolutions)

## FR2 — project-knowledge population + skill-read scope
**Fork a — population:** empty scaffold · `gogo project build` LLM flow · seeded template?
- **(RECOMMENDED) seeded template.** `.knowledge/project-knowledge.md` with headed
  sections (domain · how the sources connect · glossary · integration contracts),
  written deterministically at `project add`, idempotent. User fills it (by hand or via
  the author session).
- Defer `gogo project build` (a new command + an LLM in a new path) to a later feature.

**Fork b — skill-read scope now:** how much project-knowledge → plan integration lands?
- **(RECOMMENDED) author-session read only.** `AuthorPlanIntent`'s seed prose reads
  `~/.gogo/projects/<name>/.knowledge/`; the domain context reaches each spawned work
  item THROUGH the authored brief (which seeds the work item's goal body). Zero skill
  change; per-source pipeline stays self-contained.
- Defer a **direct `gogo-plan`-reads-project-`.knowledge/`** cross-boundary read (a skill
  change + a source skill reaching into `~/.gogo/`) — take it only if briefs prove
  insufficient.

**Recommendation:** seeded template + author-read now; both deeper integrations deferred.
**Decision:** RESOLVED (see Resolutions)

## FR3 — project-UAT gate mechanics + where recorded
**Fork a — status:** persist `awaiting-project-uat` vs derive it?
- **(RECOMMENDED) derive at read time.** `active && members>0 && all shipped` ⇒ display
  `awaiting-project-uat`; nothing new is persisted until the accept. No event/poll hook
  needed (the CLI gets no signal when a member ships), no new persisted enum value.
- Persist it — rejected: nothing naturally emits the transition.

**Fork b — the accept + where recorded:**
- **(RECOMMENDED) `gogo plan done <id>` (CLI-owned write).** Refuse unless every member
  shipped (naming unshipped ones); append a `## Project UAT` round to the plan body
  (`## UAT round N — accepted (user, <date>) — via gogo plan done`); `SetStatus done`.
  Plus a plans-tab `D` accept key. A plan is CLI-owned data, so this is a legitimate
  `~/.gogo/` write — no skill needed. The member-shipped check is a READ of each
  member's `state.md` (never a source write).
- Alternative record site: a separate `<id>.uat.md` mirroring the work-item `uat.md`.
  Cleaner parallel but a second store to sync — take it only if the round log grows.

**Fork c — loop or accept-only?** v1 is **accept-only** (no project-UAT re-plan loop). If
the cross-repo whole fails, re-run/open a member, then re-accept. A full loop mirroring
the work-item UAT is future.

**Recommendation:** derive the status; `gogo plan done` + `D` records it in the plan body;
accept-only for v1.
**Decision:** RESOLVED (see Resolutions)

## FR4 — skip-flag enforcement across the CLI/skill boundary (the biggest)
**Fork:** how do CLI-owned per-source skip flags skip SKILL-enforced gates?
- **(a) RECOMMENDED — CLI auto-accepts by LAUNCHING the existing skill.** On the flag,
  the CLI launches `/gogo:accept <slug>` (plan gate) / `/gogo:done <slug>` (UAT gate) at
  the `gogo go` hooks. Zero skill change, zero `.gogo` change; the skills' own gates
  still validate; the flag only supplies pre-declared consent. Honors "CLI reads config,
  skills write state" (the write is still a launched skill).
- (b) CLI passes `--skip-acceptance`/`--skip-uat` to `claude -p`; the skill honors it. A
  frozen SKILL-contract change; couples the skill to a CLI param.
- (c) The work item's `state.md` carries the flag; the skill reads it. A `.gogo`
  state-contract change AND it needs a CLI write into a source's `.gogo/` (the CLI
  cannot; the skill can't see CLI config) — contradictory.

**Recommendation:** **(a).** Only option with no skill and no `.gogo` change. Safety:
default false · per-source (resolved by root like `CapForSource`) · printed every time it
fires · never a global default · still routed through the gated accept/done skill.
**Note the FR3×FR4 interaction:** `uatAcceptanceSkip` removes the per-item UAT; the FR3
project-UAT still gates the whole plan — orthogonal by construction.
**Decision:** RESOLVED (see Resolutions)

## Phasing / versioning
- **(RECOMMENDED)** Phase A (0.24.0) empty projects + project knowledge · Phase B
  (0.25.0) project-plan UAT/lifecycle · Phase C (0.26.0) per-source skip flags. Each is
  additive and independently shippable; A unblocks the user's immediate pain.
- Alternative: ship A+B together as 0.24.0 (both CLI-owned, no skill touch) and C as
  0.25.0. Acceptable if the user wants fewer releases.
**Decision:** RESOLVED (see Resolutions)

## Resolutions (accepted by user 2026-07-20)
- **FR1 = A** — dual-mode `project add`: bare name → empty project; a repo path → today's project+source flow byte-for-byte; optional `--source <repo>`.
- **FR2 = recommended** — seed `.knowledge/project-knowledge.md` template on `project add`; the plan-author session reads it (domain context flows into spawned work items). Deeper `gogo project build` + direct gogo-plan cross-read deferred.
- **FR3 = recommended** — derive `awaiting-project-uat` at read (all members shipped); `gogo plan done <id>` (+ plans-tab `D`) refuses unless every member shipped, appends a `## Project UAT` round to the plan body, flips to `done`. Accept-only for v1.
- **FR4 = B (user chose over rec A)** — the SKILL honors a `--skip` param. The gogo skills (`gogo-plan` plan-acceptance gate; `gogo`/`gogo-done` UAT gate) natively honor `--skip-acceptance` / `--skip-uat`: when passed, auto-record the acceptance / auto-pass UAT instead of stopping. The CLI resolves a work item's SOURCE flags (`planAcceptanceSkip`/`uatAcceptanceSkip`, like `CapForSource`) and appends the param(s) to the launched `/gogo:go`(/`/gogo:plan`) command. This IS a gogo-skill change (additive optional params; absent → today's gated behavior byte-for-byte). Safety: default false, per-source, injection-safe single argv, printed on every fire.
- **Phasing = ALL-IN-ONE 0.24.0** — build A (empty projects + FR1) + B (project knowledge FR2) + FR3 (project-UAT) + FR4 (skip flags, incl. the skill change) and ship as a single `0.24.0`.
