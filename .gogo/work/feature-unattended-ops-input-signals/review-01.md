# Review round 1 — `unattended-ops-input-signals`

Fresh-eyes review of the implement round-1 diff against `plan.md` (FR-A1..A5,
FR-B1..B6, FR-C1..C5), `decisions.md` (D1-D10, all as recommended), and the
project's code-review / coding-rules / NFR standards. Scope: this feature's diff
only (the unrelated `feature-cli-orchestrator` code in `cli/internal/launch/launch.go`,
`cli/run.go`, `cli/internal/orchestrator/`, `cli/internal/contract/route*.go` was
excluded per the review brief).

Gates re-checked from `cli/`: `gofmt -l .` clean · `go vet` clean · in-scope
`go test` green (contract, tui, launch, main). The `rm` audit is fully complete —
`grep -rE '(^|\s)rm\s' skills/` returns **zero** command-shape `rm` in any skill.

## Verdict: CHANGES

One open **major** (enumeration out of sync) — everything else is correct.

## What was checked and holds

- **Slice A (bash-safety).** Both `gogo-done` deletes rewritten to guarded scoped
  `find … -delete`: `$dst` guarded non-empty AND under `.gogo/changelog/`, then
  `find "$dst" -maxdepth 1 -type f -name '*.mmd' -delete` + scoped before/ clean;
  board cleanup replaces `rm -f "$res" "$code"` with a literal-dir/named-file `find`.
  `set -euo pipefail`, empty-`$dst`-safe (guard exits first), idempotent, no new
  deps, never escapes `.gogo/`. `gogo-build` migration `mv`/`find` sites lightly
  hardened (`$legacy` re-guarded non-empty). The **lint** (`TestSkillsBashNoUnsafeRm`)
  is genuinely command-anchored (`(?:^|\s)rm\s`): no false-negative for glob-rm /
  `rm -rf "$var"` / `rm -f "$var"` (all three match), and no current false-positive
  (prose `dangerous rm"`, `glob-rm`, backtick-\`rm\` never match; verified by the
  zero-hit grep + green test).
- **Slice B (waiting signal).** `WaitingForInput()` is exactly the three gates;
  auto states (incl. empty) excluded. The ⏸ cue shows on all three waiting states —
  including `awaiting-plan-acceptance`, which had none — on both focused and
  unfocused cards (`cardBadgeText`), with the wait/uat accent on unfocused
  (`badgeStyleFor`). `badge()` precedence preserved (waiting-for-user first; a live
  session still outranks the resting gate badge). `boardColWidth` re-derives the
  3 gutter cells out of the budget (`(w-6-3)/4` + 3 seps ≈ `w-6`; no overflow,
  windowing/focus intact). Frozen contract stays additive — no status/class/column
  key removed; `docs/cli-contract.md §"Changed in 0.14.0"` records it as presentation.
  The golden flip (`feature-ready` done→awaiting-uat) is truthful: still
  ready-to-ship, ready-count unchanged, `legacy-ready` still covers the legacy-done
  path; the WAIT column aligns (102-char rule).
- **Slice C (board accept).** `move.go` branches on **status**, not class:
  `awaiting-plan-acceptance` → `ActionAccept`; `plan-accepted` (and every other
  unfinished) → `ActionGo`; `ClassInProgress` → `ActionGo` (dead end closed, both
  paths tested). `ActionAccept` `BuildIntent` arm mirrors `go` (single argv, sanitized
  session); `SessionMatchesSlug` adds `ActionAccept` and keeps exact-boundary matching
  (TEST-005, tested against the `awaiting-card`/`waiting-card` substring trap). CLI
  never mutates pipeline state — accept is a delegated launch; `commands/accept.md` is
  ultra-thin; `skills/gogo-accept` is present-then-record, reuses gogo-plan's single
  recording, accept-only, no second `plan-accepted` emitter.
- **Enumeration-sync + version.** `Version` 0.14.0 paired with `plugin.json`;
  12→13 synced in `docs/commands.md`, `project-knowledge.md`, README (command block +
  board-keys + "not a 14th slash command"), and `main.go` printHelp (board-keys +
  ⏸ legend). `skills/gogo/SKILL.md` carries no command count/list, so nothing to sync
  there (its single-owner-`plan-accepted` note stays accurate for the in-chat flow;
  gogo-accept records on gogo-plan's behalf, one emitter per acceptance).

## Findings

| id | sev | pri | status | title |
|----|-----|-----|--------|-------|
| REV-001 | major | P1 | open | architecture.md tree says "13 slash commands" but still lists only 12 (accept.md missing) |
| REV-002 | nit | P3 | open | Slice A `before/` refresh only deletes `*.mmd`; a non-`.mmd` file survives where `rm -rf` nuked it (harmless, plan-prescribed) |

### REV-001 (major) — `docs/architecture.md:112-114`
The label was bumped to "13 slash commands" but the file list below still enumerates
12 (`build plan go implement review test` / `report done view status resume skills`) —
`accept.md` is missing, so the doc's own count contradicts its list and Slice C's new
command is invisible in the architecture tree. This is the enumeration-sync miss FR-C5
required syncing. **Fix (AGENT-FIXABLE):** add `accept.md` to the tree list.

### REV-002 (nit) — `skills/gogo-done/SKILL.md:213-214`
`find "$dst/before" -type f -name '*.mmd' -delete` + `rmdir … || true` leaves any
non-`.mmd` file in `before/` behind (rmdir then silently no-ops), diverging from the
old `rm -rf`. A real legacy `before/manifest.json` exists, but this is harmless — the
current assembly writes only `*.mmd` into `before/` plus a top-level `manifest.json`,
so all current-format entries clear cleanly; the stale legacy file sits on a terminal,
never-re-assembled entry. It is also the exact idiom FR-A1 prescribed. **Fix (optional,
AGENT-FIXABLE; may be wontfix):** drop the `-name '*.mmd'` filter on the before/ delete
for strict parity — `find "$dst/before" -type f -delete 2>/dev/null || true`.

## Not defects (verified, out of scope, or by design)
- The large `cli/internal/launch/launch.go` additions (`ActionResume`, `PhaseOpts`,
  `PhaseArgs`, `RunResult`, `RunPhase`, `ResumeIntent`, `sessionName` signature) belong
  to the unrelated `feature-cli-orchestrator` — excluded per the review brief.
- Hands-on `/gogo:done`-no-prompt and hands-on board-accept are the tester's (④)
  user-decision gates, not a review defect.
- ⏸ (U+23F8) is a single rune; `truncate` is rune-count based and lipgloss clamps to
  the card `Width`, so no overflow — cosmetic cell-width is a ④ concern, not a defect.
