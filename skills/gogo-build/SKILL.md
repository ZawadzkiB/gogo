---
name: gogo-build
description: >-
  Initialize or refresh gogo's per-project knowledge config. Discovers the
  project's existing docs (Claude/Copilot/Cursor/Windsurf/Codex configs, README,
  manifests, test/CI configs) and wires the .gogo/knowledge/* files as proxies
  that link them — or synthesizes them from the codebase when none exist. Run on
  /gogo:build; idempotent and re-runnable (picks up new docs, preserves edits).
---

# gogo-build — wire (or re-wire) the knowledge config

`.gogo/knowledge/*` are the configuration the pipeline reads. This skill creates
them and keeps them current. It is **pure** — Glob/Grep/Read/Write only, no
compiled tool — so it works in any language ecosystem.

## Modes
- **First run** (no `.gogo/knowledge/`): scaffold + discover + wire.
- **Re-run** (default): *reconcile* — pick up new docs, refresh proxy summaries,
  **preserve** every `## gogo overrides` section and every `Mode: owned` file.
- **`--force`**: reset all knowledge files to fresh scaffolds (warn first; suggest
  committing). Always regenerate `_discovered.md`.

## Step 1 — scaffold (if absent)
Copy `${CLAUDE_PLUGIN_ROOT}/templates/knowledge/*` → `.gogo/knowledge/` for any
file that doesn't exist yet. In default mode, never overwrite an existing file.

## Step 2 — discover (read-only scan)
Glob from the repo root, skipping `node_modules`, `.git`, `dist`, `build`, and
vendored dirs. Record every hit.

- **Claude**: `CLAUDE.md`, `**/CLAUDE.md`, `.claude/`, `AGENTS.md`
- **Copilot**: `.github/copilot-instructions.md`, `.github/instructions/*.instructions.md`
- **Codex / generic**: `AGENTS.md`, `.codex/`, `codex.md`
- **Cursor**: `.cursorrules`, `.cursor/rules/*.mdc`
- **Windsurf**: `.windsurfrules`, `.windsurf/`
- **Docs**: `README*`, `CONTRIBUTING*`, `ARCHITECTURE*`, `docs/**`
- **Manifests / stack**: `package.json`, `pnpm-workspace.yaml`, `deno.json`,
  `pyproject.toml`, `requirements.txt`, `go.mod`, `Cargo.toml`, `pom.xml`,
  `build.gradle`, `Gemfile`, `composer.json` + lockfiles
- **Test / lint / CI**: `vitest|jest|playwright|cypress` configs,
  `pytest.ini`/`tox.ini`, `*_test.go`, `.eslintrc*`/`eslint.config.*`,
  `ruff.toml`, `tsconfig.json`, `mypy.ini`, `.github/workflows/*`, `.gitlab-ci.yml`

Read the small/authoritative hits (assistant configs, manifests, test/CI configs,
README top). Don't slurp giant generated files — link them, summarize headings.

## Step 3 — classify each hit to knowledge topics
- rules-ish → `coding-rules`, `code-review-standards`
- stack-ish → `tech-stack` (manifests/lockfiles), `testing-tools` (test/lint configs)
- product-ish → `project-knowledge`, `non-functional-requirements` (README/ARCHITECTURE/docs)
- test-ish → `testing-tools`, `test-strategy` (test configs, e2e dirs, CI)

One file can feed several topics (e.g. `CLAUDE.md` → rules + project-knowledge).

## Step 4 — wire each knowledge file
For each of the 8 content files (everything except `_discovered.md`):

- **If ≥1 source classified to it → PROXY**: set `gogo:meta` `Mode: proxy`,
  `Source: [the relative paths]`, `Confidence` by source quality; write a short
  summary distilled from reading the source(s); keep/seed the `## gogo overrides`
  section. **Distil, never copy the whole doc.**
- **Else → OWNED**: set `Mode: owned`, `Source: []`; synthesize from codebase
  analysis — tech-stack from manifest+lockfile+scripts; testing-tools from test
  configs / e2e dirs; coding-rules from lint config or "match nearby code";
  project-knowledge from top-level dirs / entry points;
  non-functional-requirements / test-strategy / code-review-standards from gogo
  defaults + what the code implies. Mark `Confidence: medium|low`.

On **re-run**: only refresh the summary sections and the `Source:`/`Confidence`
of proxy files; **never touch** any `## gogo overrides` section or any owned
body. Append newly-found sources to the `Source:` list.

## Step 5 — write `_discovered.md`
Regenerate it every run: the assistant configs + general docs found, the per-file
wiring table (file → mode → sources), and a "Needs review (low confidence)" list.

## Step 6 — report to the user
Summarize: what was found, which files are proxies vs synthesized, which are
low-confidence and want a human glance, and the next step
(`review .gogo/knowledge/, then run /gogo:plan "<goal>"`).

## Guardrails
- Only ever write inside `.gogo/` — never edit a discovered upstream file.
- Keep all writes inside `.gogo/knowledge/`.
- Don't auto-edit `.gitignore`; instead print guidance (commit `knowledge/`;
  the team's call on `.gogo/plans/`).
