# Review — round 2 · `notify-only-at-user-gates`

**Date:** 2026-07-31 · **Reviewer:** gogo-reviewer (fresh context) · **Contract:** `review/issues.json`
**Scope:** uncommitted diff on `main` — `hooks/notify.sh` (+518), `hooks/hooks.json`,
`cli/notify_hook_test.go` (new), `cli/main.go`, `cli/version_test.go`, `README.md`,
`docs/flow.md`, `skills/gogo/SKILL.md`, `.claude-plugin/plugin.json`, `charts/`.
**Standards:** `code-review-standards.md` · `coding-rules.md` · `non-functional-requirements.md`
**Plan:** `plan.md` (FR4 as-built refinement) + `decisions.md` D4 (user chose **A — edge-latch**).

---

## Part 1 — verification of round 1 (all 9 → `verified`)

Every round-1 fix was re-derived from the code and re-proven independently, not read off
the fix summary. **All nine hold.** Nothing is reopened.

| id | sev | verification |
|---|---|---|
| **REV-001** | major | Reproduced the D4 latch in a temp tree with one parked `awaiting-plan-acceptance` gate: 4 consecutive `agent_completed` hand-offs give **notify, silent, silent, silent** (pre-D4 shape gives 4× notify). Also probed the edges the fix depends on — closing a gate prunes the seen file to empty and re-opening it **re-pings**; empty / truncated / unreadable seen file degrades to **notify**, never to silence; 8 concurrent invocations against a cold latch produce exactly **1 notify + 7 silent** (a race can only duplicate a ping, never drop one); `grep -Fxv -f` handles the UTF-8 `·` identically under `LC_ALL=C`, `POSIX` and `en_US.UTF-8` on BSD grep. → `verified` (but see **REV-010** — the write half has no test that bites). |
| **REV-002** | major | `notify.sh:246-250` passes `msg`/`title` as **argv** into a quoted heredoc (`item 1 of argv`), so no payload byte reaches AppleScript source. Re-ran round 1's exploit in both shapes (quote-terminated `& (do shell script "touch …") & "` and backslash-quote `y\" & … --`): both exit 0 with `channels=banner` and **neither probe file is created**. Structurally gone, not escaped. |
| **REV-003** | major | Guard gained step 2b (`notify_hook_test.go:79-106`) with a vacuity check at :99. Re-ran round 1's exact mutation (`awaiting-security-signoff` added to the contract only): the guard now **FAILs**, naming the drifted status. Subtractive drift still caught by the step-4 set equality — both directions bite. contract.go restored, suite green. |
| **REV-004** | major | Re-ran both mutations against the *current* 41-case script: removing the first-token split → `FAIL completed-note`, **40/41**, exit 1; deleting the `incomment {…}` awk rule → **35/41**, exit 1 (feature-b's column-0 example leaks into `empty-payload`, `garbage-payload`, `e2e-silent`). Both mechanisms are now load-bearing **and** pinned. |
| **REV-005** | minor | `notify.sh:104-105` mirrors `contract.stateLine`. Probed end to end: `  - **status:** awaiting-uat`, `-  **status:** awaiting-uat` and `   -   **status:**   waiting-for-user  ` all yield `verdict=notify`. |
| **REV-006** | minor | `notify.sh:235` uses `--data-raw`. Re-ran the local-listener probe: the request body is the literal `@/tmp/gogo-curl-probe.txt` and the file's contents appear **0** times in the captured request. |
| **REV-007** | minor | Two whole-script e2e cases over stdin (`notify.sh:462-484`). Proved they bite: forcing `gogo_notify_main` to `exit 7` gives `FAIL e2e-notify: exit=7` + `FAIL e2e-silent: exit=7`, **39/41**, exit 1. (`set -e` is suspended inside `gogo_selftest` because it runs as an `if` condition — that is what keeps the rc assertions reachable.) Sends nothing. |
| **REV-008** | minor | `SKILL.md:405-406` mirrors the flow.md clause and cross-links it; `plan.md:325-327` records `skills/` joining the sweep. Re-ran the full sweep myself — the only remaining hits are the three the plan already defers to phase ⑤. |
| **REV-009** | nit | `notify.sh:131` globs `feature-*/`. The `scratchpad` probe now yields `verdict=silent gates=0`; `empty-root` still green. |

**Gates re-checked:** `bash hooks/notify.sh --selftest` → **41/41, exit 0** under bash 5.3
*and* `/bin/bash` 3.2 · `gofmt -l .` clean · `go vet ./...` clean · `go test ./...` green.

---

## Part 2 — fresh findings (round 2)

### REV-010 · **major** · P1 · new — the latch's WRITE half is unpinned
> `code-review-standards.md` #11(b), applied to the wiring that carries the whole D4 fix.

Deleting the three-line block at `hooks/notify.sh:294-296`

```bash
    if [ "${GOGO_NOTIFY_DRYRUN:-}" != "1" ]; then
      gogo_notify_remember "$root" "$current"
    fi
```

leaves `--selftest` at **41/41 passed, exit 0** and the whole Go suite green — while the
mutant reproduces **REV-001 verbatim**: with one parked gate, four `agent_completed`
hand-offs give `notify, notify, notify, notify` (HEAD gives `notify, silent, silent,
silent`). Without the write the seen file is never created, so
`gogo_notify_new_gates`'s `[ ! -s "$seen" ]` branch calls every open gate "new", forever.

**Root cause:** the DRYRUN seam added for REV-007 is the very flag that disables the
write, and **all four** whole-script invocations (`:466`, `:476`, `:491`, `:501`) set
`GOGO_NOTIFY_DRYRUN=1`, so the production write path is structurally unreachable from the
suite. `latch-remember` (`:509-515`) calls `gogo_notify_remember` as a **unit**, so it
cannot see a missing call site; `latch-seen` passes only because the test hand-writes
`.notify-gates` itself at `:499`. Cross-check: mutating `gogo_notify_new_gates` to return
everything **does** bite (39/41) — the *read* half is pinned, only the *write* half is not.

**Fix (AGENT-FIXABLE).** Make DRYRUN mean "send nothing", not "remember nothing" — the
write already targets a per-case temp `CLAUDE_PROJECT_DIR`, so it is repeatable. Drop the
DRYRUN guard around `gogo_notify_remember` (keep it inside `gogo_notify_send`), then add
one e2e case in a fresh latch root that runs the script **twice with no hand-written seen
file** and asserts notify→silent, plus an assertion on the seen file's contents after run 1.
**Acceptance criterion: re-run the mutation above and confirm the suite drops below 41 and
exits 1.**

### REV-011 · **minor** · P2 · new — read-only `.gogo/` leaks a raw bash error to stderr
`gogo_notify_remember` writes `printf … > "$root/.gogo/.notify-gates" 2>/dev/null || true`.
Bash applies redirections left to right, so the `>` failure is reported **before**
`2>/dev/null` is installed. Verified with `chmod 555 .gogo`: exit 0, but

```
hooks/notify.sh: line 169: /…/.gogo/.notify-gates: Permission denied
```

on **every** gate-class event, with no `GOGO_NOTIFY_DEBUG` set. Contradicts
`coding-rules.md` ("Bash hooks: … silent no-op"), the NFR ("side-effect-light"), FR6 (one
stderr line *only* under debug) and the function's own comment. The file already knows the
right idiom — `:257` uses `if { : >/dev/tty; } 2>/dev/null` with a comment explaining
exactly this. **Untested:** the only permissions fixture chmods `.gogo/work` (`:368`),
never `.gogo`. **Fix:** brace-wrap both writes; add a chmod-555-`.gogo` selftest root
asserting stderr carries no `Permission denied` and exit is still 0.

### REV-012 · **minor** · P2 · new — `.gogo/.notify-gates` is unignored and unenumerated
`git check-ignore -v .gogo/.notify-gates` → rc=1 (not ignored), and `.gogo/` is a **tracked**
directory here (1014 tracked files), so the latch surfaces as untracked in `git status` and
is one `git add -A` from entering shared history — where a per-machine latch would suppress
another developer's first gate ping. This is not confined to gogo's own repo: the hook writes
into **every** wired project's `.gogo/`, and the NFR forbids the obvious remedy there
(`non-functional-requirements.md:26` "Don't auto-edit `.gitignore`; print guidance instead").
The file is also missing from the `.gogo/` tree in `docs/architecture.md`'s "Project side"
block (~:168), which is **not** among the phase-⑤ reconcile targets `plan.md:319-321` lists.
**Fix (preferred):** move it to `.gogo/resources/notify/gates` — already covered verbatim by
the `.gogo/resources/` ignore line, already the sanctioned regenerable-runtime root
(`docs/cli-contract.md:185`), and it needs **no** user `.gitignore` edit at all.

### REV-013 · **minor** · P3 · new — the gate scan runs twice per gate-class event
`gogo_notify_decide` scans every `state.md` (`:207`), then `gogo_notify_main` discards the
result and scans again for the latch (`:282`). **Measured** in this repo (31 feature dirs,
5 runs, DRYRUN): gate-class **816 ms** vs silent-class (no scan) **74 ms** → ~742 ms of scan,
of which **~371 ms is a verbatim repeat**. One `awk` + one `grep` subprocess per feature
folder per pass, so it scales linearly (a 100-item tree ≈ 2.5 s to decide whether to buzz).
**Fix:** move the latch **read** into `decide` (reading the seen file is still
side-effect-free) and leave only the one-line write in `main` — this also makes the ~20
gate-class `expect` cases assert the *shipped* verdict (see REV-015). Record before/after ms.

### REV-014 · **minor** · P3 · new — `charts/manifest.json` predates the latch
`flow.mmd:18-21` and `sequence.mmd:26-28` draw the D4 latch (both stamped 18:00), but
`manifest.json` was last written at 15:52 and its prose never caught up: the `note` walks
"level switch → classifier → `gogo_notify_gates()` scan" and stops — no latch, no
`.notify-gates`, no D4 — and both diagram `title` strings omit it. Those titles are the
typed charts-manifest contract and the captions `/gogo:view` and the phase-⑤ report bundle
render, so the drift is user-visible (`code-review-standards.md` #1). **Fix:** docs only —
rewrite `note` and both `title`s to name the latch stage; re-validate against
`charts-manifest.schema.json`.

### REV-015 · **nit** · P3 · new — stale contract comment + a dead local
(1) `notify.sh:176` still calls `gogo_notify_decide` "the whole decision" — false since D4
for the one class the feature is about; `main` recomputes `verdict`/`gates`/`msg` at
`:280-297`. Visible consequence: ~20 `expect` cases under "FR4 — agent_completed is
gate-conditional" assert a verdict the hook does **not** ship once the gate is latched
(`completed-uat` asserts notify; production is silent on the second event). Harmless today,
but it is the same blind spot that let REV-010 through. (2) `notify.sh:155` declares
`local root="${1:-}"` in `gogo_notify_new_gates` and never reads it (shellcheck SC2034).

---

## Also checked — no finding

- **D4 latch semantics.** `grep -Fxv -f` is correct on BSD *and* GNU grep here; the `-s`
  guard closes the classic empty-pattern-file trap; the UTF-8 `·` matches byte-wise in every
  locale tested; a gate that closes and reopens **does** re-ping (prune → empty file →
  next open is "new"); a DRYRUN run can neither poison nor consume latch state.
- **Concurrency.** 8 simultaneous invocations against a cold latch → 1 notify, 7 silent.
  A truncated mid-write read degrades to an **extra** ping, never a lost one — the direction
  D4 mandates.
- **decide-vs-main.** No *external* consumer can observe the split (`decide` is script-local,
  nothing sources the file); the exposure is documentation and test framing only → REV-015.
- **Docs truth.** README ~:666-680, `docs/flow.md` decision-gate step 3, `SKILL.md:405`,
  `hooks/hooks.json` description all state the latch behaviour correctly. The abbreviated
  SKILL.md clause omits the latch, but a transition *into* `waiting-for-user` is by
  construction a newly-opened gate and it cross-links to flow.md — true as written.
- **Enumeration sweep** over `README.md docs/ hooks/ skills/ .gogo/knowledge/ agents/
  templates/`: clean apart from the three hits `plan.md` already defers to phase ⑤ (plus the
  architecture.md tree, folded into REV-012).
- **Version.** `0.31.0` in `plugin.json` and `cli/main.go`, mirrored by `TestVersionMirrorsPlugin`.
- **Security / logging.** The debug trace prints type·level·class·verdict·gates·channels —
  no message body, no `GOGO_NTFY_TOPIC`. Gate messages carry feature slugs only; strictly
  *narrower* than the pre-change behaviour of pushing every event's verbatim text.
- **Write scope.** All writes stay under `.gogo/` (REV-012 is about *which* path, not scope).

---

## Verdict

Round-1 debt is fully discharged — **all 9 findings verified**, each re-proven by mutation
or live probe. The D4 edge-latch is behaviourally correct in every edge I could construct.
But its **write half ships with no test that bites**, and deleting it silently restores the
exact major (REV-001) this round is meant to close, with a 41/41 green suite. That must not
ship unpinned.

**1 open major (REV-010), 4 minors, 1 nit.** Batch REV-011…REV-015 with the REV-010 fix.

**CHANGES**
