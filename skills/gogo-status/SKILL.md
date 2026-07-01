---
name: gogo-status
description: >-
  Read-only overview of every gogo feature — and the home of the shared work-index
  classifier that labels each .gogo/work/feature-* as shipped / ready-to-ship /
  in-progress / unfinished. /gogo:status renders that index as a table (phase,
  status, iterations, resume hint); other surfaces (the /gogo:done work board,
  roadmap #7) reuse the same classifier. Never modifies anything.
---

# gogo-status — feature overview + the shared work-index classifier

Two things live here: a **reusable work-index classifier** (Step A) that any
surface can call, and the **read-only `/gogo:status` rendering** (Step B) that
prints it. `/gogo:status` **never writes** — it only reports.

## Work-index classifier (Step A — the reusable step)

Given the repo's `.gogo/`, produce one **work-index record** per
`.gogo/work/feature-*/`. This is the single source of truth for "what state is each
feature in" — consumed by `/gogo:status` (Step B) and by the **`/gogo:done` work
board** (Stage B) and roadmap #7's commenter. It **reads only**.

### Inputs read per feature
- `state.md` — the `phase` and `status` lines, plus `iterations` and `resume`.
- `report/report.md` (new bundle) or a legacy root `report.md` — does a final
  report exist?
- `.gogo/changelog/*/` — is there a `<date>-<slug>/` entry whose slug matches this
  feature (i.e. was it already shipped)?

### Classification (first matching rule wins, top to bottom)

| Class | Rule |
|---|---|
| **shipped** | `state.md` `status: shipped`, **or** a `.gogo/changelog/*-<slug>/` entry with a `report.md` exists for this slug |
| **ready-to-ship** | not shipped, **and** a final report exists (`report/report.md`, or a legacy root `report.md`) |
| **in-progress** | no report, **and** `phase` is one of `implement` / `review` / `test` (or `status` is `implementing` / `reviewing` / `testing`) |
| **unfinished** | anything else — early/`plan` phase, planned but not built, no report |

Notes: a feature that has a report **and** a matching changelog entry is
**shipped** (the changelog wins over ready-to-ship). `aborted` features report as
**unfinished** (flag the `aborted` status in the record).

### Output shape (the documented contract the board + status consume)

An array of records, newest-first (by `state.md` `created` or dir mtime). Each:

```
{
  "slug":           "<feature slug, no 'feature-' prefix>",
  "title":          "<the feature line from state.md>",
  "phase":          "plan|implement|review|test|knowledge|done",
  "status":         "<raw state.md status>",
  "class":          "shipped|ready-to-ship|in-progress|unfinished",
  "report_path":    "<repo-relative report.md path, or null>",
  "changelog_path": "<.gogo/changelog/<date>-<slug>/ path, or null>",
  "iterations":     "plan=N implement=N review=N test=N",
  "resume":         "<state.md resume hint>"
}
```

This is a documented **shape**, not a schema-governed artifact — it is computed on
demand and passed in-memory to the consumer (no file is written; `/gogo:status`
stays read-only). A `class` is always one of the four values above.

### Enumeration (read-only)
```bash
ls -d .gogo/work/feature-*/     2>/dev/null   # every feature
ls    .gogo/work/feature-*/report/report.md 2>/dev/null  # new-bundle reports
ls    .gogo/work/feature-*/report.md         2>/dev/null  # legacy root reports
ls -d .gogo/changelog/*/        2>/dev/null   # shipped entries (match slug suffix)
```

## `/gogo:status` rendering (Step B — read-only)

Run the classifier (Step A), then print one line per feature: **slug**, feature
title, **phase**, **status**, its **work-index class**, iteration counts
(plan / implement / review / test), and the resume hint. Group or sort by class
(shipped · ready-to-ship · in-progress · unfinished) so the overview reads at a
glance. Flag any `waiting-for-user` feature with its open decision (from
`decisions.md`). **Read-only — modify nothing** (no `state.md`, no archive; that is
`/gogo:done`'s job).

## Return
The rendered overview (Step B). When called purely as the classifier by another
surface, return the work-index records (Step A output shape) — nothing is written.
