# Report — feature `<slug>`

<!-- Written by phase ⑤ (gogo-knowledge): for a clean green run, or — standalone
via /gogo:report <slug> — as a best-effort report for a PAST/BROKEN run. Rendered
to .gogo/work/feature-<slug>/report/report.md (NOT the feature root). The durable,
as-built summary of what shipped — the companion to plan.md (the contract), and
the bundle /gogo:done archives to .gogo/changelog/. Fill every section from the
ACTUAL work; link the audit-trail files (../decisions.md, ../review-NN.md,
../test-NN.md) rather than repeating them. -->

- **feature:** <one-line title>
- **status:** done   <!-- a past/broken run: "report-only (incomplete run)" -->
- **completed:** <YYYY-MM-DD>
- **branch / commits:** <branch | commit range | n/a>

## Run status / gaps
<Which phases ran (plan / implement / review / test / report) and which didn't,
plus any still-open or unverified issues. Clean green run → one line ("all phases
completed; no open issues"). A past/broken run (lenient /gogo:report) → REQUIRED:
be honest about what's missing/incomplete and list every open review/test finding
so the reader knows this is best-effort, not a clean release.>

## Summary
<2–4 sentences: what was built and why, in plain terms.>

## Planned vs shipped
<What matched the accepted plan, and every deviation (added / dropped / changed)
with the reason. If it shipped exactly as planned, say so in one line.>

## Implementation
<What was ACTUALLY built — the as-built behaviour and the approach taken (not the
plan's intentions). The reader should understand how the shipped feature works.>

### Changes (as-built)
<The files actually touched, grouped by area — the real changes checklist.>

| File | Change | Note |
|---|---|---|
| `<path>` | added / modified / removed | <what & why> |

## Decisions & rationale
<Every fork resolved this feature, reconciled from decisions.md + the implement
rounds. For EACH decision give both the choice AND the reason for it.
See [decisions.md](../decisions.md).>

| Decision | Choice | Reason |
|---|---|---|
| `<D# / topic>` | <what was chosen> | <why> |

## Review outcome
<Rounds run; notable findings and how they resolved. See the
[review-NN.md](../review-01.md) files / [review/issues.json](../review/issues.json).>

## Test outcome
<Levels exercised (UI / CLI / API) and results. See the
[test-NN.md](../test-01.md) files / [test/issues.json](../test/issues.json).
Note anything skipped and why (e.g. browser tooling absent).>

## Diagrams
<The as-built UML set, chosen by what changed — open [diagrams.html](./diagrams.html)
(same folder). List each `.mmd` with one line: flow, sequence, activity
(lifecycle / state), class (structure / types), use-case (a new user capability).
If the change was pure process, say so — no diagram.>

## Before / after comparison
<If plan ① captured a before (as-is) set — copied into this bundle as
report/before/*.mmd — compare it to the as-built after set. For each kind present in
BOTH, show the before and after diagrams side by side (fenced mermaid blocks, before
then after) with a short prose "what changed"; note any kind added (after only) or
removed (before only). No before set (feature predates this, or none drawn) → say so
in one line and show only the after set above. Side-by-side + prose only — no
computed node-diff (decision D4).>

## Knowledge updates
<Which `.gogo/knowledge/*` files were updated (gogo-owned summaries only). List any
"consider upstreaming to CLAUDE.md / README" suggestions for the user.>

## Follow-ups & known limitations
<Out-of-scope items, deferred work, tech debt, known gaps.>

## Summary (TL;DR)
<The FINAL section — a closing recap in a few bold-led lines: **what shipped**,
the **review verdict** and the **test verdict** (one line each), and a pointer to
the **follow-ups** above. A reader who skims only this block should know what
landed and whether it is clean.>
