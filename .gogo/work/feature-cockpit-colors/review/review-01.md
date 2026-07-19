# Review 01 — `cockpit-colors` (→ 0.22.0)

Fresh-eyes review (phase ③). Reviewed the full diff against `plan.md` (FR1–FR5) +
`decisions.md` (D1–D5 = A) and the knowledge standards. I did not write this code.

## Gates (verified locally, `cd cli`)
- `gofmt -l .` — clean
- `go vet ./...` — clean
- `go test -race ./...` — all packages green (incl. `TestCLICommandEnumerationInSync`
  and `TestSkillsBashNoUnsafeRm`, force-run with `-count=1`)
- Version bumped: `.claude-plugin/plugin.json` = `0.22.0` and `cli/main.go Version =
  "0.22.0"` (consistent).

## What I verified (the scrutiny list)

1. **FR1 home-dir fix — correct.** `chooseBoard(root, rootFound, dataHome, …)` guards
   with `rootFound && !sameDir(filepath.Join(root, ".gogo"), dataHome)`; `runBoard`
   passes `projects.Home()` (which honours `$GOGO_DATA_HOME`). From `~` or any child of
   `~` without its own `.gogo/`, `FindRoot` returns `~`, `~/.gogo == dataHome` → guard
   fires → falls through to the global path. A real repo elsewhere (`root/.gogo !=
   dataHome`) still resolves `single`. A custom `$GOGO_DATA_HOME` correctly makes a real
   `~/.gogo` repo open as `single`. The pathological "real repo AT the data home" is
   documented (data home wins). The pure `chooseBoard` test drives every branch and is
   meaningful. Symlink/trailing-slash edges are handled by `filepath.Clean` for the
   common case; unresolved-symlink `$HOME` is the documented degenerate edge only.

2. **Never-blank / back-compat — holds.** `colorFor(hex, idx)` returns AdaptiveColor on a
   swatch match, direct `lipgloss.Color` for an arbitrary hex, and `ColorForIndex(idx)`
   (a swatch, adaptive) for blank — never nil/grey. `sourceColorMap`/`projectColorMap`
   resolve every entity (own color else `ColorForIndex(position)`); unknown labels fall
   back through `stableIndex(name)`. `AssignColor` is deterministic, skips taken
   (case-insensitively), and wraps to a real swatch when all are taken. Re-adding a
   source preserves its color (`existingSourceColor`); re-adding a project is guarded
   (never clobbers). `ColorForIndex` handles negatives; `Swatches` is a fixed 8 and
   guarded by a shape test, so no empty-palette panic in practice.

3. **Single-repo byte-for-byte (D3) — preserved.** `changelogRowSingle` is the old
   `changelogRow` body verbatim (date/dateW hoisted to the caller); a `Source==""` row
   takes that path → no source dot. `TestChangelogSourceDot` pins both the project-board
   leading source dot + relocated trailing session dot and the single-repo no-dot case.

4. **Config color edit (TEST-001) — correct.** Both the project `c` form
   (`*formBinding.projColor`) and the source `e` form (`.srcColor`) bind heap-stable
   pointers; `finishProjectColorForm`/`finishSourceForm` read them, accept a hex OR a
   swatch name (`SwatchByName`), blank-on-add auto-assigns, persist to `config.json`
   (a `~/.gogo/` write only), and re-tint live via `refreshProject`. Save errors surface
   to `m.status`. Routing in `updateForm`/`cancelForm`/`formPreservesSelection` includes
   `pendingProject` and is mutually exclusive.

5. **Two-dot combo (D5) — correct.** `originDots` renders `●P ●S` when a project color is
   present, a single dot otherwise; `projectOriginDots` reuses it (one project dot for a
   sourceless project). Focused rows render the dots plain (focus fill owns fg/bg), no
   background holes. Width math in the project-board `changelogRow` balances to exactly
   `width`.

6. **Palette source-of-truth — clean.** `projects/palette.go` is pure strings (no
   lipgloss); the tui side resolves via `colorFor`. No import cycle, no drift. Alert-red
   is deliberately excluded.

7. **Standards — met.** No new command verb → enum-sync untouched (contract note is
   additive); `Project.Color` is additive `omitempty`, schema stays `1`; writes stay
   under `~/.gogo/`; no secrets/injection (colors are JSON strings, never reach a shell);
   swallowed `List`/`Load` errors degrade to a safe fallback, consistent with the CLI's
   lenient-reader rule.

## Findings

| id | sev | pri | status | title |
|----|-----|-----|--------|-------|
| REV-001 | nit | P3 | new | "gather taken colors" logic duplicated across three sites |
| REV-002 | nit | P3 | new | Legacy colorless source's fallback swatch shifts with slice position (cosmetic) |
| REV-003 | nit | P3 | new | Config project-switcher rows add the two-dot cue without width truncation |

All three are nit-level, non-blocking, and each is either intended-by-plan or optional
polish. No blockers, no majors, no minors. Tests cover every plan-listed family.

---

**Verdict: APPROVE**
