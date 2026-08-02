# Test — round 1 · `launch-confirm-modal-and-fast-toggle` (0.36.0)

**Scope.** Phase ④ hands-on/e2e test of the modal launch confirm + per-launch `--fast`
toggle, against the accepted plan (**D1=B** three-option Select · **D2=B** launch-confirm
site only, serving `m`/`M`/`d` · **D3=A** strip+dim backdrop · **D4=A** `formOrigin`) and
`.gogo/knowledge/test-strategy.md` / `testing-tools.md` (the tmux live-TUI path, since
tmux 3.7b is installed on this host). Also independently re-verifies **REV-006** (modal
minimum raised 12→15) and **REV-007** (stale prose) from review round 3, which shipped
after the round-2 review verdict.

## Suites (re-run, not taken on trust)

| Gate | Result |
|---|---|
| `cd cli && gofmt -l .` | clean (no output) |
| `cd cli && go vet ./...` | clean |
| `cd cli && go test -race -count=1 ./...` | **green** — 13 packages (11.5s for `internal/tui`) |
| `cd cli && go build -o gogo .` | clean; `./gogo --version` → `gogo 0.36.0` |

## Hands-on fixture

A scratch repo + a scratch `GOGO_DATA_HOME` (both removed at the end of this round — see
Cleanup), built to exercise `m`/`M`/`d` and the fastMode/cap path per the task brief:

- **Fixture repo** (`~/gg-e2e-fixture-repo`, 46 chars — a realistic root length, not the
  scratchpad's much longer path, so the `at <root>` tail in the confirm's title matches
  what review round 2 measured): 6 cards —
  `add-dark-mode-toggle-to-settings-panel` (38-char slug, plan-accepted — the primary go
  target, chosen deliberately different from the reviewer's own fixture slug/root to check
  the REV-006 fix generalises), `backdrop-card` (plan-accepted, visual backdrop),
  `pending-plan` (awaiting-plan-acceptance — exercises the accept confirm), `busy-build`
  (implementing, holds a live `gogo-go-busy-build` stub session for the cap/M-force test),
  `ship-ready-report` + `ship-ready-two` (awaiting-uat + `report/` — ready-to-ship, the
  second added mid-run to exercise the merged-ship confirm).
- **`GOGO_DATA_HOME`** (`~/gg-e2e-datahome`): one project `app`, one source `web` →
  the fixture repo's path, `concurrentWorkItems: 1`, `fastMode: true` — the registered
  source needed for the fastMode-seed and cap/M-force checks (the single-repo board never
  consults a project's sources; only `gogo global`/the unified cockpit does — confirmed by
  reading `chooseBoard`/`capWatchSources` before building the fixture).
- **Stub `claude`** on PATH: appends argv + `$PWD` to a log file, then sleeps — never a
  real Claude build. A stub `tmux` was not needed; real tmux ran the stub-spawned sessions.
- Driven with `tmux new-session -d -x <W> -y <H> ...` + `send-keys`/`capture-pane` (plain
  and `-e`), per `../skills/tui-tmux-testing/SKILL.md`.

## What was exercised

### Modal rendering + fast toggle (FR1–FR5, FR7, FR8, FR13)
- `m` on a plan-accepted card → the go **Select** opens as a **modal**: board visibly
  dimmed (confirmed via `capture-pane -e` — the background carries an SGR grey
  `38;2;154;160;170` foreground, i.e. `dimStyle`, not plain text) around a rounded box; all
  three options (`Launch` / `Launch --fast (token-lean gogo-fast)` / `Cancel`) present; title
  shows `will run: claude "/gogo:go <slug>"`.
- `j`/`k` moves the cursor; the title **updates live** with each move (confirmed to
  `--fast` and back), and the session name in the title never changes — `SetFastParam`
  touches only `Command` (FR4).
- Bare **Enter** launches: stub log shows exactly one call, argv
  `--permission-mode auto /gogo:go add-dark-mode-toggle-to-settings-panel` (no `--fast`,
  matching the seeded/shown option), `$PWD` = the fixture root.
- **Esc** and the **Cancel** option both return to the board (status line: `cancelled`);
  reopening the confirm **re-seeds** fresh each time (verified: a card launched via
  `M`→toggle-to-full-Launch, reopened, re-seeded straight back to `Launch --fast` from the
  source config — the choice is per-launch only).
- **FR5 (config immutability):** with a `fastMode: true` source, `M`→toggle to `Launch`
  (override the seed)→Enter launched `/gogo:go backdrop-card` with **no** `--fast`; the
  project's `config.json` on disk still read `"fastMode": true"` afterward, byte-identical.
- **Cap + M-force (FR3.3):** with `busy-build` holding a live `gogo-go-busy-build` session
  and `concurrentWorkItems: 1`, plain `m` on another card in the same source **bounced**
  (`⚠ cap 1 reached in gg-e2e-fixture-repo - already building busy-build ...`, no confirm
  opened); `M` opened the confirm WITH the compressed `FORCING past the source cap - cap 1
  reached in gg-e2e-fixture-repo - already building busy-build` description, pre-seeded to
  `Launch --fast` (source config), all three options visible at 200×50.
- **FR6 (no fast toggle off the go confirm):** the ship (`d`) and accept (`m` on
  `pending-plan`) confirms render as **modals** too (per D2=B's site-scope reading, REV-002)
  but as a plain **Confirm** (`Launch`/`Cancel`, no speed line) — no fast option, matching
  the plan. The **merged**-ship confirm (2 ready cards selected, `d`) adds a release-name
  `Input`; pressing `f` there **typed the character** (`> ship-readyf`) rather than
  toggling anything — confirms `f` is not intercepted off the go confirm.
- **D2=B scope boundary:** `x` (delete), `K` (kill), `P` (plan-session) all rendered
  **full-screen, byte-for-byte** — no box border, no dimmed board — at 200×50/120×30,
  confirming the modal is scoped to the one `m`/`M`/`d` launch-confirm site only. Cancelled
  every one without mutating anything (`backdrop-card` was not deleted, `busy-build`'s
  session was not killed, no plan-session was spawned).

### Size matrix (FR10–FR12) — including the REV-006 boundary
Driven on the long-slug/long-root card (`add-dark-mode-toggle-to-settings-panel`, 46-char
root) so the title wraps as much as a realistic repo would:

| Size | Result |
|---|---|
| 200×50, 120×30, 80×24 | modal renders, board visible/dimmed around it, nothing overflows |
| **60×15** (named minimum) | modal renders; **all three options visible** (unforced); **forced (`M`) with FORCING note**: seeded `Launch --fast` visible, `Cancel` reachable via one `j` (viewport scroll) — matches the review's documented acceptable degradation for the forced arm |
| 60×14 (one row below) | **full-screen fallback**, byte-for-byte — all three options visible (unsized form sizes its viewport to the rendered height) |
| 46×9 | full-screen fallback, clipped by terminal size like any small full-screen form today |

### Colourless terminal (FR13, Diagnosability)
`NO_COLOR=1` pass: `capture-pane -e` showed **zero** ESC sequences on the confirm screen;
box border glyphs (`╭─╮│╰─╯`, the field's `┃`/`>` markers), the option words, and the live
title flip (toggled `j`, title gained ` --fast`) were all still visible/assertable as plain
text — the state is carried by words/glyphs, not colour alone.

## REV-006 / REV-007 independent verification

Both were `fixed` in implement round 3, after the round-2 review verdict — this round is
their first independent check.

- **REV-006 (modal minimum 12→15) → `verified`.** Reproduced with a fixture the reviewer
  never used (38-char slug, 46-char root, vs. the reviewer's 36-char slug / 37–43-char
  root) to check the fix generalises rather than fitting one probe. The named defect — the
  go Select (unforced **and** forced) losing its action row around the old 12–14 minimum —
  is fixed: every option renders at 60×15, and the one-row-below fallback (60×14) shows
  everything via the unsized-viewport path, exactly as claimed.
- **One adjacent gap found, filed separately as `TEST-001`** (does not contradict REV-006's
  own fix — see below) — the merged-ship modal's Input+Confirm group hides the
  `Launch`/`Cancel` row until one `Enter`/`Tab` at the same 60×15 minimum, a cell
  `TestModalNeverExceedsTheTerminal`'s "merged ship" case never reaches and never asserts
  content for.
- **REV-007 (stale prose) → `verified`.** Re-grepped the whole repo (excluding
  `.gogo/work/`/`.gogo/changelog/` archives) for the four flagged phrases plus the bare
  `startForm` symbol — zero live hits. Spot-checked all six named sites (README,
  `docs/cli-contract.md`'s 0.36.0 note — now says "at least 60x15", consistent with
  REV-006's fix — `skills/gogo-cli/SKILL.md`, `move.go`'s `startFormOverriding` doc,
  `launch_modal_test.go`'s header/seed comment) — all read the new title-carries-the-command
  phrasing.

Both status flips + verification evidence are recorded in `review/issues.json` (still the
review track's file — this round only updated `status`/`fix_summary`, per the skill's
instruction to record verification evidence there).

## New issue this round

| id | severity | priority | title |
|---|---|---|---|
| TEST-001 | minor | P2 | Merged-ship modal hides Launch/Cancel at the 60×15 minimum until one Enter/Tab; untested at that size/content |

See `test/issues.json` for the full description + proposed solution (a test-gap fix is
required either way; a product change to reveal the Confirm on first paint is the
maintainer's call, not required). Not a dead end — the hint bar names the key
(`enter next`) that reveals it, matching the precedent already accepted for the forced
go-Select's scroll-to-reveal `Cancel`.

## Cleanup

All scratch tmux sessions (`gg-e2e-a`, `gg-e2e-b`, `gg-e2e-nc`, the stub-spawned
`gogo-go-*`/`gogo-done-*` sessions, `gogo-go-busy-build`) killed; `~/gg-e2e-fixture-repo`,
`~/gg-e2e-datahome`, `~/gg-e2e-stubs` removed. The real production session
(`gogo-go-launch-confirm-modal-and-fast-toggle`, the one driving this very pipeline run)
was never touched.

## Verdict

**Issues found — routes back to implement.** Build/vet/test all green; every hands-on
check in the plan's Tests section ran (none blocked — tmux + Node.js were both available,
no environment gate hit). One new minor issue (`TEST-001`) needs a fix (or an explicit
maintainer decision to accept the current behaviour) before the done-bar is met; `state.md`
routes to implement with `test/issues.json`.
