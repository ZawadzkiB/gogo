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
  for the `/gogo:xplan` browser board) is **optional** and must be detected at use;
  absence → graceful fallback, never a failure. The browser board degrades to
  `/gogo:done`'s filterable ready-to-ship list (`python3` absent → no board, no failure).

## Safety
- **Writes are confined to `.gogo/`** (one user-gated exception — see gogo
  overrides). Never mutate a proxied upstream file.
- Hooks are best-effort and side-effect-light; never block or crash a session.
- Don't auto-edit `.gitignore`; print guidance instead.
- **Vendored localhost servers (since 0.10.0):** any local HTTP server the plugin
  ships (e.g. `assets/xplan-board/server.py`) must bind **127.0.0.1 only**, enforce
  a **Host allowlist** (127.0.0.1/localhost — DNS-rebind guard) on every request and
  an **Origin same-or-absent check on mutating routes** (CSRF guard) — the D5 bar;
  serve files **path-traversal-safe** (normalize + containment, encoded variants
  included); guard mutations semantically (e.g. only ready-to-ship slugs shippable);
  and write hand-off files **atomically and race-free** (unique tmp +
  `O_CREAT|O_EXCL` + rename).

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
- Keep the published plugin lean; no build artifacts committed except the intentional
  vendored `mermaid.min.js`, authored vendored source like `assets/xplan-board/server.py`,
  and the **committed React `dist/`** the `/gogo:xplan` board serves (its source lives
  beside it; npm is dev-time only, D4=A).
- **Vendored Python must never ship compiled bytecode** — `__pycache__/` and
  `*.pyc` are gitignored so a vendored tool (e.g. `assets/xplan-board/server.py`) never
  drags platform-specific bytecode into the bundle.
- **Changelog entries are high-level syntheses with a slim footprint** (since
  0.8.0): an entry is a *written* summary + slug-prefixed `.mmd` set +
  `manifest.json` (+ `before/`) — never a full-report copy and never a
  `diagrams.html` duplicate. The full audit trail stays in `.gogo/work/` (linked);
  the interactive page is built from source by `/gogo:view`.

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
