---
name: gogo-done
description: >-
  The "ship it" step after phase ⑤ — when the user declares work done, write a
  high-level entry into the append-only .gogo/changelog/<YYYY-MM-DD>-<name>/ archive:
  a SYNTHESIZED report.md (what was changed/done/implemented, key outcomes, one-line
  decisions, review/test verdict — written, never a copy of the full report bundle;
  the audit trail stays in .gogo/work/, linked), the slug-prefixed diagram .mmd set, a
  manifest.json carrying a members[] array, and the before/ set. One OR several related
  work items can ship as ONE merged release entry. It builds the interactive viewer
  page for the entry (reusing the gogo-view build) and prints its file:// link, and
  sets each member's state.md to a terminal `shipped` status. With NO slug it classifies
  every .gogo/work/feature-* (shared gogo-status classifier), prints the four-class
  status table for context, and offers the ready-to-ship items as a filterable
  AskUserQuestion multi-select: selecting MULTIPLE items merges them into ONE changelog
  entry (release name suggested + confirmed), one pick is one entry — multi-select IS the
  merge signal, there is no extra merge-or-split question. A non-slug arg (or, when there
  are more ready items than fit one question, an answer) is a case-insensitive substring
  filter over slug+title. Use when the user runs /gogo:done or says work is shipped /
  finished / released. Synthesis-not-copy, idempotent, writes only under .gogo/, offline.
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
  release entry (the `+` is the merge signal; skips the list).
- **`/gogo:done`** (no slug, or a non-resolving filter arg) — open the **ready-to-ship
  list**: classify every `.gogo/work/feature-*` (shared gogo-status classifier), print
  the four-class status table for context, then offer the **ready-to-ship** items as a
  filterable `AskUserQuestion` **multi-select**. **Selecting multiple items merges them
  into ONE** entry (release name suggested + confirmed); one pick is one entry. There is
  **no extra merge-or-split question** — multi-select *is* the merge signal.

Either way the actual shipping is the single **"Write changelog entry (1..N members)"**
flow below — the list only *selects members*; it never archives anything itself. Pure
`Read` / `Write` / `Bash` (+ `Skill` to reuse the `gogo-view` build); only ever writes
under `.gogo/`; offline.

## Inputs (declared) and outputs

| Direction | Artifact | Contract |
|---|---|---|
| in (required, per member) | `.gogo/work/feature-<slug>/report/report.md` | the as-built report — the **synthesis source**, never copied |
| in (optional, per member) | `report/*.mmd`, `report/manifest.json` | the as-built UML set + its index (kinds/titles) |
| in (optional, per member) | `report/before/*.mmd` + `report/before/manifest.json` | the plan-time "before" set (FR8) → viewer compare mode |
| in (list mode) | the shared **work-index** (gogo-status Step A classifier, in-memory) | the four-class record shape the list consumes (`slug`, `title`, `status`, `class`, …) |
| in (assets) | `${CLAUDE_PLUGIN_ROOT}/assets/{mermaid/mermaid.min.js, viewer/*}` | vendored viewer runtime (copied on demand) |
| out | `.gogo/changelog/<YYYY-MM-DD>-<name>/` — **synthesized** `report.md` + slug-prefixed `*.mmd` + `manifest.json` (with a `members[]` array) + `before/` | append-only archive; **no `diagrams.html` copy** |
| out | `.gogo/resources/view/<date>-<name>.html` (interactive viewer page, best-effort) | self-contained offline page |
| out | each member's `state.md` (status → `shipped`) | human state |

## ① validate-in (gate)

- **`<slug>` or `slug1+slug2+...` given** (each part naming a real feature) → confirm
  **each** named feature is **report-complete**:
  `.gogo/work/feature-<slug>/report/report.md` exists. Any missing → **STOP** naming the
  missing feature(s):

  > No report found for `<feature>` — run `/gogo:report <feature>` first, then `/gogo:done`.

  **An arg containing `+` is always an explicit merge list — never a filter.** If ANY
  `+`-part names no real feature, **STOP** naming the unknown part(s) ("unknown feature
  `<part>` — check `/gogo:status`"); do not fall through to list mode. Only a **bare,
  `+`-free** arg that resolves to no feature is treated as a text **filter** for list
  mode (② Resolve mode).

- **No slug (list mode)** → no hard prerequisite: classify whatever exists. If there are
  **zero work items at all** (`.gogo/work/feature-*`), say so plainly ("no features yet —
  run `/gogo:plan` first") and stop. If there are items but **none ready-to-ship**, say so
  ("nothing report-complete to ship yet — run `/gogo:report <feature>` first, then
  `/gogo:done`") and stop — the list only ships ready-to-ship items.

`/gogo:report` works even on a past/broken run (it writes a best-effort report), so the
guidance is always actionable. Never write an entry for a feature that hasn't been reported.

## ② Resolve mode

From `$ARGUMENTS`:
- a single **slug** that names a real `.gogo/work/feature-<slug>/` → run **Write
  changelog entry** with that one member (a single entry);
- a **`+`-joined list** (`slug1+slug2+...`) → **merge**: every part must name a real
  feature (any unknown part → **STOP** per validate-in — a `+` arg is never a filter);
  derive the release name + newest member date, then run **Write changelog entry** with
  those members as ONE entry. The `+` is the merge signal, so list mode is skipped;
- a **non-empty, `+`-free arg that does NOT resolve** to a feature slug → **List mode**
  (below) with that arg as the case-insensitive substring **filter** (FR2);
- **empty** → **List mode** (below) with no preset filter — which selects the
  ready-to-ship members, then runs **Write changelog entry**.

## Write changelog entry (1..N members) — the single entry-writer

This is the one place shipping happens. `<slug>`, `slug1+slug2`, and the board all call
it (with 1, N pre-answered, or the selected members). It is idempotent and `.gogo/`-only.
A single member (`members = [<slug>]`) and a merged set share **one shape** — the only
difference is 1 vs N members; there is no divergent single-vs-merged code path.

1. **Resolve + validate the members.** For each member slug, require
   `.gogo/work/feature-<slug>/report/report.md`. Skip (with a one-line note) any slug
   that isn't report-complete — never write an entry for a feature without a report. If
   nothing report-complete remains, stop.

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
   mkdir -p "$dst"
   rm -f "$dst"/*.mmd; rm -rf "$dst/before"        # keep the dated dir; refresh the diagram set
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

6. **Build the interactive viewer page for the entry (FR10, best-effort).** Reuse the
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

## List mode (no slug) — the filterable ready-to-ship list

No slug (or a non-resolving filter arg) picks the members to ship from a plain
in-terminal list. It **selects members** for the entry-writer; it never archives or
mutates gogo state itself. Picking **multiple** items *is* the request to merge them
into ONE entry — there is **no extra merge-or-split question**.

1. **Build the work-index.** Run the shared **gogo-status Step A classifier**
   (`skills/gogo-status/SKILL.md`) to label every `.gogo/work/feature-*` as
   **shipped · ready-to-ship · in-progress · unfinished**, newest-first, in its
   documented record shape (`slug`, `title`, `status`, `class`, …). This is the same
   read-only classifier `/gogo:status` renders — reuse it, don't re-derive.
2. **Print the four-class status table (context).** Render the work-index as a table
   grouped by class (shipped · ready-to-ship · in-progress · unfinished) so the user sees
   the full picture before choosing. Only **ready-to-ship** items are shippable; the other
   classes are shown for context. Mention that `/gogo:view <slug>` opens any card's page
   and `/gogo:go <slug>` runs/resumes the pipeline.
3. **Filter (FR2 — case-insensitive substring over `slug` + `title`).** Narrow the
   **ready-to-ship** list before offering it:
   - a **non-resolving arg** was passed (② Resolve mode) → use it as the filter;
   - else there are **more than 4** ready-to-ship items (more than fit one
     `AskUserQuestion`) → ask a text filter first ("filter ready-to-ship items — a
     substring of the slug or title");
   - else → no filter.
   Match it case-insensitively as a substring of each item's `slug` + `title`. **Loop
   until the list fits:** matches **nothing** → say so and re-ask (or fall back to the
   full ready-to-ship list); still **more than 4** → state the count and re-ask for a
   narrower term (offering the 4 newest matches as the escape hatch); **≤4** → Step 4.
4. **Select + ship.** Offer the (filtered) **ready-to-ship** items via one
   `AskUserQuestion` **multi-select** ("which features to ship?"). Non-ready items are not
   selectable — they appear only in the context table (Step 2). Then hand the picks
   straight to the **"Write changelog entry (1..N members)"** writer:
   - **0 picks** → nothing to do; say so and stop.
   - **1 pick** → **Write changelog entry** with that one member (a single entry).
   - **≥2 picks** → **merge**: **Write changelog entry** **once** with all picks as
     `members[]` (derive + confirm the release name, per the writer). **Selecting multiple
     IS the merge signal — no extra merge-or-split question is asked.**
   Skip (with a one-line note) any pick that turns out not to be report-complete.

## ③ Return

- **Single entry** (`<slug>`, or one list pick) — a one-line confirmation: which
  member(s) were synthesized, to which `.gogo/changelog/<date>-<name>/`, and that each
  member's `state.md` is now `shipped`.
- **Merged entry** (`slug1+slug2`, or a multi-pick from the list) — one confirmation
  naming the release, its `.gogo/changelog/<date>-<release-name>/`, and the member slugs
  marked `shipped`.

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

- The ready-to-ship **list is always available** — a plain `AskUserQuestion`, with no
  tty, no python3, no external tool required. The only "degradation" is having **no work
  to offer**: **zero features** → "run `/gogo:plan` first"; **zero ready-to-ship** → "run
  `/gogo:report <feature>` first". Neither is a failure — each stops cleanly with
  actionable guidance.
- If a diagram artifact is absent (a pure-process feature drew nothing), the entry is
  still valid — a synthesized `report.md` alone is a complete entry. A `.mmd` glob that
  matches nothing is a no-op, not an error.
- If the viewer page can't be built (mermaid runtime missing, no diagrams, or a build
  error), fall back to printing the changelog folder path — the synthesized entry + the
  `shipped` state are the durable result; the interactive page is a convenience layered
  on top (rebuildable any time with `/gogo:view`).
