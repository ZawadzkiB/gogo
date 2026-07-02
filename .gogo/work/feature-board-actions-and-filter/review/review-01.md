# Review — feature `board-actions-and-filter` — round 1

- **Phase:** ③ review · **round:** 1 · **date:** 2026-07-02
- **Reviewer:** gogo-reviewer (fresh eyes)
- **Scope reviewed (this feature's delta only):** `assets/kanban/board.py`,
  `skills/gogo-done/SKILL.md` (board/cockpit sections), `commands/done.md`,
  the board wording in `skills/gogo/SKILL.md` · `README.md` ·
  `docs/{commands,flow,architecture,index}.md`, `commands/view.md`,
  `.claude-plugin/plugin.json` (0.9.0), and the feature charts.
  0.8.0 (`changelog-merged-entries`) synthesis-writer / `members[]` / schema
  changes were mentally attributed to that already-approved feature.

## Verdict: **CHANGES**

One open **major** (REV-001). Everything else — the action-key dispatch, per-class
guards, filter, schema-v2 intent, headless matrix, legacy back-compat, docs sync,
charts, version/command-count, and the write-scope/no-mutation invariant — checks
out. Fix REV-001 (and the minor REV-002 doc drift) and re-review.

## What I ran live

| Check | Result |
|---|---|
| `python3 -m py_compile assets/kanban/board.py` | OK (Python 3.14.5) |
| `board.py --selftest` | **31/31 PASS**, exit 0 — checks are behavioral (per-class guards, filter narrow/clear/case/title/survival, cursor re-clamp, emit round-trip, legacy normalize), not tautologies |
| Headless matrix (`ship`, `ship-merged`, `view`, `go`, `cancel`, legacy `--ship`) | Correct schema-v2 shapes + exit 0; `cancel` exit 1 no file |
| Guard violations (`ship-merged` <2, `go` on ready/shipped, unknown slug, `view` no slug) | exit 2 + one-line stderr, no file — as contracted |
| Bad index (missing file, malformed JSON, empty stdin) | exit 2 + one-line stderr (no traceback); empty stdin -> empty ship exit 0 (documented back-compat) |
| **Curses launch with no tty / `TERM` unset** | **exit 1 with full traceback, no result file** — violates the exit-2 contract (see REV-001) |
| Pure ASCII (`grep -P '[^\x00-\x7F]'`) | clean |
| Bytecode staged | none tracked; `__pycache__/` + `*.pyc` gitignored (footprint NFR held) |
| `charts/manifest.json` vs `charts-manifest.schema.json` | valid — slug ok, kinds `flow`+`activity` in enum, both `.mmd` files exist |
| Version / command count | `plugin.json` 0.9.0; 12 command files; no stale `ship-result.json` in product files |

## Findings

### REV-001 — major · P1 · new · AGENT-FIXABLE
**Interactive curses launch is unguarded: a startup failure or event-loop exception
exits 1 with a traceback (not the documented exit 2), so `gogo-done` misreads it as a
user cancel and skips the mandated fallback.**

`assets/kanban/board.py:400` (`run_board`) calls `curses.wrapper(_board_loop, records)`
with no `try/except`, and neither `main()` nor `sys.exit(main())` guards it. Any curses
startup failure (bad/absent `TERM`, a terminal that can't `cbreak`) or any uncaught
exception inside `_board_loop` propagates as a full Python traceback with exit code 1
and no intent file. Reproduced live:
`python3 assets/kanban/board.py --index <idx> --result <out> < /dev/null` → traceback,
`exit=1`, no result; same with `env -u TERM`.

This contradicts the file's own exit-code contract (docstring lines 30-37: *“2 error —
… or the board cannot start. No result file; a one-line reason on stderr and NO
traceback.”*). And the consuming skill depends on 1-vs-2 to tell cancel from crash:
`skills/gogo-done/SKILL.md:288` treats `no $res && $code == 1` as
*“board cancelled — nothing shipped”* and **stops**, whereas `exit 2` routes to the
guaranteed status-table + `AskUserQuestion` fallback (lines 290-293). So a board that
crashes on startup or mid-loop is silently reported to the user as a deliberate cancel,
and the fallback the NFR mandates (*“absence → graceful fallback, never a failure”*)
never fires. (In the normal tmux launch curses usually gets a working pty — which is
why `--selftest` can't cover it and it slipped through — but a mid-loop crash is
reachable in the real path.)

*Fix:* wrap the launch so any failure returns the documented exit 2 + one-line stderr
(no traceback):
```python
def run_board(records, result_path):
    import curses
    try:
        intent = curses.wrapper(_board_loop, records)
    except Exception as exc:            # curses.error and any event-loop crash
        print("board: cannot run the interactive board: %s" % exc, file=sys.stderr)
        return 2
    if intent is None:
        return 1
    emit_intent(result_path, intent)
    return 0
```
`curses.wrapper` already restores the terminal before the exception surfaces, and
`emit_intent` only runs after a clean return, so no partial file leaks.

### REV-002 — minor · P2 · new · AGENT-FIXABLE
**Stale validate-in wording in two FR5 reference docs contradicts the shipped relaxed
cockpit gate.**

`docs/commands.md:181` (*“board mode with nothing ready-to-ship says so and stops
without opening an empty board.”*) and `docs/flow.md:107` (*“…says so instead of
opening an empty board.”*) both describe the OLD gate. The feature **relaxed**
validate-in (`decisions.md`; `skills/gogo-done/SKILL.md:76-82`; `commands/done.md:22-23`):
the cockpit now opens whenever **any** `.gogo/work/feature-*` exists so `v`/`g`/`/`
work on non-shippable cards — only **zero** features stops. `README.md` correctly
dropped the old claim, but these two docs (both explicitly listed for FR5 sync) kept
the stale line even after their surrounding cockpit prose was rewritten — the exact
all-of-`docs/*.md` doc-sync class `code-review-standards.md` elevates.

*Fix:* replace the stale clause in both files with the relaxed behavior, matching the
wording already in `skills/gogo-done/SKILL.md:76-82` / `commands/done.md:22-23`
(zero work items → stop; items present but none ready-to-ship → cockpit still opens for
view/go/filter, can't ship/merge until report-complete).

## Dimensions checked and clean (no finding)

- **Key dispatch + per-class guards** — `v` any card; `s` ≥1 selected ready; `m` ≥2
  selected ready; `g` focused unfinished/in-progress only; space/enter toggles
  ready-to-ship only; invalid keys show a transient one-line hint (`_board_loop`
  clears `status_hint` each key). Guard logic is single-sourced in `resolve_action`
  and shared by the loop, `--headless`, and `--selftest`.
- **Filter (FR4)** — live case-insensitive slug+title match, applied while typing;
  Esc clears (both in edit mode and command mode); selection is a slug set independent
  of the filter so it survives filtering; `cur_row` re-clamps to the filtered column
  every iteration (`clamp_index`); empty-column/empty-index cases yield `focus=None`
  and a hint, never a crash.
- **Intent emission (FR2)** — exact schema `{schema:2, action, items}`; exit codes
  0/1/2 honored on every non-curses path; `cancel` writes nothing (exit 1).
- **Legacy back-compat** — `normalize_intent` maps `{"ship":[...]}` → `action:ship`;
  `--action` defaults `ship`, `--ship` still maps items (verified live).
- **Robustness** — `_addstr` clips on tiny terminals; unknown class → `unfinished`;
  records without a string slug dropped; bad/malformed/empty index → exit 2 one-line.
- **Write-scope / no-mutation (D2/D5)** — board writes only `--result` (and a tempdir
  in selftest); `board-exit.code` is written by the skill's shell, not by `board.py`;
  pure stdlib, no network. Invariant held.
- **Cockpit spec** — relaunch loop is bounded/escapable (only `go`/`cancel` end it;
  `q` always escapes a view loop) and re-classifies between relaunches (a just-shipped
  item moves to `shipped`); intent routing table (view→class-lookup→gogo-view target;
  ship→writer per slug, no separate-vs-merged question; ship-merged→writer once with
  members + name confirm; go→end loop + resume; cancel→stop) is consistent; the
  `ship-result.json`→`board-intent.json` rename is swept across all product files.
- **Charts** — `board-cockpit-flow.mmd` (flow) + `board-key-flow.mmd` (activity state
  machine) match the as-built behavior; manifest validates (kinds in enum, files exist).
- **Version / footprint** — `plugin.json` 0.9.0; 12 commands; no staged bytecode.
