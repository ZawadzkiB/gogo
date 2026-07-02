# Test round 1 — feature `xplan-board-and-simple-done`

**Date:** 2026-07-02  
**Scope:** Combined Stages A+B (FR1-FR8), round 1  
**Stages reviewed:** A (FR1-FR3) and B (FR4-FR8)  
**All REV findings (REV-001..REV-007):** verified fixed

## Overall verdict

**GREEN** — build + unit (n/a, markdown plugin) + live server matrix + Playwright board all pass. One new nit (TEST-001, fixable) found. Done-bar met.

---

## 1. Server matrix (live)

Server started: `python3 assets/xplan-board/server.py --port 4199 --dist assets/xplan-board/dist --data <scratch>/board-data --view-root <scratch>/resources/view`

Fixture: `board.json` with 5 items across all 4 columns (2 ready-to-ship, 1 in-progress, 1 unfinished, 1 shipped).

| Check | Result |
|---|---|
| `python3 -m py_compile server.py` | PASS |
| ASCII scan (0 non-ASCII bytes) | PASS |
| `python3 server.py --selftest` exit 0 | PASS |
| `GET /` → 200 | PASS |
| `GET /assets/<js>` → 200 | PASS |
| `GET /api/board` → 200 + 5 items | PASS |
| `GET /view/alpha-feature.html` → 200 | PASS |
| `POST /api/ship` valid single → 202 + intent file (action: "ship") | PASS |
| `POST /api/ship` valid multi → 202 + intent file (action: "ship-merged") | PASS |
| `POST /api/ship` while pending → 409 | PASS |
| `POST /api/ship` non-ready slug → 400 "not ready-to-ship: gamma-wip" | PASS |
| `POST /api/ship` bad JSON → 400 | PASS |
| `POST /api/ship` bad shape (schema=1) → 400 | PASS |
| `POST /api/ship` empty body → 400 | PASS |
| `Host: evil.example` → 403 (all verbs) | PASS |
| `Origin: http://evil.example` on POST → 403 (no intent written) | PASS |
| `localhost` Origin → pass | PASS |
| absent Origin → pass | PASS |
| Traversal `/view/../` raw socket → 404 (contained) | PASS |
| Traversal `/view/%2e%2e/` raw socket → 404 (contained) | PASS |
| Traversal `/view/../../etc/passwd` raw socket → 404 (contained) | PASS |
| Traversal `/view/%2e%2e/%2e%2e/etc/passwd` raw socket → 404 (contained) | PASS |
| Traversal `/assets/../../server.py` raw socket → 404 (contained) | PASS |
| Traversal `/..%2f..%2fserver.py` raw socket → 404 (contained) | PASS |
| SIGTERM removes `server.pid` | PASS |

---

## 2. React board (Playwright MCP — live)

Server: port 4200, same fixture. Board navigated at `http://127.0.0.1:4200/`.

### 2a. Initial render
- Four columns rendered: **plan** (1), **in progress** (1), **ready** (2), **changelog** (1).
- delta-todo in plan, gamma-wip in in-progress, alpha-feature + beta-feature in ready, 2026-06-01-old-release in changelog.
- Ready cards have checkboxes (`Select alpha-feature`, `Select beta-feature`); non-ready cards (plan, in-progress, changelog) have no checkboxes.
- All cards have "view" links.
- Board header shows: "gogo board", "test-fixture" repo name, filter input, "live" pulse.

**Result: PASS**

### 2b. Text filter
- Typed "alpha" into filter.
- ready column: 1/2 (alpha-feature visible, beta-feature hidden); plan: 0/1 "no match"; in-progress: 0/1 "no match"; changelog: 0/1 "no match".
- Cleared filter: all items restored.

**Result: PASS**

### 2c. Checkbox selection + footer
- Selected alpha-feature: footer shows "1 selected" + "Mark done (1)" (no merged entry text).
- Selected beta-feature additionally: footer shows "2 selected" + "Mark done (2) -> one merged entry".

**Result: PASS**

### 2d. Mark done → POST → shipping state → live card move
1. Clicked "Mark done (2)".
2. Network: `POST /api/ship` → 202 Accepted.
3. Intent file written: `{"schema":2,"action":"ship-merged","items":["alpha-feature","beta-feature"],...}`.
4. Board: alpha-feature and beta-feature show "shipping..." tags; toast "shipping 2 items... waiting for gogo" (kind:info, persistent).
5. Footer gone (selection cleared).
6. Simulated orchestrator: updated board.json (moved both to changelog column), removed intent.
7. After ~3s poll: alpha-feature + beta-feature appeared in changelog column; ready column shows "0/empty"; toast cleared.

**Result: PASS** — full ship-from-board loop including live column move confirmed.

### 2e. View link
- Clicked `view` on alpha-feature card (ready column).
- New tab opened at `http://127.0.0.1:4200/view/alpha-feature.html` (the fixture view page).
- URL matches `/view/<view_url>` pattern.

**Result: PASS**

### 2f. Drag ready → changelog
- Reset board, dragged alpha-feature card to changelog column.
- Network: `POST /api/ship` → 202 Accepted.
- Intent file: `{"schema":2,"action":"ship","items":["alpha-feature"],...}` (single card drag = action:"ship").
- alpha-feature shows "shipping..." tag; toast "shipping 1 item... waiting for gogo".

**Result: PASS**

### 2g. Drag guard — illegal move (non-ready → changelog)
- While alpha-feature was in "shipping..." state, dragged gamma-wip (in-progress) to changelog column.
- Network request count: unchanged (no third POST fired).
- Intent file: absent (no intent created).
- Hint toast appeared briefly ("only ready -> changelog is allowed - columns follow the work state"), auto-dismissed after 4s.
- Note: hint toast replaced the persistent info toast (see TEST-001).

**Result: PASS** (guard works; TEST-001 documents the toast replacement as a nit)

### 2h. Screenshot
Saved to: `/private/tmp/claude-502/-Users-bartlomiej-zawadzki-repos-gogo/f759479a-3eec-4068-9cfc-9d5b6edb19a8/scratchpad/board-screenshot.png`

---

## 3. Stage A dogfood (spec-read + code verification)

This stage is a markdown skill; live dogfood on a scratch repo is deferred to the install+dogfood step. Spec verified in the code:

| Check | Location | Result |
|---|---|---|
| `+` arg always explicit merge — unknown part → STOP (REV-001) | `skills/gogo-done/SKILL.md` lines 71-72, 92 | PASS |
| bare `+`-free non-resolving arg → filter, not STOP | `skills/gogo-done/SKILL.md` line 89-92 | PASS |
| Filter loops until list fits (REV-002) | `skills/gogo-done/SKILL.md` lines 246-249 | PASS |
| gogo-view: explicit `<slug>:plan` missing → STOP | `skills/gogo-view/SKILL.md` lines 56-59, 97-99 | PASS |
| gogo-view: bare non-resolving arg → filter | `skills/gogo-view/SKILL.md` lines 59, 107 | PASS |
| gogo-view: filter loops until menu fits (REV-002) | `skills/gogo-view/SKILL.md` lines 113-115 | PASS |
| Multiple selection = ONE merged entry; no separate-vs-merged question | `skills/gogo-done/SKILL.md` lines 254-258 | PASS |
| Single pick = single entry (list mode) | `skills/gogo-done/SKILL.md` line 255 | PASS |

---

## 4. gogo-xplan skill walk (spec-read)

| Check | Result |
|---|---|
| board.json payload matches App.tsx expectations (slug/title/class/column/view_url/iterations/status) | PASS — all fields in item shape match |
| view_url naming: `<slug>-plan.html` / `<slug>.html` / `<date>-<name>.html` | PASS — skill table at SKILL.md:92-98 |
| Watch loop: read+DELETE intent → writer → rebuild → continue | PASS — skill ⑤ step 1-4 |
| Stop semantics: `kill $(cat server.pid)` | PASS — SIGTERM verified live |
| Degradation table complete (no python3/port busy/no browser/dist missing) | PASS — all 4 rows in skill degradation table |
| `${CLAUDE_PLUGIN_ROOT}` first, repo fallback for dist path | PASS — skill ① gate |

---

## 5. FR8 + regression sweeps

| Check | Result |
|---|---|
| `plugin.json` == 0.10.0 | PASS |
| 13 commands in `commands/` dir | PASS (13 files) |
| 13 stated in `docs/commands.md` | PASS ("There are **13** commands...") |
| 13 stated in `docs/architecture.md` | PASS ("13 slash commands") |
| xplan command in `commands/xplan.md` | PASS |
| xplan cross-linked from `docs/index.md` | PASS (in /gogo:done table) |
| Product-file grep: 0 stale `tmux|board.py|board-intent|curses|cockpit|resources/kanban` | PASS |
| `assets/kanban/` removed | PASS |
| `dist/` tracked and stageable (not gitignored) | PASS |
| `node_modules/` not staged (0 entries via git add -n) | PASS |
| `__pycache__/` gitignored | PASS |
| 0.8.0 writer section (`## Write changelog entry`) byte-identical to 47d872f | PASS (md5 match) |
| Charts manifest validates (slug, kind, file, title fields; file on disk) | PASS |
| Charts xplan-board-flow.mmd exists | PASS |
| xplan repo untouched | PASS (no changes) |
| `testing-tools.md` updated to point at `assets/xplan-board/server.py` (REV-005 fix) | PASS |

---

## Issues

| id | sev | pri | status | title |
|---|---|---|---|---|
| TEST-001 | nit | P3 | new | Illegal drag while shipping replaces persistent info toast; after 4s dismiss, no toast until watchdog |

### TEST-001 — Illegal drag replaces shipping toast (nit, P3)

When a non-ready card is dragged to changelog while a ship is in progress, `onDrop` calls `setToast({...hint...})`, which replaces the persistent `info` toast ("shipping N items... waiting for gogo"). The hint (kind:'hint') auto-dismisses in 4s. After dismissal, no toast indicates the ship is still pending — only the card's "shipping..." tag remains. The shipping state is correctly maintained (ref, card class, card tag) and reconcile clears everything normally. The hint content itself is correct.

**Fix:** In `onDrop`'s illegal-drag branch, check `shippingRef.current.length > 0` before replacing the toast; if shipping, append/overlay the hint without overwriting the info toast (or skip the global toast and show a transient card-level indicator).

**Severity:** nit — the ship continues correctly; only toast UX is briefly absent.

---

## Verdict against done-bar

- Build: N/A (markdown plugin; React dist pre-committed, verified consistent with src).
- Unit: N/A (no automated suite).
- Live server matrix: GREEN (all 25+ checks pass).
- Playwright board: GREEN (all 8 scenarios pass — initial render, filter, checkboxes, mark-done, POST, live move, view link, drag, drag guard).
- Stage A spec-read: GREEN (REV-001/002 regression guards confirmed; multiple=merge, no separate-vs-merged question).
- FR8 sweep: GREEN (0.10.0, 13 commands, no stale refs, dist tracked, writer identical, xplan untouched).
- REV-001..REV-007: all verified fixed in this round.
- Open issues: 1 nit (TEST-001, fixable, does not block shipping).

**Done-bar met.** Recommend advancing to ⑤ report; TEST-001 nit deferred to next ship or folded into report.
