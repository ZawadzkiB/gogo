# Review round 01 — `session-binding-ops`

**Scope:** the uncommitted working tree (no commits). 13 modified files + 4 new
(`cli/internal/tui/session_ops.go`, `session_ops_test.go`, `key_help_sync_test.go`,
`cli/internal/launch/session_binding_test.go`), 327 insertions / 89 deletions.
**Reviewed against:** `plan.md` (accepted 2026-08-01) · `code-review-standards.md` ·
`coding-rules.md` · `non-functional-requirements.md` · `test-strategy.md`.

**Gates (run by the reviewer, not taken on trust):**
`cd cli && gofmt -l .` clean · `go vet ./...` clean · `go test -race ./...` green (13 pkgs).

---

## What holds up

Verified rather than assumed, because most of this feature's risk is in traps the repo
has already been burned by:

- **Exact tmux targets (FR1.5 / TEST-005).** `RenameSessionArgs` is
  `{"rename-session", "-t", "=<old>", "<new>"}` — exact `=` on the SESSION target, bare
  NAME in the new-name position — and `TestRenameSessionArgs` pins both halves,
  including an explicit assertion that the new name carries no `=`. `RenameSession`
  goes through `runTmux`, so a failure carries tmux's own words in the typed
  `*TmuxError` (NFR Diagnosability / standard #10).
- **Attribution stays an exact convention parse.** Every new reader
  (`boundFeature`, the already-bound refusal, the `P` join-don't-duplicate branch)
  goes through `launch.SessionAction` / `SessionMatchesSlug`; `parseSessionMeta`
  filters on the same `gogo-` prefix `ListSessions` uses. No substring matching
  reappears.
- **huh construction + TEST-001.** Both new forms are built by `newForm(...)` (never a
  direct `huh.NewForm(`) and bind through the heap-stable `*formBinding`
  (`&m.binding.confirm` / `&m.binding.selected`), so the value-type Model copy is safe.
- **Confirm-default convention.** `P` is a forward move → `formBinding{confirm: true}`;
  the kill confirm keeps `confirm: false`. Correct on both halves.
- **The `pickerOrigin` fix is real and guarded.** Replacing `pickerFromDrill` with an
  origin mode recorded where each picker starts is applied to all four picker/confirm
  entry points, and `finishKill` no longer hard-codes `modeDrill`. The stale-drill
  cancel case has a test that reproduces the old bug's precondition.
- **No pipeline-state write, no host-global remedy.** The three ops only call tmux;
  nothing writes under `.gogo/`, no registry rewrite (D3=A), and no new user-visible
  message names a bare `gogo sweep` (TEST-007) — the refusals point at cockpit keys.
- **Enumeration-sync is complete.** `P`/`K`/`R` land in `cli/main.go`'s board *and*
  drill blocks, `boardAllKeysLine`, `drillKeysLine`, `README.md` (board + drill
  bullets), `skills/gogo-cli/SKILL.md` (Board keys + drill prose) and a
  "Changed in 0.32.0" note in `docs/cli-contract.md`. Version bumped to 0.32.0 in
  `plugin.json` **and** `cli/main.go`, mirrored by `TestVersionMirrorsPlugin`.
- **Guards that bite (mutation-checked, `go vet`-compiled first).** Removing
  `· P plan session ` from `boardAllKeysLine` → `TestBoardKeyHelpInSync` fails naming
  the key; deleting the "join the live plan session" branch → `TestPlanSessionRefusals`
  fails; disabling the foreign-anchor refusal → `TestAdoptRefusals/foreign_anchor`
  fails with the rename it should not have made.
- **Checked and cleared, so it is not on the list below:** the `renamer` seam's
  production wiring is unpinned by any test — but so are the pre-existing `killer`,
  `launcher` and `capturer` seams (control mutation: no-op'ing `killer: launch.KillSession`
  also leaves the suite green). That is a pre-existing repo-wide pattern, not something
  this change introduced.

---

## Findings

| id | severity | priority | status | fixability | title |
|---|---|---|---|---|---|
| REV-001 | **major** | P1 | new | AGENT-FIXABLE | `TestBoardKeyHelpInSync` passes vacuously when the AST scan finds no keys |
| REV-002 | minor | P2 | new | AGENT-FIXABLE | FR2's shipped/changelog-card `K` clause has no test that bites |
| REV-003 | minor | P2 | new | AGENT-FIXABLE | `[R] re-assign` chip on a shipped card advertises a key that always refuses |
| REV-004 | nit | P3 | new | AGENT-FIXABLE | `planFeature` re-derives `!TerminalStatus` instead of calling `PlannableStatus` |
| REV-005 | nit | P3 | new | AGENT-FIXABLE | `SessionMeta.Attached` is parsed and tested but read by no production code |

### REV-001 — `TestBoardKeyHelpInSync` passes vacuously (major, P1, AGENT-FIXABLE)

`cli/internal/tui/key_help_sync_test.go:103` loops over `switchKeys(t, "update.go", c.fn)`
and asserts nothing when that slice is empty. **Proven by mutation:** changing only the
test's own function name to `c.fn+"NoSuchFunc"` (compile-clean under `go vet`) leaves
`go test -run TestBoardKeyHelpInSync` reporting `ok`. Any future drift that empties the
parse — `updateBoard`/`updateDrill` renamed or split, the key switch stopping being
`switch msg.String()`, `update.go` renamed — silently converts the plan's
"a new key can never ship undocumented" guarantee into a green no-op.

The sibling guard whose parser this test explicitly reuses already has the floor
(`plans_view_test.go:980`: `if len(keys) < 8 { t.Fatalf("parsed only %d keys from %s (%v) - parser drift?") }`),
and `coding-rules.md:126-128` states the rule outright — "fails loudly on an empty scan
— a guard must never pass vacuously" (test-strategy variant 6 is the same shape).

**Fix:** add that floor inside the case loop (8 is safe: `updateBoard` handles 19 cases,
`updateDrill` 9), and additionally fail when `len(tuiDoc) == 0 || len(cliDoc) == 0`, so a
renamed `board keys:` header or an emptied help const cannot make the documented-token
set trivially permissive either.

### REV-002 — FR2's shipped-card `K` clause is unguarded (minor, P2, AGENT-FIXABLE)

FR2's distinguishing clause is *"including a focused changelog/shipped card, which is
where a lingering session shows up"* — the incident's own symptom. **Proven by
mutation:** inserting a `TerminalStatus` refusal at the top of `killFeature`
(`session_ops.go:245`), i.e. deleting exactly that clause, leaves the whole `cli` suite
green. The only shipped-card assertion is that the `[K] kill` **chip renders**, which is
one level away from the behaviour (code-review-standards #11(b)). The plan's Tests row
also asks for the board's `0/1/>=2` arms and `TestBoardKill` covers only `0` and `>=2`.

**Fix:** one subtest in `TestBoardKill` — focus a changelog card (`colIdx=3`),
`hasTmux = true`, injected killer, `m.sessions = ["gogo-done-<slug>"]`, press `K`, drive
the single-session confirm to Enter, assert the killed name **exactly** and
`m.mode == modeBoard`. That covers the shipped-card clause and the missing 1-session arm
at once.

### REV-003 — the shipped card's `[R] re-assign` chip always refuses (minor, P2, AGENT-FIXABLE)

`view.go:882` renders `[K] kill` **and** `[R] re-assign` when `f.Shipped() &&
hasLiveSession(...)`, but `adoptFeature` (`session_ops.go:279`) refuses immediately on
`orchestrator.TerminalStatus(f.Status)` — and `Shipped()` (shipped|done) is a strict
subset of TerminalStatus (shipped|aborted|done). The chip can therefore never do what it
says. The contradiction is enshrined by two adjacent new tests: `TestAdoptRefusals/"terminal
target"` drives `R` at `colIdx=3` and asserts the bounce, while
`TestSessionBindingFooterChips` asserts the chip renders on that same column.

The root is a tension inside the accepted plan (FR3 refuses a terminal target; FR4 offers
`[R] re-assign` on a shipped card). The implementation kept both and softened it with a
redirecting refusal string — which is decent diagnosability, but still a footer chip that
lies (standard #8's class).

**Fix (recommended A):** drop `[R] re-assign` from the shipped arm, keep `[K] kill`; the
"R belongs on the card the session should DRIVE" message is already carried by the
unbound-count status line and by the terminal refusal. Update the chip test to assert the
absence so it cannot creep back. (B) If FR4's letter must hold, render it as the pointer
it is rather than a bare `[R] re-assign`. Either way, record the FR3/FR4 tension in the
phase-⑤ report so plan text and behaviour agree.

### REV-004 — two doors, one rule, re-derived (nit, P3, AGENT-FIXABLE)

`planFeature` gates on `!orchestrator.TerminalStatus(...)`; the other door to the same
act, `gogo plan <slug>` (`cli/go.go:318`), gates on `orchestrator.PlannableStatus(...)`.
They agree only because `PlannableStatus` is literally `!TerminalStatus` today — the
exact drift shape `coding-rules.md` TEST-006 records. **Fix:** call
`orchestrator.PlannableStatus(f.Status)` in `planFeature`. Leave `adoptFeature`'s
`TerminalStatus` check alone — "nothing to drive" is a genuinely different rule.

### REV-005 — `SessionMeta.Attached` has no production reader (nit, P3, AGENT-FIXABLE)

`launch.go:969` adds the field, `parseSessionMeta` fills it, `session_binding_test.go`
asserts it three times — and `grep -rn Attached --include='*.go'` finds no production
reader (`adoptRow` renders name / bound / repo / age only). Plan-sanctioned (FR4 names
the field), so not a deviation, but as shipped it exists for a test. It is also the most
useful missing fact in the picker: a session with a live client is one a human may be
sitting in right now. **Fix:** render `· attached` (word, not colour) in `adoptRow` and
assert it once — or drop the field and its `#{session_attached}` token rather than leave
unread ballast.

---

## Verdict

**CHANGES** — 1 major (REV-001), 2 minor, 2 nits; 0 blockers, 0 needing a user decision.
The feature itself is sound: the binding really is one string, every reader re-derives it,
the tmux contracts are exact and pinned, and the enumeration sweep is complete. What is
open is guard quality (REV-001, REV-002) plus one chip that promises what its card cannot
do (REV-003) — all agent-fixable in a single implement round, after which re-review the
same living list.
