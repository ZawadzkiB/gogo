# Decisions — feature `docs-and-verified-discovery`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block.

## D1 — Docs site tooling
- **Phase:** plan
- **Question:** How is the hosted docs site built/served?
- **Options:**
  - A. GitHub Pages from `/docs`, Jekyll + `remote_theme: just-the-docs` + mermaid
    — GitHub builds it; **nothing committed, no local build, no CI**; nav + search.
  - B. A single self-contained HTML page (like the offline diagram viewer).
  - C. A static-site generator (MkDocs Material / Docusaurus / VitePress) — prettier
    but adds a Node/Python build + deps + CI.
- **gogo recommends:** A — matches gogo's no-build / zero-local-dep value; gives
  nav + search + mermaid with nothing to maintain locally.
- **Status:** RESOLVED

### RESOLVED (user, 2026-06-30)
Accepted as recommended.

## D2 — Scope / staging
- **Phase:** plan
- **Question:** One feature (docs + verified discovery) or two?
- **Options:**
  - A. One feature, two independently-reviewable parts (A docs, B discovery).
  - B. Two separate features (two plan/accept cycles).
- **gogo recommends:** A — they share the "documentation accuracy" theme; ship A
  and B as they land.
- **Status:** RESOLVED

### RESOLVED (user, 2026-06-30)
Accepted as recommended.

## D3 — Verification depth (Part B)
- **Phase:** plan
- **Question:** How much of the wired knowledge does build verify against code?
- **Options:**
  - A. High-signal, mechanically-checkable claims (stack, build/run/test commands,
    test framework, entry points, scripts); mark the rest `unverifiable`.
  - B. Exhaustive — every distilled line.
- **gogo recommends:** A — pure Glob/Grep/Read can't check everything; verify what
  is checkable, flag the rest.
- **Status:** RESOLVED

### RESOLVED (user, 2026-06-30)
Accepted as recommended.

## D4 — Enabling GitHub Pages
- **Phase:** plan
- **Question:** Who turns on Pages (Settings → Pages → deploy from `main` `/docs`)?
- **Options:**
  - A. gogo runs the `gh api` call after merge; you confirm the published URL.
  - B. You toggle it in the repo settings UI.
- **gogo recommends:** A — one `gh` call, then verify the site is live.
- **Status:** RESOLVED

### RESOLVED (user, 2026-06-30)
Accepted as recommended.

**Layout reconciliation (orchestrator, 2026-06-30):** implement first put
`_config.yml` at the repo root (which would force deploy-from-`/(root)` + an
`exclude:` list). Reconciled to the D4-stated **deploy-from-`main`-`/docs`**:
`_config.yml` moved to `docs/_config.yml`, `exclude:` list dropped (Jekyll only
sees `docs/`). This also makes the README URL `https://zawadzkib.github.io/gogo/`
resolve to the docs index. The `gh` enable step must select source = branch
`main`, folder **`/docs`**.

## D5 — Sync the stale agent role files now, or defer? (from review REV-003)
- **Phase:** review
- **Question:** `docs/agents.md` cites `agents/*.md` as authoritative, but
  `agents/gogo-developer.md` + `agents/gogo-reviewer.md` still describe the
  pre-0.2.0 `review-NN.md` snapshot flow rather than the living `issues.json`
  contract. The new docs are correct to the live contract; the agent files lag.
  Fix the two agent files **in this feature**, or **track as a follow-up**
  (they're outside this feature's stated file scope)?
- **Options:**
  - A. **Fix now** — update `agents/gogo-developer.md` + `agents/gogo-reviewer.md`
    to the `issues.json` contract (developer reads/writes `*/issues.json` via
    `--issues`; reviewer produces the living `review/issues.json` + the `review-NN.md`
    snapshot). Small scope creep; removes a real contradiction; on-theme for a
    docs-accuracy feature. Adds a re-review of those two files.
  - B. **Defer** — leave the agent files as-is; open a follow-up. Keeps this feature
    strictly scoped; the contradiction persists until then.
- **gogo recommends:** A — the feature is *about* doc accuracy and "code/contracts
  are the source of truth"; shipping docs that cite contradictory agent files
  undercuts that. The fix is two small files.
- **Status:** RESOLVED

### RESOLVED (user, 2026-06-30)
Option A — fix now. Sync `agents/gogo-developer.md` + `agents/gogo-reviewer.md` to
the `issues.json` contract in this feature, then re-review those two files → test.
