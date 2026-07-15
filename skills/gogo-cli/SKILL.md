---
name: gogo-cli
user-invocable: false
description: >-
  The gogo CLI companion — the canonical, on-demand reference for the optional
  `gogo` binary (a deterministic terminal cockpit + persistent-session launcher).
  Load this when the gogo CLI is relevant: the user asks how to launch / attach /
  resume / sweep sessions, wants to view or manage existing gogo work from the
  terminal, mentions the `gogo` command or the board/cockpit, or you are deciding
  whether to suggest the CLI vs the in-chat /gogo:* flow. Documents the full
  command surface, the persistent-session model, and when to reach for it — and is
  the single source the later active half (assistant DRIVES the CLI) will extend.
---

# gogo-cli — the CLI companion reference (loaded on demand)

The **`gogo` binary** is an optional, deterministic **terminal cockpit** for a
project's gogo pipeline (Go + Bubble Tea, source in `cli/`). It reads the same
`.gogo/` files the pipeline writes — **no LLM in the read path** — so it opens a
board in milliseconds, and it can **launch / attach / resume** the pipeline's
persistent Claude sessions. It is a *companion*, **not** a 14th slash command:
the `/gogo:*` slash commands stay the pipeline engine; the CLI is the fast
read/launch cockpit over the work they produce.

## Conditional — the binary is a SEPARATE install, never assume it

The `gogo` binary is **NOT bundled with the plugin** — it is a separate `curl`
download (prebuilt release asset) or a `cd cli && go build -o gogo .` from source.
So **every reference to it is conditional**: only suggest or use it **if `gogo` is
on the user's `PATH`** (a quick `command -v gogo` settles it). If it is absent,
the in-chat `/gogo:*` flow does everything without it — never tell the user the
CLI is required, and never block on it. When it would clearly help but is missing,
mention that it exists and point at the README install snippet, then proceed
in-chat.

## Command surface (v0.15.0+)

The runtime truth is `cli/main.go` (`printHelp` + the dispatch switch). Keep this
list in sync with `cli/main.go`, `README.md` `## The gogo CLI`, and
`docs/cli-contract.md` (see *Enumeration-sync* below).

| Command | What it does |
|---|---|
| `gogo` | open the interactive **board** — kanban of `plan · in progress · ready · changelog`, live-refreshed while the pipeline runs; `enter` drills into a card, action keys launch Claude. |
| `gogo go [<slug>]` | **launch-or-`--resume` ONE persistent `/gogo:go` session** for the feature (implement warm in-context + review/test as nested `Task` subagents + report). Enforces the same acceptance gate `/gogo:go` does; on the session's exit surfaces the outcome (`awaiting-uat` → run `/gogo:done`; a decision gate → the parked question; an error → halt). |
| `gogo plan <slug>` | the same persistent-session machinery for a `/gogo:plan` leg. |
| `gogo status` | print the work-index classifier table (every feature's phase / status / iterations / resume hint). |
| `gogo view <target>` | view a plan/report — glamour in the terminal, or `--web [--open]` builds the self-contained interactive HTML page. Targets: `<slug>` · `<slug>:plan` · `<slug>:report` · `<date>-<name>` changelog entry. |
| `gogo events <slug>` | print a feature's `events.jsonl` timeline (the same renderer the board's drill-in tail reuses). |
| `gogo sweep [--dry-run] [<slug>...]` | reap orphaned / shipped persistent sessions (the kill-at-ship backstop); with slug(s), targeted to just those cards (what `/gogo:done` runs at ship); `--dry-run` lists without killing. |
| `gogo trash [restore <entry>]` | list `.gogo/trash/` entries (deleted work, recoverable), or `restore <entry>` moves one back to `.gogo/work/`. |
| `gogo run [<slug>]` | **DEPRECATED** alias for `gogo go` (prints a notice and forwards; will be removed). |
| `gogo --version` | print the version (mirrors the plugin's `.claude-plugin/plugin.json`). |

**`gogo go` / `gogo plan` flags:** `--attach` (launch an attachable interactive
tmux session so you can answer gates live) · `--takeover` (seize the owner lock
from a live session, reaping the prior). Env `GOGO_CLAUDE_PERMISSION_MODE` sets
the spawned session's permission mode.

**Board keys:** `←→`/`h` columns · `↑↓`/`jk` cards · `space` select (ready) ·
`enter` drill-in · `v` quick-view · `w` web page · `m` move/launch · `d` ship ·
`a` attach session · `l` peek log · `x` delete→trash · `/` filter · `G` glow ·
`q` quit. A `⏸` marks a card waiting on you (plan-acceptance / decision / UAT).

**Card vocabulary:** a card shows a **status pill** (the card's true `state.md`
state — **`running` is never a status**; a live session is the separate green `●`
next to the name plus the header's `● N session` count) and, only while a session
is actively working it, a green **`● <agent>` chip** (analyst / developer /
reviewer / tester / reporter, from the phase). The heavy **left-border stripe**
(red gate / purple UAT) is the per-card "act now" cue. The collapsed **changelog**
carries a `●` on any shipped item still holding a session. In the drill, `a` (and
`K`) open a **picker** to choose *which* session when the card has several — `K`
also offers "all N".

## The persistent-session model (what `gogo go`/`gogo plan` actually do)

`gogo go` and `gogo plan` are **session-lifecycle managers over the one skill** —
they run **no phase loop and no routing in Go** (the single routing rule lives in
the `/gogo:go` skill). Concretely:

- **One warm session per feature.** `gogo go <slug>` **launches or `--resume`s a
  single persistent `claude -p` session** running the existing `/gogo:go` skill
  for the whole feature. A re-launch resumes the SAME warm session (by its
  recorded uuid) rather than starting cold — implement stays warm in-context
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
  `gogo sweep` reaps sessions whose owning feature is terminal, plus orphans — the
  **kill-at-ship** backstop so panes don't leak.
- **The CLI never mutates pipeline state.** These are **delegated launches**: the
  launched session performs every `state.md`/contract write. All CLI bookkeeping
  lives under `.gogo/resources/cli/` (CLI-owned), never in `.gogo/work/feature-*/`.

Soft deps (detected at use, graceful fallback): `claude` (needed only to launch),
`tmux` (else a backgrounded `claude -p` + log), `glow` (the built-in glamour view
is the fallback).

## When to reach for the CLI vs the in-chat flow

- **In-chat `/gogo:*` is the default, dependency-free path** — planning,
  implementing, reviewing, testing, reporting, and pausing live at every gate all
  happen in the chat with no binary at all. When in doubt, or when the CLI is not
  on `PATH`, use the in-chat flow.
- **Suggest the CLI when it is on `PATH` and the task is management/launch-shaped:**
  - *Fast deterministic overview / triage* — "what's in flight?", "show the board",
    "which features are waiting on me?" → `gogo` / `gogo status`.
  - *Reading existing work* — view a plan or report, browse a feature's files, or
    open the interactive HTML page → `gogo view <target> [--web --open]`,
    `gogo events <slug>`.
  - *Launching / attaching / resuming persistent sessions* for long unattended
    runs, or attaching to answer a gate live → `gogo go [<slug>] [--attach]`,
    `gogo plan <slug>`, `--takeover` to seize the lock.
  - *Housekeeping* — reap leaked sessions (`gogo sweep`), recover deleted work
    (`gogo trash [restore <entry>]`).

The two paths **coexist**: the in-chat flow is unchanged and remains the default;
the CLI is the optional accelerator for managing and viewing the work it produces.

## Enumeration-sync (why this file is one of four)

The CLI command surface is enumerated in **four** places that must never drift:
`cli/main.go` help (the runtime truth), `README.md` `## The gogo CLI`,
`docs/cli-contract.md`, and **this reference**. Any change to the surface (a new,
renamed, or removed command/flag) updates **all four**. The `cli` test
`TestCLICommandEnumerationInSync` greps these sources against `main.go`'s dispatch
so a missing command can't drift silently — keep it green.

## For the later active half

This is the **passive** reference (it documents the CLI so an installed Claude
*knows* the surface and *when* to suggest it). The deferred **active** half — where
the assistant *drives* the CLI on the user's behalf (runs `gogo go` / `gogo done`
for them) — **extends this same file**, never a second copy. Keep this the single
canonical source.
