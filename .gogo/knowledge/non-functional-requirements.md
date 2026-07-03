# Non-functional requirements

**Purpose:** standing quality bars the pipeline must hold — portability,
safety, reliability, degradation.

<!-- gogo:meta
Mode: owned
Source: [ ../../README.md ]
Confidence: high
Generated-by: /gogo:build
-->

## Portability
- The **core plan→implement→review→test loop needs zero external dependencies.**
- **Mermaid is vendored** and renders offline over `file://` (no `mmdc`, no
  Chromium, no network).
- Anything else (Playwright MCP, `mmdc`, `jq`, ntfy, and — since 0.7.0 — `python3`
  + `tmux` for the `/gogo:done` work board) is **optional** and must be detected at
  use; absence → graceful fallback, never a failure. The interactive terminal TUI
  (`board.py`) degrades to the status table + `AskUserQuestion` multi-select.

## Safety
- **Writes are confined to `.gogo/`** (one user-gated exception — see gogo
  overrides). Never mutate a proxied upstream file.
- Hooks are best-effort and side-effect-light; never block or crash a session.
- Don't auto-edit `.gitignore`; print guidance instead.

## Reliability / determinism
- Phases are **resumable**: `state.md` is the single source of truth for where a
  feature is; keep it current at every transition.
- Build is **idempotent**: re-runs reconcile, preserving user/owned content.
- Because the workers are LLMs, **artifacts that cross a phase boundary should be
  validatable** (clear, checkable shape) so a bad hand-off is caught, not
  propagated. (Drives the pipeline-contracts work.)

## Footprint
- One vendored mermaid runtime per project at `.gogo/resources/mermaid.min.js`
  (shared by all features), not per feature.
- Keep the published plugin lean; no build artifacts committed except the
  intentional vendored `mermaid.min.js` (and authored source like `board.py`).
- **Vendored Python must never ship compiled bytecode** — `__pycache__/` and
  `*.pyc` are gitignored so a vendored tool (e.g. `assets/kanban/board.py`) never
  drags platform-specific bytecode into the bundle.
- **Changelog entries are high-level syntheses with a slim footprint** (since
  0.8.0): an entry is a *written* summary + slug-prefixed `.mmd` set +
  `manifest.json` (+ `before/`) — never a full-report copy and never a
  `diagrams.html` duplicate. The full audit trail stays in `.gogo/work/` (linked);
  the interactive page is built from source by `/gogo:view`.
- **Second sanctioned vendored mermaid copy (since 0.10.0, REV-012 accepted):**
  `cli/internal/pages/assets/mermaid.min.js` duplicates
  `assets/mermaid/mermaid.min.js` (~3.3 MB) because `go:embed` requires the file
  inside the module — the price of a standalone `go install`-able binary. Kept
  byte-identical via `make sync-assets` (the `assets/` copy is the source of
  truth). Exactly these two copies; never a third.

## Performance (since 0.10.0 — the CLI bar)
- **The read path is deterministic and LLM-free.** Managing/viewing existing work
  (board, status, view, events) must start in **milliseconds** — the `gogo` CLI
  parses the contract files directly; an LLM in a read-only path is a regression.
- The LLM appears only where it adds value — pipeline execution and changelog
  synthesis — and is *launched* by the cockpit (`claude` in tmux), never awaited
  inline for reads.

## gogo overrides
<!-- Preserved across re-runs. -->

### Knowledge determinism budget
- Knowledge files are **always-read context**; oversized always-read context makes
  the LLM pipeline workers wander and err. Hold each `.gogo/knowledge/*.md` body to
  OK `<200` · WARN `200-400` · OVER `>400` lines (measure the gogo-owned body
  only). Extract over-budget situational detail into **on-demand skills** with
  `/gogo:skills` so it loads only when relevant — that is the determinism win.
- **Safety exception (user-gated).** Writes stay confined to `.gogo/`; the single
  sanctioned write outside it is an **approved standalone** skill's
  `.claude/skills/<slug>/` dir — per-candidate, never automatic.
