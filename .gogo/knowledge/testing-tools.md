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
  `bash hooks/config-check.sh`. Since 0.31.0 `notify.sh` also ships a built-in
  suite — `bash hooks/notify.sh --selftest` (44 decision-table cases, sends
  nothing, exit 0/1; run under BOTH `/bin/bash` 3.2 and bash 5.x) — and
  `GOGO_NOTIFY_DEBUG=1` / `GOGO_NOTIFY_DRYRUN=1` for tracing single invocations
  without deliveries.
- **Bundled Playwright MCP** (`.mcp.json`) — for UI/e2e testing of *target*
  projects (the `gogo-tester` agent uses it). Boots via `npx`; needs Node. Absent
  → fall back to CLI/API checks + written manual steps.
- **mermaid offline viewer** — open `.gogo/work/feature-<slug>/charts/diagrams.html`
  in a browser to confirm diagrams render.
- **No vendored python remains (0.33.0)** — `assets/kanban/board.py` (and its
  `--selftest`) was retired with the no-auto-board rule; bare `/gogo:done` is an
  in-chat flow with nothing to selftest. The `--selftest` + exit-code-contract
  convention stays recorded in `coding-rules.md` for any FUTURE authored
  vendored executable.
- **Go toolchain for `cli/`** (since 0.10.0) — `cd cli && gofmt -l . &&
  go vet ./... && go test -race ./...` (always `-race`; the tui suite depends on
  it). `go build -o gogo .` for a live binary; `gogo status` on this repo's real
  `.gogo/` is a free end-to-end classifier check (golden file in `cli/testdata/`).
- **tmux drive for the Go TUI** (since 0.10.0) — the send-keys/capture-pane
  method (see test-strategy): launch the `gogo` board detached in a throwaway
  session, send keystrokes, assert the pane. Since 0.33.0 this covers the S
  sessions panel too (open, R re-assign, K close, esc-to-opener).
- **Stubbed `claude` on PATH** — to test launches without running Claude, prepend
  a scratchpad dir with an executable `claude` stub that records its argv (and a
  call count) to a file; assert **one** argv element (e.g. `/gogo:done a+b`) and
  the exact call count. Same trick works for `tmux` argv probes.

## gogo overrides
<!-- Preserved across re-runs. -->

## Custom
<!-- Yours. gogo never rewrites this section: `/gogo:build` re-runs and the report-phase
     reconcile copy it 1:1 (byte-for-byte), exactly like `## gogo overrides`. Put any
     project notes gogo should read but never touch here — safe to edit or delete. -->
