# Test round 1 — plans-board-kanban

**Verdict: PASS** (1 nit, non-blocking — no hands-on check was blocked).

## What was exercised

### 1. Existing suites (build → unit → e2e)
```
cd cli && gofmt -l . && go vet ./... && go test -race -count=1 ./...
```
- `gofmt -l .` — clean (no files listed).
- `go vet ./...` — clean.
- `go test -race -count=1 ./...` — **green**, all 13 packages, 407 test functions
  (`-count=1` to bypass Go's test cache — a real re-run, not a stale green).
- `gogo --version` → `gogo 0.26.0`, matches `cli/main.go` and
  `.claude-plugin/plugin.json`.

### 2. Real-binary CLI e2e (the actual `/tmp/gogo` binary, not the in-process test harness)
Built `cd cli && go build -o /tmp/gogo .`. Set up a scratch `GOGO_DATA_HOME`
(never touched the real `~/.gogo`) with project `app` → sources `web` (no skip)
and `api` (`planAcceptanceSkip: true`, hand-set in `config.json` — there is no
CLI verb for this flag; the TUI config tab is the normal way to set it).

Because a **real, already-running host tmux server** predates this sandboxed
session (it owns 3 of the user's own long-lived `gogo-author-*` sessions), a
detached `tmux new-session` spawns its child **outside this sandbox's file
view** — a stub `claude` placed under the sandboxed scratchpad is invisible to
that process (confirmed by direct experiment: identical stub under
`/private/tmp/claude-502/.../scratchpad/` never runs when launched via a real
`tmux new-session`, but the same stub under real `/tmp/` runs and logs
correctly). Worked around by placing the `claude` stub (records argv + a call
count) under real `/tmp/`, and — for the one live-TUI drive — mirroring
`GOGO_DATA_HOME` + the source repos + the binary into real `/tmp/` too. This is
an environment quirk of this dev host, not a product issue; noted so a re-run
elsewhere isn't surprised by it.

**Re-sequence (FR2/FR3), via the real binary + real tmux + the stub `claude`:**
- `gogo plan new` → `gogo plan add <id> web` → `gogo plan add <id> api`.
- `gogo plan ready <id>` → exit 0, status → `ready`, **zero** `claude` calls
  (no count file written) — confirms mark-ready never spawns, even with 2
  targets present.
- `gogo plan go <id>` → exit 0, plan → `active`, 2 members recorded. The real
  stub's argv log shows exactly 2 invocations:
  - web: `--permission-mode auto /gogo:plan <brief> --correlation plan-bd533408`
  - api: `--permission-mode auto /gogo:plan <brief> --correlation plan-bd533408 --skip-acceptance`
  — correlation on both, `--skip-acceptance` present only for the skip source. Matches FR3 exactly.
- `gogo plan go <id>` again (idempotent) → `"all 2 target(s) already spawned - nothing to do"`, exit 0, claude call count unchanged at 2.
- Targetless plan → `gogo plan go` → exit 1, `"plan ... has no targets - add one (\`gogo plan add ...\`) before go"`.
- Source-less project (`gogo project add emptyproj`, no sources) → `gogo plan go` → exit 1, `"project \"emptyproj\" has no sources - add one (\`gogo source add\`) before go"`.
- Unresolved target (`gogo plan add <id> ghostsource` where `ghostsource` isn't a real source) → `gogo plan go` → exit 1, names `"ghostsource"` and says `"unresolved"` on stderr.
- `gogo plan --help` lists `go` in the usage table (`gogo plan go <id> ... GO: spawn a work item into each un-spawned target (ready -> active); idempotent`).

**Done + changelog (FR5), via the real binary:** seeded `state.md` (`status:
shipped`, correlated) directly in the two scratch source repos (standing in
for the skill's real write, since the stub `claude` doesn't actually run the
pipeline). `gogo plan done <id>` → exit 0, `"accepted project-UAT ... plan is
now done"`.
- `entry.md` exists at exactly `~/.gogo/projects/app/.gogo/changelog/2026-07-23-plan-bd533408/entry.md`.
- Content: `# Second rollout`, `**plan:** plan-bd533408`, `**project:** app`, `**completed:** 2026-07-23`, `## Work items (2)`, `web : second-rollout (shipped)`, `api : second-rollout (shipped)`.
- `gogo plan list --project app` still shows `plan-bd533408 done 2 Second rollout` — the plan **stays** in the store as `done` (kanban 4th-column parity), not moved out.
- `find <web>/.gogo <api>/.gogo` after `plan done`: only the one `state.md` I manually seeded exists in each, with its **original** mtime (from before `plan done` ran) — the CLI wrote nothing new under either source's `.gogo/`.

**Live TUI drive (real binary, real detached tmux, real keystrokes — not just the message-driven tests):** mirrored the scratch data home + source repos + binary into real `/tmp` (see the sandbox note above) and drove the actual `gogo global` board:
- Opened the unified cockpit, `Tab` → plans tab: rendered **`drafts 2 · ready 0 · active 1 · done 1`**, each plan a bordered card in the right column with its `⛓ plan-XXXX` chip, `K of M work items` line, and per-source dot strip (`● ●` for the fully-resolved done plan, `· ·` for a plan whose members were never actually spawned on disk — the board correctly reads real on-disk resolution, not just the plan store's advisory member list).
- Pressed `m` on a focused draft: card moved `drafts(2)→drafts(1)`, `ready(0)→ready(1)`; status line `"marked plan-15bd1b54 ready - press m again to go (spawn its work items)"` (plain dash — REV-003's fix holds for this string); zero launches (mark-ready never spawns).
- `enter` on the done plan: detail pane rendered `WORK ITEMS` (exact heading, FR4) with `● web  second-rollout  shipped` / `● api  second-rollout  shipped`, plus the `## Project UAT` / `## UAT round 1 - accepted ... via gogo plan done` body MarkDone appends.
- `q` exited cleanly; verified via `ps` that no stray/unintended `claude` process was launched by this session (the only `claude` processes present are the user's own pre-existing long-lived sessions, untouched).
- Cleaned up: killed every scratch tmux session this test created, removed the `/tmp` mirror and the stub dir. The user's 3 pre-existing `gogo-author-*` sessions were never touched.

### 3. Phase-C auto-pickup BDD coverage (`internal/tui/pickup_test.go`)
Cross-checked every BDD case in plan.md's Phase-C list against a named test:

| Case | Test |
|---|---|
| (a) free slot fires once | `TestAutoPickupFiresIntoFreeSlot` |
| (b) fire-once, no relaunch | `TestAutoPickupFireOnce` |
| (c) non-skip source not fired | `TestAutoPickupSkipsNonSkipSource` |
| (d) at-cap: no fire, cue on both cards | `TestAutoPickupAtCapShowsCue` |
| (e) under-cap-with-one-free fires | `TestAutoPickupUnderCapWithOneFreeSlot` |
| (e, temporal half) blocked → slot frees on a later reload → transient auto-fire, cue clears | **was only proven by composing (d) + the static half of (e) — added `TestAutoPickupTransientUnblocksOnFreedSlot`** (see below) |
| (f) live session not double-fired | `TestAutoPickupNotDoubleFiredWithLiveSession` |
| (g) real gate not waived | `TestAutoPickupIgnoresRealGate` |
| — no-claude inert | `TestAutoPickupNoClaudeIsInert` |
| — REV-001 (failed launch retries) | `TestAutoPickupLaunchErrorRetries` |
| — REV-002 (reloadMsg wiring) | `TestAutoPickupFiresOnReload` |

**Finding (closed, not a bug):** case (e)'s "later reload" temporal claim (a
blocked member, once its source's slot frees, auto-fires **on that later
reload** and the cue clears — never permanently marked manual) was only
provable by composing two separate tests (the static at-cap assertion in
`TestAutoPickupAtCapShowsCue` + the separately-constructed under-cap case in
`TestAutoPickupUnderCapWithOneFreeSlot`); no single test drove the same model
through both states in sequence. Added
`TestAutoPickupTransientUnblocksOnFreedSlot` (`cli/internal/tui/pickup_test.go`)
which reconciles once at cap (asserts blocked + cued + NOT in the fire-once
set), frees the slot by clearing the busy sibling's session, reconciles again
on the **same model**, and asserts it now fires and the cue clears. All BDD
cases are now covered by a test **1:1**.

### 4. Render sanity (FR1 kanban, FR4 WORK ITEMS)
- `TestPlansTabKanbanColumns` / `TestPlansTabKanbanNavigation` assert against
  `View()` **output text** (not just model fields — the TEST-003/0.16.0
  render-vs-model lesson), confirming the four `drafts/ready/active/done`
  headers and per-status partitioning.
- Live-confirmed again via the tmux drive above (section 2).
- `plans_tab.go:1080` — `viewPlanDetail` renders the `WORK ITEMS` heading
  (verbatim), matching plan.md's approach note.

## New/extended tests added this round
- `cli/internal/plans/changelog_test.go` — `TestWriteChangelogEntryBareFallback`
  (FR5's third BDD bullet: a member with no cheap one-liner — `repo == nil` —
  records the bare `source:slug` line, no enrichment). This BDD case wasn't
  unit-tested before (and is awkward to hit live through the CLI, since the
  same repo read that gates `MembersShipped` also feeds the enrichment, so a
  black-box repro would need to race two internal reads); tested directly at
  the package level instead.
- `cli/internal/tui/pickup_test.go` — `TestAutoPickupTransientUnblocksOnFreedSlot`
  (see above — closes the case-(e) temporal-sequence gap).

Both added tests pass; the full suite (`go test -race -count=1 ./...`) is
green with them included.

## Issues found this round

| id | title | severity | status |
|---|---|---|---|
| TEST-001 | `cli/plan.go:429` still emits an em-dash in a new user-facing stderr line (`planGo`'s unresolved-target message) | nit | new |

**TEST-001** — the plan's invariants and REV-003 (review round 1) established
"no em-dash" for this feature's new user-facing strings; REV-003 fixed three
such strings but missed a fourth, new one in the same `planGo` function I
confirmed live: `gogo plan go: target "ghostsource" is not a source of "app"
— skipping` (real stderr output, captured verbatim during the unresolved-target
hands-on check above). Cosmetic only — exit code and the named target are both
correct; no test currently pins the separator character. Fix: replace the
em-dash with `' - '` at `cli/plan.go:429`, matching the three strings REV-003
already converted.

No blocker, major, or minor issues found. No hands-on/e2e check was blocked —
every check in the plan's Tests section (Phase A CLI + TUI, Phase B changelog,
Phase C auto-pickup, render sanity) ran for real, including the two live-binary
tmux drives.

## Done-bar check (test-strategy.md)
- [x] build green (`gofmt -l .`, `go vet ./...` clean)
- [x] unit + e2e green (`go test -race -count=1 ./...`, 407 test functions)
- [x] hands-on done — real binary CLI (mark-ready/go/idempotent/error paths/help/done+changelog) **and** a live tmux drive of the actual interactive board, both via `/tmp/gogo`, no hands-on check skipped or blocked
- [x] artifacts inspected directly (not vibes): `entry.md` content + path, `plan list`/`plan show` output, stderr messages, tmux `capture-pane` output
- [x] enumerations in sync: `gogo plan --help` lists `go`; `cli/main.go printHelp()` documents the kanban + `m` move + auto-pickup; version 0.26.0 everywhere checked

**Verdict: PASS with 1 nit (TEST-001), non-blocking.** The done-bar is met.
