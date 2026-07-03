# Review — `cli-cockpit-and-events` · Stage B (the Go CLI) · issues.json round 3

Fresh-eyes, staff-level review of **Stage B** — the real Go code under `cli/`
(~a dozen source files + tests) plus the plugin docs sweep. Stage A (the events
contract, REV-001..007) was approved in round 1/2 and only spot-checked here;
all seven remain **verified**. This snapshot is the second rendered human view
(`review-02.md`); the living contract is `review/issues.json` (round 3).

## Verdict: **APPROVE** — no open blockers or majors.

Five new findings, all **minor/nit** — three minors, two nits. None block the
merge; they are polish + one live-badge robustness call worth doing before this
ships as the flagship cockpit.

## What I ran (gates + probes)

- `go vet ./...` → clean · `go test ./...` → **all packages pass** (contract,
  diagram, launch, pages, tui, root; textfmt has no tests) · `go build -o
  /tmp/gogo-review-bin .` → OK.
- `gogo --version` → `gogo 0.10.0` (mirrors plugin.json).
- `gogo status` on the **real** repo → 9 features (shipped 5 · ready 3 ·
  in-progress 1 · unfinished 0); cross-checked every class against the actual
  `state.md` statuses and `.gogo/changelog/` — correct (shipped via
  `status: shipped`; done+report → ready-to-ship; reviewing → in-progress).
- `gogo events cli-cockpit-and-events` → shows the dogfooded lines.
- `gogo view cli-cockpit-and-events:plan --web` → built offline; **zero**
  leftover `{{tokens}}`, **zero** `http(s)`/network refs, 2 figure blocks.
- **Hostile fixture** (scratch repo): a feature folder named
  `feature-a b;$(touch PWNED)` + garbage `state.md`/`events.jsonl`/`manifest.json`
  → `gogo status/events/view` **never crashed**, degraded to dashes/empty enums,
  sane exit codes (0/1), and **PWNED was not created**.
- **tmux injection probe** (the security crux): replicated `launch.Launch`'s argv
  (`tmux new-session -d -s <sess> <bin> "<command-with-hostile-slug>"`) with an
  argv-dumping script (never claude). tmux **execs the command directly** — the
  hostile slug arrived as a single `ARG[0]` with **no shell interpretation**;
  neither `INJECTED` nor a `$(...)` subshell file was created.
- Byte-compared all 8 embedded assets (`cli/internal/pages/assets/*`) vs
  `assets/viewer/*` + `assets/mermaid/mermaid.min.js` → **identical**.
- `git check-ignore cli/gogo` → ignored; `git add -n cli/` stages 71 files, no
  binary/`.test`/`.out`/`.log`/`dist`, only the intentional 3.3 MB
  `mermaid.min.js`; `go.sum` present.

## Explicit verdicts on the four asked dimensions

- **(a) launch injection safety — SAFE.** Slugs come from directory names but
  never reach a shell: `exec.Command` uses no shell, tmux execs argv directly
  (proven empirically), session names are sanitized to `[a-z0-9-]`, and the
  no-tmux path passes `in.Command` as a single `claude -p` arg. Verified end to
  end. Every launch is behind the huh confirm — `Launch` is called only from
  `doLaunch`, reachable only after `huh.StateCompleted && m.confirm`.
- **(b) classifier fidelity — FAITHFUL.** `classify()` reproduces the
  `skills/gogo-status/SKILL.md` rule table verbatim and first-match order:
  shipped (`status:shipped` OR changelog folder-slug+report OR `members[]`) →
  ready (report/report.md OR legacy root) → in-progress (phase/status
  implement|review|test) → unfinished. `members[]` fallback, legacy report,
  changelog-wins-over-ready all covered by fixtures; malformed state → default
  class, no panic; the status golden pins it.
- **(c) fsnotify / tea concurrency — CORRECT (with a live-refresh gap).** The
  watcher goroutine only ever sends on a buffered channel; the model mutates
  solely inside `Update` via the `waitForReload` Cmd → `reloadMsg` loop — no
  cross-goroutine model mutation, debounce coalesces via a non-blocking send.
  Gaps: the watch set is frozen at Init (REV-010) and never closed (REV-011).
- **(d) pages fidelity vs gogo-view step-3 — FAITHFUL.** figure blocks,
  `data-diagram` stems, `before-` prefix for the before figure, compare pairing
  by stem, solo `Added`/`Removed` rows, manifest captions, mermaid fence + Status
  line stripped, all tokens replaced, no `fetch`/network, resources materialized
  only under `.gogo/resources/`. The escape-the-mmd judgment call round-trips:
  source `&amp;` → `&amp;amp;` → browser `textContent` decodes back to `&amp;`
  (identical); escaping only `& < >` keeps `<pre>` well-formed.

## Findings this round (round 3)

| ID | Sev | Status | Fix class | One line |
|---|---|---|---|---|
| REV-008 | minor | open | agent-fixable (design call) | Badge trusts the latest event over `state.md`'s current phase → stale badge that disagrees with its column (live: `implement r3` shown while state=review). |
| REV-009 | minor | open | agent-fixable | README install is wrong twice: "Go 1.22+" vs go.mod `go 1.25.0`; `go install ./cli` yields a binary named `cli`, not `gogo`. |
| REV-010 | minor | open | agent-fixable | fsnotify watch set is fixed at Init; a feature created mid-session isn't re-watched, so its later writes stop refreshing the board. |
| REV-011 | nit | open | agent-fixable | The fsnotify watcher + reload goroutine are never closed (process-lifetime leak). |
| REV-012 | nit | open | needs-user-decision | `mermaid.min.js` (3.3 MB) is committed twice (assets + go:embed); intentional for a standalone binary, but doubles the "one vendored runtime" footprint. |

### REV-008 — stale sub-phase badge (minor)
`tui/model.go badge()` prefers `f.LatestEvent` for the phase text and only falls
back to `state.md` when there is no event at all. The column comes from
`state.md` (via the classifier), so when telemetry lags — a gap the contract
calls normal — the badge and column disagree. Live on this repo:
`events.jsonl` ends at `phase-done/implement r3` while `state.md` is
`review/reviewing`, so the card sits in **in progress** (from review) but shows
`implement r3`. `docs/cli-contract.md` §5: "fall back to `state.md` for the
current phase; `events.jsonl` adds only the timeline and rounds." **Fix:** take
the phase from `state.md`, add the round from the latest event only when their
phases agree.

### REV-009 — README install instructions inaccurate (minor)
Both verified: (1) `cli/go.mod` says `go 1.25.0`, so "needs Go 1.22+" is wrong
(1.22–1.24 can't build without toolchain download). (2) The module tail is
`cli`, so `go install ./cli` / `go install …/cli@latest` installs a command
named **`cli`**, not `gogo` (`go build -o <dir>/ .` produced `cli`). The
`go build -o gogo .` form is fine. **Fix:** correct the version floor and either
drop the `go install` line, document the rename, or move the main package to a
`gogo/` dir so `go install` names it `gogo`.

### REV-010 — fsnotify not re-armed for new features (minor)
`startWatchCmd` runs once from `Init()`; `reload()` rebuilds columns but never
re-adds watches. A feature born in-session is picked up once (creation fires on
the `.gogo/work` watch) but its own dir + subdirs are never watched, so later
`state.md`/`events.jsonl` writes don't refresh the board. The primary
live scenario (an existing in-progress feature advancing) works. **Fix:** keep
the `*fsnotify.Watcher` and `Add()` newly-seen feature dirs on each reload.

### REV-011 — watcher/goroutine never closed (nit)
No `w.Close()` and the reload goroutine loops forever; harmless (process
lifetime) but unclean. Bundle with REV-010's re-arm refactor.

### REV-012 — mermaid.min.js committed twice (nit, needs-user-decision)
`cli/internal/pages/assets/mermaid.min.js` is a byte-identical 3.3 MB copy of
`assets/mermaid/mermaid.min.js`, embedded for a plugin-independent binary
(documented, kept in sync by `make sync-assets`). Correct call for standalone
`go install`, but a second sanctioned vendored runtime vs the Footprint NFR's
"one per project." Recommend **accept** (Option A) and note it in the NFR.

## Prior rounds (unchanged)
REV-001..007 (Stage A: events contract) remain **verified** — spot-checked only
this round; not re-opened.
