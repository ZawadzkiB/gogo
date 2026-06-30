# Decisions — feature `skill-extraction`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block.

## D1 — Loading model: per-candidate kind, not one global location
- **Phase:** plan
- **Question:** How is an extracted skill loaded, and where does it live?
  (Clarified with the user 2026-06-29 — see `adjustments.md`.)
- **Background:** the harness only auto-discovers skills under plugin `skills/`,
  `~/.claude/skills/`, and project `.claude/skills/`. A file under `.gogo/skills/`
  is **not** auto-discovered or invokable by name; the gogo pipeline loads it by
  reading the parent knowledge file's pointer. The determinism win (shrinking the
  always-read knowledge file) happens regardless of where the extract lands.
- **Resolution shape (recommended):** classify **each candidate** per-skill:
  - **`knowledge`** → `.gogo/skills/<slug>/` — project/pipeline-scoped detail;
    loaded by the pipeline via the parent pointer; honors `.gogo/`-only.
  - **`standalone`** → `.claude/skills/<slug>/` — a self-contained, reusable
    capability; harness auto-discovers + invokable by name; session-global.
  The command recommends a kind with a rationale; the user confirms/overrides per
  candidate at the gate. `.claude/skills/` is written **only** for an approved
  standalone (the single sanctioned write outside `.gogo/`, never automatic).
- **gogo recommends:** the per-candidate model above (supersedes the earlier
  "single global location" framing).
- **Status:** RESOLVED

### RESOLVED (user, 2026-06-29)
Accepted the per-candidate kind model: `knowledge` → `.gogo/skills/` (pipeline
loads via pointer), `standalone` → `.claude/skills/` (harness auto-discovers),
classified + recommended per candidate, `.claude/skills/` written only on explicit
per-candidate approval.

## D2 — Extraction granularity
- **Phase:** plan
- **Question:** At what heading level is a "section" extracted?
- **Options:**
  - A. H2 by default; drop to H3 only when a single H2 is itself oversized.
  - B. Always H3 (finer-grained, more/smaller skills).
- **gogo recommends:** A — H2 sections are the natural cohesive unit; H3 when an
  H2 is too big on its own.
- **Status:** RESOLVED

### RESOLVED (user, 2026-06-29)
Accepted as recommended.

## D3 — Thresholds
- **Phase:** plan
- **Question:** What line budget triggers warn vs hard-limit?
- **Options:**
  - A. 200 warn / 400 hard (per the request), overridable via `--warn`/`--max`.
  - B. Different defaults.
- **gogo recommends:** A — as specified in the request.
- **Status:** RESOLVED

### RESOLVED (user, 2026-06-29)
Accepted as recommended.

## D4 — Proposal artifact
- **Phase:** plan
- **Question:** How durable/typed should the extraction proposal be?
- **Options:**
  - A. Presented prose + a durable `.gogo/skills/index.md` registry (simple, v1).
  - B. A typed `skill-proposal` JSON contract (like `issues-list`) for a
    validatable propose→extract hand-off.
- **gogo recommends:** A — keep v1 human-gated and simple; add a typed contract
  only if `/gogo:go` ever needs to chain extraction.
- **Status:** RESOLVED

### RESOLVED (user, 2026-06-29)
Accepted as recommended.
