package contract

import (
	"path/filepath"
	"testing"
)

func testRepo(t *testing.T) *Repo {
	t.Helper()
	r, err := LoadRepo(filepath.Join("testdata", "repo"))
	if err != nil {
		t.Fatalf("LoadRepo: %v", err)
	}
	return r
}

func TestClassifier(t *testing.T) {
	r := testRepo(t)
	want := map[string]string{
		"shipped-status":     ClassShipped,
		"shipped-by-folder":  ClassShipped, // changelog wins over its own report
		"shipped-by-members": ClassShipped,
		"ready":              ClassReadyToShip,
		"legacy-ready":       ClassReadyToShip,
		"inprogress":         ClassInProgress,
		"unfinished":         ClassUnfinished,
		"aborted":            ClassUnfinished, // aborted reports as unfinished
		"malformed":          ClassUnfinished, // garbage state → default class, no crash
	}
	if len(r.Features) != len(want) {
		t.Fatalf("feature count = %d, want %d", len(r.Features), len(want))
	}
	for _, f := range r.Features {
		if got := want[f.Slug]; got != f.Class {
			t.Errorf("%s: class = %q, want %q", f.Slug, f.Class, got)
		}
	}
}

func TestChangelogResolution(t *testing.T) {
	r := testRepo(t)
	byFolder := r.Feature("shipped-by-folder")
	if byFolder.ChangelogPath == "" || filepath.Base(byFolder.ChangelogPath) != "2026-06-18-shipped-by-folder" {
		t.Errorf("shipped-by-folder changelog = %q", byFolder.ChangelogPath)
	}
	byMembers := r.Feature("shipped-by-members")
	if byMembers.ChangelogPath == "" || filepath.Base(byMembers.ChangelogPath) != "2026-06-17-big-release" {
		t.Errorf("shipped-by-members changelog = %q (members[] fallback failed)", byMembers.ChangelogPath)
	}
	// A feature with no changelog entry has an empty path.
	if r.Feature("ready").ChangelogPath != "" {
		t.Errorf("ready should have no changelog path")
	}
}

func TestReportDetection(t *testing.T) {
	r := testRepo(t)
	if p := r.Feature("ready").ReportPath; filepath.Base(p) != "report.md" || filepath.Base(filepath.Dir(p)) != "report" {
		t.Errorf("ready report path = %q, want report/report.md", p)
	}
	if p := r.Feature("legacy-ready").ReportPath; filepath.Base(p) != "report.md" || filepath.Base(filepath.Dir(p)) == "report" {
		t.Errorf("legacy-ready report path = %q, want legacy root report.md", p)
	}
	if p := r.Feature("unfinished").ReportPath; p != "" {
		t.Errorf("unfinished should have no report, got %q", p)
	}
}

func TestColumnMapping(t *testing.T) {
	cases := map[string]string{
		ClassUnfinished:  ColPlan,
		ClassInProgress:  ColInProgress,
		ClassReadyToShip: ColReady,
		ClassShipped:     ColChangelog,
	}
	for class, col := range cases {
		if got := Column(class); got != col {
			t.Errorf("Column(%q) = %q, want %q", class, got, col)
		}
	}
}

func TestStateParsing(t *testing.T) {
	r := testRepo(t)
	f := r.Feature("inprogress")
	if f.Title != "In progress feature" {
		t.Errorf("title = %q", f.Title)
	}
	if f.Phase != "review" || f.Status != "reviewing" {
		t.Errorf("phase/status = %q/%q", f.Phase, f.Status)
	}
	if f.Iterations != "plan=1 · implement=2 · review=2 · test=0" {
		t.Errorf("iterations = %q", f.Iterations)
	}
	if f.Stage != "B of B" {
		t.Errorf("stage = %q", f.Stage)
	}
	// Trailing HTML comment must be stripped from phase (fixture has one).
	s := r.Feature("shipped-status")
	if s.Phase != "done" || s.Status != "shipped" {
		t.Errorf("comment strip failed: phase=%q status=%q", s.Phase, s.Status)
	}
	if s.OpenDecision != "none (D1=A · D2=B)" {
		t.Errorf("open-decision parenthetical not preserved: %q", s.OpenDecision)
	}
}

func TestMalformedStateNeverPanics(t *testing.T) {
	r := testRepo(t)
	f := r.Feature("malformed")
	if f == nil {
		t.Fatal("malformed feature missing")
	}
	if f.Phase != "" || f.Status != "" {
		t.Errorf("malformed state should yield empty enums, got phase=%q status=%q", f.Phase, f.Status)
	}
	if f.Class != ClassUnfinished {
		t.Errorf("malformed class = %q", f.Class)
	}
}

func TestNewestFirstSort(t *testing.T) {
	r := testRepo(t)
	for i := 1; i < len(r.Features); i++ {
		if r.Features[i-1].Created < r.Features[i].Created {
			t.Errorf("features not newest-first at %d: %q(%s) before %q(%s)",
				i, r.Features[i-1].Slug, r.Features[i-1].Created,
				r.Features[i].Slug, r.Features[i].Created)
		}
	}
}

func TestEventsParsing(t *testing.T) {
	evs := ReadEvents(filepath.Join("testdata", "repo", ".gogo", "work", "feature-inprogress", "events.jsonl"))
	if len(evs) != 4 {
		t.Fatalf("events = %d, want 4", len(evs))
	}
	if !evs[0].TSValid || evs[0].Event != "phase-started" {
		t.Errorf("first event = %+v", evs[0])
	}
	last := evs[len(evs)-1]
	if last.Event != "round-opened" || !last.HasRound || last.Round != 2 {
		t.Errorf("last event = %+v, want round-opened r2", last)
	}
	if evs[2].Note != "2 blockers, 1 minor" {
		t.Errorf("note = %q", evs[2].Note)
	}
}

func TestEventsLenientOnMalformed(t *testing.T) {
	// The malformed fixture has 5 lines: valid, non-json, bad-ts, broken, valid+unknown-field.
	// Malformed JSON lines are skipped; a bad ts keeps the event (TSValid=false);
	// an unknown field is ignored (forward-compat).
	evs := ReadEvents(filepath.Join("testdata", "repo", ".gogo", "work", "feature-malformed", "events.jsonl"))
	if len(evs) != 3 {
		t.Fatalf("kept events = %d, want 3 (2 json lines skipped)", len(evs))
	}
	if evs[1].Event != "round-opened" || evs[1].TSValid {
		t.Errorf("bad-ts event should be kept with TSValid=false: %+v", evs[1])
	}
	if evs[2].Event != "phase-done" || !evs[2].TSValid {
		t.Errorf("unknown-field event should parse fine: %+v", evs[2])
	}
}

func TestReadEventsMissing(t *testing.T) {
	if evs := ReadEvents(filepath.Join("testdata", "does-not-exist.jsonl")); evs != nil {
		t.Errorf("missing events.jsonl should be nil, got %v", evs)
	}
}

func TestLatestEventBadge(t *testing.T) {
	r := testRepo(t)
	f := r.Feature("inprogress")
	if f.LatestEvent == nil || f.LatestEvent.Event != "round-opened" || f.LatestEvent.Round != 2 {
		t.Errorf("latest event = %+v", f.LatestEvent)
	}
	if r.Feature("unfinished").LatestEvent != nil {
		t.Errorf("feature with no events.jsonl should have nil LatestEvent")
	}
}

func TestIssuesReading(t *testing.T) {
	list, err := ReadIssues(filepath.Join("testdata", "repo", ".gogo", "work", "feature-inprogress", "review", "issues.json"))
	if err != nil {
		t.Fatalf("ReadIssues: %v", err)
	}
	if list.Track != "review" || list.Round != 2 || len(list.Issues) != 2 {
		t.Fatalf("list = %+v", list)
	}
	if list.Issues[0].ID != "REV-001" || list.Issues[0].Status != "open" {
		t.Errorf("issue 0 = %+v", list.Issues[0])
	}
	if list.Issues[1].FixedInRound != 2 || list.Issues[1].FixSummary != "done" {
		t.Errorf("issue 1 fix fields = %+v", list.Issues[1])
	}
}

func TestReadIssuesMissing(t *testing.T) {
	list, err := ReadIssues(filepath.Join("testdata", "nope", "issues.json"))
	if err != nil || list != nil {
		t.Errorf("missing issues should be (nil,nil), got (%v,%v)", list, err)
	}
}

func TestManifestReadingAndTitle(t *testing.T) {
	m, err := ReadManifest(filepath.Join("testdata", "repo", ".gogo", "work", "feature-inprogress", "charts", "manifest.json"))
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if got := m.TitleFor("flow"); got != "Intended flow" {
		t.Errorf("TitleFor(flow) = %q", got)
	}
	if got := m.TitleFor("missing"); got != "" {
		t.Errorf("TitleFor(missing) = %q, want empty", got)
	}
	// members[] on the merged changelog manifest.
	rm, _ := ReadManifest(filepath.Join("testdata", "repo", ".gogo", "changelog", "2026-06-17-big-release", "manifest.json"))
	if len(rm.Members) != 2 || rm.Members[0] != "shipped-by-members" {
		t.Errorf("members = %v", rm.Members)
	}
}

func TestListDiagrams(t *testing.T) {
	got := ListDiagrams(filepath.Join("testdata", "repo", ".gogo", "work", "feature-inprogress", "charts"))
	if len(got) != 1 || filepath.Base(got[0]) != "flow.mmd" {
		t.Errorf("diagrams = %v (manifest.json must be skipped)", got)
	}
}

func TestArtifactsListing(t *testing.T) {
	r := testRepo(t)
	arts := Artifacts(r.Feature("inprogress"))
	labels := make([]string, len(arts))
	for i, a := range arts {
		labels[i] = a.Label
	}
	// plan, decisions, adjustments, review-01, review-02, review issues,
	// events timeline, charts/flow.mmd (no report → no report entries).
	want := []string{
		"plan.md", "decisions.md", "adjustments.md",
		"review-01.md", "review-02.md",
		"review/issues.json", "events (timeline)", "charts/flow.mmd",
	}
	if len(labels) != len(want) {
		t.Fatalf("artifacts = %v, want %v", labels, want)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Errorf("artifact %d = %q, want %q", i, labels[i], want[i])
		}
	}
}

func TestSlugFromArg(t *testing.T) {
	cases := []struct{ arg, slug, kind string }{
		{"cli-cockpit-and-events", "cli-cockpit-and-events", ""},
		{"cli-cockpit-and-events:plan", "cli-cockpit-and-events", "plan"},
		{"feature-cli-cockpit-and-events:report", "cli-cockpit-and-events", "report"},
	}
	for _, c := range cases {
		slug, kind := SlugFromArg(c.arg)
		if slug != c.slug || kind != c.kind {
			t.Errorf("SlugFromArg(%q) = (%q,%q), want (%q,%q)", c.arg, slug, kind, c.slug, c.kind)
		}
	}
}
