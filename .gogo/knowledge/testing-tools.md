# Testing tools

**Purpose:** the test tools available and exactly how to run them.

<!-- gogo:meta
Mode: owned
Source: [ ]
Confidence: medium
Generated-by: /gogo:build
-->

## What exists
There is **no automated unit/integration suite** in this repo — it's a markdown
plugin. "Testing a change" means **dogfooding** the pipeline and inspecting the
artifacts it produces.

## Tools
- **The plugin itself** — install the dev build and run the commands:
  `/plugin marketplace add /path/to/gogo` (or `marketplace update` for the GitHub
  build) → `/reload-plugins`.
- **A scratch target repo** — a small project to run `/gogo:build`, `/gogo:plan`,
  `/gogo:go` against; inspect the resulting `.gogo/` tree.
- **`jq`** (if present) — validate JSON artifacts (e.g. an issues list) and assert
  required fields/shape.
- **Bash** — run hooks directly: `bash hooks/notify.sh <<<'{"message":"x"}'`,
  `bash hooks/config-check.sh`.
- **Bundled Playwright MCP** (`.mcp.json`) — for UI/e2e testing of *target*
  projects (the `gogo-tester` agent uses it) **and**, since 0.10.0, the plugin's
  own `/gogo:xplan` React board (drive it against `server.py` on a fixture
  `--data` dir; see `test-strategy.md`). Boots via `npx`; needs Node. Absent
  → fall back to CLI/API checks + written manual steps.
- **mermaid offline viewer** — open `.gogo/work/feature-<slug>/charts/diagrams.html`
  in a browser to confirm diagrams render.
- **Vendored Python tools ship a `--selftest`** (since 0.7.0) — run
  `python3 assets/xplan-board/server.py --selftest` to exercise the `/gogo:xplan`
  board server's guards (intent validation, ready-set derivation, path-traversal,
  Host/Origin) offline; add a `curl` smoke against a running server for the API
  routes. (The 0.9.0 curses board `assets/kanban/board.py` was removed in 0.10.0.)
  `python3` is a soft dep; absent → skip and rely on code-read + the table fallback.

## gogo overrides
<!-- Preserved across re-runs. -->
