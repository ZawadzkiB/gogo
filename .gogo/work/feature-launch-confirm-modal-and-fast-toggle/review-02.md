# Code review — round 2 (re-review after the fix round) · `launch-confirm-modal-and-fast-toggle` (0.36.0)

**Scope.** The uncommitted working tree vs `HEAD` (23 modified + 3 new files), with the
round-2 delta — `move.go`, `confirm_default_test.go`, `launch_modal_test.go`,
`docs/cli-contract.md`, `adjustments.md`, the four comment renames — reviewed both as a
fix-verification pass and with fresh eyes. Against the **resolved** plan (**D1=B** three-option
Select · **D2=B** launch-confirm site only · **D3=A** strip+dim backdrop · **D4=A** `formOrigin`),
`code-review-standards.md`, `coding-rules.md` and `non-functional-requirements.md`.

**No product file was modified by this review.** Behavioural probes ran through
`go test -overlay=<scratchpad>/overlay.json` (injects a throwaway `_test.go` at build time,
writes nothing into the repo); the two mutation checks ran against a **scratch copy** of
`cli/` (necessary because the source-scan guard reads `move.go` from disk at runtime, so an
overlay cannot reach it).

## Gates (re-run by the reviewer, not taken on trust)

| Gate | Result |
|---|---|
| `cd cli && gofmt -l .` | clean (no output) |
| `cd cli && go vet ./...` | clean |
| `cd cli && go test -race -count=1 ./...` | **green** — 13 packages, 11.4s for `internal/tui` |

## Round-1 findings — verification

| id | severity | round-1 status | round-2 status | verified how |
|---|---|---|---|---|
| REV-001 | major | fixed | **verified** | 48-cell `View()` probe + 2 mutation checks |
| REV-002 | minor | fixed | **verified** | docs + adjustments.md read |
| REV-003 | minor | fixed | **verified** | mutation check on a scratch tree |
| REV-004 | minor | fixed | **verified** | repo-wide re-grep |
| REV-005 | nit | fixed | **verified** | symbol-reference sweep |

### REV-001 — verified · the third option now renders

Every cell round 1 reported is clean, and the fix was probed **well past** them: slug `next`
**and** this feature's 36-char slug, each crossed with the fixture root `/r/web` **and** this
repo's real 37-char root, forced and unforced, at 200x40 / 120x30 / 100x24 / 80x24 / 70x14 /
60x12 — 48 cells. All three options render in `Model.View()` in every one of them except a
narrow band at the modal's own minimum (filed separately as **REV-006**).

| round-1 cell | round-1 | round-2 |
|---|---|---|
| 36-char slug @ 200x40 | `Cancel` absent | all three |
| slug `next` @ 100x24 / 80x24 | `Cancel` absent, label cut | all three |
| `M`-force @ 70x14 | **no option at all** | all three |
| `M`-force @ 60x12 (fixture root) | **no option at all** | both launch options (Cancel out — REV-006) |

The rest of the fix checks out too:

- **The title carries the exact command** in all 48 cells (`will run: claude "…"`), built
  through `launch.SetFastParam` — the same producer `doLaunch` applies, so FR4's
  shown-==-launched property is preserved by construction, not by agreement.
- **TEST-001 holds.** The `TitleFunc` closure captures `b := m.binding` (the heap-stable
  `*formBinding`), not the value-copied Model. Probe: after copying the Model and driving a
  key, the binding pointer is byte-identical and the flip is visible through the *original*
  copy.
- **The title updates LIVE in both directions** as the cursor moves —
  `Launch → Launch --fast → Cancel → back`, re-rendered at each step.
- **`forcingNote()`** keeps exactly what is being overridden
  (`FORCING past the source cap - cap 1 reached in web - already building busy`) and drops
  only the remedy tail and the rule clause. The full sentence still appears where it
  originated, on the `m` status-line bounce.
- **Both new tests bite** (mutation-checked, not assumed): restoring the round-1
  command-carrying labels fails `TestGoSelectShowsAllThreeOptions` at all four sizes;
  making `forcingNote` a pass-through fails
  `TestForcedGoSelectShowsALaunchOptionAtTheModalMinimum`.

### REV-002 — verified · option (a), said out loud

`docs/cli-contract.md:388-393` now names the scope explicitly ("the ship (`d`) and accept
confirms share it and render as modals too (incl. the merged-ship release-name input) … the
`P` plan-session confirm — a launch confirm at a different site — keeps the full-screen
takeover"), and `adjustments.md` logs the site-scope reading with its reason. No behaviour
change was needed; none was made.

### REV-003 — verified · the guard bites now

`confirm_default_test.go:197-216` requires `m.binding.launchMode = goLaunchFull` in
`startFormOverriding` and forbids `= goLaunchCancel`. Mutation-checked on a scratch tree:
seeding cancel fires **both** assertions. `confirm: true` is deliberately kept — it is still
the real discriminator for the ship/accept arm, and still pinned by the original assertion.

### REV-004 — verified · one deliberate mention left

Repo-wide `pickerOrigin` re-grep: one live hit, `model.go:402`, the lineage sentence that
*describes the rename* — correct to keep. Everything else is `.gogo/work/`, `.gogo/changelog/`
or `release-history/SKILL.md`, all historical.

### REV-005 — verified · deleted, and no new orphan

`startForm` and `confirmContext` are gone (no declaration, no call). Swept for a *new* orphan
by counting references of every symbol this change added or reshaped (`FastToken`,
`HasFastParam`, `SetFastParam`, `FastParam`, `confirmSummary`, `confirmWhere`,
`modalFormSize`, `overlayCenter`, `padToWidth`, `modalBoxStyle`, `enterLaunchModal`,
`forcingNote`, `modalLaunchConfirm`, `launchConfirmed`, `viewTab`, `viewBehindForm`): all
have real call sites. One residual doc reference is folded into REV-007 rather than
reopening this.

## New findings (round 2)

| id | severity | priority | status | one-line |
|---|---|---|---|---|
| REV-006 | minor | P2 | new | at the modal's advertised 60x12 minimum the height clamp still eats the action row (zero options when forcing with a real root; the merged-ship `Launch  Cancel` row disappears) |
| REV-007 | minor | P2 | new | six prose surfaces + the as-built chart still say the option **label** carries the exact command; `move.go:410` still names the deleted `startForm` |

### REV-006 — minor · P2 · new — the advertised minimum is 3 rows optimistic

REV-001 moved the command out of the labels and into the title — but the title is charged to
the **same** row budget huh clamps. `modalFormSize` hands huh `h = termH - 2 - 2`, so at 60x12
the form gets **8 rows** and the wrapped title alone takes 4-5. The variable round 1 never
crossed is the **repo root**: `confirmWhere` appends `  at <root>`, and this repo's own root is
37 characters.

Measured against the real `Model.View()` with the 36-char slug and
`/Users/bartlomiej.zawadzki/repos/gogo`:

| form | cells where the action row is incomplete |
|---|---|
| go Select, unforced | `Cancel` absent at 60x12, 65x12 — clean at every `h >= 13` and every `w >= 70` |
| go Select, **forced** (`M`) | **zero options** at 60x12 and 65x12; `Cancel` absent at 70-85 x12, 60-80 x13, 60-65 x14 — clean from `h >= 15` |
| merged-ship modal | the whole `Launch     Cancel` row **absent** at 60x12 and 60x14; present at 70x14 |
| single-card ship | fine at 60x12 |

Why this is a regression and not merely "a small terminal": at **59x12** — one column *below*
the minimum — the FR12 full-screen fallback renders the buttons, because an unsized form sizes
its viewport to the *rendered* height. 0.35.0 showed these rows at the same terminal size;
0.36.0 does not. And `docs/cli-contract.md`'s new note advertises the modal "on a terminal at
least 60x12", which is the claim this contradicts. This is the same failure the plan's probe
**P1** warned about ("loses the `Launch  Cancel` row entirely"), arriving via the height clamp
instead of `WithWidth`.

**FR11 is not affected** — in every cell measured the composite is exactly `termW` wide and
exactly `termH` tall and both box borders are present. This is clipped *content*
(`code-review-standards.md` **#8**), not overflow. Minor, not major: the band is roughly
60-85 columns by 12-14 rows, and Enter still launches exactly the command the (always visible)
title shows, so nothing launches unseen.

Test gap: `TestModalNeverExceedsTheTerminal` stops at 80x20 and measures only width/height;
`TestGoSelectShowsAllThreeOptions` starts at 80x24; the only 60x12 case uses a 4-char slug with
the 6-char fixture root and asserts merely that the string `Launch` appears. **The root — the
length that actually decides — is never varied anywhere in the suite.**

**Proposed fix (AGENT-FIXABLE).** (a) simplest: raise `modalMinTermH` 12 → 15 (consider
`modalMinTermW` → 70) in `modal.go:22-23` so the whole band falls into the FR12 fallback, then
update the `60x12` figure in the contract note, README and the gogo-cli skill; or (b) spend
fewer rows on the title: move the `confirmWhere` tail (`in tmux session … at <root> ·
permission: …`) into the Description, keeping the command itself in the title through
`SetFastParam`. Either way, parameterise `TestGoSelectShowsAllThreeOptions` over a ~37-char
**root**, extend it down through 70x14 / 65x13 / 60x12, strengthen the force case to the long
slug + long root, and add a merged-ship assertion at the minimum.

### REV-007 — minor · P2 · new — the prose still describes the round-1 shape

`code-review-standards.md` **#1** ("No place still describes the old behaviour. (Grep the old
terms.)") — the same class as REV-004, which this very round fixed:

| site | stale text |
|---|---|
| `README.md:469` | "each option showing the exact command it runs" |
| `docs/cli-contract.md:396` | "each launch option's label carrying the exact command it runs" |
| `skills/gogo-cli/SKILL.md:87` | "each launch option showing the exact command it runs" |
| `cli/internal/tui/move.go:418` | "each launch option's label carrying the EXACT command it runs" — **contradicted 45 lines below** by the in-body comment at `:462-466` ("The EXACT command lives in the TITLE instead") |
| `cli/internal/tui/launch_modal_test.go:17`, `:59` | "each launch option's label" / "BOTH launch options' labels carry their exact commands" — `:59` contradicted by its own body comment at `:70` |
| `charts/flow.mmd`, the `sel` node | "labels carry SetFastParam(cmd,false) / SetFastParam(cmd,true)" — the **as-built** chart, still drawn as round 1 |

Plus `cli/internal/tui/move.go:410`: "startFormOverriding is startForm plus the FR3.3 override
note" — `startForm` was deleted by the REV-005 fix, so the sentence defines the function in
terms of a symbol a reader cannot find.

Docs-only, but three sites are user-facing and one of them is the CLI contract a headless
consumer is told to trust. **Proposed fix (AGENT-FIXABLE):** one wording everywhere — the two
launch options are one-line labels, the exact command lives in the confirm's **title** and
updates live as the selection moves — then redraw the `sel` node, drop the `startForm`
reference, and re-grep.

## Also checked this round (no finding)

- **No key is intercepted on the new Select.** Driving `y`, `n`, `f`, `F` on the go confirm:
  zero launches, `launchMode` unchanged, all three options still rendered. `y`/`n` still
  submit the ship confirm (fired exactly once). The `y`/`n` loss on the go select is stated in
  the 0.36.0 contract note.
- **Resize while a modal is open.** 200x40 (modal) → 46x9 shrinks to the byte-for-byte
  full-screen fallback with no border glyph → 120x30 grows back to a correctly sized modal;
  no line exceeds the width and no view exceeds the height at any step.
- **FR11 across the small band.** At 60x12 / 60x13 / 62x12 / 70x14 / 80x20 / 80x24 / 100x24 /
  120x30 / 200x40, forced and unforced, with the long slug + real root: max line width
  `== termW`, line count `== termH`, and both `╭` and `╰` present (the box is never clipped).
- **`launchConfirmed()` folds the two discriminators correctly** and is nil-safe;
  `cancelForm` reads `binding.launchSite` before the binding is cleared.
- **Version + gates.** `plugin.json` and `cli/main.go` at 0.36.0 with `version_test.go`
  updated; `go.mod` moves `x/ansi` to the direct block with no `go.sum` change.

## Verdict

**APPROVE** — no open blockers or majors. REV-001 through REV-005 are all **verified**; the
two new findings (REV-006, REV-007) are minors and neither blocks phase ④. Both are cheap and
worth batching into one pass before ship: REV-006 is a one-constant change plus a root-length
test parameter, REV-007 is a six-site wording sweep. No `NEEDS-USER-DECISION` gate.
