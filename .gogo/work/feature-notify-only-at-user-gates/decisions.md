# Decisions — feature `notify-only-at-user-gates`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block. This is the
audit trail that lets the pipeline pause and resume safely.

<!-- Template for each decision — copy and fill:

## D1 — <short title>
- **Phase:** <plan | implement | review | test>
- **Question:** <the fork, stated plainly>
- **Options:**
  - A. <option> — <trade-off>
  - B. <option> — <trade-off>
- **gogo recommends:** <A / B> — <one-line why>
- **Status:** OPEN        # OPEN | RESOLVED

### RESOLVED (user, <YYYY-MM-DD>)
<the decision, in the user's terms>
-->

## D1 — An unknown or absent `notification_type`: quiet or loud?
- **Phase:** plan
- **Question:** The ten `notification_type` values were read out of Claude Code
  2.1.220's binary. A future build may send a type the classifier has never seen,
  and an older/other build may send no `notification_type` at all. What should the
  hook do then?
- **Options:**
  - A. **Gate-conditional** — fall through to the `.gogo/work/*/state.md` scan and
    notify only if a gogo gate is open. Quiet by default; a genuinely new
    "user needed" type is missed until the allowlist learns it (`GOGO_NOTIFY_DEBUG=1`
    surfaces it by name the first time it arrives).
  - B. **Always notify** (fail-open, today's behaviour for everything). Never misses
    a new blocking signal; a new *noisy* type reintroduces exactly the spam this
    feature is removing, silently.
- **gogo recommends:** **A** — the reported defect is over-notification, and A's
  failure mode is recoverable (the trace names the unknown type, the allowlist gains
  a line) while B's failure mode is the bug coming back unnoticed. FR2's
  `*permission*` hedge already catches the highest-cost miss, and `GOGO_NOTIFY_LEVEL=all`
  restores fail-open in one env var.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
A — gate-conditional. An unknown or absent `notification_type` falls through to the
gate scan and pings only when a gogo gate is open.

## D2 — `idle_prompt`: always, or only when a gogo item is at a gate?
- **Phase:** plan
- **Question:** `idle_prompt` ("Claude is waiting for your input") fires once per
  idle window whenever the prompt sits untouched past the threshold — which happens
  both when gogo is parked at a real gate and when Claude simply finished answering
  an ordinary question and you walked away. Ping on both, or only at gogo gates?
- **Options:**
  - A. **Always notify.** It genuinely means a human is being waited on, it is
    low-volume (once per idle window, not per event), and it still pings when the
    orchestrator paused in prose without having written `state.md` yet — which
    `non-functional-requirements.md` warns is a real failure mode ("skipped on **all
    three** of its live runs"). Cost: a ping after ordinary non-gogo chat.
  - B. **Gate-conditional**, same treatment as `agent_completed`. Strictly quieter
    and strictly on-mission for a *gogo* hook; but it makes the last-resort signal
    depend on an LLM having written `state.md` on time.
- **gogo recommends:** **A** — `idle_prompt` is not the spam source (that is
  `agent_completed`, one per subagent return), and it is the only signal that still
  reaches the user when a phase skill forgets its status write. Silencing it buys
  almost no quiet and removes the safety net.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
A — always notify on `idle_prompt`; it stays the last-resort safety net.

## D4 — REV-001: a parked gate elsewhere re-enables per-hand-off pings
- **Phase:** implement (from review round 1, REV-001, major)
- **Question:** The gate scan is repo-wide, so while ANY work item sits at a gate
  (`feature-interactive-plan-review` is parked at `awaiting-plan-acceptance` in this
  repo right now), every `agent_completed` hand-off pings — reproducing the reported
  symptom and falsifying the plan's acceptance signal ("exactly one notification per
  run"). The plan anticipated this: A4 was parked "only if spam survives the
  classifier. It survived." How should the gate-conditional branch be scoped?
- **Options:**
  - A. **Edge-latch** — ping only when the gate set GAINS a member vs the last
    notified set, remembered in a tiny `.gogo/.notify-gates` file (rewritten each
    invocation; unreadable/unwritable degrades to today's behaviour). A parked gate
    pings once, then stays silent; works identically for interactive and
    CLI-launched sessions. Trade-off: adds one small mutable state file under
    `.gogo/` (the A4 territory the plan deferred).
  - B. **Scope to the running session's slug** (reviewer's pick) — only the gate of
    the feature THIS session runs pings. No mutable state. Trade-off: the hook can
    derive the slug only for CLI/tmux-launched sessions (`gogo-go-<slug>`); a plain
    interactive `/gogo:go` has no slug source, so it must fall back to repo-wide
    (symptom persists exactly where this run is happening) or to silent (misses its
    own gate; idle_prompt still covers it).
  - C. **Accept + reword** — keep repo-wide ("a ping = something in this repo needs
    you"), amend the plan's acceptance signal. No code change; per-hand-off pings
    remain the norm in any active tree.
- **Safety note (either way):** blocking prompts and `idle_prompt` always notify
  regardless of this choice, so no real gate can go fully unnoticed.
- **gogo recommends:** **A** — it is launch-mode independent (B's slug is
  underivable for exactly the interactive sessions this command runs), keeps the
  acceptance signal true (one ping per newly-opened gate), and its failure mode
  degrades to today's behaviour, never to silence.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
A — edge-latch. The gate-conditional branch pings only for gates that NEWLY opened
since the last notified set, remembered in `.gogo/.notify-gates` (rewritten each
gate-class invocation, pruning closed gates; unreadable/unwritable degrades to
today's fire-on-open-gate behaviour, never to silence).
*(Round-3 note: the seen-file moved to `.gogo/resources/notify/gates` — REV-012,
the original root path was untracked-but-not-ignored in wired repos.)*

## D3 — Version bump shape
- **Phase:** plan
- **Question:** `0.31.0` (feature-shaped: new env knobs + selftest) or `0.30.1`
  (pure fix, per the 0.25.1 precedent)?
- **gogo recommends:** **0.31.0**
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
0.31.0.

## D5 — Two open empirical assumptions from plan.md are still unresolved (blocked hands-on check)
- **Phase:** test
- **Question:** plan.md's "Risks and unknowns" named two assumptions meant to be
  settled empirically via FR6's debug trace: (a) which `notification_type` values
  Claude Code 2.1.220 actually **emits live** (the plan's ten values came from a
  static `strings` scan of the binary, never an observed session), and (b) whether
  an ordinary **main-session** permission prompt carries a `notification_type` at
  all (only `worker_permission_prompt` appears in the binary; the plan called this
  "recorded as an assumption ... resolved empirically during implement, before the
  classifier is frozen" — it was not resolved then). Phase ④ testing exhaustively
  proved the classifier's behaviour against **synthetic** stdin payloads (the
  `--selftest` 43/43 table under both bash 3.2 and 5.3, a full `/gogo:go`
  notification-sequence dogfood replay, every FR2-FR5 edge case, and a live probe
  against this repo's two real open gates) — but this test session runs headless
  inside an agent shell with **no interactive terminal**, so it can trigger neither
  a genuine `idle_prompt`/`agent_needs_input` wait nor a real main-session
  permission gate. Re-running the plan's static `strings -a` scan against the same
  installed 2.1.220 binary reproduced the identical ten `notificationType:"..."`
  literals and found no literal `"needs your permission to use"` string — this
  corroborates the plan's snapshot but proves only what the binary *can* send, not
  what a live session *actually* sends.
- **Options:**
  - A. **User runs one live Claude Code session** (ideally `/gogo:go`) with
    `GOGO_NOTIFY_DEBUG=1` exported and 0.31.0 installed, watches the stderr trace
    across a normal run (a gate ping, a phase hand-off, an idle wait, and — if
    convenient — a real permission prompt), and reports the observed `type=`
    values back. Resume at ④ to fold the finding into `test/issues.json`.
  - B. **Accept and skip.** D1's gate-conditional default plus FR2's `*permission*`
    substring hedge already bound the highest-cost miss (any type whose name
    contains "permission" always notifies), and `GOGO_NOTIFY_LEVEL=all` is a
    standing escape hatch. Mark TEST-002/TEST-003 `wontfix` by acceptance.
  - C. **Defer** — note it in `test-strategy.md` as a standing dogfood follow-up
    rather than gating this feature; the classifier already degrades safely (D1)
    whichever way this resolves.
- **gogo recommends:** **A if convenient** (one real run settles a genuine unknown
  before wider rollout) — otherwise **B**, since FR2's hedge already bounds the
  downside and every other classifier path (43 cases) is proven.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
B — accept and skip. TEST-002/TEST-003 closed wontfix-by-acceptance: the classifier
is fully proven against synthetic payloads, FR2's `*permission*` hedge plus D1's
gate-conditional default bound the miss risk, and the user's own UAT of 0.31.0 with
`GOGO_NOTIFY_DEBUG=1` doubles as the live observation (the trace names any type the
classifier has never seen).
