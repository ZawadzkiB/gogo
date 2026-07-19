# Decisions — feature `projects-sources-plans`

Forks that needed a human call. gogo records each with options + a recommendation,
then the user's answer as a `RESOLVED` block. Audit trail for pause/resume.

> **Locked (confirmed with the user — NOT re-asked):** (L1) correlation lives in each
> source work item's `state.md` as an additive optional **list** (`correlation:
> [plan-XXXX, ...]`); (L2) **rework** the P1–P4 cockpit toward the corrected model,
> salvaging the reusable infra — do NOT restart. See plan.md → Context / Approach.

**All forks resolved at the acceptance gate on 2026-07-18.**

## D1 — Project data root
- **Question:** Where do home-folder projects live?
- **Options:** A. `~/.gogo/` as the gogo DATA home (`~/.gogo/projects/<name>/`) + a
  `$GOGO_DATA_HOME` test seam. · B. keep `~/.config/gogo` (projects under
  `~/.config/gogo/projects/<name>/`).
- **gogo recommended:** A.
- **RESOLVED → A.** Honours the user's stated `~/.gogo/projects/<name>/` path; a
  `$GOGO_DATA_HOME` seam keeps tests off the real home. `~/.config/gogo` is migrated then
  dormant; per-source locks/sessions/logs stay under each source's `.gogo/resources/cli/`.

## D2 — Project folder layout
- **Question:** Concrete shape of the project config / knowledge / plans.
- **Options:** A. `config.json` + `.knowledge/` + `.gogo/plans/<plan-id>.md`. · B. a literal
  `.config` dotfile (JSON).
- **gogo recommended:** A.
- **RESOLVED → A (`config.json`).** User picked `config.json` — tool-/diff-friendly, reuses
  the defensive JSON parser; ".config" honoured in spirit. Plans stay hand-editable markdown.

## D3 — Spawn mechanism + correlation param
- **Question:** Confirm spawn = launch `gogo:plan` (the SKILL writes the source work item +
  stamps `correlation:`), and how the id is passed.
- **Options:** A. explicit `--correlation plan-XXXX` param parsed by `gogo-plan`. · B. seeded
  prose hint (P4's advisory style).
- **gogo recommended:** A.
- **RESOLVED → A.** Matches the user's "start a new tmux session with claude -p" requirement;
  deterministic, injection-safe (single argv element), not ignorable. The CLI never writes a
  source's `.gogo/work/`.

## D4 — Migration from the flat stores
- **Question:** How to migrate the uncommitted flat `projects.json` (+ P3 drafts / P4 epics)?
- **Options:** A. one-shot, best-effort, non-destructive auto-migration at startup (guarded
  to run once; no new verb). · B. a `gogo migrate` command.
- **gogo recommended:** A.
- **RESOLVED → A.** Simplest; near-empty stores; avoids an enum-sync verb.

## D5 — `project add` / `source add` semantics
- **Question:** How does `project add` create the structure, how are sources added, and which
  board shows on `gogo`?
- **Options:** A. `project add <repo>` → project (basename or `--name`) + `<repo>` as source #1
  (require `.gogo/`); `source add <repo> [--project <name>]` appends (default sole project,
  error if ambiguous); monorepo services = separate sources; board = four-way `chooseBoard`. ·
  B. no `source` verb, re-run `project add` grouped by `--project`.
- **gogo recommended:** A.
- **RESOLVED → A.** The `--project` disambiguator + basename default keep the common case a
  one-liner while supporting multi-project machines and monorepos.

## D6 — Tabs vs modes (TUI key map)
- **Question:** Confirm the tabbed key map replacing modal `C`/`D`/`E`.
- **Options:** A. `tab`/`shift+tab` cycle **board → plans → config**; `n` new plan / `A`
  plan-with-claude (plans tab); `p` cycles source chips (board) / project switcher (config);
  `/` `q` `?` persist; drill/viewer/forms compose within the active tab. · B. keep modal
  screens + add a Plans screen.
- **gogo recommended:** A.
- **RESOLVED → A.** Matches the design's `board · plans · config` tab bar; drafts+epics unify
  into the plans tab. (See D9 — `draft`/`epic` survive as CLI aliases, not screens.)

## D7 — Versioning
- **Question:** What version(s), given released is 0.20.1 and P1–P4 (0.21–0.24) are uncommitted?
- **Options:** A. one minor `0.21.0` "projects, sources & plans" superseding 0.21–0.24 (ship
  A–D together). · B. a minor per shipped phase (A→0.21.0 … D→0.24.0).
- **gogo recommended:** B.
- **RESOLVED → A (one 0.21.0 drop).** User chose to ship A–D as a single `0.21.0` release that
  supersedes the uncommitted 0.21–0.24. Phases A–D remain the *implementation* order (build +
  test between each), but the version bumps ONCE to `0.21.0` at the end; reset the working-tree
  `0.24.0`. Enum-sync + no-unsafe-rm guards green throughout.

## D8 — Plan status lifecycle (user-added at the gate)
- **Phase:** plan
- **Question:** Is a "draft" a separate entity, or a plan status? What statuses does a plan have?
- **User's framing (verbatim intent):** "plan can also be draft or can be ready to implement,
  so plans should also have statuses, like draft and ready."
- **RESOLVED → a plan is ONE entity with a status lifecycle.** `Plan.status` ∈
  **`draft → ready → active → done`**:
  - `draft` — being authored; not yet ready to spawn (the old "draft" concept).
  - `ready` — ready to implement; targets chosen, can be spawned into sources.
  - `active` — ≥1 work item spawned (the old "epic" concept: a plan with members).
  - `done` — all members shipped (derived/terminal; optional in v1).
  The plans tab groups by status (DRAFTS · READY · ACTIVE). Supersedes plan.md FR12's
  `draft|active` — insert `ready` between them. A plan advances `draft→ready` when the user
  marks it ready (or adds a target), and `ready→active` on first spawn.

## D9 — Keep `draft` / `epic` as CLI aliases (user-chosen at the gate)
- **Phase:** plan
- **Question:** Remove the P3/P4 `draft`/`epic` verbs, or keep them?
- **RESOLVED → keep as thin aliases into `gogo plan`.** Because a draft and an epic are just a
  plan in a given status (D8), `gogo draft` and `gogo epic` remain as convenience aliases that
  forward to `gogo plan` (e.g. `gogo draft` ≈ `gogo plan` filtered to `status: draft`;
  `gogo draft new` ≈ `gogo plan new` seeded `draft`; `gogo epic` ≈ plans with members). They
  are enum-synced across the four sources alongside `plan`/`project`/`source`. Adjusts plan.md
  FR17 (which had `plan` fully *supersede* `draft`/`epic`).
