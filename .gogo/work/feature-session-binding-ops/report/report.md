# Report — feature `session-binding-ops`

- **feature:** Session binding ops — start, kill and re-assign a work item's tmux session from the cockpit (0.32.0)
- **status:** awaiting-uat
- **completed:** 2026-08-01
- **branch / commits:** main (uncommitted working tree — gogo defers commits to the user)

## Run status / gaps

All phases completed — plan (accepted 2026-08-01) → implement (3 rounds) → review (2 rounds, clean) → test (1 round, green incl. real-tmux hands-on) → report. **No open issues** in either track.

## Summary

A gogo tmux session is bound to a work item by exactly one thing — its **name** (`gogo-<action>-<slug>`), minted once at launch and, until now, never editable. When a human retasked a live pane onto a different item, every reader kept narrating the old binding: the shipped card showed `●`, the item actually being built read `· stalled`, and the concurrency cap missed a real build. This feature makes the binding **editable from the cockpit**: `P` starts a correctly-named plan session and attaches, `K` kills a chosen session from the board, and `R` re-assigns a live session onto the item it is really driving — one `tmux rename-session`, after which every reader corrects itself on its next tick.

## Planned vs shipped

Shipped as planned, with **one deliberate correction found in review**:

- **FR4's shipped-card chips (REV-003):** the plan gave a shipped card holding a session both `[K] kill` and `[R] re-assign`, but FR3 (also per plan) refuses a terminal *target* — so the `[R]` chip there was a guaranteed bounce. As-built, a shipped card offers **`[K]` only**; the re-assign of its lingering session happens via `R` on the card it should *drive*, which both the unbound count and the terminal refusal text point at. `plan.md` FR4 carries the marked correction.

Everything else — FR1 `P`, FR2 board-`K` (+ the `pickerOrigin` cancel-origin fix), FR3 `R` with derived action (D4=A), FR4 unbound count, FR5 the gogo-done spared-pane line, the enumeration sync and the 0.32.0 bump — landed as written.

## Implementation

**The whole feature is "rename the name."** Every session-aware reader re-derives its answer from the tmux session name on every tick, so making that one string editable corrects the dot, the agent chip, the `building/stalled/state-lags` cues, the per-source cap, the one-owner lock and the sweeper at once — with **no pipeline-state write, no registry rewrite, and no contract change**.

- **`P` (board + drill):** guardrails (`PlannableStatus`, claude present), then *join* an existing live `gogo-plan-<slug>` instead of duplicating; else a confirm → `launch.Launch` of `claude "/gogo:plan <slug>"` anchored at the card's own root → the TUI suspends and attaches (the plans-tab `A` shape, via the shared `planAuthorLaunchedMsg`). Uncapped and lock-free, like `gogo plan`.
- **`K` (board, promoted from the drill):** one shared `killFeature` behind both keys — 0 → named hint, 1 → Enter-safe confirm, ≥2 → one / all N / cancel picker, killing by exact name. The stale `pickerFromDrill` bool became **`pickerOrigin mode`**, recorded where each picker starts, so a cancel always restores the mode the picker was opened from.
- **`R`:** a Select over **every** live `gogo-*` session (`name · bound item | unbound · repo · age [· attached]`, from the new `launch.ListSessionMeta`), the choice being the confirmation. The action component is **derived from the target's status** (`RunnableStatus` → `go`, else `plan` — `bindAction`), shown in the picker, so adopting a building item visibly mints a cap-counted build session. Refusals all name their reason *and* unblock: terminal target, foreign `session_path` anchor, already bound, no tmux/sessions. The rename itself is `launch.RenameSessionUnique` (exact `=` target, collision-suffixed) behind a `renamer` seam.
- **Visibility (FR4):** the idle status line counts **unbound** sessions (live `gogo-*` anchored in a board root, matching no card — the plans-tab title-named spawn, or a retasked pane pre-adoption); footer chips follow the symptom (`[R] adopt` on stalled · `[K]` on shipped-with-session · `[P]` on plannable).

### Changes (as-built)

| File | Change | Note |
|---|---|---|
| `cli/internal/launch/launch.go` | modified | `SessionMeta` + `ListSessionMeta()` + pure `parseSessionMeta`; `SessionNameFor`, `RenameSessionArgs`/`RenameSession`/`RenameSessionUnique` (exact `=` session target, bare new name). `ListSessions`/`SessionAction` untouched. |
| `cli/internal/tui/session_ops.go` | **added** | The ops home: `bindAction`, `boundFeature`, `unboundSessions`, `pathWithinRoot`, `boardRoots`/`unboundHere`, `planFeature`/`startPlanSessionForm`/`finishPlanSession`, `killFeature`, `adoptFeature`/`startAdoptPicker`/`finishAdopt`, `adoptRow`, `sessionAge`. |
| `cli/internal/tui/model.go` | modified | `sessionMeta` cache + `refreshSessions()`; `renamer` seam (default `launch.RenameSessionUnique`); `pendingPlanSession`/`pendingAdopt` form state; `pickerFromDrill` → `pickerOrigin`. |
| `cli/internal/tui/update.go` | modified | Board `P`/`K`/`R` + drill `P`/`R`; form-completion routing + `formPreservesSelection`; `cancelForm`/`finishAttach`/`finishKill` on `pickerOrigin`; `killDrill` delegates to `killFeature`; session tick carries meta. |
| `cli/internal/tui/view.go` | modified | Symptom-following footer chips (`[P]` gated on `PlannableStatus` — one predicate with the handler, REV-006); unbound count on the idle status line; help lines extracted to `boardAllKeysLine`/`drillKeysLine` consts. |
| `cli/internal/launch/session_binding_test.go` | **added** | Rename-args exact-target contract, `SessionNameFor` round-trip, lenient `parseSessionMeta`. |
| `cli/internal/tui/session_ops_test.go` | **added** | `bindAction` table; P launch-once-then-attach + refusals; board-K arms (incl. the shipped-card 1-session confirm) + the `pickerOrigin` cancel fix; R derived-action renames + all refusals; unbound count; footer chips. |
| `cli/internal/tui/key_help_sync_test.go` | **added** | `TestBoardKeyHelpInSync`: every handled board/drill key must appear in the in-TUI help *and* `gogo --help`, with anti-vacuity floors. |
| `cli/main.go` | modified | Help blocks gain `P`/`K`/`R` (+ `enter/v`, `esc/q` alias forms); `Version` → 0.32.0. |
| `cli/version_test.go`, `.claude-plugin/plugin.json` | modified | 0.32.0, mirrored. |
| `README.md`, `skills/gogo-cli/SKILL.md`, `docs/cli-contract.md` | modified | Enumeration sync + a "Changed in 0.32.0" additive contract note (no `.gogo/` key changed; the name stays the binding). |
| `skills/gogo-done/SKILL.md` | modified | FR5: step 6 prints the spared-pane line when `$TMUX` is set. |
| `cli/internal/tui/tui_test.go`, `card_test.go` | modified | `newModel` zeroes `sessionMeta` (host-state leak); `pickerOrigin` assertions. |

## Decisions & rationale

All seven forks were resolved at the plan gate (2026-08-01, each on gogo's recommendation); one review finding added an as-built correction. See [decisions.md](../decisions.md).

| Decision | Choice | Reason |
|---|---|---|
| D1 — new item from the board? | **A** — card-only `P` | A goal-form spawn is born unbound (name from the label, analyst derives its own slug); C (`--slug` param) noted as the follow-up that kills the unbound-at-birth class. |
| D2 — automatic re-binding? | **A** — manual `R` + a visible detector | No deterministic item-level evidence exists (`session_path` gives the repo, nothing gives the item); the repo's NFR prefers MISSING over WRONG. |
| D3 — how far does a move go? | **A** — the rename is the whole move | Every reader derives from the name; the old registry leg truthfully renders stale; the lock's untracked-owner branch already protects the new slug. |
| D4 — action after a move | **A** — derived from the target's status | Preserving the old action (typically `done`) would leave a real build invisible to the cap — the working-tree-clobber guard. |
| D5 — card keys vs a sessions screen | **A** — card keys only | The adopt picker already lists every session; a screen is a new mode the lone repo (where the incident happened) has no tabs for. |
| D6 — sessions on shipped items | **A** — keep the truthful name, make it actionable | Auto-renaming to a neutral name makes the session a sweep-killable orphan — trading a cosmetic `●` for a data-losing reap. |
| D7 — version | **A** — 0.32.0 | 0.31.0 belongs to the in-flight notify feature; never race the same version lines. |
| REV-003 (review) | Shipped card chips = `[K]` only | FR3 refuses a terminal target by design, so a `[R]` chip there advertised a guaranteed bounce; the re-assign lives on the card the session should drive. |

## Review outcome

**Two rounds, final verdict clean.** Round 1 ([review-01.md](../review-01.md)): 1 major + 2 minor + 2 nits, all agent-fixable — the major being an anti-vacuity gap in the new key-help guard (it could pass with zero parsed keys). Fix round 2 addressed all five; round 2 ([review-02.md](../review-02.md)) **verified every fix by mutation** (a fix only counted when reverting it turned the suite red for the stated reason) and added two nits (REV-006 chip/handler predicate split, REV-007 a colliding audit tag), fixed in round 3. Living list: [review/issues.json](../review/issues.json) — 5 verified, 2 fixed, 0 open.

## Test outcome

**Green** ([test-01.md](../test-01.md), [test/issues.json](../test/issues.json) — 0 issues). Suites: `gofmt` / `go vet` / `go test -race -count=1` across 13 packages. **Hands-on, real tmux (3.7b) on this host**, driving the actual board binary in a scratch tmux with disposable fixtures: `R` adopt renamed the session and cleared `· stalled` → `● developer` immediately (the handler refreshes sessions synchronously); the `K` picker killed exactly the chosen session of two; the `P` confirm named the exact command/session/root; the unbound count counted the fixture orphan and **excluded** the real foreign-root `gogo-plan-catalogue-…` session; shipped-card refusals and `[K]`-only chips verified. The host's four real sessions were observed read-only and confirmed untouched.

## Diagrams

Open [diagrams via `w` / `gogo view session-binding-ops --web`] — the set (in this folder, prebuilt layouts in `layouts.json`):

- `flow.mmd` — as-built control flow: P/K/R write the session NAME; every reader re-derives from it.
- `sequence.mmd` — adopting a retasked session end-to-end (picker → bindAction → rename → readers correct).
- `activity.mmd` — the session lifecycle: bound-authoring / bound-build / lingering / unbound and the P/R/K transitions.

## Before / after comparison

The plan-time as-is baseline is copied into [`before/`](./before/) (flow + sequence).

**flow — what changed:** before, the name was minted once (`sessionName`) with *no op anywhere that could change it* — a retasked pane produced four symptoms (shipped-card `●`, `· stalled`, an invisible build, an unkillable-from-the-board orphan) with killing as the only remedy. After, three cockpit ops write the same string the readers parse (`P` mints, `K` prunes, `R` renames via `RenameSessionUnique`), and a new meta reader (`ListSessionMeta`) feeds the unbound count and the anchor guard.

**sequence — what changed:** before, the incident sequence ends at the board narrating the old binding with no move offered. After, the same opening (self-guard spares the ship's pane, user retasks it) continues through `R`: picker → derived action → exact-target rename → the next tick shows `● developer` on the real item, the shipped `●` gone, and the cap counting the build again.

**Added (after only):** `activity.mmd` — the session lifecycle only became a state machine worth drawing once P/R/K existed to move sessions between its states.

## Knowledge updates

- `project-knowledge.md` — `## gogo overrides` entry for 0.32.0 (the session-name binding is cockpit-editable; the three ops + the unbound count).
- `tech-stack.md` — test-function count refreshed for 0.32.0.
- No upstreaming suggestions: the change is fully described by the CLI contract note and the plugin's own docs.

## Follow-ups & known limitations

- **D1=C follow-up:** a `--slug <kebab>` param on `/gogo:plan` so a board/plans-tab spawn is bound from birth — kills the unbound-at-birth class entirely (deserves its own item; it changes a skill contract).
- A `gogo session` verb / full-screen sessions panel (D5=B) if cross-repo orphan management becomes a habit.
- A picker for `l` (peek) when a card holds ≥2 sessions; the cross-repo same-slug cap over-count (known, separate limitation).
- The `renamer`/`killer` production wirings are unpinned by tests — a repo-wide pattern the round-1 reviewer noted and deliberately did not charge to this feature.

## Summary (TL;DR)

- **What shipped:** cockpit session-binding ops (0.32.0) — `P` start-and-attach a plan session, `K` kill a chosen session from the board, `R` re-assign a live session onto the item it really drives (one exact-target rename; every reader corrects itself), plus a visible unbound-session count.
- **Review verdict:** clean after 2 rounds — 7 findings total, all fixed, the first 5 *verified by mutation*.
- **Test verdict:** green — full race suite + real-tmux hands-on checks of every op, fixtures cleaned, real sessions untouched.
- **Follow-ups:** see above — chiefly the `--slug` spawn param (D1=C) that would eliminate unbound-at-birth sessions.
