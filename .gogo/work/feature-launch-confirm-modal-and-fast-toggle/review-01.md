# Code review — round 1 · `launch-confirm-modal-and-fast-toggle` (0.36.0)

**Scope.** The uncommitted working tree vs `HEAD`: 19 modified files + 3 new
(`cli/internal/tui/modal.go`, `cli/internal/tui/launch_modal_test.go`,
`cli/internal/launch/fast_param_test.go`). Reviewed against the **resolved** plan
(**D1=B** three-option Select · **D2=B** launch-confirm site only · **D3=A** strip+dim
backdrop · **D4=A** `formOrigin` replaces `pickerOrigin`), `code-review-standards.md`,
`coding-rules.md` (Go section: TEST-001 / TEST-005 / TEST-006 / CONFIRM-DEFAULT) and
`non-functional-requirements.md`.

**No product file was modified by this review.** Every behavioural probe was run through
`go test -overlay=<scratchpad>/overlay.json`, which injects a throwaway `_test.go` into the
package at build time without writing it into the repo, plus a standalone scratch module
against the pinned `huh v1.0.0` / `lipgloss v1.1.1-…` / `x/ansi v0.11.6`.

## Gates (re-run by the reviewer, not taken on trust)

| Gate | Result |
|---|---|
| `cd cli && gofmt -l .` | clean (no output) |
| `cd cli && go vet ./...` | clean |
| `cd cli && go test -race ./...` | **green** — 13 packages |

## Findings

| id | severity | priority | status | one-line |
|---|---|---|---|---|
| REV-001 | **major** | P1 | new | the Select's `Cancel` option is not rendered whenever a label wraps — i.e. at almost every realistic size/slug |
| REV-002 | minor | P2 | new | the modal also wraps the ship (`d`) and accept confirms, not just the `/gogo:go` confirm |
| REV-003 | minor | P2 | new | the CONFIRM-DEFAULT structural guard no longer pins the go confirm's real default |
| REV-004 | minor | P3 | new | four in-code comments still name the renamed `pickerOrigin` |
| REV-005 | nit | P3 | new | dead `startForm` wrapper reworked instead of deleted |

### REV-001 — major · P1 · new — the third option is invisible

`move.go:465-486` builds option labels of the shape
`Launch --fast · will run: claude "<cmd>"  (token-lean gogo-fast pipeline)`, and
`modal.go:27` caps the form at `modalMaxFormW = 120`. The label needs
**`86 + len(slug)` columns**; the form gets `min(120, termW-12)`. So the list renders
complete only when **`termW >= 98 + len(slug)`**, and **never** when the slug is longer
than **34 characters**. Measured by driving the real `Model.View()`:

| fixture | terminal | `Cancel` rendered? |
|---|---|---|
| slug `next` (4 chars) | 200x40, 120x30 | yes — *this is what the new suite tests* |
| slug `next` | 100x24, 80x24 | **no** (and the `--fast` label is cut mid-line) |
| slug `launch-confirm-modal-and-fast-toggle` (36) | 200x40 | **no** |
| `M`-force confirm (FORCING description) | 60x12, 70x14 | **no launch option renders at all** |

Mechanism, verified in the vendored deps: huh sizes the group from `group.rawHeight()`
(one line per option, computed *pre-wrap*), `Group.WithHeight` then clamps the field
(`huh@v1.0.0/group.go:127-139`) and `Select.updateViewportHeight` sets
`viewport.Height = height - title - description` (`field_select.go:526-544`). The option
viewport is therefore ~3 **rows** for 3 options, so a wrapped label pushes the tail out of
view. The 0.35.0 Confirm could not show this (Launch/Cancel are one row), and the
small-terminal fallback cannot either (`height <= 0` sizes the viewport to the *rendered*
height — which is why `Cancel` *is* present in the 46x9 fallback). So it is a regression
introduced by D1=B + the modal's `WindowSizeMsg` layout. A source configured with
`--skip-acceptance --skip-uat` loses a further ~30 characters of slug budget.

`Cancel` stays *reachable* (↓ scrolls the viewport, `esc` aborts) — hence major, not
blocker — but the option the user was told to choose from is invisible with no scroll cue,
which defeats the exact reason **D1=B** was picked over D1=A ("maximally discoverable —
both launch modes are visible"), and trips `code-review-standards.md` **#8** (a
user-visible state must actually be rendered by `View()`).

The suite cannot see it (`code-review-standards.md` **#11**): the only fixture slug is
4 chars at 200x40; `TestModalNeverExceedsTheTerminal` measures widths/heights only; and
`TestOverlayKeepsEveryFormLine` compares the composite against `m.form.View()`, which has
**already** dropped the option.

**Proposed fix (AGENT-FIXABLE).** Keep every label to one line
(`Launch` / `Launch --fast  (token-lean gogo-fast pipeline)` / `Cancel`) and show the
effective command live in the title via `.Title(initial).TitleFunc(fn, &m.binding.launchMode)`
(huh re-evaluates on the bound value's hash change — the plan's measured probe **P3**),
with `fn` still building the string through `launch.SetFastParam(...)` so FR4's
one-producer "shown == launched" property survives. **Do not** reach for `Select.Height()`:
`Group.WithHeight` re-applies `field.WithHeight` on every resize. Then close the test gap —
parameterise over a realistic slug and add 100x24 / 80x24, asserting all three labels are
in `m.View()`.

### REV-002 — minor · P2 · new — the modal's reach is wider than D2=B's words

`move.go:454` seeds `launchSite: true` for **every** intent reaching
`startFormOverriding`, and `move.go:500` calls `enterLaunchModal` unconditionally, so the
`/gogo:done` ship confirm (merged-ship release Input included) and the `/gogo:accept`
confirm are modals too — verified by driving `d` (`modalLaunchConfirm() == true`, border +
dimmed board present). Defensible under the "one site" reading of D2=B and it is what
`charts/flow.mmd` shows, but it contradicts the strict reading of the fork
("the launch confirm ONLY for now") and of FR6 ("a ship, an accept … are byte-for-byte
today's"). `docs/cli-contract.md`'s 0.36.0 note lists what stays full-screen and silently
omits ship/accept. Net effect: `d` is a modal while `P` (also a launch confirm) is not.

**Proposed fix (AGENT-FIXABLE).** Either (a) keep the scope and say it out loud in the
cli-contract bullet, or (b) narrow with `launchSite: intent.Action == launch.ActionGo` and
assert the ship confirm renders full-screen byte-for-byte.

### REV-003 — minor · P2 · new — the structural confirm-default guard stopped biting

`launchConfirmed()` (`model.go:157-165`) branches on `launchMode` and never reads
`binding.confirm`, so `confirm: true` is vestigial on the go form.
`TestConfirmDefaultsAreAlwaysExplicit` still greps for `&formBinding{confirm: true,` and
still passes — on a field the widget no longer consults — while the real default
(`m.binding.launchMode = goLaunchFull`, `move.go:472-475`) is structurally unguarded:
seeding `goLaunchCancel` would leave the guard green. Behaviour is still pinned twice
(`TestConfirmDefaultForwardMovesSubmitOnEnter` "board m launches" + the new
`TestBareEnterStillLaunchesOnce`, both bare-Enter through the real huh lifecycle asserting
fired-exactly-once), so this is minor — but it is the "guard that looks like a check and
isn't" shape TEST-006 / standard #11 tell this repo to close.

**Proposed fix (AGENT-FIXABLE).** Extend the guard to require the go site to seed a
non-cancel `launchMode`; optionally drop the unread `confirm: true` *together with* that
assertion.

### REV-004 — minor · P3 · new — stale `pickerOrigin` prose

`session_ops_test.go:4` and `:248`, `card_test.go:345`, `sessions_panel_test.go:206` still
name the renamed field; two of those files were edited by this change (the assertion below
the sentence was updated, the sentence was not). Standard #1: grep the old term. The
`.gogo/work/` archives and `release-history/SKILL.md` are historical and correctly out of
scope.

### REV-005 — nit · P3 · new — dead `startForm`

`move.go:410-414` has no callers anywhere in `cli/` (already unreferenced at HEAD); the
diff reworked its signature rather than deleting it. Delete it.

## Verified as claimed (no finding)

- **FR4 — what the confirm shows is what launches.** Both option labels and `doLaunch`
  (`move.go:586-592`) go through the single `launch.SetFastParam` producer;
  `Session`/`Root`/`Slugs` are untouched, and the test asserts the session name explicitly.
- **FR5 — no config write.** No write path exists; `TestFastChoiceDoesNotWriteSourceConfig`
  re-reads the store after a flipped launch and the source's `fastMode` is unchanged.
- **FR6 — no fast option, no intercepted key on non-go confirms.** `launchMode` is `""`
  everywhere but the go arm; there is no `f`/`F` interception anywhere in `updateForm`, and
  typing `f` into the merged-ship release Input still types `f`. (Only the *modal* half of
  the claim differs — REV-002.)
- **FR7 — bare Enter still launches once**, at both seeds, and the structural guard still
  finds `&formBinding{confirm: true,` in `startFormOverriding` (its *strength* is REV-003).
- **FR11 — the modal never exceeds the terminal.** Stress-probed over 200x40 / 120x30 /
  90x24 / 80x20 / 60x12 / 60x40 with a 90-char unbreakable slug and an 82-char root: max
  line width always **== termW**, height always **== termH**, and the box never overflows
  vertically (worst case measured 13 rows on a 14-row terminal).
- **FR12 — fallback.** At 46x9 and unsized, `View()` is `"\n" + m.form.View() + "\n"`
  byte-for-byte, with no border glyph; shrinking a live modal below the minimum falls back
  the same way (one producer, `modalFormSize`, used by both layout and render).
- **D4=A — `formOrigin`.** Recorded at all seven former `pickerOrigin` sites plus
  `enterLaunchModal`; every `finish*` and `cancelForm` reads it, and `cancelForm` reads
  `m.binding.launchSite` at line 655 **before** clearing the binding at line 683 — the
  order holds. The completion branch's `m.mode = m.formOrigin` is reachable only for the
  launch site (every other form returns earlier via its `pending*`), so there is no
  stale-origin path.
- **TEST-005 exactness.** `HasFastParam`/`SetFastParam` split on whitespace and compare the
  token exactly; `fast-path`, `fast`, `--fastest` are untouched, and `SetFastParam(c,false)`
  early-returns on the no-op so no command is re-spaced.
- **Version + docs.** `plugin.json` and `cli/main.go` both at 0.36.0 with
  `version_test.go` updated; README, `skills/gogo-cli/SKILL.md`, `docs/cli-contract.md`
  (0.36.0 presentation-only note) and `printHelp` all carry the change; `go.mod` moves
  `x/ansi` to the direct block with no `go.sum` change; charts are refreshed to as-built.

## Verdict

**CHANGES** — 1 major (REV-001) is open, so the round loops back to ② implement with
`--issues review/issues.json`. No blocker, no `NEEDS-USER-DECISION` gate; REV-002 carries
two acceptable answers and the reviewer recommends option (a).
