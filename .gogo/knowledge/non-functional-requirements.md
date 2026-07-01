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
