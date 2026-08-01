# Review — round 01 — `sessions-panel-and-no-auto-board`

Phase ③ · fresh-eyes review against `plan.md` (accepted 2026-08-01, revised round 1),
`code-review-standards.md`, `coding-rules.md`, `non-functional-requirements.md`.
Machine contract: `review/issues.json`.

**Scope reviewed.** Only this feature's delta. The working tree also carries the
already-shipped 0.32.0 (session-binding-ops), so `git diff` was read against the plan's
named file set: the two new markdown surfaces (`skills/gogo-session-update/`,
`commands/session-update.md`), the `gogo-done` surgery + its ~8 doc surfaces, the
`board.py` deletion + `TestNoInteractiveBoardInSkills`, and the `modeSessions` Go slice
(`model.go` · `session_ops.go` · `update.go` · `view.go` · `main.go` ·
`key_help_sync_test.go` · `sessions_panel_test.go` · version mirror).

## Gates (verified, not taken on trust)

| Gate | Result |
|---|---|
| `cd cli && gofmt -l .` | clean |
| `go vet ./...` | clean |
| `go test -race -count=1 ./...` | green — all 13 packages |
| Version mirror | `.claude-plugin/plugin.json` + `cli/main.go Version` + `cli/version_test.go` all `0.33.0` (FR7 / D4=A) |

## What holds up

- **The five-place `pending*` contract is complete** for both new fields
  (`pendingReassign`, `pendingKillSession`): field decl, `StateCompleted` dispatch,
  `cancelForm` `returnMode`, `cancelForm` clear, `formPreservesSelection`. The plan's
  named trap did not bite.
- **`pickerOrigin` vs `sessionsOrigin` is correct.** The panel remembers its own opener
  (`sessionsOrigin`, board vs drill) while the R/K forms record `pickerOrigin =
  modeSessions`, so Cancel/Esc land in the panel and `esc`/`q` from the panel land where
  `S` was pressed. Both directions are asserted through `Update`.
- **One producer, two doors.** `reassign(session, target)` carries the refusal chain +
  derived-action rename + status message; `finishAdopt` is now a thin resolver in front
  of it and its 0.32.0 behaviour is unchanged (`TestAdoptRenamesToTheDerivedAction`
  still pins the exact `gogo-go-inprogress` / `gogo-plan-malformed` bases). The panel's
  already-bound refusal is asserted *through* the shared core, so the wiring is pinned,
  not just the producer.
- **Every form goes through `newForm()`**; targets sit behind the heap-stable
  `*formBinding` (TEST-001). Panel `K` is Cancel-default (`confirm: false`) and the
  bare-Enter-is-safe arm is asserted; panel `R` uses choice-is-confirmation.
- **Exact tmux targets** throughout — the panel kills via the `m.killer` seam
  (`KillSession` → `-t "=<name>"`) and renames via `m.renamer`
  (`RenameSessionUnique`); no new `exec.Command` and no discarded stderr.
- **No pipeline state is written** by any of the three doors — the rename/kill is the
  whole move (0.32.0 D3=A), and the render path reads only cached `m.sessionMeta`
  (FR3.2/FR3.5).
- **Guards that bite (mutation-verified by me, in a scratch copy):**
  `TestNoInteractiveBoardInSkills` fails when `board.py` is re-mentioned in a skill;
  `TestBoardKeyHelpInSync`'s new `updateSessions` case fails when `R` is dropped from
  `sessionsKeysLine`; `cancelForm`'s `pendingReassign` leg fails
  `TestSessionsPanelReassign/cancel_returns_to_the_panel` when deleted.
- **Enumeration sync (mostly).** `board.py` / `board-intent` / `resources/kanban` are
  gone from every skill and command; `/gogo:session-update` is present in `README.md`,
  `docs/commands.md`, `skills/gogo/SKILL.md` (as an ops command), `skills/gogo-cli/SKILL.md`,
  `commands/`, `skills/`. `S` appears in `boardAllKeysLine`, `drillKeysLine`, both
  `cli/main.go` blocks, the new `sessions panel keys` block, README §The gogo CLI, and
  `docs/cli-contract.md`'s 0.33.0 note. `TestCLICommandEnumerationInSync` is untouched
  and green.
- **The two markdown skills read as product.** `gogo-session-update`'s bash is sound:
  the has-session collision loop suffixes `-2`/`-3` correctly and is guarded against
  self-collision (`[ "$new" != "$old" ]`), the `=<old>` target / bare-NAME destination
  split is right, `$TMUX`-unset degrades rather than errors, `bindAction`'s rule is
  cited-with-source rather than forked, and D5=A's enrolment disclosure is present.
  `gogo-done`'s No-slug mode is complete and self-consistent — no exit codes, no intent
  schema, no relaunch loop survive.

## Findings

| id | sev | pri | status | title | fix |
|---|---|---|---|---|---|
| REV-001 | **major** | P1 | new | `docs/index.md:83` still says bare `/gogo:done` "opens the work board" | AGENT-FIXABLE |
| REV-002 | minor | P2 | new | `gogo-done` SKILL.md:123 still says "the board" calls the entry-writer | AGENT-FIXABLE |
| REV-003 | minor | P2 | new | Panel `R` with zero drivable items opens a Cancel-only picker, no named refusal | AGENT-FIXABLE |
| REV-004 | minor | P2 | new | `TestSessionsPanelCursorClamps` does not bite (mutation-proven) | AGENT-FIXABLE |
| REV-005 | minor | P2 | new | `pendingKillSession`'s `cancelForm` `returnMode` leg is unpinned (mutation-proven) | AGENT-FIXABLE |
| REV-006 | minor | P2 | new | "cap at 48 chars" is ambiguous — a whole-name reading breaks FR10 attribution | AGENT-FIXABLE |
| REV-007 | minor | P2 | new | The ⑤ reconcile list misses 4 surfaces still pointing at the deleted `board.py` | AGENT-FIXABLE (at ⑤) |
| REV-008 | nit | P3 | new | `updateSessions` anti-vacuity floor is 4 against 8 real keys; comments contradict | AGENT-FIXABLE |

No finding needs a user decision.

### REV-001 — `docs/index.md` still describes the retired board · **major** · P1 · new

`docs/index.md:83`, the quick-reference command table, still reads *"Ship one
report-complete feature, or several as ONE merged release entry **(no slug opens the
work board)**"*. Every other surface was synced; index.md is the one straggler, and it
is the first page a reader lands on.

This is the surface `code-review-standards.md` check #1 names verbatim: *"A doc-sync
sweep must enumerate **all** of `docs/*.md` — including the `docs/index.md`
quick-reference table — never just the plan's hand-listed subset (the surface REV-001
caught slipping through in 0.8.0)."* The plan's checklist item 7 hand-listed only
`docs/commands.md` / `docs/flow.md` / `docs/architecture.md`, and the build followed the
list rather than the standard. The new `TestNoInteractiveBoardInSkills` guard cannot
catch it — it scans `skills/*/SKILL.md` + `commands/*.md` only, and the stale sentence
contains none of its three banned tokens.

**Fix (AGENT-FIXABLE).** Replace the parenthetical with the in-chat wording already used
in `docs/flow.md`, then re-grep `docs/*.md` for `work board` / `no slug` to confirm
nothing else is stale (`docs/cli-contract.md`'s 0.33.0 note mentions the retired scratch
deliberately and is correct).

### REV-002 — dangling "the board" in the entry-writer intro · minor · P2 · new

`skills/gogo-done/SKILL.md:123`: *"This is the one place shipping happens. `<slug>`,
`slug1+slug2`, and **the board** all call it…"* — "the board" was the done-board, the
only third caller that sentence ever had. FR2.1 required the skill to be self-consistent
after the deletion.

**Fix (AGENT-FIXABLE).** Name the live third caller: "…and No-slug mode's multi-select
all call it". Lines 132/253/287/290 use "board" for the `gogo` cockpit and stay.

### REV-003 — panel `R` with nothing drivable is a silent dead end · minor · P2 · new

`cli/internal/tui/session_ops.go:437-462` filters out every `TerminalStatus` feature. On
a board where nothing is drivable — a mature repo where everything shipped, and the
panel deliberately lists other repos' sessions too — the option slice collapses to the
Cancel sentinel. Driven through the model (all fixture features forced `shipped`):

```
┃ Re-assign gogo-go-inprogress onto which work item?
┃ renames the session — every reader (dot, cues, cap, lock, sweep) follows the n
┃ > Cancel
after Enter: mode=modeSessions status="cancelled" renames=[]
```

FR4.2's own words are *"a refusal explains, a missing row puzzles (Diagnosability)"*,
and the NFR bar requires a refusal to carry its unblock. The card-anchored twin
(`adoptFeature`) already refuses up front for its analogous empty cases.

**Fix (AGENT-FIXABLE).** Count drivable features in `updateSessions`' `case "R"` and
`m.statusBlocked("no drivable work item to re-assign onto — every card here is
shipped/aborted; K closes the session, or plan a new item first")` at zero. Add the
matching subtest.

### REV-004 — the cursor-clamp test asserts through a read-time clamp · minor · P2 · new

`sessions_panel_test.go:244-263` claims FR3.4 coverage but asserts via
`m.focusedSession()`, which clamps on read itself. **Mutation-verified:** deleting
`m.clampSessIdx()` from `Update`'s `case sessionsMsg` leaves the whole `internal/tui`
package green. The production clamp is not dead — without it the first `up` after a
shrink is swallowed, because `clamp(sessIdx-1, …)` starts from the stale index.
`code-review-standards.md` #11(c).

**Fix (AGENT-FIXABLE).** Assert the field the handler writes: `if m.sessIdx != 0 { … }`
after the shrinking tick (and re-run the mutation to confirm it now fails).

### REV-005 — the panel-K Esc path is wired but unguarded · minor · P2 · new

The plan calls out the five-place contract and warns *"Miss the third and Esc bounces to
the board instead of back to the panel"*. All five places are correct — but only
`pendingReassign` is guarded. **Mutation-verified:** deleting
`|| m.pendingKillSession != ""` from `update.go:633` leaves `go test -count=1
./internal/tui/` fully green, while Esc-aborting the panel's `K` confirm would then land
on the board. The panel-K test only exercises the bare-Enter *completion* path
(`finishKillSession`), never `cancelForm`. `code-review-standards.md` #11(b) + #12
("pin the wirings, not just the producer").

**Fix (AGENT-FIXABLE).** Mirror the reassign subtest: `S` → `K` → `tea.KeyEsc` → assert
`modeSessions` + zero kills. Add the Cancel-*option* arm of panel `R` while there, so
both doors the plan names ("pickerOrigin on Cancel **and** Esc") are covered.

### REV-006 — "cap at 48 chars" can fork the name producer · minor · P2 · new

`skills/gogo-session-update/SKILL.md:77-80` states the transform as *"…trim `-`, cap at
48 chars"*. The Go producer caps the **label**, not the name
(`launch.MaxSessionLabel = 48`, applied by `sanitizeLabel`), and cuts on the last `-`
only when that boundary is past 24, else takes a hard cut. Read as a whole-name cap, any
slug ≥ 39 chars (`gogo-plan-` is 10) yields a truncated slug component — which matches
**neither** candidate base `SessionMatchesSlug` tries (it keeps the capped and the
*uncapped* label, not an arbitrary truncation). The renamed session then binds to
nothing: unbound on the board, uncounted by the cap. That is degrading to WRONG, which
the NFRs forbid, and it is exactly what FR10 exists to prevent. Not far-fetched: this
repo's longest live slug is `plans-tab-launch-diagnostics-and-view` (37), two under the
threshold.

**Fix (AGENT-FIXABLE).** Pin the cap to the label and name its source in step 5:
"cap the SANITIZED SLUG (not the whole name) at 48 — `launch.MaxSessionLabel`; when the
cut lands past char 24, cut back to the last `-` and trim it, else keep the hard cut."

### REV-007 — the ⑤ reconcile list is incomplete · minor · P2 · new

`board.py` is deleted, but four live surfaces still describe it as present and only
three are on the plan's item-19 list. Missing:

- `.gogo/knowledge/testing-tools.md:37` — "run `python3 assets/kanban/board.py --selftest`"
- `.gogo/knowledge/test-strategy.md:49,63` — the same, plus a `tmux new-session … board.py --index` recipe
- `.gogo/knowledge/coding-rules.md:41` — the "Vendored executable assets" rule whose only example is `board.py`
- `.gitignore:22-25` — the "assets/kanban/board.py is vendored source" comment + the now-dead `__pycache__/` / `*.pyc` entries

Knowledge files are always-read context for every pipeline worker, so an instruction to
run a deleted file is worse than ordinary doc drift. Raised against ⑤ (not implement)
because the plan deliberately deferred knowledge reconcile — the defect is that the ⑤
list is short.

**Fix (AGENT-FIXABLE, at ⑤).** Extend the reconcile set to those four files; decide in
one line whether the vendored-executable rule survives without a live example.

### REV-008 — the new anti-vacuity floor is half the real key count · nit · P3 · new

`key_help_sync_test.go:109` sets `floor: 4` for `updateSessions` with a comment counting
key *groups*, while the block comment two lines down says "only ~8". The parser counts
keys: `esc q up k down j R K` = 8. FR6.2 asked for a per-case floor precisely so a small
switch gets a real one; at 4, half the panel's keys could vanish undetected.

**Fix (AGENT-FIXABLE).** `floor: 8`, and fix the contradictory comments.

## Plan fidelity

FR1–FR10 are all implemented; nothing unplanned crept in. The only plan-side gaps are
REV-001 (checklist item 7's doc subset was narrower than the standard requires) and
REV-007 (item 19's reconcile subset). D1–D5 are all honoured, including D5=A's
disclosure line and D2=C's board-**and**-drill `S`.

---

**Verdict: CHANGES** — 1 open major (REV-001), 6 minors, 1 nit; no blockers, no
user decision needed.
