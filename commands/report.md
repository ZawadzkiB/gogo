---
description: Run phase ⑤ report standalone — finalize the plan to as-built, write the report/ bundle (report/report.md + as-built UML diagrams), and update the gogo-owned knowledge docs. Also (re)generates a best-effort report for a PAST or BROKEN run from whatever artifacts exist. Validates inputs.
argument-hint: "[feature-slug]"
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Skill, TodoWrite, AskUserQuestion
model: opus
---

Run **phase ⑤ (report)** standalone via the `gogo-knowledge` skill, with a
**validate-in** gate (using `gogo-contracts`).

Two ways it runs (the gate is the only difference — see `gogo-knowledge`):

- **All-green feature** — the normal case: finalize the plan to as-built and write
  the full `report/` bundle.
- **Past / broken / incomplete run** — standalone `/gogo:report <feature>` does
  **not** refuse on a non-green run. It synthesizes a **best-effort**
  `report/report.md` from whatever exists in `.gogo/work/<feature>/` (`plan.md`,
  `decisions.md`, `review`/`test` issues + snapshots, `state.md`, `charts/`, any
  `implement/result.json`) and **clearly marks which phases ran, which didn't, and
  what is still open** (a "Run status / gaps" section). `plan.md` is the one true
  prerequisite — if even that is missing, it STOPs.

Target: $ARGUMENTS  (if no slug, pick the most recent `.gogo/work/feature-*/`;
prefer one whose `state.md` shows test done / all-green, else the most recent; if
several, ask which.)

Documents it accepts: `plan.md` and `state.md` (required), `review/issues.json` /
`test/issues.json` / `charts/manifest.json` (optional inputs to report on), and
the `.gogo/knowledge/*` files (gogo-owned summaries it may update).

Load `gogo-knowledge` and follow it:

1. **validate-in (strict vs lenient)** — `plan.md` must exist (the one true
   prerequisite; missing → STOP). For a clean green run, also confirm tests are
   all-green and validate any present `review/issues.json`, `test/issues.json`,
   `charts/manifest.json` against their schemas. **For a past/broken run, do not
   refuse** — proceed in lenient mode, validating present artifacts but treating a
   missing/red one as a gap to report, not a STOP.
2. **Work** — finalize `plan.md` to as-built, draw the as-built UML set (chosen by
   what changed) into `report/` (lenient: only what the available artifacts
   support), write `report/report.md` (a **Run status / gaps** section marking what
   ran vs what's missing/open, plus implementation + decisions & reasons +
   outcomes), update the gogo-owned knowledge docs (never the proxied originals),
   and summarise to the user.
3. **validate-out** — write `report/manifest.json` + `report/result.json`; set
   `state.md` to done.
