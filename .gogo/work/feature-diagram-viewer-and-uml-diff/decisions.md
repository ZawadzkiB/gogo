# Decisions — feature `diagram-viewer-and-uml-diff`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block.

## D1 — Interactive-renderer approach (load-bearing)
- **Question:** How do we get xplan-style custom-styled, movable diagrams while
  staying offline / zero-dep and keeping `.mmd` authoring?
- **Options:**
  - A. **Hybrid (xplan seeded from mermaid):** mermaid lays out → parse its SVG
    into `{nodes,edges}` → custom render + own the edge layer + per-node drag with
    live re-routing. Flowchart-family rich; graceful fallback for other kinds.
  - B. **Post-process mermaid SVG only:** recolor + translate node groups, keep
    mermaid's baked edges (edges don't follow a dragged node — looks broken).
  - C. **Full upstream structured model:** gogo diagrams become JSON nodes/edges;
    rewrite gogo-mermaid + every phase (biggest change; loses simple `.mmd` authoring).
- **gogo recommends:** A — the research's "sweet spot": ~xplan interaction without
  an auto-layout engine or a pipeline-wide rewrite; keeps portable `.mmd`.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-01)
Accepted as recommended.

## D2 — "Modify" scope
- **Question:** What does "modify" include in v1?
- **Options:** A. reposition + restyle + **persist positions** (label-edit a
  stretch). B. include in-session **label editing** now (write-back to the model +
  layout; no `.mmd` round-trip).
- **gogo recommends:** A — ship move + restyle + persisted layout; add scoped
  label-edit only if cheap (even xplan doesn't edit labels).
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-01)
Option A — move + restyle + persist; label editing deferred/out of scope for v1.

## D3 — Which diagram kinds get the rich renderer
- **Question:** Rich interaction for all kinds, or flowchart-family first?
- **Options:** A. flowchart-family (gogo `flow` + `use-case`) rich;
  `sequence`/`class`/`activity`(stateDiagram) fall back to pan/zoom-canvas.
  B. all kinds now (much larger; each mermaid kind has a different SVG structure).
- **gogo recommends:** A — flowcharts are the common case and cleanly parseable;
  extend to other kinds later.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-01)
Accepted as recommended.

## D4 — Before/after compare mechanism
- **Question:** How rich is the comparison in the report + viewer?
- **Options:** A. **side-by-side** before|after + a prose "what changed".
  B. also a computed **structural node-diff** (added/removed/changed highlighted),
  enabled by the FR1 parser.
- **gogo recommends:** A committed; B a stretch once the FR1 model exists.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-01)
Accepted as recommended.

## D5 — Before-set storage
- **Question:** Where does the plan-time "before" UML live?
- **Options:** A. `charts/before/*.mmd` at plan; report ⑤ copies it into the report
  bundle so the archive/viewer is self-contained. B. a single combined manifest.
- **gogo recommends:** A.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-01)
Accepted as recommended.

## D6 — Layout persistence
- **Question:** Where are dragged node positions saved?
- **Options:** A. sidecar `.gogo/resources/view/<name>.layout.json` (inspectable,
  portable). B. browser `localStorage`.
- **gogo recommends:** A.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-01)
Accepted as recommended.

## D7 — Persistence write-back mechanism (raised by review REV-002)
- **Phase:** review
- **Question:** A `file://` page **can't write files from JS**, so the D6 sidecar
  can only be *read*, not auto-written on drag. How do we actually deliver
  "drag → persists → reopen keeps the arrangement"?
- **Options:**
  - A. **localStorage + export button** — auto-save positions to `localStorage`
    (seamless reopen in the same browser) **and** an "Export layout" button that
    downloads/copies the `<name>.layout.json` so it can be saved/committed/shared
    (the skill keeps seeding from that sidecar). Best of both.
  - B. **localStorage only** — auto-save, no portable sidecar export. Simplest;
    not portable across machines/browsers.
  - C. **Manual sidecar only** — an "Export layout" button; the user saves the
    JSON as the sidecar by hand; no auto-persist. Matches D6 literally but manual.
  - D. **Read-only v1** — seed from a sidecar only; dragging doesn't persist. Defer.
- **gogo recommends:** A — fully delivers FR4's intent within the `file://`
  constraint: auto-persist for a seamless reopen, plus the portable sidecar via
  export. (localStorage keyed by diagram name; export writes the D6 JSON shape.)
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-01)
Option A — localStorage auto-persist (keyed by diagram name) + an "Export layout"
button that downloads the `<name>.layout.json` (D6 shape); the skill keeps seeding
from that sidecar when present.
