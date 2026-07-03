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
  projects (the `gogo-tester` agent uses it). Boots via `npx`; needs Node. Absent
  → fall back to CLI/API checks + written manual steps.
- **mermaid offline viewer** — open `.gogo/work/feature-<slug>/charts/diagrams.html`
  in a browser to confirm diagrams render.
- **Vendored Python tools ship a `--selftest`** (since 0.7.0) — run
  `python3 assets/kanban/board.py --selftest` (and `--headless --ship a,b`) to
  exercise the `/gogo:done` board logic live without a terminal/tmux. `python3` is
  a soft dep; absent → skip and rely on code-read + the table fallback.
- **Go toolchain for `cli/`** (since 0.10.0) — `cd cli && gofmt -l . &&
  go vet ./... && go test -race ./...` (always `-race`; the tui suite depends on
  it). `go build -o gogo .` for a live binary; `gogo status` on this repo's real
  `.gogo/` is a free end-to-end classifier check (golden file in `cli/testdata/`).
- **tmux drive for the Go TUI** (since 0.10.0) — the send-keys/capture-pane
  method (see test-strategy) applies to the `gogo` board exactly as to `board.py`:
  launch detached in a throwaway session, send keystrokes, assert the pane.
- **Stubbed `claude` on PATH** — to test launches without running Claude, prepend
  a scratchpad dir with an executable `claude` stub that records its argv (and a
  call count) to a file; assert **one** argv element (e.g. `/gogo:done a+b`) and
  the exact call count. Same trick works for `tmux` argv probes.

## gogo overrides
<!-- Preserved across re-runs. -->
