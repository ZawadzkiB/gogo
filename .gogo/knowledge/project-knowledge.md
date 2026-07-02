# Project knowledge

**Purpose:** what this project is — architecture, domains, and the key decisions
the plan phase must respect.

<!-- gogo:meta
Mode: proxy
Source: [ ../../README.md ]
Confidence: high
Generated-by: /gogo:build
-->
> Architecture, domains, key decisions. Source of truth: **../../README.md**
> (the published plugin docs). This file distils it for the pipeline.

## What this project is
**gogo** is a Claude Code **plugin**: a portable, knowledge-grounded development
pipeline — **plan → implement → review → test → report**. The *flow* ships with
the plugin; the *rules* are per-project markdown in `.gogo/knowledge/`. "Same
pipeline everywhere; the behaviour is configuration."

## Architecture
Three layers, all plain markdown (+ a little bash and one vendored JS):

- **`commands/*.md`** — ultra-thin entry points. Orchestration: `build|plan|go|
  status|resume`. **Standalone phase commands** (since 0.2.0): `implement|review|
  test|report` — each runnable alone, each a typed function (validate-in → work →
  validate-out). **Knowledge maintenance** (since 0.3.0): `skills` — audit the
  knowledge line budget + extract on-demand skills. No flow logic; each just
  invokes a skill.
- **`skills/*/SKILL.md`** — the operating manuals (the real logic):
  - `gogo` — the orchestrator (phases, loops, decision gates, feature-folder state).
  - `gogo-plan` ①, `gogo-implement` ②, `gogo-review` ③, `gogo-test` ④, `gogo-knowledge` ⑤.
  - `gogo-contracts` — the pipeline's "type system": JSON-Schema registry +
    portable two-tier validate-in/out gate (since 0.2.0).
  - `gogo-build` — wire `.gogo/knowledge/` from project docs.
  - `gogo-skills` — audit the knowledge line budget; extract bloat into on-demand
    skills (`knowledge` → `.gogo/skills/`, `standalone` → `.claude/skills/`).
  - `gogo-mermaid` — portable diagrams (vendored mermaid, offline viewer).
- **`templates/contracts/*.schema.json`** — the artifact contracts that cross
  phase boundaries: `issues-list`, `phase-result`, `pipeline`, `charts-manifest`.
- **`agents/*.md`** — specialist subagents the orchestrator delegates to:
  `gogo` (orchestrator, hands-off), `gogo-developer` ②, `gogo-reviewer` ③, `gogo-tester` ④.
- **`hooks/`** — `config-check.sh` (SessionStart reminder), `notify.sh`
  (Notification → ntfy/macOS/bell). **`assets/mermaid/`** — vendored UMD build +
  offline `viewer.template.html`. **`.mcp.json`** — bundled Playwright MCP.

## Domains & glossary
- **Knowledge file** — a `.gogo/knowledge/*.md` config file, `proxy` (links the
  project's real doc) or `owned` (gogo authored it). Read at specific phases.
- **Feature folder** — `.gogo/work/feature-<slug>/`: `plan.md` (the contract),
  `adjustments.md`, `state.md`, `decisions.md`, `review-NN.md`, `test-NN.md`,
  `report.md`, `charts/`. The pipeline's memory + audit trail.
- **Phase / gate / loop** — five fixed phases; decision gates pause for the user;
  implement↔review↔test loop until clean (bounded ~3 rounds per finding).
- **Contract / validate-gate** (since 0.2.0) — a typed artifact (`issues.json`,
  `charts/manifest.json`, `result.json`, `pipeline.json`) governed by a JSON Schema
  in `templates/contracts/`. Each phase runs **validate-in** (required inputs exist,
  parse, conform) and **validate-out** (its output conforms) via `gogo-contracts` —
  portable: `jq`/validator if present, else the agent checks against the schema.

## Key decisions (constraints the pipeline must respect)
- **Generic flow, per-project config** — never bake project specifics into the flow.
- **Portability** — core loop needs **no external deps**; mermaid is vendored
  (offline); Playwright/`mmdc`/`jq` are optional and degrade gracefully.
- **Only ever write under `.gogo/`** — never edit a proxied upstream file.
- **Hard gate** — never implement an unaccepted plan.
- **Idempotent build** — re-runs preserve `## gogo overrides` and `Mode: owned`.

## gogo overrides
<!-- gogo-specific notes not in the linked source. Preserved across re-runs. -->
- The repo IS the plugin source; `${CLAUDE_PLUGIN_ROOT}` references resolve to it.
- Installed via marketplace `gogo` → GitHub `ZawadzkiB/gogo`; version in
  `.claude-plugin/plugin.json` must be bumped for installs to detect updates.
- **Knowledge vs on-demand skills (since 0.3.0):** always-read `.gogo/knowledge/*`
  is held to a line budget (OK `<200` / WARN `200-400` / OVER `>400`) so workers
  stay deterministic; `/gogo:skills` extracts bloat into on-demand skills. The
  `.gogo/`-only write rule has **one user-gated exception**: an approved
  `standalone` skill written to `.claude/skills/`. Full model: `docs/architecture.md`.
- **Hosted docs + code-verified discovery (since 0.4.0):** a GitHub Pages docs
  site (Jekyll + `just-the-docs` remote theme, GitHub-built, no local build) lives
  under `docs/` and deploys from branch `main` folder `/docs` (config at
  `docs/_config.yml`) — published at `https://zawadzkib.github.io/gogo/`;
  **code/skills stay authoritative**, the site is generated from them. `/gogo:build` now ends with a **verify-against-code**
  pass: high-signal claims (stack, build/run/test commands, test framework, entry
  points) are cross-checked against the code and **code wins** on conflict
  (correct the gogo summary, never the upstream), recorded in `_discovered.md`.
- **Workspace + changelog + viewer (since 0.5.0):** the feature workspace is
  **`.gogo/work/`** (was `.gogo/plans/`) and the vendored mermaid runtime lives at
  **`.gogo/resources/`** (shared; `/gogo:build` Step 0 auto-migrates legacy layouts).
  Report ⑤ writes a **`report/` bundle** (report.md + a diff-chosen UML set incl.
  the **`use-case`** kind + offline `diagrams.html`). **`/gogo:done`** copies a
  feature's report bundle into the append-only **`.gogo/changelog/<date>-<slug>/`**;
  **`/gogo:view`** opens an offline page with the summary + custom pan/zoom/drag
  diagrams (renderer vendored at `.gogo/resources/viewer/`). `/gogo:report` has a
  **lenient mode** to document past/broken runs. Command set is now **12**.
- **Interactive diagrams + before/after compare (since 0.6.0):** `/gogo:view`'s
  renderer is now **xplan-style** — mermaid lays out, its SVG is parsed into a
  `{nodes,edges}` model and re-rendered as custom node cards with an owned edge
  layer; **drag a node and its edges re-route live**, plus zoom/fit/minimap and a
  **persisted layout** (localStorage + an Export button → `<name>.layout.json`).
  Non-flowchart kinds fall back to the pan/zoom canvas. Plan ① now draws an as-is
  **`charts/before/`** baseline; report ⑤ copies it to **`report/before/`** and adds
  a **before/after** side-by-side comparison; `/gogo:done` **prints a `file://`
  viewer link**. Renderer modules: `assets/viewer/{geometry,viewport,mermaid-parse,
  render,interactive}.js`.
- **View menu + plan bundles + `/gogo:done` work board (since 0.7.0):**
  `/gogo:view` (no arg) now shows a grouped **Work** (each feature's plan + report)
  / **Changelog** (shipped reports) `AskUserQuestion` picker; **plans are viewable
  bundles** rendered **in place** from `plan.md` + `charts/` (D1=A), and plans/
  reports are authored **article-style** (lead summary, bold key parts). `/gogo:done`
  (no slug) classifies all `.gogo/work/*` via the shared **`gogo-status`** work-index
  (shipped / ready-to-ship / in-progress / unfinished) and opens an **interactive
  terminal kanban** — vendored `python3` curses `assets/kanban/board.py` in a tmux
  pane that ships on drop — or, when `python3`/`tmux`/tty are absent (**soft deps**),
  the status-table + `AskUserQuestion` multi-select fallback; shipping stays
  single-sourced. Command set still **12**; version **0.7.0**.
- **Merged + synthesized changelog entries (since 0.8.0):** a changelog entry is a
  **written synthesis, never a copy** — for merged releases AND ordinary single
  ships (supersedes the 0.5.0 "copies the report bundle" behaviour above).
  `/gogo:done` can ship several related features as ONE merged entry at
  `.gogo/changelog/<date>-<name>/`: board/multi-select picks ≥2 → one
  separate-vs-merged gate, or the direct `slug1+slug2` arg pre-answers it; the
  release name is suggested + confirmed (D2), date = newest member. Entries carry
  a **slim set** — synthesized `report.md` + slug-prefixed `.mmd` + `manifest.json`
  with an additive optional **`members[]`** (charts-manifest schema) + `before/`;
  **no `diagrams.html` copy** (`/gogo:view` builds the page from source).
  `gogo-status` classifies a member as shipped via `members[]` even though the
  entry dir is named after the release; `board.py` untouched; the full audit trail
  stays in `.gogo/work/` (linked). Command set still **12**; version **0.8.0**.
- **Board cockpit — action keys + filter + intent protocol v2 (since 0.9.0;
  the curses/tmux TUI was REPLACED by the browser board in 0.10.0 — see next):**
  the `/gogo:done` board is the **pipeline cockpit** — one mode, action keys
  (`v` view · `s` ship · `m` ship-merged · `g` go/resume · `/` live filter ·
  `q` cancel) with per-class guards. Every action is a **single-shot schema-v2
  intent** `{"schema":2, "action", "items"}` written to **`board-intent.json`**
  (renamed from `ship-result.json`; legacy `{"ship":[...]}` still parsed as
  `action: ship`); `gogo-done` executes the intent and **relaunches the board**
  (re-classifying in between) — only `go`/`cancel` end the loop; `board.py` stays
  a **no-mutation selector** with the 0/1/2 exit contract, now **crash-safe**
  (any TUI failure → exit 2 + one-line stderr, routed to the fallback, never
  misread as a cancel). validate-in relaxed: the cockpit opens whenever **any**
  `.gogo/work/feature-*` exists (only zero features stops). The chat fallback
  stays ship-focused (`/gogo:view` + `/gogo:go` cover the rest). Command set
  still **12**; version **0.9.0**.
- **Browser board + simplified done/view lists (since 0.10.0):** the curses/tmux
  TUI (`assets/kanban/board.py`) is **removed**; **`/gogo:xplan`** opens a
  **browser kanban** — a React board (`assets/xplan-board/` src + committed
  `dist/`, a port of xplan's board view) served by a pure-stdlib, localhost-only
  `assets/xplan-board/server.py` (`GET /api/board` polled live, `POST /api/ship`
  writes a schema-v2 intent the orchestrator ships via the gogo-done writer then
  rebuilds; columns plan · in-progress · ready · changelog map 1:1 to the
  `gogo-status` classes). npm/node is a **dev-time** dep only. `/gogo:done` and
  `/gogo:view` now use **simplified list pickers** (a text filter that loops
  until the set fits one `AskUserQuestion`); a multi-select still ships as **one
  merged entry**. Command set now **13**; version **0.10.0**.
