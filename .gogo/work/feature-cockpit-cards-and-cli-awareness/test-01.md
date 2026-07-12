# Test round 01 — cockpit-cards-and-cli-awareness

- **Phase:** ④ test (fresh-eyes, hands-on)
- **Date:** 2026-07-12
- **Scope:** working-tree diff (uncommitted) — Slice A (plugin CLI-awareness) + Slice B (rich board drill-in card), version 0.15.0 → 0.16.0. Ignored the unrelated `.gogo/work/feature-persistent-session-orchestrator/state.md` change (out of scope) and the untracked `feature-immediate-kill-at-ship/` folder (a different feature, not part of this plan).

## Verdict: NOT all-green — 1 fixable defect + 1 required user-decision gate

Build/unit gate is fully green. Hands-on Slice B exploration surfaced one real
defect (**TEST-001**, major, agent-fixable). Hands-on Slice A proof cannot be
tester-verified and is recorded as a required user-decision gate (**TEST-002**,
major, per plan.md — never a silent skip). Neither blocks re-running this phase
once TEST-001 is fixed and TEST-002 is resolved by the user.

## 1. Automated Go gate

Run from `cli/` (Go 1.26.0 on this host):

```
gofmt -l .                    → clean (no output)
go vet ./...                  → clean
go test -race -count=1 ./...  → ok, all 9 packages (cli, contract, diagram,
                                 diagram/mermaidascii, launch, orchestrator,
                                 pages, trash, tui) — 2.5–3.1s per package
```

Confirmed **every** named new test actually executes (not just "the package
passed") via `-run` + `-v`:

- `TestCLICommandEnumerationInSync` — PASS (root `cli` package).
- `internal/tui`: `TestDrillCardDetailRender`, `TestSessionRowsTable` (all 6
  subtests incl. `exact-match guard (TEST-005)` and the missing-registry
  degrade), `TestEventsTail`, `TestDrillAttachWiring`, `TestDrillKillWiring`
  (all 3 subtests, incl. the REV-001 regression: esc/cancel-button both assert
  `mode == modeDrill`), `TestDrillDegradesNoSessions` — all PASS.

## 2. Hands-on Slice B — live tmux drive of the drill card

Per `test-strategy.md`'s "Go TUI — unit tests are NOT enough" override, built
the binary (`cd cli && go build -o /tmp/gogo-test .`) and drove the real board
with real keystrokes in a throwaway tmux session, exactly the established
pattern (`gogo-test-board-e2e`-style, see `feature-unattended-ops-input-signals`
round 1).

**Fixture:** `scratchpad/drillfix` = a copy of
`cli/internal/contract/testdata/repo/.gogo` plus a hand-built
`.gogo/resources/cli/sessions/*.json` registry (not part of the fixture
repo — added for this round) so every `sessionRows` case could be exercised
against the real render, not just the table test:
- `inprogress.json` — a `go` leg, `status: running`, `tmux:
  gogo-go-inprogress` → paired with a **real** decoy tmux session
  `gogo-go-inprogress` → **tracked live**.
- `legacy-ready.json` — a `go` leg, `status: reaped`, `tmux:
  gogo-go-legacy-ready` → **no** matching live tmux session → **tracked
  stale/reaped**.
- A decoy tmux session `gogo-go-ready` with **no** registry entry for
  `ready` → **untracked live racer**.
- `unfinished` — no registry, no live session → **no tracked sessions**
  degrade case.

Launched detached: `tmux new-session -d -s gogo-test-drill-e2e -x 220 -y 50
"cd <fixture> && /tmp/gogo-test"`; drove it with `tmux send-keys`; asserted
with `tmux capture-pane -pt gogo-test-drill-e2e`.

**Confirmed live (FR-B1/B2/B4/B6):**
- **`inprogress` (tracked live):** the detail panel renders `description`,
  `folder feature-inprogress/`, `status reviewing · review r2`, a sessions
  section reading `●  go  live  running  gogo-go-inprogress  $0.42 · 12
  turns`, a recent-events tail (4 real lines from the fixture's
  `events.jsonl`, correctly formatted `HH:MM:SS  event phase [note]`), the
  file list below, and the help line `↑↓ files · enter open · a attach · K
  kill · G glow · w web · esc back`.
- **`ready` (untracked live racer):** sessions section reads `●  untracked
  live  gogo-go-ready` — a live `gogo-*` session with no registry entry is
  still shown, per FR-B2.
- **`legacy-ready` (tracked stale/reaped):** sessions section reads `○  go
  stale  reaped  gogo-go-legacy-ready  $1.10 · 30 turns` — a reaped tracked
  leg is labelled, not dropped, per FR-B2.
- **`unfinished` (no registry, no sessions):** sessions section reads `no
  tracked sessions` — the FR-B5 degrade case, live.
- **`K` kill-confirm dialog (on the live `inprogress` card):** renders
  exactly `Kill inprogress's live session?` / `kills gogo-go-inprogress
  (tmux) — the pipeline state is untouched` / `Kill    Cancel`, matching the
  plan's copy.
- **REV-001 regression, live:** pressed `Esc` on that confirm — landed back
  on the `inprogress` **drill card** (not the board), with the live session
  row unchanged (`running`, untouched) — the review-round fix holds under
  real keystrokes, not just the unit test.
- **No accidental mutation:** `tmux list-sessions` before/after every step
  confirmed the pre-existing real orchestrator session
  (`gogo-go-cockpit-cards-and-cli-awareness`) and the two decoy sessions were
  never killed; nothing was ever confirmed through a Kill dialog. Quit with
  `q` (drill → board → quit); the test session left no residue. Decoy
  sessions (`gogo-go-inprogress`, `gogo-go-ready`) killed manually afterward
  as cleanup — never anything real.

**Found live, not by the unit suite (TEST-001):** on the `legacy-ready` card
(stale session, no live session), pressed `a` — the pane was byte-identical
before/after, no hint appeared. Pressed `K` — same silent no-op, no confirm,
no hint. Traced this to `viewDrill()` (`cli/internal/tui/view.go`) never
rendering `m.status` at all, unlike `viewBoard()` which does. Corroborated by
returning to the board after a drill-mode `K`-cancel: the board's status line
then read `cancelled` — proof `m.status` **was** being set correctly
(`finishKill`), just never rendered while `mode == modeDrill`. This silences
five distinct confirmations: attach-no-session, kill-no-session, kill
cancelled, kill succeeded, and post-attach detach. See `test/issues.json`
TEST-001 for the full trace and proposed fix. This is exactly the failure
mode `test-strategy.md`'s "Go TUI — unit tests are NOT enough" override
exists to catch: `TestDrillAttachWiring`/`TestDrillKillWiring` assert
`Model.status` and pass, but never assert `View()` renders it.

## 3. Slice A markdown — content read + lint cross-check

Read all four FR-A5 sync sources directly (not just trusting the lint):
`skills/gogo-cli/SKILL.md` (new — frontmatter description reads as a real
discoverability trigger: "Load this when the gogo CLI is relevant: the user
asks how to launch/attach/resume/sweep sessions..."), the lean
`**Load when:**` pointer in `skills/gogo/SKILL.md`, the `## Command surface
(enumeration-sync anchor)` in `docs/cli-contract.md`, and the new "CLI
companion reference" line in `README.md`. Content matches FR-A1/A2/A4/A6:
conditional framing ("NOT bundled with the plugin... only suggest or use it
if gogo is on the user's PATH"), the full v0.15.0 command surface + flags +
persistent-session model (one-owner lock, kill-at-ship/sweep), explicit
when-to-use guidance, and the "later active half extends this file" note.

Code-read `cli/cli_enum_test.go`'s `TestCLICommandEnumerationInSync`: it is a
real, non-tautological lint — derives the 9 canonical verbs from
`main.go`'s actual `switch args[0] { case ... }` dispatch (not
hand-maintained prose) and greps all 4 sources including `printHelp` itself
(the REV-002 fix), so a future dropped/renamed command fails the build.
Manually cross-checked the dispatch block against the derived verb list —
correct.

**The real proof is hands-on, and it is out of reach for this tester
(TEST-002):** whether an *installed Claude*, in a project with `gogo` on
PATH, actually surfaces/suggests the CLI when asked to manage or view gogo
work is a live skill-selection behaviour of a *different* Claude session —
not an artifact this tester can assert. Per `plan.md`'s Acceptance signal
("The hands-on proof ... is a user-decision check, never a silent skip") and
the gogo-test skill's blocked-hands-on-check rule, this is recorded as
**TEST-002** (major, needs-user-decision) rather than silently skipped or
silently assumed to pass. All artifact-level prerequisites for it are
verified green (above) — only the live behavioural proof remains, and only
the user can run it.

## Issues this round

| id | title | severity | priority | status | tag |
|---|---|---|---|---|---|
| TEST-001 | Drill status/hint messages set on the model but never rendered (`viewDrill()` has no status line) | major | P1 | new | agent-fixable |
| TEST-002 | Hands-on proof of Slice A CLI-awareness (installed Claude surfaces the CLI) | major | P1 | new | needs-user-decision |

Full detail (file:line pointers, exact repro, proposed fixes) in
`test/issues.json`.

## Verdict against the done-bar (`test-strategy.md`)

- **Build + unit green — MET.** `gofmt`/`vet`/`go test -race -count=1 ./...`
  all clean; every named new test confirmed executing and passing.
- **Hands-on/e2e done — NOT fully met.**
  - Slice B hands-on: **done**, and it found a real defect (TEST-001) — a
    blocker for calling this round clean, not for the hands-on check itself
    (the check ran to completion).
  - Slice A hands-on: **blocked for this tester by design** (a check only a
    user-facing Claude session can perform) — recorded as TEST-002, a
    user-decision gate per `plan.md`, not a silent skip.
- **Net: the done-bar is NOT met this round.** Route: TEST-001 back to ②
  implement (fixable) → re-review if warranted → back here to re-test; TEST-002
  needs the user's decision (run the check now, explicitly skip it, or report a
  failure to iterate the skill wording) before phase ④ can call itself
  all-green and advance to ⑤ report.
