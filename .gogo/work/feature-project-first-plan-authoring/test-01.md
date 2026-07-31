# Test round 1 — project-first plan authoring (0.30.0)

**Verdict: GREEN.** Build + unit suite + hands-on CLI + hands-on live TUI all pass. Zero issues
found. Done-bar met — no hands-on check was blocked.

---

## 1. Suite (must be green first)

```
cd cli && gofmt -l . && go vet ./... && go test -race -count=1 ./...
```

- `gofmt -l .` — clean (no files listed).
- `go vet ./...` — clean.
- `go test -race -count=1 ./...` — **542 PASS, 0 FAIL**, across all 12 packages
  (`cli`, `config`, `contract`, `diagram`, `diagram/mermaidascii`, `launch`, `orchestrator`,
  `pages`, `plans`, `projects`, `trash`, `tui`).

## 2. Build

`go build -o /tmp/gogo-test-bin .` — succeeded. `/tmp/gogo-test-bin --version` → `gogo 0.30.0`
(matches plan.md FR6.3).

## 3. CLI-level hands-on

`--help` enumerations (FR6.1) confirmed in one pass:
- plans-tab key block: `m move (...) · p switch project · x delete` — the new key present.
- `multi-line entry: in a goal/description textarea, enter inserts a new line · tab advances ·
  ctrl+e opens $EDITOR; an optional attachments field takes one local path or http(s) URL per
  line (stored in the plan's front matter + named to the launched session)`.

Built an isolated fixture (`GOGO_DATA_HOME=$(mktemp -d)`, never touched the real `~/.gogo`):
3 projects (`projA`/`projB`/`projC`, each with one scratch source), 4 plans across them.

**The key regression check — attachments front-matter round-trip (FR4.1):**
1. Hand-edited `plan-19678b4f.md`'s front matter to add `attachments: /tmp/x.png,
   https://example.com`.
2. `gogo plan show plan-19678b4f --project projA` printed the `attachments:` line correctly.
3. Ran a CLI write path that re-saves the file: `gogo plan ready plan-19678b4f --project projA`
   (draft → ready).
4. Re-read the raw file: `attachments: /tmp/x.png, https://example.com` **survived byte-for-byte**
   — the closed front-matter parse/render round-trip did not drop the hand-added key. This is
   exactly the failure mode plan.md calls out as FR4.1's reason to exist, and it does not occur.

No other CLI-level defects observed.

## 4. Live TUI (tmux, mandatory per test-strategy.md)

Launched detached: `tmux new-session -d -s "gogo-test-plansauth-$$" -x 220 -y 50
"GOGO_DATA_HOME=$GDH /tmp/gogo-test-bin global"`, `Tab` to the plans tab.

- **Header always names the project (FR2.2).** Before pressing anything:
  `● projA  (p to switch)` was on screen.
- **`p` switches in place (FR2.1).** Pressed twice: `● projA` → `● projB` → `● projC`, kanban
  reloaded to each project's own plan on every press, no detour off the plans tab.
- **`n` mint form is project-first (FR1.1/FR1.3).** First field was `Project` with `> projC`
  pre-selected (the then-focused project); description read "creates a draft plan in the
  project selected above".
- **`enter` inserts a newline, does not submit (FR3, the direct regression guard).** Typed
  `first line of the goal`, `Enter`, `second line of the goal` into the Description field —
  both lines rendered inside the textarea, form stayed open (footer still showed `enter new
  line • ctrl+e open editor • shift+tab back • tab next`).
- **`esc` cancels cleanly (FR1.6).** Status line showed `cancelled`; `gogo plan list --project
  projC` confirmed the draft count did not change (still 1).
- **`A` form is project-first too.** Same `Project` select as the first field, pre-seeded to
  `projC`; description named "the selected project's sources". Cancelled with `esc` before
  reaching submit — never risked launching a real `claude` session.
- **No stray `claude` process/session.** `tmux list-sessions` and `ps aux | grep claude` after
  both drives showed only the user's own pre-existing sessions (`gogo-accept-*`, background
  daemons) — none attributable to this test run, and no new one appeared.
- **ATTACHMENTS block renders (FR4.5).** Opened `plan-19678b4f`'s detail: `ATTACHMENTS` block
  listed `/tmp/x.png` and `https://example.com`. Hand-edited `plan-112c9d54` (projB) to point at
  a nonexistent local file and re-opened its detail: rendered `/tmp/this-file-does-not-exist-
  gogo-test.png · missing` — the missing marker works live.
- **Comma refusal renders live (FR4.4).** In a second tmux drive, typed `/tmp/a,b.png` into the
  Attachments field of a fresh `n` form and pressed `ctrl+d`: the form stayed open and rendered
  `* "/tmp/a,b.png": a comma cannot be stored (the attachments: list splits on it)`. Cancelled
  with `esc`; `gogo plan list --project projA` confirmed nothing was minted (still 2 plans).

Both tmux test sessions were killed on completion (`tmux kill-session`); `ps aux` confirmed no
leftover `gogo-test-bin` process.

## 5. New/extended tests

Reviewed the diffs already shipped by implement/review rounds before considering additions:
`git diff abb3def -- cli/internal/plans/plans_test.go cli/internal/launch/launch_test.go
cli/internal/tui/plans_tab_test.go`.

- `plans_test.go`: `TestAttachmentsRoundTrip`, `TestAttachmentsSurviveResave`,
  `TestNoAttachmentsNoLine` — cover exactly the New→SetAttachments→List/Get round-trip, the
  re-save-does-not-drop case, and the empty-set-writes-no-line byte parity I exercised by hand.
- `launch_test.go`: `TestWithAttachmentsEmptyUnchanged` (byte-for-byte via `reflect.DeepEqual`),
  `TestWithAttachmentsNamesEach`, `TestWithAttachmentsBounded` (both the entry-count and the
  byte bound, with a named `+N more` truncation marker) — matches FR5.1/5.4.
- `plans_tab_test.go`: `TestMintFormsProjectFirst`, `TestMintLandsInChosenProject`,
  `TestPlanWithClaudeAnchorFollowsChoice` (the FR1.5 trap asserted directly via the fake
  launcher's recorded root), `TestMintCancelMintsNothing`, `TestPlansTabPSwitchesInPlace`,
  `TestGoalMultilineEntry` (drives the real huh form message-by-message, the FR3 direct guard),
  `TestAttachmentNormalize` (comma on raw AND normalized value per REV-004, directory refusal
  per REV-007), `TestAttachmentFormValidationAndPersist`, `TestSpawnCarriesAttachments`,
  `TestNewFormIsTheOnlyFormConstructionSite` (REV-002/REV-009's wiring guard with the
  reachability check), `TestPlansBoardFitsTerminalHeight` (REV-005's rendered-height guard).

This coverage already matches every hands-on behaviour observed above one-for-one — no genuine
gap surfaced by exploration, so **no new tests were added this round**.

## 6. Non-functional bars checked

- **Portability** — the URL attachment (`https://example.com`) was shape-checked only; nothing
  in this session made a network call, and `validateAttachments`/`normalizeAttachment` never
  fetch.
- **Safety** — every write landed under the isolated `$GOGO_DATA_HOME`; the real `~/.gogo/
  projects/` was confirmed to carry none of `projA`/`projB`/`projC` after cleanup. No source's
  `.gogo/work/` was touched (no real spawn was triggered — both `A` drives were cancelled before
  submit).
- **Diagnosability** — the comma refusal and the `· missing` marker are both glyph/word-based
  (`*`, `· missing`), not colour-only, and rendered in a TTY-less `tmux capture-pane -p` exactly
  as they would in colour.

## 7. Issues this round

None. `test/issues.json` written with an empty `issues` array (round 1, no prior file existed).

## 8. Hands-on checks blocked?

None. tmux was available, the binary built and ran, `gogo global` opened the cockpit under the
isolated data home, and every planned hands-on step (CLI round-trip, TUI project-select/switch/
multiline/attachments/cancel) completed to a real terminal state. No user-decision gate needed.

## Done-bar

Build green + unit green (542/542) + e2e/hands-on done at every relevant level (CLI + live TUI),
zero open issues. **Advance to phase ⑤ report.**
