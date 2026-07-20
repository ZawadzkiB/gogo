# Decisions — feature `unified-board`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block. This is the
audit trail that lets the pipeline pause and resume safely.

## D1 — `Feature.Project`: first-class field vs derive at render
- **Phase:** plan
- **Question:** how does a merged feature know which project it belongs to?
- **Options:**
  - A. Add a first-class `Feature.Project string`, stamped once by `LoadWorkspace`.
  - B. Derive it at render time by matching `f.Root` against `projects.AllSources`.
- **gogo recommends:** A — a one-time stamp vs an O(features×sources) render-path
  lookup; the field is exactly what tags + the project filter want.
- **Status:** RESOLVED (see Resolutions)

## D2 — `gogo global` default: unified, no flag
- **Phase:** plan
- **Question:** is the unified board THE behavior, or opt-in behind a flag?
- **Options:**
  - A. Unified is THE behavior; the single-project board is retired. A project chip
    (`all → one`) reproduces the old single-project view on demand — no flag needed.
  - B. Keep single-project default; add a `--unified`/`--all` flag to opt in.
- **gogo recommends:** A — one board matches the design; two modes for one design is
  needless surface.
- **Status:** RESOLVED (see Resolutions)

## D3 — Source filtering in the unified view
- **Phase:** plan
- **Question:** the board `p` currently cycles the SOURCE chip within one project. In
  the unified view `p` must cycle PROJECT (design 3a). What happens to source filtering?
- **Options:**
  - A. Project chips are the primary `p`-cycled row; source stays as the per-card dot
    + the free-text `@` token (secondary), ANDing with the project chip. Retire the
    interactive source-chip row (source labels collide across projects; two chip rows
    is noise). Also extend `@name` to match project OR source (today it matches only
    `f.Source`, though the code calls it a "project" token — a drift this fixes).
  - B. Keep BOTH chip rows: project primary, the focused project's source chips as a
    secondary row.
  - C. Drop source filtering entirely (project chip + free text only).
- **gogo recommends:** A — cleanest; keeps source narrowing via `@`, matches the design's
  single chip row, fixes the `@`-token naming drift.
- **Status:** RESOLVED (see Resolutions)

## D4 — Plans/config focus when the board is "all"
- **Phase:** plan
- **Question:** the board aggregates all projects, but plans + config are project-scoped.
  Which project do they act on?
- **Options:**
  - A. The focused project = the project-chip selection; `all` defaults the focus to
    `allProjects[0]`; the board chip and the config switcher share ONE `m.project`.
  - B. Plans tab shows every project's plans grouped by project; config stays focused.
- **gogo recommends:** A — coherent (the project you pick is the one you configure),
  closest to today's `switchProject`; B is more surface, deferred.
- **Status:** RESOLVED (see Resolutions)

## D5 — Card two-dot tag layout
- **Phase:** plan
- **Question:** how does a card carry both project + source?
- **Options:**
  - A. Name-row right tag = `●project ●source <source-label>` (both dots carry color;
    the label names the source; the project is named by the chip-row legend + filter).
  - B. Two separate tags: `●P project` + `●S source` (wider, eats the slug budget).
  - C. Dots only `●P ●S`, drop the source label (most compact; loses the source name).
- **gogo recommends:** A — both signals, one compact tag, reuses `originDots` +
  `fitSourceTag` truncation.
- **Status:** RESOLVED (see Resolutions)

## Resolutions (accepted by user 2026-07-20)
- **D1 = A** — first-class `Feature.Project` field, stamped by `LoadWorkspace`.
- **D2 = A** — `gogo global` is unified with NO flag; the single-project view is the `all → one` project chip.
- **D3 = A** — PROJECT chips are the sole `p`-cycled row; the interactive source-chip row is RETIRED; source narrowing survives via the per-card source dot + the free-text `@name` token (extended to match project OR source).
- **D4 = A** — plans/config act on the FOCUSED project (the project-chip selection; `all` defaults to `allProjects[0]`); the board chip + config switcher share one `m.project`.
- **D5 = B (user overrode rec A)** — the card name-row tag shows BOTH names: `●<project-name> ●<source-name>` (e.g. `●gogo ●gogo-cli`) with both color dots. Wider than rec A, so the slug budget shrinks — reuse `fitSourceTag`/truncate so the composed name row NEVER wraps (REV-006 discipline), and truncate the names themselves at very narrow widths.
