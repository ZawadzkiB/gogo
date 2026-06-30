---
description: Build and open a self-contained, offline interactive webpage for a gogo report — the report.md summary as readable HTML plus its mermaid diagrams in a pan/zoom/drag canvas (vendored runtime, no network, no build).
argument-hint: "[changelog-entry | feature-slug]"
allowed-tools: Read, Write, Bash, Glob, Grep, Skill, TodoWrite, AskUserQuestion
model: opus
---

View a gogo report as an interactive webpage, via the `gogo-view` skill.

Target: $ARGUMENTS  (a `.gogo/changelog/<date>-<slug>` entry, a feature slug, or a
path. If absent, list the available reports and pick — newest changelog entry by
default, else the newest `.gogo/work/feature-*/report/`.)

Load `gogo-view` and follow it:

1. **Enumerate + pick** — list every report (`.gogo/changelog/*/` +
   `.gogo/work/feature-*/report/`, plus older root `report.md`s) that has a
   `report.md`; resolve `$ARGUMENTS` directly, else ask which (AskUserQuestion).
2. **Ensure resources** — copy the vendored `mermaid.min.js` (if missing) and the
   viewer `interactive.js` + `viewer.css` into `.gogo/resources/` (idempotent).
3. **Build the page** — pre-render the chosen `report.md` to readable HTML and
   inline each `*.mmd` into `<pre class="mermaid">` blocks, from the vendored
   template; write `.gogo/resources/view/<date-or-slug>.html` with relative paths
   (`../mermaid.min.js`, `../viewer/...`). Offline, self-contained, no network.
4. **Open it** — `open`/`xdg-open` best-effort; on failure print the absolute
   `file://` path so the user can open it manually.
