# Adjustments — feature `workspace-changelog-viewer`

Running log of changes / clarifications requested during planning.

## 2026-06-30 — Scope add (during build): `/gogo:report` on past/broken runs + `/gogo:done` guidance → FR12
User: "we need a `gogo:report <feature>` command to generate on past or broken
runs — take all plans/review/decisions/state in `.gogo/work/<feature>` and prepare
a report; then `gogo:done` copies it + prepares the changelog entry; if the report
is missing, tell the user to run `gogo:report <feature>` first."

`/gogo:report` already exists (standalone phase ⑤) but is gated on a clean green
run. Added **FR12**: a lenient mode for `/gogo:report <feature>` that produces a
best-effort `report/report.md` from whatever artifacts exist (marking what's
complete vs missing/open) for past/broken runs; the in-pipeline ⑤ keeps its strict
gate. `/gogo:done`'s missing-report STOP message now names `/gogo:report <feature>`.
Folded into the final pass alongside the FR11 docs/version sweep.
