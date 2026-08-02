# card-selection-border — border-only card selection (0.35.0)

Focusing or selecting a card in the gogo cockpit no longer repaints its content. The old
focused-card treatment was one foreground+background fill over the whole card — and
because a fill tears at every inner ANSI reset, the focused card's body was deliberately
rebuilt with **all colours stripped** (pill tint, origin dots, session ●, chips gone the
moment the cursor arrived). Now the body is **one styled render for every state** and
focus/selection choose only the **frame**: a double-line border (`╔ ═ ║`) in the column
accent marks the focused card (visible even on a colourless terminal), a
focused+selected card keeps the green select accent on the double frame, and the `┃`
gate stripe is composed over whichever frame focus chose — a focused gate card shows
both. The plans-tab kanban card gets the identical treatment, and the changelog +
config-tab rows keep their colours too (focus = their existing `▸` cursor + a bright
slug/name).

**Decisions:** D1 = double-line border + column accent (the only marker that survives
`NO_COLOR` and stays testable); D2 = the changelog/config rows included (same defect
class, retiring the last `plain bool` parameters).

**Review/test verdict:** one fresh review pass found 0 blockers and 4 mechanical
findings (mutation-survivable tests, stale comments); all fixed and mutation-verified in
one delta re-review — APPROVE, 0 open. Full `go test -race` green; hands-on verification
drove the real 0.35.0 binary in tmux and confirmed, capture-by-capture, styled content
inside the focus frame (including a gate card's tinted `⏸ accept plan` pill — the
previously-stripped case).

**Notable:** first feature shipped through the **gogo-fast** pipeline (0.34.0) — one
warm build+verify context + one fresh review, 17 artifact files / 65KB total vs the
~800KB a comparable full run produced.

Full audit trail: [.gogo/work/feature-card-selection-border/](../../work/feature-card-selection-border/)
