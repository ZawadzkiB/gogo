# Review — round 01 — feature `workspace-changelog-viewer` (Stage 1 of 3)

**Phase ③ (review).** Fresh-eyes review of Stage 1 only — the mechanical refactor:
FR1 (`.gogo/plans` → `.gogo/work` rename), FR2 (`.assets` → `.gogo/resources`
move + viewer path math), FR3 (build-time legacy-layout migration). Stage 2/3 work
and the deferred version bump are out of scope and intentionally not flagged.

Contract: `review/issues.json` (round 1). This file is the rendered snapshot.

## Verdict: APPROVE

No blockers, no majors. Two non-blocking findings (1 minor, 1 nit), both
agent-fixable. Stage 1 is correct and faithful to the plan; it can advance to
④ test. Fixing REV-002 before/with Stage 2 is recommended but not required.

## Findings

| id | severity | file:line | finding | fix | tag |
|---|---|---|---|---|---|
| REV-002 | minor | skills/gogo-build/SKILL.md (Step 0 bash) | Partial-migration case (both `.gogo/plans/` and `.gogo/work/` exist) skips the move correctly but the final log line still echoes `migration: already current (no-op)`, contradicting the prose's "flag for a human to merge" and FR3's "logged" requirement — a real conflict reported as success. | Detect the both-exist conflict and `echo "WARN: legacy .gogo/plans/ remains..."`; only emit the no-op line when `.gogo/plans/` is genuinely absent. | AGENT-FIXABLE |
| REV-001 | nit | docs/architecture.md:140 | `resources/` tree line's `#` comment column is one space left of its siblings (`knowledge/`/`skills/`), so the comment column no longer lines up. Cosmetic. | Add one space before the `#` on line 140. | AGENT-FIXABLE |

## What was verified clean (no findings)

**FR1 — rename completeness.** Every tracked plugin file (commands, skills, agents,
templates, `templates/contracts/*`, README, `docs/*`) consistently uses
`.gogo/work`. The only tracked source still naming `.gogo/plans` / `.assets` is
`commands/build.md` + `skills/gogo-build/SKILL.md`, exclusively inside the FR3
migration logic (it must name the legacy path to detect it). No half-renamed doc.
Not over-eager: the verb "plans" ("what it plans against") in README.md and
docs/index.md is correctly left untouched; other features' historical audit-trail
files under `.gogo/work/*/` keep their original paths (move-never-delete) as
intended.

**FR2 — resources move + path math.** `gogo-mermaid` copies the runtime to
`.gogo/resources/mermaid.min.js` via `${CLAUDE_PLUGIN_ROOT}`; the viewer
`<script src>` is `../../../resources/mermaid.min.js` (exactly three `../` — correct
for `.gogo/work/feature-<slug>/charts/` reaching `.gogo/resources/`).
`assets/mermaid/viewer.template.html` correctly *tokenizes* the path
(`GOGO_MERMAID_SRC`) — not a miss. The NFR footprint note (dogfooded
`.gogo/knowledge/non-functional-requirements.md`) reads the new
`.gogo/resources/mermaid.min.js`; the tracked NFR template carries no footprint
note, so nothing is stale there. `docs/architecture.md` places `.gogo/resources/`
as a sibling of `work/` at the `.gogo/` level (not nested).

**FR3 — migration.** Step 0 runs before scaffold/discover (correct order); is
idempotent (skips if migrated; no-clobber if `.gogo/work` exists); move-never-delete
(`mv`); rewrites moved `diagrams.html` script paths with a portable `sed -i.bak`
(plus a stated Grep/Read/Write fallback when `sed` is absent — never installs a
tool); stays `.gogo/`-only; and is logged (Step 6 `_discovered.md` + Step 7 report).
`commands/build.md` advertises it in both the description and body.

**Dogfood.** `.gogo/` now contains exactly `knowledge/ resources/ work/` — no
`plans/`, no `.assets/`. All four `.gogo/work/feature-*/charts/diagrams.html`
reference `../../../resources/mermaid.min.js`, and `.gogo/resources/mermaid.min.js`
exists. The two live knowledge docs updated (`testing-tools.md`,
`project-knowledge.md`) point at the new layout and are minimal/correct (no
over-reach).

**Invariants.** `${CLAUDE_PLUGIN_ROOT}` still used for in-plugin asset paths; plain
ASCII (glyph/arrow exceptions consistent with existing style); enumerations
internally consistent after the rename. No version bump — explicitly deferred to
the cross-cutting stage per the plan (not flagged).

## Route

Clean of blockers/majors → Stage 1 may advance to **④ test**. REV-001/REV-002 are
agent-fixable and can be batched (loop back to ② implement with
`--issues review/issues.json`) or carried into the next stage's implement round.
No needs-user-decision findings.
