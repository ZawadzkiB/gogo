---
description: Ship a report-complete feature — copy its report bundle into the append-only .gogo/changelog/<date>-<slug>/ and set state.md to a terminal shipped status. Validates inputs.
argument-hint: "[feature-slug]"
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Skill, TodoWrite, AskUserQuestion
model: opus
---

Mark a feature **shipped** and archive it to the changelog, via the `gogo-done`
skill. The explicit post-report "this is the end" gate.

Target: $ARGUMENTS  (if no slug, pick the most recent `.gogo/work/feature-*/`
whose `state.md` is report-complete — phase=done / status=done; if several, ask which.)

Load `gogo-done` and follow it:

1. **validate-in** — the feature must be report-complete:
   `.gogo/work/feature-<slug>/report/report.md` must exist. Missing → STOP with:
   "No report found for `<feature>` — run `/gogo:report <feature>` first, then
   `/gogo:done`." (`/gogo:report` works even on a past/broken run.)
2. **Work** — derive the entry date (the report's `completed:` field, else a date
   you supply, else today; never hardcoded) and **copy** (not move) the report
   bundle — `report.md` + the `.mmd` UML set + `diagrams.html` (+ `manifest.json`) —
   into `.gogo/changelog/<date>-<slug>/`. Append-only and idempotent: re-running
   overwrites the same dated dir.
3. **Finish** — set `state.md` to `status: shipped`, `resume: none`, and confirm the
   archived path.
