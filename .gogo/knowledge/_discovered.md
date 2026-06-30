# Discovered project docs & synthesis log

**Purpose:** a snapshot of what `/gogo:build` found in this project and how each
knowledge file was wired. **Regenerated on every `/gogo:build` run — do not
hand-edit.**

<!-- gogo:meta
Mode: report
Generated-by: /gogo:build
-->

## Assistant configs found
- Claude (`CLAUDE.md`, `.claude/`, `AGENTS.md`) — ✗ none in repo (a global
  `~/.claude/CLAUDE.md` exists but is user-level, not project doc).
- Copilot / Cursor / Windsurf / Codex — ✗ none.

## General docs & manifests found
- `README.md` — ✓ (rich; the authoritative project doc).
- `LICENSE` — ✓ (MIT; not knowledge-bearing).
- `.claude-plugin/plugin.json` — ✓ (manifest + version).
- `.claude-plugin/marketplace.json` — ✓.
- `.mcp.json` — ✓ (bundled Playwright MCP).
- No `package.json` / lockfile / CI / lint / test configs (markdown plugin).

## Other docs (markdown sweep)
- `skills/*/SKILL.md`, `commands/*.md`, `agents/*.md`, `templates/**` — the
  plugin's own behaviour specs. Treated as the codebase itself, not as external
  docs to proxy.

## In-code documentation
- Bash hooks (`hooks/*.sh`) carry header comments documenting their behaviour.
- ✗ no package/module doc-comment conventions (no compiled languages here).

## Knowledge file wiring
| Knowledge file | Mode | Sources linked / synthesized-from |
|---|---|---|
| project-knowledge.md | proxy | `README.md` |
| tech-stack.md | owned | repo structure (markdown plugin; no build) |
| non-functional-requirements.md | owned | `README.md` "Portability & prerequisites" + invariants |
| coding-rules.md | owned | authoring conventions inferred from skills/commands + README |
| code-review-standards.md | owned | gogo defaults + plugin invariants |
| testing-tools.md | owned | repo (no suite; dogfood + Playwright MCP) |
| test-strategy.md | owned | repo (dogfood verification) |

## Needs review (low confidence)
- None low; `testing-tools.md` / `test-strategy.md` are **medium** (synthesized —
  there is no test suite to anchor them). Confirm the dogfood flow matches how you
  actually verify changes.
