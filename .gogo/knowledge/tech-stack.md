# Tech stack

**Purpose:** languages, frameworks, and the build / run / test commands.

<!-- gogo:meta
Mode: owned
Source: [ ]
Confidence: high
Generated-by: /gogo:build
-->

## Languages / formats
- **Markdown** — skills (`skills/*/SKILL.md`), commands (`commands/*.md`), agents
  (`agents/*.md`), templates (`templates/**`). This is where ~all the logic lives.
- **Bash** — hooks (`hooks/*.sh`): `config-check.sh`, `notify.sh`. POSIX-ish,
  `set -euo pipefail`, best-effort (never hard-fail the session).
- **JavaScript (vendored, not authored)** - `assets/vnm/vnm-browser.js`, the
  [very-nice-mermaid](https://www.npmjs.com/package/very-nice-mermaid) browser
  build (classic script, works over `file://`). **Generated - do not edit**;
  regenerate with `node assets/vnm/build-bundle.mjs`. The gogo orchestrator
  (`assets/vnm/viewer.js`) and the layout tool (`assets/vnm/layout.mjs`) ARE
  authored.
- **Python (vendored, authored) — RETIRED in 0.33.0** — `assets/kanban/board.py`
  (the 0.7.0 `/gogo:done` work-board curses TUI) was deleted with the no-auto-board
  rule: bare `/gogo:done` ships in chat, and the interactive cockpit is the `gogo`
  binary. No vendored python ships today.
- **JSON** — `.claude-plugin/plugin.json` (manifest + version), `marketplace.json`,
  `.mcp.json` (Playwright MCP server).
- **Go (since 0.10.0)** — the **`gogo` CLI** in `cli/` (module
  `github.com/ZawadzkiB/gogo/cli`, **Go 1.25**): a deterministic cockpit that
  parses the `.gogo/` contract files (spec: `docs/cli-contract.md`) — no LLM in
  the read path. Pinned deps: the Charm stack (**bubbletea**, **bubbles**,
  **lipgloss**, **glamour**, **huh**) + **goldmark** (md→HTML) + **fsnotify**
  (live refresh). Viewer assets + `vnm-browser.js` are `go:embed`ded
  (`cli/internal/pages/assets/`, synced from `assets/` via `make sync-assets`).

## "Build"
The **markdown plugin** has no compile/build step — it is consumed as files; the
release action is bumping `version` in `.claude-plugin/plugin.json` so installs
can detect the update. The **CLI** (since 0.10.0) does build:
`cd cli && go build -o gogo .` (or `make build`); the binary is gitignored.
Note: `go install ./cli` names the binary after the module tail (`cli`, not
`gogo`) — use the explicit `-o gogo` build. `gogo --version` mirrors the plugin
version.

## Run / install
- Marketplace: `gogo` → GitHub `ZawadzkiB/gogo`.
- Update loop (installs read a *local* marketplace cache, so update first):
  `/plugin marketplace update gogo` → `/plugin install gogo@gogo` → `/reload-plugins`.
- Local dev alternative: `/plugin marketplace add /path/to/gogo` (then `git pull`
  + `/reload-plugins`; no marketplace-update needed).

## Test
The markdown-plugin side has no unit suite — verification = **dogfood**:
install, then run `/gogo:build`, `/gogo:plan`, `/gogo:go` on a sample repo and
inspect the produced `.gogo/` artifacts. The **CLI** (since 0.10.0) has a real
Go suite: `cd cli && gofmt -l . && go vet ./... && go test -race ./...`
(**566** test functions as of 0.33.0 - verified by grep, was 559 as of 0.32.0 - across
13 packages: contract/tui/launch/pages/plans/projects/orchestrator/diagram/**trash**/config
+ a `gogo status` golden; 0.31.0 added the two `notify_hook_test.go` guards, which also
run `hooks/notify.sh --selftest` in CI; 0.32.0 added the session-binding suites incl.
`TestBoardKeyHelpInSync`, the anti-vacuity-floored board/drill key-help guard; 0.33.0
added the sessions-panel suite + `TestNoInteractiveBoardInSkills`, the no-auto-board
source guard). UI/browser testing for *target* projects
uses the bundled **Playwright MCP** (boots via `npx`, needs Node). See
`testing-tools.md` / `test-strategy.md`.

## Optional tooling (graceful — never required)
- `very-nice-mermaid` (`vnm`, Node >= 20) - prebuilds `layouts.json` so the
  viewer can render sequence/class/state interactively, and exports SVG/PNG.
  Absent → flowcharts still render (parsed in-browser); other kinds show an
  inline error. Install with `npm i -g very-nice-mermaid`. **`mmdc` is no longer
  used anywhere in gogo.**
- `jq` — handy for validating/reading JSON artifacts when present.
- Node.js — only for the Playwright MCP.
- `tmux` — a soft dep of the **launch/session machinery** (attachable sessions,
  the S sessions panel, `/gogo:session-update`); absent → the backgrounded
  `claude -p` fallback and in-chat flows. `python3` is **no longer needed by
  gogo** (0.33.0 retired the `/gogo:done` board.py; bare `/gogo:done` ships in
  chat with no environment deps). tmux is installed on this dev host, so the
  live-TUI test path in `test-strategy.md` applies to the Go board.

## tmux platform constraints (measured on tmux 3.7b, 0.28.0)

Two hard facts about tmux that anything building a launch argv must respect. Both were
measured on this host, not inferred, and both caused real bugs:

- **A tmux command line over ~16 KB is refused** with `command too long` on stderr and
  exit status 1. Bisected: last accepted **16317** bytes, first refused 16318, and it is
  the WHOLE command line (session name included) that is bounded. `launch` pins
  `MaxTmuxCommandBytes = 16317` and preflights every `new-session`. Never inline a
  multi-KB body into a launched command - point at the file that holds it.
- **A plain `-t` target resolves exact -> prefix -> fnmatch**, so it can hit the wrong
  session: `kill-session -t gogo-plan-foo` provably killed `gogo-plan-foobar-long`.
  Always use the exact form, in the shape the target position accepts: **`=<name>`** for a
  **session** target (`has-session`, `kill-session`, `attach-session`, `switch-client`)
  and **`=<name>:`** for `capture-pane`, whose `-t` is a **pane** target and rejects the
  bare `=<name>` outright (`can't find pane`). `new-session -s` takes a NAME, not a
  target - the `=` would become part of the session name.

Also verified: tmux does **not** run the launched command through a shell (argv arrives
verbatim, so `$(...)`, backticks, `;`, `*` and newlines are safe as separate argv
elements), and neither `-c <bad dir>` nor a missing binary nor `.`/`:` in a name fails on
3.7b.

## Custom
<!-- Yours. gogo never rewrites this section: `/gogo:build` re-runs and the report-phase
     reconcile copy it 1:1 (byte-for-byte), exactly like `## gogo overrides`. Put any
     project notes gogo should read but never touch here — safe to edit or delete. -->
