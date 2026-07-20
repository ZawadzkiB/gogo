---
name: gogo-done
user-invocable: false
description: >-
  The "ship it" step after phase ⑤ — when the user declares work done, write a
  high-level entry into the append-only .gogo/changelog/<YYYY-MM-DD>-<name>/ archive:
  a SYNTHESIZED report.md (what was changed/done/implemented, key outcomes, one-line
  decisions, review/test verdict — written, never a copy of the full report bundle;
  the audit trail stays in .gogo/work/, linked), the slug-prefixed diagram .mmd set, a
  manifest.json carrying a members[] array, and the before/ set. One OR several related
  work items can ship as ONE merged release entry. It builds the interactive viewer
  page for the entry (reusing the gogo-view build) and prints its file:// link, and
  sets each member's state.md to a terminal `shipped` status. With NO slug it opens the
  interactive work board — the pipeline's COCKPIT (terminal-TUI kanban, or a status
  table + multi-select fallback): from the four-class table the user can view any card
  (v), ship the selection separately (s) or merged (m), run/resume the pipeline on an
  unbuilt card (g), and filter by text (/). Each key writes a single-shot INTENT and the
  board relaunches after the orchestrator executes it (go ends the loop and hands off;
  cancel stops); when the selection is shipped merged the user is asked for a release
  name. Use when the user runs /gogo:done or says work is shipped / finished / released.
  Synthesis-not-copy, idempotent, writes only under .gogo/, offline.
---

# gogo-done — synthesize report-complete features into the changelog

The explicit post-report gate. `/gogo:report` (⑤) finalizes the full report bundle in
the **work** folder (the audit trail); `/gogo:done` is the user saying *"this is
shipped"* — it **synthesizes a high-level changelog entry** from that work (*what was
changed/done/implemented*, key outcomes, decisions, review/test verdict), **builds +
prints the interactive viewer link** for the entry, and marks the member feature(s)
terminal. The changelog reads like a release history; the full detail stays where it
already lives, in `.gogo/work/feature-<slug>/` (linked from the entry).

Three ways in, one entry-writer:
- **`/gogo:done <slug>`** — ship that one feature as a single-member entry.
- **`/gogo:done slug1+slug2+slug3`** — ship those `+`-joined features as ONE **merged**
  release entry (the `+` pre-answers the merge gate; skips the board).
- **`/gogo:done`** (no slug) — open the **work board** (D5=A), the pipeline's
  **cockpit** (D1=A/D2=A/D3=A of this feature): classify every `.gogo/work/feature-*`
  and, from the four-class table, the user can **view** any card, **ship** ready cards
  (separately or **merged**), **go** (run/resume the pipeline) on an unbuilt card, and
  **filter** by text. Each action key writes a single-shot **intent** and exits; the
  orchestrator executes it and **relaunches** the board (so it feels persistent). An
  explicit `s` (separate ship) does **not** ask the merge gate; `m` (merge) ships all
  picks as one entry (release-name confirmed in chat).

Either way the actual shipping is the single **"Write changelog entry (1..N members)"**
flow below — the board only *collects intents*; it never archives anything itself or
mutates gogo state (D5). Pure `Read` / `Write` / `Bash` (+ `Skill` to reuse the
`gogo-view` build and to hand off `go` to the pipeline); only ever writes under
`.gogo/`; offline.

## Inputs (declared) and outputs

| Direction | Artifact | Contract |
|---|---|---|
| in (required, per member) | `.gogo/work/feature-<slug>/report/report.md` | the as-built report — the **synthesis source**, never copied |
| in (optional, per member) | `report/*.mmd`, `report/manifest.json` | the as-built UML set + its index (kinds/titles) |
| in (optional, per member) | `report/before/*.mmd` + `report/before/manifest.json` | the plan-time "before" set (FR8) → viewer compare mode |
| in (board mode) | the shared **work-index** (gogo-status Step A classifier, in-memory) | the four-class record shape the board consumes; the orchestrator also uses each card's `class` to route a `view` intent |
| in (assets) | `${CLAUDE_PLUGIN_ROOT}/assets/kanban/board.py` | vendored terminal-TUI cockpit (copied on demand; soft dep) |
| in (assets) | `${CLAUDE_PLUGIN_ROOT}/assets/{mermaid/mermaid.min.js, viewer/*}` | vendored viewer runtime (copied on demand) |
| in (board mode) | `.gogo/resources/kanban/board-intent.json` — the board's schema-v2 **intent** `{schema:2, action, items}` (legacy `{"ship":[...]}` accepted as `action:ship`) | what the orchestrator reads + routes each loop iteration |
| out | `.gogo/changelog/<YYYY-MM-DD>-<name>/` — **synthesized** `report.md` + slug-prefixed `*.mmd` + `manifest.json` (with a `members[]` array) + `before/` | append-only archive; **no `diagrams.html` copy** |
| out | `.gogo/resources/view/<date>-<name>.html` (interactive viewer page, best-effort) | self-contained offline page |
| out (board mode) | `.gogo/resources/kanban/{board.py, work-index.json, board-intent.json, board-exit.code}` | runtime scratch for the TUI (`.gogo/`-only) |
| out (per member) | `.gogo/work/feature-<slug>/uat.md` — the UAT accept round appended before shipping | append-only gate log |
| out | each member's `state.md` (status → `shipped`) | human state |

## ① validate-in (gate) — report-complete **and** at the UAT gate

- **`<slug>` or `slug1+slug2+...` given** → confirm **each** named feature is
  **report-complete**: `.gogo/work/feature-<slug>/report/report.md` exists. Any missing
  → **STOP** naming the missing feature(s):

  > No report found for `<feature>` — run `/gogo:report <feature>` first, then `/gogo:done`.

  **The UAT gate (from 0.11.0 — the plan-gate symmetry).** Phase ⑤ now ends at
  `state.md` `status: awaiting-uat`, and **running `/gogo:done` IS the UAT acceptance**
  (there is no extra confirmation question — mirroring how accepting a plan unlocks
  `/gogo:go`). So for each named member require `status: awaiting-uat`.

  **Pre-declared skip (`--skip-uat`).** When a member's SOURCE opted out of the UAT gate
  via `uatAcceptanceSkip`, the gogo orchestrator auto-invokes this ship for that member
  (rather than a human running `/gogo:done`) — the acceptance is **pre-declared in the
  source's CLI config**, not a silent bypass. Nothing changes here: the accept round + the
  single-owner `uat-passed` event recorded below are byte-for-byte identical whether the
  ship was human-run or auto-invoked (see the gogo orchestrator's *UAT → Pre-declared
  skip* note). **Back-compat:**
  a pre-0.11 feature reported at `status: done` (or any already-shipped `done`/`shipped`
  member) is **also accepted** — note in the run summary that it predates the UAT gate.
  **A report-complete member at `waiting-for-user` is REFUSED** — that status means a
  **mid-UAT re-plan is in progress** (the user raised issues, the plan was revised but not
  yet re-accepted or re-implemented), so its `report.md`/`plan.md` no longer match the
  code and it must not ship. **STOP**:

  > `<feature>` is mid-UAT re-plan (`waiting-for-user`) — re-accept the adjusted plan and
  > run `/gogo:go` to rerun ②→⑤, landing back at `awaiting-uat`, before shipping.

  A member still mid-pipeline (`implementing`/`reviewing`/`testing`) has no report yet, so
  the report-complete check above already stops it. Net: ship a report-complete member
  only at `awaiting-uat` (normal path) or `done` (legacy) — **never** at
  `waiting-for-user`.

- **No slug (board mode / cockpit)** → no hard prerequisite: the board classifies
  whatever exists. If there are **zero work items at all** (`.gogo/work/feature-*`),
  say so plainly ("no features yet — run `/gogo:plan` first") and stop without opening
  an empty board. If there are items but **none ready-to-ship**, the cockpit still
  opens — its `view` / `go` / `filter` work on any card (D3=A: `g` runs/resumes
  unbuilt work); the user just can't `ship` / `merge` until something is
  report-complete (note that plainly, and point at `/gogo:report <feature>`).

`/gogo:report` works even on a past/broken run (it writes a best-effort report), so the
guidance is always actionable. Never write an entry for a feature that hasn't been reported.

## ② Resolve mode

From `$ARGUMENTS`:
- a single **slug** → run **Write changelog entry** with that one member (a single entry);
- a **`+`-joined list** (`slug1+slug2+...`) → **merge**: derive the release name (D2=A)
  + newest member date, then run **Write changelog entry** with those members as ONE
  entry. The `+` pre-answers the separate-vs-merged gate, so the board is skipped;
- **empty** → run **Board mode** (below), which selects the ready-to-ship slugs, applies
  the merge gate, and then runs **Write changelog entry**. (Back-compat: if there is
  exactly one report-complete feature and no slug, you may write it directly — but
  prefer the board so the user sees the full picture.)

## Write changelog entry (1..N members) — the single entry-writer

This is the one place shipping happens. `<slug>`, `slug1+slug2`, and the board all call
it (with 1, N pre-answered, or the selected members). It is idempotent and `.gogo/`-only.
A single member (`members = [<slug>]`) and a merged set share **one shape** — the only
difference is 1 vs N members; there is no divergent single-vs-merged code path.

**Slim by design — plain file ops + synthesis (D2).** The whole job is *"prepare the
changelog entry from the work item's report + files"*: **Read** the report(s), **Write**
the synthesized `report.md` + `manifest.json` + the `uat.md` accept round, **copy** the
`.mmd`/`before/` set, and flip each `state.md` — nothing here **requires** running a
script, so a board-launched session in Claude's **auto (classifier) permission mode**
covers it without a bypass or an approval nag. The `bash` blocks below (date derivation,
the `cp`/`mkdir`/`rm` assembly) are `.gogo/`-only file conveniences; where a plain Read
suffices (e.g. reading a member's `completed:` line) a Read is equally fine, and any
`jq`/`python` stays **optional + graceful** — never a hard dependency, never a required
approval.

1. **Resolve + validate the members.** For each member slug, require
   `.gogo/work/feature-<slug>/report/report.md`. Skip (with a one-line note) any slug
   that isn't report-complete — never write an entry for a feature without a report. If
   nothing report-complete remains, stop.

   **Record the UAT acceptance FIRST (FR4 — the plan-gate symmetry).** For **each**
   member, before anything else, append a UAT accept round to its
   `.gogo/work/feature-<slug>/uat.md` (create the file from
   `${CLAUDE_PLUGIN_ROOT}/templates/uat.template.md` if absent) — a one-line verdict:

   > `## UAT round N — accepted (user, <YYYY-MM-DD>) — via /gogo:done`

   where `N` continues that member's existing round numbering (1 if there are no prior
   rounds). Running `/gogo:done` **is** the acceptance, so this is the record of it — no
   separate confirmation question is asked. Then ship exactly as today. (A legacy member
   at `status: done` that never had a UAT gate gets the same one-line accept round, noted
   as pre-0.11.) This is a plain **Write/append** — no script, auto-mode-safe.

2. **Derive the entry date + name — do not hardcode.**
   - **Date** = the **newest** member's `- **completed:** <YYYY-MM-DD>` field (the value
     is markdown-bolded, so extract the ISO date itself — never a naive
     `sed 's/.*completed://'`); else a date the user supplied; else today (`date +%F`).
     For a single member it is simply that member's `completed:`.
     ```bash
     newest=""
     for slug in <members>; do
       d=$(grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}' ".gogo/work/feature-${slug}/report/report.md" | head -1)
       if [ -n "$d" ] && { [ -z "$newest" ] || [ "$d" \> "$newest" ]; }; then newest="$d"; fi
     done
     date="${newest:-$(date +%F)}"
     ```
   - **Name** — **single member** → the slug (entry dir `<date>-<slug>`). **Merged** →
     derive a suggested **release name** from the members' common theme (the longest
     shared slug word / prefix, e.g. `appointments-*` → `appointments`), kebab-case it,
     and **confirm via one `AskUserQuestion`** (D2=A — the user can override; a bad name
     is annoying to live with in an append-only archive). Entry dir `<date>-<release-name>`.

3. **Synthesize the high-level `report.md` (FR2 — written, never copied).** Read each
   member's `report/report.md` (and `decisions.md` where useful) and **WRITE** a
   high-level entry — *what was changed / done / implemented* — with the **Write tool**,
   never `cp`:
   - a **lead paragraph**: what shipped + the key outcomes;
   - **one-line decisions**: the important forks and how they resolved;
   - a **one-sentence review/test verdict**;
   - **N>1** → a **member table** (`slug` · title · one-line outcome) + a short section
     per member; **N=1** → the condensed synthesis of that one feature;
   - a **link back** to each member's `.gogo/work/feature-<slug>/` for the full audit
     trail (review/test rounds, per-file changes table, decisions detail).
   It is a *synthesis* — do **not** duplicate any source `report.md` verbatim.

4. **Assemble the slim file set** — `report.md` + `*.mmd` + `manifest.json` + `before/`
   **only** (the static `diagrams.html` is **dropped**; `/gogo:view` builds the
   interactive page from these sources). Flatten every member's diagrams to
   **slug-prefixed** names (`<slug>-<name>.mmd`) and merge every member's `before/` the
   same way, so a merged entry keeps the viewer's **flat** layout (and a single entry is
   the same shape with one member). Clear the target's diagram set first so a re-ship
   with a changed member set never leaves stale prefixed files (`.gogo/`-only; only ever
   inside the target dir):
   ```bash
   set -euo pipefail
   date="<derived-date>"; name="<entry-name>"      # slug for single, release-name for merged
   dst=".gogo/changelog/${date}-${name}"
   # Guard BEFORE any delete: $dst must be non-empty AND resolve under .gogo/changelog/.
   # An empty date/name would collapse $dst toward the filesystem root — the exact shape
   # the harness's "dangerous rm" classifier fears. The deletes below use scoped
   # `find ... -delete` (no glob-rm, no bare-variable `rm`) for the same reason: gogo's
   # own mechanical cleanup must run prompt-free and never escape the target dir.
   [ -n "$date" ] && [ -n "$name" ] || { echo "refuse: empty changelog date/name" >&2; exit 1; }
   case "$dst" in
     .gogo/changelog/*) : ;;
     *) echo "refuse: changelog dir '$dst' escapes .gogo/changelog/" >&2; exit 1 ;;
   esac
   mkdir -p "$dst"
   # Refresh the entry's diagram set in place — empty-$dst-safe + idempotent.
   # Top level: only *.mmd (the entry's report.md + manifest.json must survive).
   find "$dst" -maxdepth 1 -type f -name '*.mmd' -delete
   # before/ holds only the copied diagram set — clear it WHOLE (any file), matching the
   # old `rm -rf "$dst/before"` exactly, then drop the now-empty dir. Still a scoped find
   # under the guarded $dst (no glob-rm, no bare-variable rm).
   find "$dst/before" -type f -delete 2>/dev/null || true
   rmdir "$dst/before" 2>/dev/null || true
   for slug in <members>; do
     src=".gogo/work/feature-${slug}/report"
     [ -f "${src}/report.md" ] || { echo "skip ${slug}: no report.md"; continue; }
     for f in "${src}"/*.mmd; do [ -e "$f" ] || continue; cp "$f" "${dst}/${slug}-$(basename "$f")"; done
     if [ -d "${src}/before" ]; then
       mkdir -p "${dst}/before"
       for f in "${src}/before"/*.mmd; do [ -e "$f" ] || continue; cp "$f" "${dst}/before/${slug}-$(basename "$f")"; done
     fi
   done
   # report.md is WRITTEN (step 3) with the Write tool — never copied.
   # manifest.json is WRITTEN (step 5) with the Write tool.
   ```
   Then **write ONE `manifest.json`** (Write tool) for the entry: `slug` = the entry
   name, and a `diagrams[]` array whose **every** entry is schema-complete —
   `{ kind, file, title }` — so the written manifest **validates against**
   `templates/contracts/charts-manifest.schema.json`:
   - **`kind`** — one of the schema's kinds (`flow` / `sequence` / `class` / `activity`
     / `use-case`), **carried over** from the member's source `report/manifest.json`;
     if absent there, **infer** it from the `.mmd`'s first header line
     (`flowchart`/`graph` → `flow`, `sequenceDiagram` → `sequence`, `classDiagram` →
     `class`, `stateDiagram` → `activity`);
   - **`file`** — the slug-prefixed `.mmd` basename written into the entry dir, relative
     to the entry (e.g. `<slug>-flow.mmd`);
   - **`title`** — the human title **slug-prefixed** (`<slug>: <title>`).
   Plus a **`members` array** — `[<slug>]` for a single entry, `[slug1, slug2, ...]` for a
   merged one (the additive optional key the schema now allows; it is what `gogo-status`
   reads to classify a merged entry's members).
   **Idempotent:** re-running for the same `<date>-<name>` overwrites that same dated dir
   (a refreshed report re-ships cleanly); it never creates duplicates and never deletes
   anything outside the target dir.

5. **Mark each member terminal (FR3).** For **every** member, set its `state.md`:
   `status: shipped`, `resume: none — shipped to .gogo/changelog/<date>-<name>/`
   (leave `phase: done`). This is what lets `/gogo:status` and the board treat a merged
   entry's members as shipped even though the folder is named after the release.

   **Append the ship events (telemetry).** Beside each member's `state.md` write,
   append two compact JSON lines to that member's
   `.gogo/work/feature-<member-slug>/events.jsonl` per `events.schema.json`
   (`${CLAUDE_PLUGIN_ROOT}/templates/contracts/`) — **first the UAT-pass**, then the
   ship, so the timeline reads accept → ship:
   `{"ts":"<RFC3339>","event":"uat-passed","phase":"done","status":"awaiting-uat","note":"accepted via /gogo:done","slug":"<member-slug>"}`
   then
   `{"ts":"<RFC3339>","event":"shipped","phase":"done","status":"shipped","note":".gogo/changelog/<date>-<name>/","slug":"<member-slug>"}`
   (for a merged entry the ship `note` may also list the members). `uat-passed` is the
   UAT gate's acceptance event — **this skill (`gogo-done`) owns it** (the orchestrator
   owns `uat-opened`/`uat-failed` for the feedback loop). `shipped` is the done phase's
   **terminal** event — this skill owns it and there is no `phase-done`/done. Create the
   file if absent; **best-effort** — never fail `/gogo:done` if the append fails
   (append-only telemetry; `state.md` stays the human resume file).

6. **Reap the shipped session(s) at ship (FR1/FR2 — best-effort, TARGETED).** Now that
   every member is `shipped` (step 5 flipped its `state.md` first — order matters, so
   the sweeper sees the feature as terminal), reap the live `gogo-*` tmux session(s)
   that drove them so a just-shipped card never shows a phantom "● session running"
   badge and nobody runs `gogo sweep` by hand. Run a **targeted** sweep — pass **the
   shipped member slug(s)** as arguments so the reap touches **only this ship's own
   cards**, never another feature's session. One best-effort, classifier-safe line
   (substitute the actual member slugs — one for a single ship, all of them for a
   merged entry):
   ```bash
   command -v gogo >/dev/null 2>&1 && gogo sweep <member-slug>... >/dev/null 2>&1 || true
   ```
   `gogo sweep <slug>...` (D4=B) restricts the reap to sessions attributing (exact
   `SessionMatchesSlug` parse) to the named slug(s): because those members are already
   terminal it kills their `gogo-go-<slug>` / `gogo-plan-<slug>` driving sessions —
   and, thanks to the sweeper's **self-guard (FR3)**, never the `gogo-done-<slug>`
   session hosting *this* `/gogo:done` (the board's `d`/`m` keys launch `/gogo:done`
   into such a session). Passing the slugs (not a bare `gogo sweep`) is what stops a
   ship from truncating a **different** feature's concurrent `/gogo:done` (REV-002) —
   a bare whole-board `gogo sweep` stays the user's **manual** orphan cleanup, not the
   ship path. **Best-effort (D3=A):** if `gogo` is not on PATH, tmux is absent, or the
   sweep errors, the guard swallows it and the ship still proceeds — the standalone
   `gogo sweep` / next-launch reap stays the backstop. Never fails a ship; writes
   nothing itself under `.gogo/`.

7. **Build the interactive viewer page for the entry (FR10, best-effort).** Reuse the
   **`gogo-view` build** — don't reimplement it — so the entry gets the same xplan-style
   interactive page (draggable token-styled node cards + owned edge layer + minimap for
   flowchart-family, pan/zoom fallback otherwise, and **before/after compare** when the
   entry carries a `before/` set; the prefixed `.mmd`/`before/` names pair by basename,
   so compare mode still matches per member per kind). Load the `gogo-view` skill and run
   its build against the **just-written changelog entry** (`.gogo/changelog/<date>-<name>/`),
   writing the page to `.gogo/resources/view/<date>-<name>.html` — i.e. gogo-view's
   **Step 2 (ensure shared resources)** then **Step 3 (build the page)**, but **skip its
   Step 4 auto-open** (this skill prints the link in Return instead). Ensure the vendored
   runtime is present first (copy from `${CLAUDE_PLUGIN_ROOT}` only if missing):
   ```bash
   set -euo pipefail
   mkdir -p .gogo/resources/viewer .gogo/resources/view
   [ -f .gogo/resources/mermaid.min.js ] || \
     cp "${CLAUDE_PLUGIN_ROOT}/assets/mermaid/mermaid.min.js" .gogo/resources/mermaid.min.js
   cp "${CLAUDE_PLUGIN_ROOT}"/assets/viewer/*.js       .gogo/resources/viewer/ 2>/dev/null || true
   cp "${CLAUDE_PLUGIN_ROOT}/assets/viewer/viewer.css" .gogo/resources/viewer/viewer.css 2>/dev/null || true
   ```
   Then assemble the page from the entry exactly as gogo-view Step 3 does (template
   tokens; the synthesized `report.md` → HTML summary; one `figure.diagram` per prefixed
   `.mmd`; compare-pair markup when `before/*.mmd` sits beside `report.md`; seed
   `GOGO_VIEW_LAYOUT` from `.gogo/resources/view/<date>-<name>.layout.json` if it exists,
   else `{}`). **`.gogo/`-only, offline** — no network, no `http(s)://`. **Best-effort +
   graceful:** if the page can't be built (mermaid missing, no diagrams, or any build
   error), do **not** fail `/gogo:done` — skip the page and let Return fall back to the
   changelog folder path.

## Board mode (no slug) — the work cockpit (D5=A · D1/D2/D3=A)

The board is a **selector/visualizer** over every work item — the pipeline's
**cockpit**. It never archives or mutates gogo state (D5): every action key just
writes a single-shot **intent** and exits; the orchestrator **executes** the intent
and **relaunches** the board, so it feels persistent. The loop continues until the
user picks **go** (hand off) or **cancel** (stop).

1. **Build the work-index.** Run the shared **gogo-status Step A classifier**
   (`skills/gogo-status/SKILL.md`) to label every `.gogo/work/feature-*` as
   **shipped · ready-to-ship · in-progress · unfinished**, newest-first, in its
   documented record shape (`slug`, `title`, `status`, `class`, `changelog_path`, …).
   This is the same read-only classifier `/gogo:status` renders — reuse it, don't
   re-derive. **Keep these records in hand:** the orchestrator uses each card's `class`
   (and `changelog_path`) to route a `view` intent to the right page.
2. **Choose the surface.** The interactive terminal kanban is used only when all of
   these hold; otherwise fall back (never fail over the board):
   - `python3` is available (`command -v python3`),
   - `tmux` is available (`command -v tmux`),
   - there is an interactive **tty** (`[ -t 0 ] && [ -t 1 ]`, or a resolvable
     `$TMUX` / terminal).

3. **Interactive TUI path — the relaunch loop (all three present).** Launch the board
   for **one iteration**, read its intent, **route** it (table below), and — for
   `view` / `ship` / `ship-merged` — **return here and relaunch** so the cockpit feels
   persistent. Only `go` and `cancel` end the loop.

   `board.py`'s exit codes are the contract: **0** = an action (a schema-v2 intent was
   written), **1** = user cancel (no intent file), **2** = error (bad/missing index or
   cannot start). A tmux *client's* own exit status is unreliable, and `tmux
   new-session` refuses to **nest** when `$TMUX` is already set (the norm for tmux
   users) — so launch nesting-safely and capture the **board's own** exit code. Never
   assume `new-session` works.
   ```bash
   set -euo pipefail
   mkdir -p .gogo/resources/kanban
   cp "${CLAUDE_PLUGIN_ROOT}/assets/kanban/board.py" .gogo/resources/kanban/board.py  # vendored, idempotent copy
   idx=".gogo/resources/kanban/work-index.json"       # write the classifier records here (Write tool) first
   res=".gogo/resources/kanban/board-intent.json"     # board writes the schema-v2 intent ONLY on an action
   code=".gogo/resources/kanban/board-exit.code"      # the board's OWN exit code (tmux's is unreliable)
   # Clear any stale intent/exit file before the board runs. Scoped `find` on the literal
   # dir + named files (no bare-variable `rm`) so this mechanical step never trips the
   # "dangerous rm" classifier and never needs a permission prompt.
   find .gogo/resources/kanban -maxdepth 1 -type f \( -name board-intent.json -o -name board-exit.code \) -delete 2>/dev/null || true
   sess="gogo-done-$$"                                 # unique target -> a stale/duplicate session can't block the launch
   # record the board's exit code, then signal a wait-for channel so we can block on it
   run="python3 '.gogo/resources/kanban/board.py' --index '$idx' --result '$res'; echo \$? > '$code'; tmux wait-for -S '$sess'"
   if [ -n "${TMUX:-}" ]; then
     # already inside tmux: new-session would refuse to nest -> run in a NEW WINDOW, then block on the channel
     tmux new-window -n "$sess" "$run" && tmux wait-for "$sess" 2>/dev/null || true
   elif [ -t 0 ] && [ -t 1 ]; then
     # a real tty: an attached, uniquely-named session blocks until the board exits
     tmux new-session -A -s "$sess" "$run" || true
   else
     # PROVEN detached-launch pattern (orchestrator shell has no tty): start detached, tell the
     # user to attach, then BLOCK on the wait-for channel + the board's own exit code.
     tmux new-session -d -s "$sess" "$run" || true
     echo "Board running in tmux — attach in another terminal:  tmux attach -t $sess"
     tmux wait-for "$sess" 2>/dev/null || true
   fi
   ```
   - Write the classifier records array to `$idx` first (the board reads
     `{slug, class, title, status}`; extra keys are ignored). `board.py` renders the
     four columns; the user moves the cursor (arrows/hjkl), **space/enter** toggles a
     **ready-to-ship** card (only those are selectable), **v** views the focused card,
     **s** ships the selection separately, **m** ships it merged (≥2), **g** runs/resumes
     the focused card, **/** filters, **q** cancels.
   - **First, sort a cancel / error from an action — a launch failure or error is NOT a
     cancel:**
     ```bash
     if [ ! -f "$res" ] && [ -f "$code" ] && [ "$(cat "$code")" = "1" ]; then
       echo "board cancelled — nothing shipped"        # the board RAN and the user quit (q): stop
     elif [ ! -f "$res" ]; then
       echo "board did not run (launch failed / exit 2 / error) — using the status-table fallback"
       # -> fall through to the Step 4 fallback (status table + AskUserQuestion)
     fi
     ```
     A real cancel is only: no `$res` **and** `$code` is `1` (the board ran and the user
     quit) → stop. If `$res` is **missing** and `$code` is **absent** (tmux never
     started the board: a missing binary, a stale session) OR is **2** (bad/missing
     index) OR any other non-`0`/`1` value, treat it as a **board error → the guaranteed
     fallback** — never silently do nothing (Degradation rule; matches
     `charts/board-cockpit-flow.mmd`: "board error → fallback, never fail over the board").
   - **Route the intent (exit 0 — `$res` exists).** Read `board-intent.json`
     (schema-v2 `{schema:2, action, items}`; **also accept the legacy `{"ship":[...]}`
     shape as `action:ship` for back-compat), then:

     | `action` | The orchestrator does | Then |
     |---|---|---|
     | **view** | Build + open the page for the focused card's class (look its `class` up in the Step-1 work-index): **unfinished / in-progress → `<slug>:plan`** (plan bundle), **ready-to-ship → `<slug>:report`** (work report), **shipped → its changelog `<date>-<name>`** (from `changelog_path`). Reuse the **`gogo-view` build** — don't reimplement it — and print its `file://` link. | **relaunch** the board (return to step 3) |
     | **ship** | Run **Write changelog entry** once **per slug** in `items` (each a single-member entry). Explicit `s` = **separate** → do **NOT** ask the separate-vs-merged gate. | **relaunch** |
     | **ship-merged** | Run **Write changelog entry** **once** with all `items` as `members[]` (derive + **confirm the release name** in chat, D2=A). | **relaunch** |
     | **go** | **END the loop** and hand off to the pipeline: resume the focused feature per its `state.md` (exactly like `/gogo:go <slug>`). | loop ends |
     | **cancel** (exit 1, no `$res`) | Stop — nothing shipped. | loop ends |

     After a `view`, `ship`, or `ship-merged` intent, **relaunch the board** (repeat
     step 3 with a freshly-written `$idx` if state changed — a just-shipped feature now
     classifies as `shipped`). Skip (with a one-line note) any `ship` slug that turns
     out not to be report-complete.
4. **Fallback path (no tmux / no python3 / no tty / tmux launch failure /
   `board.py` exit 2 / board error).** **Never** fail over the board — degrade to
   the guaranteed in-terminal flow. The fallback stays **ship-focused** (no relaunch
   loop, no view/go surface — it just ships):
   - Render the work-index as a **status table** grouped by class (shipped ·
     ready-to-ship · in-progress · unfinished) so the user sees the full picture.
   - Offer the **ready-to-ship** items via **`AskUserQuestion` multi-select** ("which
     features to ship?"). Non-ready items are shown for context but are **not**
     selectable (same guard the TUI enforces).
   - Hand the chosen slugs to **Step 5** (merge gate + entry-writer) — which **keeps**
     the separate-vs-merged gate for a ≥2 selection.
   - **Mention** that `/gogo:view <slug>` opens any card's page and `/gogo:go <slug>`
     runs/resumes the pipeline — the fallback doesn't surface `v` / `g` itself.
   - (You may also drive `board.py` headlessly as the emit step —
     `board.py --index <idx> --result <res> --headless --action ship --ship <slug,slug>`
     — which applies the same ready-to-ship guard and writes the intent. The
     `AskUserQuestion` multi-select is the primary fallback UI.)
5. **Merge gate + write (FR1) — the fallback's ship path.** With the fallback's
   selected slugs in hand (the TUI's explicit `s` / `m` already pre-answer this gate —
   `s` = separate, `m` = merged — so they skip straight to the writer):
   - **0 slugs** → nothing to do; say so and stop.
   - **1 slug** → run **Write changelog entry** with that one member (a single entry;
     **no** merge question is asked for N=1).
   - **≥2 slugs** → ask **one** `AskUserQuestion`: ship **separately** (N entries) or
     **merged** (1 entry)?
     - *separate* → run **Write changelog entry** once **per slug** (each a single-member
       entry).
     - *merged* → derive the release name (D2=A, confirm) + newest member date, then run
       **Write changelog entry** **once** with all selected members.
   Because the entry-writer is idempotent and `.gogo/`-only, N separate entries is just
   the one flow looped. Skip (with a one-line note) any selected slug that turns out not
   to be report-complete.

## ③ Return

- **Single entry** (`<slug>`, or one board pick, or one of N "separate" entries) — a
  one-line confirmation per entry: which member(s) were synthesized, to which
  `.gogo/changelog/<date>-<name>/`, and that each member's `state.md` is now `shipped`.
- **Merged entry** — one confirmation naming the release, its
  `.gogo/changelog/<date>-<release-name>/`, and the member slugs marked `shipped`.
- **Board mode (cockpit)** — the board relaunches after each `view` / `ship` /
  `ship-merged`, so confirm **per intent as it runs**: one line per changelog entry
  written (per ship / merge), the `file://` link per `view`, and note any features left
  unshipped when the user finally `q`-cancels. A **`go`** intent ends the board loop —
  say which feature the pipeline is resuming, then hand off (like `/gogo:go <slug>`).

Then, for each entry, **print the interactive viewer link** — the absolute `file://`
URL to the built page — with the changelog **folder path** as the fallback:
```bash
page=".gogo/resources/view/${date}-${name}.html"
if [ -f "$page" ]; then
  abs="$(cd "$(dirname "$page")" && pwd)/$(basename "$page")"
  echo "Interactive viewer: file://$abs"
else
  echo "Changelog entry:    $(cd ".gogo/changelog/${date}-${name}" && pwd)"
fi
```
If the interactive page wasn't built, print the changelog folder path instead —
**never fail `/gogo:done` over the link**. Also mention `/gogo:view` re-opens any entry
any time (it builds the page from the entry's `report.md` + `.mmd`).

## Degradation

- **No `tmux` / no `python3` / no tty / a tmux launch failure (e.g. nested `$TMUX`,
  stale session) / `board.py` exit 2 / any board error** → the status table +
  `AskUserQuestion` multi-select **ship** fallback (above). The fallback is
  ship-focused: no relaunch loop, no `v` / `g` surface — it just mentions `/gogo:view`
  and `/gogo:go`. Only a clean board run that returns `1` with no intent file is a real
  user cancel (stop, ship nothing); every other no-intent outcome routes to the
  fallback, never a silent no-op. The board is a convenience layered on top; the classify
  → select → merge gate → write result is identical for the ship path either way.
- If the ship-reap step (step 6) can't run — no `gogo` on PATH, no tmux, or `gogo sweep`
  errors — it is **skipped silently and the ship still completes** (D3=A). The session is
  then reaped by the next `gogo sweep` or opportunistically on the next `gogo go`/`gogo
  plan`; the badge self-corrects. Best-effort by design; a reap never gates a ship.
- If a diagram artifact is absent (a pure-process feature drew nothing), the entry is
  still valid — a synthesized `report.md` alone is a complete entry. A `.mmd` glob that
  matches nothing is a no-op, not an error.
- If the viewer page can't be built (mermaid runtime missing, no diagrams, or a build
  error), fall back to printing the changelog folder path — the synthesized entry + the
  `shipped` state are the durable result; the interactive page is a convenience layered
  on top (rebuildable any time with `/gogo:view`).
