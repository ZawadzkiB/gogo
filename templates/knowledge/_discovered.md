# Discovered project docs & synthesis log

**Purpose:** a snapshot of what `/gogo:build` found in this project and how each
knowledge file was wired. **Regenerated on every `/gogo:build` run — do not
hand-edit.**

<!-- gogo:meta
Mode: report
Generated-by: /gogo:build (scaffold)
-->

_Run `/gogo:build` to populate this file._

## Assistant configs found
<Claude (CLAUDE.md, .claude/, AGENTS.md) / Copilot / Cursor / Windsurf / Codex — ✓ or ✗ per full path, incl. nested monorepo packages e.g. frontend/.github/…>

## General docs & manifests found
<README, CONTRIBUTING, ARCHITECTURE, docs/, package.json + lockfile, test / CI configs>

## Other docs (markdown sweep)
<knowledge-bearing **/*.md(x) outside the known filenames — ADRs, runbooks, design notes, per-module READMEs>

## In-code documentation
<package/module doc comments that describe how the code is built/structured — file:line per hit, or ✗ none>

## Knowledge file wiring
| Knowledge file | Mode | Sources linked / synthesized-from |
|---|---|---|
| project-knowledge.md | — | — |
| tech-stack.md | — | — |
| non-functional-requirements.md | — | — |
| coding-rules.md | — | — |
| code-review-standards.md | — | — |
| testing-tools.md | — | — |
| test-strategy.md | — | — |

## Code verification
**Code is the source of truth; docs may be outdated.** After wiring, `/gogo:build`
cross-checks each high-signal distilled claim against the actual code — on a
conflict **code wins**: the gogo-owned summary is corrected (never the upstream
`Source:` file; that gets a "looks stale" suggestion instead).

| Claim (file › topic) | Result | Detail |
|---|---|---|
| <e.g. tech-stack › test framework> | verified / corrected / unverifiable | <code confirms · doc said X → code shows Y · not mechanically checkable> |

## Needs review (low confidence)
<files where gogo guessed — please confirm>
