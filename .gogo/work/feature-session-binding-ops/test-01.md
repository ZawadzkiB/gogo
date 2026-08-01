# Test round 01 — `session-binding-ops`

**Scope:** the uncommitted working tree (13 modified + 4 new files under `cli/`), against
`plan.md`'s phase-④ row (as adapted by the orchestrator's brief, honoring the FR4 REV-003
correction: a shipped card offers `[K]` only, and `R` on a terminal card refuses naming the
real move). Review rounds 1-2 are clean (`review-02.md` **APPROVE**); round-3 implement fixed
the two open nits (REV-006, REV-007) that round 2 left open — confirmed by reading the
current code (`view.go:892` already calls `orchestrator.PlannableStatus`, and the stray
`(REV-004)` tag at `session_ops.go:224` is gone).

**Host:** tmux 3.7b, `claude` on PATH (`/Users/bartlomiej.zawadzki/.local/bin/claude`).

---

## Level 1 — suites (all green)

| Check | Command | Result |
|---|---|---|
| build | `cd cli && go build -o /tmp/gogo-test-bin .` | clean, exit 0 |
| format | `gofmt -l .` | no output — nothing unformatted |
| vet | `go vet ./...` | clean |
| unit + race | `go test -race -count=1 ./...` | **all 13 packages ok** (`cli`, `config`, `contract`, `diagram`, `diagram/mermaidascii`, `launch`, `orchestrator`, `pages`, `plans`, `projects`, `trash`, `tui`; `textfmt` has no test files) |

---

## Level 2 — hands-on, real tmux on this host

**Safety.** Baseline host sessions before any fixture work: `gogo-accept-catalogue-vectors-and-filters`,
`gogo-accept-session-binding-ops` (this agent's own host pane), `gogo-author-for-gogo-project-lets-add-few-new-tasks-to-plan`,
`gogo-plan-catalogue-side-of-the-matching-engine---normalise-store-embed-hard-filter`. All
four were **read-only observed**, never sent keys, attached, renamed, or killed, and all four
are still present, unchanged, at the end of the run (verified by a final `tmux list-sessions`
diff against the baseline — byte-for-byte the same four names).

**Fixture.** A disposable git repo at
`.../scratchpad/sbo-fixture-repo` (`git init` + minimal `.gogo/work/feature-<slug>/{plan.md,state.md}`
per `docs/cli-contract.md` §1/§2), with four work items:

| slug | status | purpose |
|---|---|---|
| `sbotest-b` | `implementing`, no session | check (a): starts `· stalled`, gets adopted |
| `sbotest-k` | `implementing` | check (b): board `K` picker over 2 sessions |
| `sbotest-p` | `plan-accepted` | check (c): `P` launch confirm; also the foreign-anchor `R` refusal |
| `sbotest-shipped` | `shipped` | check (e): `[K]`-only chip, `R` refusal |

Fixture tmux sessions, every one carrying the unmistakable `sbotest` infix, running `sleep
6000`, anchored `-c` at the fixture repo: `gogo-go-sbotest-a`, `gogo-go-sbotest-k`,
`gogo-plan-sbotest-k`, `gogo-go-sbotest-shipped`. The board itself ran in a harness tmux
session named `sbo-test-harness` (deliberately **not** `gogo-`-prefixed, so it can never be
picked up by `ListSessions` and pollute the unbound count). `gogo status` against the fixture
confirmed the classifier (shipped 1 · in-progress 2 · unfinished 1) before driving the TUI.

`cd cli && go build -o /tmp/gogo-test-bin .` was the binary under test throughout.

### (a) `R` adopt end-to-end — PASS

Initial board (`tmux send-keys` + `capture-pane`):

```
sbotest-b   implement r1   · stalled
sessions: ... gogo-go-sbotest-a ... · 1 unbound session (matches no card here — R on a card adopts it)
```

`R` on `sbotest-b` opened the adopt picker over every live `gogo-*` session (including the
three real host ones and the real `dotai`-anchored catalogue session — the picker is
deliberately unfiltered by root). Navigated down to `gogo-go-sbotest-a · unbound ·
sbo-fixture-repo · 1m` (verifying the cursor position by `capture-pane` before every keypress,
never landing on a real session) and pressed Enter:

```
re-assigned gogo-go-sbotest-a → gogo-go-sbotest-b — sbotest-b's readers follow the new name
● sbotest-b   [m] go   [enter] drill   [v] view   [l] peek   [a] attach   [P] plan   [w] web
```

- `tmux list-sessions` confirmed **immediately** (no need to wait out the 5s tick — the rename
  handler calls `refreshSessions()` synchronously): `gogo-go-sbotest-a` gone, `gogo-go-sbotest-b`
  live.
- The card's `· stalled` cue was gone and `● developer` appeared in the same capture.
- The unbound count dropped to 0 (confirmed via the `sessions:` status line once `m.status`
  was cleared) — the counter tracks both directions correctly, not just up.

Matches the plan's acceptance signal and the gherkin scenario "re-assign a retasked session
onto the item it is really driving" exactly, including the immediate (sub-tick) correction.

### (b) `K` picker on a two-session card — PASS

Focused `sbotest-k` (footer offers no `[K]` chip — correct per REV-003's symptom-driven design,
since the card is neither stalled nor shipped; the key still works unadvertised, per FR2's
"K on the board" being universal). Pressed `K`:

```
Kill which session for sbotest-k?
> gogo-go-sbotest-k
  gogo-plan-sbotest-k
  all 2 sessions
  Cancel
```

Selected `gogo-plan-sbotest-k` only. Result: `killed 1 session`; `tmux list-sessions` confirmed
**exactly** `gogo-plan-sbotest-k` died and `gogo-go-sbotest-k` still lives (card still shows
`● developer`). No pipeline state touched (`sbotest-k`'s `state.md` untouched).

### (c) `P` on a non-terminal card — PASS (confirm + cancel path; launch verified via the unit suite)

Focused `sbotest-p` (`plan-accepted`), pressed `P`:

```
will run: claude "/gogo:plan sbotest-p"  in tmux session gogo-plan-sbotest-p
  at .../scratchpad/sbo-fixture-repo  · permission: auto (classifier)
plans sbotest-p in an attached tmux session — uncapped (planning never touches the working tree)
```

Confirms the exact command, the exact session name (`gogo-plan-sbotest-p`), and — critically —
the **correct anchor root** (the fixture repo, not the gogo repo or the harness's cwd).

**Decision: cancelled rather than launching for real** (the orchestrator's brief explicitly
allows this alternative when launching is disruptive). Rationale: completing the launch would
spawn a real `claude` process, nest a real tmux attach inside the capture-pane harness, cost
real wall-clock time and tokens for an LLM session with no useful planning target (the fixture
repo has no real work to plan), and the launcher-fires-once-then-attach mechanics are already
pinned by the green unit suite (`cd cli && go test -race ./internal/tui/...`, which per
`plan.md`'s own Tests table asserts "`P` fires the launcher **exactly once** with `/gogo:plan
<slug>` at the card's own root, then attaches" against an injected fake launcher). Pressing `n`
returned `cancelled`; `tmux list-sessions` confirmed **no** `gogo-plan-sbotest-p` was created —
the cancel path leaked nothing.

Combining the real, keystroke-driven confirm-dialog text with the already-verified unit path is
judged sufficient for this leg; the one gap this leaves is a real end-to-end `ExecProcess`
attach, which was not exercised in this round (see Notes below — not filed as an issue, since
the plan itself offers cancellation as an explicit alternative here).

**Bonus check — foreign-anchor `R` refusal (not in the required list, done because it was safe
and cheap once the picker was open).** Pressed `R` on `sbotest-p` again and deliberately
selected the real, unrelated `gogo-plan-catalogue-side-of-the-matching-engine-…` session
(anchored at `/Users/bartlomiej.zawadzki/repos/dotai`) to prove FR3's foreign-root refusal fires
**before** any tmux mutation:

```
⚠ that session is anchored at /Users/bartlomiej.zawadzki/repos/dotai, but sbotest-p lives in
  .../scratchpad/sbo-fixture-repo — a claude working elsewhere cannot drive this item
```

`tmux list-sessions` confirmed that session's name was **unchanged** afterwards — the refusal
short-circuits before `renamer` is ever called, exactly as the code shows (`finishAdopt`'s
`pathWithinRoot` check precedes the rename call).

### (d) Unbound count — PASS, including the foreign-root exclusion

- Before adopting `gogo-go-sbotest-a` (check a), the idle status line read `... · 1 unbound
  session (matches no card here — R on a card adopts it)` — correctly counting the one fixture
  session that matched no fixture card.
- After the adopt in (a), the same line dropped the `unbound` suffix entirely (count 0) —
  confirms the counter is live/bidirectional, not just additive.
- **Foreign-root exclusion, read-only verified:** the real
  `gogo-plan-catalogue-side-of-the-matching-engine---normalise-store-embed-hard-filter` session
  is listed in the board's plain `sessions:` summary (which lists every live `gogo-*` session
  host-wide) but **never** counted in the "N unbound" figure, because its `session_path`
  (`/Users/bartlomiej.zawadzki/repos/dotai`, confirmed via a read-only `tmux list-sessions -F
  session_name|session_path`) is outside every root the fixture board shows. This is exactly
  the FR4 exclusion rule and was never sent a keystroke, attached, or otherwise touched — pure
  observation.

### (e) Refusals spot-check on the shipped card — PASS

Focused `sbotest-shipped` (which carries the live `gogo-go-sbotest-shipped` fixture session).
Footer read:

```
● sbotest-shipped   [enter] drill   [v] view   [l] peek   [a] attach   [K] kill   [w] web
```

**No `[R] adopt`, no `[P] plan`** — exactly the FR4 REV-003 correction. Pressed `R` anyway (the
key itself, not just the missing chip):

```
⚠ sbotest-shipped is shipped — nothing to drive; press R on the card the session should drive
  (this one's session appears in that picker), or K to kill it
```

Refusal names the real move, per FR3. `sbotest-shipped`'s `state.md` was never touched (no
pipeline-state write from any of the three ops, confirmed by inspection — the fixture's
`state.md` files were not modified by any operation this round).

---

## Cleanup — confirmed

- Fixture tmux sessions killed by exact name (`tmux kill-session -t "=<name>"`, never a
  pattern/bare kill): `gogo-go-sbotest-b` (renamed from `-a`), `gogo-go-sbotest-k`,
  `gogo-go-sbotest-shipped` (the picker already reaped `gogo-plan-sbotest-k` as part of check
  b). Harness `sbo-test-harness` killed.
- Final `tmux list-sessions` showed **exactly** the four real baseline sessions, unchanged, and
  nothing else — no leaked fixture or harness session.
- `/tmp/gogo-test-bin` removed.
- The fixture repo directory (`.../scratchpad/sbo-fixture-repo`) removed.
- No product code was edited. `.gogo/work/feature-session-binding-ops/` gained only this
  round's test artifacts (`test/issues.json`, `test-01.md`) plus the occupancy write to
  `state.md`/`events.jsonl`.

## Notes (not filed as issues)

- Check (c) did not drive a real `tea.ExecProcess` attach leg end-to-end — cancelled per the
  orchestrator's explicit permission to avoid a disruptive real `claude` launch, backed by the
  already-green unit coverage of the launcher-fires-once-then-attach wiring. If a future round
  wants that last mile covered live, the safe way is a `claude` PATH stub (per
  `testing-tools.md`'s "Stubbed `claude` on PATH" pattern) so the attach completes against a
  fast, harmless fake instead of a real LLM session.
- `sbotest-k`'s footer never offered a `[K]` chip despite holding two live sessions — this is
  the plan's own documented intent (`review-02.md`: "chips are symptom-driven … the absence of
  `[K]` on a live, non-shipped card is the plan's intent, not an omission"), reconfirmed live
  here; not a defect.

## Verdict: **GREEN**

Build + `gofmt` + `go vet` + `go test -race -count=1 ./...` all clean. Every relevant hands-on
check from the plan's phase-④ row (as adapted) ran to completion with real tmux on this host:
`R` adopt end-to-end (including the sub-tick correction and the bidirectional unbound count),
`K`'s picker killing exactly the chosen session, `P`'s launch confirm (command/session/root all
correct) with an explicitly-permitted cancel in place of a disruptive real launch, and the
shipped-card refusal/chip spot-check. Two bonus real-tmux checks (the foreign-anchor `R`
refusal on both a fixture and the real catalogue session) also passed. No hands-on check was
blocked; no issue was found. `test/issues.json` carries zero open/new issues — **done-bar met,
advance to ⑤ report.**
