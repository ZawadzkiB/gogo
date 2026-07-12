# Adjustments — feature `immediate-kill-at-ship`

Log of changes / clarifications requested during planning and the UAT loop. Each
entry: date, what was asked, and how the plan changed.

- **2026-07-12 (review decision D4 → B).** Review surfaced REV-002 (a plain-`gogo sweep`
  ship-reap can truncate a *different* feature's concurrent `/gogo:done`). User overrode the
  plan's **D1=A** (plain sweep): the `/gogo:done` ship-reap must be **targeted** -
  `gogo sweep <slug>...` reaping ONLY the shipped slug(s)' sessions - so a ship never touches
  another card. Plain `gogo sweep` (no slug) stays the manual whole-board cleanup. This moves
  the previously **out-of-scope** targeted-sweep (D1=B) **into scope** for 0.17.0. REV-001
  (own host window lingers) accepted as works-as-designed.
  - **Plan reconciliation (done at ④ test, closing TEST-001, 2026-07-12):** `plan.md` was
    reconciled to the as-built D4→B design — an "As-built adjustment" banner up top, and the
    acceptance signal, Approach step 2, the "why targeted sweep" rationale, the D1 alternative,
    the Changes checklist, the Tests table, and the Out-of-scope line all updated to the shipped
    targeted `gogo sweep <slug>...`. (This entry originally claimed the plan was already updated;
    that reconciliation is what this line records.)
