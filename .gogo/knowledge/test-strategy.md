# Test strategy

**Purpose:** how to test a gogo change — what to exercise and the done-bar.

<!-- gogo:meta
Mode: owned
Source: [ ]
Confidence: medium
Generated-by: /gogo:build
-->

## How to verify a gogo change (dogfood)
1. **Install the dev build** — `marketplace add /path` or `marketplace update` +
   `install` + `/reload-plugins`. Confirm the new `version` is active
   (`ls ~/.claude/plugins/cache/gogo/gogo/`).
2. **Exercise the affected command(s)** on a scratch target repo:
   - changed `gogo-build` → run `/gogo:build`, check `.gogo/knowledge/*` wiring +
     `_discovered.md`.
   - changed a phase → run `/gogo:plan` then `/gogo:go`, watch the phase behave.
   - changed diagrams → open `charts/diagrams.html`, confirm the right subject.
3. **Inspect artifacts, not vibes.** Open the produced files and assert they match
   the contract (plan shape, issues-list fields, state transitions, report).
4. **Validation hand-offs (for pipeline changes).** Confirm each command rejects a
   malformed/missing input and produces an output the next command accepts.

## Levels
- **CLI / command** — the primary surface; every command runnable standalone.
- **Artifact** — the markdown/JSON each phase writes (the real "output under test").
- **UI** — only for *target* projects via Playwright MCP (gogo-tester); N/A to the
  plugin itself.

## Done-bar
- The changed command(s) run end-to-end on a scratch repo.
- Artifacts conform to their contract; bad inputs are rejected, not propagated.
- All enumerations in sync (grep); version bumped; portability intact.
- For a full feature: review clean + tests green → `report.md` + as-built charts.

## gogo overrides
<!-- Preserved across re-runs. -->
