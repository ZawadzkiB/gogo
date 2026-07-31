# Review 01 - plan-readiness-gate (0.29.0)

Round **1** · reviewer: `gogo-reviewer` (fresh eyes) · 2026-07-30
Scope: 46 files, +1650/-224 (`git diff` + the 4 new untracked test files)
Contract: `review/issues.json` (this file is its rendered snapshot)

Gates re-run independently: `gofmt -l .` clean · `go vet ./...` clean ·
`go test ./...` green (12 packages) **on this host** - see REV-001 for the host it is
not green on. Version: `plugin.json` + `cli/main.go` both `0.29.0`, pinned by
`TestVersionMirrorsPlugin`.

## What I verified rather than took on trust

- **FR4a (the load-bearing invariant) holds, re-derived independently.** `planReadinessBounce`
  sits at the top of `attemptActionForce`'s `ClassUnfinished` arm (`move.go:85`), outside both
  `!force` guards. I enumerated every path that can reach an accept intent: `ActionAccept` is
  produced at exactly one site (`move.go:95`), behind the bounce; the selection branch can only
  hold ready-to-ship cards (`toggleSelect` refuses anything else, `update.go:303`), so it cannot
  carry an authoring card into a merged `/gogo:done`; `ship=true` requires `ClassReadyToShip`;
  `cmdGo`'s FR8 gate is above `capBlock` and unaffected by `--force`. Mutating the call site to
  `bounce != "" && !force` fails `TestForceCannotOverrideMissingPlan` in both subtests plus
  `TestAcceptedButUnwrittenPlanRefused`; deleting the block fails six tests.
- **The 43 new tests bite.** I ran a **20-mutation revert sweep** in a throwaway copy of the tree
  (never the repo), `go build ./...` first for every mutation and a positive check that the edit
  landed: **20/20 caught**, each in the expected test (M12, `PlanSectionsRequired 2 -> 1`, is
  caught by three sibling contract tests rather than the one I predicted - still caught). Covered:
  the FR4a call site (2 ways), the cap's action test, re-adding the `Class` filter,
  `unreadable -> unwritten`, the `WaitingForInput` authoring arm, `stripPlaceholder`, the footer
  chip, `sessionAgent` at the render site, `HasSessionAction`'s scan-every-session rule,
  `cmdGo`'s FR8 gate, the threshold constant, `pillStyleFor`/`badge`'s authoring arms, the stalled
  cue, the drill note, `status.go`'s LIVE marker, `loadFeature`'s derivation, `buildingDisagreement`
  and `runnableHint`. My first M4 attempt reported `EDIT-DID-NOT-LAND` from my own harness check
  (the replacement text contains the anchor) - re-run with a corrected verification it is CAUGHT
  by `TestCapCountsLiveBuildRegardlessOfClass`. **I found no other assertion of the M20 shape**
  (a comparison that is identical under TTY-less lipgloss): the pill test now compares
  `GetForeground()`/`GetBackground()`/`GetBold()` properties, and every other new cue assertion
  goes through `View()` substring matching on glyph+word, which is legible without colour.
- **`planWritten`'s asymmetry is sound.** The three states are genuinely distinguishable via
  `PlanSections`' `(n, err)`: `nil` = counted, `fs.ErrNotExist` = absent, anything else =
  unreadable. A permission error (`EACCES`) and an I/O error (`EISDIR`, `bufio.ErrTooLong`) both
  land in `default: return true`, so they can never invent a defect - and `syscall.Errno.Is` never
  maps a real file's error to `fs.ErrInvalid`, so the only producer of the "unwritten" branch
  besides a genuinely absent file is the explicit `dir == ""` guard (which exists to stop a
  `Dir`-less Feature from opening a relative `plan.md`, and is pinned by a test).
- **`stripPlaceholder` can only degrade.** It wipes a value that opens `<` and closes `>`; `<`
  alone, an empty value and `a < b > c` all survive (pinned). The worst realistic false positive
  is a title that both starts and ends with an angle bracket, and its effect is a card that falls
  back to its slug - never invented data. Placeholder `created:` now sorts last instead of first
  (`TestTemplatePlaceholdersNeverRender` asserts the ordering, not just the field).
- **The cap's safety property.** A live `gogo-go-<slug>` counts regardless of class (the
  pre-first-write window is the fixture); `gogo-plan-*` and the other four actions do not; the
  target slug is still excluded so a resume never blocks itself; the cross-repo **over**-count is
  untouched and is still documented as the opposite direction, not conflated with this fix.
  One uncovered consequence of dropping the filter: REV-008.
- **Single-clause diagnosis (FR6-of-the-brief).** `contract.PlanUnwrittenReason` is quoted by the
  board bounce, `cmdGo`'s refusal, `runnableHint` and the drill note (which reuses
  `planReadinessBounce` verbatim - `TestQuickViewNamesTheMissingPlan` asserts the two strings are
  equal, so they cannot drift). The footer chip is a space-bounded label (`[m] x plan not
  written`), not a second diagnosis - consistent, so I did not raise it.
- **Write scope + portability.** No `os.WriteFile`/`MkdirAll`/`exec.Command` added outside tests;
  every CLI change is read/display-side. `cmdStatus` now calls `ListSessions()`, and an empty
  session set renders the pre-0.29.0 table byte-for-byte (pinned), so tmux stays a soft dep. No
  em dashes in any added line.
- **The `coding-rules.md` deferral to phase (5) is correct.** The plan's own checklist assigns that
  write to (5) ("Phase (5) owns this write"), knowledge files are reconciled by the report phase,
  and the carry is recorded in both `plan.md`'s build-note and `state.md`'s `resume:` line - so it
  cannot be silently dropped.

## Findings

| id | severity | pri | status | title |
|---|---|---|---|---|
| REV-001 | **blocker** | P0 | new | Three new `cli` tests need `claude` on PATH - they fail on a clean machine and break the release CI gate |
| REV-002 | **major** | P1 | new | FR12a incomplete: three live copies of the OLD cap rule remain (`--help`, config form, source detail) |
| REV-003 | **major** | P1 | new | FR8's refusal is missing on the reload AUTO-PICKUP path - the one launch path with no human in the loop |
| REV-004 | minor | P2 | new | `cap_test.go` comments still document the deleted `Class` filter as if it were under test |
| REV-005 | minor | P2 | new | The drill note is asserted on `Model.status` only, not in `viewDrill()` output (standard #8) |
| REV-006 | minor | P2 | new | FR11's entry write was skipped on its first live run - this round is reviewing with `status: implementing` |
| REV-007 | minor | P3 | new | The render path opens `plan.md` every frame just to test a boolean |
| REV-008 | minor | P3 | new | Dropping the `Class` filter also lets a terminal feature consume a cap slot - untested, undocumented |

### REV-001 - blocker · P0 · new

**Three new `cli` tests depend on the developer's `claude` binary.**
`TestCmdGoRefusesAcceptedButUnwrittenPlan` (both subtests) and
`TestCmdGoAuthoringHintReachesTheUser` call `cmdGo` without stubbing `claude`, and `cmdGo`
checks `launch.HasClaude()` (`cli/go.go:130`) *before* it loads the repo. Reproduced:

```
$ cd cli && env PATH="$(dirname $(which go)):/usr/bin:/bin" go test ./...
--- FAIL: TestCmdGoRefusesAcceptedButUnwrittenPlan/no_plan.md_at_all
    stderr missing "demo is plan-accepted but its plan.md is not written"; got:
    gogo go: claude CLI not on PATH - the persistent session runs `claude -p`
--- FAIL: TestCmdGoRefusesAcceptedButUnwrittenPlan/a_one-section_stub
--- FAIL: TestCmdGoAuthoringHintReachesTheUser
FAIL	github.com/ZawadzkiB/gogo/cli
(every other package: ok)
```

Nothing else in the suite fails, so this is a **new** hermeticity regression.
`.github/workflows/release.yml` runs `go test -race ./...` on `ubuntu-latest` as *"the one gate:
the CLI suite must be green before anything is published"* - claude is not installed there, so
tagging `v0.29.0` fails the gate and ships no binaries. It also breaks `coding-rules.md`'s
non-negotiable `go test -race ./...` for any other contributor.

**Fix (AGENT-FIXABLE).** Use the package's own hermetic seam, as `go_cap_test.go` does:
`binDir := writeStubClaude(t)` + prepend it to `PATH`. Re-verify with the claude-less `PATH`
command above. Leave `TestCmdGoRunsAWrittenPlan` alone - its empty `PATH` is the deliberate
"reached the next guard" proof.

### REV-002 - major · P1 · new

**FR12a moved the rule but not all of its copy.** Still saying the pre-0.29.0 rule, in live
user-visible text:

- `cli/main.go:204` (`gogo --help`) - *"N caps THAT source's **in-progress** work items that have
  a live session."*
- `cli/internal/tui/config_tab.go:117` - the source-edit form description shown **while the user
  sets the cap**.
- `cli/internal/tui/config_tab.go:479` (`capScopeNote`) - the source-detail line
  *"(this source only . counts in-progress work items with a live session . plans never count)"*.

0.28.0 deliberately wrote the rule in four places (bounce · config form · source detail ·
`--help`); only the bounce moved, and all three survivors are now false (a mid-build item is
`ClassUnfinished` and *is* counted). FR12a's own words: *"or the most legible message in the
cockpit becomes the most wrong"*. Review standard #1 as well.
The two remaining hits outside the CLI (`docs/cli-contract.md:88`,
`.gogo/knowledge/project-knowledge.md:402`) are dated 0.28.0 release history and are fine to
leave.

### REV-003 - major · P1 · new

**The unattended launch path is not gated.** `autoPickupReady` (`cli/internal/tui/pickup.go:33`)
gates on correlation + `plan-accepted` + skip-source + no live session, and never reads
`f.PlanUnwritten`. Probe (throwaway copy of the tree, fake launcher):

```
autoPickupReady=true cmds=1
UNATTENDED launch fired at a plan-accepted feature with NO plan.md:
  "/gogo:go web-token --skip-acceptance" (action=go)
```

The same card refuses a human (`m` bounces, the footer reads `[m] x plan not written`), so the
asymmetry is precisely backwards: the path with nobody watching is the unguarded one. This is the
case the plan's Context names as the aggravator, and it is reachable by exactly the failure mode
the feature exists to fix (state.md written before plan.md, plan gate auto-accepted by the
source's skip flag). What stands between it and a build is `gogo-implement` §① prose - prose
guarding prose. **Fix:** `if f.PlanUnwritten { return false }` in `autoPickupReady` + a
pickup_test.go assertion that zero cmds fire (and that a written-plan member still fires once).

### REV-004 - minor · P2 · new

`cli/internal/orchestrator/cap_test.go` still says `// wrong class -> not counted` (line 24) and
describes the count as "in-progress ∩ live" in two doc comments. The assertions pass, but they now
pass for a different reason (no attributed session), so the file documents a filter that no longer
exists - standard #11's "a comment claiming coverage its assertions do not provide". Reword, and
optionally give the `ready` fixture a `gogo-done-ready` session so the line proves the *action*
test instead.

### REV-005 - minor · P2 · new

`TestQuickViewNamesTheMissingPlan` asserts `Model.status` / `statusLevel` / `mode`, never
`View()`. Standard #8 names this exact shape for this exact surface (the drill-card status line
that shipped as a silent no-op once). I probed the rendered output and the note **does** render
today (`viewDrill` renders `renderStatus`, and both the sentence and `⚠ ` appear), so this is a
coverage gap, not a live defect - but it is the only new user-visible string in the change whose
render is unasserted. Add the `View()` assertions the board-mode sibling test already makes.

### REV-006 - minor · P2 · new

**FR11's writer half was skipped on its first live run.** While this review executed,
`state.md` read `phase: implement` / `status: implementing` and `events.jsonl`'s last line was
implement's `phase-done` - no review `phase-started`, i.e. `gogo-review` §①b (added by this very
diff) did not happen before delegation. Implement's own entry write **did** work (`phase-started`
23:22, `phase-done` 00:06 - 44 minutes apart), which is the FR11 win; but the reader-side guards
do not cover the review/test lag (the cap and the `● building` cue both key on the live
`gogo-go` session, which persists across all three phases, and D6=A deliberately keeps the
file-derived column). Suggest folding the entry write into the numbered `## ② Steps` list rather
than a sibling `①b` section, and recording the limitation in ⑤ so the release does not over-claim
for ③/④.

### REV-007 - minor · P3 · new

`footerChips` (`view.go:824`) decides the `[m]` chip with `planReadinessBounce(f) != ""`, which
opens and scans `plan.md` and formats a full sentence, on every `View()` of a focused
`ClassUnfinished` card - while `f.PlanUnwritten`, derived once per load, already answers it.
Extract a pure `planUnready(f)` predicate for the render/branch sites and keep the message
producer for the keystroke paths that actually display it.

### REV-008 - minor · P3 · new

With the class filter gone, a `shipped`/`done` feature still holding a live `gogo-go-<slug>`
session now consumes a cap slot (it never did before). D5=A sanctions "regardless of class", and
the session *is* the clobber risk - but no test pins the case and `cap.go`'s doc comment
enumerates parked/authoring/other-actions without it, so intent is indistinguishable from
oversight. Since the ship-reap is best-effort (`gogo-done` swallows its errors), a failed reap
silently caps the source. Pin the intended answer in a test and state it in the doc comment.

## Verdict

**CHANGES** - 1 blocker (REV-001, the release CI gate) and 2 majors (REV-002 FR12a copy,
REV-003 the unattended launch path). The core of the change is strong: FR4a is correct on every
path I could reach, the plan-readiness derivation is genuinely crash-safe and asymmetric in the
safe direction, the cap fix does what D5=A says, and the new suite survives a 20/20 mutation
sweep. Fix the three, batch the five minors, and re-review.
