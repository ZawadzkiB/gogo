# Review — round 2 (Stage B) — feature `xplan-board-and-simple-done`

**Scope:** Stage B (FR4–FR8), the working-tree diff on top of `47d872f` (v0.9.0):
the new `assets/xplan-board/` (React+Vite source + committed `dist/` + `server.py`),
`skills/gogo-xplan/SKILL.md` + `commands/xplan.md`, and the FR8 sweep across
`skills/gogo/SKILL.md`, `skills/gogo-status/SKILL.md`, `README.md`,
`docs/{index,commands,flow,architecture}.md`, four `.gogo/knowledge/*` files,
`.gitignore`, and `plugin.json` → 0.10.0. Stage A (REV-001/002) was approved in
round 1 — spot-checked here, both fixes still hold.

Reviewed against: `plan.md` (accepted), `decisions.md` (D2=A long-running · D3=A
pre-build · D4=A committed dist/ npm dev-time), `code-review-standards.md`,
`coding-rules.md`, `non-functional-requirements.md`, `tech-stack.md`.

## Verdict: **APPROVE** (Stage B) — no open blockers or majors

Five findings this round (3 minor + 2 nit), all agent-fixable except one
user-decision (REV-007). None blocks the merge. Prior Stage-A findings REV-001 /
REV-002 re-verified fixed.

## What I ran live

- `python3 -m py_compile server.py` → OK; ASCII scan → **0 non-ASCII bytes**; grep
  for `0.0.0.0` / bind-all → none (binds `127.0.0.1` only).
- `python3 server.py --selftest` → **PASS** (exit 0). Bad-args (`--dist` invalid) →
  exit **2**. The selftest is behavioural (validate_intent matrix, ready-set
  derivation, path-traversal containment), not tautological.
- **Live smoke** (fixture `board.json` in a scratch `--data` dir): `GET /` 200,
  `/assets/*.js` 200, `GET /api/board` 200 (re-read per request), `/view/alpha.html`
  200, `/mermaid.min.js` 200; `POST /api/ship` → **202** valid (intent written
  atomically) / **400** non-ready / **400** bad schema / **400** traversal-slug /
  **409** while pending / **400** empty body; unknown POST route → 404.
- **Traversal probes** (`/../topsecret.txt`, `/%2e%2e/...`, `/view/../../...`,
  `/view/%2e%2e/%2e%2e/...`, `/assets/../../server.py`, `/..%2f..%2f...`) → all
  contained (404, never leaked a file outside root).
- **SIGTERM** → server unwinds and removes its own `server.pid` (no leak).
- **dist spot-check**: the committed `dist/assets/index-*.js` contains the
  distinctive new strings `ship-merged`, `waiting for gogo`, `gogo board`,
  `only ready -> changelog`, `filter by slug`; the CSS carries `wf-panel` /
  `board-col` / `toast`. `dist/` is in sync with `src/`.
- **`git add -n assets/xplan-board`**: `dist/` (3 files) + `src/` + `server.py` +
  `package.json` are stageable; `node_modules/` produces **0** staged entries
  (gitignored). `.gitignore` ignores `node_modules/`, `__pycache__/`, `*.pyc`,
  `.gogo/resources/` and keeps `dist/`.
- **0.8.0 writer guard**: extracted the 122-line `## Write changelog entry (1..N
  members)` section from `47d872f` and the working tree — **byte-identical**.

## Dimension verdicts

**(a) server.py security posture — STRONG.** Localhost-only (`HOST = "127.0.0.1"`,
no 0.0.0.0 path). `safe_path()` unquotes, `posixpath.normpath`s, then resolves and
enforces containment (`cand == root or root in cand.parents`), catching
`OSError/RuntimeError/ValueError` (null bytes → 403). Every static route
(`/`, `/assets`, `/view/*`, `/viewer/*`, `/mermaid.min.js`) goes through it;
directories map to `index.html` (no listing); content-types are sane with an
octet-stream default. `POST /api/ship` validates schema==2, action∈{ship,
ship-merged}, non-empty unique kebab-slug `items`, action↔count coherence, and the
ready-only guard against a freshly-read `board.json`; body-size capped at 1 MB;
atomic tmp+rename; 202/400/409/500 semantics correct. All traversal probes verified
contained live. Residuals: no Host/Origin check (REV-007, within the plan's accepted
no-auth scope) and a non-triggerable POST TOCTOU (REV-006).

**(b) App.tsx ↔ board.json ↔ skill payload contract — CONSISTENT, one dead field.**
Columns (plan · in-progress · ready · changelog), `class` enum (shipped /
ready-to-ship / in-progress / unfinished), the class→column map, and `view_url`
naming (`<slug>-plan.html` / `<slug>.html` / `<date>-<name>.html`) all agree across
the skill's board.json builder, App.tsx, `server.ready_slugs`, and gogo-view's real
output names. The ship POST shape `{schema:2, action, items}` with `action=ship-merged`
iff `items>1` matches the server validator exactly. Selection/drag semantics are
unambiguous and match the skill (selected-drag ships the whole selection; unselected
drag ships just that card). The **one** mismatch: App renders a `version` chip the
board.json shape never emits (REV-003, minor).

**(c) FR8 sweep — COMPLETE on the product surface, residue in dogfood knowledge.**
No `tmux|board.py|curses|cockpit|board-intent|resources/kanban` hits in
skills/commands/docs/README/templates; command count **13** everywhere stated
(incl. docs/architecture.md; docs/index.md's quick-ref cross-links `/gogo:xplan`),
version exactly **0.10.0**, architecture.md file map correct (assets/xplan-board
src+dist+server.py; `.gogo/resources/xplan-board` runtime). Gap: two committed
`.gogo/knowledge` files still describe the removed TUI, one referencing the
just-deleted `board.py` (REV-005, minor) — not in the plan's FR8 scope, reconciled
at ⑤ / next `/gogo:build`.

## What else checks out

- **Poll loop** (3s) is ref-based (`draggingRef`, `shippingRef`) — no stale-closure;
  paused during drag, resumes on `dragend` (which always fires). Filter is
  case-insensitive over `slug + title` across all columns with a `n/total` count.
- **Selection** is pruned to still-ready cards on every board refresh; checkboxes
  only on ready cards; the footer "Mark done" ships `[...selected]`.
- **Illegal drags** bounce client-side (any non `ready→changelog` move → hint), and
  the server rejects a non-ready slug with 400 as a second line of defence.
- **Merged reconcile**: both members land in the changelog column keeping their own
  slug, so the client's shipping-set reconciliation clears for both; no duplicate
  orphan changelog item is added (covered by `changelog_path`).
- **Soft-dep + degradation**: gogo-xplan gates on `command -v python3` → points at
  `/gogo:done` and stops; `${CLAUDE_PLUGIN_ROOT}` first with repo fallback; missing
  `dist/` → dev build hint; port auto-roll (25 tries); browser open best-effort; the
  degradation table is complete. npm/node stays dev-time only — no runtime node/npm.
- **Pre-build** reuses the gogo-view build (Step 2 once + Step 3 per item, skips
  Step 4 auto-open), builds only missing/stale pages, and seeds
  `.gogo/resources/{mermaid.min.js, viewer/*}` the server serves under `../`.
- **Writes** are `.gogo/`-only at runtime; the xplan repo is untouched (no edits
  outside this repo); `.gogo/changelog` untouched; the vite `base:'./'` makes dist
  path-agnostic.
- **charts/xplan-board-flow.mmd** matches the as-built flow (React board, server.py
  serving dist + the two API routes, intent → writer → rebuild → poll) — no drift.

## Findings

| id | sev | pri | status | title |
|---|---|---|---|---|
| REV-001 | minor | P2 | verified | partial `+`-merge regressed to empty filter — **fixed, re-verified** |
| REV-002 | minor | P3 | verified | filter narrowed only once (gogo-done + gogo-view) — **fixed, re-verified** |
| REV-003 | minor | P2 | new | App renders a `version` chip board.json never emits (dead field) — **AGENT-FIXABLE** |
| REV-004 | minor | P2 | new | ship that never completes → stuck "shipping..." toast/card, no timeout — **AGENT-FIXABLE** |
| REV-005 | minor | P3 | new | FR8 residue: two `.gogo/knowledge` files still describe the removed TUI (incl. deleted board.py) — **AGENT-FIXABLE** |
| REV-006 | nit | P3 | new | POST /api/ship TOCTOU (fixed tmp + exists-guard) under threading — defense-in-depth — **AGENT-FIXABLE** |
| REV-007 | nit | P3 | new | no Host/Origin check → DNS-rebind/CSRF residual (accepted no-auth scope) — **NEEDS-USER-DECISION** |

### REV-003 — `version` chip is dead (minor, P2)
`App.tsx` types `version?` and renders a version chip (lines 24, 332-338), but the
gogo-xplan board.json item shape (SKILL.md 108-129) never emits `version` (grep
confirms). Guarded by `&&` so it degrades cleanly, but it can never render. **Fix:**
either emit `version` for shipped/ready items, or drop the chip + field.

### REV-004 — stuck "shipping..." on an incomplete ship (minor, P2)
The info toast + shipping-set clear ONLY when the slug reaches the changelog column
(App.tsx 76-84); the info toast is exempt from the 4s auto-dismiss (101-107). If the
orchestrator deletes the intent but the writer fails/abandons, the card never moves
and the surface wedges until a page reload. **Fix:** arm a 60-90s timeout that clears
the shipping state + toast with a recoverable hint.

### REV-005 — FR8 residue in dogfood knowledge (minor, P3)
`.gogo/knowledge/testing-tools.md:33` (owned) still says `python3
assets/kanban/board.py --selftest` — a deleted file; `project-knowledge.md:113-140`
(proxy of README) still has the removed "Board cockpit / board-intent.json" section.
Product surface is clean; these are gogo's own always-read knowledge, reconciled at ⑤
/ next `/gogo:build`. **Fix:** repoint the board.py line to `assets/xplan-board/
server.py` (mirroring the note test-strategy.md got); reconcile the proxy.

### REV-006 — POST /api/ship TOCTOU (nit, P3)
`ThreadingHTTPServer` + `exists()`-then-`os.replace` + a fixed `ship-intent.json.tmp`
lets two truly-concurrent valid POSTs both 202 while one intent is lost. Not
reachable from the single-user UI (posts are sequential; a merge is one POST) — code
hardening only. **Fix:** unique `mkstemp` tmp and/or `O_CREAT|O_EXCL` on the final
file.

### REV-007 — no Host/Origin check (nit, P3, user decision)
The server does no Host/Origin validation, so a malicious page open while the board
runs could `fetch` `POST /api/ship` (DNS-rebind/CSRF) and ship the user's ready
features. Bounded impact; the plan scopes auth OUT ("localhost only"). **Decide:**
accept the residual, or add a stdlib Host-allowlist / Origin check (not auth).

## Route

No open blockers/majors → Stage B **APPROVE**. The five open findings are minor/nit;
REV-003/004/005/006 are agent-fixable (batch back into ② implement or defer the
knowledge reconcile to ⑤), and REV-007 is a one-line user decision. Advance to ④ test
once the batch is dispositioned.
