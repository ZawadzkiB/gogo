# Adjustments — feature `project-first-plan-authoring`

Changes and clarifications requested during planning, newest last. Each entry
records what was asked and how `plan.md` changed in response.

---

## 2026-07-29 — scope fixed to the user's three priorities (pre-acceptance)

**Asked:** turn the existing project plan `plan-1948afcd` into work, scoped to the three
stated priorities — project choice at mint, multi-line goal entry, attachments — plus the
plans-tab visibility bug. Slices C ("start work directly") and D ("add a source by browsing")
are out of scope; do not recommend the native `gogo plan go` route as the answer.

**Applied:** the plan covers A1 + B1 + B2 + the visibility bug only. Slices C and D are listed
under *Out of scope* with a pointer to `plan-1948afcd` (left at `status: ready`, untouched, as
the backlog). The spawn-vs-scope fork is recorded as **D0, already RESOLVED by the user** —
not carried to the acceptance gate as an open question.
