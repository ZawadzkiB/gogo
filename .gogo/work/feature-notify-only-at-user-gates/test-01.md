# Test round 1 — feature `notify-only-at-user-gates`

**Date:** 2026-07-31 · **Round:** 1 · **Verdict:** NOT green — one blocked hands-on
check gates the phase (decisions.md D5), plus one fixable minor bug (TEST-001).

## What shipped (as-built, per plan.md FRs 1-8 + decisions D1-D4)

`hooks/notify.sh` is now a notification classifier: blocking prompts
(`agent_needs_input` / `worker_permission_prompt` / `idle_prompt` / any
`*permission*`) always notify; lifecycle noise never notifies;
`agent_completed` + unknown/absent types notify only when a `.gogo/work` item
**newly** arrives at a user gate (D4 edge-latch, `.gogo/resources/notify/gates`).
Knobs: `GOGO_NOTIFY_LEVEL` (gates/all/off), `GOGO_NOTIFY_DEBUG=1`,
`GOGO_NOTIFY_DRYRUN=1`. `bash hooks/notify.sh --selftest` = 43 cases. Two Go
guards in `cli/notify_hook_test.go`. Review ended APPROVE after 3 rounds (19
findings, all verified/fixed/wontfix in `review/issues.json`).

## Suites (existing) — all green

| Suite | Command | Result |
|---|---|---|
| Bash selftest, bash 5.3 | `bash hooks/notify.sh --selftest` under `/opt/homebrew/bin/bash` (GNU bash 5.3.15) | **43/43 passed**, exit 0 |
| Bash selftest, bash 3.2 | `/bin/bash hooks/notify.sh --selftest` (the shipped macOS bash) | **43/43 passed**, exit 0 |
| `gofmt -l cli` | — | clean (no output) |
| `go vet ./...` | `cli/` | clean |
| `go test -race -count=1 ./...` | `cli/` (13 packages) | **all `ok`** — `.` 17.0s, `internal/config` 3.0s, `internal/contract` 6.1s, `internal/diagram` 2.2s, `internal/diagram/mermaidascii` 1.7s, `internal/launch` 2.9s, `internal/orchestrator` 6.1s, `internal/pages` 2.1s, `internal/plans` 4.3s, `internal/projects` 6.0s, `internal/trash` 4.5s, `internal/tui` 23.2s, `internal/textfmt` (no test files) |
| `TestNotifyHookSelftest` / `TestNotifyHookGateStatusesMatchContract` | `go test -race -run TestNotifyHook -v .` | both **PASS** individually |

**Environment note:** bash 5.3 was not present on this host at test start (only
`/bin/bash` 3.2.57, and `bash` on `PATH` resolved to the same binary — no
Homebrew `bash` formula was installed). Installed it via `brew install bash`
(pulled `bash 5.3.15` + its `json-c` dependency) specifically to fulfil this
round's "under bash 5.3 AND /bin/bash 3.2" requirement; both are now green.
Flagging as a transparency note, not an issue — the package is a normal,
reversible Homebrew install, unrelated to the repo.

## Hands-on / E2E dogfood — the plan's acceptance signal

Built a scratch tree outside the repo
(`/private/tmp/.../scratchpad/notify-e2e/`) with a `feature-scratch-dogfood`
work item, then replayed a full `/gogo:go` notification sequence against the
**real** `hooks/notify.sh` (never the installed 0.30.0 plugin cache) with
`GOGO_NOTIFY_DEBUG=1` and `GOGO_NTFY_TOPIC=` unset on every invocation:

| Step | `state.md` status | Event sent | Trace | Result |
|---|---|---|---|---|
| 1 | `implementing` | `agent_completed` | `class=gate verdict=silent gates=0 channels=none` | silent ✓ |
| 2 | `reviewing` | `agent_completed` | `class=gate verdict=silent gates=0 channels=none` | silent ✓ |
| 3 | `testing` | `agent_completed` | `class=gate verdict=silent gates=0 channels=none` | silent ✓ |
| 4 | `awaiting-uat` | `agent_completed` | `class=gate verdict=notify gates=1 channels=banner` | **notify** ✓ — a real macOS banner fired (delivery evidence; `GOGO_NTFY_TOPIC` stayed unset so no phone push reached the user) |
| 5 | `awaiting-uat` (repeat) | `agent_completed` | `class=gate verdict=silent gates=0 channels=none` | silent ✓ — D4 latch |

The latch file after step 4 read exactly `feature-scratch-dogfood ·
awaiting-uat`; step 5's silence confirms the edge-latch (D4) suppresses a
re-ping for the same still-open gate. **Matches the plan's acceptance signal
byte-for-byte.** Directly checked the actual message text via the internal
`gogo_notify_decide()` function (sourced, functions-only): the gate ping's
message is `feature-msgcheck · awaiting-uat` — names the feature and the gate,
as FR4 requires.

### Additional probes (all via the real hook, `GOGO_NTFY_TOPIC=` unset)

| Probe | Trace | Result |
|---|---|---|
| `worker_permission_prompt` (exact Claude-Code-shaped payload: `session_id`/`transcript_path`/`cwd`/`hook_event_name`/`title` all present) | `class=notify verdict=notify gates=0 channels=banner` | notify ✓, exit 0 |
| `auth_success` | `class=silent verdict=silent gates=0 channels=none` | silent ✓ |
| `GOGO_NOTIFY_LEVEL=off` + `agent_needs_input` | `class=off verdict=silent` | silent regardless of type ✓ |
| `GOGO_NOTIFY_LEVEL=all` + `auth_success` (lifecycle noise) | `class=all verdict=notify channels=banner` | notify ✓ — legacy fire-on-everything, byte-for-byte |
| Unknown type, no gate open | `class=gate verdict=silent` | silent ✓ |
| Unknown type, gate open | `class=gate verdict=notify gates=1 channels=banner` | notify ✓ |
| No `.gogo` at all | `class=gate verdict=silent` | silent, exit 0, no crash ✓ |
| `state.md` with a garbage status (`¯\_(ツ)_/¯ not-a-real-status`) | `class=gate verdict=silent` | silent, exit 0, no crash ✓ |
| Garbage payload (`this is not json {{{`) | `type=(none) class=gate verdict=silent` | silent, exit 0, no crash ✓ |
| Authoring carve-out: `awaiting-plan-acceptance`, no `plan.md` | `verdict=silent` | silent ✓ (unwritten plan is not a gate) |
| Same, then a real `plan.md` written (2 `## ` sections) | `verdict=notify gates=1 channels=banner` | notify ✓ (gate now real) |

**Every probe exited 0.** No case crashed or hung, including garbage JSON,
missing `.gogo`, unreadable files, and every notification_type combination
tested.

### Live real-repo probe (REV-001 regression check)

This repo currently has **two real open gates**
(`feature-interactive-plan-review` and `feature-session-binding-ops`, both
`awaiting-plan-acceptance` with written plans) — the exact scenario REV-001
found broken in round 1. Ran one deliberate, dry-run probe against the real
repo root (`CLAUDE_PROJECT_DIR=<repo>`, `GOGO_NOTIFY_DRYRUN=1` so nothing sent,
`GOGO_NTFY_TOPIC=` unset):

- First `agent_completed`: `gates=2 verdict=notify channels=banner` — the latch
  file wrote both gate lines.
- Immediate repeat: `verdict=silent` — both already latched.

This is the D4 fix working exactly as designed against the real, currently-open
gates. **Cleanup:** deleted the probe-created
`.gogo/resources/notify/gates` file afterward
(`find .gogo/resources/notify -name gates -delete`); confirmed via
`git check-ignore` that this path is gitignored (`.gitignore:17`), so
`git status` shows no trace of the probe. No repo file was left modified.

### Two open plan assumptions — could not be resolved empirically here

plan.md's "Risks and unknowns" named two things meant to be settled
empirically via FR6's debug trace. Both require a **real** Claude-Code-triggered
Notification event (an actual idle wait, a genuine permission prompt), which
this headless agent-shell test session has no way to produce — every check
above used a synthetic stdin payload, proven exhaustively, but never an
event Claude Code itself generated.

- **(a) Which `notification_type` values Claude Code 2.1.220 sends live.**
  Re-ran the plan's static verification instead: `strings -a` against the
  installed `2.1.220` binary (unchanged version, re-verified via
  `claude --version`) reproduces the identical ten `notificationType:"..."`
  literals the plan found — no drift detected — but this proves only what the
  binary *can* emit, not what a live session actually does.
- **(b) Whether an ordinary main-session permission prompt carries a
  `notification_type` at all.** `strings -a` again finds no literal
  `"needs your permission to use"` tied to notification metadata (only
  `worker_permission_prompt` appears), consistent with the plan's assumption —
  but an absent string in a binary dump doesn't prove runtime behaviour.

**Disposition: blocked / needs-user-decision** — filed as **TEST-002** and
**TEST-003** in `test/issues.json`, with a new **D5** in `decisions.md` laying
out the options. Per the `gogo-test` skill, a blocked hands-on check is a user
decision, never a silent skip — **only the user may decide to skip it.**
Concrete unblock: run one real Claude Code session (ideally `/gogo:go`) with
`GOGO_NOTIFY_DEBUG=1` exported after installing 0.31.0, and report the observed
`type=` trace values back (including, if convenient, from a real permission
prompt) — then resume at phase ④ to close these out.

## New finding — TEST-001 (fixable, minor)

**Notification message extraction appends a spurious trailing space to every
non-empty Claude-authored message.** `gogo_notify_decide()`'s
`msg="$(gogo_json_field "$payload" message | tr '\n\t' '  ')"` line: `jq -r`
(and the no-jq sed fallback) print the extracted value with a trailing
newline; piping that through `tr '\n\t' '  '` converts that trailing newline
into a trailing *space* instead of removing it (tr is 1:1, so no newline
survives for the outer `$(...)` to strip). Reproduced directly against the
shipped functions on **both** the jq path and a PATH-narrowed no-jq path:
`"pick an option"` (14 chars, confirmed via `wc -c`) round-trips as
`"pick an option "` (15 chars) both ways; same for the permission-prompt
fixture (37 → 38 chars). Hits every FR2 `notify`-class message and
`level=all` — the paths the plan calls "verbatim." Invisible to the shipped
`--selftest` because its message assertions are substring matches, which a
trailing space can never fail — 43/43 still passes with the bug present.
Fix: capture the raw value first (`raw="$(gogo_json_field ...)"` — command
substitution already strips trailing newlines), *then* run the internal
`tr '\n\t' '  '` normalization on `$raw`; add a selftest case asserting an
**exact** string, not a substring, so this class of defect can't regress
silently. Full detail and byte-level repro in `test/issues.json` TEST-001.

## Done-bar assessment (per `test-strategy.md`)

- Changed command(s) run end-to-end on a scratch repo — **yes** (dogfood
  above, byte-for-byte match to the plan's acceptance signal).
- Artifacts conform to contract; bad inputs rejected not propagated — **yes**
  (every degradation case: no `.gogo`, garbage status, garbage payload, no
  `plan.md` — all silent, all exit 0).
- Enumerations in sync; version bumped; portability intact — **yes**
  (`TestNotifyHookGateStatusesMatchContract` passes; `plugin.json` and
  `cli/main.go` both `0.31.0`, `TestVersionMirrorsPlugin` passes;
  `docs/flow.md` and `skills/gogo/SKILL.md` updated per the plan's checklist;
  `testing-tools.md`/`docs/architecture.md` reconcile is correctly deferred to
  phase ⑤ per the plan's own changes-checklist item 7, not a gap).
- Review clean + tests green → verdict feeds ⑤ — **review is clean; tests are
  NOT fully green** — one blocked hands-on check (TEST-002/TEST-003) is a
  standing user gate, per the skill's explicit rule: *"if any relevant hands-on
  check was blocked, the done-bar is not met."*

## Verdict

**NOT green.** Every suite is green (bash ×2 versions, `gofmt`/`vet`/`go test
-race` full suite, both Go notify guards) and every hands-on check that
*could* run in this environment passed, including the full acceptance-signal
dogfood replay and a live regression check against this repo's two real open
gates. Blocking this round from a clean hand-off:

- **TEST-001** (minor, fixable) — spurious trailing space in the notification
  message text; agent-fixable in the next implement round.
- **TEST-002** (minor, needs-user-decision, blocked hands-on) — which
  `notification_type` values Claude Code emits *live* could not be observed
  from this headless session.
- **TEST-003** (minor, needs-user-decision, blocked hands-on) — whether a
  main-session permission prompt carries a `notification_type` at all could
  not be observed from this headless session.

`decisions.md` D5 records the fork and options for TEST-002/TEST-003.
`state.md` is set to `waiting-for-user` (`resume: test`, `open-decision: D5`)
per the skill's "blocked hands-on → user decision gate" rule — **no
`phase-done` was emitted this round.**
