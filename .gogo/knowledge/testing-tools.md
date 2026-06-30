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

## gogo overrides
<!-- Preserved across re-runs. -->
