# Review — round 1 — feature `changelog-merged-entries`

Fresh-eyes review of the working-tree diff on top of `f3e0521` (v0.7.0). Scope:
`skills/gogo-done`, `skills/gogo-status`, `commands/done.md`, `skills/gogo`,
`README.md`, `docs/{commands,flow,architecture}.md`, `plugin.json`,
`templates/contracts/charts-manifest.schema.json`. Reviewed against `plan.md`
(FR1–FR4), `decisions.md` (D1 custom / D2=A / D3=A), `code-review-standards.md`
and `non-functional-requirements.md`.

## What holds up (verified)

- **Unified writer is genuinely single-sourced.** "Ship one feature" is fully
  gone (grep-clean); `<slug>`, `slug1+slug2`, and both board paths all funnel
  through the one **"Write changelog entry (1..N members)"** flow. N=1 and N>1
  share one shape — no divergent single-vs-merged branch.
- **Three-outcome board routing intact** (confirmed→merge gate→write; exit 1 =
  cancel; anything else = fallback). Merge gate correctly: 0=stop, 1=no question,
  ≥2=one `AskUserQuestion` separate-vs-merged. `+`-arg pre-answers merged. D2
  suggest+confirm release name present. Degradation chain updated to
  classify → select → merge gate → write.
- **Date derivation** = newest member `completed:` via lexical ISO compare, single
  member = its own date, no hardcoding, `date +%F` last resort.
- **Idempotency** preserved and improved — the writer clears `*.mmd` + `before/`
  in the target dir before refreshing, so a re-ship with a changed member set
  leaves no stale prefixed files; writes stay inside `.gogo/changelog/<date>-<name>/`.
- **`+` grammar is unambiguous** — slugs are kebab-case (no `+`), so splitting is
  safe; no collision with gogo-view's `:plan`/`:report` grammar.
- **Classifier (gogo-status)** rule 1 correctly adds a `members[]` match with
  first-match precedence preserved; `changelog_path` note covers the merged dir;
  the `cat …/manifest.json 2>/dev/null` hint is crash-safe on malformed/absent
  manifests (LLM reads text, no `jq` hard-parse). Old entries still match by
  folder slug.
- **Schema change is clean** — `members` is additive, optional, kebab-case items,
  root stays `additionalProperties:false`; OLD manifests (no members) still
  validate.
- **Viewer needs no code change** — slim (no `diagrams.html`) entries enumerate and
  render; compare mode pairs by **basename**, and since both `before/<slug>-x.mmd`
  and `<slug>-x.mmd` carry the same prefix, per-member/per-kind pairing still
  matches.
- **NFR/portability** — offline throughout, `.gogo/`-only writes, `board.py`
  untouched (empty diff), no bytecode, shipped entries left as-is, soft-dep
  degradation intact. `plugin.json` = **0.8.0** exactly.

## Findings

| id | sev | pri | title | fix |
|---|---|---|---|---|
| REV-001 | major | P1 | `docs/index.md:83` still says `/gogo:done` "copies the bundle to `.gogo/changelog/<date>-<slug>/`" for a single "report-complete feature" — stale, and violates the plan's own "no stale copies the report bundle" acceptance test | AGENT-FIXABLE |
| REV-002 | minor | P2 | `templates/contracts/README.md:55` documents the manifest as `{ slug, updated?, note?, diagrams[] }` — omits the new `members?` | AGENT-FIXABLE |
| REV-003 | minor | P2 | gogo-done step 4 under-specifies entry `manifest.json` diagrams (names only `title`, not the schema-required `kind`+`file`) — risk of a contract-invalid manifest | AGENT-FIXABLE |
| REV-004 | minor | P3 | `<date>-<slug>` wording now inaccurate for merged entries (name ≠ slug) in gogo-view labels/arg-grammar, `commands/view.md:11` hint, gogo-status output-shape example | AGENT-FIXABLE / scope call |

Details, pointers, and proposed fixes are in `review/issues.json`.

## Notes for the fixer

- REV-001 is the only thing gating approval. It is a one-line doc edit and is an
  explicit plan acceptance criterion (FR4 sync sweep) — the sweep simply missed
  `docs/index.md` (not in the plan's file list, but the standard requires it).
- REV-003 does not break the viewer (gogo-view infers kind and captions from
  title), but the entry manifest is validated against `charts-manifest.schema.json`,
  so spell out `kind` + `file` per diagram to keep the artifact contract-valid.
- REV-004 straddles the plan's "zero viewer changes" scope; the user-visible bit is
  `commands/view.md`'s arg hint. Reasonable to fix the hints and leave gogo-view's
  internal examples, or to wontfix as out-of-scope.

## Verdict

**CHANGES** — 1 open major (REV-001). No blockers; the three minors are
polish/contract-hygiene.
