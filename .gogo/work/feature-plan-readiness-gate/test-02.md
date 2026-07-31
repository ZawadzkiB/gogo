# Test round 02 — plan-readiness-gate (ships as 0.29.0)

**Track:** test (④) · **Round:** 2 · **Date:** 2026-07-30

Round 1 found one open finding, TEST-001 (a raw `templates/state.template.md` scaffold
rendered a bogus `⛓ ×3` correlation chip because `parseStateFile` was not multi-line
`<!-- ... -->`-aware). The developer fixed it (`contract.advanceComment`, a new
`inComment` flag carried across lines in `parseStateFile`). This round re-verifies the
fix **hands-on**, per `test-strategy.md`'s TUI rule, and sweeps round 1's six scenarios
for regressions since the touched file (`state.go`) is shared by all of Slice A.

## Environment / isolation

Same method as round 1: a fresh scratch root (`/tmp/gogo-e2e-r2-<pid>/`, deleted at the
end), one scratch source repo, no `GOGO_DATA_HOME`/`~/.gogo/` reuse from round 1 (built
fresh). **Host safety verified before AND after:** `tmux list-sessions` showed exactly
the two protected user sessions before any scratch session was created and again after
every scratch session was killed — neither touched. `gogo sweep` was not invoked this
round (no cap scenario was re-driven — see "Not re-run" below). Nothing written under
`~/.gogo/` or this repo's own `.gogo/work/`.

## 1. Baseline (re-verified independently)

| Check | Result |
|---|---|
| `gofmt -l .` | clean |
| `go vet ./...` | clean |
| `go test -race -count=1 ./...` | green, all 13 packages |
| hermetic `go test -count=1 ./...` (minimal `PATH`) | green |
| `gogo --version` | `gogo 0.29.0` |

Matches the coordinator's independently-verified baseline exactly.

## 2. TEST-001 — hands-on re-verification

Fixture `feature-noplan-probe`: the **shipped** `templates/state.template.md` copied
verbatim (`cp`, not hand-retyped) into `state.md`, no `plan.md` — the exact fixture the
bug was found on.

- **At 200 columns:** card reads `✎ authoring`, title falls back to the slug
  (`noplan-probe`), **no `⛓` chip anywhere on the card.**
- **At 400 columns — the exact width round 1 saw the three literal bogus ids at:**
  resized the live tmux pane to 400×50 and recaptured. Still **no `⛓` chip**. This is
  the direct, width-matched disproof of the original defect.
- **`m`:** `⚠ plan.md not written yet - noplan-probe is still being authored (no
  plan.md on disk yet); finish it with `gogo plan noplan-probe`` — byte-for-byte the
  same refusal as round 1.
- **`M` (force):** identical refusal, no confirm form opened — FR4a still holds.
- Footer key-chip: `[m] ✗ plan not written` — unchanged.

**Item 1 (scaffold-with-no-chip): PASS, hands-on, at both named widths.**

## 3. A genuine correlation line still renders correctly

Fixture `feature-genuine-corr`: a real, uncommented
`- **correlation:** [plan-7f3a, plan-9c2e]` line (plus a written `plan.md` so the card
is a clean accept-pending card, isolating the correlation check). Live board, both 200
and 400 columns:

`⏸ accept plan  ⛓ plan-7f3a ⛓ plan-9c2e`

Exactly two real chips, the correct ids — not three bogus ones, not zero. **The fix
did not trade a false-positive for a false-negative.**

**Item 2: PASS, hands-on.**

## 4. Regression sweep (items the coordinator flagged: state.go is shared by all of Slice A)

- **Authoring gate:** `noplan-probe`'s `✎ authoring` pill, slug-fallback title, `m`/`M`
  refusal text, and footer chip all reproduced identically to round 1 (above).
- **FR4a:** re-confirmed above — `M` still refuses a missing plan, no form opens.
- **FR8 (accepted-but-unwritten), headless:** built `feature-accepted-noplan`
  (`status: plan-accepted`, no `plan.md`). `gogo go accepted-noplan` →
  `gogo go: accepted-noplan is plan-accepted but its plan.md is not written (no plan.md
  on disk yet) - nothing to build. re-plan it with `gogo plan accepted-noplan`; ...`,
  exit code 1 — unchanged.
- **`state lags` cue:** rebuilt the round-1 arm-A fixture (`phase: implement` /
  `status: implementing`, newest event `phase-done` for `implement`) with a live
  `gogo-go-lag-arm-a` session. Rendered: `implement r2   ● developer  · state lags` —
  unchanged.
- Cap/Slice B mechanics (`cap.go`, `launch.go`) were **not** touched by this fix and
  were not re-driven live this round (out of scope for this fix's blast radius, and
  already covered by the untouched, still-green `cap_rule_test.go` /
  `internal/orchestrator` suite) — flagged here rather than silently assumed.

**No regressions found in any round-1 scenario re-checked.**

## 5. Edge cases (the "missing, never wrong" decisions)

Built one fixture per case and checked both `gogo status` (headless) and the live
board:

| Case | Fixture | Result |
|---|---|---|
| `<!--`/`-->` on one line (trailing same-line comment) | `feature-edge-sameline` | `phase`/`status`/`iterations` all parsed correctly (`implement`/`implementing`/`plan=0·implement=1·review=0·test=0`); card classified in-progress, badge `implement r1` — byte-for-byte the pre-existing single-line behaviour. |
| A stray `-->` with nothing open | `feature-edge-straycloser` | Ignored; every other key on the file parsed normally (`plan`/`awaiting-plan-acceptance`, iterations all `0`); card reads `✎ authoring` cleanly. |
| A closer-then-key on one line (`--> - **stage:** ...`) | `feature-edge-closerkey` | The **whole line** (including the key after the closer) was skipped — `stage` never appears, but `phase`/`status`/`iterations` on later lines parsed fine (comment state correctly reset to closed after that line). Confirms "skip the whole line the block started on" per the doc comment. |
| An unterminated `<!--` | `feature-edge-unterminated` | `phase: review` / `status: reviewing` (both **before** the opener) parsed correctly; `gogo status`'s ITERATIONS column read `-` (empty) for this card — `created`/`branch`/`iterations`/`resume`/`open-decision` (all **after** the opener) were swallowed for the rest of the file, exactly as documented. Not silent: the card is visibly missing data rather than showing a plausible-looking wrong value. |

**All four edge cases behave exactly as documented, verified hands-on rather than by
trusting the unit tests alone.**

## 6. Independent mutation check (not just trusting the fix_summary)

Mirrored the developer's own method: temporarily edited `state.go`'s skip to
`if commented && false { continue }` (`go build ./...` confirmed it compiles — a
scoreable mutation, not a `BUILD-FAIL`), then ran `TestShippedTemplateScaffoldParsesClean`:

```
Correlations = []string{"plan-XXXX]   plan id(s) this item belongs to", "e.g. [plan-7f3a", "plan-9c2e"},
want none - a commented-out legend example must not parse as a real field
```

The guard **fails exactly as claimed**, with the exact three bogus ids — confirming the
guard genuinely bites and this is not a vacuous pass. Restored `state.go` immediately
from a pre-mutation copy; `md5` before/after matched byte-for-byte
(`25ff3850983142710ba5811bf65d1231`), and the full contract-package regression suite was
re-confirmed green afterward. Also ran the three named guards directly
(`TestShippedTemplateScaffoldParsesClean`, `TestAdvanceCommentEdges` — all 11 cases,
`TestParseStateFileCommentBlocks` — all 6 cases) plus the render-layer
`TestTemplateScaffoldRendersNoCorrelationChip` in `internal/tui`: all green.

## 7. The template itself

Read the shipped `templates/state.template.md` directly (the same bytes `cp`'d into the
`noplan-probe` fixture, not a paraphrase). Confirmed:

- **Still contains the commented-out example** (`- **correlation:**` appears inside the
  legend), so the guard's own sanity check ("the fixture must still contain the hazard,
  or this test would pass because the hazard was removed rather than handled") holds —
  the fix genuinely handles the hazard rather than deleting it.
- **Well-formed comment nesting:** 8 `<!--` openers and 8 `-->` closers, alternating and
  paired (verified programmatically over the raw file, not by eye).
- **No literal `-->` inside the new warning note** (the developer's own first draft had
  one, which closed the legend early and reopened the defect — caught by the shipped
  test). The final wording ("Never write an HTML comment opener or closer inside this
  block…") carries no comment tokens itself.

## 8. `events.jsonl` chronology

Re-read `.gogo/work/feature-plan-readiness-gate/events.jsonl` directly. All 12 lines are
monotonically increasing by timestamp, including the previously-miscorded pair:
`...T07:26:29Z phase-done (round 3) → ...T10:30:00Z fix-round 4 → ...T11:18:48Z
phase-done (round 4)` — correctly chronological now. No out-of-order entries anywhere
in the stream.

## Issues found

None new. **TEST-001 → `verified`** in `test/issues.json` (round bumped to 2).

## Verdict — PASS

- Build/unit: green (all four gates, independently re-run).
- TEST-001: **verified hands-on**, at both the 200- and 400-column widths that
  originally exposed it, plus an independent mutation check that confirms the guard
  bites (not vacuous).
- The fix did not regress the genuine-correlation case (still renders correctly) or any
  of round 1's six scenarios re-swept (authoring gate, FR4a, FR8, `state lags`).
- All four documented edge cases (`same-line`, `stray closer`, `closer-then-key`,
  `unterminated`) behave exactly as specified, verified hands-on.
- `events.jsonl` is chronological.
- No hands-on check was blocked; nothing was skipped.

**`test/issues.json`: 0 open/new issues. Recommending phase ④ closed — advance to ⑤
report.**
