# Test round 01 — `plans-tab-launch-diagnostics-and-view`

**Date:** 2026-07-29 · **Tester:** gogo-tester · **Scope:** the round-01+round-02 fixes
per plan.md (all 11 FRs) and review-02.md's APPROVE verdict, re-verified hands-on.

**Everything below was DRIVEN, not read.** A fresh isolated fixture (`GOGO_DATA_HOME` under
`/tmp/kastest-pt-launch-diag/`, never the user's real `~/.gogo/`) with a project `kastest`
(sources `srca`/`srcb`/`srcclean`/`srcupgrade`, all plain temp dirs with **no** `.gogo/` of
their own except where a board fixture feature required one), a stubbed `claude` on PATH that
records its argv, and real scratch tmux sessions prefixed so they can never be confused with
the two protected user sessions.

---

## 0. Baseline

```
gofmt -l .            → (no output)
go vet ./...           → clean
go test -race ./...    → ok, all 13 packages (contract/tui/launch/plans/projects/pages/...)
go build -o /tmp/kastest-gogo .   → builds
/tmp/kastest-gogo --version        → gogo 0.28.0
```
All green before any hands-on exploration, re-confirmed again at the end (see below).

---

## 1. The original bug, end-to-end, with a real ~20KB plan (TUI drive)

Built `plan-kastesthuge1` (20343 bytes on disk, 20217-byte body, deliberately **no**
`## Source briefs` section — the "whole body becomes the goal" reproduction from plan.md's
Context) targeting source `srca`, status `ready`. Launched `/tmp/kastest-gogo` in scratch
session `kastest-plans-item1` (`GOGO_DATA_HOME` + stub-`claude`-first `PATH`), switched to the
plans tab, navigated to the card, pressed `m`, confirmed the "Accept plan-kastesthuge1 and spawn
1 work item(s)? into: srca" dialog with `y`.

**Result:** `accepted plan-kastesthuge1 — spawned 1 work item(s)` (dim, no error marker) — the
plan moved ready→active. A real tmux session `gogo-plan-kastest-huge-plan-fixture-one` was
created (`tmux list-sessions` confirmed it, then killed by exact name). Its pane (stub `claude`
echoing argv) read:

```
ARG[3]: /gogo:plan read your brief at /tmp/kastest-pt-launch-diag/datahome/projects/kastest/.gogo/plans/plan-kastesthuge1.md, section `## Source briefs` -> `### srca` --correlation plan-kastesthuge1
```

This is the exact `pointerText()` shape, naming the plan's real on-disk path AND the
`### srca` section — a locatable brief, not a stranded analyst. **Not** `exit status 1`.

---

## 2. Both CLI doors, headlessly, at over-budget size

Same reproduction shape, two fresh plans (`plan-kastestgo1`, `plan-kastestpromote1`, ~20KB
bodies, target `srcb`). With the isolated `GOGO_DATA_HOME` + stub-`claude` `PATH`:

```
$ gogo plan go plan-kastestgo1 --project kastest
spawned work item for plan plan-kastestgo1 in srcb - launched /gogo:plan read your brief at
.../plan-kastestgo1.md, section `## Source briefs` -> `### srcb` --correlation plan-kastestgo1
(gogo-plan-kastest-cli-door-fixture-plan-kastestgo1)
plan plan-kastestgo1 - spawned 1 work item(s) (now active)
exit=0

$ gogo plan promote plan-kastestpromote1 srcb --project kastest
spawned work item for plan plan-kastestpromote1 in srcb - launched /gogo:plan read your brief
at .../plan-kastestpromote1.md, section `## Source briefs` -> `### srcb` --correlation
plan-kastestpromote1 (gogo-plan-kastest-cli-door-fixture-plan-kastestpromote1)
exit=0
```

Both exit 0, both created a real scratch tmux session (confirmed via `list-sessions` +
capture-pane showing the same folded-pointer argv, then killed). Both CLI doors got the
identical fold this round (REV-002 confirmed live, not just in the developer's/reviewer's own
harness).

---

## 3. The backstop — a real case where even the FOLDED command still overflows

Went with the **`A` (plan-with-claude), many-sources** angle (more realistic than an
astronomically long project name, which macOS's ~255-byte path-component limit makes
impractical). Built project `kastestbig` with **320 sources**, each a real temp dir
(`svc-000`..`svc-319`), registered in its own isolated `GOGO_DATA_HOME`. Sanity-derived the
expected size first with a temporary Go probe (`launch.AuthorPlanIntent` + `FoldToPointer`
called directly, removed after use): 19403 bytes before fold, **19510 after** — the
NOT-foldable source list alone (`name -> path; ...`) blows the budget regardless of the goal.

Then drove it for real: launched the TUI on the `kastestbig`-only data home, pressed `A`,
typed a one-line goal ("kastest item3 backstop goal"), left the title blank, submitted.

**Result (live, real UI):**

```
✗ plan-with-claude failed: tmux new-session refused before launch: the command line is 19747
bytes, tmux accepts at most 16317 - shorten the brief (it lives on disk; the launch already
points at it)
```

This is the **typed** `CommandTooLongError.Error()` string verbatim (byte count + limit named),
rendered with the red `✗` marker — never a bare `exit status 1`. `tmux list-sessions` confirmed
**no** session was created (the preflight refused before `runTmux`/tmux was ever invoked). The
draft plan itself was still minted (`plans.New` runs before the launch attempt — expected,
unrelated to the failure). Spot-checked two of the 320 source dirs (`svc-000`, `svc-319`):
still completely empty — no source write occurred.

---

## 4. `v` and `w` on the plans tab

Used `plan-kastestview1` (draft, body containing a distinctive marker
`KASTEST_VW_MARKER_XYZ`).

- **`v` from the LIST:** glamour viewer rendered the marker sentence correctly.
  **`esc`** → back on the plans board (list), pane alive (`pane_dead=0`), no crash.
- **`enter`** into the plan DETAIL, **`v` from DETAIL:** same content rendered.
  **`esc`** → back on the plan DETAIL (not the board, not `modeDrill`), pane alive, no crash.
  This is the FR2.2 regression guard (`viewDrill` used to deref a nil `m.drill`) — confirmed
  live from **both** origins.
- **`w`** (from the detail): status line read
  `page: /tmp/kastest-pt-launch-diag/datahome/projects/kastest/.gogo/resources/view/plan-kastestview1.html`
  — under the isolated `GOGO_DATA_HOME`, never a source repo. File confirmed on disk (1801
  bytes), contains the marker text. The stub `open` (PATH-first, ahead of the real `open`) was
  invoked exactly once, logging the same path.
- **FR2.4 invariant:** before/after the `w` build, `find $SRC_CLEAN` (a source of the same
  project that carries **no** `.gogo/` of its own) still returns only the bare directory —
  confirmed no `.gogo/` (or anything else) ever appeared there. Also diffed `src-a/.gogo`,
  `src-b/.gogo`, `src-upgrade/.gogo` trees before/after the whole session: only the fixture
  files I planted, nothing added by the CLI.

---

## 5. Cap legibility (work board — real keystrokes)

Fixture features (state.md, real classify()-driven classes) + one real fake-but-live tmux
session per "busy" card:

| card (source) | class/status | test | result |
|---|---|---|---|
| `kastest-second` (srca, cap 1, `kastest-busy` live+in-progress) | unfinished/plan-accepted | `m` | **⚠ amber bounce**: `cap 1 reached in src-a - already building kastest-busy (the cap counts in-progress work items with a live session, per source; plans are never counted); press M to force, ship one, or run gogo go kastest-second --force` |
| same card | — | `M` | confirm opened, `FORCING past the source cap - cap 1 reached in src-a - already building kastest-busy ...` shown verbatim (cancelled, not submitted, to avoid an unnecessary real spawn) |
| `kastest-pending` (srca, `awaiting-plan-acceptance`) | unfinished | `M` | confirm = `will run: claude "/gogo:accept kastest-pending" ...` with **no** "FORCING" text at all — REV-010's fix confirmed live (accept is uncapped by design) |
| `kastest-freework` (srcb, uncapped) | unfinished/plan-accepted | `m` → `Enter` | **dim/plain success**: `launched /gogo:go kastest-freework → tmux gogo-go-kastest-freework (press a to attach)`; real session created, confirmed, killed |
| `kastest-willfail` (srcb) | unfinished/plan-accepted | `m` → confirm → (stub `claude` removed from PATH, **and** relaunched the whole TUI with a minimal PATH so no *real* claude could be found either — see note below) → `Enter` | **✗ red failure**: `launch failed: claude CLI not found on PATH — cannot launch "/gogo:go kastest-willfail"` |
| `plan-kastestdangling1` (`targets: srcmissing`, not a real source) | plans tab | `m` | **⚠ amber refusal, before any confirm**: `plan targets srcmissing, which is not a source of project kastest - add it in the config tab, or retarget the plan` — zero confirm opened, zero launch (confirmed via `list-sessions`), not the old silent "spawn 1 work item(s)" |

All three severities (red/amber/dim) are visually distinct in the real rendered `View()`
output, matching FR3.2 exactly.

**One methodology note, not a product bug:** my first attempt at the failure case (renaming
just the stub `claude` off PATH) actually found the user's **real** `claude` binary elsewhere
on the inherited `PATH` and started a genuine Claude Code session (stuck at the first-run
trust prompt, never accepted, no action taken) — I killed it within seconds and re-ran the
whole scenario with the TUI launched under a **minimal, explicit** `PATH`
(`stubdir:/opt/homebrew/bin:/usr/bin:/bin`, no `~/.local/bin`) so "claude not found" is
genuine. No real Claude session did anything; this is recorded so the next tester doesn't
repeat the mistake.

---

## 6. The upgrade transition (REV-009)

**Code-level, against the two REAL live sessions** (read-only `tmux list-sessions`, never
touched otherwise): added a temporary `_test.go` in `cli/internal/launch` asserting
`SessionMatchesSlug(realSession, derivedTitle)` for both:

```
gogo-plan-catalogue-side-of-the-matching-engine---normalise-store-embed-hard-filter (73-char label)
  vs "Catalogue side of the matching engine - normalise, store, embed, hard-filter" → PASS
gogo-author-for-gogo-project-lets-add-few-new-tasks-to-plan (47-char label, under the cap)
  vs "for gogo project lets add few new tasks to plan" → PASS
```

Both pass under 0.28.0. **Removed the temp file afterward** (not kept as permanent coverage —
it is keyed to two specific, transient real session names that will eventually end; the
durable regression coverage already lives in the developer's own `TestSessionMatchesSlugCoversAuthorAndResume` etc.).

**Live scratch-TUI proof** (never touching the real sessions): fixture source `srcupgrade`
with feature `kastest-refactor-notificationdeliveryorchestrationpipelineforrealtimeevents`
(75-char slug, `implementing`) + a real fake tmux session named with the **old, unbounded**
label form (`gogo-go-kastest-refactor-notificationdeliveryorchestrationpipelineforrealtimeevents`,
i.e. what a pre-0.28.0 gogo would have minted). The board rendered it with a live **`●`** dot
immediately (`kastest-refactor-notif… ●`), and pressing `m` on a second `srcupgrade` card
(cap 1) correctly **bounced**, citing the long slug by name:

```
cap 1 reached in src-upgrade - already building kastest-refactor-notificationdeliveryorchestrationpipelineforrealtimeevents ...
```

This is the definitive live consequence: the widened `SessionMatchesSlug` correctly attributes
an old-style long session to its feature — `●` dot, and the cap correctly counts it.

**Addendum — `a` (attach), driven live (added after a peer review pass flagged it as the one
un-driven consequence of the three named in the original ask).** Recreated the same fake
old-style session (`gogo-go-kastest-refactor-notificationdeliveryorchestrationpipelineforrealtimeevents`,
never one of the two real protected sessions — reconfirmed present/untouched before starting),
relaunched the scratch TUI on the `kastest` fixture, focused that exact card (status line
confirmed focus: `● kastest-refactor-notificationdeliveryorchestrationpipelineforrealtimeevents
has a live session — l peek · a attach`), and pressed `a`. Result:

```
detached from gogo-go-kastest-refactor-notificationdeliveryorchestrationpipelineforrealtimeevents
```

— i.e. `attachSession`'s `tea.ExecProcess` actually ran the real `tmux attach-session`/
`switch-client` against that **exact** session name and returned a clean (non-error) outcome,
not `no running session` and not a fall-back to the log peek. Confirmed via a fresh research
pass (read-only, file:line) that this is **not** a separate lookup: `a` → `attachFeature`
(`cli/internal/tui/update.go:595-614`) → `liveSessionsFor` (`update.go:848-858`) →
`launch.SessionMatchesSlug` (`update.go:853`) — the identical predicate the `●` dot
(`view.go:614` → `model.go:1106-1108`'s `hasLiveSession` → `liveSessionFor`,
`update.go:839-846` → `SessionMatchesSlug` at `update.go:841`) and the cap counter
(`orchestrator/cap.go:82`, calling `SessionMatchesSlug` directly) both bottom out in
(`launch.go:758`). One shared exact-match resolver behind all three surfaces, confirmed both
by code and by this live drive. Cleaned up immediately after: killed the fake session and the
scratch TUI session; `tmux list-sessions` reconfirmed only the two real protected sessions
remain.

---

## Tests added/extended

None left in the tree. Two temporary `_test.go` probes were used during this round (REV-009's
real-session check, item 3's sizing probe) and **deleted** after use — both were tied to
either transient real data (live session names) or one-off sizing math already covered by
the shipped `TestFoldToPointer*`/`TestTmuxCommandBytesAndLimit` suite. No gap in the existing
FR-guard coverage was found that warranted new permanent tests (all 11 FRs are already guarded
per review-02.md, independently re-derived live above).

## Issues found this round

See `test/issues.json` (1 entry):

- **TEST-001** (minor, P3, status `new`, **fixable** — a one-line product-code change in
  `cli/internal/tui/plans_tab.go`): the plans-tab spawn/accept confirms
  (`startPlanSpawnForm`, `startPlanDoneForm`) default their `formBinding.confirm` to `false`
  (Cancel), unlike the board's `m`/`M` confirm (`move.go`'s `startFormOverriding`, which
  explicitly sets `confirm: true`) — a bare `Enter` silently cancels a plans-tab spawn where
  the same keystroke launches on the board. Reproduced live (huge-plan confirm cancelled on
  bare Enter, required `y` instead). Likely pre-existing from the 0.25.0 plans-board work, not
  introduced by this round's changes — flagging for the implement agent's call on whether to
  fold it in now or defer.

No blockers, no majors. Everything this plan's six review-flagged behaviours claim to fix
was independently reproduced working, live, with real keystrokes/CLI invocations/tmux
sessions — not re-read from the developer's or reviewer's own test suite.

## Cleanup confirmation

- `tmux list-sessions` before starting: the two real user sessions present.
- Every `kastest-*`-tagged scratch/fixture tmux session created during this round (TUI
  driving sessions `kastest-plans-item1`, `kastest-item3`; fixture placeholder sessions
  `gogo-go-kastest-busy`, `gogo-go-kastest-refactor-notificationdeliveryorchestrationpipelineforrealtimeevents`;
  every spawn/CLI-door/board-launch session created along the way) was killed by **exact**
  session name as soon as it was no longer needed.
- `tmux list-sessions` at the end: **only** the two real user sessions remain, untouched
  (never attached to, killed, resized, or sent keys).
- No writes landed under any source repo's own tree beyond the fixtures I deliberately
  planted (`.gogo/work/feature-kastest-*` under `src-a`/`src-b`/`src-upgrade`, created by me,
  not the CLI); `src-clean` stayed completely empty throughout.
- Scratch data lives entirely under `/tmp/kastest-pt-launch-diag/` and `/tmp/kastest-gogo`;
  the user's real `~/.gogo/projects/` was never touched (every invocation carried its own
  `GOGO_DATA_HOME`).

## Verdict

**Done-bar MET.** `gofmt -l .` clean, `go vet ./...` clean, `go test -race ./...` green
(13 packages), and all six required hands-on/e2e checks were run to completion with real
keystrokes / real CLI invocations / real (stubbed-claude) tmux sessions — none were skipped
or blocked. One minor, non-blocking, likely-pre-existing UX inconsistency found
(TEST-001, fixable) and logged; no blockers, no majors. Ready to advance past phase ④ (the
one open item is a minor for the next implement round to pick up, or defer, at the
orchestrator's/user's discretion).
