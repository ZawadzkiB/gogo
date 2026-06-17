---
description: Show all gogo features and their phase, status, and iteration counts.
allowed-tools: Read, Bash, Glob, Grep
model: haiku
---

List every `.gogo/plans/feature-*/` and summarise each from its `state.md`: slug,
feature title, phase, status, iteration counts (plan / implement / review / test),
and the resume hint. Flag any `waiting-for-user` feature with its open decision
(from `decisions.md`). Read-only — do not modify anything.
