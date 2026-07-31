# Review — round 3 · `notify-only-at-user-gates`

**Date:** 2026-07-31 · **Reviewer:** gogo-reviewer (fresh context) · **Contract:** `review/issues.json`
**Scope:** uncommitted diff on `main` — `hooks/notify.sh` (+581/-40), `hooks/hooks.json`,
`cli/notify_hook_test.go` (new), `cli/main.go`, `cli/version_test.go`, `README.md`,
`docs/flow.md`, `skills/gogo/SKILL.md`, `.claude-plugin/plugin.json`, `charts/`.
**Standards:** `code-review-standards.md` · `coding-rules.md` · `non-functional-requirements.md`
**Plan:** `plan.md` (FR4 as-built refinement) + `decisions.md` D4 (user chose **A — edge-latch**).
**Round-3 delta reviewed:** latch READ folded into `gogo_notify_decide` (5-field record),
unconditional `gogo_notify_remember` in `main`, brace-guarded quiet writes, seen-file moved
to `.gogo/resources/notify/gates`, manifest/comment truth fixes.

> Every probe that could reach `gogo_notify_send` ran with `GOGO_NTFY_TOPIC` unset
> (`env -u`) and/or `GOGO_NOTIFY_DRYRUN=1`. **No push was sent this round.**

---

## Part 1 — verification of round 2's six fixes (all 6 → `verified`)

Each was re-derived from the code and re-proven by mutation or live probe, not read off the
fix summary. **All six hold. Nothing is reopened.**

| id | sev | verification |
|---|---|---|
| **REV-010** | major | The acceptance criterion the finding set for itself is met. `notify.sh:296-299` calls `gogo_notify_remember` **unconditionally** for `class=gate` at `level=gates` (the DRYRUN guard survives only inside `gogo_notify_send`), fed by decide's 5th record field so there is no second scan; the knob header (`:27-28`) states the trade-off truthfully. Re-ran the prescribed mutation myself — no-op'ing the remember call site → **39/42, exit 1**, with `latch-wrote`, `latch-seen` and `latch-diff` all failing. Complementary mutation (forcing `gogo_notify_new_gates` down its degrade branch) → **40/42**: *both* halves are now pinned. The rebuilt e2e cases hand-write **no** state — `latch-wrote` (`:502-506`) asserts the *script* created the seen file with exactly the set it pinged about. Live lifecycle in a 2-gate temp tree: open → notify(2), repeat → silent, close one → seen pruned to the survivor, close both → file truncated to 0 bytes, reopen → notify. |
| **REV-011** | minor | Both writes use the brace idiom (`:176`, `:178`), mkdir quiet-guarded (`:174`). Proven where it was reported: with `.gogo/resources/notify` present but mode 555, HEAD writes **nothing** to stderr and exits 0, while an unbraced mutant prints `…/notify/gates: Permission denied`. The new `readonly-gogo` case (`:526-545`) is `id -u` guarded and restores 755 at `:544` **before** the `find -delete` at `:550`; because `gogo_selftest` runs as an `if` condition, errexit is suspended for its whole body, so no intermediate failure can jump past that chmod — **the cleanup cannot be stranded**. Caveat filed as **REV-016**, not a reopen. |
| **REV-012** | minor | The preferred remedy, verbatim. `git check-ignore -v .gogo/resources/notify/gates` → `.gitignore:17:.gogo/resources/`; a live hook probe created the file and `git status --porcelain .gogo/resources` stayed **empty**. The destination is the project's established runtime root (`docs/cli-contract.md:185` — CLI locks, session registry, logs already live there), so a wired repo gains no new *class* of artifact and needs no user `.gitignore` edit. All product references moved together (`notify.sh:20,153,159,173,503` · `README.md:676` · `docs/flow.md:213` · `plan.md:152` · both `.mmd`s · `manifest.json`); the only surviving `.gogo/.notify-gates` strings are in `decisions.md`'s historical D4 record, which carries an explicit round-3 "moved to" note. |
| **REV-013** | minor | Option **(b)**, as recommended. `gogo_notify_main` no longer calls `gogo_notify_gates` at all — it decodes decide's `gatelist` field and re-remembers it (`:288-299`), so the `state.md` scan runs **exactly once**. Re-measured here (31 feature folders, DRYRUN, wall clock): gate-class **457 / 530 / 538 ms** vs the round-2 baseline **816 ms**; silent-class 139-156 ms, unchanged (still no scan). `decide` stays side-effect-free — the latch READ only reads — which is what keeps `--selftest` able to drive it. |
| **REV-014** | minor | `manifest.json`'s `note` now walks the D4 stage (diff vs `.gogo/resources/notify/gates`, pruned set re-remembered by main after every gate-class event, missing/unreadable → fire-on-open-gate, never silence) and both `title`s name the latch and the path. Re-validated against `charts-manifest.schema.json`: required `slug`/`diagrams`, slug pattern, both `kind`s in enum, both `file`s match `\.mmd$` **and exist on disk**, no additional properties. The `.mmd` sources match the as-built shape (`flow.mmd:18-21`, `sequence.mmd:26-28`). |
| **REV-015** | nit | `notify.sh:183-191` now describes what the function does — the whole decision *including* the latch READ, with the WRITE attributed to `main` — and REV-013(b) made that **true**, not merely reworded. Dead local gone: `gogo_notify_new_gates:157-159` uses `$root` to build `$seen`. The decide-level gate cases do assert the shipped first-event verdict: instrumenting the suite shows only `quiet/`, `gate-uat/` and `gate-latch/` ever gain a latch file, and all three writes happen **after** the last `expect`. That ordering dependence is the residue — **REV-019**, a nit. |

**Gates re-checked this round:** `bash hooks/notify.sh --selftest` → **42/42, exit 0** under
bash 5.3 **and** `/bin/bash` 3.2 · `gofmt -l .` clean · `go vet ./...` clean ·
`go test -race` green for `TestNotifyHookSelftest`, `TestNotifyHookGateStatusesMatchContract`,
`TestVersionMirrorsPlugin` · version bumped to **0.31.0** in `plugin.json` **and** `cli/main.go`.

---

## Part 2 — fresh findings (round 3)

Four, none of them blocking: two minors and two nits. No blocker, no major.

### REV-016 · **minor** · P2 · new — the REV-011 guard is vacuous for the write it protects
> `code-review-standards.md` #11/#12 — a test whose comment claims coverage its assertions don't provide.

`readonly-gogo` (`notify.sh:524-545`) chmods the whole `.gogo` to 555 and comments
*"555 keeps the scan readable; the remember write fails"*. The write does not fail —
**it never runs**: `gogo_notify_remember` now opens with

```bash
  mkdir -p "$dir" 2>/dev/null || return 0     # $dir = .gogo/resources/notify
```

and under a 555 `.gogo` that `mkdir` fails, so the function returns before either
redirection. Mutation, both directions:

| mutation | result |
|---|---|
| restore the pre-fix **unbraced** writes (`printf … > "$dir/gates" 2>/dev/null \|\| true`) | **42/42, exit 0** — REV-011's exact defect is undetectable again |
| drop only the `mkdir`'s `2>/dev/null` | **41/42, exit 1** — this is all the fixture actually pins |

The uncovered path is real: with `.gogo/resources/notify` present but unwritable (a
root-owned `gates` from a sudo'd run, a read-only mount), HEAD is silent while the unbraced
mutant prints `<path>/gates: Permission denied` on **every** gate-class event — reproduced
live. Deliberately **not** rated major, unlike REV-010: the shipped code is correct and the
uncovered blast radius is stderr noise on an exotic permissions path. It is still a guard
that cannot bite.

**Fix (AGENT-FIXABLE, two lines):** make the fixture fail at the *write* — pre-create
`$tmp/rogogo/.gogo/resources/notify` and `chmod 555` **that**, leaving `.gogo` writable;
keep the `id -u` guard and the 755 restore. Optionally keep the 555-`.gogo` variant as a
second case so both guards are pinned. **Acceptance:** with the fixture changed, the
unbraced-write mutation must drop below 42 and exit 1.

### REV-017 · **minor** · P3 · new — `plan.md` FR6 still says "Nothing is written to disk"
`plan.md:170` (FR6, *The hook is observable*) asserts *"…Nothing is written to disk."* As
built, `notify.sh:296-299` writes `.gogo/resources/notify/gates` on **every** gate-class
invocation and — since this round's REV-010 fix — under `GOGO_NOTIFY_DRYRUN=1` too. The
write is sanctioned (D4 resolution explicitly accepts "one small mutable state file under
`.gogo/`"), it is confined and gitignored; only the plan's statelessness claim is stale.
The same file already shows the convention: **FR4 carries an as-built note** (`plan.md:149-154`).
Two neighbours were checked and are still true — "Out of scope: a notification history / log
file" (`:354`; the latch is state, not a log) and the pre-implementation Approach sketch
(`:234-246`). `code-review-standards.md` #1.

**Fix (AGENT-FIXABLE, one sentence):** annotate FR6 the way FR4 is annotated, naming the
latch path and the DRYRUN semantics; leave the observability claim itself intact.

### REV-018 · **nit** · P3 · new — the gatelist round-trip joins on `|`
`notify.sh:234` joins the gate set with `tr '\n' '|'` and `:298` splits it back with
`tr '|' '\n'`; the delimiter is unescaped and can occur in a directory name. Verified with
`.gogo/work/feature-a|b/` at `awaiting-uat`: the seen file is written as **two** lines
(`feature-a` / `b · awaiting-uat`), neither of which can match the single scan line, so
`grep -Fxv -f` calls the gate new **on every event** — three consecutive `agent_completed`
runs give notify, notify, notify: REV-001's spam, permanently, for that item. Kept at nit —
reachability is low (slugs are kebab-case by `gogo-plan` prose, admittedly unenforced) and
the failure degrades **noisy**, the direction the NFRs prefer.

The `-` placeholder was checked separately and is **sound**: every real line contains
` · `, so a legitimate gatelist can never be the literal `-`, and the placeholder is
genuinely required — tab is IFS whitespace, so an empty middle field collapses and shifts
`msg` left (reproduced).

**Fix (AGENT-FIXABLE, two characters):** join/split on ASCII unit separator
(`tr '\n' '\037'` / `tr '\037' '\n'`), re-run the selftest on bash 3.2 and 5.3, and
optionally pin the round trip with one delimiter-bearing fixture.

### REV-019 · **nit** · P3 · new — e2e cases pollute two shared fixture roots
Now that the latch READ lives in `decide`, a root that has been through a whole-script run
behaves differently for every later `expect` on it. Instrumenting the suite before cleanup
shows three roots gain `.gogo/resources/notify/gates`: `gate-latch/` (by design) plus
`quiet/` and `gate-uat/` — which back **ten** and **twelve** `expect` cases respectively.
The suite is correct **today** only because the two e2e cases (`:468`, `:478`) run last and
because the surviving `gate-uat` silent expectations short-circuit before the gate scan
(checked case by case). It is nonetheless the fixture-reuse shape `code-review-standards.md`
#11(c) names: add a gate-class `expect` on `gate-uat` below `:468` and it silently becomes a
latch test.

**Fix (AGENT-FIXABLE, two lines):** give the e2e cases their own roots (`e2e-gate`,
`e2e-quiet`), as the latch block already does, and state the invariant in the block comment.

---

## Also checked, deliberately **not** filed

Recorded so the next round doesn't re-derive them:

- **`main` re-parses `notification_type` (`:286`) purely for the DEBUG line** — one extra
  `jq` spawn (**measured 15.9 ms**, ~11 % of the 140 ms silent path) on every invocation,
  even with `GOGO_NOTIFY_DEBUG` unset. Same *class* as REV-013 but two orders of magnitude
  smaller, and pre-existing since round 1. One-line fix if ever touched: move the extraction
  inside the debug block.
- **`[ "$(gogo_notify_level)" = "gates" ]` in the remember guard (`:296`) is unreachable-false** —
  `class=gate` already implies `level=gates` (off/all return early with their own class).
  Harmless belt-and-braces.
- **`-H "Title: gogo • ${PWD##*/}"`** — a newline in the *repo directory* name would be the
  only header-injection surface; unchanged from pre-feature code, and curl rejects CR/LF in
  headers.
- **Concurrent writers to the single latch file** — a torn write can only duplicate a ping,
  never drop one; round 2 already probed 8 concurrent invocations (1 notify + 7 silent).
- **Write-scope, secrets, injection** — writes stay under `${CLAUDE_PROJECT_DIR}/.gogo/` and
  only when `.gogo/` already exists (`:172`); the DEBUG line prints no message text and never
  the ntfy topic; the seen file holds folder names and statuses only.
- **`docs/architecture.md:167`** lists `resources/`'s contents without `notify/` — but it also
  omits the far larger `resources/cli/`, and `plan.md`'s checklist item 7 routes
  `architecture.md` to the phase-⑤ knowledge reconcile.

---

## Verdict

| | count |
|---|---|
| verified this round | 6 (REV-010 … REV-015) |
| verified overall | 15 of 19 |
| open blockers / majors | **0** |
| open minors / nits | 2 minors (REV-016, REV-017) · 2 nits (REV-018, REV-019) |
| needs-user-decision | none — all four are **AGENT-FIXABLE** |

All six round-2 fixes are real and independently re-proven; the D4 latch is now pinned on
both halves, the double scan is gone, the state file is ignored, and the comments tell the
truth. The four new findings are batched minors/nits: none changes shipped behaviour, none
needs a user decision.

**Verdict: APPROVE** (no open blockers or majors — batch REV-016…REV-019 into the next
implement pass or carry them to ④ test).
