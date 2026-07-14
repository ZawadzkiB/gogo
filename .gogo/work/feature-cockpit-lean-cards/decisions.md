# Decisions — feature `cockpit-lean-cards`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block. This is the
audit trail that lets the pipeline pause and resume safely.

Both forks below were **pre-confirmed with the user before planning** — they are recorded
here as settled, not surfaced as open questions.

## D1 — When does the agent chip show?
- **Phase:** plan
- **Question:** Should the green `● <agent>` chip appear on every in-progress card, or
  only when a session is actively live on it?
- **Options:**
  - A. Always show the current phase's agent — simpler, but a chip on an idle/parked card
    over-claims "someone is working right now."
  - B. Show it **only when `hasLiveSession(f.Slug, m.sessions)` AND `!f.WaitingForInput()`**
    (short lowercase label; idle in-progress cards show status alone) — the chip means
    "an agent is on this *right now*," reusing the existing liveness decoupling.
- **gogo recommends:** B — liveness is already a first-class, decoupled signal (the green
  `●` dot); gating the chip on it keeps the board honest.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-13)
B. Agent chip only when live and not a gate. Labels: analyst / developer / reviewer /
tester / reporter.

## D2 — Keep the `1..9` gate number-key shortcut?
- **Phase:** plan
- **Question:** The number keys `1..9` (`jumpToGate`/`gateNumberKey`) focused + read a
  strip gate. With the strip removed, keep them or drop them?
- **Options:**
  - A. Keep the number keys as a headless jump-to-gate — but with no strip they answer an
    invisible list, and they consume `1..9`.
  - B. **Remove** `jumpToGate`, `gateNumberKey`, the number-key branch in `updateBoard`,
    and the `1–N answer gate` help text. The left-border cue + normal arrow navigation
    reach a gate.
- **gogo recommends:** B — the shortcut only made sense as the strip's answering surface.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-13)
B. Remove the number-key shortcut and its `?`-help mention.
