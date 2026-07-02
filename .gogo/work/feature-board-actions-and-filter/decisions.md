# Decisions — feature `board-actions-and-filter`

## D1 — Interaction model: modes vs action keys

**Question:** view/manage mode switch, or one mode with action keys?
- **A (recommended — the user's own closing lean):** one mode, action keys —
  `v` view · `s` ship · `m` ship-merged · `g` go · `/` filter · `q` quit. Column
  "moves" ARE the actions; guards enforce legality per class.
- **B:** explicit view-mode / manage-mode toggle (original first idea).

**Recommendation:** A — modes double key-map + state for zero extra capability.

**RESOLVED (2026-07-02):** **A** (action keys, no modes).

## D2 — Board↔orchestrator protocol

**Question:** how do richer actions get back to the orchestrator?
- **A (recommended):** single-shot **intents** — any action exits the board writing
  `{"schema":2, "action", "items"}`; the orchestrator executes and **relaunches**
  the board (feels persistent, keeps the exit-code + wait-for contract, headless-
  testable, board stays a no-mutation selector per D5/0.7.0).
- **B:** persistent board that polls/streams intents while running.

**Recommendation:** A.

**RESOLVED (2026-07-02):** **A** (single-shot intents + relaunch loop) — accepted
with the plan ("Accept (all recs)").

## D3 — Action scope for v1

**Question:** which actions ship now?
- **A (recommended):** `v` view + `s` ship + `m` ship-merged + `g` go + `/` filter
  (everything the user listed, minus fallback-mode parity for view/go).
- **B:** view + ship + merge + filter only; defer `g` (go/resume) to roadmap #7.

**Recommendation:** A — `g` is the "move planned → in-progress" half of the ask;
without it the cockpit only ships.

**RESOLVED (2026-07-02):** **A** (full cockpit incl. `g`).

## Implementation notes (round 1 — recorded, not user-gated)

- **`go` ends the board loop** (only `go` + `cancel` do); `view`/`ship`/`ship-merged`
  relaunch — matches D2's relaunch-loop intent.
- **`view` routing looks the class up in the Step-1 work-index** (the intent carries
  only the slug) to pick `<slug>:plan` / `<slug>:report` / changelog entry.
- **validate-in relaxed:** the cockpit opens whenever ANY `.gogo/work/feature-*`
  exists (v/g are useful with nothing ready-to-ship); only "zero features" stops.
- **Runtime file renamed** `ship-result.json` → `board-intent.json` (no longer
  ship-only); legacy `{"ship":[...]}` still parsed as `action: ship`.
- **Interactive empty-`s` is a hint** (no empty-ship emit); headless keeps the
  old empty-ship back-compat.
