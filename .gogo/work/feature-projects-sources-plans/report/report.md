# Report — feature `projects-sources-plans`

- **feature:** projects · sources · plans — the gogo cockpit re-architecture (corrected model)
- **status:** awaiting-uat
- **completed:** 2026-07-18
- **branch / commits:** main (working tree, uncommitted) · n/a — one `0.21.0` drop, not yet committed

**What shipped, in one breath:** the gogo cockpit stopped treating a "project" as a single repo. A **project** is now a home-folder entity at `~/.gogo/projects/<name>/` that links many **sources** (repos/services with their own `.gogo/`), owns project-scoped **plans**, and spawns **work items** into sources by launching `claude -p` — each stamped with the plan's **`correlation:` list in `state.md`**, which the board reads back for `⛓ plan-XXXX` chips and `#plan-XXXX` filtering. It ships as a tabbed TUI (**board · plans · config**) as a single `0.21.0` release that supersedes the never-committed 0.21–0.24 flat model.

## Run status / gaps

**All phases completed; no open issues.** Plan (accepted 2026-07-18, D1–D9 resolved at the gate) → implement (5 rounds, phases A→B→C→D built + tested in order) → review (1 round: 7 findings, all fixed) → test (1 round: 3 findings, all fixed) → report (this bundle). Gates green throughout: `gofmt -l .` clean, `go vet ./...` clean, `go test -race ./...` PASS across every package, `gogo --version` → `0.21.0`, enum-sync + no-unsafe-rm guards green. Zero findings remain `open`/`new` in either [review/issues.json](../review/issues.json) or [test/issues.json](../test/issues.json).

## Summary

The uncommitted P1–P4 epic (0.21–0.24) had built the cockpit on a **flat model where `project == one registered repo`**, with correlation kept in a CLI-side `epics.json` store deliberately *outside* `.gogo`. This feature re-grounds the cockpit on the **correct three-level model** and **reworks** that infrastructure rather than restarting it (the two locked decisions). A home-folder **project** links many **sources**; a project-scoped **plan** (id `plan-<hash>`, status lifecycle `draft → ready → active → done`) targets sources and is **spawned into each by launching `claude -p` `gogo:plan --correlation plan-XXXX`** — the skill (not the CLI) writes the source's work item and stamps an **additive, optional `correlation:` list** into `state.md`. The board reads that list directly (no store overlay) for chips and filtering. The modal `C`/`D`/`E` screens are replaced by a **tabbed TUI** (board · plans · config), and legacy `~/.config/gogo/` data is folded in non-destructively. It all ships as **one `0.21.0`**.

## Planned vs shipped

Shipped substantially **as planned** — all 18 FRs delivered. Five deviations, each a truthful refinement made during implement/review (details in **Decisions & rationale** and the deviations note in [plan.md](../plan.md)):

| FR | Shipped | Deviation |
|---|---|---|
| **FR1** Project entity store (`cli/internal/projects`) | ✅ home-folder `Project{schema,name,description,sources[]}`, defensive reads | — |
| **FR2** Source model | ✅ `{path,name,mainBranch,concurrentWorkItems,color}`; per-project `MaxConcurrent` → per-source `concurrentWorkItems` (0 = unlimited) | — |
| **FR3** Project folder layout + `GOGO_DATA_HOME` seam | ✅ `config.json` + `.knowledge/` + `.gogo/plans/`; `~/.gogo/` data home, env seam | — |
| **FR4** `gogo project add/list/rm` | ✅ creates the structure, source #1, rm never touches a source's `.gogo/` | — |
| **FR5** `gogo source add/rm` | ✅ `--project` disambiguator, monorepo-service = separate source | — |
| **FR6** Migration (one-shot, non-destructive) | ✅ legacy `projects.json` + `drafts/` + `epics.json` folded, guarded, idempotent | **Split into two packages** — `projects.MigrateLegacy()` + `plans.MigrateLegacy()` (import-cycle avoidance) |
| **FR7** Board retagged by source + graceful fallback | ✅ per-source dot+tag, source chips, `p` cycle; lone repo = today's board byte-for-byte | — |
| **FR8** Tabbed navigation board→plans→config | ✅ `tab`/`shift+tab`; modal `C`/`D`/`E` removed | — |
| **FR9** Config tab | ✅ project switcher → sources (`a`/`x`/`e` per-source cap/branch/color) → knowledge explorer; writes only `~/.gogo/` | — |
| **FR10** Plans tab grouped by status | ✅ DRAFTS · READY · ACTIVE sections, `⛓` chips, per-source dots | — |
| **FR11** Plan detail + spawn (`c create work item`) | ✅ target rows, `c` launches `claude -p` `gogo:plan`, CLI never writes a source's `.gogo/work/` | — |
| **FR12** Project-scoped plans store + lifecycle | ✅ `cli/internal/plans` collapsing drafts+epics; **D8** lifecycle `draft→ready→active→done` | `ready` inserted into the planned `draft\|active` (D8, at the gate) |
| **FR13** Correlation in `state.md` (reverses P4 D1=B) | ✅ `parseCorrelationList` → `Feature.Correlations`; absent → nil (parity); template + `docs/cli-contract.md §2` updated | — |
| **FR14** Correlation overlay + `#plan-XXXX` filter | ✅ chips read straight from `state.md`, many-to-many, unknown `#token` = literal match | — |
| **FR15** `gogo-plan` `--correlation` param | ✅ explicit param, single injection-safe argv element; absent → byte-for-byte parity | — |
| **FR16** Add existing work item (retroactive, many-to-many) | ✅ `gogo plan add <id> <source>:<slug>` + plan-detail, re-stamps the list | — |
| **FR17** Command surface + enum-sync | ✅ `project`/`source`/`plan` canonical; enum-synced across all 4 sources | **`draft`/`epic` kept as thin aliases** (D9), not fully superseded |
| **FR18** Versioning — one `0.21.0` drop | ✅ bumped once in `plugin.json` + `cli/main.go`, reset from `0.24.0` | **D7=A** (user overrode the recommended per-phase minors) |

## Implementation

The rework is a **salvage-and-lift**: the defensive store pattern, the board renderer, the launch/tmux machinery, the huh forms, and the `plan-<hash>` id minting were all kept and re-shaped rather than rewritten.

**Stores.** `cli/internal/config` (the flat P1–P4 registry) survives as a **read-only legacy shape + migration source** — not deleted (the minimal green path). The new `cli/internal/projects` owns the home-folder Project entity + per-source Source at `~/.gogo/projects/<name>/config.json` behind a `GOGO_DATA_HOME` seam; `cli/internal/plans` collapses the old drafts + epics stores into one project-scoped plan store (hand-editable markdown front-matter + body, `plan-<hash>` id, `AddMember`/`RemoveMember`/`SetStatus`, the D8 lifecycle).

**The plan → spawn → correlation path (load-bearing).** A plan mints `plan-<hash>` at creation — this *is* the correlation id. In plan detail, `c create work item` on a source S calls `launch.PlanIntent(rootS, body, "plan-XXXX")`, which folds `--correlation plan-XXXX` into the launched command as a **single trailing argv element** (no shell — injection-safe), and launches `claude -p` anchored at S. **The CLI writes nothing under S's `.gogo/work/`.** The `gogo-plan` skill, seeing `--correlation`, writes `feature-<slug>/` and stamps `- **correlation:** [plan-XXXX]` into `state.md`. On the reader side, `contract.LoadProject` runs `LoadRepo` once per source and `parseCorrelationList` lifts the list onto `Feature.Correlations`; the board paints one `⛓ plan-XXXX` chip per id and `#plan-XXXX` narrows to that plan's members across sources. The old `StampEpics`/`Feature.Epics` store overlay and the `contract → epics` import were **removed** — a net simplification (the reader now has no store dependency).

**TUI.** The `mode` switch's `modeConfig`/`modeDrafts`/`modeEpics` became a `tab` field (board/plans/config) with within-tab modes (drill, form, plan-detail). Cards are retagged by `Feature.Source`; source filter chips replace the old `@project` token; the plans tab lists by status; the config tab is a project switcher → sources → knowledge explorer.

**The `A` "author with claude" trigger (reworked at review — REV-002).** `A` mints a fresh draft, then fires a **plain `claude` session** via the new `launch.AuthorPlanIntent` (`ActionAuthor` — **no `/gogo:plan` slash, no `--correlation` flag**), anchored at the project's **first source root** (a trusted repo; the `~/.gogo` project home is used only when the project is sourceless). It edits the `~/.gogo` plan file in place and creates **no** `.gogo/work/` scaffold. `n` stays the quick inline draft.

### Changes (as-built)

| File | Change | Note |
|---|---|---|
| `cli/internal/projects/{projects,migrate}.go` | added | Home-folder Project + Source store; `GOGO_DATA_HOME` seam; `projects.MigrateLegacy` (flat `projects.json` → single-source project) |
| `cli/internal/plans/{plans,migrate}.go` | added | Project-scoped plan store (drafts+epics collapsed); D8 lifecycle; `plans.MigrateLegacy` (drafts + `epics.json` → plans) — separate package to avoid a projects↔plans import cycle |
| `cli/internal/config/config.go` | kept (read-only) | Legacy flat registry retained as the migration source (not deleted) |
| `cli/internal/contract/state.go` | modified | `parseCorrelationList` → `Feature.Correlations` (absent → nil, parity) |
| `cli/internal/contract/contract.go` | modified | `LoadProject(projects.Project)` (per-source `LoadRepo`, stamps `Source`+`Root`); dropped `StampEpics`/`Feature.Epics` + the `contract→epics` import |
| `cli/internal/launch/launch.go` | modified | `PlanIntent(label, body, correlation)`; new `AuthorPlanIntent` + `ActionAuthor` (REV-002); `PlanIntentWithHint` retired |
| `cli/internal/orchestrator/cap.go` | modified | Concurrency cap retargeted to a source's `concurrentWorkItems` |
| `cli/internal/tui/*` | modified | Tabs (board/plans/config); source tags/chips; plans tab + detail; config tab + knowledge explorer; `⛓` chips + `#plan-XXXX`; narrow-width tag fit (REV-006) + count fallback (TEST-002); post-success member/active flip (REV-005) |
| `cli/main.go`, `cli/go.go` | modified | `chooseBoard` four-way; `cmdPlan` store-verb vs bare-slug session coexistence + combined `plan -h` (REV-004/REV-007); `printHelp`; `Version = "0.21.0"` |
| `cli/{project,source,plan,draft,epic}.go` | added | Command handlers; `draft`/`epic` thin aliases into `plan` (D9); `epic list` by membership (REV-003); `draft rm <id>` deletes (TEST-001) |
| `templates/state.template.md` | modified | Optional `correlation:` line + file-list note |
| `docs/cli-contract.md` | modified | §2 documents `correlation:` (additive, optional); command rows re-titled to the alias surface |
| `skills/gogo-plan/SKILL.md` | modified | Accept `--correlation plan-XXXX`, stamp/append; **trailing token is authoritative** (TEST-003) |
| `skills/gogo-cli/SKILL.md`, `README.md` | modified | Rewritten to the shipped tabbed `~/.gogo` model (REV-001) |
| `.claude-plugin/plugin.json` | modified | `version` → `0.21.0` |

## Decisions & rationale

Nine forks were resolved at the acceptance gate (see [decisions.md](../decisions.md)); three further forks were escalated and resolved during the implement/review rounds.

| Decision | Choice | Reason |
|---|---|---|
| **L1** (locked) correlation location | in each work item's `state.md` as an additive list | Self-describing, survives repo moves, one-line additive parse; the list handles many-to-many. Reverses P4's CLI-store D1=B. |
| **L2** (locked) rework vs restart | rework | Salvages the board renderer, defensive store pattern, launch/tmux, huh forms, id minting. |
| **D1** project data root | A — `~/.gogo/` data home + `$GOGO_DATA_HOME` seam | Honours the user's stated path; seam keeps tests off the real home. |
| **D2** folder layout | A — `config.json` + `.knowledge/` + `.gogo/plans/<id>.md` | Tool-/diff-friendly, reuses the defensive JSON parser; plans stay hand-editable markdown. |
| **D3** spawn mechanism | A — explicit `--correlation` param | Deterministic, injection-safe (single argv element), not ignorable (unlike advisory prose). |
| **D4** migration | A — one-shot, best-effort, non-destructive at startup | Simplest; near-empty stores; avoids an enum-sync verb. |
| **D5** add semantics | A — `project add`/`source add` + four-way `chooseBoard` | `--project` disambiguator + basename default keep the common case a one-liner. |
| **D6** tabs vs modes | A — `tab` board→plans→config | Matches the design's tab bar; drafts+epics unify into the plans tab. |
| **D7** versioning | A — one `0.21.0` drop (**user overrode** the recommended per-phase minors) | User chose to ship A–D together superseding the uncommitted 0.21–0.24. |
| **D8** plan status lifecycle (user-added) | one entity, `draft → ready → active → done` | "A draft is a plan in draft status; a plan becomes ready to implement." Inserted `ready`. |
| **D9** keep `draft`/`epic` (user-chosen) | thin CLI aliases into `plan` | A draft/epic is just a plan in a status — keep the convenience verbs, don't remove them. |
| **Fork A** — `gogo plan <slug>` vs store verbs (REV-004) | reserved store-verb set shadows a bare slug; softened the "never ambiguous" claim | A single-token slug that IS a store verb (e.g. a feature named `ready`) resolves to the store; multi-word slugs never collide. Documented, no behaviour change. |
| **Fork B** — migration placement (REV-007 / build) | two packages: `projects.MigrateLegacy` + `plans.MigrateLegacy` | Avoids a `projects`↔`plans` import cycle; each package migrates its own legacy shape. Guarded so a no-op run leaves `~/.gogo` uncreated. |
| **Fork C** — `A` authoring mechanism (REV-002) | a **plain** `claude` session (`ActionAuthor`), not `/gogo:plan` | `gogo-plan` Step 1 unconditionally scaffolds a work item; advisory prose can't reliably stop it (the exact "prose is ignorable" failure D3 cited). A dedicated plain-session intent authors the project plan file with no scaffold. |
| **Fork C.1** — alias semantics (REV-003 / TEST-001) | `epic list` filters by membership (any status); `draft rm <id>` deletes | A just-linked epic stays `draft` (AddMember doesn't flip status), so a status filter would hide it; `draft rm` matches the documented "delete a draft". |

## Review outcome

**Round 1 verdict: CHANGES-REQUESTED → all 7 findings fixed and re-verified in the same round.** The reviewer confirmed every load-bearing invariant held (spawn seam fires exactly once and the CLI never writes a source's `.gogo/work/`; correlation round-trip and injection-safety; single-repo fallback byte-for-byte; the `StampEpics`/`Feature.Epics`/`contract→epics` overlay cleanly removed). Findings (full detail + fix summaries in [review/issues.json](../review/issues.json), snapshot [review-01.md](../review/review-01.md)):

- **REV-001** (major) — README + `gogo-cli` SKILL still documented the superseded P1–P4 UX (dead `C`/`D`/`E`/`e` keys, `~/.config/gogo`, failing `draft edit`/`--to`, `epic add <repo>:<slug>`). **Fixed:** both rewritten to the shipped tabbed `~/.gogo` model.
- **REV-002** (major, needs-user-decision) — `A` relied on advisory prose to stop `gogo-plan` scaffolding a work item — the anti-pattern D3 rejected. **Fixed (orchestrator decision):** `A` now uses `launch.AuthorPlanIntent` (a plain session, no slash, no `--correlation`) anchored at the first source root, editing only the `~/.gogo` plan file.
- **REV-003** (minor) — `epic list` filtered `status==active` but `epic add` never flips status → a just-linked epic vanished. **Fixed:** `epic list` filters by membership/targets, any status.
- **REV-004** (minor) — `gogo plan <slug>` shadows reserved store verbs; the "never ambiguous" comment overstated. **Fixed:** comment/help softened to state the reserved-word caveat.
- **REV-005** (minor) — spawn recorded member + flipped `active` *before* the launch fired → phantom active member on a failed spawn. **Fixed:** member/status transition moved into the launcher-success branch; `launchDoneMsg` reloads the plans.
- **REV-006** (nit) — right-aligned source tag could wrap the name row at narrow widths. **Fixed:** `fitSourceTag` truncates/drops the tag; pinned narrow-width regression test.
- **REV-007** (nit) — `MigrateLegacy` touched `~/.gogo` on a no-op run; `plan -h` showed store help only. **Fixed:** migration guarded to leave `~/.gogo` uncreated when there's nothing to migrate; combined `plan -h`.

## Test outcome

**Round 1 verdict: ISSUES-FOUND (non-blocking) — 0 blockers · 0 majors · 1 minor · 2 nits, all fixed; no regressions on the 7 review-fixed items.** Levels exercised (this is a Go CLI/TUI — no browser): **CLI e2e** against the built `0.21.0` binary with isolated `GOGO_DATA_HOME`/`GOGO_CONFIG_HOME` temp dirs and ≥2 fake source repos (project/source/plan/draft/epic surfaces, migration both stages, error/adversarial paths); the **spawn contract verified live** under a stubbed `claude` + real `tmux` — the whole `/gogo:plan <body> --correlation plan-XXXX` arrived as **one trailing argv element** even with embedded quotes, `$()`, `;`, `|`, a newline, and a `--correlation` lookalike (no shell interpretation → injection-safe under a real spawned process); **correlation round-trip** against 4 hand-written on-disk `state.md` fixtures (one/many/absent/empty-list); **TUI** tab bar / plans grouping / config switcher / single-repo fallback via `Update`/`View`. Findings (see [test/issues.json](../test/issues.json), snapshot [test-01.md](../test/test-01.md)):

- **TEST-001** (minor) — `gogo draft rm <id>` was documented as "delete a draft" but forwarded to plan-rm-target (needs `<source>`) and always exited 2. **Fixed:** `draft rm <id>` now deletes the memberless draft; help rewritten; e2e-verified (`draft rm <id> --project acme` → `deleted plan …`, exit 0).
- **TEST-002** (nit) — multiple correlation chips truncated to an indistinguishable `⛓ plan-…` at narrow widths. **Fixed:** compact `⛓ ×N` count fallback below the fit threshold; full ids at comfortable widths.
- **TEST-003** (nit) — a plan body containing a literal `--correlation`-shaped substring was ambiguous for the skill's NL capture (CLI-side injection-safety itself was solid). **Fixed:** `gogo-plan` Step 0 now treats only the **trailing** token as authoritative.

## Diagrams

The as-built UML set — open [diagrams.html](./diagrams.html) (same folder):

- **class** ([class.mmd](./class.mmd)) — the shipped data model: home-folder Project → many Sources → project-scoped Plans (D8 lifecycle) → WorkItems carrying the additive `correlation[]` list in `state.md`; legacy `~/.config/gogo` folded by the two `MigrateLegacy` seams.
- **sequence** ([sequence.mmd](./sequence.mmd)) — the two authoring entry points (`n` quick draft · `A` plain-claude `AuthorPlanIntent`) and the `c create work item` → `claude -p` `gogo:plan --correlation` → skill stamps `state.md` → board reads `correlation[]` back flow, with the REV-005 post-success member/active flip.
- **activity** ([activity.mmd](./activity.mmd)) — the D8 plan status lifecycle state machine `draft → ready → active → done`.

## Before / after comparison

A plan-time **before** set exists (the flat P1–P4 baseline), copied into [before/](./before/). Only the **class** kind is present in both, so the comparison is class-vs-class; the **sequence** and **activity** kinds are **added** (after only) — there was no runtime/lifecycle diagram in the baseline because the flat model had no plan→spawn→correlation-in-state.md flow and no plan status lifecycle to draw.

**Class — before (flat P1–P4):**

```mermaid
classDiagram
  direction TB
  note for RegistryFile "TODAY (uncommitted P1-P4): FLAT model — project == one registered repo"

  class RegistryFile {
    ~/.config/gogo/projects.json
    +int schema
  }
  class Project {
    +string name
    +string path  "== a single repo root with .gogo/"
    +string color
    +int maxConcurrent  "per-PROJECT cap (P2)"
  }
  class DraftsStore {
    ~/.config/gogo/drafts/&lt;slug&gt;.md  (P3)
    global, repo-agnostic briefs
  }
  class EpicsStore {
    ~/.config/gogo/epics.json  (P4)
    +Epic[] epics
  }
  class Epic {
    +string id  "epic-&lt;hash&gt; (correlation)"
    +Member[] members  "{repo, slugHint}"
  }
  class Feature {
    +string Project  "CLI overlay"
    +string Root
    +string[] Epics  "overlay from EpicsStore (NOT state.md)"
  }

  RegistryFile "1" *-- "many" Project : registry
  EpicsStore "1" *-- "many" Epic : store
  Epic "1" o-- "many" Feature : members (StampEpics overlay)
  Project ..> Feature : LoadProjects (one LoadRepo per repo)

  note for Epic "P4 D1=B: correlation kept in the CLI store, OUT of .gogo/state.md\n(this rework REVERSES it → correlation[] in state.md)"
  note for DraftsStore "drafts + epics are TWO separate global stores\n(this rework COLLAPSES them into project-scoped plans)"
```

**Class — after (as-built 0.21.0):**

```mermaid
classDiagram
  direction TB
  note "AS-BUILT 0.21.0 — corrected model: home-folder Project links many Sources, owns project-scoped Plans; correlation[] LIST lives in each work item's state.md (L1)."

  class Project {
    +string name
    +string description
    ~/.gogo/projects/&lt;name&gt;/config.json
    .knowledge/ (project knowledge)
    .gogo/plans/ (project-scoped plans)
    cli/internal/projects
  }
  class Source {
    +string path  (a repo/service with .gogo/)
    +string name
    +string mainBranch
    +int concurrentWorkItems  "0 = unlimited (was per-project MaxConcurrent)"
    +string color
  }
  class Plan {
    +string id  "plan-&lt;hash&gt; = the correlation id"
    +string title
    +string description
    +string status  "draft | ready | active | done (D8)"
    +Member[] members  "{source, slug}"
    ~/.gogo/projects/&lt;name&gt;/.gogo/plans/&lt;id&gt;.md
    cli/internal/plans
  }
  class Target {
    +string source  "which source this plan targets"
    +string slugHint
  }
  class WorkItem {
    +string slug
    &lt;source&gt;/.gogo/work/feature-&lt;slug&gt;/
    CLI-READ-ONLY (skills write it)
  }
  class StateMd {
    +string phase
    +string status
    +correlation[] plan-XXXX  "ADDITIVE optional LIST (L1)"
  }
  class LegacyConfig {
    ~/.config/gogo/ (projects.json · drafts/ · epics.json)
    cli/internal/config  "kept read-only; folded by MigrateLegacy"
  }

  Project "1" *-- "many" Source : links (config.json)
  Project "1" *-- "many" Plan : owns (.gogo/plans/)
  Plan "1" o-- "1..*" Target : targets sources
  Target ..> Source : resolves to
  Plan "1" ..> "0..*" WorkItem : spawns via claude -p (gogo:plan)
  WorkItem "1" *-- "1" StateMd : carries
  StateMd "correlation[]" ..> "many" Plan : belongs to (many-to-many)
  LegacyConfig ..> Project : projects.MigrateLegacy (one-shot, non-destructive)
  LegacyConfig ..> Plan : plans.MigrateLegacy (drafts+epics → plans)
```

**What changed:** the flat `RegistryFile → Project(path)` (one repo per project) becomes a home-folder **Project → many Sources**; the two separate global stores (`DraftsStore` + `EpicsStore`) **collapse into one project-scoped Plan store**; and correlation moves from the `EpicsStore` overlay (`Feature.Epics` via `StampEpics`, *outside* `.gogo`) into each work item's **`state.md` `correlation[]` list** read directly by the board. The `Feature.Epics` CLI overlay and `epic-<hash>` id are gone; `plan-<hash>` is now both the plan id and the correlation id. The old flat `config` package is retained only as the read-only migration source.

## Knowledge updates

- **`.gogo/knowledge/project-knowledge.md`** (`Mode: proxy` — edited only the gogo-owned `## gogo overrides` region, never the proxied README source nor the `## Custom` section). The four narration entries that described the **uncommitted, never-released** P1–P4 epic as shipped (0.21.0 multi-project cockpit, 0.22.0 config screen+cap, 0.23.0 global plan drafts, 0.24.0 multi-repo epics+correlation) were **replaced by one corrected entry**: the released line was **0.20.1**, and **0.21.0 is the projects · sources · plans cockpit re-architecture** — the corrected model that reworked/replaced the flat `project=repo` P1–P4. This closes the knowledge drift the plan flagged for report ⑤ (and that REV-001 explicitly deferred here).
- **No upstream suggestion needed.** README and `gogo-cli` SKILL.md were already corrected to the shipped model during REV-001 (those are product files, edited in the implement/review phases, not knowledge docs).

## Follow-ups & known limitations

- **P5 opt-in worktrees** — still queued (the roadmap's final phase; out of scope here).
- **`gogo plan <slug>` reserved-word shadow** (REV-004, accepted) — a feature whose slug is a single token equal to a store verb (`ready`, `promote`, `show`, …) can't be launched via `gogo plan <slug>`; use `gogo go`/the board. Documented, no behaviour change.
- **Narrow-width chip legibility** (TEST-002, accepted-as-fixed) — at very narrow card widths multiple correlation chips collapse to a `⛓ ×N` count; full ids show at comfortable widths.
- **`A` authoring is dogfood-asserted via the launcher seam**, not a live `claude` run (test scope) — the intent (plain session, no slash, no `--correlation`, first-source anchor, fires once) is pinned; an end-to-end live authoring pass is a natural UAT check.
- **This was the deferred knowledge reconciliation** — now done (see Knowledge updates).

## Summary (TL;DR)

- **What shipped:** the gogo cockpit re-architected onto the corrected **project → sources → plans** model — home-folder projects (`~/.gogo/projects/<name>/`) linking many sources, project-scoped plans (lifecycle `draft→ready→active→done`) that spawn work items into sources via `claude -p` `gogo:plan --correlation plan-XXXX`, with an additive **`correlation:` list in `state.md`** driving `⛓ plan-XXXX` chips + `#plan-XXXX` filtering, all behind a tabbed TUI (**board · plans · config**). One `0.21.0` release, superseding the never-committed 0.21–0.24 flat model.
- **Review verdict:** CHANGES-REQUESTED → all 7 findings fixed and re-verified; every load-bearing invariant held.
- **Test verdict:** ISSUES-FOUND (non-blocking, 0 blk/0 maj/1 min/2 nit) → all 3 fixed; spawn contract verified live and injection-safe; gates green at `0.21.0`.
- **Follow-ups:** P5 worktrees queued; two accepted cosmetic/edge caveats (reserved-word slug shadow, narrow-width chip count); a live `A`-authoring pass is a good UAT check. See **Follow-ups & known limitations** above.

---

*Audit trail:* [plan.md](../plan.md) · [decisions.md](../decisions.md) · [adjustments.md](../adjustments.md) · [review/issues.json](../review/issues.json) · [test/issues.json](../test/issues.json) · [state.md](../state.md)

---

## UAT round 1 delta — two-mode model + `gogo global` (2026-07-18)

After the first UAT check, the tabbed cockpit was found to be hidden behind project
registration (a lone repo hit the single-repo fallback). Reshaped to an explicit two-mode model
(user-confirmed command surface):

- **`gogo global init`** — initializes the global cockpit home `~/.gogo/` (creates `projects/` +
  the `~/.gogo/config.json` marker via `projects.EnsureHome`); idempotent; writes only `~/.gogo/`.
- **`gogo global`** (/ `gogo global board`) — opens the cross-project tabbed cockpit from anywhere;
  uninitialized → hint to `gogo global init`; 0 projects → hint to `gogo project add`.
- **`chooseBoard` reshaped** (`cli/main.go`): inside a repo → THAT repo's board ALWAYS (dropped the
  case-1 in-project auto-route; `projectOwning` removed); outside → the global cockpit when
  initialized (≥1 project), else a friendly hint. Injected `initialized func() bool` seam keeps all
  branches pure/no-TTY testable.
- **`gogo project add`** auto-initializes the home (`projects.EnsureHome`) so registering is forgiving.

Enum-synced the `global` verb across the four sources; README/SKILL document the two modes. Version
stays `0.21.0`. Gate green (`gofmt`/`vet`/`go test -race ./...`). Deferred fast-follow: a UNIFIED
all-projects board (design 3a) — `gogo global` currently opens the per-project cockpit with the `p`
switcher.
