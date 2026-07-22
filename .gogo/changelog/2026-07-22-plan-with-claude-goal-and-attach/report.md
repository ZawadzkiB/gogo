# A plan-with-claude: goal input + attach the session (fix) — 0.25.1

A patch fixing the 0.25.0 `A` plan-with-claude flow, which was unusable.

## What changed

- **`A` now asks WHAT to plan.** It was minting a blank "Untitled plan" with no goal input. `A` now
  opens a goal form first (a "what should gogo plan for this project?" textarea + optional title);
  the plan is minted with the goal as its description (never blank), title derived rune-safely.
  Cancel/empty → nothing minted.
- **`A` now attaches you to the analyst session.** It was firing the `claude`/tmux session DETACHED,
  so you never saw it and the analyst was never driven. `A` now suspends the TUI and attaches you
  into the live session (the same `attachSession` the board's `a` uses); on detach the board reloads
  with the analyst's writes. No tmux → a headless status naming the background log; no-source → the
  project-home anchor note.
- Plans also gained a real **description** (the `n` quick draft captures one; plan detail renders it),
  and the analyst prompt now names the goal explicitly.

## Review / test

Review APPROVE (0 blocker/major; 2 minor + 3 nit fixed — rune-safe title, headless log path, dead
field, anchor note, message-driven submit test). Gate green; `gogo --version` → 0.25.1. The human
loop (press `A` → goal form appears, nothing minted/launched until submit → mint+launch+attach) was
verified by rendering the real entry point, not just the launcher seam.

Full audit: `.gogo/work/feature-smart-project-plans/` (this patches that 0.25.0 feature).
