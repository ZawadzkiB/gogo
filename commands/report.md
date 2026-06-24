---
description: Run phase ⑤ report standalone — finalize the plan to as-built, write report.md + as-built diagrams, and update the gogo-owned knowledge docs. Validates inputs.
argument-hint: "[feature-slug]"
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Skill, TodoWrite, AskUserQuestion
model: opus
---

Run **phase ⑤ (report)** standalone for an all-green feature, via the
`gogo-knowledge` skill, with a **validate-in** gate (using `gogo-contracts`).

Target: $ARGUMENTS  (if no slug, pick the most recent `.gogo/plans/feature-*/`
whose `state.md` shows test done / all-green; if several, ask which.)

Documents it accepts: `plan.md` and `state.md` (required), `review/issues.json` /
`test/issues.json` / `charts/manifest.json` (optional inputs to report on), and
the `.gogo/knowledge/*` files (gogo-owned summaries it may update).

Load `gogo-knowledge` and follow it:

1. **validate-in** — tests are all-green (`state.md` test done / no `open`/`new`
   issues in `test/issues.json`) and `plan.md` present; validate any present
   `review/issues.json`, `test/issues.json`, `charts/manifest.json` against their
   schemas. Not yet green → STOP and tell the user to finish ④ test first.
2. **Work** — finalize `plan.md` to as-built, draw the as-built diagram set, write
   `report.md`, update the gogo-owned knowledge docs (never the proxied
   originals), and summarise to the user.
3. **validate-out** — write `report/result.json`; set `state.md` to done.
