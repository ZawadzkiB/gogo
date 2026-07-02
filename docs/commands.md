---
title: Commands
nav_order: 2
---

# Commands

Every command is an **ultra-thin entry point** — it carries no flow logic, it
just invokes a skill and passes arguments. The logic lives in the skills (the
"operating manuals"). Source of truth: `commands/*.md` and the `skills/*/SKILL.md`
they invoke.

There are **13** commands in four groups: **orchestration** (`build`, `plan`,
`go`, `status`, `resume`), the **standalone phase commands** (`implement`,
`review`, `test`, `report` — each a typed function with validate-in /
validate-out), **ship & view** (`done`, `view`, `xplan`), and **knowledge
maintenance** (`skills`).

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
  `## gogo overrides` section and every `Mode: owned` body. `--force` resets to
  fresh scaffolds.

### `/gogo:plan "<goal>"`

Runs **phase ① (plan) only**. Acts as the orchestrator via the `gogo` + `gogo-plan`
skills.

- **Reads:** `.gogo/knowledge/*` (config gate — stops if missing) and the codebase.
- **Writes:** `.gogo/work/feature-<slug>/` with `plan.md` (incl. the feature's
  functional requirements), `adjustments.md`, `state.md`, and an intended-design
  mermaid chart.
- **Stops for acceptance** — no code is written until you accept. Hard gate.

### `/gogo:go [feature-slug]`

Runs **phases ② -> ③ -> ④ -> ⑤** for an accepted plan. Acts as the orchestrator
in chat, so it can pause at gates.

- **Reads:** `state.md` (refuses unless `plan-accepted` or a resumable in-loop
  state) and the relevant knowledge.
- **Delegates:** ② implement -> `gogo-developer`, ③ review -> `gogo-reviewer`,
  ④ test -> `gogo-tester`; routes findings through the loop (fixable ->
  re-implement; decision -> ask the user; clean/green -> advance) and keeps
  `state.md` current. Bounds implement<->review at ~3 rounds.
- The four phase commands below are the same steps it chains.

### `/gogo:status`

Lists every `.gogo/work/feature-*/` with slug, title, phase, status, iteration
counts, and resume hint; flags any `waiting-for-user` feature with its open
decision. Read-only. It is also the home of the shared **work-index classifier**
(Step A) that labels each feature **shipped · ready-to-ship · in-progress ·
unfinished** — the same classifier `/gogo:done`'s ready-to-ship list and `/gogo:xplan`'s
browser board reuse to decide what is shippable.

### `/gogo:resume [feature-slug]`

Resumes a feature that paused for your decision.

- **Reads:** `state.md` + `decisions.md`.
- **Writes:** appends a `### RESOLVED (user, <date>)` block, clears
  `open-decision`, and re-enters the pipeline at `state.md`'s `resume:` phase.

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
  originals), `report/result.json`, and sets `state.md` to done.
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
ready-to-ship list** (the browser board is [`/gogo:xplan`](#gogoxplan)).

- **Ready-to-ship list (no slug):** classifies every `.gogo/work/feature-*` via the
  shared `gogo-status` Step A classifier (shipped · ready-to-ship · in-progress ·
  unfinished), prints the four-class table for context, then offers the ready-to-ship
  items as a **filterable `AskUserQuestion` multi-select**. It is a plain terminal list —
  no tty, no soft dep, always available. A non-slug arg is a case-insensitive substring
  filter over slug+title; with more ready items than fit one question, it asks a filter
  first. The list *selects members*; the entry-writer does the shipping.
- **Multiple = merge (no extra question):** **selecting multiple items merges them into
  ONE entry**; one pick is one entry — multi-select *is* the merge signal. A `+`-joined
  arg is the same merge signal. For a merged entry gogo suggests a release name from the
  members' common theme and confirms it (you can override). There is **no** separate
  merge-or-split question.
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
  status.
- **Prints:** the `file://` link to each built interactive viewer page (with the
  changelog folder path as a fallback — it never fails the command over the link).
- **Validate-in:** a missing report for a named slug STOPs with "No report found for
  `<feature>` — run `/gogo:report <feature>` first, then `/gogo:done`."; the list (no
  slug) stops cleanly when there are no work items (run `/gogo:plan` first) or none
  ready-to-ship (run `/gogo:report <feature>` first) — never a failure, just guidance.

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

### `/gogo:xplan`

Open the gogo work as a **browser kanban**, via `gogo-xplan`. A React board (ported from
xplan) served by a `python3` **stdlib** server on **localhost** — the terminal
`/gogo:done` list's visual sibling.

- **Board:** four fixed columns **plan · in progress · ready · changelog**, fed by the
  shared `gogo-status` classifier plus the changelog entries. Native HTML5 drag-and-drop,
  a live text filter (over slug + title), a **view** button per card that opens its
  pre-built HTML page, and **mark-done from the board** — check ready cards, or drag a
  ready card onto the changelog column (dragging a selected card ships the whole
  selection). **Multiple = ONE merged entry** (same signal as `/gogo:done`); any other
  column move bounces with a hint (columns are derived state).
- **Transport:** the board is committed static files (`assets/xplan-board/dist/`) — the
  server adds `GET /api/board` (the work index it polls every ~3s) and `POST /api/ship`
  (which writes a schema-v2 `ship-intent.json`, atomically, 202). The orchestrator
  **pre-builds all view pages at launch** (D3=A, reusing the `gogo-view` build), then
  keeps a **long-running** server and **watches for the intent**: each one runs the same
  `gogo-done` synthesis writer, rebuilds `board.json`, and the polling board moves the
  card to changelog **live** (D2=A — multiple ships per session).
- **Reads / writes:** reads the classifier + `.gogo/changelog/*/`; writes
  `.gogo/resources/xplan-board/board.json`, the pre-built `.gogo/resources/view/*.html`,
  and (server-side) `ship-intent.json` + `server.pid`; each ship writes a
  `.gogo/changelog/<date>-<name>/` entry via the writer.
- **Soft dep + degradation:** `python3` is the only dependency and it is **soft** —
  absent, `/gogo:xplan` points you at `/gogo:done`'s list and never fails. A busy port
  rolls to the next free one; a browser that won't open just prints the
  `http://127.0.0.1:<port>` URL. npm/node is **dev-time only** (the `dist/` is committed).
  Localhost only, offline, writes only under `.gogo/`.

## Knowledge maintenance

### `/gogo:skills ["<prompt>"] [--warn N] [--max N] [--include <path>]`

Keep `.gogo/knowledge/*` lean so the pipeline stays deterministic. Runs the
`gogo-skills` skill.

- **No prompt** -> audit / auto-discover: measure each file's body lines (OK
  `<200` · WARN `200-400` · OVER `>400`), discover cohesive extraction candidates,
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
