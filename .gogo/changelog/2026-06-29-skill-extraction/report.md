# Report — feature `skill-extraction`

- **feature:** Knowledge skill-extraction command (`/gogo:skills`)
- **status:** done
- **completed:** 2026-06-29
- **branch / commits:** main · uncommitted (no commit requested)

## Summary
Added `/gogo:skills` — a knowledge-maintenance command (sibling to `/gogo:build`)
that keeps `.gogo/knowledge/*.md` lean so the pipeline's LLM workers stay
deterministic. It audits each knowledge file against a line budget (OK `<200` /
WARN `200-400` / OVER `>400`), auto-discovers cohesive sections worth pulling out
(or takes a directed `"<prompt>"`), **classifies each candidate** as a `knowledge`
skill (→ `.gogo/skills/`, pipeline-loaded via a `Load when:` pointer) or a
`standalone` skill (→ `.claude/skills/`, harness auto-discovered), **proposes and
STOPS** for per-candidate approval, then extracts each into a standalone `SKILL.md`
(+ optional `scripts/`/`.env.example`), replaces the parent section with a pointer,
and updates a `.gogo/skills/index.md` registry. Shipped with a full
`docs/architecture.md` explaining the model.

## Planned vs shipped
Shipped **as planned** — all of FR1–FR11 plus the mid-flight scope add **FR12**
(the architecture doc). No features dropped or changed. The only post-plan
refinement was during planning (D1 reframed from a single global location to a
**per-candidate kind**, logged in [adjustments.md](./adjustments.md)); the
implementation followed that accepted model. Version bumped **0.2.0 → 0.3.0**.

## Changes (as-built)

| File | Change | Note |
|---|---|---|
| `commands/skills.md` | added | thin entry point; modes + `--warn`/`--max`/`--include` args |
| `skills/gogo-skills/SKILL.md` | added | the operating manual (116 lines, under its own 200 budget) |
| `templates/skill.template.md` | added | scaffold for an extracted skill (frontmatter + standalone body + scripts/.env) |
| `templates/skills-index.template.md` | added | `.gogo/skills/index.md` registry scaffold (kind · destination · trigger · source · lines saved) |
| `docs/architecture.md` | added | FR12 — flow-vs-knowledge split, knowledge-vs-skills split, full file map |
| `skills/gogo-build/SKILL.md` | modified | Step 6 over-budget nudge |
| `skills/gogo/SKILL.md` | modified | `Load when:` on-demand note in the knowledge-read guidance |
| `.gogo/knowledge/coding-rules.md` | modified | 200/400 budget + user-gated `.claude/skills/` exception (gogo overrides) |
| `.gogo/knowledge/non-functional-requirements.md` | modified | determinism budget bar + safety exception |
| `templates/knowledge/coding-rules.md` | modified | same budget rule, so new projects inherit it |
| `templates/knowledge/non-functional-requirements.md` | modified | same; added a `## gogo overrides` section |
| `README.md` | modified | `/gogo:skills` in Commands; `.gogo/skills/`+`.claude/skills/` in "What gets created"; "How it works" pointer |
| `.claude-plugin/plugin.json` | modified | version 0.2.0 → 0.3.0 |

## Decisions
- **D1** — loading model is a **per-candidate kind** (`knowledge` → `.gogo/skills/`,
  `standalone` → `.claude/skills/`), not one global location; `.claude/skills/`
  written only on explicit per-candidate approval. **D2** H2 granularity (H3 when
  oversized). **D3** 200/400 thresholds (overridable). **D4** prose + registry, no
  typed JSON contract in v1. All resolved — see [decisions.md](./decisions.md).

## Review outcome
2 rounds. Round 1 verdict **APPROVE** — 0 blockers, 0 majors, 2 minors
(REV-001: `coding-rules.md` invariant didn't mirror NFR's user-gated exception;
REV-002: `--include` advertised but not wired into the audit step). Both fixed in a
scoped pass and **verified**. Plan fidelity (FR1–FR12), hard invariants, budget
self-consistency, and enumeration sync all confirmed. See
[review/issues.json](./review/issues.json) · [review-01.md](./review-01.md).

## Test outcome
**GREEN.** Dogfood per `test-strategy.md` (markdown plugin — no compile/unit
suite): a throwaway scratch fixture (OVER/WARN/OK + proxy files) was built and the
`gogo-skills` procedure walked literally against it, producing real sample
extraction artifacts. All 10 plan test items verified — thresholds + overrides,
proxy body-only measurement, discovery (incl. no false positives), classification,
propose-then-STOP, knowledge + standalone extraction, idempotency, directed mode,
`--include` report-only, and the build-nudge/template/docs integration. 4 doc
findings (TEST-001–004: architecture-tree completeness, a misleading comment, a
Modes-section ambiguity, and a missing discovery floor) fixed + **verified**.
**Skipped:** browser/Playwright (no UI in this change) and a live harness install
(environment lacks marketplace) — the approval gate + live invocation were verified
by instruction-walk, not a live run. See [test/issues.json](./test/issues.json) ·
[test-01.md](./test-01.md).

## Diagrams
As-built — open [charts/diagrams.html](./charts/diagrams.html):
- **Flow** (`charts/skills-flow.mmd`) — the command runtime: audit/directed →
  classify → approval gate → kind-routed extract → re-measure → report.
- **Sequence** (`charts/sequence.mmd`) — the propose → STOP → per-candidate
  approve → kind-routed extract hand-off, with parent-pointer + registry update.

No class/activity diagrams: the change adds no new types or state machine that
would carry signal beyond the flow + sequence.

## Knowledge updates
gogo-owned summaries updated (this is the gogo plugin repo, so these are both the
project's config **and** the shipped templates):
- `coding-rules.md` (this repo + template) — the 200/400 authoring budget and the
  documented user-gated `.claude/skills/` write exception.
- `non-functional-requirements.md` (this repo + template) — the determinism budget
  bar; softened the absolute "writes confined to `.gogo/`" Safety bullet to name
  the one exception.
- `project-knowledge.md` (this repo) — gogo-overrides note recording `/gogo:skills`,
  `.gogo/skills/`, and the knowledge-vs-on-demand-skills split.
- No upstream/proxied files were edited. **Consider upstreaming:** nothing required —
  `README.md` (the proxied source for `project-knowledge.md`) was already updated as
  part of this feature.

## Follow-ups & known limitations
- **Live dogfood (residual done-bar):** run `/gogo:skills` once against a real
  over-budget project after a marketplace install to confirm the `AskUserQuestion`
  approval gate fires live (verified here only by instruction-walk).
- **Not committed:** working tree is uncommitted by design (no commit requested).
- **Out of scope (as planned):** the reverse op (re-inlining a skill), cross-file
  skill synthesis, and a typed `skill-proposal` JSON contract — revisit if
  `/gogo:go` ever needs to chain extraction.
- **Roadmap (captured separately, not this feature):** pre/post per-phase agent
  extensions; xplan integration (UML/design diff at report + a written-vs-updated
  test manifest).
