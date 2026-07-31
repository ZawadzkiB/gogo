# Test round 01 — plan-readiness-gate (ships as 0.29.0)

**Track:** test (④) · **Round:** 1 · **Date:** 2026-07-30

This is a TUI/CLI feature. Per `test-strategy.md`'s explicit rule ("never sign off an
interactive flow that has not been driven with real keystrokes") and the 0.28.0
mutation-sweep hardening, every scenario below was driven **live**: real `tmux
send-keys` / `capture-pane` against the actual `gogo` binary and isolated scratch
fixtures — not code-reading, not unit-test trust alone.

## Environment / isolation

- Built `gogo` fresh from HEAD (`cd cli && go build -o /tmp/gogo-test-binary .`).
- All fixtures lived under a scratch root (`/tmp/gogo-e2e-<pid>/`, deleted at the end):
  two scratch source repos (`src-a`, `src-b`), each with its own `.gogo/work/`, and a
  scratch `GOGO_DATA_HOME` registering one project (`testproj`) with both as sources
  (default cap 1 each). **Nothing was written under `~/.gogo/projects/` or under this
  repo's own `.gogo/work/`.**
- **Host safety (verified before AND after):** `tmux list-sessions` showed exactly the
  two protected user sessions (`gogo-author-for-gogo-project-lets-add-few-new-tasks-to-plan`,
  `gogo-plan-catalogue-side-of-the-matching-engine---normalise-store-embed-hard-filter`)
  before any scratch session was created and again after every scratch session was
  killed at the end — neither was attached to, killed, resized, or sent keys.
- **`gogo sweep` was only ever run in its targeted form** (`gogo sweep <scratch-slug>`),
  never bare, exactly as instructed.

## 1. Baseline (build/unit)

| Check | Result |
|---|---|
| `gofmt -l .` | clean |
| `go vet ./...` | clean |
| `go test -race -count=1 ./...` | green, all 13 packages |
| `env PATH="$(dirname $(which go)):/usr/bin:/bin" go test -count=1 ./...` (hermetic) | green |
| `gogo --version` | `gogo 0.29.0` |

Baseline matches what was claimed going in. No new automated Go tests were added this
round — the existing suite (449+ tests, review-round mutation-swept per
`test-strategy.md`'s 0.28.0 rules) already covers the unit layer for every FR; this
round's job was specifically the hands-on layer unit tests cannot provide.

## 2. Slice A — the authoring gate (item 1)

Fixtures in `src-a`: `feature-noplan-probe` (bare `templates/state.template.md`, no
`plan.md`), `feature-stub-probe` (template + a 1-section `plan.md`), and
`feature-real-dated` (a genuine, properly-dated, acceptable plan) for contrast.

Driven live (`tmux new-session -d` running the plain `gogo` board, `send-keys` +
`capture-pane`):

- Both probes rendered **`✎ authoring`**, never `⏸ accept plan`. `real-dated` correctly
  showed `⏸ accept plan`.
- Card titles fell back to the slug (`noplan-probe`, `stub-probe`) — the
  `<one-line title>` placeholder never rendered.
- Header: `⏸ 1 need you` with only the probes present (excludes both authoring cards);
  confirmed headlessly too via `gogo status` (`WAIT` column reads `-` for both
  probes, `WAIT` for `real-dated`).
- **`m` on `noplan-probe`:** `⚠ plan.md not written yet - noplan-probe is still being
  authored (no plan.md on disk yet); finish it with `gogo plan noplan-probe``.
- **`m` on `stub-probe`:** same clause, numeric: `(plan.md has 1 of the 2 sections a
  written plan needs)`.
- **`M` (force) on both:** identical refusal, byte-for-byte — no confirm form opened,
  nothing launched (FR4a holds under force).
- **`v`/`enter` (drill-in) on `noplan-probe`** (no `plan.md` at all): `(no files)` plus
  the same `⚠` message, instead of a silent/odd empty file list.
- **Sort order:** `real-dated` (a real date) sorted above both probes (empty
  `Created` after `stripPlaceholder`); between the two probes, alphabetical tie-break —
  confirms the `<YYYY-MM-DD>` placeholder no longer sorts a broken card to the top.
- **Crash-safety / becomes acceptable:** wrote a real 3-section `plan.md` into
  `noplan-probe` with no other action; the board's fsnotify picked it up
  (~2s) and the card flipped to `⏸ accept plan` with a gate stripe, `⏸ 2 need you`,
  footer `[m] accept`. Pressed `m` → the real `/gogo:accept noplan-probe` launch
  confirmation opened (`will run: claude "/gogo:accept noplan-probe" …`); cancelled
  without launching (no session spawned).
- **FR8 (accepted-but-unwritten):** built `feature-accepted-noplan` (`status:
  plan-accepted`, no `plan.md`). `m` → `⚠ accepted-noplan is plan-accepted but its
  plan.md is not written (no plan.md on disk yet) - nothing to build; re-plan it with
  `gogo plan accepted-noplan``. `M` → identical. Headless `gogo go accepted-noplan` →
  same message, exit code 1. **All three surfaces refuse identically.**
- **`/gogo:accept`'s own second gate (FR5):** this is an LLM-prose surface; verified by
  reading `skills/gogo-accept/SKILL.md` (step "2b. The plan is WRITTEN…", with the
  matching refusal text and "Never record acceptance for a plan that is not written"
  in Hard rules). Not executed live (would require spawning a real `claude` session for
  a scratch feature) — the deterministic CLI-side gates above are the load-bearing
  ones by design (D1), and those were fully hands-on verified.

**Result: item 1 and item 2 (FR4a) — PASS, hands-on.**

## 3 & 4. Slice B — the cap hole + the targeted-sweep remedy (items 3-4)

Correction made mid-round: a bare `gogo` inside a repo **never** applies the
per-source cap (single-repo mode ignores the projects store by design — confirmed by
reading `runBoard`/`chooseBoard` in `main.go`). The cap only applies on `gogo global`
(the unified cockpit) or a focused project view. Re-launched via `gogo global` with the
scratch `GOGO_DATA_HOME` exported (`tmux new-session … -e "GOGO_DATA_HOME=…"
gogo global`), and reconfirmed it showed only the scratch project (1 project, 7
features) before proceeding — no real project data was ever exposed to a keypress.

Fixtures: `buildslug-a` (`plan-accepted`, written plan, live `gogo-go-buildslug-a`
tmux session — the "pre-first-write window"), `buildslug-b` (`plan-accepted`, written
plan, no session — the second launch attempt), `planning-c` (`awaiting-plan-acceptance`
+ written plan, live `gogo-plan-planning-c` session — an authoring session).

- `buildslug-a`'s card showed **`● building`** + **`● developer`** live (FR14/FR13),
  even though `status:` on disk still said `plan-accepted`.
- **`m` on `buildslug-b`** (cap 1, `buildslug-a` already building):
  `⚠ cap 1 reached in src-a - already building buildslug-a (the cap counts work items
  with a live build session (gogo-go-<slug>), per source; an authoring session, any
  other session, and plans are never counted); press M to force, ship one, run `gogo
  sweep buildslug-a` if a blocker already shipped, or run `gogo go buildslug-b
  --force``.
  - Names **only** `buildslug-a` — `planning-c`'s live authoring session never appears
    in the blocking list. **An authoring session does not consume a slot.**
  - Headless `gogo go buildslug-b` (same cwd) → the same rule, same targeted remedy,
    exit code 1.
- **Terminal-but-live (D5):** flipped `buildslug-a`'s `state.md` to `status: shipped`
  (session still live). The cap **still** refused `buildslug-b`, still citing
  `buildslug-a` — confirms a just-shipped feature holding a lingering build session
  still counts, exactly as `cap.go`'s doc comment and D5 intend.
- **Targeted sweep frees the slot:** `gogo sweep buildslug-a` while `plan-accepted`
  (not yet shipped) → `nothing to reap` (correctly refuses — not an orphan, not
  terminal). After flipping to `shipped`, the **same** targeted sweep →
  `reaped gogo-go-buildslug-a (owning feature buildslug-a is shipped)`; `m` on
  `buildslug-b` then opened the real launch confirmation (cap cleared). Cancelled
  without launching.
- **Multi-source safety (item 4c):** built a second terminal+live-session pair,
  `srca-shipped-live` (source A) and `srcb-shipped-live` (source B) — both equally
  "terminal with a lingering session". Ran `gogo sweep srca-shipped-live` (targeted,
  scoped to the source-A slug only) from `src-a`: it reaped **only**
  `gogo-go-srca-shipped-live`; `gogo-go-srcb-shipped-live` was untouched. (The bare
  form was never run, per the hard constraint — `matchesOnly`'s slug-scoping in
  `internal/orchestrator/sweep.go` is what makes this safe, confirmed by both the code
  and this live proof.)
- **No bare `gogo sweep` at the three named surfaces** — checked live/rendered at all
  three: the board bounce (above, targeted), the headless `gogo go` refusal (above,
  targeted), and the config tab's source-detail cap row (`cap 1  (the cap counts work
  items with a live build session … an authoring session, any other session, and plans
  are never counted)` — no sweep mention at all on this surface, so a bare form isn't
  even possible there). *Context, not a defect:* `internal/orchestrator/orchestrator.go`
  has two **unrelated** bare `gogo sweep` mentions (an attach-launch informational note
  and the owner-lock-conflict hint) — these belong to a different gate (single-feature
  ownership/attach), not the cap, and the plan's own "Out of scope" section already
  records hardening bare `gogo sweep` itself as separate future work.

**Result: items 3 and 4 — PASS, hands-on, including the multi-source safety proof.**

## 5. The `state lags` cue (item 5)

Fixtures with hand-authored `events.jsonl` + live `gogo-go-<slug>` sessions:

- **Arm A** (`lag-arm-a`): `phase: implement` / `status: implementing`, newest event
  `phase-done` for `implement` (the same phase the line names). Rendered:
  `implement r2   ● developer  · state lags`.
- **Arm B** (`lag-arm-b`): `phase: review` / `status: reviewing`, newest event
  `fix-round` for `implement` (an entry event naming a *different* phase — the loop-
  back shape). Rendered: `review r2   ● reviewer  · state lags`.
- **Negative case** (`lag-aborted`): identical arm-A disagreement shape
  (`phase: implement`, newest event `phase-done`/`implement`) but `status: aborted`,
  plus a live, lingering `gogo-go-lag-aborted` session. Rendered: `implement r1
  ● developer` — **no cue at all** (REV-010's whitelist holds: `aborted` is not in
  `{implementing, reviewing, testing}`, so neither `state lags` nor `stalled` fires).
- The cue renders as **glyph + word** (`· state lags`) in a plain (non-`-e`)
  `capture-pane`, i.e. it is legible with zero ANSI/color — satisfies the
  colorless-TTY requirement structurally, not just in `go test`.

**Result: item 5 — PASS, hands-on, both arms plus the negative case.**

## 6. This repo's own card (item 6, read-only)

Read-only against the real repo (never mutated, verified unchanged before/after via
`git status`/`cat`):

- `gogo status` on `/Users/bartlomiej.zawadzki/repos/gogo` right now: `plan-readiness-gate`
  reads `WAIT -`, `LIVE -`, `implement` / `implementing` — **no live session currently
  attributes to it**, so the TUI's `· state lags` cue (which *requires* a live build
  session) would **not** render right now. This is expected, not a contradiction: the
  plan itself says "silence is not proof of health" — the disagreement lives in the
  files, independent of whether a session happens to be live at inspection time.
- `events.jsonl`'s newest line is `{"event":"phase-done","phase":"implement",...}`
  while `state.md` still names `phase: implement` / `status: implementing` — the exact
  **arm A** shape.
- To prove the mechanism reacts to this *exact real shape* without touching the real
  repo or its slug: copied (read-only) the real `state.md` + `events.jsonl` into a
  scratch fixture under a **different** slug (`realrepo-mirror`) in a scratch source,
  spun up a decoy `gogo-go-realrepo-mirror` session (never `gogo-go-plan-readiness-gate`),
  and confirmed live: `implement r4   ● developer  · state lags`. This directly
  confirms the plan's claim — a board watching this repo **with a live build session**
  would show `· state lags` on `plan-readiness-gate` right now, given the file/telemetry
  disagreement really is present on disk today.

**Result: item 6 — PASS (read-only; confirmed via safe mirrored fixture, real repo
never mutated).**

## Issues found

One new finding this round — see `test/issues.json` for the full record:

- **TEST-001** (severity `minor`, priority `P2`, status `new`, fixable) — a work item
  scaffolded straight from `templates/state.template.md` with the legend intact
  (**exactly** this plan's own prescribed Slice-A fixture) renders a bogus `⛓ ×3`
  correlation chip. Root cause: `parseStateFile` (`cli/internal/contract/state.go`)
  matches its `- **key:** value` grammar line-by-line with no awareness of a still-open
  multi-line `<!-- ... -->` block; the template's own correlation-field legend comment
  contains an example `- **correlation:** [plan-XXXX] ... e.g. [plan-7f3a, plan-9c2e]`
  line that independently matches the grammar and gets parsed as 3 real plan ids.
  Verified live: at full pane width the chip expands to the three literal bogus ids,
  visual proof of the exact misparse. **Pre-existing** (predates 0.29.0 — the
  correlation field/legend shipped earlier; this plan's FR6 added `stripPlaceholder`
  next to `stripComment` but did not touch `stripComment` itself), and **orthogonal**
  to every FR/BDD scenario this plan defines — but real, reproducible, and surfaced
  specifically because this plan's own hands-on test fixture is the trigger.

No other issues found. Every explicit BDD scenario in `plan.md`'s Tests section that
this round targeted was reproduced live and passed.

## Verdict

**Build + unit: green. Every requested hands-on scenario (items 1-6) was driven live
with real keystrokes and passed — no hands-on check was blocked, so there is nothing to
raise as a user decision gate on that front.**

**This plan's specific FRs/BDD scenarios: done-bar MET**, verified hands-on rather than
by code-reading or unit-test trust alone, per `test-strategy.md`'s TUI rule.

**Whole-issues-list done-bar: NOT fully clean** — one new, minor, agent-fixable finding
(TEST-001) is open. It does not touch any FR this plan defines and predates this plan,
but it was discovered via this plan's own prescribed fixture and is a real, visible
defect. Recommend the user decide, the same way `plan.md` itself already scoped two
adjacent findings (the bare-`gogo sweep` hardening, the cross-repo same-slug cap
over-count): either (a) loop back to ② implement for a small, contained fix to
`parseStateFile`'s comment handling, or (b) mark it `wontfix`/deferred as a fast-follow
and ship 0.29.0 as-is. Given it is pre-existing and out of this plan's explicit FR
scope, (b) is a defensible call — but that scope call belongs to the user, not to me.
