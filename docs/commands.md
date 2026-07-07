---
title: Commands
nav_order: 2
---

# Commands

Every command is an **ultra-thin entry point** — it carries no flow logic, it
just invokes a skill and passes arguments. The logic lives in the skills (the
"operating manuals"). Source of truth: `commands/*.md` and the `skills/*/SKILL.md`
they invoke.

**Architecture:** commands invoke the **orchestrator**; it runs ② implement
in-context and delegates the fresh-eyes phases to specialist agents (① `gogo-analyst`
· ③ `gogo-reviewer` · ④ `gogo-tester`; ⑤ orchestrator + `gogo-knowledge`), owning the
gates in chat. (`gogo-developer` backs standalone `/gogo:implement` + hands-off runs.)

There are **12** commands in four groups: **orchestration** (`build`, `plan`,
`go`, `status`, `resume`), the **standalone phase commands** (`implement`,
`review`, `test`, `report` — each a typed function with validate-in /
validate-out), **ship & view** (`done`, `view`), and **knowledge maintenance**
(`skills`).

## Orchestration

### `/gogo:build [--force]`

Set up or refresh the project's knowledge config. Runs the `gogo-build` skill.

- **Reads:** the project's existing docs at any depth — Claude / Copilot / Cursor
  / Windsurf / Codex configs, README/CONTRIBUTING/ARCHITECTURE, `docs/`,
  manifests + lockfiles, test/lint/CI configs — plus a full markdown sweep and a
  light pass over in-code doc comments. On a re-run it also reads the existing
  `.gogo/knowledge/*`.
- **Writes:** `.gogo/knowledge/*` (each wired as a **proxy** that links the real
  source, or **owned** and synthesized when none exists) and `_discovered.md`.
- **Now also:** verifies the high-signal distilled facts against the actual code
  and records verified / corrected / unverifiable (see [Discovery](discovery.md)).
- **Idempotent:** re-run anytime — picks up new docs, preserves every
  `## gogo overrides` section, every **`## Custom`** section (**user-owned — copied
  1:1, never rewritten**, in any mode; build reports which it preserved), and every
  `Mode: owned` body. `--force` resets to fresh scaffolds (but still carries any
  `## Custom` over verbatim). The distinction: `## gogo overrides` is gogo-authored;
  `## Custom` is yours, untouchable.

### `/gogo:plan "<goal>"`

Runs **phase ① (plan) only**. Acts as the orchestrator, delegating ① to the
**`gogo-analyst`** agent (the `gogo` + `gogo-plan` skills).

- **Reads:** `.gogo/knowledge/*` — especially `analysis.md` (the analysis
  procedure), `project-knowledge.md`, `tech-stack.md`,
  `non-functional-requirements.md`, `coding-rules.md` (config gate — stops if
  missing) — and the codebase (**code = source of truth**).
- **Writes:** `.gogo/work/feature-<slug>/` with `plan.md` (incl. the feature's
  functional requirements), `adjustments.md`, `state.md`, and an intended-design
  mermaid chart.
- **Stops for acceptance** — no code is written until you accept. Hard gate.

### `/gogo:go [feature-slug]`

Runs **phases ② -> ③ -> ④ -> ⑤** for an accepted plan. Acts as the orchestrator
in chat, so it can pause at gates.

- **Reads:** `state.md` (runs only on `plan-accepted` or a resumable mid-pipeline
  state — `implementing` / `reviewing` / `testing`; **`awaiting-uat` and
  `waiting-for-user` are not runnable** — `awaiting-uat` is the user's UAT gate and a
  mid-UAT re-plan sits at `waiting-for-user` until re-acceptance) and the relevant
  knowledge.
- **Runs ② implement in-context** (warm across the fix loop, no re-spawn/re-read)
  and **delegates** ③ review -> `gogo-reviewer`, ④ test -> `gogo-tester`; routes
  findings through the loop (fixable -> re-implement in-context; decision -> ask the
  user; clean/green -> advance) and keeps `state.md` current. Bounds
  implement<->review at ~3 rounds.
- The four phase commands below are the same steps it chains.

### `/gogo:status`

Lists every `.gogo/work/feature-*/` with slug, title, phase, status, iteration
counts, and resume hint; flags any `waiting-for-user` feature with its open
decision. Read-only. It is also the home of the shared **work-index classifier**
(Step A) that labels each feature **shipped · ready-to-ship · in-progress ·
unfinished** — the same classifier the `/gogo:done` work board reuses to decide what
is shippable.

### `/gogo:resume [feature-slug]`

Resumes a feature that paused for your decision — including a feature at the **UAT gate**
(`awaiting-uat`) when you have feedback rather than a `/gogo:done` acceptance.

- **Reads:** `state.md` + `decisions.md` (and, at the UAT gate, `plan.md` + `uat.md`).
- **Writes:** appends a `### RESOLVED (user, <date>)` block, clears
  `open-decision`, and re-enters the pipeline at `state.md`'s `resume:` phase. At the UAT
  gate it folds your feedback into the loop — the orchestrator hands it to `gogo-analyst`,
  which records the `uat.md` round and adjusts `plan.md`; you re-accept and `/gogo:go`
  reruns ②→⑤ on the same work item (see [Flow → UAT](flow.md)).

## Standalone phase commands

Each is a thin, idempotent entry point to its phase skill that **validates its
inputs** before working and **validates its outputs** before hand-off, via the
[contract layer](contracts.md). `/gogo:go` chains these same commands.

### `/gogo:implement [feature-slug] [--issues <path>]`

Phase ② via `gogo-implement` (delegates to `gogo-developer`).

- **Reads:** `plan.md` (accepted), `coding-rules.md`, `tech-stack.md`; with
  `--issues <path>`, a `review/issues.json` or `test/issues.json`.
- **Writes:** code, the as-built `charts/` set + `charts/manifest.json`,
  `implement/result.json`. In `--issues` mode it fixes the `open`/`new` issues
  and writes back `status: fixed`, `fix_summary`, `fixed_in_round`.

### `/gogo:review [feature-slug]`

Phase ③ via `gogo-review` (delegates to `gogo-reviewer`).

- **Reads:** `plan.md`, `code-review-standards.md`, `coding-rules.md`,
  `non-functional-requirements.md`, the as-built `charts/manifest.json`, any prior
  `review/issues.json`.
- **Writes:** the living `review/issues.json` (open -> fixed/verified, append
  `new`), a `review-NN.md` snapshot, `review/result.json`. Routes: open issues ->
  implement with `--issues`; clean -> test.

### `/gogo:test [feature-slug]`

Phase ④ via `gogo-test` (delegates to `gogo-tester`).

- **Reads:** `plan.md` (Tests section), `testing-tools.md`, `test-strategy.md`,
  `tech-stack.md`, `non-functional-requirements.md`, the as-built
  `charts/manifest.json`, any prior `test/issues.json`.
- **Writes:** the living `test/issues.json`, a `test-NN.md` snapshot,
  `test/result.json`. Routes: open issues -> implement with `--issues`;
  all-green -> report.

### `/gogo:report [feature-slug]`

Phase ⑤ via `gogo-knowledge`. For an all-green feature — **and** for a past or
broken run.

- **Reads:** `plan.md`, `state.md`, `review/issues.json`, `test/issues.json`,
  `charts/manifest.json`, the gogo-owned `.gogo/knowledge/*` summaries.
- **Writes:** the finalized as-built `plan.md`, the `report/` bundle
  (`report/report.md` + the as-built UML set + `report/diagrams.html` +
  `report/manifest.json`), updated gogo-owned knowledge docs (never the proxied
  originals, and never a `## Custom` section), `report/result.json`, and sets
  `state.md` to **`awaiting-uat`** (the UAT gate — no longer `done`).
- **Strict vs lenient:** in-pipeline (right after a green ④) it keeps a strict
  validate-in gate. Run **standalone on a past/broken/incomplete run** it does
  **not** refuse — it synthesizes a best-effort `report/report.md` from whatever
  artifacts exist and clearly marks which phases ran and what's still open (a "Run
  status / gaps" section). `plan.md` is the one true prerequisite; without it, STOP.

## Ship & view

### `/gogo:done [feature-slug | slug1+slug2+...]`

Ship report-complete features into a high-level changelog, via `gogo-done`. The
explicit post-report "this is the end" gate. A **slug** ships that one;
**`slug1+slug2+...`** ships those as ONE merged release entry; **no slug opens the
work board**.

- **Work board cockpit (no slug):** classifies every `.gogo/work/feature-*` via the
  shared `gogo-status` Step A classifier (shipped · ready-to-ship · in-progress ·
  unfinished) and, from the four-class table, lets you act with **action keys** — an
  **interactive terminal kanban** (`assets/kanban/board.py` in a tmux pane; `python3`
  + `tmux` are soft deps) when the tooling and a tty are present, otherwise a
  **status table + `AskUserQuestion` multi-select** ship fallback (never fails over the
  board). Keys: **space/enter** select a ready-to-ship card, **v** view the focused card
  (any class), **s** ship the selection separately, **m** ship it merged (≥2), **g**
  run/resume the pipeline on an unbuilt card, **/** filter by text (Esc clears), **q**
  cancel. Each key writes a single-shot **intent** `{schema:2, action, items}`; the
  orchestrator executes it (view build / ship writer / pipeline handoff) and
  **relaunches** the board — `go` ends the loop, `cancel` stops. The board only
  *collects intents*; it never mutates gogo state.
- **Merge gate:** when you ship merged (`m`), or a fallback selection is **≥2** slugs,
  one `AskUserQuestion` — ship **separately** (N entries) or **merged** (1 entry). A
  `+`-joined arg pre-answers *merged*; an explicit `s` pre-answers *separate*; a single
  slug never asks. For a merged entry gogo suggests a release name from the members'
  common theme and confirms it (you can override).
- **`/gogo:done` IS the UAT acceptance (0.11.0 — the plan-gate symmetry).** Phase ⑤ now
  leaves a feature at `status: awaiting-uat`; running `/gogo:done` is what accepts the UAT
  gate — **no extra confirmation question is asked** (mirroring how accepting a plan
  unlocks `/gogo:go`). It records a one-line accept round in the member's `uat.md`
  (`## UAT round N — accepted (user, <date>) — via /gogo:done`), emits a `uat-passed`
  event, and ships. If instead you have questions or issues, don't run `/gogo:done` —
  describe them and the orchestrator's UAT loop re-plans the SAME item (see
  [Flow → UAT](flow.md) and `/gogo:resume`).
- **Every entry is a synthesis, not a copy.** `report.md` is **written** — a
  high-level summary of *what was changed/done/implemented* (lead paragraph, key
  outcomes, one-line decisions, a review/test verdict, a member table + per-member
  section when merged), with a **link back** to each member's `.gogo/work/` folder for
  the full audit trail. No full-report duplication.
- **Reads:** for each member, `.gogo/work/feature-<slug>/report/report.md` (required —
  the synthesis source) + the `report/*.mmd`, `report/manifest.json`, and `before/`
  set.
- **Writes:** the synthesized entry to `.gogo/changelog/<YYYY-MM-DD>-<name>/` — the
  written `report.md`, the **slug-prefixed** `.mmd` set, a merged `manifest.json`
  carrying a **`members[]`** array, and the merged `before/` set (append-only,
  idempotent; **no `diagrams.html` copy** — the viewer builds from source); builds the
  interactive viewer page under `.gogo/resources/view/` (best-effort, reusing the
  `gogo-view` build); and sets **each member's** `state.md` to a terminal `shipped`
  status. Board mode also writes runtime scratch under `.gogo/resources/kanban/`
  (`board.py`, `work-index.json`, `board-intent.json`, `board-exit.code`).
- **Prints:** the `file://` link to each built interactive viewer page (with the
  changelog folder path as a fallback — it never fails the command over the link).
- **Validate-in:** a named slug must be **report-complete** *and* at the UAT gate
  (`status: awaiting-uat`; a pre-0.11 `done`/`shipped` member is accepted for
  back-compat). A missing report STOPs with "No report found for `<feature>` — run
  `/gogo:report <feature>` first, then `/gogo:done`."; board mode opens the cockpit
  whenever **any** feature exists (view `v` and go `g` are useful with nothing
  ready-to-ship) and stops only when there are zero features.

### `/gogo:view [changelog-entry | feature-slug[:plan|:report]]`

Open a gogo **plan or report** as a self-contained, offline **interactive webpage**,
via `gogo-view`.

- **Reads:** the **plan** bundles (`.gogo/work/feature-*/plan.md` + `charts/`,
  viewed in place — D1=A) and the **report** bundles under `.gogo/changelog/*/` and
  `.gogo/work/feature-*/report/` (incl. a `before/` set, which triggers compare
  mode); the vendored `.gogo/resources/` viewer assets. With no resolvable arg it
  presents a grouped **Work** (each feature's plan + report) / **Changelog** (shipped
  reports) picker; an explicit `<slug>`, `<slug>:plan`, `<slug>:report`, or changelog
  entry resolves directly.
- **Writes:** a built page under `.gogo/resources/view/` (the `plan.md` / `report.md`
  summary as readable HTML + its mermaid diagrams made **interactive**; no network, no
  build) and opens it (`open`/`xdg-open`, best-effort; prints the `file://` path on
  failure).
- **Interactive rendering:** flowchart-family diagrams (`flow` + `use-case`) get an
  xplan-style **rich renderer** — custom token-styled node cards you can **drag**
  with edges that **re-route live**, plus **zoom / fit / minimap** and a persisted
  layout (dragged positions auto-save to `localStorage`; an **export** control
  downloads the portable `<name>.layout.json` sidecar). Other kinds
  (`sequence` / `class` / `stateDiagram`) fall back to a **pan / zoom / drag**
  canvas. A bundle carrying a `before/` set renders **before | after side by side**
  (compare mode).

## Knowledge maintenance

### `/gogo:skills ["<prompt>"] [--warn N] [--max N] [--include <path>]`

Keep `.gogo/knowledge/*` lean so the pipeline stays deterministic. Runs the
`gogo-skills` skill.

- **No prompt** -> audit / auto-discover: measure each file's body lines (OK
  `<200` · WARN `200-400` · OVER `>400`; a user-owned `## Custom` section is
  excluded from the count and is never an extraction candidate — like
  `## gogo overrides`, never proposed or rewritten), discover cohesive extraction
  candidates,
  classify each as a `knowledge` skill (-> `.gogo/skills/`) or a `standalone`
  skill (-> `.claude/skills/`), and **propose them, then STOP for per-candidate
  approval** before writing anything.
- **`"<prompt>"`** -> directed: extract exactly the section you name.
- **`--warn N` / `--max N`** -> override the 200 / 400 thresholds.
- **`--include <path>`** -> also audit a path outside `.gogo/` (report-only,
  never extracted).
- On approval it writes each skill's `SKILL.md` (+ optional `scripts/` /
  `.env.example`), replaces the parent section with a `**Load when:**` pointer,
  and updates `.gogo/skills/index.md`. The only write outside `.gogo/` is an
  approved standalone skill's `.claude/skills/<slug>/`. See
  [Discovery](discovery.md#knowledge--on-demand-skills) for the budget rationale.
