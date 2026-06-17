---
name: gogo-mermaid
description: >-
  Author portable Mermaid diagrams for the gogo pipeline (plan/flow/state/sequence
  charts) with zero external dependencies, and build a self-contained offline
  viewer. Use when a gogo phase needs to create or update a feature's diagrams
  (the plan phase draws the change/flow; the report phase re-renders it), or when
  the user asks for a diagram inside a gogo feature folder.
---

# gogo-mermaid — portable diagrams, no CLI required

This skill produces Mermaid diagrams that render **anywhere**, with **no global
`mmdc`, no Chromium, no network**. Mermaid is vendored inside this plugin
(`${CLAUDE_PLUGIN_ROOT}/assets/mermaid/mermaid.min.js`, a UMD build that works
over `file://`).

## What "a diagram" means in gogo — three artifacts

For each diagram, produce all three so it renders in every context:

1. **A fenced ` ```mermaid ` block** inside the relevant markdown (e.g. `plan.md`).
   This renders natively in GitHub, VS Code, and JetBrains previews — zero deps.
2. **A standalone `.mmd` file** in the feature's `charts/` folder
   (`.gogo/plans/feature-<slug>/charts/<name>.mmd`) holding the same source.
3. **The offline viewer** `.gogo/plans/feature-<slug>/charts/diagrams.html` — a
   self-contained page that renders every `.mmd` in the folder. Open it in any
   browser; it needs only the vendored `mermaid.min.js`.

## Generating / refreshing `charts/diagrams.html`

1. **Ensure the shared runtime exists** (one copy per project, not per feature):
   if `.gogo/plans/.assets/mermaid.min.js` is missing, copy it from
   `${CLAUDE_PLUGIN_ROOT}/assets/mermaid/mermaid.min.js`.
   ```bash
   mkdir -p .gogo/plans/.assets
   [ -f .gogo/plans/.assets/mermaid.min.js ] || cp "${CLAUDE_PLUGIN_ROOT}/assets/mermaid/mermaid.min.js" .gogo/plans/.assets/mermaid.min.js
   ```
2. **Start from the template** `${CLAUDE_PLUGIN_ROOT}/assets/mermaid/viewer.template.html`
   and replace the three tokens:
   - `GOGO_FEATURE_SLUG` → the feature slug.
   - `GOGO_MERMAID_SRC` → the **relative** path from `charts/` to the shared
     runtime: `../../.assets/mermaid.min.js`.
   - `<!-- GOGO:DIAGRAMS -->` → one block per `.mmd`, in this exact shape (inline
     the source — do **not** `fetch()` it; `file://` forbids it):
     ```html
     <h2>Plan flow</h2>
     <div class="diagram"><pre class="mermaid">
     flowchart TD
       A[user goal] --> B[plan]
     </pre></div>
     ```
3. Write the result to `.gogo/plans/feature-<slug>/charts/diagrams.html`.

> Why a shared `.gogo/plans/.assets/` copy: it keeps the runtime out of every feature
> folder (one ~3 MB file per project), the path is relative (so the repo stays
> portable if moved or shared), and it works fully offline.

## Optional SVG/PNG export (graceful, never required)

Only if a renderer is already present — never install one:
```bash
if command -v mmdc >/dev/null 2>&1; then
  mmdc -i charts/plan.mmd -o charts/plan.svg -t default -b transparent || true
fi
```
If `mmdc` is absent, **skip silently** — the `.mmd` source + the offline viewer
are the durable artifacts. Note the skip in the report rather than erroring.

## Diagram conventions

- **Pipeline / change flow** → `flowchart TD` (or `LR`).
- **Lifecycle / status** → `stateDiagram-v2`.
- **Interactions** → `sequenceDiagram`.
- Keep node labels short; quote labels with punctuation. Prefer one focused
  diagram per concern over one giant chart.

## Portability contract

- Never depend on a globally-installed mermaid skill or CLI.
- The fenced block is the minimum viable output; the `.mmd` + viewer are
  enhancements. If anything optional fails, the markdown still renders.
