# Decisions — feature `board-session-picker`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block. This is the
audit trail that lets the pipeline pause and resume safely.

Both decisions below were **settled with the user before planning** and are folded
into the plan as resolved — do not re-open them.

## D1 — Issue 1 fix shape: keep the collapsed list + dot, or revert to cards?
- **Phase:** plan
- **Question:** Should surfacing "which shipped item has a session" revert the changelog
  column to full cards, or keep the 0.18.0 collapsed list and add a per-row cue?
- **Options:**
  - A. Keep the collapsed `✓ slug … MM-DD` list; add a green `●` on rows with a live
    session — a minimal, presentation-only cue.
  - B. Revert the changelog column to full cards so the existing card `●` dot applies.
- **gogo recommends:** A — the collapsed list is a deliberate 0.18.0 redesign; a per-row
  dot solves the visibility gap without undoing it.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-14)
A — keep the collapsed list and add the `●` dot. Not a revert to card view.

## D2 — Issue 2 fix shape: how much choice for attach/kill over multiple sessions?
- **Phase:** plan
- **Question:** When an item has ≥2 live sessions, how should attach and kill let the
  user choose?
- **Options:**
  - A. Attach picker = choose exactly one; Kill picker = choose one session OR an
    explicit "all N" option, plus Cancel. Single-session case keeps the current UX
    (direct attach / single confirm).
  - B. Multi-select both (checkbox-style) for a fully general pick-any-subset.
- **gogo recommends:** A — pick-one-or-all matches the real need (attach to *a* session;
  kill a stray one or clear them all) with the simplest interaction, and keeps the
  single-session path untouched (no test regressions).
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-14)
A — pick-one-or-all. Attach picker = choose one; Kill picker = choose one OR "all N" +
Cancel. Exactly one session → keep the existing direct/confirm UX.
