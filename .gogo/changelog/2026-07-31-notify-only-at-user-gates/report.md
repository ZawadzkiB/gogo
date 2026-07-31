# notify-only-at-user-gates — shipped 2026-07-31 (0.31.0)

**gogo's phone/desktop ping now fires only when a work item actually needs you.** Since its first commit, the Notification hook pushed "gogo needs your input" on *every* Claude Code notification — one buzz per subagent hand-off, 4-6 per `/gogo:go` run, none of them a real gate. 0.31.0 rebuilds `hooks/notify.sh` into a classifier: a full pipeline run now produces **exactly one notification, at the UAT gate, naming the feature and the gate** (`feature-x · awaiting-uat`).

**What was implemented:**

- **A notification classifier** in `hooks/notify.sh`: blocking prompts (`agent_needs_input`, `worker_permission_prompt`, `idle_prompt`, anything `*permission*`) always notify with Claude's own message; lifecycle noise (`elicitation_*`, `auth_success`, `computer_use_*`, `push_notification`) never does; `agent_completed` and unknown types notify **only when a `.gogo/work` item newly arrives at a user gate**.
- **The gate scan mirrors the CLI contract** — `awaiting-plan-acceptance` (with a written plan) · `waiting-for-user` · `awaiting-uat`, exactly `contract.WaitingForInput()`, pinned by a three-source Go anti-drift guard.
- **An edge-latch** (`.gogo/resources/notify/gates`): an already-pinged parked gate stays silent instead of re-buzzing on every event; closed gates prune; a missing/unwritable latch degrades to fire-on-open-gate, never to silence.
- **Knobs + self-verification:** `GOGO_NOTIFY_LEVEL` (`gates` default · `all` legacy · `off`), `GOGO_NOTIFY_DEBUG=1` (one stderr trace line per event), `GOGO_NOTIFY_DRYRUN=1`, and `bash hooks/notify.sh --selftest` (44 cases, sends nothing) — also run in CI by `cli/notify_hook_test.go`.
- Hardened delivery: argv-form osascript (an AppleScript injection found in review is gone), `curl --data-raw`, no stderr leaks without a tty or with a read-only `.gogo/`.

**Key decisions (one line each):** D1 unknown types are gate-conditional, not fail-open · D2 `idle_prompt` always pings (the last-resort safety net) · D3 version 0.31.0 · D4 the parked-gate re-ping is fixed by an edge-latch, not slug-scoping (launch-mode independent) · D5 the two live-session observations were accepted-and-skipped (the first `GOGO_NOTIFY_DEBUG=1` run doubles as them).

**Review:** APPROVE after 3 adversarial rounds — 19 findings, every fix exploit- or mutation-verified, none open. **Test:** green — selftest on macOS bash 3.2 + bash 5.x, full `go test -race`, and an E2E dogfood replay proving the exactly-one-ping acceptance signal.

Full audit trail: [.gogo/work/feature-notify-only-at-user-gates/](../../work/feature-notify-only-at-user-gates/) (plan, 5 decisions, 3 review rounds, test round, report bundle with before/after diagrams).
