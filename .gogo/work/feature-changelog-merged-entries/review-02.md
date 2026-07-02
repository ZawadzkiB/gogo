# Review — round 2 (fix verification) · `changelog-merged-entries`

**Phase:** ③ review · **Round:** 2 · **Date:** 2026-07-02 · **Mode:** fix verification
(re-check REV-001..004 from round 1 + a regression glance over the touched files)

Round 1 returned CHANGES with REV-001 (major) + REV-002/003/004 (minor), all marked
`fixed` by implement in round 2. This round verifies each fix specifically and glances
at the 6 touched files for new inconsistencies. It is **not** a full re-review.

## Verification results

| id | sev | prior status | verdict | evidence |
|---|---|---|---|---|
| REV-001 | major | fixed | **VERIFIED** | `docs/index.md:83` now: "Synthesizes a high-level entry (`report.md` **written**, not copied) into `.gogo/changelog/<date>-<name>/` …"; single-or-merged framing + no-slug board note. No "copies" wording anywhere on the page. |
| REV-002 | minor | fixed | **VERIFIED** | `templates/contracts/README.md:55` field list = `{ slug, updated?, note?, diagrams[], members? }`; :62-66 clause mirrors the schema (additive/optional, one-vs-several, absent on plain manifests, read by gogo-status). |
| REV-003 | minor | fixed | **VERIFIED** | `skills/gogo-done/SKILL.md:156-170` specifies each `diagrams[]` entry as `{ kind, file, title }`, gives the kind-inference mapping, and states the manifest must validate against `charts-manifest.schema.json`. |
| REV-004 | minor | fixed | **VERIFIED** | `<date>-<slug>` → `<date>-<name>` across gogo-view (enumeration/arg-grammar+parenthetical/picker/output-name/bundle-layout), `commands/view.md:10`, `gogo-status:60`. Arg-resolution semantics unchanged (git diff: "Resolves to" column untouched). |

## Detail

**REV-001 (major → verified).** `docs/index.md:83` was the one row the FR4 sweep missed.
It now matches the other synced docs: synthesis (written, not copied), `<date>-<name>`
dir, single-or-merged framing, viewer-link output. Line 75 was already correct.

**REV-002 (minor → verified).** The human-facing contracts index is back in sync with
`charts-manifest.schema.json` (which carries the additive optional `members` array). The
new README clause reads consistently with the schema's own `members` description.

**REV-003 (minor → verified).** The entry-writer no longer risks emitting a
contract-invalid manifest: every `diagrams[]` entry is spelled out as schema-complete
`{ kind, file, title }`, with `kind` carried from the member manifest else inferred
(`flowchart`/`graph`→`flow`, `sequenceDiagram`→`sequence`, `classDiagram`→`class`,
`stateDiagram`→`activity`) — consistent with the schema enum and gogo-view's kind
handling — and an explicit must-validate statement. The secondary `slug`=release-name
semantic stretch stands (release name is kebab-cased, matches the pattern).

**REV-004 (minor → verified).** Wording-only accuracy pass, exactly as scoped. The
`git diff` for `skills/gogo-view/SKILL.md` shows every hunk is a label change; the arg
grammar's **Resolves to** column ("that changelog report") is untouched, enumeration
still globs `.gogo/changelog/*/`, so a typed `<date>-<name>` still resolves by directory
match — behaviour unchanged. The arg-grammar row gained a parenthetical clarifying
`<name>` = slug (single) / release name (merged).

## Regression glance (the 6 touched files)

- **Stale "copies the bundle" wording:** repo-wide grep for `copies the (report )?bundle`
  returns **zero** hits outside `.gogo/work` — only `plan.md` (its own acceptance-test
  text) and `review-01.md` (the round-1 snapshot). Clean.
- **`<date>-<slug>` remaining:** only three, all **legitimate single-vs-merged contrasts**,
  not stale: `gogo-status:28` and `:45` (the single-entry side paired with `<date>-<name>`
  for the merged case) and `gogo-done:111` (single-member `<date>-<slug>` vs merged
  `<date>-<release-name>`). `gogo-view` and `commands/view.md` have none.
- **Broken example paths / front-matter drift:** none introduced. `gogo-view` line-49
  output token `<date-or-slug>.html` is pre-existing, untouched, and still accurate
  shorthand for the report page name. Front-matter descriptions of the touched skills are
  unchanged and consistent with the new `<date>-<name>` wording.

## Verdict

**APPROVE** — all four round-1 findings verified fixed; no open blockers or majors; the
regression glance over the 6 touched files is clean.
