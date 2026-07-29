# Test round 02 — `plans-tab-launch-diagnostics-and-view`

**Date:** 2026-07-29 · **Tester:** gogo-tester · **Scope:** TEST-001's fix
(`confirm: true` seeded at `startPlanSpawnForm`/`startPlanDoneForm`, the new confirm-default
convention, and the two round-01 test-fixture `Correlations` bugs the developer found and
fixed alongside it) — re-verified hands-on against the real TUI, plus a focused regression
resweep of what `plans_tab.go` touches.

Same isolated `GOGO_DATA_HOME` fixture tree as round 1 (`/tmp/kastest-pt-launch-diag/`),
same stub `claude`, same "never the two real user sessions" discipline throughout.

---

## 0. Baseline (re-confirmed independently, not just taken from the developer's report)

```
gofmt -l .            → (no output)
go vet ./...           → clean
go test -race ./...    → ok, all 13 packages
go build -o /tmp/kastest-gogo .   → builds
/tmp/kastest-gogo --version        → gogo 0.28.0
```

Also read the new `cli/internal/tui/confirm_default_test.go` in full before driving anything
live: it names the exact convention (`forward pipeline move → confirm: true → Enter submits`;
`destructive → confirm: false → Enter is safe`), asserts both halves by driving a bare Enter
through the real huh lifecycle and checking the real side effect, and has a structural test
(`TestConfirmDefaultsAreAlwaysExplicit`) that fails if any of the five confirm constructors
ever seeds an implicit/unstated default again. This is genuinely thorough — my job below is to
confirm the SAME properties hold when driven with real keystrokes against the real binary, not
just inside `go test`.

---

## 1. Plans tab, `m` on a ready plan → bare Enter → the real side effect

Fresh fixture (not reused from round 1): `plan-kastestr2go1` (ready, target `srca`, small
body — this check is about the confirm default, not the fold).

```
BEFORE: plan-kastestr2go1  [ready]   (no work items)
m → "Accept plan-kastestr2go1 and spawn 1 work item(s)? into: srca"
Enter (bare)
AFTER:  plan-kastestr2go1  [active]
        1 work item:
          srca : kastest-r2-go-fixture
```

A real tmux session `gogo-plan-kastest-r2-go-fixture` was created (`tmux list-sessions`
confirmed it; pane showed the stub `claude` invoked with `/gogo:plan ... --correlation
plan-kastestr2go1`), then killed. **Real launcher fired, plan flipped ready→active, member
recorded** — not just "the dialog closed".

---

## 2. Plans tab, `m` on an all-members-shipped plan → bare Enter → the project-UAT accept lands

Built the fixture carefully to avoid the exact mistake the developer's own fix report flagged
(two of ITS round-02 fixtures passed for the wrong reason because they omitted the member's
`Correlations`, so the gate refused on "member not found" rather than "member unshipped"):

- `srcb/.gogo/work/feature-kastest-r2-shipped/state.md`: `status: shipped`, **and**
  `correlation: [plan-kastestr2uat1]` — the plan's own id, so `memberFeature` can actually
  resolve it.
- `plan-kastestr2uat1.md`: `status: active`, `targets: srcb`, `members: srcb:kastest-r2-shipped`.

Loading the plans tab, the card already showed **`awaiting-project-uat · press m`** before I
touched anything — confirming the derived-status computation resolved the member correctly.

```
BEFORE: plan-kastestr2uat1  [active]   1 work item: srcb : kastest-r2-shipped
m → "Accept project-UAT for plan-kastestr2uat1? all members shipped — flips this plan to
     done + records a project-UAT round (~/.gogo/ only)"
Enter (bare)
AFTER:  plan-kastestr2uat1  [done]
```

Raw plan file after, confirming the real write (not just a status flip):

```
status: done
...
## Project UAT
## UAT round 1 - accepted (user, 2026-07-29) - via gogo plan done
```

**The accept genuinely landed** — plan flipped to `done`, a `## Project UAT` round was
appended by `plans.MarkDone`, exactly as the contract describes.

---

## 3. Board, `m` → bare Enter still launches (quick confirmation, unchanged)

Reused round-1's `kastest-freework` (srcb, `plan-accepted`, uncapped once I killed an
unrelated fixture session occupying `srcb`'s cap — see item 5 below, this was itself a nice
incidental re-confirmation that cap-bounce severity is untouched). `m` → confirm → bare
`Enter` → `launched /gogo:go kastest-freework → tmux gogo-go-kastest-freework (press a to
attach)` (dim/plain), real session confirmed via `tmux list-sessions`, then killed. Unchanged.

---

## 4. THE SAFETY CONVENTION — highest priority, driven live, both halves

**`x` delete, bare Enter (`cli/internal/tui/delete.go:31`, `startDeleteForm`):**

Fresh fixture `feature-kastest-r2-deleteme` (srcb). Confirmed present on disk and
`.gogo/trash/` empty (0 entries) before. Pressed `x` →
`"Move kastest-r2-deleteme (unfinished) to .gogo/trash/? Delete   Cancel"` → bare **Enter**.

```
status line: "cancelled"
AFTER: feature-kastest-r2-deleteme/  → still present on disk
       .gogo/trash/                  → still 0 entries
```

**`K` kill, bare Enter (`update.go:725`, `startKillForm`, reached via drill-in):**

Fresh fixture `feature-kastest-r2-killme` (srcb, `implementing`) + a real fake live session
`gogo-go-kastest-r2-killme`. Drilled in (`enter`), confirmed the drill panel showed
`● untracked live gogo-go-kastest-r2-killme`, pressed `K` →
`"Kill kastest-r2-killme's live session? Kill   Cancel"` → bare **Enter**.

```
status line: "cancelled"
AFTER: tmux list-sessions still shows gogo-go-kastest-r2-killme: 1 windows (alive)
```

**Both destructive confirms are still safe on a bare Enter. No blocker. The convention has
not moved** — `startDeleteForm`/`startKillForm` still explicitly seed `confirm: false`
(confirmed both in source, with the "do not flip this for consistency" comment intact, and
now live).

---

## 5. Regression resweep — scoped down where the peer's ask said it was safe to

**Fold-to-pointer (re-driven, briefly):** fresh `plan-kastestr2huge1` (~20KB body, no
Source-briefs section, same reproduction shape as round 1) → `m` → bare **Enter** (this
round's fix means Enter alone submits now) → `accepted plan-kastestr2huge1 — spawned 1 work
item(s)`. Session `gogo-plan-kastest-r2-huge-fold-regression` created; pane showed:

```
ARG[3]: /gogo:plan read your brief at .../plan-kastestr2huge1.md, section
`## Source briefs` -> `### srca` --correlation plan-kastestr2huge1
```

Still clean, still the pointer — **no regression to inlining or `exit status 1`**. Killed.

**`v`/`w` (re-driven, both origins):** `plan-kastestview1` (unchanged from round 1) — `v`
from the list rendered the `KASTEST_VW_MARKER_XYZ` marker, `esc` returned to the list
(pane alive, no crash); `enter` into detail, `v` again rendered the same content, `esc`
returned to the detail (not `modeDrill`, pane alive). `w` rebuilt the page (status line named
the same `.../resources/view/plan-kastestview1.html` path); re-checked `src-clean` empty
before AND after — still 0 entries, invariant holds.

**Scoped down, as agreed with the peer (confirm-default seeding is orthogonal to these, and
nothing in this fix touches them):**
- **Item 3 (the backstop)** — not re-derived; the 320-source `A`-flow fixture and the
  `CommandTooLongError` path are untouched by this change (no confirm involved before the
  preflight fires).
- **Item 6 (REV-009 upgrade transition)** — not re-derived; `SessionMatchesSlug` and the
  session-attribution widening are untouched by a confirm-default seed.
- **Item 5 (full cap-legibility sweep)** — scoped to one incidental re-confirmation (the
  `⚠ cap 1 reached in src-b - already building kastest-r2-killme ...` bounce hit while
  setting up item 3 above) plus the dangling-target refusal below, per the peer's "one
  severity check is enough" allowance.

**One dangling-target refusal (quick sanity):** `plan-kastestdangling1` (unchanged,
`targets: srcmissing`) → `m` → still refuses before any confirm: `⚠ plan targets srcmissing,
which is not a source of project kastest - add it in the config tab, or retarget the plan`.
Unaffected, as expected.

---

## Issues found this round

**None new.** `test/issues.json` updated in place: `round` → 2, `updated` → today,
**TEST-001 → `verified`** (both bare-Enter forward-move paths fire their real side effect
live, and — critically — the safety convention on delete/kill was independently re-driven
and holds; no blocker found, so verification is warranted). No TEST-002 was needed.

## Cleanup confirmation

- `tmux list-sessions` before starting this round: the two real user sessions present.
- Every scratch/fixture session created this round (`kastest-r2-main`,
  `gogo-go-kastest-r2-killme`, `gogo-go-kastest-freework`, `gogo-plan-kastest-r2-go-fixture`,
  `gogo-plan-kastest-r2-huge-fold-regression`) was killed by exact name as soon as it was no
  longer needed.
- Final `tmux list-sessions`: **only** the two real protected sessions remain, untouched
  (read-only lists only; never attached to, killed, resized, or sent keys).
- No product code was edited this round (a needed change would have been a new finding, not
  a fix) — `git status --porcelain` in `cli/` shows no kastest-related files, confirming
  nothing stray was left in the tree.

## Verdict

**PASS.** TEST-001's fix holds under real, live keystrokes: both forward-move bare-Enter
paths (plans-tab spawn, plans-tab project-UAT accept) fire their genuine side effect
(real launcher invocation, real store mutation — not just a closed dialog), the board's own
bare-Enter launch is unchanged, and — the highest-priority check this round — the destructive
half of the convention (delete, kill) still defaults to Cancel and a bare Enter does nothing
on either, driven live, no blocker. The round-01 regressions this fix could plausibly have
touched (fold-to-pointer, `v`/`w`) were re-swept clean; the three items orthogonal to this
fix (backstop, REV-009, full cap sweep) were explicitly scoped down per the peer's guidance
rather than silently skipped. Reporting this verdict for the peer's independent spot-check
and `test/result.json` write, per the round-02 instructions — I have not written
`test/result.json` myself.
