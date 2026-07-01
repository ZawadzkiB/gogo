---
name: gogo-done
description: >-
  The "ship it" step after phase ⑤ — when the user declares a feature done, copy
  its report bundle (report/report.md + the UML .mmd set + the before/ set +
  diagrams.html) into the append-only .gogo/changelog/<YYYY-MM-DD>-<slug>/ archive,
  build the interactive viewer page for that entry (reusing the gogo-view build) and
  print its file:// link, and set the feature's state.md to a terminal `shipped`
  status. Use when the user runs /gogo:done or says a feature is shipped / finished
  / released. Copy-not-move, idempotent, writes only under .gogo/, offline.
---

# gogo-done — promote a report-complete feature to the changelog

The explicit post-report gate. `/gogo:report` (⑤) finalizes the report bundle in
the **work** folder; `/gogo:done` is the user saying *"this is shipped"* — it
**copies** that bundle into the chronological `.gogo/changelog/` archive, **builds +
prints the interactive viewer link** for that entry, and marks the feature terminal.
Pure `Read` / `Write` / `Bash` (+ `Skill` to reuse the `gogo-view` build); only ever
writes under `.gogo/`; offline throughout.

## Inputs (declared) and outputs

| Direction | Artifact | Contract |
|---|---|---|
| in (required) | `.gogo/work/feature-<slug>/report/report.md` | the as-built report bundle |
| in (optional) | `report/*.mmd`, `report/diagrams.html`, `report/manifest.json` | the as-built UML set + viewer |
| in (optional) | `report/before/*.mmd` + `report/before/manifest.json` | the plan-time "before" set (FR8) → viewer compare mode |
| in (assets) | `${CLAUDE_PLUGIN_ROOT}/assets/{mermaid/mermaid.min.js, viewer/*}` | vendored viewer runtime (copied on demand) |
| out | `.gogo/changelog/<YYYY-MM-DD>-<slug>/` (copy of the bundle, incl. `before/`) | append-only archive |
| out | `.gogo/resources/view/<date>-<slug>.html` (interactive viewer page, best-effort) | self-contained offline page |
| out | `state.md` (status → `shipped`) | human state |

## ① validate-in (gate)

Confirm the feature is **report-complete**: `.gogo/work/feature-<slug>/report/report.md`
exists. If it's missing → **STOP** with exactly this guidance (name the feature):

> No report found for `<feature>` — run `/gogo:report <feature>` first, then `/gogo:done`.

`/gogo:report` works even on a past/broken run (it writes a best-effort report),
so this is always actionable. Never archive a feature that hasn't been reported.

## ② Steps

1. **Resolve the slug.** From `$ARGUMENTS`; if absent, pick the feature whose
   `state.md` shows phase=done / status=done (report-complete). If several, ask which.
2. **Derive the date** for the changelog entry — **do not hardcode**:
   - prefer the report's `- **completed:** <YYYY-MM-DD>` field. That value is
     markdown-bolded, so extract the ISO date itself — never a naive
     `sed 's/.*completed://'` (which would capture the trailing `**`):
     ```bash
     date=$(grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}' .gogo/work/feature-${slug}/report/report.md | head -1)
     ```
   - else a date the user supplied;
   - else today's date (`date +%F`).
3. **Copy (never move) the bundle** into `.gogo/changelog/<date>-<slug>/`: the
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
4. **Mark the feature terminal.** Set `state.md`: `status: shipped`, `resume: none`
   (leave `phase: done`). Note the changelog path in the resume/summary line.

5. **Build the interactive viewer page for this entry (FR10, best-effort).** Reuse
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

## ③ Return

A one-line confirmation: which bundle was archived, to which
`.gogo/changelog/<date>-<slug>/`, and that `state.md` is now `shipped`. Then
**print the interactive viewer link** — the absolute `file://` URL to the built
page — plus the archived static `diagrams.html` path as a fallback:
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
mention `/gogo:view` re-opens the entry any time.

## Degradation

If a diagram artifact is absent (a pure-process feature drew nothing), copy what
exists — `report.md` alone is a valid entry. If `cp` of the glob fails because
there are no `.mmd` files, that's a no-op, not an error. If the viewer page can't
be built (mermaid runtime missing, no diagrams, or a build error), fall back to
printing the archived `diagrams.html` / changelog folder path — the archive + the
`shipped` state are the durable result; the link is a convenience layered on top.
