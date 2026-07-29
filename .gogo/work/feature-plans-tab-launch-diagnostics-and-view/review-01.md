# Review round 01 — `plans-tab-launch-diagnostics-and-view`

**Date:** 2026-07-29 · **Reviewer:** gogo-reviewer (fresh eyes) · **Scope:** 18 modified files
(+862/-119) plus `cli/internal/launch/tmux_test.go` and `cli/internal/tui/plans_view_test.go`
· **Contract:** `plan.md` (accepted, 11 FRs, three legs), `decisions.md` D1-D6, `adjustments.md` A2.

**Gates re-run independently:** `gofmt -l .` clean · `go vet ./...` clean · `go test -race ./...`
green (13 packages) · `gogo --version` → `0.28.0`. Version bumped in both
`.claude-plugin/plugin.json` and `cli/main.go`, pinned by `TestVersionMirrorsPlugin`.

---

## What holds up

Verified, not assumed — each of these was probed, not read:

- **The under-budget no-op claim is true.** I compiled `HEAD:cli/internal/launch/launch.go`
  and the shipped one side by side and diffed the produced argv for four representative
  intents plus an `AuthorPlanIntent`. The `Command` argv element is **byte-identical** in every
  case; the only delta anywhere is the `-s` session-name value on a 76-char title, which is
  FR1.4's deliberate cap, not the fold. `Intent.Body` never reaches an argv
  (`TmuxNewSessionArgs` / `TmuxPersistentArgs` / `ClaudePrintArgs` / `PhaseArgs` read only
  `Session` and `Command`) and is never serialized anywhere.
- **A2's `exactTarget` / `exactPaneTarget` split is applied correctly at every call site.**
  `has-session` and `kill-session` get the bare `=<name>` session target; `capture-pane` gets
  the `=<name>:` pane target; `new-session -s` gets the bare name on **both** create paths, and
  `TestExactTargetOnProbesNotOnCreate` asserts no argv element anywhere in either create argv
  starts with `=`. Mutation-checked: reverting `HasSessionArgs` to a bare target fails that test.
  The one `-t` left un-exacted is `AttachArgs` — see **REV-008**.
- **`m.planViewing` cannot leak or double-set.** `openArtifact` unconditionally enters
  `modeViewer`, and `modeViewer` has exactly one exit (`q`/`esc`/`left`/`h` in `updateViewer`),
  which checks `peeking` then `planViewing`. `peeking` is only set by `startPeek`, reachable
  only from the board — the two flags have no common entry. `esc` from a view opened in a plan
  **detail** correctly returns to the detail (`closePlanView` leaves `m.planDetail` intact and
  `viewPlans` dispatches on it); from the kanban it returns to the kanban. Mutation-checked:
  removing the `planViewing` branch fails `TestPlansTabViewEscReturnsToPlansTab`; removing the
  `viewDrill` nil guard **panics** that test. Both guards bite.
- **`resolveTargets` refuses correctly.** A plan with no resolvable target opens no confirm and
  names the source *and* the project; a mixed plan confirms only the spawnable half and says
  what it is skipping; the launcher fires exactly once at the right root. Mutation-checked:
  reverting the partition fails two tests.
- **The `~/.gogo/`-only invariant holds.** `TestPlansTabWebPageWritesUnderGogoHome` asserts the
  page lands under `~/.gogo/projects/<name>/.gogo/resources/view/` **and** that a real temp
  source dir is left without a `.gogo/` at all. Mutation-checked as a real render assertion, not
  a model-field check (`TestStatusSeverityDistinguishesOutcomes` fails when `renderStatus` is
  flattened — the 0.16.0 lesson is respected).
- **Doc sync is complete for the surface that changed.** `main.go printHelp`, `README.md`,
  `docs/cli-contract.md` and `skills/gogo-cli/SKILL.md` all carry `v`/`w`/`M` and the cap scope;
  no other `docs/*.md` or skill enumerates the cockpit keymap (grepped). The new
  `TestPlansTabKeyHelpInSync` genuinely bites — dropping `v` from the kanban help line fails it,
  and its segment-head matching (not `strings.Contains`) is the right call.
- **No new em dash in any added line** (the 16 hits are pre-existing strings re-indented into
  `statusBlocked(...)`).

---

## Findings

| id | sev | pri | status | title |
|---|---|---|---|---|
| REV-001 | **major** | P1 | new | `sanitizeLabel`'s dash-boundary cut collapses a long label to its first word |
| REV-002 | **major** | P1 | new | Fold wired only into the TUI — `gogo plan go` / `promote` still build a 20 951-byte command |
| REV-003 | minor | P2 | new | `TestAttachFailureIsReported` asserts a test-local copy, not the production callback |
| REV-004 | minor | P2 | new | `planKebab` left on the old regex — the two doors write different `SlugHint`s |
| REV-005 | minor | P2 | new | Three shipped wirings have no test that bites (mutation-verified) |
| REV-006 | minor | P2 | new | FR3.2 classification incomplete — four outcome sites still render dim |
| REV-007 | minor | P2 | new | `M` + a ready selection claims to force past a cap it never consulted |
| REV-008 | nit | P3 | new | `AttachArgs` is the one `-t` left on prefix/fnmatch resolution |

---

### REV-001 — `sanitizeLabel`'s dash-boundary cut collapses a long label to its first word · **major** · P1

`cli/internal/launch/launch.go:684-698`. The FR1.4 cap cuts on the last `-` inside `s[:48]` with
no floor on what survives. Measured against the shipped code:

```
"A supercalifragilisticexpialidociousreallylongidentifiername"        -> "a"          (1)
"x abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz"              -> "x"          (1)
"Refactor NotificationDeliveryOrchestrationPipelineForRealtimeEvents" -> "refactor"   (8)
```

The third is an ordinary plan-title shape (a verb plus one long identifier). It defeats FR1.4's
own goal ("48 keeps them readable and still unique"): two plans sharing a first word now mint the
same session base, `uniqueSession` starts appending `-2`/`-3`, and `SessionMatchesSlug` cannot
separate them — the attribution ambiguity `coding-rules.md` TEST-005 exists to prevent, arriving
through a lossy transform instead of a substring match. `planSlugHint` runs the same transform, so
the member hint shown in the detail and written into the project changelog degrades too. Before
this change both were fully distinct. `TestSanitizeLabelBounded` passes on all three fixtures — its
"last segment must be a whole word" assertion is satisfied by a 1-char label.

**Fix (AGENT-FIXABLE):** give the boundary cut a floor —
`if i := strings.LastIndex(s, "-"); i > MaxSessionLabel/2` — falling through to the hard cut that
already exists and is already tested. Add the degenerate fixtures to `TestSanitizeLabelBounded`.

### REV-002 — the fold is TUI-only; the CLI door to the same bug is still open · **major** · P1

`cli/plan.go:444` (`gogo plan go` fan-out) and `cli/plan.go:559` (`gogo plan promote`) build the
identical `launch.PlanIntent(p.Title, goal, p.ID)` and launch it with **no** `FoldToPointer`.
Measured with the user's real shape (~20 KB per-source brief):

```
gogo plan go argv bytes     = 20951 (limit 16317) -> over budget: true
plans-tab (folded) argv     =   275                -> fits: true
```

So the headline bug is fixed behind `m`/`c` in the cockpit and still fully live behind
`gogo plan go`, which `README.md` advertises as the scriptable equivalent. The new preflight does
improve the message there (a typed `CommandTooLongError` naming the byte count), but the user is
still blocked on a limit they did not create — the outcome D1 rejected as option B.

**Scope caveat, stated honestly:** `plan.md`'s Changes checklist scopes the fold to three TUI
sites and does not list `cli/plan.go`, so this is a gap in the plan's coverage rather than a
deviation from it — but FR1.3 is phrased as a property of *the launch*, not of a key binding.

**Fix (AGENT-FIXABLE, 2 lines per site):** mirror `plans_tab.go:707` — set `intent.Root = src.Path`
and call `launch.FoldToPointer(intent, plans.Path(project, p.ID), <source>)` after the existing
`SkipParams` append; cover both through the injected `planLauncher` seam. If the scope call is to
keep this release strictly TUI-only, mark it `wontfix` and record it in `decisions.md` — but do not
ship it silently.

### REV-003 — the FR1.6 test does not exercise the FR1.6 code · minor · P2

`cli/internal/tui/plans_view_test.go:471-502` calls `m.attachSession(...)` only to check the cmd is
non-nil, then asserts against `attachOutcome` — a helper **defined in the test file** (line 497)
that re-implements the production decision. Mutation-verified: deleting the `if err != nil` branch
from `update.go:623-626` (reverting FR1.6 exactly) leaves `go test ./internal/tui/` **green**. Same
for `peek.go:114-117`. The test's comment claims it asserts "both attach sites"; it asserts neither.

**Fix (AGENT-FIXABLE):** promote `attachOutcome` to a package-level function in `update.go`, call it
from both `tea.ExecProcess` callbacks, delete the test-local copy, and re-run the mutation.

### REV-004 — `planKebab` was left behind, so two doors write different `SlugHint`s · minor · P2

`cli/plan.go:657-666` still uses `[^a-z0-9]+` with no bound while `plans_tab.go:47` moved to
`launch.SlugFromLabel`. Both write the same `plans.Member.SlugHint` field of the same store
(`plan.go:453`/`566` vs `plans_tab.go:525`/`730`), which is rendered in the plan detail, in
`MembersShippedIn`'s unshipped list and in the project changelog. For `"Catalogue side - normalise"`
the CLI now writes `catalogue-side-normalise` and the TUI `catalogue-side---normalise`. The two were
identical before this release. Not a correctness bug — the spawn guards key on `Source` alone, so no
duplicate member is created — but it is exactly the drift `code-review-standards.md` #1 forbids.

**Fix (AGENT-FIXABLE):** delegate `planKebab` to `launch.SlugFromLabel` and assert
`planKebab(x) == planSlugHint(x)` for a `" - "` title and a 60-char title.

### REV-005 — three shipped wirings have no test that bites · minor · P2

Each mutation applied alone against the shipped tree, then `go test ./internal/...`:

| mutation | result |
|---|---|
| revert `planSlugHint` to the old regex (undoes half of FR1.7) | **green** |
| delete the `FoldToPointer` call at `plans_tab.go:522` (the `c` site) | **green** |
| delete the `FoldToPointer` call at `plans_tab.go:963` (the `A` site) | **green** |

`plan.md`'s checklist names all three explicitly. `TestSpawnOversizedBriefFoldsToPointer` covers only
the `m` fan-out; `TestFoldToPointerFoldsAnAuthoringGoal` covers the launch-package function, not the
TUI wiring that calls it.

**Fix (AGENT-FIXABLE):** a `planSlugHint ≡ launch.SlugFromLabel` assertion, plus `c`-key and `A`-key
variants of the oversized-brief test.

### REV-006 — FR3.2's "every outcome is classified" is incomplete · minor · P2

Four outcome sites still render as plain dim success, one of them beside a classified sibling in the
same `if`:

- `plans_tab.go:402` — `"refusing — %d of %d member(s) not shipped"`; the `len(p.Members) == 0`
  arm two lines above **did** get `statusBlocked`, so the same project-UAT refusal is amber or dim
  depending on which arm fires.
- `plans_tab.go:456` — the same refusal inside `finishPlanDone`.
- `update.go:157-165` — the `planAuthorLaunchedMsg` headless branch ("no tmux — the analyst is
  running headless … `tmux` recommended"), a degraded fallback rendered as a success.
- `update.go:808-810` — `"killed N session(s), M failed"`; a partial kill failure now has tmux's own
  words available through `TmuxError` and discards both the words and the severity.

`plan.md`'s outcome-taxonomy diagram places the first three in `Blocked` and the fourth in `Failed`.
Secondary note on discipline: the reset lives only at `Update`'s `tea.KeyMsg` choke point, so the
third site and `attachSession`'s bare `m.status = "attaching " + session` (`update.go:620`) inherit
whatever level the previous async message left. I could **not** construct a concrete sequence where a
stale error level survives to colour them — every async status-setter either carries a level or is
immediately preceded by a keypress — but classifying them removes the question.

**Fix (AGENT-FIXABLE):** wrap the four sites in `statusBlocked` / `statusFailed`, classify
`attachSession`, and extend `TestStatusSeverityDistinguishesOutcomes` with the project-UAT refusal.

### REV-007 — `M` + a ready selection shows a force note on a ship confirm · minor · P2

`move.go:193-199` computes `override := m.capBounce(m.focusedCard())` unconditionally, but
`attemptActionForce` returns the merged-ship intent from the **selection** branch (`move.go:42-61`)
before any `capBounce` is reached. Reproduced against the shipped tree (busy in-progress + live
session, next unfinished, shipme ready; cap 1 — select `shipme`, focus `next`, press `M`):

```
┃ will run: claude "/gogo:done shipme"  in tmux session gogo-done-shipme  at /r/
┃ FORCING past the source cap - cap 1 reached in web - already building busy (th
```

A confirm dialog is the safety surface for a state-changing launch, so wrong text there is more than
cosmetic.

**Fix (AGENT-FIXABLE):** compute the override note only when the resolved action is the one the cap
guards — `capBounce` is a pure read, so it can safely move below `attemptActionForce` and be gated on
`!isShip && len(m.selectedFeatures()) == 0`. Add a selection case to `TestForceMoveOverridesCap`.

### REV-008 — `AttachArgs` is the one `-t` left on prefix resolution · nit · P3

`launch.go:844-849` still emits `switch-client -t <session>` / `attach-session -t <session>` while the
three probes moved to the exact form. Both take a target-session and are subject to the same
exact → prefix → fnmatch resolution `plan.md` measured and called a wrong-session hazard. The window
is narrow — every caller passes a name that came from `ListSessions()` or a fresh `Result.Session`, so
an exact match normally exists and wins — but if that session exits between the listing and the
attach, tmux falls back to a prefix match and drops the user **into a different feature's live
session**. Raised as a nit because FR1.5 named only the three probes and the change wants the same
live tmux verification A2 got.

**Fix (AGENT-FIXABLE, verify first):** use `exactTarget(session)` (the bare `=<name>` session form,
**not** the pane form) on both branches, confirm live on tmux 3.7b that both subcommands accept it and
refuse a prefix, and pin it in `launch_test.go`. If either rejects `=`, close as `wontfix` with the
measurement in `adjustments.md`.

---

## Plan fidelity

| FR | built | note |
|---|---|---|
| FR1.1 typed `TmuxError` + stderr | ✅ | `Error()` reading pinned; `Unwrap()` reaches `*exec.ExitError`; bounded capture never short-writes the child |
| FR1.2 preflight + `MaxTmuxCommandBytes` | ✅ | 16317 pinned; boundary (at-limit accepted) tested; A2's conservatism recorded in the doc comment |
| FR1.3 fold-to-pointer | ⚠️ | correct and no-op under budget, but wired only into the TUI — **REV-002**; two of three TUI sites untested — **REV-005** |
| FR1.4 bounded session names | ⚠️ | cap works; the boundary cut degenerates — **REV-001** |
| FR1.5 exact probes (+A2) | ✅ | all three call sites correct; `new-session -s` clean on both paths; `AttachArgs` out of scope — **REV-008** |
| FR1.6 attach failures reported | ⚠️ | code is correct at both sites; the test does not exercise it — **REV-003** |
| FR1.7 attribution covers every action | ⚠️ | action list ✅ and tested; `planSlugHint` alignment untested — **REV-005**; CLI mirror not aligned — **REV-004** |
| FR2.1-FR2.4 `v`/`w` + invariant | ✅ | seams reused, no new renderer; `esc` return correct from both kanban and detail; write-scope asserted against a real temp source dir |
| FR2.5 key/help sync + guard | ✅ | the guard is real (segment-head matching, AST-derived keys) and bites |
| FR3.1 unknown targets refused up front | ✅ | both `m` and `c`; confirm names what it skips; plan left untouched |
| FR3.2 status severity | ⚠️ | plumbing + render assertions are good; four sites unclassified — **REV-006** |
| FR3.3 `M` force | ⚠️ | works and is tested; bogus note on the selection path — **REV-007** |
| FR3.4 cap scope documented | ✅ | config form, source detail, bounce text, `--help`, README, SKILL |

Nothing unplanned crept in. Out-of-scope items (D4's session recording, D5's cross-repo over-count,
`--plan-file`, cap default) were all correctly left alone. The A2 correction is real, correctly
implemented, and documented in three places.

---

**Verdict: CHANGES** — 2 open majors (REV-001, REV-002); 5 minors and 1 nit alongside. All 8 are
AGENT-FIXABLE; REV-002 additionally carries a scope question worth a one-line confirmation.
