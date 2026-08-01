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
  with **no external dependencies**. Anything optional (`very-nice-mermaid`, Playwright, `jq`,
  ntfy) must degrade gracefully and never hard-fail.
- **`${CLAUDE_PLUGIN_ROOT}`** for all in-plugin asset/template paths — never
  hard-code absolute paths.

## Vendored executable assets (since 0.7.0; no live example since 0.33.0 retired board.py)
- An **authored** vendored executable (e.g. the retired `assets/kanban/board.py`, distinct from
  the third-party `vnm-browser.js` snapshot) must be **pure standard library** (no
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
  - **The one sanctioned exception (0.29.0):** a presence check may only ever
    **REFUSE**, never **PROMOTE**, and only on a **monotonic** artifact. The
    original rule reacted to presence **over-claiming** (a stale `report/` present
    while the state had moved on). `plan.md` is the mirror image: it is *Guaranteed
    (from plan ①)* by `docs/cli-contract.md`, phase ⑤ updates it to as-built, and
    nothing in the tree deletes it - so its **absence** can only mean "never
    written", never "stale". `contract.planWritten` therefore drives only rules
    that **narrow** (the `✎ authoring` pill dims; the four accept paths refuse) and
    **never** a class, a column, or an unlocked action. Keep the flag
    **defect-positive** (`PlanUnwritten`, zero value = "written") so a synthetic
    struct keeps its old meaning, and treat a read error as *written* so a
    permissions hiccup can never invent a defect.
- **A phase writes its occupancy status at entry AND again at exit (0.29.0).**
  `state.md` is what every reader believes - the board's column, `gogo status`, the
  concurrency cap, `pages`, any headless consumer. Written only at a phase's exit it
  records that the phase just *finished*, so for the whole run the disk describes the
  previous phase and the board narrates the past. So write `phase` + `status` as the
  **first act after validate-in passes** - and write them **again at the exit**, with
  the `iterations` bump (a completion count) and `phase-done`.
  **Keep both writers; the redundancy IS the design.** 0.29.0 first moved the write to
  entry and *removed* the exit write as newly-redundant. That made it worse: the entry
  write is prose an LLM follows, and with only that half `state.md` stopped advancing
  at all - arbitrarily stale instead of reliably one phase behind. Two writers, one at
  each end: floor = one phase behind, ceiling = live.
  **But do not let a safety property depend on it.** This is LLM prose, and it was
  skipped on **all three** of its live runs in the very release that added it - so
  pair every occupancy-derived rule with a deterministic reader-side guard (the cap
  keys on a live `gogo-go-<slug>` session, not on a file-derived class) and make the
  writer's failure **visible** rather than silent (`tui.phaseLineLags` → the
  `· state lags` cue). A detector is a detector, not a guarantee: its silence is not
  proof of health.
- **A user-visible rule stated in more than one place is ONE constant (TEST-006,
  0.29.0):** 0.28.0 deliberately wrote the concurrency cap's rule into four
  surfaces by hand; 0.29.0 changed the rule and found **three of the four stale** -
  including the source-edit form the user reads *while setting the cap*, which
  stated the opposite of the code. Extract the sentence
  (`orchestrator.CapRuleClause`) and have every surface quote it, so the drift is
  **impossible** rather than merely forbidden. Then **pin the wirings, not just the
  producer** - a call site can stop calling the producer and hand-write fresh copy
  with the whole suite green (see `test-strategy.md`, variant 8).
- **A remedy a message recommends is part of the product's safety surface
  (TEST-007, 0.29.0):** a cap refusal began recommending a **bare `gogo sweep`**,
  which is HOST-GLOBAL - it judges every `gogo-*` session on the machine against
  *one repo's* feature list, so another source's in-flight build is classified
  "orphan" and killed with no confirmation. Never emit a host-global destructive
  command from an inline remedy: produce the **targeted** form from a single
  producer (`orchestrator.CapSweepRemedy` → `gogo sweep <slug>...`), return `""`
  rather than degrading to the bare form, and guard both the presence of the
  targeted string **and the absence of the bare one**.
- **Every huh form goes through `newForm()` — never a direct `huh.NewForm(`
  (0.30.0):** `newForm` (cli/internal/tui/model.go) is the ONE construction site
  applying `gogoKeyMap()`, whose Text group rebinds `enter` to INSERT A NEWLINE
  (huh's default Text keymap has `enter` in both Next and Submit, so a typed
  newline is impossible and a Text-bearing form submits mid-thought — measured).
  A direct `huh.NewForm(` silently regresses the next Text field; the source-scan
  guard `TestNewFormIsTheOnlyFormConstructionSite` fails the suite if one
  reappears (and fails loudly on an empty scan — a guard must never pass
  vacuously).
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
