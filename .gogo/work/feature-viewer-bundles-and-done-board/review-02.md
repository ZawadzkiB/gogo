# Review — round 04 (Stage B + FR5) · `viewer-bundles-and-done-board`

*(2nd rendered snapshot — `review-02.md`. Stage A round 1 is in `review-01.md`;
rounds 2-3 were the implement-fix + verify of REV-001..005 in the living
`review/issues.json`.)*

**Scope reviewed:** Stage B only — FR4 (`/gogo:done` interactive terminal kanban,
D5=A) — plus the FR5 sweep (docs / version / enumeration sync). Fresh eyes on the
delta; Stage A (view menu, plan bundle, classifier internals) is verified
(REV-001..005) and not re-reviewed.

**Files:** `assets/kanban/board.py` (new), `skills/gogo-done/SKILL.md`,
`commands/done.md`, `skills/gogo/SKILL.md`, `.claude-plugin/plugin.json` (0.7.0),
`docs/{architecture,commands,flow}.md`, `README.md`, and the feature's
`charts/done-board-flow.mmd` + `charts/manifest.json` (as-built done-board sequence).

## Verdict: **APPROVE** — no open blockers or majors

Stage B is correct and honors D5=A. `board.py` is a clean, safe, pure-stdlib
*selector* (no archive duplication); shipping stays single-sourced in `gogo-done`;
FR5 sync is complete (0.7.0, command count still 12, `assets/kanban/` documented).
Two **non-blocking** findings (1 minor, 1 nit) about the interactive tmux path's
error handling — batch them; they do not gate approval.

| Severity | Open/new this round | Total (all rounds) |
|---|---|---|
| blocker | 0 | 0 |
| major | 0 | 1 (verified) |
| minor | 1 (REV-006) | 3 |
| nit | 1 (REV-007) | 3 |

Both new findings are **AGENT-FIXABLE**; neither needs a user decision.
Prior REV-001..005 remain **verified / wontfix** (unchanged).

## What was verified good

- **board.py is a safe stdlib selector (no archive dup).** Pure stdlib only
  (`argparse`, `json`, `os`, `sys`, `tempfile`, `curses` imported lazily) — no
  third-party import, no network, no pip. `python3 -m py_compile` passes; pure
  ASCII (byte-checked). It **never** copies/archives/mutates gogo state — it only
  writes the result file, and `--result` is **required** (no default path, so it
  can never write outside where the skill points it, i.e. `.gogo/`). The ship guard
  is real: `filter_shippable` intersects with `ready-to-ship`, drops non-ready
  slugs and dedups; the interactive confirm returns only selected `ready-to-ship`
  cards (`selected` can only ever hold shippable slugs). `--selftest` passes (7/7)
  and `--headless --ship a,b,c,a` correctly emits `{"ship":[...]}` with non-ready
  dropped + deduped. Cancel writes nothing and exits non-zero. Curses is guarded:
  a clipping `_addstr`, `col_w = max(8, …)`, `card_rows = max(1, …)`, empty/one-
  column handling, scroll offset, and `KEY_RESIZE` redraw — no unguarded draws on
  tiny terminals, no `KeyError` (unknown `class` -> `unfinished`), no `IndexError`
  on empty columns, no infinite CPU loop.
- **Single-sourced ship.** Both `/gogo:done <slug>` and the board route through the
  one "Ship one feature" flow (date-derive -> copy bundle -> mark terminal -> build
  viewer link). The board only *selects*; `gogo-done` loops that single flow over
  the picks. No archive logic is duplicated in `board.py`. Idempotent, `.gogo/`-only,
  copy-not-move — the `--slug` path is unchanged.
- **Graceful fallback (common path).** `python3` / `tmux` are treated as soft deps
  (detected via `command -v`, plus a `[ -t 0 ] && [ -t 1 ]` tty check), matching how
  Playwright / mmdc / jq are handled — no new hard dependency. Absent tooling / no
  tty degrades to a status table + `AskUserQuestion` multi-select that enforces the
  **same** ready-only guard. That fallback is fully specified (Step 3 + Degradation),
  not an afterthought — important since the dev host has no tmux.
- **FR5 sync.** `plugin.json` = **0.7.0**; command count still **12** (board is a
  mode of `done`, no new command); `assets/kanban/` is listed in
  `docs/architecture.md` and `.gogo/resources/kanban/` scratch is documented;
  `docs/{commands,flow,architecture}.md`, `README.md`, and `skills/gogo/SKILL.md`
  all describe the view menu (plans+reports), plan-viewing in place (D1=A),
  friendlier output, and the done board consistently. Grep found **no** stale
  "reports only" / "ships one only" claims; `gogo-status` is documented everywhere
  a roster appears.
- **Invariants.** `${CLAUDE_PLUGIN_ROOT}/assets/kanban/board.py` is the copy source
  (no hard-coded path); `commands/done.md` is thin and delegates; the `.md`/skill
  text is plain ASCII; the classifier contract the board consumes matches the
  `gogo-status` Step A output shape (`{slug, class, title, status}`, extra keys
  ignored). Stage A not regressed.

## Findings

### REV-006 — tmux launch is fragile inside tmux / on a stale session, and a launch failure is silently read as cancel · **minor** · P2 · new · AGENT-FIXABLE
`gogo-done` Step 2 launches with `tmux new-session -s gogo-done "python3 … board.py"`
then treats "exit non-zero / no `$res`" as "user cancelled; ship nothing". But
`board.py` exits 1 on a real cancel **and** on any launch/crash failure. Verified:
run from **inside** tmux (`$TMUX` set — the norm for tmux users) `new-session`
refuses to nest and exits non-zero without ever running the board; a leftover
`gogo-done` session makes it fail "duplicate session". In both cases the board
never renders, no `$res` is written, and the user gets **neither** the board **nor**
the guaranteed table fallback — contradicting the skill's own Step 3 / Degradation
("board error -> the status table … never fail over the board") and
`charts/done-board-flow.mmd` line 17. Non-blocking (it never hard-crashes
`/gogo:done`; the far more common no-tmux/no-tty route degrades correctly), but a
real robustness gap in the interactive path for a scenario tmux users hit routinely.
**Fix:** make the launch nesting-safe (`tmux new-window` / `display-popup -E` /
`TMUX= tmux new-session` when `$TMUX` is set; unique or `-A`/kill-guarded session
name), and capture the tmux launch status separately from the board's exit — if the
pane never started, fall through to the table + `AskUserQuestion` fallback instead of
treating it as a cancel. Only a clean board run returning non-zero should read as
cancel.

### REV-007 — board.py raises an uncaught traceback on a missing/malformed --index · **nit** · P3 · new · AGENT-FIXABLE
`load_index()` opens the path (line 49) and `json.loads()` it (line 51) with no
guard, reached from `main()` (line 328) before curses. Verified: a missing index
file prints a `FileNotFoundError` traceback (exit 1); a malformed file prints a
`JSONDecodeError` traceback (exit 1). Defensive-only (the skill writes the index
itself, well-formed) — hence a nit — but a vendored tool dumping a stack trace is
untidy, and (compounding REV-006) a load-error exit is indistinguishable from a
cancel, so a bad index would silently "ship nothing" with no fallback. Empty index
is already handled (`[]` -> empty board).
**Fix:** wrap the load in `main()` with `try/except (OSError, ValueError)`, print
`board: cannot read work-index <path>: <reason>` to stderr, and `return 2` — a
distinct code the skill can tell apart from a cancel (document exit codes
0 ship / 1 cancel / 2 load-error).

---
*Contract: `review/issues.json` (round 4). This markdown is the rendered snapshot.
Route: no open blockers/majors -> **APPROVE**; batch REV-006 (minor) + REV-007 (nit)
into the next implement pass or defer as accepted follow-ups.*
