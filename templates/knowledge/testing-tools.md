# Testing tools

**Purpose:** the concrete tools the test phase uses and exactly how to invoke
them. (How to *use* them is in `test-strategy.md`.)

<!-- gogo:meta
Mode: proxy
Source: [ ]            # e.g. ../../playwright.config.ts, ../../vitest.config.ts, ../../package.json
Confidence: low
Generated-by: /gogo:build (scaffold)
-->
> The tools + how to run them. Strategy lives in `test-strategy.md`.

## Inventory
| Concern | Tool | How to run |
|---|---|---|
| unit / integration | <vitest \| jest \| pytest \| go test> | `<cmd>` |
| e2e / browser | Playwright (bundled `gogo-playwright` MCP) | `<cmd>` / MCP `browser_*` tools |
| typecheck | <tsc \| mypy> | `<cmd>` |
| lint | <eslint \| ruff> | `<cmd>` |
| build / deploy check | <…> | `<cmd>` |

## Browser tooling
The gogo plugin bundles a Playwright MCP server (`gogo-playwright`). Drive the UI
via its `browser_*` tools. If it is unavailable (e.g. no Node), fall back to
API / CLI tests and write manual UI-check steps into the test report.

## Where tests live
<e2e/, *.test.ts co-located, tests/, …>

## gogo overrides
