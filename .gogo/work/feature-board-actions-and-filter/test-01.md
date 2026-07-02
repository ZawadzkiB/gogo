# Test — feature `board-actions-and-filter` — round 1

- **Phase:** ④ test · **round:** 1 · **date:** 2026-07-02
- **Tester:** gogo-tester
- **Scope:** FR1-FR5 + Tests section of plan.md; REV-001 regression; real TUI via tmux

## Verdict: ALL GREEN

Build green · unit/selftest green (31/31) · headless matrix green · REV-001 regression fixed · TUI live via tmux: all scenarios pass · FR5 sweep clean · 0.8.0 regression clear. No new issues. Open issues: **0**.

---

## What ran live

| Check | Method | Result |
|---|---|---|
| `python3 -m py_compile assets/kanban/board.py` | live | PASS (Python 3.14.5) |
| `--selftest` 31/31 | live | PASS — exit 0 |
| Headless matrix: ship / ship-merged / view / go / cancel | live | All PASS — correct schema-v2 shapes, exit codes 0/1 |
| Legacy `--ship a,b` back-compat | live | PASS — emits schema-v2 ship intent, exit 0 |
| Guard violations: m<2, g on ready/shipped, view no slug, unknown slug | live | All PASS — exit 2, one stderr line, no result file |
| Bad index: missing file, malformed JSON | live | Both PASS — exit 2, one stderr line (no traceback) |
| Empty stdin headless | live | PASS — exit 0 (documented back-compat empty ship) |
| ASCII-only check | live | PASS — pure ASCII |
| stdlib-only imports | live | PASS — argparse, json, os, sys, tempfile |
| REV-001 regression (TERM= + no tty) | live | PASS — exit 2, "board: cannot run the TUI: setupterm: could not find terminal", no traceback, no result file |
| TUI scenario a — space on ready card | tmux live | PASS — [x] toggles; selected counter increments |
| TUI scenario a — space on shipped card | tmux live | PASS — hint "only ready-to-ship cards can be selected", no toggle |
| TUI scenario a — space on in-progress card | tmux live | PASS — same hint, no toggle |
| TUI scenario b — / filter live narrow | tmux live | PASS — columns narrow as you type; "filter (type; Enter=apply, Esc=clear): bra_", unfinished:0 in-progress:0 ready-to-ship:1 shipped:0 |
| TUI scenario b — C-m to apply filter | tmux live | PASS — exits edit mode, header shows "filter: bra  (1/5 shown)   Esc=clear" |
| TUI scenario b — Esc clears filter | tmux live | PASS — clears after ESCDELAY (~1.5s, expected curses behavior); all 5 cards visible again |
| TUI scenario b — selection survives filtering | tmux live | PASS — ready-bravo [x] remained selected through filter operations |
| TUI scenario c — select 2 ready + m → ship-merged | tmux live | PASS — exit 0; intent: `{"schema":2,"action":"ship-merged","items":["ready-bravo","ready-charlie"]}` |
| TUI scenario d — v on shipped card | tmux live | PASS — exit 0; intent: `{"schema":2,"action":"view","items":["shipped-alpha"]}` |
| TUI scenario e — g on ready card | tmux live | PASS — hint "go: only unfinished / in-progress cards", no exit |
| TUI scenario e — g on in-progress card | tmux live | PASS — exit 0; intent: `{"schema":2,"action":"go","items":["inprogress-delta"]}` |
| TUI scenario f — q cancel | tmux live | PASS — exit 1, no intent file |
| Cockpit routing spec — all 5 actions | spec-read | PASS — complete, unambiguous (see section below) |
| FR5 sweep | live + spec-read | PASS — all sub-checks green (see section below) |
| 0.8.0 regression | spec-read | PASS — synthesis writer / members[] / selftest legacy intact |

---

## TUI fixture

Work-index used for all tmux scenarios (5 cards, all 4 classes):

```json
[
  {"slug": "shipped-alpha",    "class": "shipped",       "title": "Alpha Release shipped feature"},
  {"slug": "ready-bravo",      "class": "ready-to-ship", "title": "Bravo ready to ship"},
  {"slug": "ready-charlie",    "class": "ready-to-ship", "title": "Charlie also ready"},
  {"slug": "inprogress-delta", "class": "in-progress",  "title": "Delta in progress"},
  {"slug": "unfinished-echo",  "class": "unfinished",   "title": "Echo not started"}
]
```

Sessions: each scenario used a unique `gogo-test-board-$$` session name. All sessions killed after use. No `gogo-done` session was present or touched. All fixture files written to the designated scratchpad.

---

## Headless matrix — exact intent shapes verified

| Action | `--headless` call | Emitted intent | Exit |
|---|---|---|---|
| ship | `--action ship --ship ready-one,ready-two` | `{schema:2, action:"ship", items:["ready-one","ready-two"]}` | 0 |
| ship-merged | `--action ship-merged --ship ready-one,ready-two` | `{schema:2, action:"ship-merged", items:["ready-one","ready-two"]}` | 0 |
| view | `--action view --ship ready-one` | `{schema:2, action:"view", items:["ready-one"]}` | 0 |
| go | `--action go --ship in-progress-feat` | `{schema:2, action:"go", items:["in-progress-feat"]}` | 0 |
| cancel | `--action cancel` | (no file) | 1 |

Guard violations (all exit 2, one stderr line, no result file):

| Guard | Stderr (verbatim) |
|---|---|
| ship-merged <2 | `board: ship-merged needs >= 2 ready-to-ship slugs` |
| go on ready-to-ship | `board: go only applies to unfinished / in-progress (ready-one is ready-to-ship)` |
| go on shipped | `board: go only applies to unfinished / in-progress (shipped-feature is shipped)` |
| view no slug | `board: --action view needs a slug in --ship` |
| unknown slug | `board: unknown slug 'nonexistent-slug'` |
| missing index | `board: cannot read work-index /nonexistent/path/index.json: [Errno 2] ...` |
| malformed JSON | `board: cannot read work-index <path>: Expecting value: line 1 column 1 (char 0)` |

---

## Cockpit routing spec — completeness verdict

All 5 actions present in the routing table (`skills/gogo-done/SKILL.md`, lines 307-311):

| Action | Routing | Gate skip | End loop? |
|---|---|---|---|
| view | class lookup from Step-1 work-index → `<slug>:plan` (unfinished/in-progress), `<slug>:report` (ready-to-ship), changelog path (shipped) via gogo-view build | n/a | no — relaunch |
| ship | Write changelog entry once per slug (0.8.0 writer) | explicit `s` = separate, do NOT ask gate | no — relaunch |
| ship-merged | Write changelog entry once with all items as members[], confirm release name | gate pre-answered by `m` | no — relaunch |
| go | End loop + hand off to pipeline per state.md (like /gogo:go) | n/a | YES |
| cancel (exit 1, no $res) | Stop — nothing shipped | n/a | YES |

Additional spec checks:

- **Re-classify between relaunches**: SPECIFIED — "repeat step 3 with a freshly-written `$idx` if state changed — a just-shipped feature now classifies as `shipped`" (line 314). A view relaunch rebuilds the index; a ship relaunch re-classifies after marking state.md shipped. Not silent.
- **s skips the separate-vs-merged gate**: CONFIRMED — "Explicit `s` = **separate** → do **NOT** ask the separate-vs-merged gate." (line 308).
- **go ends loop**: CONFIRMED — "**END the loop** and hand off to the pipeline: resume the focused feature per its `state.md` (exactly like `/gogo:go <slug>`)" (line 310).
- **cancel stops**: CONFIRMED — "(exit 1, no `$res`) Stop — nothing shipped. | loop ends" (line 311).
- **exit-2/launch-fail → fallback (never cancel)**: CONFIRMED — lines 291-300: only `$code == 1` with no `$res` is cancel; absent/2/other → guaranteed fallback.
- **Legacy `{"ship":[...]}` back-compat**: CONFIRMED — stated in skill inputs table (line 62) and routing section (lines 302-303).
- **Detached no-tty pattern**: CONFIRMED — lines 272-276: `tmux new-session -d -s "$sess" "$run" || true`, echo attach instruction, `tmux wait-for "$sess"`.
- **board-intent.json rename consistent**: CONFIRMED — zero `ship-result.json` references in product files (assets/, skills/, commands/, docs/, README.md).

---

## FR5 sweep

| Check | Result |
|---|---|
| `plugin.json` version | **0.9.0** |
| command files | **12** (build done go implement plan report resume review skills status test view) |
| stale "space/enter + s/q only" wording | none found |
| `docs/commands.md` relaxed validate-in (REV-002) | PASS — "opens the cockpit whenever **any** feature exists ... stops only when there are zero features." |
| `docs/flow.md` relaxed validate-in (REV-002) | PASS — "board mode opens the cockpit whenever any feature exists ... stops only when there are zero features." |
| `README.md` cockpit action keys | PASS — v/s/m/g/filter/q all described |
| `commands/done.md` action keys + filter | PASS |
| `skills/gogo/SKILL.md` cockpit wording | PASS |
| `docs/architecture.md` board-intent reference | PASS |
| charts manifest validates | PASS — slug ok, kinds "flow"+"activity" in enum, both .mmd files exist |
| `ship-result.json` in product files | **zero** references |
| `__pycache__` in repo after runs | None (cleaned up after testing; properly gitignored) |
| pure stdlib, pure ASCII | PASS |

---

## 0.8.0 regression

- **Synthesis writer section** (`Write changelog entry (1..N members)`): intact — members[], slim file set (report.md + *.mmd + manifest.json + before/), no diagrams.html copy, idempotent.
- **Classifier members[] rule** (`skills/gogo-status/SKILL.md`): intact — a slug in any manifest's `members[]` == shipped; `changelog_path` set to the merged release dir.
- **--selftest legacy ship-shape check**: present — check "legacy {'ship':[...]} parses as action ship" is check #28 of 31 in selftest().

---

## Issues this round

| ID | Title | Severity | Status |
|---|---|---|---|
| (none) | | | |

No new issues. REV-001 and REV-002 (review track) already verified and recorded in `review/issues.json`; they have no test-track entries.

---

## Explicit verdicts

**(a) Per-class guards in real TUI** — GREEN. Confirmed live via tmux: space on shipped/in-progress shows "only ready-to-ship cards can be selected" hint, no toggle; g on ready-to-ship shows "go: only unfinished / in-progress cards" hint, no exit; g on in-progress exits with go intent; m with 2 ready exits with ship-merged intent.

**(b) Filter behavior live** — GREEN. Confirmed live: / opens edit mode, typing "bra" narrows to 1/5 live; C-m exits edit mode and shows "filter: bra  (1/5 shown)   Esc=clear" in header; Esc clears (after ESCDELAY, expected curses behavior); ready-bravo selection survived all filter operations.

**(c) Intent emission per action live** — GREEN. Confirmed live: ship-merged intent `{schema:2, action:"ship-merged", items:["ready-bravo","ready-charlie"]}`; view intent `{schema:2, action:"view", items:["shipped-alpha"]}`; go intent `{schema:2, action:"go", items:["inprogress-delta"]}`; cancel (q) → exit 1, no file.

**(d) Cockpit routing spec completeness** — GREEN. All 5 actions in routing table; view class-lookup documented per class; re-classify between relaunches specified; s skips gate; go ends loop; cancel stops; fallback wired for exit-2/launch-fail; legacy back-compat stated; detached no-tty pattern documented; board-intent.json consistent.

**(e) REV-001 regression** — GREEN. `TERM= python3 assets/kanban/board.py --index <idx> --result <res> </dev/null` → exit 2, stderr "board: cannot run the TUI: setupterm: could not find terminal" (1 line), no traceback, no result file.

---

## Sessions and scratchpad

- All tmux sessions used unique names (`gogo-test-board-<pid>`); none named `gogo-done`.
- All sessions killed after their scenario.
- Fixture files written to scratchpad only; no mutation of `.gogo/work/` state.md files, `.gogo/changelog/`, or `.gogo/resources/kanban/`.
- `__pycache__` generated during testing was cleaned up; verified gitignored.
