package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/orchestrator"
)

// capRuleSurfaces are the source files that state the concurrency cap's rule to the USER.
// 0.28.0 deliberately wrote the rule in four places; FR12 changed the rule and only ONE of
// them was updated, leaving the source-edit form the user reads WHILE SETTING the cap
// stating the opposite of what the code does (REV-002). All four now quote
// orchestrator.CapRuleClause, and these guards are what keep them doing so.
var capRuleSurfaces = []string{
	"main.go",                      // gogo --help
	"internal/tui/move.go",         // the board's cap bounce
	"internal/tui/config_tab.go",   // the source-edit cap field AND capScopeNote
	"internal/orchestrator/cap.go", // the clause itself + the counting rule's doc comment
}

// staleCapRulePhrases are every phrasing of the PRE-0.29.0 rule. The cap no longer consults
// a feature's file-derived class, so any of these in live copy is now factually wrong: a
// mid-build item reads `plan-accepted`, classifies unfinished, and IS counted.
var staleCapRulePhrases = []string{
	"in-progress work items with a live session",
	"in-progress work items that have a live session",
	"in-progress features that carry a live",
	"in-progress+live",
}

// TestCapRuleIsSingleSourced pins that every user-visible statement of the cap's rule comes
// from the ONE constant, PER SURFACE and in CODE - not per file, and not counting comments.
//
// The per-file version had the hole it was meant to prevent (REV-012): config_tab.go carries
// TWO cap surfaces, and hand-writing one of them left "CapRuleClause" in the file - in the
// other surface and in three explanatory comments - so a fresh hand-written sentence in the
// field the user reads WHILE SETTING the cap passed the whole suite. That is the same
// "assertion that looks like a check and isn't" class this feature keeps hitting, in the guard
// written to stop it.
//
// So: strip comments, then require at least as many code references as that file has surfaces.
// The behavioural assertions below (rendered `--help`, capScopeNote, capFieldDescription) are
// the real backstop - a rendered string cannot be satisfied by a comment or by a sibling
// surface - and this structural check catches a surface that stops referencing the constant at
// all.
func TestCapRuleIsSingleSourced(t *testing.T) {
	// file → how many DISTINCT cap surfaces it owns.
	surfaces := map[string]int{
		"main.go":                      1, // gogo --help
		"internal/tui/move.go":         1, // the board's cap bounce
		"internal/tui/config_tab.go":   2, // the cap form field AND capScopeNote
		"internal/orchestrator/cap.go": 1, // the clause's own definition
	}
	for rel, want := range surfaces {
		code := stripGoLineComments(readRepoFile(t, rel))
		got := strings.Count(code, "CapRuleClause")
		if got < want {
			t.Errorf("%s references CapRuleClause %d time(s) in CODE, want at least %d (one per cap surface "+
				"it owns). A surface that hand-writes the sentence again is how three of four copies went "+
				"stale (FR12a/REV-002); counting per FILE let a partial hand-write survive (REV-012).",
				rel, got, want)
		}
	}
}

// The two config-tab surfaces are asserted BEHAVIOURALLY in their own package, where their
// producers live (internal/tui/authoring_test.go, TestConfigTabCapSurfacesRenderTheRule) -
// rather than exporting test-only hooks out of production code just to reach them from here.
// The rendered `--help` surface is asserted below, in this package.

// TestCapBlockComposesThroughCapRefusal is the headless twin of the tui package's
// TestCapRefusalsComposeThroughCapRefusal (REV-019). Both cap refusals must build their remedy
// list through orchestrator.CapRefusal so an empty remedy DROPS OUT instead of rendering ", ,".
// Structural for the same reason: the empty case is unreachable from this call site (capBlock
// sits behind CapExceeded, which needs a non-empty blocker list), so no behavioural assertion
// can tell the composed form from unconditional interpolation.
func TestCapBlockComposesThroughCapRefusal(t *testing.T) {
	body := stripGoLineComments(readRepoFile(t, "go.go"))
	i := strings.Index(body, "\nfunc capBlock(")
	if i < 0 {
		t.Fatal("func capBlock not found in go.go")
	}
	rest := body[i+1:]
	end := strings.Index(rest, "\n}\n")
	if end < 0 {
		t.Fatal("could not find the end of func capBlock")
	}
	fn := rest[:end]
	if !strings.Contains(fn, "orchestrator.CapRefusal(") {
		t.Errorf("capBlock no longer composes its remedies through orchestrator.CapRefusal, so an empty "+
			"remedy would render as double punctuation (REV-019). Body:\n%s", fn)
	}
	if !strings.Contains(fn, "orchestrator.CapSweepRemedy(") {
		t.Errorf("capBlock no longer names the targeted sweep remedy at all (REV-015). Body:\n%s", fn)
	}
}

// stripGoLineComments removes `//` line comments so a structural guard reads code, not prose -
// the same technique tuiFuncBody uses. A comment naming the constant must not satisfy a check
// that the CODE references it (REV-012).
func stripGoLineComments(src string) string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		inStr := false
		for i := 0; i < len(line)-1; i++ {
			switch {
			case line[i] == '"' && (i == 0 || line[i-1] != '\\'):
				inStr = !inStr
			case !inStr && line[i] == '/' && line[i+1] == '/':
				line = line[:i]
				i = len(line)
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// TestNoStaleCapRuleWording greps the whole CLI (production code only) for every pre-0.29.0
// phrasing of the rule. The one sanctioned exception is a NEGATIVE assertion in a test that
// checks the old wording is gone.
func TestNoStaleCapRuleWording(t *testing.T) {
	root := "."
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil // a test may quote the old wording to assert its ABSENCE
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		for _, stale := range staleCapRulePhrases {
			if strings.Contains(string(body), stale) {
				rp, _ := filepath.Rel(root, path)
				t.Errorf("%s still states the PRE-0.29.0 cap rule (%q). The cap no longer filters on the "+
					"file-derived class - a mid-build item reads plan-accepted, classifies unfinished, and IS "+
					"counted. Quote orchestrator.CapRuleClause instead.", rp, stale)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// TestCapRuleClauseSaysWhatItCounts pins the CONTENT of the shared clause, so single-sourcing
// cannot be satisfied by a constant that says nothing. It must name the build session, its
// per-source scope, and what is NOT counted - the three facts a user needs to predict a bounce.
func TestCapRuleClauseSaysWhatItCounts(t *testing.T) {
	c := orchestrator.CapRuleClause
	for _, want := range []string{"live build session", "gogo-go-<slug>", "per source", "authoring session", "never counted"} {
		if !strings.Contains(c, want) {
			t.Errorf("CapRuleClause = %q, missing %q", c, want)
		}
	}
	for _, stale := range staleCapRulePhrases {
		if strings.Contains(c, stale) {
			t.Errorf("CapRuleClause carries the stale phrasing %q", stale)
		}
	}
}

// TestHelpStatesTheCapRule proves the `gogo --help` SURFACE renders the clause, not just that
// main.go mentions the identifier - the review-#11b lesson (a wiring named in a plan, shipped,
// and never asserted at its call site).
func TestHelpStatesTheCapRule(t *testing.T) {
	out, _ := captureStdout(t, func() int { printHelp(); return 0 })
	if !strings.Contains(out, orchestrator.CapRuleClause) {
		t.Errorf("gogo --help does not state the cap rule; help:\n%s", out)
	}
	for _, stale := range staleCapRulePhrases {
		if strings.Contains(out, stale) {
			t.Errorf("gogo --help still states %q", stale)
		}
	}
}
