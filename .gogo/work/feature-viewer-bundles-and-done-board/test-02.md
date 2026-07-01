# Test Round 2 — feature `viewer-bundles-and-done-board`

Stage B + FR5 sweep. Date: 2026-07-01. Tester: gogo-tester (claude-sonnet-4-6).

---

## Environment

- Platform: macOS 14.6 (Darwin)
- Python 3: present (`/opt/homebrew/bin/python3`, 3.14)
- tmux: **absent** — the fallback path is the live path on this host
- Node.js / Playwright: not invoked (no browser surface to test; plugin is CLI/artifact only)
- Working dir: `/Users/bartlomiej.zawadzki/repos/gogo`

---

## What was exercised

### Level: CLI / board.py (all ran live)

Every test path ran against the real `assets/kanban/board.py` with `python3`.

**TC-B1: Compile check**
```
python3 -m py_compile assets/kanban/board.py
```
Result: PASS — no errors.

**TC-B2: Selftest (7 checks)**
```
python3 assets/kanban/board.py --selftest
```
Result: ALL PASS (7/7)
- normalize keeps 6 records with valid slugs
- unknown class defaults to unfinished
- selectable == [bravo, delta]
- guard filters non-ready slugs -> [bravo, delta]
- guard drops duplicates
- emit shape {"ship":[...]}
- empty selection -> {"ship": []}
Exit code: 0

**TC-B3: Exit-code contract — missing/nonexistent --index**
```
python3 board.py --index /nonexistent/path.json --result /tmp/result.json
```
Result: PASS
- stderr: `board: cannot read work-index /nonexistent/path.json: [Errno 2] No such file or directory`
- exit code: 2
- no traceback

**TC-B4: Exit-code contract — garbage JSON file**
```
echo "not valid json {{{" > garbage.json
python3 board.py --index garbage.json --result /tmp/result.json
```
Result: PASS
- stderr: `board: cannot read work-index ...: Expecting value: line 1 column 1 (char 0)`
- exit code: 2
- no traceback

**TC-B5: Exit-code contract — garbage via stdin (`--index -`)**
```
echo "not valid json {{{" | python3 board.py --index - --result /tmp/result.json
```
Result: PASS
- stderr: `board: cannot read work-index <stdin>: Expecting value: line 1 column 1 (char 0)`
- exit code: 2
- no traceback

**TC-B6: Headless --ship with valid index, all ready-to-ship**
```
python3 board.py --index work-index.json --result result.json --headless --ship "feature-bravo,feature-charlie"
```
Index had: bravo(ready), charlie(ready), alpha(shipped), delta(in-progress), echo(unfinished).
Result: PASS — exit 0, output: `{"ship": ["feature-bravo", "feature-charlie"]}`

**TC-B7: Ready-only guard — non-ready slugs dropped**
```
python3 board.py --index ... --headless --ship "feature-alpha,feature-delta,feature-charlie"
```
Requested: alpha(shipped), delta(in-progress), charlie(ready).
Result: PASS — only charlie survived: `{"ship": ["feature-charlie"]}`

**TC-B8: Dedup**
```
--ship "feature-bravo,feature-bravo,feature-charlie"
```
Result: PASS — `{"ship": ["feature-bravo", "feature-charlie"]}` (bravo once)

**TC-B9: Empty/whitespace index**
- `[]` via `--index -`: exit 0, `{"ship": []}` — PASS
- empty string via stdin: exit 0, `{"ship": []}` — PASS

**TC-B10: Missing --result (interactive path)**
Result: PASS — argparse error, exit 2, no traceback:
`board.py: error: --result is required for the interactive board`

**TC-B11: Pure stdlib — imports verified**
`import argparse, json, os, sys, tempfile` — no third-party imports. PASS.

**TC-B12: Pure ASCII output**
No non-ASCII characters in the source. PASS.

**TC-B13: No network calls**
No `urllib`, `http`, `requests`, `socket`, `urlopen` calls. PASS.

---

### Level: Artifact / dogfood — fallback path (scratchpad fixture)

A fixture `.gogo/work` tree was created in the scratchpad with all five states:
- `feature-old-shipped` — shipped (state.md: status=shipped + changelog entry)
- `feature-ready-one` — ready-to-ship (has report/report.md, not in changelog)
- `feature-ready-two` — ready-to-ship (same)
- `feature-in-progress` — in-progress (phase=test, no report)
- `feature-unfinished` — unfinished (phase=plan, no report)

**Classifier simulation on fixture**
The gogo-status Step A classifier logic was exercised against the fixture in bash:
- `in-progress` → in-progress (phase=test, no report) ✓
- `old-shipped` → shipped (status=shipped, changelog exists) ✓
- `ready-one` → ready-to-ship (has report, not shipped) ✓
- `ready-two` → ready-to-ship (has report, not shipped) ✓
- `unfinished` → unfinished (phase=plan, no report, no changelog) ✓

All five classes correctly classified on a realistic fixture (addresses TEST-001: the unfinished class now has a fixture exemplar).

**board.py guard on fixture work-index**
Attempted to ship `[old-shipped(shipped), ready-one(ready), in-progress(in-progress), unfinished(unfinished)]` headlessly:
- Result: `{"ship": ["ready-one"]}` — only ready-to-ship survived. PASS.

**Ship one feature flow (ready-one)**
Simulated archive step in scratchpad:
- Date derived from report.md: `2026-06-15` (correct grep-oE extraction)
- Destination created: `.gogo/changelog/2026-06-15-ready-one/`
- report.md copied: ✓
- *.mmd glob: no .mmd in fixture (expected, `|| true` handled gracefully)
- state.md updated to `status: shipped, resume: none` ✓
- No files written outside `.gogo/`: confirmed by `find` — PASS.

---

### Level: Code-read + reasoned — interactive TUI path (no tmux present)

The interactive curses path cannot be exercised on this host (no tmux). The following was verified by code-read and reasoning:

**REV-006: Three-outcome routing (gogo-done SKILL.md)**

The routing block at SKILL.md §Board mode step 2:
```bash
if [ -f "$res" ]; then
  # outcome 1: result file written by board on confirm → ship
elif [ -f "$code" ] && [ "$(cat "$code")" = "1" ]; then
  # outcome 2: board ran and user quit → cancel (stop, ship nothing)
else
  # outcome 3: everything else → board error / launch failure → fallback
fi
```

Verified correct:
- Outcome 1 (confirmed): `$res` file exists regardless of `$code` — ships
- Outcome 2 (cancel): `$code` file exists AND equals "1" AND no `$res` — explicit user quit
- Outcome 3 (error → fallback): missing `$code` (tmux never ran), or `$code == 2`, or any other value — routes to status table + AskUserQuestion. Never a silent no-op.

**No bare `new-session -s gogo-done` assumption**
```
grep -rn "new-session -s gogo-done" skills/
```
Result: NONE FOUND. The implementation uses `sess="gogo-done-$$"` (PID-unique) throughout. ✓

**Inside-tmux vs outside-tmux routing**
- Inside `$TMUX`: `tmux new-window -n "$sess" "$run" && tmux wait-for "$sess"` ✓
- Outside tmux: `tmux new-session -A -s "$sess" "$run"` (`-A` = attach-or-create) ✓

**Manual steps for the curses TUI (for a tmux-capable host):**
1. Set `$TMUX` to any non-empty value OR leave unset.
2. Create a valid work-index JSON in `.gogo/resources/kanban/work-index.json` with at least one `ready-to-ship` record.
3. Run: `tmux new-session -s test-board "python3 assets/kanban/board.py --index .gogo/resources/kanban/work-index.json --result /tmp/ship.json"`
4. Verify: arrows/hjkl navigate columns; space/enter toggles ready-to-ship cards; `s` ships; `q` cancels and returns exit 1.
5. Verify on `s`: exit code 0 and `/tmp/ship.json` contains `{"ship": [...selected slugs...]}`.
6. Verify on `q`: exit code 1 and no result file written.

---

### Level: FR5 sync — artifact inspection

**plugin.json version**: `0.7.0` ✓

**Command count**:
```
ls commands/ | wc -l  →  12
```
build.md done.md go.md implement.md plan.md report.md resume.md review.md skills.md status.md test.md view.md — exactly 12. ✓

**assets/kanban/ in docs/architecture.md**:
Line 132: `kanban/ # board.py — vendored python3 curses TUI for the /gogo:done work board (soft dep; --selftest headless)` ✓
Line 148: `.gogo/resources/` description includes `kanban/ (the /gogo:done work-board scratch — the vendored board.py, the work-index, and the ship-result)` ✓

**README — done board**:
Correctly describes the interactive terminal kanban when `python3` + `tmux` + tty are present, otherwise status table + multi-select. No stale "ships one only" language. ✓

**README — view**:
"With no arg it presents a grouped **Work** (each feature's plan + report) / **Changelog** (shipped reports) picker." — plans+reports correctly. No stale "reports only" language. ✓

**docs/commands.md — view**:
`/gogo:view [changelog-entry | feature-slug[:plan|:report]]` — `:plan`/`:report` arg grammar present. Grouped Work/Changelog picker described. ✓

**docs/commands.md — done**:
Work board described with kanban + table fallback. `assets/kanban/board.py` named. ✓

---

### Level: Stage A regression (quick sanity)

**gogo-status classifier**: Step A intact with all 4 classes (shipped, ready-to-ship, in-progress, unfinished), classifier record shape unchanged. ✓

**gogo-view plan bundle**: plan.md viewable in place (D1=A), plan bundle enumeration present in SKILL.md Step 1. ✓

---

## Issues this round

| ID | Title | Severity | Priority | Status |
|---|---|---|---|---|
| TEST-001 | Work-index 'unfinished' class has no live exemplar | nit | P3 | wontfix (fixture now covers it) |
| TEST-002 | assets/kanban/__pycache__/ untracked with no .gitignore exclusion | nit | P3 | new |

**TEST-002 detail**: `assets/kanban/` is new (untracked). Python produced `__pycache__/board.cpython-314.pyc`. The root `.gitignore` has no `__pycache__/` or `*.pyc` exclusion. A `git add assets/kanban/` would commit platform-specific bytecode. Fix: add `__pycache__/` to root `.gitignore`. Severity: nit (P3). Fixable.

---

## Verdict

**a) board.py exit-code contract + ready-only guard**: GREEN. Exit codes 0/1/2 are correct. Garbage inputs produce exit 2 with a one-line stderr message, no traceback. Headless `--ship` and the `filter_shippable` guard drop non-ready slugs and deduplicates. Empty/whitespace index returns exit 0 with empty ship list.

**b) No-tmux fallback ships correctly via single-sourced flow**: GREEN. The SKILL.md §Board mode step 2 correctly detects absence of tmux → step 3 fallback (status table + AskUserQuestion multi-select), never a silent no-op. The classifier simulation on a real 5-state fixture classifies all states correctly. The "Ship one feature" archive step works correctly in isolation (date derived, bundle copied, state.md updated, no writes outside `.gogo/`). The `board.py --headless --ship` path (the optional emit step in fallback) guards non-ready slugs correctly.

**c) REV-006 three-outcome routing**: GREEN. The three branches are correct: result-file-present → ship; code-file=="1" → cancel; else → fallback. No bare `new-session -s gogo-done` assumption. Inside-tmux uses `new-window` + `wait-for`; outside uses `new-session -A -s`. Session uniqueness via `gogo-done-$$`.

**Overall**: Build PASS, selftest PASS, all live board.py paths PASS, fallback dogfood PASS, FR5 sync PASS. One nit (TEST-002, fixable, not blocking). No open or new blocking issues.

**Done-bar status**: GREEN (build + unit/selftest + hands-on CLI + artifact + code-read verification; 0 open/blocking issues).
