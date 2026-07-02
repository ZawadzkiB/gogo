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
- **Python (vendored, authored) — since 0.7.0** — `assets/xplan-board/server.py`, the
  `/gogo:xplan` browser-board server (serves the committed React `dist/` + `GET /api/board`
  + `POST /api/ship`, localhost only). **Pure stdlib** (no pip), pure ASCII, ships a
  `--selftest`, documented exit codes (0 pass / 2 bad args or selftest fail); a soft dep
  (see below). (Replaced the 0.7.0–0.9.0 `board.py` curses TUI, removed in 0.10.0.)
- **TypeScript / React (authored, since 0.10.0)** — `assets/xplan-board/src/*`, the
  `/gogo:xplan` kanban ported from xplan, built with **Vite**. The built `dist/` is
  **committed** so runtime needs no toolchain; npm/node is **dev-time only** (D4=A).
- **JSON** — `.claude-plugin/plugin.json` (manifest + version), `marketplace.json`,
  `.mcp.json` (Playwright MCP server).

## "Build"
The plugin is consumed as files — **no build step for the markdown / skills / hooks**.
The **one** built asset is the `/gogo:xplan` React board: `cd assets/xplan-board &&
npm install && npm run build` (Vite) regenerates the **committed** `dist/`. That is a
**dev-time** step only — plugin users serve the committed `dist/` with `python3`, no npm
required (D4=A). The release action is bumping `version` in
`.claude-plugin/plugin.json` so installs can detect the update.

## Run / install
- Marketplace: `gogo` → GitHub `ZawadzkiB/gogo`.
- Update loop (installs read a *local* marketplace cache, so update first):
  `/plugin marketplace update gogo` → `/plugin install gogo@gogo` → `/reload-plugins`.
- Local dev alternative: `/plugin marketplace add /path/to/gogo` (then `git pull`
  + `/reload-plugins`; no marketplace-update needed).

## Test
No unit/integration suite (it's a markdown plugin). Verification = **dogfood**:
install, then run `/gogo:build`, `/gogo:plan`, `/gogo:go` on a sample repo and
inspect the produced `.gogo/` artifacts. UI/browser testing for *target* projects
uses the bundled **Playwright MCP** (boots via `npx`, needs Node). See
`testing-tools.md` / `test-strategy.md`.

## Optional tooling (graceful — never required)
- `mmdc` (mermaid CLI) — only used for SVG/PNG export if already present.
- `jq` — handy for validating/reading JSON artifacts when present.
- Node.js — for the Playwright MCP, and (dev-time only) to build the `/gogo:xplan` React
  board (`npm run build`).
- `python3` (since 0.7.0) — soft dep for the `/gogo:xplan` browser board: its stdlib
  `server.py` serves the committed React `dist/` + the board API on `127.0.0.1`. Detected
  at use (`command -v python3`); absent → point at `/gogo:done`'s filterable
  ready-to-ship list (no board, no hard failure). The 0.7.0–0.9.0 `board.py` curses TUI
  and its `tmux` soft dep were removed in 0.10.0.
