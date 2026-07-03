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
- **JavaScript (vendored, not authored)** — `assets/mermaid/mermaid.min.js` (UMD
  build, works over `file://`). Do not edit; it's a dependency snapshot. The
  `/gogo:view` renderer modules (`assets/viewer/*.js`) ARE authored.
- **Python (vendored, authored) — since 0.7.0** — `assets/kanban/board.py`, the
  `/gogo:done` work-board curses TUI. **Pure stdlib** (no pip), pure ASCII, ships a
  `--selftest`; a soft dep (see below).
- **JSON** — `.claude-plugin/plugin.json` (manifest + version), `marketplace.json`,
  `.mcp.json` (Playwright MCP server).
- **Go (since 0.10.0)** — the **`gogo` CLI** in `cli/` (module
  `github.com/ZawadzkiB/gogo/cli`, **Go 1.25**): a deterministic cockpit that
  parses the `.gogo/` contract files (spec: `docs/cli-contract.md`) — no LLM in
  the read path. Pinned deps: the Charm stack (**bubbletea**, **bubbles**,
  **lipgloss**, **glamour**, **huh**) + **goldmark** (md→HTML) + **fsnotify**
  (live refresh). Viewer assets + `mermaid.min.js` are `go:embed`ded
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
(~50+ tests across contract/tui/launch/pages/diagram + a `gogo status` golden). UI/browser testing for *target* projects
uses the bundled **Playwright MCP** (boots via `npx`, needs Node). See
`testing-tools.md` / `test-strategy.md`.

## Optional tooling (graceful — never required)
- `mmdc` (mermaid CLI) — only used for SVG/PNG export if already present.
- `jq` — handy for validating/reading JSON artifacts when present.
- Node.js — only for the Playwright MCP.
- `python3` + `tmux` (since 0.7.0) — soft deps for the `/gogo:done` interactive
  work board (`board.py` curses TUI in a tmux pane; since 0.9.0 the pipeline
  **cockpit** — action keys + filter + intent relaunch loop). Detected at use
  (`command -v` + tty check); absent → the status-table + `AskUserQuestion`
  multi-select fallback. tmux is installed on this dev host (so the live-TUI test
  path in `test-strategy.md` applies), but it **stays a soft dep** — same
  detection, same fallback.
