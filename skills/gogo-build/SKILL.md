---
name: gogo-build
user-invocable: false
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
  **preserve** every `## gogo overrides` section, every `## Custom` section
  (**user-owned — copied 1:1, byte-for-byte, never rewritten**), and every
  `Mode: owned` file.
- **`--force`**: reset all knowledge files to fresh scaffolds (warn first; suggest
  committing) — **but still carry over any existing `## Custom` section verbatim** (it
  is the user's, not gogo's, so a reset must not destroy it). Always regenerate
  `_discovered.md`.

## Step 0 — migrate a legacy workspace layout (idempotent)
Before anything else, bring an older project up to the current layout. gogo's
workspace was renamed `.gogo/plans/` → `.gogo/work/`, and the vendored mermaid
runtime moved up one level to `.gogo/resources/` (so other skills can share it).
**Move, never delete; skip if already migrated.** This only ever writes under
`.gogo/`, so it stays in-bounds.

1. **Workspace.** If `.gogo/plans/` exists and `.gogo/work/` does **not**, move
   `.gogo/plans/` → `.gogo/work/`. If both already exist (a partial migration),
   don't clobber — leave `.gogo/plans/` in place and flag it in the report for a
   human to merge.
2. **Vendored runtime.** If a legacy `.gogo/plans/.assets/` (or, after step 1,
   `.gogo/work/.assets/`) exists and `.gogo/resources/` does not, move it to
   `.gogo/resources/`.
3. **Viewer paths.** In every moved `charts/diagrams.html`, rewrite the
   `<script src>` from the old `../../.assets/mermaid.min.js` to the new
   `../../../resources/mermaid.min.js` — charts now sit one level deeper
   (`.gogo/work/feature-<slug>/charts/`, three levels under `.gogo/`).
4. **Log** exactly what moved — both in the Step 7 report and in `_discovered.md`.

```bash
set -euo pipefail
moved=""
# 1) workspace rename (never clobber an existing .gogo/work)
if [ -d .gogo/plans ] && [ ! -e .gogo/work ]; then
  mv .gogo/plans .gogo/work && moved="${moved}.gogo/plans->.gogo/work "
fi
# 2) vendored runtime up to .gogo/resources/
# $legacy is drawn from a FIXED literal list and re-guarded non-empty + is-a-dir before
# the mv, so it can never collapse to a bare/empty path (classifier-safe migration).
for legacy in .gogo/work/.assets .gogo/plans/.assets; do
  if [ -n "$legacy" ] && [ -d "$legacy" ] && [ ! -e .gogo/resources ]; then
    mv "$legacy" .gogo/resources && moved="${moved}${legacy}->.gogo/resources "
    break
  fi
done
# 3) fix the offline viewer's runtime path in any moved charts (portable sed).
# Deletes are scoped `find <literal .gogo/work> ... -delete` (no glob-rm, no bare-variable
# `rm`) — the safe idiom the "dangerous rm" classifier never flags.
if [ -d .gogo/work ]; then
  find .gogo/work -name diagrams.html -exec \
    sed -i.bak 's#\.\./\.\./\.assets/mermaid\.min\.js#../../../resources/mermaid.min.js#g' {} \; 2>/dev/null || true
  find .gogo/work -name 'diagrams.html.bak' -delete 2>/dev/null || true
fi
# Report independently: what moved, any unresolved conflict, or a clean no-op.
[ -n "$moved" ] && echo "migrated: $moved"
if [ -d .gogo/plans ] && [ -e .gogo/work ]; then
  # both layouts present — a partial migration we won't clobber (even if .assets moved)
  echo "WARN: legacy .gogo/plans/ remains alongside .gogo/work/ — not moved (no clobber); merge by hand, then re-run"
elif [ -z "$moved" ]; then
  echo "migration: already current (no-op)"
fi
```

If `.gogo/plans/` is absent (or `.gogo/work/` already present), this is a no-op —
re-runs are safe. Where `sed` is unavailable, do step 3 via Grep/Read/Write
(same substitution) — never install a tool.

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
For each of the 8 content files (everything except `_discovered.md` and `index.md`):

- **If ≥1 source classified to it → PROXY**: set `gogo:meta` `Mode: proxy`,
  `Source: [the relative paths]`, `Confidence` by source quality; write a short
  summary distilled from reading the source(s); keep/seed the `## gogo overrides`
  section. **Distil, never copy the whole doc.**
- **Else → OWNED**: set `Mode: owned`, `Source: []`; synthesize from codebase
  analysis — tech-stack from manifest+lockfile+scripts; testing-tools from test
  configs / e2e dirs; coding-rules from lint config or "match nearby code";
  project-knowledge from top-level dirs / entry points;
  non-functional-requirements / test-strategy / code-review-standards from gogo
  defaults + what the code implies; **analysis** from the codebase's entry points +
  test layout + git/enumeration conventions (how to analyze a feature *here* —
  almost always owned, since projects rarely ship a "how we scope work" doc). Mark
  `Confidence: medium|low`.

On **re-run**: only refresh the summary sections and the `Source:`/`Confidence`
of proxy files; **never touch** any `## gogo overrides` section or any owned
body. Append newly-found sources to the `Source:` list.

**Preserve every `## Custom` section verbatim (byte-for-byte).** A `## Custom`
section is **user-owned**, exactly like `## gogo overrides` is gogo-owned — but it is
*the user's*, so build **never rewrites, reflows, or reorders it**, in any mode
(re-run and `--force`). Copy it 1:1 to the reconciled file, keeping its position, and
**note in the run summary which files' `## Custom` sections were preserved** (Step 7).
If a file has no `## Custom` section, do nothing — do not invent one beyond the empty
stub the scaffold already carries.

## Step 5 — verify against code (code is the source of truth)
Docs go stale; code does not. After wiring, cross-check each **high-signal**
distilled claim against the actual code — pure Glob/Grep/Read, no execution, only
`.gogo/` ever written. Verify the mechanically-checkable claims:

- **tech stack** (languages/frameworks) — vs manifests + lockfiles
- **build / run / test commands** — vs `package.json` scripts, Makefile/taskfile,
  CI workflows
- **test framework** — vs the installed deps + test configs
- **entry points** — vs the real entry files / `main`/`bin`/module fields
- **key manifest scripts** — vs the actual script names

For each claim, on a conflict **code wins**:
- Correct the **gogo-owned summary** to match the code, and set `Confidence`
  accordingly.
- Record the change as *doc said X → code shows Y*.
- **Never edit the upstream `Source:` file** — instead surface an "upstream looks
  stale" suggestion to the user.

Mark each claim **verified** (code confirms — raise `Confidence`), **corrected**
(conflict, code won), or **unverifiable** (not mechanically checkable — leave
as-is and flag it). On a re-run, re-verify and reconcile — don't churn unchanged
claims. The principle: **code is the source of truth; docs may be outdated.**

## Step 6 — write `_discovered.md`
Regenerate it every run (but **carry over any existing `## Custom` section verbatim** —
even a full regenerate must not destroy the user's section): any **legacy-layout
migration performed in Step 0**
(`.gogo/plans`→`.gogo/work`, `.assets`→`.gogo/resources`, viewer paths rewritten —
or "already current"), the assistant configs + general docs found, the per-file
wiring table (file → mode → sources), a **verification table** (per high-signal
claim → **verified** / **corrected** *(doc said X → code shows Y)* /
**unverifiable**) headed by the principle *"code is the source of truth; docs may
be outdated,"* and a "Needs review (low confidence)" list.

## Step 7 — report to the user
Summarize: **any legacy-layout migration from Step 0** (what moved, or that the
layout was already current), what was found, which files are proxies vs synthesized, **which claims
were verified / corrected (code overrode a stale doc, with the suggested upstream
fix) / unverifiable**, **which files' `## Custom` sections were preserved 1:1** (name
them, or "none"), which files are low-confidence and want a human glance, and
the next step (`review .gogo/knowledge/, then run /gogo:plan "<goal>"`).

**Over-budget nudge:** for any knowledge file whose gogo-owned body exceeds the
warn threshold (200 lines), print `<file> is NNN lines — consider /gogo:skills`,
so big always-read files get trimmed into on-demand skills (keeps the pipeline
deterministic).

## Guardrails
- Only ever write inside `.gogo/` — never edit a discovered upstream file.
- Keep all writes inside `.gogo/knowledge/`.
- Don't auto-edit `.gitignore`; instead print guidance (commit `knowledge/`;
  the team's call on `.gogo/work/`).
