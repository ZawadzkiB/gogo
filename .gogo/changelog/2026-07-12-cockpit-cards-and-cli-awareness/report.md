# cockpit cards & CLI-awareness

- **shipped:** 2026-07-12
- **version:** 0.16.0
- **work:** [.gogo/work/feature-cockpit-cards-and-cli-awareness/](../../work/feature-cockpit-cards-and-cli-awareness/) — full audit trail (plan, 3 implement rounds, review-01, test-01, decisions)

Two linked slices on top of the v0.15.0 persistent-session CLI, shipped A→B under one **0.16.0** bump: **(A)** the plugin became **CLI-aware** — a canonical, on-demand `gogo-cli` reference plus a lean pointer, kept honest by a four-source enumeration-sync lint — and **(B)** the board's drill-in became a **rich card** — description / folder / status / the card's sessions / a recent-events tail, with `a` attach and `K` kill. Together they close the two discoverability/depth gaps the v0.15.0 CLI left open: it was powerful but under-discovered (no canonical command reference, so an installed Claude didn't cleanly know the surface or *when* to suggest the CLI), and the drill-in was thin (a file list only). These are the deferred **Slice 3** (drill-in) and the **passive half of Slice 4** (the gogo-cli reference) of the persistent-session program.

**Outcomes**

- **Slice A — CLI-awareness (markdown + one Go lint).** A new `skills/gogo-cli/SKILL.md` is the canonical, on-demand CLI companion reference — its frontmatter `description` *is* the discoverability mechanism (an installed Claude loads the body only when the CLI is relevant), documenting the full v0.15.0+ command surface, the persistent-session model (launch-or-`--resume` one warm `claude -p`; one-owner lock; kill-at-ship / `gogo sweep`), the conditional "separate curl install, not bundled" framing, and when to use the CLI vs the in-chat `/gogo:*` flow. `skills/gogo/SKILL.md` gained a lean `**Load when:**` pointer (no always-read bloat). A new `TestCLICommandEnumerationInSync` derives the command verbs from `main.go`'s dispatch and asserts each appears in `printHelp`, README, `docs/cli-contract.md`, and the reference — a real drift guard.
- **Slice B — rich board drill-in card (Go/TUI).** `openDrill` now assembles the card via **deterministic, LLM-free reads**: a pure `sessionRows(reg, live, slug)` reader merges tracked registry legs (go/plan, with lifecycle status + cost/turns) with live tmux sessions, each cross-checked by **exact `SessionMatchesSlug`** and flagged live/stale/untracked; `viewDrill` renders a detail panel (description / folder / status / sessions / events tail) above the still-openable file list; `updateDrill` wires `a` (attach) and `K` (kill → `huh` confirm → injectable `killer` seam), with `k` left as up-nav. Kill/attach act only on real live sessions and **never mutate pipeline state**. Version bumped in `plugin.json` + `main.go` together; README keymap + help updated.

**Decisions** — all five plan forks (D1–D5) were accepted as recommended before code: D1 on-demand skill as the CLI reference home · D2 inline `a`/`K` keys · D3 compact inline events tail · D4 registry + live-tmux cross-check for liveness · D5 A→B slice order. D6 arose at the test gate → user chose to **skip** the Slice A live behavioural proof (all artifact-level prerequisites green; confirm opportunistically in real CLI use).

**Review / test verdict** — Review: **APPROVE**, clean (one round); two low findings fixed and re-verified (REV-001 kill-confirm cancel returns to the drill; REV-002 the sync lint now greps `printHelp` too — the fourth source). Test: **green** (one round); automated gate green (`gofmt -l .` · `go vet ./...` · `go test -race ./...`) and the drill card driven hands-on in a throwaway tmux pane; TEST-001 (an unrendered status line that made the `a`/`K` hints and kill/detach confirmations silent no-ops in the live TUI) fixed + regression-tested; TEST-002 (whether an *installed* Claude surfaces the CLI) user-skipped per D6.

**Knowledge** — added the TEST-001 lesson to `test-strategy.md` (assert the rendered `View()` output for status/hint paths, not just `Model.status`) and a mirrored reviewer check to `code-review-standards.md`.

**Follow-ups** — the **active** gogo-cli half (the assistant *drives* the CLI) is deferred; this shipped the passive reference it will extend. Also open: an optional lock-owner line in the panel (D4=B), a first-class cost/turns metrics view, and the opportunistic TEST-002 live check.

## Diagrams

- **`cockpit-cards-and-cli-awareness-flow.mmd`** (flow) — Part B: `enter` → `openDrill`/`loadDrillCard` assembles the card from state.md + registry + live-tmux + events; `viewDrill` renders the panel; `a`/`K` act on live sessions (never mutating pipeline state).
- **`cockpit-cards-and-cli-awareness-class.mmd`** (class) — Part B structure: the new `sessionRow` type and the `Model` drill fields/seams (`killer`, `registry`) → `orchestrator.Registry`/`PersistentSession`.
- **`cockpit-cards-and-cli-awareness-flow-cli-awareness.mmd`** (flow) — Part A: the lean pointer → on-demand `gogo-cli` reference, and the four-source enumeration-sync rule.

The `before/` set carries the plan-time as-is drill-in (file-list-only) baseline for the viewer's before/after compare mode.
