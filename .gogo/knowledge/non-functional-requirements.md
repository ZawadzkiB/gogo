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
- **The renderer is vendored** - `very-nice-mermaid`'s browser build renders
  offline over `file://` (no `mmdc`, no Chromium, no network).
- Anything else (Playwright MCP, `very-nice-mermaid`, `jq`, ntfy, and - since 0.7.0 - `python3`
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
- One vendored renderer per project at `.gogo/resources/vnm-browser.js`
  (~450 KB, shared by all features), not per feature.
- Keep the published plugin lean; no build artifacts committed except the
  intentional vendored `vnm-browser.js` (and authored source like `board.py`).
- **Vendored Python must never ship compiled bytecode** — `__pycache__/` and
  `*.pyc` are gitignored so a vendored tool (e.g. `assets/kanban/board.py`) never
  drags platform-specific bytecode into the bundle.
- **Changelog entries are high-level syntheses with a slim footprint** (since
  0.8.0): an entry is a *written* summary + slug-prefixed `.mmd` set +
  `manifest.json` (+ `before/`) — never a full-report copy and never a
  `diagrams.html` duplicate. The full audit trail stays in `.gogo/work/` (linked);
  the interactive page is built from source by `/gogo:view`.
- **Second sanctioned vendored renderer copy (since 0.10.0, REV-012 accepted):**
  `cli/internal/pages/assets/vnm-browser.js` duplicates `assets/vnm/vnm-browser.js`
  (~450 KB) because `go:embed` requires the file inside the module - the price of
  a standalone `go install`-able binary. Kept byte-identical via
  `make sync-assets` (the `assets/` copy is the source of truth). Exactly these
  two copies; never a third.

## Diagnosability (since 0.28.0 - the "why did nothing happen" bar)
A failure the user can see but not explain is a bug, even when the code is correct.
The 0.28.0 incident: a launch failed with `exit status 1` for weeks because tmux's
`command too long` was written to a stderr the code never captured.
- **Never discard a child process's stderr on a user-visible path.** Capture it into a
  bounded buffer (so a huge stderr cannot grow unbounded, and a capture failure can never
  break a call that would otherwise have worked) and carry it in a **typed** error that
  names the subcommand and wraps the original (`Unwrap()` must still reach the
  `*exec.ExitError`). A wrapped exit code alone is not a diagnosis.
- **A limit must name its number.** "Too big" is not actionable; "the command line is
  20128 bytes, the limit is 16317" is. Prefer refusing **before** invoking the tool, so
  the message is the real reason rather than the tool's generic failure.
- **Distinguish blocked from failed from done.** A refusal must carry the *unblock*; a
  failure must carry the tool's own words. And the distinction must survive a
  **colourless** terminal - colour alone is flattened by a no-colour TTY and by
  TTY-less `go test`, so pair it with a glyph or a word, which is also what makes it
  **assertable in `View()`**.

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

## Custom
<!-- Yours. gogo never rewrites this section: `/gogo:build` re-runs and the report-phase
     reconcile copy it 1:1 (byte-for-byte), exactly like `## gogo overrides`. Put any
     project notes gogo should read but never touch here — safe to edit or delete. -->
