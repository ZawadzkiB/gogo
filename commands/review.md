---
description: Run phase ③ review standalone — fresh-eyes review against the project's standards; emit the living issues.json + a review-NN.md snapshot. Validates inputs and outputs.
argument-hint: "[feature-slug]"
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Skill, Task, TodoWrite, AskUserQuestion
model: opus
---

Run **phase ③ (review)** standalone for a feature, via the `gogo-review` skill,
with **validate-in → work → validate-out** (using `gogo-contracts`).
Re-running it after fixes updates the same living `issues.json` in place
("review after fixes" = just re-run this).

Target: $ARGUMENTS  (if no slug, pick the most recent `.gogo/plans/feature-*/`
that has been implemented; if several, ask which.)

Documents it accepts: `plan.md` (required, accepted), `code-review-standards.md` /
`coding-rules.md` / `non-functional-requirements.md` (required knowledge), the
as-built `charts/manifest.json` (optional input), and any existing
`review/issues.json` (optional — updated in place).

Load `gogo-review` and follow it:

1. **validate-in** — `plan.md` present and past acceptance; validate
   `charts/manifest.json` and any prior `review/issues.json` against their
   schemas. Invalid/missing required input → STOP with a contract error.
2. **Work** — delegate to `gogo-reviewer`; update the living `review/issues.json`
   (open → fixed/verified, append `new`); render the `review-NN.md` snapshot.
3. **validate-out** — validate `review/issues.json` against
   `issues-list.schema.json`; write `review/result.json`; route on issues-list
   emptiness (open issues → implement with `--issues`; clean → test). Update
   `state.md`.
