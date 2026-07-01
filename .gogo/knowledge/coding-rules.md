# Coding rules

**Purpose:** conventions the implementation must follow when changing gogo.

<!-- gogo:meta
Mode: owned
Source: [ ../../README.md ]
Confidence: high
Generated-by: /gogo:build
-->

## Authoring conventions (this is a markdown plugin)
- **Commands stay ultra-thin.** `commands/*.md` only invoke a skill + pass args.
  No flow logic in commands — it lives in the skill (the "operating manual").
- **Skills are the source of logic.** Each `skills/<name>/SKILL.md` has YAML
  frontmatter (`name`, `description`) and prose steps. Keep steps numbered,
  imperative, and concise; prefer tables for enumerations.
- **One concern per knowledge file.** Don't bloat; cross-link with `[[name]]`-style
  references where useful.
- **Keep enumerations in sync.** A change to the phase list, the feature-folder
  file set, or discovery patterns must be reflected in **every** place that
  enumerates it: `skills/gogo/SKILL.md`, the relevant phase skill, the templates
  (`templates/state.template.md` file-list comment), and `README.md`. Grep before
  you finish.
- **Bump the version.** Any behavioural change → bump `.claude-plugin/plugin.json`
  `version` (installs only detect new versions).

## Hard invariants (never violate)
- **Only ever write under `.gogo/`** (one user-gated exception — approved
  `standalone` skills; see `## gogo overrides`). Never edit a proxied upstream
  file (the project's CLAUDE.md / README / configs). If a change belongs upstream,
  surface a suggestion to the user instead.
- **Never implement an unaccepted plan.** Acceptance is the gate before code.
- **Portability contract.** The core plan→implement→review→test loop must work
  with **no external dependencies**. Anything optional (`mmdc`, Playwright, `jq`,
  ntfy) must degrade gracefully and never hard-fail.
- **`${CLAUDE_PLUGIN_ROOT}`** for all in-plugin asset/template paths — never
  hard-code absolute paths.

## Vendored executable assets (since 0.7.0)
- An **authored** vendored executable (e.g. `assets/kanban/board.py`, distinct from
  the third-party `mermaid.min.js` snapshot) must be **pure standard library** (no
  pip/network), **pure ASCII**, ship a **`--selftest`**, and expose a **documented
  exit-code contract** the calling skill branches on. It stays a **soft dep**
  (detected at use; graceful fallback) and **never commits compiled bytecode**
  (`__pycache__/`, `*.pyc` are gitignored).

## Style
- Plain ASCII where practical; the phase glyphs `①②③④⑤` are an intentional exception.
- Bash hooks: `set -euo pipefail`, best-effort (`|| true`), silent no-op when a
  tool is absent.
- Keep diffs minimal and scoped to the plan; match the surrounding file's tone.

## gogo overrides
<!-- Preserved across re-runs. -->

### Knowledge file line budget
- Keep each `.gogo/knowledge/*.md` body **lean**: OK `<200` lines · WARN
  `200-400` · OVER `>400` (defaults; `/gogo:skills --warn N --max N` overrides).
  Big always-read context makes the LLM pipeline workers wander and lose
  determinism — measure the **gogo-owned body** only (for a proxy, never the
  linked upstream).
- When a file goes over budget, extract cohesive, situational sections into
  **on-demand skills** with `/gogo:skills` (the parent keeps a `**Load when:**`
  pointer). `/gogo:build` prints a nudge once a file passes the warn line.
- **Write rule + its one exception.** Default writes stay under `.gogo/`. The
  **only** sanctioned write outside `.gogo/` is an extracted **standalone** skill's
  `.claude/skills/<slug>/` dir — and only when the user approves that candidate as
  standalone (never automatic). Everything else still honors `.gogo/`-only.
