---
description: Show all gogo features and their phase, status, work-index class, and iteration counts.
allowed-tools: Read, Bash, Glob, Grep, Skill
model: haiku
---

Show a read-only overview of every gogo feature, via the `gogo-status` skill.

Load `gogo-status` and follow it: run its **work-index classifier** (Step A) to
label each `.gogo/work/feature-*` as **shipped / ready-to-ship / in-progress /
unfinished**, then render the overview (Step B) — slug, feature title, phase,
status, class, iteration counts (plan / implement / review / test), and the resume
hint. Flag any `waiting-for-user` feature with its open decision (from
`decisions.md`). **Read-only — do not modify anything.**
