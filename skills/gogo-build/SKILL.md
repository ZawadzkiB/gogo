---
name: gogo-build
description: >-
  Initialize or refresh gogo's per-project knowledge config. Discovers the
  project's existing docs (Claude/Copilot/Cursor/Windsurf/Codex configs, README,
  manifests, test/CI configs) — searching every depth including nested monorepo
  packages, sweeping all project markdown, and reading in-code doc comments — and
  wires the .gogo/knowledge/* files as proxies that link them, or synthesizes them
  from the codebase when none exist. Run on /gogo:build; idempotent and
  re-runnable (picks up new docs, preserves edits).
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
Glob from the repo root, **recursively**, skipping `node_modules`, `.git`,
`.gogo`, `dist`, `build`, `out`, `.next`, `target`, `vendor`, `coverage`,
`__pycache__`, `.venv`, and other build/output/vendor dirs. Record every hit with
its **full relative path** (a nested hit like `frontend/.github/…` is a distinct
source from a root one — keep both). Three passes:

### 2a — assistant configs & known files (search every depth, not just root)
Each pattern matches at any depth — a monorepo keeps per-package configs under
`frontend/`, `services/api/`, etc., so never anchor these to the root.

- **Claude**: `**/CLAUDE.md`, `**/.claude/`, `**/AGENTS.md`
- **Copilot**: `**/.github/copilot-instructions.md`, `**/.github/instructions/*.instructions.md`
- **Codex / generic**: `**/AGENTS.md`, `**/.codex/`, `**/codex.md`
- **Cursor**: `**/.cursorrules`, `**/.cursor/rules/*.mdc`
- **Windsurf**: `**/.windsurfrules`, `**/.windsurf/`
- **Editor / repo config**: `**/.editorconfig`, `**/.github/PULL_REQUEST_TEMPLATE*`
- **Docs**: `**/README*`, `**/CONTRIBUTING*`, `**/ARCHITECTURE*`, `**/docs/**`
- **Manifests / stack**: `**/package.json`, `**/pnpm-workspace.yaml`, `**/deno.json`,
  `**/pyproject.toml`, `**/requirements.txt`, `**/go.mod`, `**/Cargo.toml`,
  `**/pom.xml`, `**/build.gradle`, `**/Gemfile`, `**/composer.json` + lockfiles
- **Test / lint / CI**: `**/{vitest,jest,playwright,cypress}*` configs,
  `**/pytest.ini`/`**/tox.ini`, `**/*_test.go`, `**/.eslintrc*`/`**/eslint.config.*`,
  `**/ruff.toml`, `**/tsconfig.json`, `**/mypy.ini`, `.github/workflows/*`,
  `**/.gitlab-ci.yml`

### 2b — full markdown sweep
Glob `**/*.{md,mdx}` (minus the skip dirs and `.gogo/` itself) to catch knowledge
that lives outside the well-known filenames — design notes, runbooks, ADRs,
per-module `README`s, `docs/` pages, wiki dumps. Skip pure noise:
`CHANGELOG*`, `LICENSE*`, issue/PR templates, and auto-generated API dumps.
For anything that reads like rules / architecture / conventions / how-to-run,
read its headings + top section and link it; don't slurp huge files.

### 2c — in-code documentation (light, high-signal only)
Many projects document conventions and architecture in **doc comments**, not
markdown. Do a *targeted* pass — module/package entry points and obvious
"conventions/architecture" blocks, **not** every inline comment:

- **JS/TS**: top-of-file JSDoc/TSDoc on entry points / index files (`@module`, `@packageDocumentation`)
- **Python**: module & package `__init__.py` docstrings
- **Go**: package doc comments (`// Package x …`) and `doc.go`
- **Java/Kotlin**: package-level Javadoc (`package-info.java`), class-level docs on core types
- **Rust**: crate/module docs (`//!`) and item docs (`///`) at module roots
- **C#**: XML doc comments (`///`) on public types in entry assemblies

Grep for these markers near package roots; capture the few that describe *how the
code is meant to be built/structured*, and link the file + line.

Read the small/authoritative hits (assistant configs, manifests, test/CI configs,
README/doc tops, doc-comment blocks). Don't slurp giant or generated files — link
them and summarize headings. De-dup: a file found in an earlier pass isn't a new
source in a later one.

## Step 3 — classify each hit to knowledge topics
Classify by **content signal**, regardless of which pass found it — a nested
`frontend/.github/copilot-instructions.md`, a `docs/conventions.md`, and a
package-level Javadoc can all feed `coding-rules`/`code-review-standards` just as
a root config would.

- rules-ish → `coding-rules`, `code-review-standards`
- stack-ish → `tech-stack` (manifests/lockfiles), `testing-tools` (test/lint configs)
- product-ish → `project-knowledge`, `non-functional-requirements` (README/ARCHITECTURE/docs)
- test-ish → `testing-tools`, `test-strategy` (test configs, e2e dirs, CI)

One file can feed several topics (e.g. `CLAUDE.md` → rules + project-knowledge).
When sources for one topic disagree (e.g. a frontend config vs a backend one),
keep both in `Source:` and note the scope each applies to in the summary.

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
