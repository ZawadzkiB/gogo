# Project-first plan authoring — shipped 2026-07-31 (0.30.0)

The gogo cockpit's plans tab no longer guesses which project a plan belongs to. Before this release, every plan minted from the plans tab silently landed in whichever registered project sorted first alphabetically — the cockpit's default focus — and the tab never even named the project on screen. This release makes plan authoring **project-first and body-first**: both mint forms (`n` quick-draft and `A` plan-with-claude) ask **which project** whenever several are registered (a Select pre-seeded to the focused one, mirroring the CLI's refuse-to-guess `--project` rule), and the chosen project is focused **before** anything mints — so the plan file, the knowledge dir, the source refs, and the launched session's anchor always agree. The tab itself gained a `p` project switcher plus an always-on header row naming the on-screen project. Typing became honest too: `enter` now **inserts a newline** in every multi-line field (`tab` advances, `ctrl+d` submits) via one guarded form-construction site, and plans gained first-class **attachments** — local file paths / http(s) URLs validated at submit, stored in the plan's `attachments:` front matter, shown in the plan detail and `gogo plan show`, and named (bounded) to every launched analyst/spawn session.

## What was changed / done / implemented

- **Project choice at mint** — a destination-project Select as the first field of both plans-tab mint forms when >1 project exists; submit switches the shared focus to the choice *before* minting (`focusChosenProject`), closing the silent `projs[0]` misfile for good.
- **Plans-tab reachability** — `p` switches the tab's project in place (the config tab's existing mover, reused) and a header row (`● <project>  (p to switch)`) always names what's on screen; the kanban got its own height budget so the new header never pushes the status/help lines off screen.
- **Real multi-line entry** — one `gogoKeyMap()` (Text group only: `enter` = newline) applied by `newForm()`, the single construction site behind **all 12** huh forms — pinned by a source-scan test so a direct `huh.NewForm(` can never silently regress the next Text field.
- **Attachments, end to end** — `Plan.Attachments` joined the closed front-matter set (parse + render + `SetAttachments`); the mint forms validate one path/URL per line (`~`-expansion, absolute paths, commas refused on the raw *and* resolved value, directories refused, URLs shape-checked never fetched); the detail view marks vanished paths `· missing`; and a bounded `launch.WithAttachments` decorator (≤12 entries / ≤2048 bytes, composing before `FoldToPointer`) names them to launched sessions.
- **Contracts kept honest** — the `gogo-project-plan` analyst skill now preserves every cockpit-written front-matter key (it would have silently dropped `attachments:`), the `gogo-plan` skill's param wording matches the decorated command shape, and help/README/skill/cli-contract enumerations were synced. Version **0.30.0**.

## Key decisions (one line each)

- **Keep the `projs[0]` default; make it visible + overridable** — fixes the silent misfile with the smallest surface (D1).
- **Per-project switcher, no all-projects view** — `plans.Plan` carries no project identity; a merged list would act on the wrong project (D2).
- **Attachments referenced, never copied** — plain-markdown store, no invented storage lifecycle; missing paths marked instead (D3).
- **Typed `attachments:` front-matter key, comma refused loudly** — round-trips for free, can't be broken by prose edits (D4).
- **Keymap at all 12 form sites via one `newForm()`** — the structural fix for enumeration drift, made *enforced* by a guard test in review (D5).
- **`AddAttachment` dropped in review** — exported surface with no caller is scope creep; `SetAttachments` is the one write path.

## Review / test verdict

Review: **APPROVE** after 2 rounds — 9 findings (2 major), all fixed and verified, two by mutation testing, zero open. Test: **GREEN** — 542/542 race-suite pass plus hands-on CLI and live tmux TUI verification (project-first mint, `p` switching, multi-line `enter`, attachment round-trip through a CLI re-save), nothing skipped.

## Members

| Work item | Outcome |
|---|---|
| [`feature-project-first-plan-authoring`](../../work/feature-project-first-plan-authoring/) | Shipped — full audit trail (plan, decisions, 2 review rounds, test round, as-built report + diagrams) in the work folder |
