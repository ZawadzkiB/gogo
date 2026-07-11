# Decisions — feature `persistent-session-orchestrator`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block. This is the
audit trail that lets the pipeline pause and resume safely.

## D1 — Lock mechanism: lockfile+PID, live-session check, or both?
- **Phase:** plan
- **Question:** How does the one-owner lock decide a feature is *already owned by
  a live session*? A lockfile can go stale (crash); a pure live-session scan
  misses a session that hasn't registered a tmux name yet.
- **Options:**
  - A. **Lockfile + PID only** — `.gogo/resources/cli/locks/<slug>.lock` with the
    owner PID; liveness = signal-0. Simple, but a board-launched tmux racer whose
    PID we don't own is invisible, and a crash leaves a stale lock.
  - B. **Live-session scan only** — no file; "owned" iff a `gogo-*` tmux session
    matches the slug (`ListSessions` + `SessionMatchesSlug`). Catches the
    incident's racers, but headless `-p` runs (no tmux) are invisible and there's
    no record of *who* holds it.
  - C. **Both** — a lockfile (PID + UUID + tmux name + host + started-at) whose
    liveness is cross-checked against **both** signal-0 **and** a matching live
    `gogo-*` tmux session; either alive → live, both dead → stale/reclaimable.
- **gogo recommends:** **C** — the incident was a chat + board-launched tmux
  sessions racing; only the cross-check catches the tmux racer *and* records the
  owner *and* self-heals a stale lock. Reuses `SessionMatchesSlug` (exact parse,
  TEST-005) — no new attribution logic.
- **Status:** RESOLVED (user, 2026-07-11; see RESOLVED block at end)

## D2 — `gogo go` vs reworking `gogo run` (naming / deprecation)
- **Phase:** plan
- **Question:** The subcommand today is `gogo run` (per-phase semantics). The new
  behaviour runs the `/gogo:go` skill. Keep the name, or rename?
- **Options:**
  - A. **Rename to `gogo go`** (matches the `/gogo:go` skill it launches), add
    `gogo plan`, keep `gogo run` as a **deprecated alias** (one version) that
    prints a notice and forwards.
  - B. **Keep `gogo run`** and just change its behaviour — no new verb.
  - C. **Both `gogo run` and `gogo go` as equals**, indefinitely.
- **gogo recommends:** **A** — the verb should match the skill it runs (`go`↔`/gogo:go`,
  `plan`↔`/gogo:plan`); `run`'s per-phase meaning is gone, so keeping it as the
  primary name misleads. The alias protects muscle-memory/scripts for a version.
- **Status:** RESOLVED (user, 2026-07-11; see RESOLVED block at end)

## D3 — Fate of `contract.Route` + the per-phase orchestrator
- **Phase:** plan
- **Question:** With the Go loop gone, `contract.Route` (+ `route_test.go`) and
  the per-phase spawn logic have no caller. Delete or park?
- **Options:**
  - A. **Delete** `route.go` / `route_test.go` and the loop; keep + extend only
    `registry.go`. The skill is the single routing source.
  - B. **Park** them (unused, still tested) for a possible multi-model future
    where Go might route per phase again.
  - C. **Delete the loop, keep `Route`** as a public helper in `contract` in case
    a consumer wants it.
- **gogo recommends:** **A** — a second copy of the routing rule is *exactly* the
  drift bug this slice removes; multi-model will also run through the skill via the
  persistent session, so a Go router is unlikely to return. Dead, tested code
  invites the drift back. Re-derive later if ever needed. (Minor sub-fork: keep
  the `internal/orchestrator` package name to minimize import churn, vs rename to
  `internal/session` — recommend **keep the name**.)
- **Status:** RESOLVED (user, 2026-07-11; see RESOLVED block at end)

## D4 — Headless `-p` vs attachable-tmux for the persistent session (and how a gate surfaces)
- **Phase:** plan
- **Question:** The persistent session hits decision gates + the UAT gate. In
  headless `-p` it **exits** at a gate (can't ask interactively); an attachable
  tmux session could answer live but can't cleanly capture the exit/JSON and is
  the shape that leaked.
- **Options:**
  - A. **Headless `-p` foreground default** (blocks, exit = leg-done, JSON
    captured); a gate is surfaced as a printed hint + parked `state.md`, resolved
    out-of-band (`/gogo:resume <slug>`, or re-run `gogo go` to resume warm).
  - B. **Attachable tmux default** — user attaches to answer gates live in the
    warm session; CLI polls `state.md` + tmux liveness; no clean JSON/exit; needs
    the reaper (leaks otherwise).
  - C. **A default + an `--attach` option** — `-p` foreground by default; with
    `--attach`, on a gate resume the **same** persistent session interactively in
    a reaped-at-close tmux pane (`claude --resume <uuid>`) so the user answers
    live and warm, mirroring today's `gogo run --attach` / `ResumeIntent`.
- **gogo recommends:** **C** — the `-p` default is spike-proven, cost-lean, and
  gives a race-free exit signal; `--attach` preserves the live-answer ergonomics
  for those who want them, without making the leak-prone path the default. The
  reaper (FR8) covers the `--attach` tmux session.
- **Status:** RESOLVED (user, 2026-07-11; see RESOLVED block at end)

## D5 — Who reaps at ship / completion?
- **Phase:** plan
- **Question:** `/gogo:done` ships a feature but never kills the session that
  drove it (the incident's root cause). `gogo done` is out of scope this slice —
  so where does kill-at-ship live?
- **Options:**
  - A. **`gogo sweep` + opportunistic reap** — `gogo go`/`gogo plan`/the board
    reap the target's session when they see it's terminal, and `gogo sweep` is the
    on-demand + backstop reaper for orphans and shipped features.
  - B. **A `/gogo:done` skill hook** — the ship skill runs a reap step
    (kills the tracked session) as it ships.
  - C. **A Go `gogo done` wrapper** that ships and reaps (pulls `done` into the
    CLI — but that's the deferred gate/state-CLI slice).
- **gogo recommends:** **A** — keeps `/gogo:done` and the skills untouched (FR10),
  needs no new CLI gate command (deferred), and `gogo sweep` is the durable
  orphan-reaper the incident demands anyway. A `/gogo:done` hook (B) is noted as a
  later refinement once we want *immediate* kill-at-ship rather than
  next-sweep/next-launch.
- **Status:** RESOLVED (user, 2026-07-11; see RESOLVED block at end)

## D6 — Lock contention: refuse vs take-over as the default
- **Phase:** plan
- **Question:** When a **live** owner already holds the lock and the user runs
  `gogo go <slug>` again, what happens by default?
- **Options:**
  - A. **Refuse by default**, offer `--takeover` (seize + reap the prior) — safe;
    the user opts into stealing.
  - B. **Take over by default** — assume the newest invocation wins; auto-reap the
    prior.
- **gogo recommends:** **A** — the incident was *silent* double-driving; refusing
  loudly (with how to attach or `--takeover`) is the safe default. Take-over
  should be a deliberate, explicit act.
- **Status:** RESOLVED (user, 2026-07-11; see RESOLVED block at end)

---

## RESOLVED (user, 2026-07-11) — plan-acceptance gate

Accepted the Slice-1 plan **as-is**, taking each fork's recommendation:

- **D1 → C** — lock = lockfile (PID+UUID+tmux+host+started-at) cross-checked against BOTH signal-0 and a live `gogo-*` tmux session (catches the board racer, records owner, self-heals stale).
- **D2 → A** — rename `gogo run` → `gogo go` (+ add `gogo plan`); `gogo run` = deprecated alias for one version.
- **D3 → A** — DELETE `contract.Route` + `route_test.go` + the per-phase `orchestrator.Run` loop (the skill is the single router); keep the `internal/orchestrator` package name.
- **D4 → C** — headless `-p` foreground default + `--attach` option (resume the same warm session in a reaped-at-close tmux for live gate answers).
- **D5 → A** — `gogo sweep` + opportunistic reap; a `/gogo:done` reap-hook is a later refinement.
- **D6 → A** — refuse-by-default on a live-owner collision, `--takeover` to seize+reap.

`state.md` → `plan-accepted`; `/gogo:go` unlocked to build the foundation.

---

## D7 — Blocked hands-on check: the true end-to-end (real `claude -p` driving `/gogo:go`)
- **Phase:** test (round 1)
- **Question:** Test round 1 (TEST-001, major/P1, needs-user-decision) reports
  everything unattended-runnable is green — `gofmt`/`go vet`/`go test -race`
  (149/149, incl. the hermetic stub-claude e2e `TestGoE2EStubClaude`), plus every
  safe hands-on CLI path (version/help, `gogo sweep --dry-run` correctly sparing
  the live in-flight `gogo-go-persistent-session-orchestrator` tmux session, the
  REV-001 path-traversal refusal, the `gogo run` deprecation-forward, and a
  throwaway-fixture status-gate check for `awaiting-uat`/`waiting-for-user`/`shipped`).
  What has **not** run is the true end-to-end: a real `gogo go <slug>` driving a
  real `claude -p "/gogo:go <slug>"` through implement → review/test (nested
  `Task`) → report on a live model. The tester was explicitly instructed not to
  trigger this (non-deterministic, billable, and this feature's own slug is
  mid-pipeline right now) — this is a hands-on user-decision gate, never a
  silent skip.
- **Options:**
  - A. **Run it by hand once, now** — you (or I, with your go-ahead) run
    `gogo go <some-throwaway-runnable-slug>` on a scratch/disposable feature (not
    this in-flight one) and confirm it drives implement→review→test→report
    correctly end-to-end; then resume phase ④ to close TEST-001 as verified.
  - B. **Explicitly skip for this round** — accept the hermetic stub-claude e2e
    (`TestGoE2EStubClaude`) as sufficient proof for 0.15.0's ship, close TEST-001
    as `wontfix`/accepted-risk, and advance to ⑤ report. (You could still run the
    real end-to-end later, dogfooding on the next real feature — this literal
    feature's own `gogo go` run, already in flight, is itself a live real-world
    instance of the new code path, though not one gogo-tester independently
    verified as passing.)
  - C. **Something else** — tell me what you'd like to verify or how.
- **gogo recommends:** **B is reasonable given the circumstance** — this very
  session (`gogo go persistent-session-orchestrator`) IS a live real-world run of
  the exact code path in question (launch-or-resume, lock, registry, exit
  classification), so the true e2e is already being dogfooded end-to-end as we
  speak, just not as an independent tester-driven check with a disposable slug.
  If you want the stronger, cleaner signal (a tester-observed run on a
  throwaway slug, isolated from this in-flight one), **A** is the rigorous
  choice and cheap (one extra billable session). Either is defensible; this is
  your call, not gogo's to make silently.
- **Status:** RESOLVED (user, 2026-07-11) — see below.

### RESOLVED (user, 2026-07-11) — D7 → light real smoke

The user chose a **light real smoke** (a scoped variant of A): run
`gogo plan <throwaway>` **once** — a single cheap live `claude -p "/gogo:plan …"`
leg — rather than a full `gogo go` pipeline. Rationale: the change under test is
the **CLI session-manager** (the `/gogo:go` skill itself is unchanged, FR10), so a
real launch of *any* skill leg exercises exactly the new code — `Acquire` (lock) →
`ResolveInvocation` (fresh `--session-id`) → `RunPhase` (real
`exec.Command("claude", …)` + JSON-envelope parse) → `classifyExit` (reads
`state.md`) → lock release + registry write — at a fraction of a full pipeline's
cost/time.

**Correction to the recommendation's premise:** this chat is the **in-chat
`/gogo:go` plugin path**, *not* the `gogo` CLI binary, so the new CLI code
(`LaunchOrResume` / lock / registry / exit-classification) is **not** being
dogfooded by this session; the separate live `gogo-go-…` tmux session is unrelated.
Hence a real CLI smoke is genuinely warranted. TEST-001 stays open until the smoke
is run and the orchestrator verifies the observed wiring, then it closes `verified`
and the pipeline advances to ⑤.
