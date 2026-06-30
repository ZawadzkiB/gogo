---
name: gogo-done
description: >-
  The "ship it" step after phase ⑤ — when the user declares a feature done, copy
  its report bundle (report/report.md + the UML .mmd set + diagrams.html) into the
  append-only .gogo/changelog/<YYYY-MM-DD>-<slug>/ archive and set the feature's
  state.md to a terminal `shipped` status. Use when the user runs /gogo:done or
  says a feature is shipped / finished / released. Copy-not-move, idempotent,
  writes only under .gogo/.
---

# gogo-done — promote a report-complete feature to the changelog

The explicit post-report gate. `/gogo:report` (⑤) finalizes the report bundle in
the **work** folder; `/gogo:done` is the user saying *"this is shipped"* — it
**copies** that bundle into the chronological `.gogo/changelog/` archive and marks
the feature terminal. Pure `Read` / `Write` / `Bash`; only ever writes under `.gogo/`.

## Inputs (declared) and outputs

| Direction | Artifact | Contract |
|---|---|---|
| in (required) | `.gogo/work/feature-<slug>/report/report.md` | the as-built report bundle |
| in (optional) | `report/*.mmd`, `report/diagrams.html`, `report/manifest.json` | the as-built UML set + viewer |
| out | `.gogo/changelog/<YYYY-MM-DD>-<slug>/` (copy of the bundle) | append-only archive |
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
   `report/report.md`, every `report/*.mmd`, `report/diagrams.html`, and
   `report/manifest.json` if present. The work folder stays the working source.
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
   ```
   **Idempotent:** re-running for the same `<date>-<slug>` overwrites that same
   dated dir (a refreshed report re-ships cleanly); it never creates duplicates and
   never deletes anything outside the target dir.
4. **Mark the feature terminal.** Set `state.md`: `status: shipped`, `resume: none`
   (leave `phase: done`). Note the changelog path in the resume/summary line.

## ③ Return

A one-line confirmation: which bundle was archived, to which
`.gogo/changelog/<date>-<slug>/`, and that `state.md` is now `shipped`. Point the
user at the archived `diagrams.html` (and, once it lands, `/gogo:view`).

## Degradation

If a diagram artifact is absent (a pure-process feature drew nothing), copy what
exists — `report.md` alone is a valid entry. If `cp` of the glob fails because
there are no `.mmd` files, that's a no-op, not an error.
