# Review round 01 — cockpit-cards-and-cli-awareness

- **Phase:** ③ review (fresh-eyes)
- **Date:** 2026-07-12
- **Scope:** working-tree diff (uncommitted) — Slice A (plugin CLI-awareness) + Slice B (rich board drill-in card), version 0.15.0 → 0.16.0
- **Gates (run from `cli/`):** `gofmt -l .` clean · `go vet ./...` clean · `go test -race ./...` green (incl. the 7 new tests)

## Verdict: APPROVE

No open blockers or majors. Two low-severity findings (1 minor, 1 nit) recorded
for the audit trail; both are agent-fixable and neither blocks advancing to ④ test.

## What was verified (held against plan.md FR-A1..A6 / FR-B1..B6 + coding-rules invariants)

**Hard invariants — all upheld:**
- **No LLM in the read path.** `loadDrillCard`/`sessionRows`/`eventsTail` are pure
  deterministic reads (`orchestrator.LoadRegistry`, `launch.ListSessions`,
  `contract.ReadEvents`); opening/refreshing the drill performs zero launches and
  no registry write (FR-B5, verified by `TestDrillDegradesNoSessions`).
- **Kill/attach touch sessions only, never pipeline state.** `K` → confirm →
  `m.killer` (default `launch.KillSession`); `a` → `attachFeature` → `tea.ExecProcess`.
  No `state.md`/contract write anywhere in the path.
- **Injection safety.** `KillSession` and `AttachArgs` are single-argv `exec.Command`
  ("tmux", "kill-session"/"attach-session", "-t", name) — no shell, slug never reaches one.
- **Exact session attribution (TEST-005).** `sessionRows`/`liveSessionsFor` gate on
  `launch.SessionMatchesSlug` (convention parse + numeric collision suffix), never
  substring. Table test asserts `oauth ≠ auth`, `awaiting-card ≠ waiting-card`.
- **Value-type Model gotcha (TEST-001).** The kill confirm binds
  `.Value(&m.binding.confirm)` behind the heap-stable `*formBinding`, exactly like
  the delete/ship forms; `updateForm` forwards every `tea.Msg` to the child form.
- **Enumeration-sync + version.** `TestCLICommandEnumerationInSync` derives verbs
  from `main.go`'s dispatch and asserts each in README / cli-contract / gogo-cli
  SKILL. `plugin.json` and `main.go` `Version` both → 0.16.0. Help + README keymap
  document the new `a`/`K` drill keys.
- **Import direction.** `tui → orchestrator` is acyclic (as the plan predicted).

**FR coverage:** FR-A1..A6 met (canonical `skills/gogo-cli/SKILL.md`, conditional
"separate curl install / if on PATH" framing, lean `**Load when:**` pointer in
`skills/gogo/SKILL.md`, when-to-use guidance, sync-lint, "active half extends this
file" note). FR-B1..B6 met (detail panel description/folder/status-with-phase+round;
registry ⨯ live-tmux session rows with live/stale + untracked-live racer; `a`/`K`
behind a confirm; inline events tail + full timeline still openable; pure
table-tested `sessionRows` with graceful degrade; additive cli-contract; versioned).
Tests are meaningful (real substring/behaviour assertions, exact-match guard,
fire-exactly-once kill, cancel-never-kills), not tautological.

## Findings

### REV-001 — minor (P2) — status: new
**Cancelling the drill `K` kill-confirm with Esc/abort drops the user back on the
board, not the drill card.** `finishKill` deliberately returns to `modeDrill` (and
refreshes the panel), but the Esc branch and `huh.StateAborted` both call
`cancelForm`, which unconditionally sets `m.mode = modeBoard`. So the "Cancel"
button returns to the drill while Esc/abort bounces to the board — an inconsistency
with the author's own intent; the user loses the card they were inspecting. Session-
only, non-destructive path, so UX-only (no correctness/safety impact). Not caught by
a test (`TestDrillKillWiring` asserts the killer isn't called, never the mode).
*Fix (agent-fixable):* when `m.pendingKill != nil`, have `cancelForm` return to
`modeDrill` (keep `m.drill`); add a mode assertion to the cancel subtests.

### REV-002 — nit (P3) — status: new
**Enumeration-sync lint never checks `cli/main.go` `printHelp`, one of the four
named sync sources.** `TestCLICommandEnumerationInSync` derives verbs from the
dispatch switch and greps README / cli-contract / gogo-cli SKILL, but never asserts
the `printHelp` text lists each verb, so the help block (the "runtime truth" the
plan calls out) could drift from the dispatch undetected. All four are in sync
today; the gap is only future drift detection of the fourth source. *Fix
(agent-fixable):* also grep the `printHelp` string for each `gogo <verb>` /
`--version`.

## Route

Clean round (no open/new blockers/majors) → advance to ④ **test**. The two minors/
nits are batched into the living `review/issues.json` for the audit trail and can be
folded into a later pass; they do not gate the phase.
