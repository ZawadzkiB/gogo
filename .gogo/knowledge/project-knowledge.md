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
  accept|status|resume`. **Standalone phase commands** (since 0.2.0): `implement|review|
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
  - `gogo-mermaid` - portable diagrams (vendored very-nice-mermaid, offline viewer).
- **`templates/contracts/*.schema.json`** — the artifact contracts that cross
  phase boundaries: `issues-list`, `phase-result`, `pipeline`, `charts-manifest`.
- **`agents/*.md`** — specialist subagents the orchestrator delegates to:
  `gogo` (orchestrator, hands-off), `gogo-developer` ②, `gogo-reviewer` ③, `gogo-tester` ④.
- **`hooks/`** — `config-check.sh` (SessionStart reminder), `notify.sh`
  (Notification → ntfy/macOS/bell, **since 0.31.0 gate-filtered**: blocking prompts
  always ping, lifecycle noise never, `agent_completed`/unknown types only when a
  `.gogo/work` item NEWLY reaches a user gate — edge-latch state in
  `.gogo/resources/notify/gates`; knobs `GOGO_NOTIFY_LEVEL`/`GOGO_NOTIFY_DEBUG`;
  `--selftest` self-verifies). **`assets/vnm/`** - vendored very-nice-mermaid
  browser build + the viewer (template/js/css) + `layout.mjs`/`build-bundle.mjs`.
  **`.mcp.json`** - bundled Playwright MCP.

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
- **Portability** - core loop needs **no external deps**; the renderer is vendored
  (offline); Playwright/`very-nice-mermaid`/`jq` are optional and degrade gracefully.
- **Only ever write under `.gogo/`** — never edit a proxied upstream file.
- **Hard gate** — never implement an unaccepted plan.
- **Idempotent build** — re-runs preserve `## gogo overrides` and `Mode: owned`.

## gogo overrides
<!-- gogo-specific notes not in the linked source. Preserved across re-runs. -->
- The repo IS the plugin source; `${CLAUDE_PLUGIN_ROOT}` references resolve to it.
- Installed via marketplace `gogo` → GitHub `ZawadzkiB/gogo`; version in
  `.claude-plugin/plugin.json` must be bumped for installs to detect updates.
- **Release-history notes moved to an on-demand skill.** One line per shipped
  mechanism below; the full per-release notes (what/why/defects fixed) live in
  the skill — and future release notes are APPENDED THERE, with only a new title
  line added here.
  **Load when:** a change touches a specific mechanism/release below → `../skills/release-history/SKILL.md`
  - Knowledge vs on-demand skills (since 0.3.0)
  - Hosted docs + code-verified discovery (since 0.4.0)
  - Workspace + changelog + viewer (since 0.5.0)
  - Interactive diagrams + before/after compare (since 0.6.0)
  - View menu + plan bundles + `/gogo:done` work board (since 0.7.0)
  - Merged + synthesized changelog entries (since 0.8.0)
  - Board cockpit — action keys + filter + intent protocol v2 (since 0.9.0)
  - The `gogo` CLI + events telemetry (since 0.10.0)
  - Planning analyst + the UAT gate + CLI ops (since 0.11.0)
  - In-context implement + the CLI process-orchestrator (0.12.0 → 0.13.0)
  - Unattended ops + input signals + board accept (0.14.0)
  - Persistent-session CLI orchestrator (0.15.0)
  - Cockpit redesign — 1b + 1c board restyle (0.18.0)
  - Running-vs-status decoupling + UAT re-plan label (0.19.0)
  - Lean cockpit cards (0.20.0)
  - Per-session attach/kill pickers + changelog live-session dot (0.20.0)
  - Docs sync + install hardening (0.20.1)
  - Cockpit re-architecture: projects · sources · plans (0.21.0)
  - Cockpit fast-follows (0.22.0-0.25.1)
  - Plans-board kanban + re-sequence + auto-pickup (0.26.0)
  - Diagrams rendered by very-nice-mermaid (0.27.0)
  - Diagnosable cockpit launches + plan `v`/`w` + a legible cap (0.28.0)
  - The board stops narrating the past (0.29.0)
  - Session-binding ops — the tmux session NAME is cockpit-editable (since 0.32.0)
  - No auto-board + the S sessions panel + `/gogo:session-update` (since 0.33.0)
  - Go-fast mode — the token-lean ②→⑤ pipeline + per-source fastMode (since 0.34.0)
  - Border-only card selection — focus keeps card colours (0.35.0)

## Custom
<!-- Yours. gogo never rewrites this section: `/gogo:build` re-runs and the report-phase
     reconcile copy it 1:1 (byte-for-byte), exactly like `## gogo overrides`. Put any
     project notes gogo should read but never touch here — safe to edit or delete. -->
