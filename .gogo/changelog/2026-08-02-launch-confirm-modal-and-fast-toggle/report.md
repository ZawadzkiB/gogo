# launch-confirm-modal-and-fast-toggle — a modal launch confirmation with a per-launch --fast toggle (0.36.0)

**Shipped 2026-08-02.** The cockpit's launch confirmation stopped being a full-screen takeover: pressing `m`/`M`/`d` now draws a **bordered modal composited over the still-visible, dimmed board** (every backdrop line ANSI-stripped and re-rendered dim, then the box's cells spliced in — the SGR-bleed bug class is deleted, not managed). And the `/gogo:go` confirm became a **three-option select** — `Launch` / `Launch --fast  (token-lean gogo-fast)` / `Cancel` — pre-highlighted from the source's `fastMode` config, with the exact command shown in the title and **re-evaluated live** as the selection moves. One producer (`launch.SetFastParam`) builds both the shown and the launched command, so they agree by construction; a bare Enter still launches exactly once; the choice is **per-launch only** — the source's `config.json` is never written. The modal needs a terminal of at least **60x15**; below that (or before the first resize event) the render is the old full-screen form **byte-for-byte**.

**What was changed / done / implemented:**

- `launch` package: the `FastToken` / `HasFastParam` / `SetFastParam` producer trio (field-exact token matching, idempotent both ways).
- `tui`: a new pure `modal.go` (`modalFormSize` — the one size rule for layout AND render; `overlayCenter` — strip+dim splice via `x/ansi`, whose ruler IS lipgloss's), the go Select with a live `TitleFunc` title, `enterLaunchModal` recording **`formOrigin`** (one field replacing `pickerOrigin` for backdrop + return mode), and the `M`-force note compressed to the cap + blocking slugs.
- Scope: the one `m`/`M`/`d` launch-confirm **site** composites (ship + accept confirms included); `P` and the other 14 form sites keep the full-screen takeover — stated in `docs/cli-contract.md`'s "Changed in 0.36.0 — presentation/interaction only" note.
- 19 new test functions (598 total): seed-from-config, live title, fires-exactly-once, no-config-write, the FR6 no-fast boundary, modal-over-recorded-origin, a realistic-slug/root size matrix down to the exact 60x15 minimum, byte-for-byte fallback, and pure overlay units.

**Key decisions (one line each):**

- **D1=B** — a three-option Select over an `f`-toggle: the user's literal "select option we want"; both launch modes visible, one Enter kept.
- **D2=B** — modal at the launch-confirm site only "for now" (clarified in-flight: the *site* serves `m`/`M`/`d`, so ship/accept are modals too; `modal.go` stays reusable per-site).
- **D3=A** — strip + dim backdrop (deterministic; no ANSI bleed possible).
- **D4=A** — one `formOrigin` field replaces `pickerOrigin` (backdrop and return mode can never disagree).
- **REV-001 (review's major)** — the command moved from the option labels into a live title: huh sizes a Select one *pre-wrap* row per option, so command-carrying labels silently pushed `Cancel` out of view at realistic slug lengths.
- **REV-006** — the modal minimum is the measured **60x15**, not the drafted 60x12 (a real-length repo root wraps the title taller).

**Review:** clean after 2 rounds — 7 findings (1 major, 5 minor, 1 nit), all fixed and **verified** (the round-2 reviewer mutation-checked the new guards; the phase-④ tester independently verified the last two fixes live). **Test:** green — full `-race` suite (13 packages) plus a real-terminal tmux drive of the built binary at six sizes with a stubbed `claude` (launch argv asserted, config-never-written confirmed, `NO_COLOR=1` pass); one minor closed by its own tests-only required part.

**Diagrams:** the as-built confirm flow (Select/Confirm fork → modal composite → one-producer launch), with the plan-time before set for side-by-side compare.

Full audit trail: [.gogo/work/feature-launch-confirm-modal-and-fast-toggle/](../../work/feature-launch-confirm-modal-and-fast-toggle/) — plan (with the as-built deltas), decisions, 2 review + 1 test round, per-file changes table, UAT log.
