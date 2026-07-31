package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestSkillsBashNoUnsafeRm is the durable regression guard for Slice A
// (unattended-ops-input-signals, FR-A5): gogo's own mechanical file steps must
// never reintroduce a bash shape the harness's "dangerous rm" permission
// classifier flags, or /gogo:done starts halting on a false prompt again.
//
// It scans every skills/*/SKILL.md for the three unsafe idioms the plan
// enumerated and fails on any reappearance:
//   - glob-rm            e.g.  rm -f "$dst"/*.mmd
//   - rm -rf "$var…"     e.g.  rm -rf "$dst/before"
//   - rm -f "$var"       e.g.  rm -f "$res" "$code"  (bare-variable rm)
//
// The safe replacements are guarded, scoped `find <dir> … -delete` (no glob,
// no bare-variable rm) — see skills/gogo-done/SKILL.md and skills/gogo-build.
//
// The patterns anchor `rm` as an actual COMMAND (start-of-line or whitespace,
// then whitespace) so prose mentions like "dangerous rm", "glob-`rm`", or the
// words confirm / warm / term never match — only real invocations do.
func TestSkillsBashNoUnsafeRm(t *testing.T) {
	// rm as a command: preceded by start/whitespace, followed by whitespace.
	// This excludes word-internal "rm" (confirm, warm, term) and backtick-quoted
	// prose ("`rm`", "dangerous rm\"") which are never followed by a bare space.
	unsafe := []struct {
		name string
		re   *regexp.Regexp
	}{
		{"glob-rm (rm …*)", regexp.MustCompile(`(?:^|\s)rm\s[^\n]*\*`)},
		{`rm on a bare variable (rm … "$var")`, regexp.MustCompile(`(?:^|\s)rm\s+(?:-\S+\s+)*"?\$`)},
	}

	skills, err := filepath.Glob(filepath.Join("..", "skills", "*", "SKILL.md"))
	if err != nil {
		t.Fatalf("glob skills: %v", err)
	}
	if len(skills) == 0 {
		t.Fatal("no skills/*/SKILL.md found — wrong cwd? (expected to run from cli/)")
	}

	for _, path := range skills {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, u := range unsafe {
				if u.re.MatchString(line) {
					t.Errorf("%s:%d — unsafe %s reappeared: %q\n"+
						"  rewrite to guarded scoped `find <dir> … -delete` (no glob, no bare-variable rm)",
						path, i+1, u.name, strings.TrimSpace(line))
				}
			}
		}
	}
}

// phaseOccupancyWriters are the three phase skills that must record occupancy in `state.md`,
// with the status each writes.
var phaseOccupancyWriters = map[string]string{
	"gogo-implement": "implementing",
	"gogo-review":    "reviewing",
	"gogo-test":      "testing",
}

// TestPhaseSkillsWriteOccupancyAtEntryAndExit is the deterministic guard for the one rule in
// this release that has already regressed once, in the release that introduced it.
//
// FR11 moved each phase's `phase`/`status` write to phase ENTRY and - as an unintended
// consequence, argued for by no FR - REMOVED the exit write that had been there since before
// 0.28.0. The entry write is prose an LLM follows, and it was skipped on all three of its live
// runs, so with only that half `state.md` stopped advancing at all: arbitrarily stale instead
// of reliably one phase behind, which is worse than the defect the release set out to fix. It
// was restored by user decision, entry AND exit.
//
// The reason this is a TEST and not another paragraph: the removal happened once because the
// exit write looked redundant, and `code-review-standards.md` then briefly instructed the next
// fresh-eyes reviewer to flag a post-work write as a defect - i.e. the standards themselves
// would have driven the regression back in. Prose guarding prose is exactly what this feature
// exists to replace. A reviewer or an agent that "tidies away" either write now fails here
// instead of shipping.
func TestPhaseSkillsWriteOccupancyAtEntryAndExit(t *testing.T) {
	for skill, status := range phaseOccupancyWriters {
		body := readRepoFile(t, "..", "skills", skill, "SKILL.md")
		steps := sectionBetween(body, "## ② Steps", "## ③")
		exit := sectionBetween(body, "## ④", "## ")
		if steps == "" || exit == "" {
			t.Fatalf("%s: could not locate its ② Steps / ④ sections - the parser drifted from the skill's shape", skill)
		}
		// Match the WRITE INSTRUCTION's shape - "status: reviewing" with a colon-space - not the
		// bare word. Both sections also carry the phase-started/phase-done JSON events, whose
		// `"status":"reviewing"` form satisfied a naive Contains check and made the first draft of
		// this guard pass against the regression it was written to catch.
		want := "status: " + status
		// ENTRY: step 1 of the numbered flow writes the occupancy status.
		if !strings.Contains(steps, want) {
			t.Errorf("skills/%s/SKILL.md §② does not tell the phase to write `status: %s` as its first act. "+
				"`state.md` is what every reader believes (the board's column, `gogo status`, the "+
				"concurrency cap); written only at the exit it records work that has already stopped.",
				skill, status)
		}
		// EXIT: §④ writes it AGAIN. This is the half that was removed once.
		if !strings.Contains(exit, want) {
			t.Errorf("skills/%s/SKILL.md §④ no longer writes `status: %s`. The entry write does NOT make this "+
				"redundant - it is prose that gets skipped (n=3 in the release that added it), and with only "+
				"the entry write `state.md` stops advancing at all. Two writers, one at each end: floor = one "+
				"phase behind, ceiling = live. This exact removal shipped once and had to be reverted.",
				skill, status)
		}
		// And the exit write must be SCOPED so it cannot clobber a user gate. Match the SCOPING
		// SENTENCE, not the string "waiting-for-user" - §④ is also where the decision-gate ROUTE
		// is listed, so the bare name is present whether or not the write is scoped (this guard's
		// first draft passed against an unscoped write for exactly that reason).
		if !strings.Contains(exit, "Never overwrite a gate status") {
			t.Errorf("skills/%s/SKILL.md §④ does not scope its write away from a gate status. An unscoped "+
				"write lets a decision-gate round overwrite `waiting-for-user` with the working status, "+
				"losing the ⏸ gate count, the card stripe and the /gogo:go refusal that stops an unattended "+
				"rerun.", skill)
		}
	}
}

// sectionBetween returns the text of body from the first line starting with start up to the
// next line starting with after (exclusive), or "" when start is absent.
func sectionBetween(body, start, after string) string {
	i := strings.Index(body, start)
	if i < 0 {
		return ""
	}
	rest := body[i+len(start):]
	if j := strings.Index(rest, "\n"+after); j >= 0 {
		return rest[:j]
	}
	return rest
}

// TestReviewStandardsDoNotInviteTheExitWriteRemoval closes the OTHER path by which the
// occupancy regression returns - the human one, which is how it actually happened.
//
// `code-review-standards.md` check #13 briefly read "flag a phase skill whose `phase`/`status`
// write sits after the work rather than as its first act after validate-in". A fresh-eyes
// reviewer following the standards in good faith would have flagged the restored §④ write as a
// defect and had it removed again. The skills guard above cannot see that coming: by the time
// the skill text changes, the damage is agreed. So assert the standard itself still tells a
// reviewer that BOTH writes are required and that the exit write is not duplication.
func TestReviewStandardsDoNotInviteTheExitWriteRemoval(t *testing.T) {
	// Normalise whitespace before matching (TEST-003). These are phrases in prose that gets
	// re-wrapped by ordinary editing, and a phrase-match against raw text stops matching the
	// moment a line break lands inside it - so an innocuous reflow would silently retire the
	// guard while leaving it green. Collapsing every whitespace run to one space makes the
	// match wrap-insensitive; it is the text that matters, not its line breaks.
	body := strings.Join(strings.Fields(readRepoFile(t, "..", ".gogo", "knowledge", "code-review-standards.md")), " ")
	if !strings.Contains(body, "Do NOT flag the exit write as duplication") {
		t.Error(".gogo/knowledge/code-review-standards.md no longer tells a reviewer that the exit write is " +
			"deliberate. Without that, a reviewer following the standards flags it as duplication and the " +
			"occupancy regression comes back - which is exactly how it happened the first time.")
	}
	// And it must not have reverted to the wording that invited the removal.
	if strings.Contains(body, "write sits after the work rather than") {
		t.Error(".gogo/knowledge/code-review-standards.md is back to instructing a reviewer to flag a " +
			"post-work phase/status write. The rule is BOTH writes: first act after validate-in AND again " +
			"at the exit.")
	}
}
