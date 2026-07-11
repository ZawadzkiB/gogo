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

## Go code in `cli/` (since 0.10.0)
- **Gates before hand-off:** `gofmt -l .` clean · `go vet ./...` clean ·
  `go test -race ./...` green. Non-negotiable for any `cli/` change.
- **The CLI stays a deterministic reader.** It parses the frozen contract
  (`docs/cli-contract.md`) leniently (skip bad lines, degrade on garbage — never
  crash) and **never mutates pipeline state** — every state-changing action
  launches Claude (`/gogo:go`, `/gogo:done`).
- **Injection safety:** spawned commands are a **single argv element, no shell**
  (tmux/exec direct); slugs must never reach a shell.
- **Injectable seams for launch-class side effects** (e.g. `Model.launcher`
  defaulting to `launch.Launch`) so tests can assert fire-exactly-once without
  spawning anything.
- **Bubbletea gotcha (recorded, TEST-001):** the Model is a **value type copied
  on every Update** — never bind library pointers (`huh` `.Value(&m.field)`)
  into it; put mutable form/dialog targets behind a **heap-stable pointer**
  (e.g. `*formBinding`) shared across copies. And forward **every** `tea.Msg`
  (not just `KeyMsg`) to an active child component — async protocols like huh's
  `nextFieldMsg` die silently otherwise.
- **State rules gate on `status`, never on artifact presence (TEST-004, 0.11.0):**
  artifacts outlive the state that produced them — a stale `report/` survives a
  UAT rerun, so a classifier/validate rule keyed on file existence lies
  mid-pipeline. Key such rules on the `state.md` status (ready-to-ship = report
  AND `awaiting-uat`/legacy `done`), and treat any relaxation as a contract change.
- **Attribute sessions by exact convention parse, never substring (TEST-005,
  0.11.0):** matching a slug into session names with `strings.Contains`
  cross-attributes overlapping slugs (`auth`/`oauth`, `waiting-card` inside
  `awaiting-card`). Parse the `gogo-<action>-<sanitized-slug>` convention (plus
  the numeric collision suffix) where it is OWNED — the launch package — and
  compare the slug component exactly (`launch.SessionMatchesSlug`).

## Style
- Plain ASCII where practical; the phase glyphs `①②③④⑤` are an intentional exception.
- Bash hooks: `set -euo pipefail`, best-effort (`|| true`), silent no-op when a
  tool is absent.
- Keep diffs minimal and scoped to the plan; match the surrounding file's tone.

## Classifier-safe skill bash (since 0.14.0)
- **Never author a skill-bash delete that trips Claude Code's "dangerous rm"
  permission classifier** — gogo's own mechanical file steps (e.g. `/gogo:done`
  changelog assembly / board cleanup) must run **prompt-free**. Forbidden shapes:
  a **glob-`rm`** (`rm …/*`), **`rm -rf "$var…"`**, and **`rm -f "$var"`** on a bare
  variable. Use a **guarded, scoped `find <dir> … -delete`** instead: prove the
  variable is non-empty AND resolves under `.gogo/` (refuse + exit otherwise), then
  delete via `find` (no glob, no bare-variable `rm`). Same idempotent effect, no
  prompt, never escapes the guarded target. The `cli/` test
  **`TestSkillsBashNoUnsafeRm`** greps every `skills/*/SKILL.md` and fails if any
  forbidden shape reappears — it is the durable regression guard, so keep it green.

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

## Custom
<!-- Yours. gogo never rewrites this section: `/gogo:build` re-runs and the report-phase
     reconcile copy it 1:1 (byte-for-byte), exactly like `## gogo overrides`. Put any
     project notes gogo should read but never touch here — safe to edit or delete. -->
