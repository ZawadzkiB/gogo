# Review round 1 — cockpit-redesign (fresh-eyes ③)

Feature: restyle the terminal cockpit TUI (`cli/internal/tui/`) into the
Claude-Design **1b + 1c** mockup. Presentation-only, over the same
`contract.Repo`; no contract change, no new pipeline state. Version → 0.18.0.

Reviewed against `plan.md` (FR-1..FR-10, D1/D2/D3, the Tests section),
`design/design-spec.md`, `code-review-standards.md`, `coding-rules.md`,
`non-functional-requirements.md`.

## Gates (run from the `cli/` module)
- `gofmt -l cli/` — clean.
- `go vet ./...` — clean.
- `go test -race ./...` — green (all packages, incl. tui).

## What I verified as correct
- **Shared phase model (FR-4/FR-9, D2).** `phaseProgress` never panics: an
  unknown phase/status → all-pending; `phaseIndex("done")==5` and
  `Class==ClassShipped` → all-done via the `idx >= len(out)` guard. The gate/
  terminal cases (awaiting-plan-acceptance→[c,p,p,p,p], awaiting-uat/knowledge→
  [d,d,d,d,c], shipped→all-done, implementing-via-status→[d,c,p,p,p]) all map as
  pinned in `TestPhaseProgressVector`. Dots and the segmented bar are two thin
  renderers over the one vector.
- **Pill color vs text agree (FR-3).** `pillStyleFor`'s precedence
  (WaitingForUser → session → AwaitingUAT → awaiting-plan-acceptance → phase-amber
  → dim) matches `badge()`'s order exactly, so the chip color can never disagree
  with `pillLabel`'s text (e.g. a live-session UAT card reads green "running" in
  both, not purple). Focused cards render the label plain + `phaseDotsPlain` so the
  full-card highlight doesn't punch per-glyph background holes.
- **Strip ↔ windowing coupling (D3).** No cycle: `stripDegraded()` reads
  `m.height` directly (never `colAvail`, which subtracts the strip). `colAvail()`
  clamps to ≥1 in every caller and `fitEnd` always shows ≥1 card, so tiny
  terminals degrade rather than panic/overflow; the `colAvail = height - 5 -
  stripHeight` accounting is conservative (reserves one extra row — under-fills,
  never overflows). `cardHeights` unit-rows the collapsed changelog; the
  degradation threshold `3 + 4·gates + minBoard` tracks the strip's true rendered
  height. Confirmed against the repointed window integration tests (plan column,
  height 20 → colAvail 8) and `TestChangelogOverflowBrowseHint` (height 9 →
  degraded 2-row strip → colAvail 2).
- **Coding-rules invariants.** CLI stays a deterministic, LLM-free reader — the new
  number keys only focus a card + open a file viewer (`quickView`); `?` toggles a
  bool; nothing mutates pipeline state. New elements are plain-text/substring
  assertable under `go test` (no TTY). Version bumped in **both**
  `.claude-plugin/plugin.json` and `cli/main.go` → 0.18.0. `main.go` diff is the
  version line only — dispatch/`printHelp` untouched, so the four-source CLI-enum
  sync is not triggered (number keys/`?` are board handlers, not CLI verbs).
- **Test fidelity.** The updated existing tests still assert real rendered
  behaviour (they were repointed, not weakened: card-windowing moved from the
  now-collapsed changelog column to the plan column — the windowing logic is
  column-agnostic — and gained changelog-collapse coverage). `redesign_test.go`
  pins FR-1..FR-10 (phase vector, dots/bar plain text, pill labels, stripe glyph
  focus-independence, gates enumeration, strip render, header summary, footer
  chips, `?` toggle, number-key jump + out-of-range).

## Findings

| id | sev | pri | status | title |
|----|-----|-----|--------|-------|
| REV-001 | minor | P2 | new | Decision-gate strip advertises `[g] resume`, but `g` is not a board key |
| REV-002 | nit | P3 | new | Dead leftovers from the badge→pill migration (`uatStyle`, `colStyleSet.badge`) |
| REV-003 | nit | P3 | new | `gateNumberKey` parses only 1–9; strip advertises `[N]` for N that can exceed 9 |

### REV-001 (minor, AGENT-FIXABLE)
`gateFor()`'s default (waiting-for-user / decision) branch labels the answer key
`[g] resume`, rendered in the strip row. `updateBoard` has **no `g` case** (only
`updateViewer` uses `g`); the real resume key for an in-progress/waiting-for-user
card is `m` (`move.go`: in-progress → go/resume). So a decision-gate row tells the
user to press a dead key. The awaiting-uat (`[d] ship`) and plan (`[m] accept`)
rows are correct. The number-key jump still works, so only the secondary answer
label is wrong; this path has no fixture, so tests didn't catch it. **Fix:**
change the decision-gate `answer` to `[m] resume` (optionally add a waiting-for-user
fixture to assert the row).

### REV-002 (nit, AGENT-FIXABLE)
`uatStyle` (styles.go:107) and the `badge` field of `colStyleSet` (struct +
init()) are orphaned — their sole consumer was the removed
`badgeStyleFor(f, columnStyles[colIdx].badge)`. Neither is read anywhere in `cli/`
now (verified). Not a compile error (Go ignores unused package vars / fields), but
dead code the coding-rules "no dead code / minimal diff" bar asks to drop. `uatAccent`
stays (used by `pillPurple`/`stripeAccent`). **Fix:** delete both.

### REV-003 (nit, AGENT-FIXABLE / could wontfix)
`gateNumberKey` accepts a single digit 1..9, but the strip labels rows `[i+1]` and
the degraded summary says `press 1–N`, which can render `[10]+` when >9 cards are
WaitingForInput() — those gates are then unreachable by number key. Negligible
real-world frequency. **Fix (optional):** cap the numbered affordance to the first
9 gates, or accept as-is.

## Verdict

**APPROVE** — no open blockers or majors. The redesign is correct, deterministic,
substring-assertable, version-bumped, and matches the plan's FR-1..FR-10 and
D1/D2/D3. The three findings are minor/nit polish (all agent-fixable) and do not
block advancing to phase ④ test; REV-001 is the one worth fixing (a wrong,
user-facing key label on the decision-gate row).

---

## Resolution (orchestrator, in-context ② round 2 · 2026-07-12)

All three findings **fixed in-context** (no re-explore needed — the implement
context was kept warm), then gates re-run: `gofmt -l` clean · `go vet ./...` clean
· `go test -race ./...` green.

- **REV-001 fixed** — `gateFor()` decision-gate `answer` `[g] resume` → `[m] resume`
  (`m` = the board's go/resume key per `move.go`).
- **REV-002 fixed** — removed dead `uatStyle` + `colStyleSet.badge` (field + `init()`
  assignment).
- **REV-003 fixed** — `renderNeedsYouStrip` now only numbers gates 1..9 (`[N]` shown
  for `i<9`; 10th+ gate keeps its column, no dead key), and the degraded summary
  caps at `press 1–9`.

Verdict stands: **APPROVE → advance to ④ test.**
