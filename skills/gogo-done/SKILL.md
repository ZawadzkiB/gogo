---
name: gogo-done
description: >-
  The "ship it" step after phase ⑤ — when the user declares a feature done, copy
  its report bundle (report/report.md + the UML .mmd set + the before/ set +
  diagrams.html) into the append-only .gogo/changelog/<YYYY-MM-DD>-<slug>/ archive,
  build the interactive viewer page for that entry (reusing the gogo-view build) and
  print its file:// link, and set the feature's state.md to a terminal `shipped`
  status. With NO slug it opens an interactive work board (terminal-TUI kanban, or a
  status table + multi-select fallback) to pick which ready-to-ship features to ship.
  Use when the user runs /gogo:done or says a feature is shipped / finished /
  released. Copy-not-move, idempotent, writes only under .gogo/, offline.
---

# gogo-done — promote report-complete features to the changelog

The explicit post-report gate. `/gogo:report` (⑤) finalizes the report bundle in
the **work** folder; `/gogo:done` is the user saying *"this is shipped"* — it
**copies** that bundle into the chronological `.gogo/changelog/` archive, **builds +
prints the interactive viewer link** for that entry, and marks the feature terminal.

Two modes, one shipping path:
- **`/gogo:done <slug>`** — ship that one feature directly (unchanged path).
- **`/gogo:done`** (no slug) — open the **work board** (D5=A): classify every
  `.gogo/work/feature-*`, let the user pick which **ready-to-ship** features to ship,
  then ship each. The board is an interactive terminal kanban when the tooling is
  present; otherwise a status table + `AskUserQuestion` multi-select. **Either way,
  the actual shipping is the single "Ship one feature" flow below** — the board only
  *selects*; it never archives anything itself.

Pure `Read` / `Write` / `Bash` (+ `Skill` to reuse the `gogo-view` build); only ever
writes under `.gogo/`; offline throughout.

## Inputs (declared) and outputs

| Direction | Artifact | Contract |
|---|---|---|
| in (required, per shipped slug) | `.gogo/work/feature-<slug>/report/report.md` | the as-built report bundle |
| in (optional) | `report/*.mmd`, `report/diagrams.html`, `report/manifest.json` | the as-built UML set + viewer |
| in (optional) | `report/before/*.mmd` + `report/before/manifest.json` | the plan-time "before" set (FR8) → viewer compare mode |
| in (board mode) | the shared **work-index** (gogo-status Step A classifier, in-memory) | the four-class record shape the board consumes |
| in (assets) | `${CLAUDE_PLUGIN_ROOT}/assets/kanban/board.py` | vendored terminal-TUI (copied on demand; soft dep) |
| in (assets) | `${CLAUDE_PLUGIN_ROOT}/assets/{mermaid/mermaid.min.js, viewer/*}` | vendored viewer runtime (copied on demand) |
| out | `.gogo/changelog/<YYYY-MM-DD>-<slug>/` (copy of the bundle, incl. `before/`) | append-only archive |
| out | `.gogo/resources/view/<date>-<slug>.html` (interactive viewer page, best-effort) | self-contained offline page |
| out (board mode) | `.gogo/resources/kanban/{board.py, work-index.json, ship-result.json, board-exit.code}` | runtime scratch for the TUI (`.gogo/`-only) |
| out | `state.md` (status → `shipped`) | human state |

## ① validate-in (gate)

- **`<slug>` given** → confirm that feature is **report-complete**:
  `.gogo/work/feature-<slug>/report/report.md` exists. Missing → **STOP** with
  exactly this guidance (name the feature):

  > No report found for `<feature>` — run `/gogo:report <feature>` first, then `/gogo:done`.

- **No slug (board mode)** → no hard prerequisite: the board classifies whatever
  exists. If the classifier finds **zero** ready-to-ship features, say so plainly
  ("nothing is report-complete yet — run `/gogo:report <feature>` first") and stop
  without opening an empty board.

`/gogo:report` works even on a past/broken run (it writes a best-effort report),
so the guidance is always actionable. Never archive a feature that hasn't been reported.

## ② Resolve mode

From `$ARGUMENTS`:
- a **slug** → run **Ship one feature** (below) for exactly that slug;
- **empty** → run **Board mode** (below), which selects the ready-to-ship slugs and
  then runs **Ship one feature** for each. (Back-compat: if there is exactly one
  report-complete feature and no slug, you may ship it directly — but prefer the
  board so the user sees the full picture.)

## Ship one feature (`<slug>`) — the single shipping flow

This is the one place shipping happens. Both `--slug` and the board call it (the
board loops it over the selected slugs). It is idempotent and `.gogo/`-only.

1. **Derive the date** for the changelog entry — **do not hardcode**:
   - prefer the report's `- **completed:** <YYYY-MM-DD>` field. That value is
     markdown-bolded, so extract the ISO date itself — never a naive
     `sed 's/.*completed://'` (which would capture the trailing `**`):
     ```bash
     date=$(grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}' .gogo/work/feature-${slug}/report/report.md | head -1)
     ```
   - else a date the user supplied;
   - else today's date (`date +%F`).
2. **Copy (never move) the bundle** into `.gogo/changelog/<date>-<slug>/`: the
   `report/report.md`, every `report/*.mmd`, `report/diagrams.html`,
   `report/manifest.json` if present, **and the `report/before/` set** (the
   plan-time "before" UML + its manifest, FR8) so the archive is self-contained and
   the viewer's before/after compare works from the changelog entry alone. The work
   folder stays the working source.
   ```bash
   set -euo pipefail
   slug="<slug>"; date="<derived-date>"
   src=".gogo/work/feature-${slug}/report"
   dst=".gogo/changelog/${date}-${slug}"
   [ -f "${src}/report.md" ] || { echo "not report-complete: ${src}/report.md missing"; exit 1; }
   mkdir -p "${dst}"
   cp "${src}/report.md" "${dst}/report.md"
   cp "${src}"/*.mmd "${dst}/" 2>/dev/null || true
   [ -f "${src}/diagrams.html" ] && cp "${src}/diagrams.html" "${dst}/diagrams.html" || true
   [ -f "${src}/manifest.json" ] && cp "${src}/manifest.json" "${dst}/manifest.json" || true
   # the before/ set (FR8) — copy it in so the archive is self-contained + compare mode works
   [ -d "${src}/before" ] && { mkdir -p "${dst}/before"; cp "${src}/before"/* "${dst}/before/" 2>/dev/null || true; } || true
   ```
   **Idempotent:** re-running for the same `<date>-<slug>` overwrites that same
   dated dir (a refreshed report re-ships cleanly); it never creates duplicates and
   never deletes anything outside the target dir.
3. **Mark the feature terminal.** Set `state.md`: `status: shipped`, `resume: none`
   (leave `phase: done`). Note the changelog path in the resume/summary line.
4. **Build the interactive viewer page for this entry (FR10, best-effort).** Reuse
   the **`gogo-view` build** — don't reimplement it — so the shipped entry gets the
   same xplan-style interactive page (draggable token-styled nodes + owned edge
   layer + minimap for flowchart-family, pan/zoom fallback otherwise, and
   **before/after compare** when the entry carries a `before/` set). Load the
   `gogo-view` skill and run its build against the **just-archived changelog entry**
   (`.gogo/changelog/<date>-<slug>/`), writing the page to
   `.gogo/resources/view/<date>-<slug>.html` — i.e. gogo-view's **Step 2 (ensure
   shared resources)** then **Step 3 (build the page)**, but **skip its Step 4
   auto-open** (this skill prints the link in Return instead). Ensure the vendored
   runtime is present first (copy from `${CLAUDE_PLUGIN_ROOT}` only if missing):
   ```bash
   set -euo pipefail
   mkdir -p .gogo/resources/viewer .gogo/resources/view
   [ -f .gogo/resources/mermaid.min.js ] || \
     cp "${CLAUDE_PLUGIN_ROOT}/assets/mermaid/mermaid.min.js" .gogo/resources/mermaid.min.js
   cp "${CLAUDE_PLUGIN_ROOT}"/assets/viewer/*.js       .gogo/resources/viewer/ 2>/dev/null || true
   cp "${CLAUDE_PLUGIN_ROOT}/assets/viewer/viewer.css" .gogo/resources/viewer/viewer.css 2>/dev/null || true
   ```
   Then assemble the page from the archived bundle exactly as gogo-view Step 3 does
   (template tokens; `report.md` → HTML summary; one `figure.diagram` per archived
   `.mmd`; compare-pair markup when `before/*.mmd` sits beside `report.md`; seed
   `GOGO_VIEW_LAYOUT` from `.gogo/resources/view/<date>-<slug>.layout.json` if it
   exists, else `{}`). **`.gogo/`-only, offline** — no network, no `http(s)://`.
   **Best-effort + graceful:** if the page can't be built (mermaid missing, no
   diagrams, or any build error), do **not** fail `/gogo:done` — skip the page and
   let Return fall back to the archived `diagrams.html` / folder path.

## Board mode (no slug) — the work board (D5=A)

The board is a **selector/visualizer** over every work item. It never archives or
mutates state — on confirm it hands a list of slugs to **Ship one feature**.

1. **Build the work-index.** Run the shared **gogo-status Step A classifier**
   (`skills/gogo-status/SKILL.md`) to label every `.gogo/work/feature-*` as
   **shipped · ready-to-ship · in-progress · unfinished**, newest-first, in its
   documented record shape (`slug`, `title`, `status`, `class`, …). This is the same
   read-only classifier `/gogo:status` renders — reuse it, don't re-derive.
2. **Choose the surface.** The interactive terminal kanban is used only when all of
   these hold; otherwise fall back (never fail over the board):
   - `python3` is available (`command -v python3`),
   - `tmux` is available (`command -v tmux`),
   - there is an interactive **tty** (`[ -t 0 ] && [ -t 1 ]`, or a resolvable
     `$TMUX` / terminal).

   **Interactive TUI path (all three present).** `board.py`'s exit codes are the
   contract: **0** = confirmed (result written, possibly `{"ship":[]}`), **1** =
   user cancel (no result), **2** = error (bad/missing index or cannot start). A
   tmux *client's* own exit status is unreliable, and `tmux new-session` refuses to
   **nest** when `$TMUX` is already set (the norm for tmux users) — so launch
   nesting-safely and capture the **board's own** exit code, then branch on the
   three outcomes below. Never assume `new-session` works.
   ```bash
   set -euo pipefail
   mkdir -p .gogo/resources/kanban
   cp "${CLAUDE_PLUGIN_ROOT}/assets/kanban/board.py" .gogo/resources/kanban/board.py  # vendored, idempotent copy
   idx=".gogo/resources/kanban/work-index.json"     # write the classifier records here (Write tool) first
   res=".gogo/resources/kanban/ship-result.json"    # board writes this ONLY on confirm
   code=".gogo/resources/kanban/board-exit.code"    # the board's OWN exit code (tmux's is unreliable)
   rm -f "$res" "$code"
   sess="gogo-done-$$"                               # unique target -> a stale/duplicate session can't block the launch
   # record the board's exit code, then signal a wait-for channel so we can block on it
   run="python3 '.gogo/resources/kanban/board.py' --index '$idx' --result '$res'; echo \$? > '$code'; tmux wait-for -S '$sess'"
   if [ -n "${TMUX:-}" ]; then
     # already inside tmux: new-session would refuse to nest -> run in a NEW WINDOW, then block on the channel
     tmux new-window -n "$sess" "$run" && tmux wait-for "$sess" 2>/dev/null || true
   else
     # outside tmux: an attached, uniquely-named session blocks until the board exits
     tmux new-session -A -s "$sess" "$run" || true
   fi
   ```
   - Write the classifier records array to `$idx` first (the board reads
     `{slug, class, title, status}`; extra keys are ignored). `board.py` renders the
     four columns; the user moves the cursor (arrows/hjkl), **space/enter** toggles a
     **ready-to-ship** card (only those are selectable), **s** ships the selection,
     **q** cancels.
   - **Branch on the three outcomes — a launch failure or an error is NOT a cancel:**
     ```bash
     if [ -f "$res" ]; then
       # confirmed (exit 0): parse {"ship":[...]} and run "Ship one feature" per slug
       ships=$(jq -r '.ship[]?' "$res" 2>/dev/null) || ships=""   # else read $res with the Read tool
       # -> ship each slug in $ships (an empty list means the user picked nothing: ship nothing, done)
     elif [ -f "$code" ] && [ "$(cat "$code")" = "1" ]; then
       echo "board cancelled — nothing shipped"                    # the board RAN and the user quit: stop
     else
       echo "board did not run (launch failed / exit 2 / error) — using the status-table fallback"
       # -> fall through to the Step 3 fallback (status table + AskUserQuestion)
     fi
     ```
     Only the **middle** branch is a real cancel: the board actually ran and returned
     `1` with no result. If `$code` is **missing** (tmux never started the board:
     nested `$TMUX`, a stale session, a missing binary) OR is **2** (bad/missing
     index) OR any other non-`0`/`1` value, treat it as a **board error → the
     guaranteed fallback** — never silently do nothing (Degradation rule; matches
     `charts/done-board-flow.mmd`: "board error → fallback, never fail over the board").
3. **Fallback path (no tmux / no python3 / no tty / tmux launch failure /
   `board.py` exit 2 / board error).** **Never** fail over the board — degrade to
   the guaranteed in-terminal flow:
   - Render the work-index as a **status table** grouped by class (shipped ·
     ready-to-ship · in-progress · unfinished) so the user sees the full picture.
   - Offer the **ready-to-ship** items via **`AskUserQuestion` multi-select** ("which
     features to ship?"). Non-ready items are shown for context but are **not**
     selectable (same guard the TUI enforces).
   - For each chosen slug, **run Ship one feature**.
   - (You may also drive `board.py` headlessly as the emit step —
     `board.py --index <idx> --result <res> --headless --ship <slug,slug>` — which
     applies the same ready-to-ship guard and writes the result file; then ship each.
     The `AskUserQuestion` multi-select is the primary fallback UI.)
4. **Ship the selection.** For every selected slug, run **Ship one feature** (date
   derive → copy bundle → mark terminal → build viewer page). Because that flow is
   idempotent and `.gogo/`-only, shipping N features is just the single flow looped.
   Skip (with a one-line note) any selected slug that turns out not to be
   report-complete — never archive a feature without a `report/report.md`.

## ③ Return

- **Single slug** — a one-line confirmation: which bundle was archived, to which
  `.gogo/changelog/<date>-<slug>/`, and that `state.md` is now `shipped`.
- **Board mode** — one confirmation line **per shipped feature** (archived path +
  `shipped`), and note any the user left unselected.

Then, for each shipped entry, **print the interactive viewer link** — the absolute
`file://` URL to the built page — plus the archived static `diagrams.html` path as a
fallback:
```bash
page=".gogo/resources/view/${date}-${slug}.html"
if [ -f "$page" ]; then
  abs="$(cd "$(dirname "$page")" && pwd)/$(basename "$page")"
  echo "Interactive viewer: file://$abs"
fi
static=".gogo/changelog/${date}-${slug}/diagrams.html"
[ -f "$static" ] && echo "Static fallback:    file://$(cd "$(dirname "$static")" && pwd)/$(basename "$static")" || true
```
If the interactive page wasn't built, print the static `diagrams.html` link (or the
changelog folder path) instead — **never fail `/gogo:done` over the link**. Also
mention `/gogo:view` re-opens any entry any time.

## Degradation

- **No `tmux` / no `python3` / no tty / a tmux launch failure (e.g. nested `$TMUX`,
  stale session) / `board.py` exit 2 / any board error** → the status table +
  `AskUserQuestion` multi-select fallback (above). Only a clean board run that
  returns `1` with no result file is a real user cancel (stop, ship nothing); every
  other no-result outcome routes to the fallback, never a silent no-op. The board is
  a convenience layered on top; the classify → select → ship result is identical
  either way.
- If a diagram artifact is absent (a pure-process feature drew nothing), copy what
  exists — `report.md` alone is a valid entry. If `cp` of the glob fails because
  there are no `.mmd` files, that's a no-op, not an error.
- If the viewer page can't be built (mermaid runtime missing, no diagrams, or a
  build error), fall back to printing the archived `diagrams.html` / changelog folder
  path — the archive + the `shipped` state are the durable result; the link is a
  convenience layered on top.
