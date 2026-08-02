---
name: assertion-vacuity-catalog
description: >-
  Use when WRITING or REVIEWING tests for the gogo CLI — before trusting any
  assertion, and always when running a mutation sweep: the catalog of FOURTEEN
  distinct ways an assertion (or the mutation harness itself) looks like a check
  while checking nothing. Keywords: vacuous test, mutation testing, coverage that
  bites, false-green, anti-vacuity.
---

# assertion-vacuity-catalog — fourteen ways a "check" checks nothing

Lifted from `knowledge/test-strategy.md › The FOURTEEN variants` by /gogo:skills on 2026-08-02.

## When this applies
Writing new tests, reviewing a diff's tests, or running/auditing a mutation
sweep. The parent test-strategy.md keeps the standing rule (mutation IS the
coverage check); this is the reference catalog of the failure shapes.

### The FOURTEEN variants of "an assertion that looks like a check and isn't" (0.28.0 + 0.29.0)
Two releases produced **fourteen distinct** ways for a test to look like a check while checking
nothing - **or for the mutation sweep itself to report a false result**. Two were the
reviewer's own harness mistakes, which is exactly why they are here: *a mutation count
produced by a broken harness is not trustworthy in either direction.* Walk this list before
signing off a guard.

**The four harness rules (a wrong sweep is worse than no sweep):**
1. **Compile-check every mutation FIRST, with `go vet ./...` - not `go build`.** A mutation
   that does not compile is `BUILD-FAIL`, not a result. And **`go build` does not type-check
   `_test.go`**, so a mutation to a test file passes `go build` and then fails at test time
   for the wrong reason. `go vet` type-checks tests.
2. **Assert the edit landed via a marker unique to the NEW text**, never via "the anchor is
   gone" - an **insertion whose replacement contains its anchor** trips a naive check and
   reports `EDIT-DID-NOT-LAND` for a perfectly applied mutation.
9. **A nameless CAUGHT is UNSCORED.** A failure with no test name attached is usually a
   compile error in the mutation, not a catch. Re-run it compile-clean before scoring.
10. **Never `&&` after a pipe when the pipe's exit code is the result** - the `&&` sees the
    last stage's status, so a broken mutation reports success. Check the compile step's
    status directly, or `set -o pipefail`.

**The six ways an assertion misses its target:**
3. **A structural guard that matches its own comment.** Grepping source text is satisfied by
   the doc comment *describing* the rule, so deleting the code passes. **Strip `//` comments
   before a structural grep** (`tuiFuncBody`) and pair the structural half with a
   **behavioural** half.
4. **Two styles that render identically under a TTY-less terminal.** A "these differ"
   assertion comparing *rendered* lipgloss strings passes for the right AND the wrong style,
   because colour is flattened. Compare style **properties**
   (`GetForeground()`/`GetBackground()`/`GetBold()`), and make every user-visible cue
   **glyph + word** so `View()` substring matching works without colour.
5. **A fixture whose removal changes no assertion** - decoration masquerading as an input.
   **Mutate the fixture, not only the code**: if deleting a fixture element leaves the suite
   green, that element is not under test.
6. **An exclusivity/invariant assertion that is true VACUOUSLY** - "no two arms overlap"
   passes trivially if the matrix never reaches an arm. **Pair it with a reachability
   guard**; shrinking the matrix must FAIL.
7. **A guard-only mutation can never fail while the production code is correct**, so scoring
   it SURVIVED is meaningless. Use a **two-part mutation**: weaken the guard **and**
   introduce the defect it exists to catch, then check something else still bites.
8. **A guard satisfied by its own producer's body.** Extracting a decision into a producer
   and asserting the producer leaves the **call sites** unguarded - either surface can stop
   calling it and hand-write fresh copy with the whole suite green. **Assert the wiring**:
   the rendered output where the surface is readable, and a structural call-site check where
   the value cannot be read back (a huh field's `Description`, for instance).

**The four found by applying this list to the guards written for it (0.29.0 rounds 04-07):**

11. **A guard that is unreachable because an earlier branch always returns.** A message arm
    was aligned to another surface's terminal case - which that surface never reaches, because
    it returns earlier. Aligning to dead code propagates a falsehood (an `aborted` feature was
    told it had "already shipped"). Check the arm you are matching can actually execute.
12. **A guard matched a substring that its own subject also contains.** `Contains(exit,
    "reviewing")` passed against the regression because the section also carries a
    `phase-done` JSON event with `"status":"reviewing"`. Match the shape of the instruction
    (`status: reviewing`), not a word that appears in the neighbourhood.
13. **An anchor written from memory never lands.** A mutation whose anchor was recalled
    rather than read did not match; the edit silently did nothing and the run reported a
    PASS. Always re-read the bytes you are about to mutate, and verify the edit landed.
14. **A test that pins ONE surface of a shared predicate.** A fix unified three call sites
    behind one predicate; the test asserted only the action path, so reverting the renderer
    or the toggle left the suite green. Mutate EVERY surface the fix claims to unify.

**The shape that recurs:** the thing asserted was **one level away from the thing that
matters** - the producer instead of the wiring (8), the comment instead of the code (3), the
arm instead of its reachability (6, 11), one surface instead of all of them (14). When you
cannot write the test you want, say so instead of writing one that passes for a weaker
reason: 0.29.0's review asked for a test that fails when two disjoint predicates are swapped,
and **no such test can exist** - the honest answer was a disjointness proof plus a
reachability guard.

**Corollary for a guard over a SHIPPED template or asset:** read the shipped file itself, not
a copy, and **first assert the file still contains the hazard** - otherwise the guard passes
because someone deleted the hazard instead of handling it. 0.29.0's TEST-001 guard caught a
literal comment closer in the template's own new warning note within minutes of it being
written.

**Corollary for scoring a GUARD-HARDENING change (the control pair).** Variant 7 says a
guard-only mutation cannot fail while the defect is absent; the practical form is a **control
pair**. Reintroduce the defect in the shape the hardening targets (0.29.0: the forbidden
phrase *wrapped across a line break*), then assert the **hardened** guard fails **and** the
old raw-matching one passes. One run, two data points, and it distinguishes "the guard is
stronger" from "nothing changed". Restore both files byte-for-byte and md5-verify afterwards.
