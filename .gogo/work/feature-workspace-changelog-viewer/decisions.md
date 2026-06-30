# Decisions — feature `workspace-changelog-viewer`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block.

## D1 — Staging
- **Question:** Ship this large change in stages or one big bang?
- **Options:** A. One feature, **three stages** (mechanical rename/resources →
  report/changelog/done → viewer), each its own implement→review→test loop.
  B. One monolithic pass.
- **gogo recommends:** A — the rename is mechanical and low-risk and should land
  first; the viewer is the biggest unknown and lands last.
- **Status:** RESOLVED

### RESOLVED (user, 2026-06-30)
Accepted as recommended.

## D2 — Existing-project migration
- **Question:** How do projects with an existing `.gogo/plans/` (+ `.assets/`) move
  to the new layout?
- **Options:** A. `/gogo:build` **auto-migrates** (move + log; idempotent).
  B. Documented manual `mv`.
- **gogo recommends:** A — it stays within the `.gogo/`-only rule, is idempotent,
  and spares every existing project a manual step.
- **Status:** RESOLVED

### RESOLVED (user, 2026-06-30)
Accepted as recommended.

## D3 — Viewer interactivity scope
- **Question:** How interactive must `/gogo:view` be?
- **Options:** A. canvas **pan + zoom + drag + reset/fit** over the mermaid-rendered
  SVG now; per-node repositioning a later stretch. B. full **per-node drag + edge
  re-routing** now (much larger over mermaid SVG).
- **gogo recommends:** A — delivers the "move them around / not ugly" goal with a
  realistic, offline, zero-dep vanilla-JS layer; per-node editing is a separate lift.
- **Status:** RESOLVED

### RESOLVED (user, 2026-06-30)
Accepted as recommended.

## D4 — Use-case diagrams
- **Question:** mermaid has no native use-case diagram type — support it how?
- **Options:** A. Render use-case as a **flowchart actor↔use-case graph** and add
  `use-case` to the `charts-manifest` kind enum. B. Drop use-case; keep
  {flow,sequence,class,activity}.
- **gogo recommends:** A — the goal explicitly asks for use-case "when relevant";
  a flowchart approximation is the standard mermaid workaround.
- **Status:** RESOLVED

### RESOLVED (user, 2026-06-30)
Accepted as recommended.

## D5 — Changelog entry naming
- **Question:** How are `.gogo/changelog/` entries named?
- **Options:** A. `<YYYY-MM-DD>-<slug>/` (dated, chronologically ordered).
  B. `<slug>/` (plain).
- **gogo recommends:** A — a changelog is chronological; dated dirs sort + dedupe
  re-ships naturally.
- **Status:** RESOLVED

### RESOLVED (user, 2026-06-30)
Accepted as recommended.

## D6 — Report layout move
- **Question:** Where does `report.md` live?
- **Options:** A. Consolidate under `report/` (`report/report.md` + diagrams +
  `result.json`). B. Leave `report.md` at the feature-folder root.
- **gogo recommends:** A — groups the report bundle (md + UML + result) that
  `/gogo:done` copies and `/gogo:view` reads as one unit.
- **Status:** RESOLVED

### RESOLVED (user, 2026-06-30)
Accepted as recommended.

## D7 — Viewer summary rendering
- **Question:** How is the report.md summary turned into the readable webpage?
- **Options:** A. The `gogo-view` skill **pre-renders** md→HTML (no extra dep;
  mermaid stays client-side). B. Vendor a JS markdown library to render at runtime.
- **gogo recommends:** A — keeps the offline / zero-dep / no-build bar.
- **Status:** RESOLVED

### RESOLVED (user, 2026-06-30)
Accepted as recommended.
