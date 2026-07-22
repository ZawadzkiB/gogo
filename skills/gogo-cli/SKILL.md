---
name: gogo-cli
user-invocable: false
description: >-
  The gogo CLI companion - the canonical, on-demand reference for the optional
  `gogo` binary (a deterministic terminal cockpit + persistent-session launcher).
  Load this when the gogo CLI is relevant: the user asks how to launch / attach /
  resume / sweep sessions, wants to view or manage existing gogo work from the
  terminal, mentions the `gogo` command or the board/cockpit, or you are deciding
  whether to suggest the CLI vs the in-chat /gogo:* flow. Documents the full
  command surface, the persistent-session model, and when to reach for it - and is
  the single source the later active half (assistant DRIVES the CLI) will extend.
---

# gogo-cli - the CLI companion reference (loaded on demand)

The **`gogo` binary** is an optional, deterministic **terminal cockpit** for a
project's gogo pipeline (Go + Bubble Tea, source in `cli/`). It reads the same
`.gogo/` files the pipeline writes - **no LLM in the read path** - so it opens a
board in milliseconds, and it can **launch / attach / resume** the pipeline's
persistent Claude sessions. It is a *companion*, **not** a 14th slash command:
the `/gogo:*` slash commands stay the pipeline engine; the CLI is the fast
read/launch cockpit over the work they produce.

## Conditional - the binary is a SEPARATE install, never assume it

The `gogo` binary is **NOT bundled with the plugin** - it is a separate `curl`
download (prebuilt release asset) or a `cd cli && go build -o gogo .` from source.
So **every reference to it is conditional**: only suggest or use it **if `gogo` is
on the user's `PATH`** (a quick `command -v gogo` settles it). If it is absent,
the in-chat `/gogo:*` flow does everything without it - never tell the user the
CLI is required, and never block on it. When it would clearly help but is missing,
mention that it exists and point at the README install snippet, then proceed
in-chat.

## Command surface (v0.15.0+)

The runtime truth is `cli/main.go` (`printHelp` + the dispatch switch). Keep this
list in sync with `cli/main.go`, `README.md` `## The gogo CLI`, and
`docs/cli-contract.md` (see *Enumeration-sync* below).

| Command | What it does |
|---|---|
| `gogo` | open the interactive **board** - kanban of `plan · in progress · ready · changelog`, live-refreshed while the pipeline runs; `enter` drills into a card, action keys launch Claude. |
| `gogo go [<slug>]` | **launch-or-`--resume` ONE persistent `/gogo:go` session** for the feature (implement warm in-context + review/test as nested `Task` subagents + report). Enforces the same acceptance gate `/gogo:go` does; on the session's exit surfaces the outcome (`awaiting-uat` → run `/gogo:done`; a decision gate → the parked question; an error → halt). |
| `gogo plan …` | **two surfaces off one verb.** `gogo plan <slug>` (a bare feature SLUG) is the same persistent-session machinery for a `/gogo:plan` leg. `gogo plan <verb>` (a reserved store verb: `new "<title>"` · `list` · `show <id>` · `add <id> <source>[:<slug>]` · `rm <id> <source>[:<slug>]` · `ready <id>` · `promote <id> <source>` · `done <id>` · `delete <id>`) manages the **project-scoped plan store** at `~/.gogo/projects/<name>/.gogo/plans/<plan-id>.md` - a plan targets the project's sources, and `promote` **launches** `/gogo:plan <body> --correlation plan-<hash>` in a source (the skill writes `.gogo/work/` + stamps the correlation; the CLI never writes a source's `.gogo/`). Since **0.25.0** `gogo plan ready <id>` is the headless **accept**: a plan WITH targets auto-spawns a work item into each un-spawned target (its per-source brief as the goal, `--skip-acceptance` when the source opted out, records a member + flips the plan active); a **targetless** plan is today's plain mark-ready. `promote` stays the single-source manual spawn. Since **0.24.0** `gogo plan done <id>` is the **project-UAT accept**: it REFUSES unless every member work item is shipped (naming any that aren't), then records a `## Project UAT` round in the plan body + flips the plan to `done`. A slug that IS a reserved verb resolves to the store (launch such a feature via `gogo go` / the board). `gogo plan -h` shows both surfaces. |
| `gogo status` | print the work-index classifier table (every feature's phase / status / iterations / resume hint). |
| `gogo view <target>` | view a plan/report - glamour in the terminal, or `--web [--open]` builds the self-contained interactive HTML page. Targets: `<slug>` · `<slug>:plan` · `<slug>:report` · `<date>-<name>` changelog entry. |
| `gogo events <slug>` | print a feature's `events.jsonl` timeline (the same renderer the board's drill-in tail reuses). |
| `gogo sweep [--dry-run] [<slug>...]` | reap orphaned / shipped persistent sessions (the kill-at-ship backstop); with slug(s), targeted to just those cards (what `/gogo:done` runs at ship); `--dry-run` lists without killing. |
| `gogo trash [restore <entry>]` | list `.gogo/trash/` entries (deleted work, recoverable), or `restore <entry>` moves one back to `.gogo/work/`. |
| `gogo project [add <name\|repo> [--source <repo>] [--name <name>] \| list \| rm <name>]` | manage home-folder projects (`~/.gogo/projects/<name>/config.json`) - a project links many sources (repos with their own `.gogo/`). Since 0.24.0 `add` is dual-mode: a bare NAME (`gogo project add sanoma`) creates an EMPTY project (no repo, no source) scaffolded with a `.knowledge/` dir (a seeded cross-repo `project-knowledge.md`) + a `.gogo/plans/` dir; a PATH arg (or `--source <repo>`) creates the project with that repo as source #1 (verifies it contains `.gogo/`, defaults `concurrentWorkItems` to 1, detects its `mainBranch`) - byte-for-byte the pre-0.24 behaviour. Removing a project deletes only its home folder, never a source's `.gogo/`. `add` also auto-initializes the global cockpit home (the `~/.gogo/config.json` marker - FR22). Writes only `~/.gogo/` (CLI-owned data, never `.gogo/` pipeline state). |
| `gogo global [init \| board]` | the **two modes**: repo-local vs global. `gogo` INSIDE a repo (a dir with `.gogo/`) always shows THAT repo's own single board - even when the repo is a registered project's source. `gogo global` opens the cross-project **cockpit** (board · plans · config) from anywhere; `gogo` OUTSIDE any repo does the same. `gogo global init` sets up the global cockpit home `~/.gogo/` - creates `~/.gogo/projects/` and the `~/.gogo/config.json` marker (its existence = "initialized"); idempotent (re-run → "already initialized"). Bare `gogo global` (or `gogo global board`) opens the cockpit; an uninitialized home hints `gogo global init`, an empty one hints `gogo project add`. Setup: `gogo global init` → `gogo project add <repo>`. Writes ONLY `~/.gogo/`. |
| `gogo source [add <repo> [--project <name>] \| rm <repo\|name> [--project <name>]]` | add / remove a source to a project. `add` links a repo (or a monorepo service dir, each with its own `.gogo/`) as a source; `--project` defaults to the sole project and is required when several exist. Writes only the project entity under `~/.gogo/`, never a source's `.gogo/`. |
| `gogo draft [new "<title>" \| list \| show <id> \| ready <id> \| rm <id>]` | a thin alias into `gogo plan` (D9) - a draft is a plan in the `draft` status. Every subcommand forwards to the plan store; `draft list` narrows to `status: draft`, `draft new` seeds a `draft` plan, `draft ready` advances it, `draft rm` deletes. `--project` defaults to the sole project (required when several exist). There is no `edit` and no `promote --to`; use `gogo plan promote <id> <source>` to spawn. |
| `gogo epic [new "<title>" \| list \| show <id> \| add <id> <source>:<slug> \| rm <id> <source>:<slug> \| delete <id>]` | a thin alias into `gogo plan` (D9) - an epic is a plan that owns members. Every subcommand forwards to the plan store; `epic list` narrows to plans with ≥1 member (or ≥1 target), regardless of status (so a just-linked epic still shows). `add` / `rm` link/unlink an existing work item (`<source>` is one of the project's sources) - the many-to-many correlation. Correlation itself lives in each work item's `state.md` as a `correlation:` list (read straight by the board), NOT a CLI store. |
| `gogo run [<slug>]` | **DEPRECATED** alias for `gogo go` (prints a notice and forwards; will be removed). |
| `gogo --version` | print the version (mirrors the plugin's `.claude-plugin/plugin.json`). |

**`gogo go` / `gogo plan` flags:** `--attach` (launch an attachable interactive
tmux session so you can answer gates live) · `--takeover` (seize the owner lock
from a live session, reaping the prior) · `--force` (`gogo go` only - override the
per-project concurrency cap). Env `GOGO_CLAUDE_PERMISSION_MODE` sets the spawned
session's permission mode.

**Per-source gate-skip (since 0.24.0).** A source can opt OUT of the per-work-item
plan-acceptance and/or UAT gate via `planAcceptanceSkip` / `uatAcceptanceSkip` in its
`config.json` (both default false; edited in the config tab). When set, `gogo go`
appends `--skip-acceptance` / `--skip-uat` to the launched `/gogo:go` and prints a note;
the **gogo skills honor those params** (auto-record the plan acceptance / auto-pass UAT
as a **pre-declared consent**, exactly as a human accept would - one recorded acceptance,
the same single-owner events - never a silent bypass). Absent the flags, both gates are
today's hard stops byte-for-byte. Orthogonal to the FR3 **project-UAT** (`gogo plan
done`): `uatAcceptanceSkip` drops the per-work-item UAT; the project-UAT still gates the
whole cross-repo plan.

**Board keys:** `←→`/`h` columns · `↑↓`/`jk` cards · `space` select (ready) ·
`enter` drill-in · `v` quick-view · `w` web page · `m` move/launch · `d` ship ·
`a` attach session · `l` peek log · `x` delete→trash · `tab`/`shift+tab` cycle the
**board · plans · config** tabs · `p` cycles the board's **project chips** (`all` +
one per project) / the config-tab **project switcher** (they share one focus) · `/`
filter (an `@name` token narrows to a project **or** source; a `#plan-<id>` token to
that plan's members across sources) · `G` glow · `q` quit. A `⏸` marks a card waiting
on you (plan-acceptance / decision / UAT). Since **0.23.0** `gogo global` opens ONE
**unified board across every project** - each card + changelog row tagged
`●project ●source`. On a **lone repo** with no home project there are no tabs / chips -
just the single-repo board (byte-for-byte).

**Plans tab keys:** `↑↓` plans · `enter` open the detail · `n` new plan · `A`
**plan-with-claude** (since 0.25.0 an analyst-grade session: mints a draft, then opens a
`claude` session anchored at a source that **loads the `gogo-project-plan` skill**, reads
+ analyzes the project's real source repos read-only, **auto-selects** the sources the
plan needs, and writes the plan file in place with front-matter `targets:` + a
`## Source briefs` section per target - not a `/gogo:plan` scaffold) · `r` **accept**
(since 0.25.0 - a plan WITH targets confirms then **auto-spawns** a work item into each
un-spawned target: one `/gogo:plan <brief> --correlation plan-<hash>` per source honoring
that source's `--skip-acceptance`, recording a member + flipping the plan active; a
**targetless** plan is today's plain mark-ready with zero launches; idempotent - an
already-spawned target is skipped) · `D` **accept project-UAT** (since 0.24.0 - the same
gate as `gogo plan done`: refuses unless every member is shipped, else a confirm → flips
the plan to `done`) · `x` delete. A plan whose members are all shipped renders the derived
**`awaiting-project-uat`** status (distinct from `active`). In a plan's detail: `↑↓`
target sources · `c` **create work item** (launches `/gogo:plan <body> --correlation
plan-<hash>` in that source) · `+` add a target source · `D` accept project-UAT · `e`
edit the plan file · `esc` back.

**Config tab + per-source concurrency cap (since 0.21.0):** the config tab manages
the focused project's **sources** (`a` add · `x` remove · `e` edit a source's
`concurrentWorkItems` cap / `mainBranch` / color / **plan-accept skip** /
**uat skip**), offers a **project switcher** (`p`), and a **knowledge explorer** that
(since 0.24.0) splits **project knowledge** (`~/.gogo/projects/<name>/.knowledge/` - the
cross-repo domain) from **source knowledge** (the focused source's `.gogo/knowledge/`)
into two labelled groups. The source detail shows the two gate-skip flags
(`plan-accept skip` / `uat skip`, both `no` by default). All writes land under `~/.gogo/`
only - never a source's `.gogo/`. A capped source refuses a `gogo go` (or board
`m`→go) that would start an `(N+1)`th live in-progress feature in it (so two build
sessions can't clobber one working tree); `--force` overrides. `0` = unlimited; an
uncapped source behaves as before.

**Card vocabulary:** a card shows a **status pill** (the card's true `state.md`
state - **`running` is never a status**; a live session is the separate green `●`
next to the name plus the header's `● N session` count) and, only while a session
is actively working it, a green **`● <agent>` chip** (analyst / developer /
reviewer / tester / reporter, from the phase). The heavy **left-border stripe**
(red gate / purple UAT) is the per-card "act now" cue. The collapsed **changelog**
carries a `●` on any shipped item still holding a session. In the drill, `a` (and
`K`) open a **picker** to choose *which* session when the card has several - `K`
also offers "all N".

## The persistent-session model (what `gogo go`/`gogo plan` actually do)

`gogo go` and `gogo plan` are **session-lifecycle managers over the one skill** -
they run **no phase loop and no routing in Go** (the single routing rule lives in
the `/gogo:go` skill). Concretely:

- **One warm session per feature.** `gogo go <slug>` **launches or `--resume`s a
  single persistent `claude -p` session** running the existing `/gogo:go` skill
  for the whole feature. A re-launch resumes the SAME warm session (by its
  recorded uuid) rather than starting cold - implement stays warm in-context
  across the review/test fix loop.
- **One-owner lock.** Before launching, `gogo go`/`gogo plan` acquire an exclusive
  owner lock for the slug (`.gogo/resources/cli/locks/<slug>.lock`), cross-checked
  against **both** the owner PID *and* a matching live `gogo-*` tmux session
  (exact convention parse, never substring). A live owner is **refused** by
  default (`--takeover` seizes it); a stale lock (both signals dead) is silently
  reclaimed.
- **Session registry + `gogo sweep`.** Sessions are tracked in
  `.gogo/resources/cli/sessions/<slug>.json` (per-leg `go`/`plan` uuid, tmux name,
  PID, lifecycle status `running|parked|awaiting-uat|shipped|reaped`, cost/turns).
  `gogo sweep` reaps sessions whose owning feature is terminal, plus orphans - the
  **kill-at-ship** backstop so panes don't leak.
- **The CLI never mutates pipeline state.** These are **delegated launches**: the
  launched session performs every `state.md`/contract write. All CLI bookkeeping
  lives under `.gogo/resources/cli/` (CLI-owned), never in `.gogo/work/feature-*/`.

Soft deps (detected at use, graceful fallback): `claude` (needed only to launch),
`tmux` (else a backgrounded `claude -p` + log), `glow` (the built-in glamour view
is the fallback).

## When to reach for the CLI vs the in-chat flow

- **In-chat `/gogo:*` is the default, dependency-free path** - planning,
  implementing, reviewing, testing, reporting, and pausing live at every gate all
  happen in the chat with no binary at all. When in doubt, or when the CLI is not
  on `PATH`, use the in-chat flow.
- **Suggest the CLI when it is on `PATH` and the task is management/launch-shaped:**
  - *Fast deterministic overview / triage* - "what's in flight?", "show the board",
    "which features are waiting on me?" → `gogo` / `gogo status`.
  - *Reading existing work* - view a plan or report, browse a feature's files, or
    open the interactive HTML page → `gogo view <target> [--web --open]`,
    `gogo events <slug>`.
  - *Launching / attaching / resuming persistent sessions* for long unattended
    runs, or attaching to answer a gate live → `gogo go [<slug>] [--attach]`,
    `gogo plan <slug>`, `--takeover` to seize the lock.
  - *Housekeeping* - reap leaked sessions (`gogo sweep`), recover deleted work
    (`gogo trash [restore <entry>]`).

The two paths **coexist**: the in-chat flow is unchanged and remains the default;
the CLI is the optional accelerator for managing and viewing the work it produces.

## Enumeration-sync (why this file is one of four)

The CLI command surface is enumerated in **four** places that must never drift:
`cli/main.go` help (the runtime truth), `README.md` `## The gogo CLI`,
`docs/cli-contract.md`, and **this reference**. Any change to the surface (a new,
renamed, or removed command/flag) updates **all four**. The `cli` test
`TestCLICommandEnumerationInSync` greps these sources against `main.go`'s dispatch
so a missing command can't drift silently - keep it green.

## For the later active half

This is the **passive** reference (it documents the CLI so an installed Claude
*knows* the surface and *when* to suggest it). The deferred **active** half - where
the assistant *drives* the CLI on the user's behalf (runs `gogo go` / `gogo done`
for them) - **extends this same file**, never a second copy. Keep this the single
canonical source.
