package contract

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writePlan writes a plan.md with n `## ` sections into dir and returns dir.
func writePlan(t *testing.T, dir string, n int) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("# Plan - fixture\n\n")
	for i := 0; i < n; i++ {
		b.WriteString("## Section ")
		b.WriteByte(byte('A' + i))
		b.WriteString("\n\nbody\n\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "plan.md"), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write plan.md: %v", err)
	}
	return dir
}

// writeState writes a state.md with the given bolded lines.
func writeState(t *testing.T, dir string, lines ...string) {
	t.Helper()
	body := "# State - fixture\n\n" + strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "state.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write state.md: %v", err)
	}
}

// TestPlanSectionsCounts pins the D2=A stub check at its exact boundary: the threshold is
// PlanSectionsRequired `## ` sections, so 1 is a stub and 2 is written. The boundary is the
// whole rule, and a proxy assertion (e.g. "a big file is written") would not exercise it.
func TestPlanSectionsCounts(t *testing.T) {
	for _, n := range []int{0, 1, 2, 8} {
		dir := writePlan(t, t.TempDir(), n)
		got, err := PlanSections(dir)
		if err != nil {
			t.Fatalf("PlanSections(%d sections): %v", n, err)
		}
		// The scan stops early once the threshold is met, so a written plan reports exactly
		// the threshold rather than its full count - the count only has to be exact BELOW
		// the bar, which is where the refusal quotes it.
		want := n
		if n > PlanSectionsRequired {
			want = PlanSectionsRequired
		}
		if got != want {
			t.Errorf("PlanSections with %d sections = %d, want %d", n, got, want)
		}
		if written := planWritten(dir); written != (n >= PlanSectionsRequired) {
			t.Errorf("planWritten with %d sections = %v, want %v", n, written, n >= PlanSectionsRequired)
		}
	}
}

// TestPlanSectionsIgnoresOtherHeadingLevels: the check is `## ` sections specifically. A
// scaffold that is nothing but a `# title` (exactly what the observed broken folder had) is
// a stub, and `###` sub-headings do not lift a one-section plan over the bar.
func TestPlanSectionsIgnoresOtherHeadingLevels(t *testing.T) {
	dir := t.TempDir()
	body := "# plan\n\n### Sub one\n\n### Sub two\n\n#not a heading\n\n## Goal\n\nreal\n"
	if err := os.WriteFile(filepath.Join(dir, "plan.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	n, err := PlanSections(dir)
	if err != nil {
		t.Fatalf("PlanSections: %v", err)
	}
	if n != 1 {
		t.Errorf("PlanSections = %d, want 1 (only `## ` counts)", n)
	}
	if planWritten(dir) {
		t.Error("planWritten = true for a plan with one `## ` section - the stub check did not bite")
	}
}

// TestPlanSectionsAbsent: no plan.md at all is an fs.ErrNotExist, which planWritten reads as
// genuinely unwritten. This is the observed defect's shape (state.md + decisions.md +
// charts/, no plan.md).
func TestPlanSectionsAbsent(t *testing.T) {
	dir := t.TempDir()
	if _, err := PlanSections(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("PlanSections err = %v, want fs.ErrNotExist", err)
	}
	if planWritten(dir) {
		t.Error("planWritten = true with no plan.md on disk")
	}
	if got := PlanUnwrittenReason(dir); got != "no plan.md on disk yet" {
		t.Errorf("PlanUnwrittenReason = %q, want the absent-file clause", got)
	}
}

// TestPlanSectionsUnreadableCountsAsWritten pins the deliberate asymmetry: a read error is
// NOT a defect. A permissions hiccup or a directory-shaped plan.md must never invent an
// authoring state that refuses the user's accept - only a plan that is PROVABLY absent or
// PROVABLY a stub does that.
func TestPlanSectionsUnreadableCountsAsWritten(t *testing.T) {
	dir := t.TempDir()
	// A plan.md that is a DIRECTORY: os.Open succeeds, the read fails - a non-ErrNotExist
	// error that must degrade to "written".
	if err := os.Mkdir(filepath.Join(dir, "plan.md"), 0o755); err != nil {
		t.Fatalf("mkdir plan.md: %v", err)
	}
	if _, err := PlanSections(dir); err == nil {
		t.Fatal("PlanSections over a directory returned no error - the fixture no longer exercises the case")
	}
	if !planWritten(dir) {
		t.Error("planWritten = false for an UNREADABLE plan.md - a read error must never invent a defect")
	}
	if got := PlanUnwrittenReason(dir); got != "" {
		t.Errorf("PlanUnwrittenReason = %q for an unreadable plan, want \"\" (nothing to refuse)", got)
	}
}

// TestPlanSectionsEmptyDir: a Dir-less Feature (every synthetic Feature in the suite) must
// never make PlanSections stat a RELATIVE "plan.md" in the process cwd - it refuses outright.
func TestPlanSectionsEmptyDir(t *testing.T) {
	if _, err := PlanSections(""); !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("PlanSections(\"\") err = %v, want fs.ErrInvalid", err)
	}
	if planWritten("") {
		t.Error("planWritten(\"\") = true - an empty dir must not resolve to a relative plan.md")
	}
}

// TestPlanUnwrittenReasonNamesItsNumber pins the Diagnosability bar's "a limit must name its
// number" rule: the stub refusal says HOW FAR SHORT it fell, not "too small".
func TestPlanUnwrittenReasonNamesItsNumber(t *testing.T) {
	dir := writePlan(t, t.TempDir(), 1)
	got := PlanUnwrittenReason(dir)
	if got != "plan.md has 1 of the 2 sections a written plan needs" {
		t.Errorf("PlanUnwrittenReason = %q, want the exact 1-of-2 tally", got)
	}
	if PlanUnwrittenReason(writePlan(t, t.TempDir(), 2)) != "" {
		t.Error("PlanUnwrittenReason non-empty for a written plan")
	}
}

// TestAuthoringMatrix pins Authoring()'s exact shape: an unwritten plan AND a status that is
// the plan-acceptance gate or empty. Crucially it is DEFECT-POSITIVE - a zero-value Feature
// (PlanUnwritten false) is never authoring, which is what keeps every synthetic Feature in
// the suite on its pre-0.29.0 meaning.
func TestAuthoringMatrix(t *testing.T) {
	cases := []struct {
		name      string
		unwritten bool
		status    string
		want      bool
	}{
		{"zero value (the byte-for-byte default)", false, "", false},
		{"unwritten + at the plan gate", true, "awaiting-plan-acceptance", true},
		{"unwritten + no status (a bare/garbage state.md)", true, "", true},
		{"unwritten + plan-accepted (FR8's case, NOT authoring)", true, "plan-accepted", false},
		{"unwritten + implementing", true, "implementing", false},
		{"unwritten + aborted", true, "aborted", false},
		{"unwritten + shipped", true, "shipped", false},
		{"WRITTEN + at the plan gate (the normal gate)", false, "awaiting-plan-acceptance", false},
	}
	for _, c := range cases {
		f := &Feature{PlanUnwritten: c.unwritten, Status: c.status}
		if got := f.Authoring(); got != c.want {
			t.Errorf("%s: Authoring() = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestWaitingForInputExcludesAuthoring pins FR2: an authoring item is NOT a user gate, so it
// drops out of the header's "⏸ K need you" pill and the `gogo status` WAIT column - while a
// genuine plan gate (a WRITTEN plan awaiting acceptance) still counts. waiting_test.go pins
// the zero-value behaviour unchanged; this pins the one new arm.
func TestWaitingForInputExcludesAuthoring(t *testing.T) {
	authoring := &Feature{Status: "awaiting-plan-acceptance", PlanUnwritten: true}
	if authoring.WaitingForInput() {
		t.Error("WaitingForInput = true for an AUTHORING item - it would inflate the ⏸ gate count")
	}
	gate := &Feature{Status: "awaiting-plan-acceptance"}
	if !gate.WaitingForInput() {
		t.Error("WaitingForInput = false for a real plan gate - the gate itself regressed")
	}
	// The other two gates are status-only and must be untouched by the plan check.
	for _, s := range []string{"waiting-for-user", "awaiting-uat"} {
		if !(&Feature{Status: s, PlanUnwritten: true}).WaitingForInput() {
			t.Errorf("WaitingForInput(%q) = false with an unwritten plan - only the plan gate consults it", s)
		}
	}
}

// TestStripPlaceholder pins FR6 at the parser: a value that is exactly a `<…>` template
// placeholder reads as ABSENT, while a value that merely contains angle brackets is kept.
func TestStripPlaceholder(t *testing.T) {
	cases := []struct{ in, want string }{
		{"<one-line title>", ""},
		{"<slug>", ""},
		{"<YYYY-MM-DD>", ""},
		{"<git branch | n/a>", ""},
		// A value that is angle-bracket junk end to end is template junk too (the resume
		// line's `<phase to re-enter> - <next action>`), so it also reads as absent. The
		// rule is deliberately the simple one: `<` … `>` spanning the WHOLE value.
		{"<phase to re-enter> - <next action>", ""},
		{"Real title", "Real title"},
		{"a < b > c", "a < b > c"},
		{"", ""},
		{"<", "<"},
		{"<>", ""},
	}
	for _, c := range cases {
		if got := stripPlaceholder(c.in); got != c.want {
			t.Errorf("stripPlaceholder(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestTemplatePlaceholdersNeverRender is FR6 end to end over the loader: a folder scaffolded
// from templates/state.template.md and not finished must yield an EMPTY title (so the card
// falls back to the slug) and an EMPTY created date (so the broken card stops sorting to the
// TOP of the plan column - '<' 0x3C sorts above '2' 0x32, which put the most-broken card in
// the most prominent position).
func TestTemplatePlaceholdersNeverRender(t *testing.T) {
	root := t.TempDir()
	scaffold := filepath.Join(root, ".gogo", "work", "feature-demo")
	real := filepath.Join(root, ".gogo", "work", "feature-realdate")
	for _, d := range []string{scaffold, real} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	writeState(t, scaffold,
		"- **feature:** <one-line title>",
		"- **phase:** plan",
		"- **status:** awaiting-plan-acceptance",
		"- **created:** <YYYY-MM-DD>",
		"- **branch:** <git branch | n/a>",
	)
	writeState(t, real,
		"- **feature:** A real feature",
		"- **phase:** plan",
		"- **status:** awaiting-plan-acceptance",
		"- **created:** 2026-01-01",
	)
	writePlan(t, real, 8)

	repo, err := LoadRepo(root)
	if err != nil {
		t.Fatalf("LoadRepo: %v", err)
	}
	demo := repo.Feature("demo")
	if demo == nil {
		t.Fatal("demo feature missing")
	}
	if demo.Title != "" {
		t.Errorf("Title = %q, want \"\" so the card falls back to the slug", demo.Title)
	}
	if demo.Created != "" {
		t.Errorf("Created = %q, want \"\" (a placeholder date must not sort as a date)", demo.Created)
	}
	if demo.Branch != "" {
		t.Errorf("Branch = %q, want \"\"", demo.Branch)
	}
	// Newest-first ordering: the real 2026-01-01 card must beat the placeholder-dated one.
	if repo.Features[0].Slug != "realdate" {
		t.Errorf("first card = %q, want realdate - the placeholder `created` still sorts to the top",
			repo.Features[0].Slug)
	}
}

// templatePath is the SHIPPED templates/state.template.md, read from the repo rather than
// copied into the test. That is deliberate (TEST-001): the template IS this release's
// prescribed Slice-A fixture, and a copied fixture would have kept passing while the shipped
// template grew the legend block that broke it. Reading the real file is the structural form
// that catches the class.
func templatePath(t *testing.T) string {
	t.Helper()
	p := filepath.Join("..", "..", "..", "templates", "state.template.md")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("cannot read the shipped template at %s: %v", p, err)
	}
	return p
}

// TestShippedTemplateScaffoldParsesClean is TEST-001's regression guard, run against the
// SHIPPED template itself.
//
// The template's optional-correlation legend wraps an EXAMPLE `- **correlation:** [plan-XXXX]`
// line in a multi-line `<!-- ... -->` block. parseStateFile matched that example as a real
// field and parseCorrelationList split its prose on commas into three bogus plan ids, so any
// item scaffolded straight from the template claimed membership in three cross-source plans and
// painted a `⛓ ×3` chip. Same family as stripPlaceholder: the template's own legend leaking
// into card UI.
func TestShippedTemplateScaffoldParsesClean(t *testing.T) {
	raw, err := os.ReadFile(templatePath(t))
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	// Sanity: the fixture must still CONTAIN the commented-out example, or this test would
	// pass because the hazard was removed from the template rather than handled by the parser.
	if !strings.Contains(string(raw), "- **correlation:**") {
		t.Fatal("the shipped template no longer carries a commented-out `- **correlation:**` example, " +
			"so this guard no longer exercises TEST-001 - re-point it at whatever legend example replaced it")
	}
	root := t.TempDir()
	dir := filepath.Join(root, ".gogo", "work", "feature-demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "state.md"), raw, 0o644); err != nil {
		t.Fatalf("write state.md: %v", err)
	}

	repo, err := LoadRepo(root)
	if err != nil {
		t.Fatalf("LoadRepo: %v", err)
	}
	f := repo.Feature("demo")
	if f == nil {
		t.Fatal("demo feature missing")
	}
	if len(f.Correlations) != 0 {
		t.Errorf("Correlations = %#v, want none - a commented-out legend example must not parse as a "+
			"real field (it painted a bogus `⛓ ×%d` chip)", f.Correlations, len(f.Correlations))
	}
	// The keys OUTSIDE the comments must still parse, or the fix would have thrown the file away.
	if f.Phase != "plan" || f.Status != "awaiting-plan-acceptance" {
		t.Errorf("phase=%q status=%q, want plan/awaiting-plan-acceptance - the real keys must still parse",
			f.Phase, f.Status)
	}
	// And the placeholder keys stay empty (FR6), which is what makes this a clean scaffold.
	if f.Title != "" || f.Created != "" || f.Branch != "" {
		t.Errorf("title=%q created=%q branch=%q, want all empty", f.Title, f.Created, f.Branch)
	}
}

// TestAdvanceCommentEdges pins every boundary the block tracker has to get right. Each case is
// the shape of a real hazard, not a permutation for its own sake.
func TestAdvanceCommentEdges(t *testing.T) {
	cases := []struct {
		name string
		line string
		in   bool
		want bool
	}{
		{"a plain key line opens nothing", "- **phase:** plan", false, false},
		{"a trailing comment opens AND closes on one line", "- **phase:** plan <!-- a | b -->", false, false},
		{"an opener with no closer stays open", "<!-- legend starts here", false, true},
		{"the closer ends the block", "-->", true, false},
		{"a line fully inside a block stays inside", "- **correlation:** [plan-XXXX] example", true, true},
		{"a stray closer with nothing open changes nothing", "- **phase:** plan -->", false, false},
		{"two comments on one line both close", "a <!-- x --> b <!-- y --> c", false, false},
		{"the second comment is left open", "a <!-- x --> b <!-- y", false, true},
		{"a closer then a new opener leaves it open", "--> tail <!-- again", true, true},
		{"empty line carries the state through", "", true, true},
		{"empty line outside stays outside", "", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := advanceComment(c.line, c.in); got != c.want {
				t.Errorf("advanceComment(%q, in=%v) = %v, want %v", c.line, c.in, got, c.want)
			}
		})
	}
}

// TestParseStateFileCommentBlocks pins the parse-level consequences of the block tracker: what
// a commented block hides, what it must NOT hide, and the documented unterminated-opener call.
func TestParseStateFileCommentBlocks(t *testing.T) {
	write := func(t *testing.T, body string) *Feature {
		t.Helper()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "state.md"), []byte(body), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		return parseStateFile(filepath.Join(dir, "state.md"))
	}

	t.Run("a genuine correlation line outside any comment still parses", func(t *testing.T) {
		f := write(t, "- **phase:** plan\n- **correlation:** [plan-7f3a, plan-9c2e]\n")
		if len(f.Correlations) != 2 || f.Correlations[0] != "plan-7f3a" || f.Correlations[1] != "plan-9c2e" {
			t.Errorf("Correlations = %#v, want exactly [plan-7f3a plan-9c2e] - the fix must not blind the "+
				"parser to REAL correlation lines", f.Correlations)
		}
	})

	t.Run("a key inside a multi-line block is ignored", func(t *testing.T) {
		f := write(t, "- **phase:** plan\n<!-- example:\n- **correlation:** [plan-XXXX], e.g. [plan-7f3a, plan-9c2e]\n-->\n- **status:** plan-accepted\n")
		if len(f.Correlations) != 0 {
			t.Errorf("Correlations = %#v, want none", f.Correlations)
		}
		// The keys on BOTH sides of the block must survive - the block must not eat the file.
		if f.Phase != "plan" || f.Status != "plan-accepted" {
			t.Errorf("phase=%q status=%q, want plan/plan-accepted", f.Phase, f.Status)
		}
	})

	t.Run("a single-line trailing comment is byte-for-byte unchanged", func(t *testing.T) {
		f := write(t, "- **phase:** plan            <!-- plan | implement | review | test -->\n"+
			"- **iterations:** plan=1 · implement=2   <!-- add · uat=N -->\n")
		if f.Phase != "plan" {
			t.Errorf("phase = %q, want plan", f.Phase)
		}
		if f.Iterations != "plan=1 · implement=2" {
			t.Errorf("iterations = %q, want the value with its trailing comment trimmed", f.Iterations)
		}
	})

	t.Run("an unterminated opener hides what FOLLOWS it and nothing before", func(t *testing.T) {
		f := write(t, "- **phase:** implement\n- **status:** implementing\n<!-- truncated write\n- **correlation:** [plan-7f3a]\n- **created:** 2026-01-01\n")
		if f.Phase != "implement" || f.Status != "implementing" {
			t.Errorf("phase=%q status=%q - keys BEFORE an unterminated `<!--` must still parse, so a "+
				"truncated write loses only what follows it", f.Phase, f.Status)
		}
		if len(f.Correlations) != 0 || f.Created != "" {
			t.Errorf("Correlations=%#v created=%q - an unterminated opener comments out the rest of the "+
				"file (what a markdown renderer shows), so these must be missing rather than wrong",
				f.Correlations, f.Created)
		}
	})

	t.Run("a key whose trailing comment OPENS a multi-line block still parses", func(t *testing.T) {
		// The line carries a real key AND opens a comment that closes two lines later. The key
		// is outside the comment, so it must parse - which is why the parser reads the
		// "was I already inside a comment" flag as it CARRIES IN, not as it stands after the
		// line has been consumed. A mutation that swapped those two survived every other test.
		f := write(t, "- **status:** implementing   <!-- a long note that\n     wraps across lines\n     and closes here -->\n- **phase:** implement\n")
		if f.Status != "implementing" {
			t.Errorf("status = %q, want implementing - a key line that OPENS a trailing multi-line "+
				"comment must still yield its value", f.Status)
		}
		if f.Phase != "implement" {
			t.Errorf("phase = %q, want implement - the keys after the block must resume", f.Phase)
		}
	})

	t.Run("a stray closer does not enable a comment", func(t *testing.T) {
		f := write(t, "--> stray\n- **phase:** review\n")
		if f.Phase != "review" {
			t.Errorf("phase = %q, want review - a `-->` with nothing open must not change the parse", f.Phase)
		}
	})
}

// TestLoadRepoDerivesAuthoring is the integration probe from the plan's acceptance signal: a
// folder with a template state.md and NO plan.md reads as authoring, is not a user gate, and
// still classifies unfinished (the four classes and the class→column mapping are UNCHANGED -
// this is a derived display state, not a new class).
func TestLoadRepoDerivesAuthoring(t *testing.T) {
	root := t.TempDir()
	mk := func(slug string, planSections int, lines ...string) {
		dir := filepath.Join(root, ".gogo", "work", "feature-"+slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if len(lines) > 0 {
			writeState(t, dir, lines...)
		}
		if planSections > 0 {
			writePlan(t, dir, planSections)
		}
	}
	mk("scaffold", 0, "- **phase:** plan", "- **status:** awaiting-plan-acceptance", "- **created:** 2026-07-01")
	mk("stub", 1, "- **phase:** plan", "- **status:** awaiting-plan-acceptance", "- **created:** 2026-07-02")
	mk("written", 8, "- **phase:** plan", "- **status:** awaiting-plan-acceptance", "- **created:** 2026-07-03")
	mk("nostate", 0) // no state.md at all

	repo, err := LoadRepo(root)
	if err != nil {
		t.Fatalf("LoadRepo: %v", err)
	}
	cases := []struct {
		slug          string
		wantAuthoring bool
		wantWaiting   bool
	}{
		{"scaffold", true, false},
		{"stub", true, false},
		{"written", false, true},
		{"nostate", true, false},
	}
	for _, c := range cases {
		f := repo.Feature(c.slug)
		if f == nil {
			t.Fatalf("feature %q missing", c.slug)
		}
		if got := f.Authoring(); got != c.wantAuthoring {
			t.Errorf("%s: Authoring() = %v, want %v (PlanUnwritten=%v status=%q)",
				c.slug, got, c.wantAuthoring, f.PlanUnwritten, f.Status)
		}
		if got := f.WaitingForInput(); got != c.wantWaiting {
			t.Errorf("%s: WaitingForInput() = %v, want %v", c.slug, got, c.wantWaiting)
		}
		if f.Class != ClassUnfinished {
			t.Errorf("%s: Class = %q, want %q - the classifier must be UNCHANGED", c.slug, f.Class, ClassUnfinished)
		}
		if f.Column() != ColPlan {
			t.Errorf("%s: Column = %q, want %q", c.slug, f.Column(), ColPlan)
		}
	}
}

// TestLoadRepoPlanUnwrittenOnEveryClass: PlanUnwritten is derived for every feature, not
// just plan-column ones - FR8's `gogo go` refusal reads it on a plan-accepted feature, whose
// class is also unfinished, and the drill note reads it on any plan-column card.
func TestLoadRepoPlanUnwrittenOnEveryClass(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".gogo", "work", "feature-accepted")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeState(t, dir, "- **phase:** plan", "- **status:** plan-accepted", "- **created:** 2026-07-01")

	repo, _ := LoadRepo(root)
	f := repo.Feature("accepted")
	if f == nil {
		t.Fatal("accepted feature missing")
	}
	if !f.PlanUnwritten {
		t.Error("PlanUnwritten = false for a plan-accepted feature with NO plan.md - FR8 has nothing to refuse on")
	}
	if f.Authoring() {
		t.Error("Authoring() = true at plan-accepted - that is FR8's case, a different refusal")
	}
	// And with the plan written, the flag clears.
	writePlan(t, dir, 8)
	repo, _ = LoadRepo(root)
	if repo.Feature("accepted").PlanUnwritten {
		t.Error("PlanUnwritten = true once plan.md is written - the signal is not derived at read time")
	}
}
