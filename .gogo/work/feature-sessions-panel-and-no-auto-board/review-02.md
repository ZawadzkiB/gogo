# Code review — round 2 (verify pass) · `sessions-panel-and-no-auto-board`

**Scope.** Verification of the 7 findings the fix round marked `fixed`, the scope check on
`REV-007` (deliberately left `open`), a fresh-eyes sweep of the fix-round diff only, and a
re-run of the gates. The fix round touched exactly 8 files - `.gitignore`, `docs/index.md`,
`skills/gogo-done/SKILL.md`, `skills/gogo-session-update/SKILL.md`,
`cli/internal/tui/{update.go, session_ops.go, sessions_panel_test.go, key_help_sync_test.go}`
- and nothing outside the fix set was disturbed (checked by mtime against the round boundary).

Mutation checks were run in a throwaway copy of the repo
(`scratchpad/mut/`); **no product file in the working tree was modified by this review.**

## Gates (re-run by the reviewer, not taken on trust)

| Gate | Result |
|---|---|
| `cd cli && gofmt -l .` | clean (no output) |
| `go vet ./...` | clean |
| `go test -race -count=1 ./...` | **green** — 13 packages, `internal/tui` 12.1s |

## Round-1 findings — verification

| id | severity | status | verdict |
|---|---|---|---|
| REV-001 | major | **verified** | `docs/index.md:83` now reads the in-chat wording; full re-grep of `docs/*.md` + README + `commands/` + `skills/` leaves only live-cockpit uses and `cli-contract.md:395-397`'s deliberate retired note |
| REV-002 | minor | **verified** | entry-writer intro names the three live callers (`<slug>`, `slug1+slug2`, No-slug multi-select) |
| REV-003 | minor | **verified** | guard + subtest; **mutation bites** |
| REV-004 | minor | **verified** | field-level assertion; **mutation bites** |
| REV-005 | minor | **verified** | Esc-abort subtest pins the `pendingKillSession` returnMode leg; **mutation bites** |
| REV-006 | minor | **verified** | step 5 matches `sanitizeLabel` exactly (incl. the past-24 dash rule) |
| REV-007 | minor | **open (⑤)** | correctly scoped — see below |
| REV-008 | nit | **verified** | floor 8 = the parsed key count; **mutation bites**; contradictory comment gone |

### Mutation evidence (each re-run in the sandbox, whole `internal/tui` package)

- **REV-003** — `if !m.hasDrivableFeature()` → `if false`:
  `TestSessionsPanelReassign/zero_drivable_items_refuses_with_a_named_reason` **FAILS**
  (`zero-target R left the panel (mode=3)`). The guard also shares its predicate with the
  picker's own filter (`session_ops.go:434-441` vs `:454-456`), so the refusal and the row
  set cannot drift apart, and its remedy (`K closes the session…`) is targeted, not a
  host-global command (standard #12).
- **REV-004** — `m.clampSessIdx()` no-opped in `Update`'s `case sessionsMsg`:
  `TestSessionsPanelCursorClamps` **FAILS** (`sessIdx = 2 … want 0`).
- **REV-005** — `|| m.pendingKillSession != ""` dropped from `cancelForm`'s **returnMode
  line only** (the clear + `formPreservesSelection` legs left intact):
  `TestSessionsPanelKill/esc-abort_returns_to_the_panel_and_kills_nothing` **FAILS**
  (`landed in mode=0`). The added Cancel-OPTION arm was probed too: it genuinely lands on the
  Cancel row (`status="cancelled"`, level OK), so huh's wrap assumption holds — but its
  assertions stop at the outcome, which is REV-010 below.
- **REV-008** — `j` alias removed from `updateSessions`' down case: `TestBoardKeyHelpInSync`
  **FAILS** (`parsed only 7 keys from updateSessions`).

### REV-007 — the `open` status is correctly scoped

The `.gitignore` half is done and reads correctly (`no vendored python ships today…`). A
tree-wide re-grep for `board.py` / `assets/kanban` / `__pycache__` shows the only remaining
live surfaces are `.gogo/knowledge/`: `testing-tools.md:37,45` · `test-strategy.md:49,63,72` ·
`coding-rules.md:41,46` (this finding's three) plus `tech-stack.md:23,74` ·
`project-knowledge.md:118,133,142` · `non-functional-requirements.md:20,61-63` (the plan's own
item-19 list). Nothing outside `.gogo/knowledge/` still points at the deleted file — so
deferring to the phase-⑤ reconcile is right, provided the ⑤ sweep uses the **union** of the
plan's item-19 list and this finding's three files. One more line for that pass:
`tech-stack.md` still reports "**559** test functions as of 0.32.0".

## New findings (round 2)

### REV-009 · minor · open — the sessions panel's status line is unpinned
`cli/internal/tui/view.go:244-246` is the panel's only path from `m.status` to the screen.
Deleting it leaves `go test -count=1 ./internal/tui/` **green** (mutation-verified), while the
user-visible effect is a panel that silently swallows every message its handlers write —
including the zero-drivable refusal this very round added. All panel tests assert
`m.status`/`m.statusLevel` only; the two `View()` assertions that exist check the *empty-list*
reason lines from a different branch. That is `code-review-standards.md` **#8** verbatim, on
the exact class that "shipped a silent no-op once (the drill-card status line)". The code is
correct today; nothing keeps it correct.
**AGENT-FIXABLE**, one line: add
`if !strings.Contains(m.View(), "no drivable work item") { … }` to the zero-drivable subtest —
it pins the wording *and* the render together.

### REV-010 · nit · open — the Cancel-option subtest asserts the outcome, not the reason
`sessions_panel_test.go:208-224` checks only `mode == modeSessions` and zero renames. Both also
hold on the *failure* path: drop the `sel == adoptCancel` sentinel from `finishReassignSession`
(`session_ops.go:489`) and the suite stays **green** (mutation-verified) while a deliberate
Cancel starts reporting the amber "that work item is no longer present". `code-review-standards.md`
**#11(c)** asks for the exact reason.
**AGENT-FIXABLE**, one line: `if m.status != "cancelled" { … }`.

### REV-011 · minor · open — the session-update collision snippet still mints from the raw slug
`skills/gogo-session-update/SKILL.md:89` — `new="gogo-${action}-${slug}"` — re-derives the name
from the uncapped slug, three lines under the prose the fix round added that (correctly) caps
the **sanitized label** at `launch.MaxSessionLabel`. For a >48-char slug the operative snippet
and `launch.sessionName` mint different names for the same item: the FR10 "three doors agree"
property the parent finding exists to protect. Narrow (no slug in this repo exceeds 37 chars,
and `SessionMatchesSlug` accepts both candidates so attribution survives), hence minor per the
severity guide's "an example that no longer matches".
**AGENT-FIXABLE**, one line: have the snippet consume step 5's computed label
(`new="gogo-${action}-${label}"`).

## Also checked, no finding

- **Plan fidelity** — the fix round added no unplanned surface; `hasDrivableFeature` is a
  shared predicate, not a second copy of the terminal rule.
- **Test isolation** — the zero-drivable subtest mutates `f.Status` on fixture features, but
  `newModel` → `New(fixtureRoot)` → `contract.LoadRepo` re-loads fresh pointers per model, so
  nothing leaks into sibling tests.
- **Cancel-row navigation** — the new subtest's "huh's Select wraps" assumption is real
  (probed), and it fails loudly if wrap ever changes (a feature row would rename).
- **Refusal remedies** — the new refusal points at the panel's own targeted `K`, never a
  host-global `gogo sweep` (standard #12).
- **Diagnosability / write-scope / version** — refusals name their unblock; no write outside
  `.gogo/`; `plugin.json` + `version_test.go` at `0.33.0`.

**Verdict: APPROVE** — no open blockers or majors (REV-001 verified fixed); advance to ④ test, carrying REV-007 into the phase-⑤ reconcile and the three one-line follow-ups REV-009/010/011.
