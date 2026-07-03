# Test round 01 — feature `cli-cockpit-and-events` (combined Stage A+B)

Fresh hands-on pass over the frozen `events.jsonl` contract (Stage A) and the
real Go CLI cockpit (Stage B), per `plan.md`'s Tests section and
`docs/cli-contract.md`. Everything below was **run live** — no scenario was
reasoned about without executing it, except where a scenario was structurally
unreachable (noted under TC5).

## Verdict: **back to implement** — 1 blocker + 1 major, both agent-fixable.

Every other surface (Go gates, the events contract, the classifier/status
truth, hostile-input robustness, fsnotify, and the doc/asset sweep) is
**green**. The two new findings are both in the interactive huh-form flow —
the one part of the "star" TUI feature that had only ever been probed via unit
tests of pure guard logic and a stand-alone argv replication, never driven to
completion with real keystrokes through the live Bubble Tea event loop. That
gap is exactly what this round's TC5 pass was for.

## TC1 — Go gates (live): GREEN

```
cd cli && gofmt -l .        → clean
go vet ./...                → clean
go test -race ./...         → 52 tests pass, 5 packages (root, contract, diagram,
                               launch, pages, tui); textfmt has no tests
go build -o /tmp/gogo-t .   → OK
/tmp/gogo-t --version       → gogo 0.10.0  (matches .claude-plugin/plugin.json)
```

## TC2 — Stage A contract conformance (live): PASS

- Validated all 4 lines of the real
  `.gogo/work/feature-cli-cockpit-and-events/events.jsonl` against
  `templates/contracts/events.schema.json` field-by-field (required keys,
  `additionalProperties:false`, `ts` RFC3339, `event`/`phase` enums, `round`
  integer≥1, `slug` pattern) — **4/4 conform**.
- Cross-checked the `docs/cli-contract.md` §5 emitter table against every
  skill's actual emission instructions via grep across
  `skills/{gogo,gogo-plan,gogo-implement,gogo-review,gogo-test,gogo-knowledge,gogo-done}/SKILL.md`:
  **single owner per event, no duplicates** — `skills/gogo/SKILL.md` emits only
  `gate-opened`/`gate-resolved`; each phase skill owns exactly its own
  phase's lifecycle events. Matches REV-001's recorded fix.
- Negative test: a synthetic line with `"phase":"knowledge"` on a
  `gate-opened` event is correctly **rejected** by the schema (enum has no
  `knowledge`); the same line with `"phase":"report"` **passes** — confirms
  the documented knowledge→report mapping is load-bearing and correct. Also
  confirmed a non-RFC3339 `ts` and an extra unrecognized field are rejected
  per the schema's producer-side strictness.
- **Observation (not a defect):** the real events.jsonl for this feature only
  has 4 lines, all `implement` r3/r4 — no `plan`, `review`, or `test` phase
  events, even though the feature went through 2 plan rounds, 2 review
  rounds and is now in its 1st test round. This is inside the contract's
  documented envelope ("telemetry is best-effort... the stream may have
  gaps... a missing events.jsonl is never an error") and is explainable
  (earlier rounds ran before/while the emission instructions themselves were
  being written — a chicken-and-egg gap and/or the round-3 review's final
  verification pass was done by the orchestrator directly rather than a full
  `gogo-review` round). Flagging for visibility only: the CLI's flagship
  "live timeline" value is only as good as how reliably an LLM-driven skill
  actually appends the line in practice, which this dogfood run shows is not
  yet 100%. No `TEST-*` issue raised — nothing here violates the contract or
  is fixable in code.

## TC3 — CLI vs the real repo (live): PASS

- `gogo status` on the real repo: **9 features** (5 shipped / 3 ready / 1
  in-progress / 0 unfinished). Hand-verified **all 9** against their actual
  `state.md` phase/status and `.gogo/changelog/*/manifest.json` `members[]`
  (all 5 current changelog entries lack `members[]` — confirms REV-007's
  folder-slug-fallback note) — every classification correct, including the 3
  `done`-but-not-shipped features correctly landing in **ready-to-ship**
  (no changelog folder or `members[]` names them).
- `gogo events cli-cockpit-and-events` timeline matches the raw
  `events.jsonl` file exactly (ts formatted, event/phase/round/note all
  correct).
- `gogo view changelog-merged-entries` (glamour → stdout): renders correctly.
- `gogo view cli-cockpit-and-events:plan --web`: page built offline, **zero**
  leftover `{{TOKEN}}`s, **zero** `http(s)` refs, 2 figure blocks (solo —
  the plan's before/after `.mmd` stems genuinely don't match, so solo is
  correct per the stem-pairing rule, not a bug).
- `gogo view 2026-07-01-viewer-bundles-and-done-board --web`: built offline,
  all 3 diagrams render **solo** (`before-current-view-done`,
  `done-board-flow`, `view-done-flow`) because none of that entry's on-disk
  filenames share a stem with its `before/` file — a property of the real
  data, not the pairing logic. Verified the pairing logic itself DOES
  produce true `compare-before`/`compare-after` classes when stems match, by
  also viewing `board-actions-and-filter:report` (whose `report/flow.mmd`
  and `report/before/flow.mmd` share the `flow` stem) — paired compare mode
  confirmed working.
- `--version` = `gogo 0.10.0` = `.claude-plugin/plugin.json`.

## TC4 — Hostile/edge fixtures (live, scratch): PASS

Built a fixture `.gogo/` tree in the scratchpad
(`scratchpad/hostile-fixture/`) with: a malformed `state.md` (no valid
bolded lines at all), `events.jsonl` mixing garbage (`not even json`, a
truncated JSON object) with valid lines and an unknown `event` value, a
feature folder literally named `feature-a b"c$(touch PWNED_test4)'d`, a
legacy root `report.md` (no `report/` bundle), a changelog entry **without**
`members[]`, an empty `feature-*` folder, and a directory with no `.gogo/` at
all.

- `status` / `events` / `view` **never panicked** on any of these.
- The hostile-named feature parsed and classified correctly
  (`in-progress`/`review`/`reviewing`) with **no shell interpretation** — no
  `PWNED_test4` file was ever created (checked after every command).
- Malformed `state.md` degraded to dashes (`phase: -`, `status: -`),
  classified `unfinished`, no crash.
- Garbage-mixed `events.jsonl` skipped the unparsable lines and rendered the
  4 well-formed ones (including the unrecognized event name and an unknown
  extra field) — exactly the documented lenient-consumer behaviour.
- The `members[]`-less changelog entry correctly classified its feature as
  **shipped** via the §3 folder-slug fallback.
- `view` on a nonexistent slug and on a feature with no `plan.md` both exit
  **1** with a helpful one-line message; `status`/`events`/`view` outside any
  `.gogo/` project all print `gogo: no .gogo/ found from here up...` and exit
  **1**. No stack traces anywhere.
- Confirmed via `find -newer` that none of these read-only commands wrote
  anything into the fixture's `.gogo/` beyond what I'd created.

## TC5 — The TUI live via tmux: 1 BLOCKER found

Ran inside tmux 3.7b sessions (`gogo-test-tui`, `gogo-test-fsnotify`,
`gogo-test-tui2` — all killed afterward) against a purpose-built fixture in
the scratchpad (`scratchpad/tui-fixture/`: 1 plan-only, 1 in-progress with
`charts/flow.mmd`+`charts/sequence.mmd`+`review/issues.json`+`events.jsonl`,
2 ready-to-ship, 1 shipped/changelog).

**PASS:**
- Board renders all 4 columns with correct counts and badges
  (`plan r1`, `review r1`→ later `test r1` live, `done`, `shipped`).
- `←→`/`↑↓` navigation moves the cursor correctly.
- `/` filter narrows to matching cards; `Esc` clears back to the full set.
- `enter` opens the drill-in file list showing **only files that exist**
  (no `report.md`/`decisions.md`/`test/*` shown for a feature that has none).
- Opening `plan.md` renders via glamour (headers styled, content visible).
- Opening the events timeline renders exactly the fixture's `events.jsonl`.
- Opening `review/issues.json` renders a readable table + detail.
- Opening `charts/flow.mmd` renders **ASCII boxes+arrows**; opening
  `charts/sequence.mmd` shows the **source + "press w" hint** (no ASCII
  attempted) — both fallback paths confirmed live.
- `space` on a non-ready (in-progress) card bounces with
  `"select only ready cards (space) — this card is in-progress"`, no state
  change.
- Selecting 2 ready cards + `d` opens the huh **merged** form: release-name
  input defaults to **`ready`** (the correct longest-common-leading-word
  suggestion for `ready-alpha`+`ready-beta`), and the confirm line reads
  `will run: claude "/gogo:done ready-alpha+ready-beta" in tmux session
  gogo-don…` — both exactly right.
- `Ctrl+C` cleanly **aborts** the form back to the board with
  `status="cancelled"` and **no** tmux session / claude process created —
  the do-NOT-launch-on-cancel requirement holds.
- Touching the fixture's `feature-brewing/state.md` (review→test) while the
  TUI ran updated the badge from `review r1` to `test r1` within ~2s, no
  keypress — **fsnotify live refresh confirmed**. Creating a **brand-new**
  feature folder mid-session was picked up immediately (6 features, new
  `plan` card), and a **second** write to that same new feature's own
  `state.md` (plan→implement) was *also* picked up live — confirms REV-010's
  watch-re-arm fix holds for a feature born mid-session.
- `a` (attach) on a card with no live session bounces with
  `"no running session for <slug>"`, no crash.
- `q` quits cleanly — the tmux session's pane process exits and (being the
  only session) the tmux server itself shut down; no orphan process, no
  panic in output.

**BLOCKER — TEST-001 (new):** Neither the merged release-name form nor the
plain single-card confirm form can be **submitted** through any of their
documented keys. Deselecting to a single ready card and pressing `d` opens
the confirm-only form; pressing `y` correctly flips the visual highlight to
"Launch" (the underlying bool is set synchronously inside huh's field), but
pressing **Enter** ("enter submit" per the footer hint) does nothing further
— `tmux list-sessions` afterward shows no `gogo-done-*`/`gogo-go-*` session
was ever created. The **only** key that has any effect at all is **Ctrl+C**
(abort/cancel). Root cause (read in `cli/internal/tui/update.go`): the
top-level `Update()` only forwards `tea.KeyMsg` to the form while
`mode==modeForm`; every other message type — including huh's own internal
`nextFieldMsg`/`nextGroupMsg` that its Cmd→Msg round trip depends on to
actually advance a field or complete the form — falls through the bottom
`if m.mode == modeViewer {...}; return m, nil` and is silently dropped. This
means **FR5 is unreachable in a live interactive session**: a user sees the
right confirmation, but can never actually launch anything. This slipped
past `go test ./...` (the tui suite only drives `attemptAction`'s pure guard
function, never a real keystroke-to-`StateCompleted` round trip — grep
confirms no test does) and past review round 3 (which validated the launch
*primitive* in isolation via a stand-alone argv-dumping script, not the live
form). I separately re-verified `launch.Launch` itself is correct once
reached: a temporary, since-deleted test called `BuildIntent`+`Launch`
directly with a PATH-stubbed `claude` — it created tmux session `gogo-done-a`
and the stub received **exactly one** argv element, `/gogo:done a+b`
(injection-safe, no shell splitting) — so the fix is scoped to the message
routing in `update.go`, not the launch mechanics. See `test/issues.json` for
the full repro + proposed fix (forward all messages to the form in
`modeForm`, mirroring the existing `modeViewer` fallback).

**MAJOR — TEST-002 (new):** Cancelling a form does not clear the ready-card
selection. After the abort above, both `ready-alpha`/`ready-beta` still show
`[x]`. Moving focus to the unrelated, unselected `plan-only` card and
pressing `m` (intending to move THAT card to implement) instead silently
reopens `will run: claude "/gogo:done ready-beta" ...` — the stale ship —
because `attemptAction` checks the lingering selection before ever looking
at the focused card or the `ship` flag. Once TEST-001 is fixed this becomes
a real footgun (confirming what you think is an unrelated move actually
ships old, unintended slugs). See `test/issues.json` for the fix (clear
`m.selected` on cancel).

**Launch-guard stub scenario — completed via direct code path, not via the
live TUI (see above):** since the interactive form cannot be submitted, the
task's "confirm a ship with a stubbed claude and assert the tmux session +
argv" scenario could not be exercised *through keystrokes*. I exercised the
same underlying code (`launch.BuildIntent` + `launch.Launch`) directly with a
temporary, non-committed test file and a PATH-stubbed `claude`: session
`gogo-done-a` was created and the stub logged **exactly one** argv element
(`/gogo:done a+b`), then the session was killed and the temp test file
deleted. This confirms the launch primitive is injection-safe; it does not
confirm the live TUI can reach it, which is precisely TEST-001.

## TC6 — Sweep: PASS

- README's "The gogo CLI" section: `Go 1.25+` (matches `cli/go.mod`'s
  `go 1.25.0`), install block correctly shows `cd cli && go build -o gogo .`
  with the `go install ./cli` binary-name caveat (REV-009's fix, verified in
  place).
- `docs/architecture.md` has the `cli/` entry and the `events.jsonl` line in
  the work-folder tree (REV-002's fix, verified in place); `docs/index.md`
  has the CLI-contract pointer.
- 12 slash commands, unchanged (`commands/*.md` count = 12, matches
  `docs/architecture.md`'s comment).
- `.gitignore` covers `cli/gogo` and `cli/dist/`; `git add -n cli/` stages 71
  files, no binary/`.test`/`.out`/`.log`.
- All 7 `cli/internal/pages/assets/*.js|css|html` files are byte-identical
  to `assets/viewer/*`; `cli/internal/pages/assets/mermaid.min.js` is
  byte-identical to `assets/mermaid/mermaid.min.js` (confirmed via `diff`).
- `.claude-plugin/plugin.json` = `0.10.0`.
- `git diff skills/gogo-done/SKILL.md` is **purely additive** (10 insertions,
  0 deletions) — the 0.8.0 writer logic is untouched; only the events-emission
  paragraph was added.

## Explicit verdicts on the five asked dimensions

- **(a) Events contract conformance end-to-end:** schema-conformant (4/4),
  single-owner emission is consistent across all skills, and the
  knowledge→report gate mapping is enforced by the schema. The one caveat is
  a content-completeness observation (not a conformance failure) noted under
  TC2 — this feature's own events.jsonl doesn't yet carry its full plan/
  review/test history, which is inside the contract's documented best-effort
  envelope.
- **(b) Classifier/status truth on the real repo:** **faithful** — all 9 of
  this repo's real features classify correctly, hand-verified against
  `state.md` and `.gogo/changelog/`.
- **(c) The TUI interaction loop, incl. form-cancel safety and the stubbed
  launch argv:** navigation/filter/drill-in/viewers/fsnotify are all
  **correct and live-verified**. Form-cancel safety **holds** (Ctrl+C never
  launches anything). But **form-confirm is broken** (TEST-001) — the
  interaction loop cannot complete a launch at all through the live TUI, and
  a related stale-selection bug (TEST-002) compounds it. The stubbed-launch
  argv is **safe** when reached (verified via a direct, temporary probe of
  the same code `launch.Launch` uses), but the live TUI currently cannot
  reach it.
- **(d) Hostile-input robustness:** **solid** — no panics, no shell
  injection, sane exit codes across every fixture tried.
- **(e) fsnotify live refresh:** **solid** — sub-2s live badge/column
  updates, including for a feature folder created mid-session (REV-010's
  fix holds) and a second write to that same new folder.

## Cleanup confirmation

- All tmux sessions I created (`gogo-test-tui`, `gogo-test-fsnotify`,
  `gogo-test-tui2`, plus the transient `gogo-done-a` from the stub probe)
  were killed; `tmux list-sessions` shows no server running.
- The `claude` PATH stub was only ever applied inline to a single command
  (`PATH="$SCRATCH/stubbin:$PATH" go test ...`) in an isolated Bash
  invocation, never exported process-wide; `which claude` resolves to the
  real CLI.
- The temporary `cli/internal/launch/zz_scratch_stub_test.go` probe file was
  deleted; `find cli -iname '*scratch*'` is empty.
- All fixtures live under the scratchpad
  (`scratchpad/hostile-fixture/`, `scratchpad/tui-fixture/`,
  `scratchpad/validate_events.py`, `scratchpad/neg_events.jsonl`,
  `scratchpad/claude-stub.log`, `scratchpad/stubbin/`) — nothing was written
  into the repo's real `.gogo/work/` or `.gogo/changelog/` (confirmed with
  `git status` + `find -newer /tmp/gogo-t`, which shows zero files touched
  in the feature folder during this session). The only real-repo writes were
  under the gitignored `.gogo/resources/` (view pages built by `gogo view
  --web`), which is the CLI's sanctioned write path.

## Route

**Back to implement** with `test/issues.json` (TEST-001, TEST-002) — both
agent-fixable, no user decision needed. Re-run this test round after the fix
to re-drive the exact same tmux repro (select 2 ready + `d` + edit the
suggested release name + Enter/confirm to actually complete the merge-ship,
and a single-card `m`/`d` end to end) before calling the TUI done.
