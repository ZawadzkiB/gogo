# Release — `analyst-uat-and-cli-ops`

- **shipped:** 2026-07-04
- **members:** `analyst-uat-and-cli-ops`
- **plugin version:** 0.11.0 (plugin + `gogo` CLI, versions moved together)

## What shipped

**The pipeline got a brain at the front and a human gate at the back — 0.11.0.**
Phase ① gained its own fresh-context specialist, the **`gogo-analyst`** agent,
driven by **`analysis.md`** — a new 10th knowledge file that turns "read the
knowledge" into an ordered per-project procedure: which files to read by name,
what to inspect in the code (entry points, behavior specs, git history, blast
radius), a capability-detected external-docs hook, and the code-is-source-of-truth
rule. At the other end, phase ⑤ no longer ends at `done`: it lands at
**`awaiting-uat`**, where the user verifies the work — **running `/gogo:done` IS
the acceptance** (the plan-gate symmetry, no confirmation ceremony), while issues
route back through a locked, analyst-driven re-plan of the **same work item**
(`uat.md` records every round; `uat=N` counts loop-backs; mid-re-plan features
are refused by both `/gogo:done` and `/gogo:go`).

Key outcomes:

- **Planning intelligence** — `gogo-analyst` joins developer/reviewer/tester;
  `gogo-plan` rewritten as its operating manual; every command doc states the
  **orchestrator-first** chain uniformly (command → orchestrator → specialist).
- **The UAT gate** — a state-machine extension, not a new pipeline: ⑤ ends at
  `awaiting-uat`; acceptance = `/gogo:done` (recorded as a `uat.md` verdict
  line); issues → lock (`waiting-for-user`) → analyst plan delta → re-accept →
  rerun ②→⑤ on the same item. Three additive events (`uat-opened`, `uat-failed`,
  `uat-passed`) with single owners; pre-0.11 `done` features still ship.
- **`## Custom` sections** — user-owned regions in every knowledge file,
  preserved byte-for-byte by `/gogo:build` (default and `--force`), exempt from
  phase-⑤ reconciles and `/gogo:skills` extraction.
- **CLI ops (0.11.0)** — `x` deletes a card to a recoverable **`.gogo/trash/`**
  (`gogo trash` lists/restores; the changelog is un-deletable at two layers);
  `l` peeks at a live session read-only (`tmux capture-pane`, never an attach);
  launches run Claude in **`--permission-mode auto`** (a
  `GOGO_CLAUDE_PERMISSION_MODE` tri-state overrides) so shipped skills stop
  nagging unattended sessions; `awaiting-uat` badge + status-gated classifier.

## Decisions (one line each)

- **D1 — UAT gate mechanic:** plan-gate symmetry — `/gogo:done` IS the
  acceptance, no confirm question; issues route to the analyst (custom answer).
- **D2 — launched-session permissions:** `--permission-mode auto` + env
  tri-state, never `--dangerously-skip-permissions`; `gogo-done` slimmed to file
  ops + synthesis so auto mode covers it (custom answer).
- **D3 — delete semantics:** move to `.gogo/trash/` with restore — destructive
  from a TUI must be reversible; the changelog stays append-only.
- **D4 — awaiting-uat on the board:** a badge on ready cards — classifier
  classes stay stable, the frozen contract stays additive.
- **REV-004 (orchestrator-resolved):** the mid-UAT lock — raising issues sets
  `waiting-for-user` until re-acceptance, making accept and re-plan branches
  mutually exclusive.
- **TEST-004 (orchestrator-resolved):** ready-to-ship gates on **status**, never
  artifact presence — a UAT rerun's stale `report/` no longer classifies as
  shippable.

## Review / test verdict

Six implement, three review, and one test round closed with all **22 findings
(REV-001..012 + TEST-001..010) verified fixed, zero open** — round 2 caught the
mid-UAT lock gap, live tmux driving caught the classifier stale-report and
session cross-attribution majors (both re-reproduced in both directions), and
`-race` ran green across 8 Go packages (33 new tests).

## Notable

This feature exited through the gate it built: its report landed at
`status: awaiting-uat`, and this changelog entry's own ship was the **first
production traversal of the UAT gate** — accepted via `/gogo:done`, recorded as
UAT round 1 in [uat.md](../../work/feature-analyst-uat-and-cli-ops/uat.md).

## Diagrams

Slug-prefixed as-built set beside this report (open interactively with
`/gogo:view 2026-07-04-analyst-uat-and-cli-ops` or the `gogo` CLI): the
UAT-gated pipeline flow and the UAT-loop sequence — the feature's heart;
`before/` carries the plan-time baseline (no analyst, no gate, nagging
launches) for the viewer's compare mode.

## Summary (TL;DR)

0.11.0: a `gogo-analyst` agent + `analysis.md` procedure make planning a
specialist's job; the UAT gate makes shipping a human decision (`/gogo:done` is
the acceptance, issues loop back on the same item); `## Custom` sections are
untouchable; and the CLI deletes recoverably, peeks read-only, and launches
quietly. Full audit trail:
[.gogo/work/feature-analyst-uat-and-cli-ops/](../../work/feature-analyst-uat-and-cli-ops/)
