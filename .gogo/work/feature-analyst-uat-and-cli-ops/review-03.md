# Review round 3 — feature `analyst-uat-and-cli-ops`

**Scope:** Stage C (CLI ops, 0.11.0) delta, fresh eyes, on top of the
Stages-A+B-approved working tree. Reviewed against plan.md FR6-FR9, decisions.md
D2/D3/D4 + the REV-004 mid-UAT note, and `.gogo/knowledge/{code-review-standards,
coding-rules,non-functional-requirements}.md`. Prior findings REV-001..008
spot-checked and hold (all `verified`). Stage-A/B prose untouched beyond the
declared doc sweep; embedded viewer assets unchanged (no viewer work in C).

**Verdict: APPROVE** — no open blockers/majors. 4 new findings, all **nit** (P3),
all AGENT-FIXABLE hardening/polish in the trash + delete-confirm paths. Gates
green: `gofmt -l` clean · `go vet` clean · `go build` clean · `go test -race
-count=1 ./...` all packages green · `gogo --version` = `0.11.0` = plugin.json.

| id | sev | prio | status | one-line |
|---|---|---|---|---|
| REV-001 | major | P1 | verified | README "nine"→"ten" knowledge files (Stage A) |
| REV-002 | nit | P3 | verified | orchestrator-first note on implement/report cmds (Stage A) |
| REV-003 | major | P1 | verified | commands/report.md now sets `awaiting-uat` (Stage B) |
| REV-004 | major | P2 | verified | mid-UAT lock = `waiting-for-user`; CLI badge agrees (Stage B) |
| REV-005 | minor | P2 | verified | `plan-accepted` single-owner restored (Stage B) |
| REV-006 | minor | P2 | verified | architecture.md flow one-liner shows UAT gate (Stage B) |
| REV-007 | minor | P2 | verified | `/gogo:skills` exempts `## Custom` (Stage B) |
| REV-008 | nit | P3 | verified | report.md orchestrator note reworded (Stage B) |
| REV-009 | nit | P3 | new | EXDEV copyTree follows symlinks (os.Stat) — deref + possible cycle recursion |
| REV-010 | nit | P3 | new | MoveToTrash has no package-level work-folder guard (changelog defense-in-depth is UI-only) |
| REV-011 | nit | P3 | new | uniqueDest `-N` suffix leaks into parsed slug → wrong restore name on same-second re-delete |
| REV-012 | nit | P3 | new | Esc-cancel of a delete confirm wipes the ready-ship selection (inconsistent with the Cancel button) |

## New findings (round 3)

### REV-009 (nit) — EXDEV fallback dereferences symlinks / can recurse a cycle
`cli/internal/trash/trash.go:257-278` `copyTree` (taken only when `os.Rename`
returns EXDEV — trash on a different filesystem than work) uses `os.Stat`, which
follows symlinks: a symlink is copied dereferenced (not byte-faithful), and a
cyclic symlink drives unbounded recursion → stack overflow. Reachability is very
low (cross-device trash **and** a symlink in a gogo-authored work dir, which never
has one). The same-device `os.Rename` happy path is unaffected, and the fallback's
data-safety is otherwise intact (source removed only after a successful copy;
copy error removes the partial DEST, never the source). **Fix:** `os.Lstat` +
explicit `ModeSymlink` handling (recreate or skip); optional depth guard.

### REV-010 (nit) — changelog-not-deletable is enforced UI-only, no package guard
The append-only-changelog invariant (D3/FR6) lives in one spot:
`cli/internal/tui/delete.go:19-22` bounces `ColChangelog`. `MoveToTrash`
(`trash.go:62`) validates nothing and would trash any path it is handed. It is
safe **by construction** today — a single guarded caller (grep-confirmed
`delete.go:58`), and changelog-archive dirs are never board cards (a shipped
card's `Dir` is its `.gogo/work/feature-*` folder, never the `.gogo/changelog/`
entry). The gap is belt-and-suspenders only. **Fix:** in `MoveToTrash` reject a
`featureDir` whose base lacks the `feature-` prefix and/or that does not resolve
under `.gogo/work/`, so the invariant holds at the package boundary.

### REV-011 (nit) — uniqueDest suffix corrupts the restored folder name
`uniqueDest` (`trash.go:214-226`) appends `-2`/`-3` on a same-second collision, so
the base becomes `…Z-my-slug-2`; `parseBase` splits on the first `-`, so the slug
parses as `my-slug-2` and `Restore` recreates `feature-my-slug-2`, not
`feature-my-slug`. Needs two deletes of the same slug within one UTC second —
unreachable at board speed, so a latent bug in defensive code, no data loss.
**Fix:** carry the true slug on the Entry independent of the base name, or widen
`tsLayout` to sub-second precision, or move the counter before the first `-`.

### REV-012 (nit) — delete-cancel via Esc wipes the ready-ship selection
Two cancel routes differ: Esc → `cancelForm` (`update.go:363-373`) resets
`m.selected`; the Cancel **button** → `finishDelete` (`delete.go:47-57`) leaves it
intact. So opening `x` on any card and pressing Esc silently clears a pending
space-selection of ready cards. `cancelForm`'s "clear so a stale target can't be
re-shipped" rationale is about launch forms; for a delete the ship-selection is
unrelated. Cosmetic, no state-machine impact. **Fix:** guard the `m.selected`
reset on `m.pendingDelete == nil` so a delete-abort keeps the selection.

## Priority-dimension verdicts (as asked)

- **(a) Trash data-safety incl. the EXDEV fallback — SAFE.** `MoveToTrash` never
  removes the source before the destination lands: same-device is an atomic
  `os.Rename`; the EXDEV path does `copyTree` → then `os.RemoveAll(src)`, and a
  copy error removes the partial DEST and surfaces the error with the source
  intact (`moveDir:232-243`). `copyFile` checks both the `io.Copy` and the
  `out.Close()` errors, so a short/failed copy is caught before any removal.
  Missing/empty target → clean error, no move. Restore refuses a name collision
  and a missing entry (tested). The one blemish is symlink handling in the
  rare fallback (REV-009), a nit.
- **(b) Changelog-deletion defense depth — INVARIANT HOLDS, guard is UI-only.**
  The changelog archive is unreachable by the destructive path by construction
  (never a card, never handed to `MoveToTrash`); the sole enforcement point is the
  `ColChangelog` UI bounce (tested: `TestDeleteChangelogBounces`). No package-level
  belt-and-suspenders (REV-010, nit).
- **(c) Env-override tri-state — CORRECT.** `PermissionMode` uses `os.LookupEnv`
  (not `Getenv`), so **unset** → `auto`, **set-empty** → omit the flag entirely,
  **set-nonempty** → verbatim; the flag+value are always two separate argv
  elements (`--permission-mode`, `<mode>`), spliced into both the tmux `new-session`
  argv and the `claude -p` fallback argv — never a shell string. Pinned by
  `TestPermissionArgsMatrix` (incl. the unset case via a custom set/unset helper),
  `TestClaudePrintArgs`, `TestTmuxNewSessionArgs`. The huh confirm shows the
  effective mode via `PermissionSummary` (`move.go:104`).
- **(d) Badge priority — CORRECT.** `model.go` `badge()` precedence is
  waiting-for-user > running > awaiting-uat > phase(+round), matching the REV-004
  design (waiting-for-user wins mid-UAT) and D4 (awaiting-uat badge on a still-ready
  card). Pinned by `TestBadgeAwaitingUAT` + `TestAwaitingUATBadgeStyled`.

## What else was checked and is clean

- **Delete confirm flow.** `MoveToTrash` has exactly one call site (`delete.go:58`),
  reachable only via `x` → `deleteFocused` (changelog-guarded) → confirm form →
  `huh.StateCompleted` with `pendingDelete!=nil` → `finishDelete`, which only trashes
  when `binding.confirm` is true. The confirm defaults to **Cancel** (safe Enter);
  Esc/StateAborted → `cancelForm`; all paths clear `pendingDelete`/`form`/`mode`
  (TEST-001/002 lessons observed). Tested end-to-end (`TestDeleteToTrashFlow`,
  `TestDeleteCancelKeepsCard`).
- **Injection safety.** The whole `trash` package has **no** `os/exec`/shell import
  (grep-confirmed) — a hostile slug can never execute; paths are built with
  `filepath.Join`/`filepath.Base` (separators stripped). Hostile probe: a fixture
  dir `feature-a;$(touch PWNED)` and `gogo trash restore` with `;$(…)` /
  `../../etc` inputs created **no** artifacts and errored cleanly. Peek/attach/launch
  pass session names (from tmux itself) and slugs as single argv elements, no shell.
- **Peek (FR7).** Async off-goroutine capture (no sync terminal queries — TEST-003),
  applied only if the viewer is still the requesting peek (slug match); `r`
  re-captures; live session → `capture-pane -p -S -300`, else newest matching
  background `-p` log tail, else a no-session hint; `q`/`esc` return to the board
  and clear peek state; `a` escalates to a real attach. Tested across all branches.
- **`l` keymap reassignment.** `l` = peek on the board; column-right is the arrow
  only (`h`/`left` stays left-alias, `j`/`k`/arrows intact); drill still uses
  `l`/right/enter/v to open. Board/drill/viewer help lines and the README key legend
  all updated to `l peek · x delete→trash`. No other binding regressed.
- **Contract + docs.** `AwaitingUAT()`/`WaitingForUser()` added; `uat.md` listed in
  the drill file list only when present (`TestUATArtifactInFileList`); the 3 new UAT
  events and any unknown event parse leniently (unknown fields ignored, bad line
  skipped — `TestUATEventsParseLeniently`); `docs/cli-contract.md` 0.11.0 block
  covers trash (the one write outside `.gogo/resources/`), the awaiting-uat badge,
  and the permission flag (incl. the empty-string-omits note) with the uat events'
  owners in the single-owner table; README covers trash/peek/permission/badge.
- **Version + scope.** `cli/main.go` `Version = "0.11.0"` = plugin.json 0.11.0 =
  binary `--version`; `main.go` dispatches `trash`. No changes to the embedded
  viewer/pages/mermaid assets (git-clean). `gogo trash` on a repo with no trash dir
  returns the empty listing (exit 0), not an error.
