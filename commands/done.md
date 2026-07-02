---
description: Ship report-complete features into a high-level changelog — with a slug, ship that one; with slug1+slug2 ship those as ONE merged release entry; with no slug (or a filter word), classify every .gogo/work/feature-*, print the four-class status table, and offer the ready-to-ship items as a filterable AskUserQuestion multi-select where selecting MULTIPLE items merges them into ONE entry (release name suggested + confirmed) and one pick is one entry. Every entry is a SYNTHESIZED report.md (not a copy of the report bundle) + the slug-prefixed .mmd set + a manifest.json with members[] + before/, plus a printed interactive viewer link; each member's state.md becomes shipped. Validates inputs.
argument-hint: "[feature-slug | slug1+slug2+... | filter-text]"
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Skill, TodoWrite, AskUserQuestion
model: opus
---

Mark features **shipped** and write a high-level entry into the changelog, via the
`gogo-done` skill. The explicit post-report "this is the end" gate. The changelog
reads like a release history; the full audit trail stays in `.gogo/work/` (linked).

Target: $ARGUMENTS  (a **slug** ships that one; **`slug1+slug2+...`** ships those as
ONE merged release entry; **no slug** opens the ready-to-ship **list** over every
`.gogo/work/feature-*`; a **non-slug word** filters that list.)

Load `gogo-done` and follow it:

1. **validate-in** — each **named** slug (or every `+`-joined slug) must be
   report-complete: `.gogo/work/feature-<slug>/report/report.md` must exist. Missing →
   STOP with: "No report found for `<feature>` — run `/gogo:report <feature>` first,
   then `/gogo:done`." (`/gogo:report` works even on a past/broken run.) For the **list**
   (no slug), if there are no work items at all, say so and stop; if there are items but
   none ready-to-ship, say so and stop — the list only ships ready-to-ship items. A
   non-resolving arg is not an error — it becomes a text filter for the list.
2. **List** — a slug / `slug1+slug2` resolves directly; **no slug** (or a filter word)
   opens the ready-to-ship list: classify every work item via the shared gogo-status
   Step A classifier (shipped · ready-to-ship · in-progress · unfinished), print the
   four-class status table for context, then offer the **ready-to-ship** items as a
   filterable `AskUserQuestion` **multi-select**. A non-slug arg is a case-insensitive
   substring filter over slug+title; with more ready items than fit one question, ask a
   filter first. It mentions `/gogo:view <slug>` (open a card's page) and `/gogo:go
   <slug>` (run/resume the pipeline).
3. **Multiple = merge (no extra question)** — **selecting multiple items merges them into
   ONE entry**; one pick is one entry. A `+`-joined arg is the same merge signal. For a
   merged entry, gogo suggests a release name from the members' common theme and confirms
   it (you can override). There is no extra merge-or-split question.
4. **Write the entry (per entry)** — a **synthesized** high-level `report.md` (*what was
   changed/done/implemented*, key outcomes, one-line decisions, review/test verdict,
   member table + per-member section when merged, links back to each `.gogo/work/`
   folder) — **written, never a copy** — plus the slug-prefixed `.mmd` set, a merged
   `manifest.json` carrying a `members[]` array, and the merged `before/` set, into
   `.gogo/changelog/<date>-<name>/` (date = newest member's `completed:`; append-only,
   idempotent). **No `diagrams.html` copy** — the viewer builds from source.
5. **Finish** — set **each member's** `state.md` to `status: shipped`; **build the
   interactive viewer page** for each entry (reusing the `gogo-view` build,
   best-effort) and **print its `file://` link** (changelog folder path as fallback —
   never fail `/gogo:done` over the link); confirm each entry.
