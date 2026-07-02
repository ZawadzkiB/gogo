# Adjustments — feature `changelog-merged-entries`

Running log of user-requested changes / clarifications during planning.

## 2026-07-02 — D1 answered with a scope expansion (user, at the acceptance gate)

> "full report isn't it too much? …synthesis should be enough **also for single
> gogo:done work item**, changelog should be just high level info of what was
> changed/done/implemented"

**Change applied to the plan:** the changelog entry is a **synthesized high-level
summary in BOTH modes** — merged releases *and* ordinary single-feature ships. No
`report-<slug>.md` copies, no full-report duplication; the full audit trail
(review/test rounds, decisions detail, per-file changes) stays in
`.gogo/work/feature-<slug>/` and the entry links back to it. FR2 rewritten
accordingly; entry file set slimmed (report.md + `.mmd` set + manifest.json +
`before/`; the static `diagrams.html` copy is dropped from new entries — the
interactive viewer builds from source and remains the way to view).

Existing (already-shipped) heavy entries are left as-is — append-only archive; a
retro-slim migration is out of scope (noted as a possible follow-up).
