# Adjustments — feature `project-scope-and-gates`

Log of changes / clarifications requested during planning (newest first). Each entry:
date · what changed · why.

<!-- No adjustments yet — initial plan awaiting acceptance. -->

## 2026-07-20 — accepted; FR4=B (skill change) + all-in-one 0.24.0
User chose FR4 option B (the gogo skills honor `--skip-acceptance`/`--skip-uat` params) over the
analyst's rec A (CLI auto-fires the gate skill). So this feature DOES include a gogo-skill change
(additive optional params). All four FRs ship together as 0.24.0 (not phased across versions).
