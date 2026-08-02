# Report — card-selection-border (0.35.0, fast-mode run)

**Shipped:** border-only card selection. Focusing or selecting a cockpit card no longer
repaints its content — the status pill (incl. its tint), origin dots, session ●, ⛓ chips
and slug/title styling are byte-identical to an unfocused card, and focus is carried by
the card's **frame**: a double-line border (`╔ ═ ║`) in the column accent. A
focused+selected card keeps the green select accent on the double frame; the `┃` gate
stripe is composed **over** whichever frame focus chose. Same treatment for the plans-tab
kanban card; the changelog and config-tab rows keep their colours too, marked by their
`▸` cursor + a bright slug/name.

This was the **first live `gogo-fast` run** (one warm build+verify context, ONE fresh
review pass + one delta re-review, in-context final suites, this short report — no
per-round artifacts).

## Planned vs shipped

Everything in [plan.md](../plan.md)'s Changes checklist shipped, as planned (D1=A double
border + accent, D2=A rows included): `styles.go` frame-only styles (+`cardFocusedSelected`,
`focusBorder`; `gateBorder` and `changelogFocusStyle` deleted), `renderCard` one styled
body + composed stripe, `renderPlanCard` collapse, changelog/config rows cursor-only
focus, all five `plain bool` params retired, new `card_selection_test.go` (9 guards),
version 0.35.0 (plugin.json + cli/main.go + version_test.go), `termenv` promoted to a
direct test dependency. No layout, key, or data changes; `focusBg`/`focusFg` intentionally
survive for the tab/filter/footer chips (out of scope, per plan).

## Review outcome (1 pass + 1 delta, verdict APPROVE)

| Id | Sev | Finding | Outcome |
|---|---|---|---|
| REV-001 | major | FR3 frame-pick test didn't bite (mutation-survivable) | fixed r2 → **verified** (top-border compare on column 0; mutation now fails the test) |
| REV-002 | minor | plans-tab focus cue unguarded | fixed r2 → **verified** (glyph assertions; mutation bites) |
| REV-003 | minor | config-tab rows unguarded | fixed r2 → **verified** (new seeded test; + post-approve nit: no-background-SGR assertion closes the fill-only gap) |
| REV-004 | minor | two stale comments (gateBorder, "selection bar") | fixed r2 → **verified** (+ post-approve nit: stale test comment in redesign_test.go) |

Routing note (skill feedback): all four findings were mechanical, so the round-2 budget
was spent fixing + delta-verifying them instead of parking them at this gate — you get a
clean table instead of four accept-or-bounce questions. The two nit residuals the delta
review recorded were closed post-approve and the full race suite re-run green.

## Test outcome

- **Suites:** `gofmt -l` clean · `go vet ./...` clean · `go test -race ./...` green
  (all packages; the pinned gate-stripe, workspace, waiting, correlation, unified-board
  and pickup suites unchanged and green).
- **Hands-on (real binary):** built `gogo 0.35.0` from the tree and drove the actual
  cockpit in tmux (200×45, xterm-256color), capturing ANSI at three focus positions.
  Every capture: exactly one double-framed card; inner content fully styled under focus —
  including a **gate card** showing the red `┃` stripe + blue double frame + the red
  `⏸ accept plan` pill *with its background tint* (the previously-stripped case); pill
  tints identical focused vs unfocused; rounded borders intact elsewhere; no frame fills.
- **Left for your UAT (visual-taste calls a capture can't make):** arrow around in your
  own terminal/theme — does the double border read well, not too heavy? Optionally try
  `NO_COLOR=1` (focus stays obvious by glyph) and a light-background profile.

## Diagrams

The plan-time intended-design flow (`../charts/flow.mmd`, with the as-is baseline in
`../charts/before/`) still describes the as-built frame decision exactly — no drift, so
no new chart set was drawn (fast mode draws only on genuine signal).

## Open items

None. No open review/test issues; no decisions pending. Ship via `/gogo:done
card-selection-border` (release = commit + tag v0.35.0 when you say so), or describe
issues to loop back.
