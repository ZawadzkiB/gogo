# Decisions — feature `cli-orchestrator`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block. This is the audit
trail that lets the pipeline pause and resume safely.

These are open at plan time — each carries my recommendation. The orchestrator gates
them with you at acceptance; your answers fold into the plan before `/gogo:go`.

## D1 — Entry point: new `gogo run <slug>` subcommand vs extend the board's launch
- **Phase:** plan
- **Question:** Where does the Go loop live — a dedicated CLI subcommand, or folded
  into the board's existing `g` (go/resume) launch?
- **Options:**
  - A. **New `gogo run <slug>` subcommand** — the loop is a long-running foreground
    process; a separate entry keeps the board deterministic/fast and the loop
    independently testable. Board wiring becomes Slice 3 (FR13).
  - B. **Extend the board `g` launch** — one entry, but couples the millisecond
    board to a spawn/wait loop and complicates the TUI.
- **gogo recommends:** **A** — matches the NFR "the read path is deterministic and
  LLM-free / starts in milliseconds"; the board stays a selector, `gogo run` owns the
  loop. Board integration lands later as Slice 3.
- **Status:** RESOLVED → A (user, 2026-07-10; see RESOLVED block at end)

## D2 — How the loop detects a phase finished
- **Phase:** plan
- **Question:** Wait on the `claude -p` process exit, or poll `state.md`/`result.json`?
- **Options:**
  - A. **Wait for `claude -p` exit** (batch mode), then read `result.json` +
    `issues.json` — a precise, race-free completion signal; no polling.
  - B. **Poll the contract files** on a timer — needed only if phases ran as
    long-lived interactive sessions.
- **gogo recommends:** **A** — `-p` is print/batch mode (runs headless under
  permission-auto and exits); exit is the natural phase-done edge and warm continuity
  survives it (claude persists the session by uuid — spike-proven). Polling adds
  latency for no gain.
- **Status:** RESOLVED → A (user, 2026-07-10; see RESOLVED block at end)

## D3 — How a decision gate is presented to the human
- **Phase:** plan
- **Question:** When a phase writes `waiting-for-user`, does the loop auto-attach the
  human into an interactive resume session, or pause + notify and let them attach/
  `/gogo:resume` themselves?
- **Options:**
  - A. **Pause + notify + print the attach/resume command** (`tmux attach -t …` /
    `/gogo:resume <slug>`), loop waits for `state.md` to return runnable. Deterministic,
    no forced context switch; an opt-in `--attach` flag can auto-attach.
  - B. **Auto-attach immediately** — most ergonomic, but yanks the user into a session
    the moment any fork appears, even mid-batch.
- **gogo recommends:** **A** for the skeleton (reuses the existing attach path from
  `launch.go`; auto-attach as an opt-in flag) — the judgment + answer stay with the
  human via the *existing* attachable-tmux path (constraint 2), and the loop never
  guesses.
- **Status:** RESOLVED → A (user, 2026-07-10; see RESOLVED block at end)

## D4 — The in-session `/gogo:implement` path (`--resume` continuity)
- **Phase:** plan
- **Question:** How does the dev session run the `gogo-implement` skill **in-session**
  (no inner `gogo-developer` `Task`), so `--resume` continues the REAL worker?
- **Options:**
  - A. **A documented `--in-session` flag** on `/gogo:implement` (+ a one-line note in
    the skill/command) that runs the skill in-context. Small, explicit, testable; one
    command, one new flag.
  - B. **A prompt-suffix instruction** ("run in-context, don't spawn a Task") — no
    plugin change, but fragile and un-testable.
  - C. **A dedicated `/gogo:implement-session` command** — heavier; duplicates the
    command surface.
- **gogo recommends:** **A** — the skill front-matter already says it runs "for the
  orchestrator when it implements in-context"; a flag simply lets the CLI trigger that
  path over `-p`. This is the one plugin-side change Slice 1 needs (FR11) and it must
  land first in the checklist.
- **Status:** RESOLVED → A (user, 2026-07-10; see RESOLVED block at end)

## D5 — Where the phase-session registry lives
- **Phase:** plan
- **Question:** Under `.gogo/resources/cli/` (CLI-owned) or in the feature folder?
- **Options:**
  - A. **`.gogo/resources/cli/sessions/<slug>.json`** — the CLI's sanctioned write
    root; keeps orchestration bookkeeping out of the pipeline's contract surface.
  - B. **In `.gogo/work/feature-<slug>/`** — colocated with the feature, but that
    folder is pipeline state the CLI must **not** mutate (hard invariant + frozen
    contract) and would leak a CLI-only file into the consumer contract.
- **gogo recommends:** **A** — honors "the CLI never mutates pipeline state" and
  "writes confined to `.gogo/`"; the registry is bookkeeping (dev uuid, rounds, costs),
  not a phase artifact. A missing/garbled registry degrades to "first run".
- **Status:** RESOLVED → A (user, 2026-07-10; see RESOLVED block at end)

## D6 — Loop bounds: round cap + cost ceiling
- **Phase:** plan
- **Question:** What bounds the fix loop, and what happens at the bound?
- **Options:**
  - A. **~3 rounds on the same finding (mirrors the in-chat bound) + a per-feature
    cost ceiling** (sum `total_cost_usd` from each phase's `--output-format json`),
    both env-configurable (`GOGO_RUN_MAX_ROUNDS`, `GOGO_RUN_COST_CEILING`); hitting
    either **converts to a gate** (pause + surface to the human).
  - B. **Round cap only** — simpler, but a pathological loop can burn unbounded cost
    (the spike found a ~$0.13/session baseline — cheap per call, but unbounded rounds
    add up).
- **gogo recommends:** **A** — the round cap matches the existing pipeline bound and
  the cost ceiling is a cheap safety net the telemetry hands us for free; both gate
  rather than silently abort, so the human always decides.
- **Status:** RESOLVED

---

## RESOLVED (user, 2026-07-10) — plan-acceptance gate

The user accepted the plan **as-is**, taking the recommended option on every fork:

- **D1 → A** — new `gogo run <slug>` subcommand (board wiring deferred to Slice 3).
- **D2 → A** — wait on the `claude -p` process exit; no polling.
- **D3 → A** — gate = pause + notify + print the attach/resume command; opt-in `--attach`.
- **D4 → A** — a documented `--in-session` flag on `/gogo:implement` (FR11, lands first).
- **D5 → A** — registry at `.gogo/resources/cli/sessions/<slug>.json` (CLI-owned).
- **D6 → A** — ~3-round bound + cost ceiling, both env-configurable, both gate (never abort).

No changes to the plan resulted (recommendations = plan-as-written). `state.md` → `plan-accepted`; `/gogo:go` unlocked to build Slice 1.
