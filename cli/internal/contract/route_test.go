package contract

import (
	"os"
	"path/filepath"
	"testing"
)

// issuesWith builds an IssuesList whose issues have the given statuses and severity;
// the first issue's proposed_solution carries the NEEDS-USER-DECISION tag when needsDec.
func issuesWith(sev string, needsDec bool, statuses ...string) *IssuesList {
	if sev == "" {
		sev = "major"
	}
	list := &IssuesList{Slug: "x", Track: "review", Round: 1}
	for i, st := range statuses {
		is := Issue{ID: "REV-00" + string(rune('1'+i)), Status: st, Severity: sev, ProposedSolution: "do the thing"}
		if needsDec && i == 0 {
			is.ProposedSolution = "NEEDS-USER-DECISION: pick an approach"
		}
		list.Issues = append(list.Issues, is)
	}
	return list
}

func TestRouteTable(t *testing.T) {
	cases := []struct {
		name   string
		track  string
		result *PhaseResult
		issues *IssuesList
		want   RouteDecision
	}{
		{"nil/nil → advance", TrackReview, nil, nil, Advance},
		{"review: ok + no open → advance", TrackReview, &PhaseResult{Status: "ok"}, issuesWith("major", false), Advance},
		{"review: open major → re-implement", TrackReview, &PhaseResult{Status: "ok"}, issuesWith("major", false, "open"), ReImplement},
		{"review: new blocker → re-implement", TrackReview, &PhaseResult{Status: "ok"}, issuesWith("blocker", false, "new"), ReImplement},
		{"review: open MINOR is batched → advance", TrackReview, &PhaseResult{Status: "ok"}, issuesWith("minor", false, "open"), Advance},
		{"review: open NIT is batched → advance", TrackReview, &PhaseResult{Status: "ok"}, issuesWith("nit", false, "open"), Advance},
		{"test: open minor → re-implement (test routes on any)", TrackTest, &PhaseResult{Status: "ok"}, issuesWith("minor", false, "open"), ReImplement},
		{"test: open nit → re-implement", TrackTest, &PhaseResult{Status: "ok"}, issuesWith("nit", false, "open"), ReImplement},
		{"open needs-user-decision → gate", TrackReview, &PhaseResult{Status: "ok"}, issuesWith("major", true, "open"), Gate},
		{"needs-decision even in a batched minor → gate", TrackReview, &PhaseResult{Status: "ok"}, issuesWith("minor", true, "open"), Gate},
		{"result waiting-for-user → gate", TrackReview, &PhaseResult{Status: "waiting-for-user"}, issuesWith("major", false), Gate},
		{"result blocked → gate", TrackTest, &PhaseResult{Status: "blocked"}, issuesWith("major", false, "open"), Gate},
		{"all resolved → advance", TrackReview, &PhaseResult{Status: "ok"}, issuesWith("major", false, "fixed", "verified", "wontfix"), Advance},
	}
	for _, c := range cases {
		if got := Route(c.track, c.result, c.issues); got != c.want {
			t.Errorf("%s: Route(%s) = %v, want %v", c.name, c.track, got, c.want)
		}
	}
}

func TestNeedsUserDecisionScansAllFields(t *testing.T) {
	// The tag may land in title or description (the tester doesn't pin it to
	// proposed_solution) — Route must still gate (REV-004).
	inTitle := &IssuesList{Issues: []Issue{{ID: "TEST-1", Status: "open", Severity: "minor",
		Title: "NEEDS-USER-DECISION: which env?", ProposedSolution: "tbd"}}}
	if got := Route(TrackTest, &PhaseResult{Status: "ok"}, inTitle); got != Gate {
		t.Errorf("marker in title should gate, got %v", got)
	}
	inDesc := &IssuesList{Issues: []Issue{{ID: "TEST-2", Status: "open", Severity: "major",
		Description: "this is NEEDS-USER-DECISION territory", ProposedSolution: "tbd"}}}
	if got := Route(TrackReview, &PhaseResult{Status: "ok"}, inDesc); got != Gate {
		t.Errorf("marker in description should gate, got %v", got)
	}
}

func TestOpenIssueCount(t *testing.T) {
	// counts every open/new regardless of severity; falls back to result.open_issues.
	if n := OpenIssueCount(nil, issuesWith("nit", false, "open", "new", "fixed", "verified", "wontfix")); n != 2 {
		t.Errorf("open/new count = %d, want 2", n)
	}
	if n := OpenIssueCount(&PhaseResult{OpenIssues: 4}, nil); n != 4 {
		t.Errorf("fallback to result.open_issues = %d, want 4", n)
	}
	if n := OpenIssueCount(nil, nil); n != 0 {
		t.Errorf("no signal → %d, want 0", n)
	}
}

func TestReadResultMissing(t *testing.T) {
	r, err := ReadResult(filepath.Join(t.TempDir(), "nope", "result.json"))
	if err != nil || r != nil {
		t.Errorf("missing result.json → (nil,nil); got (%v,%v)", r, err)
	}
}

func TestReadResultParses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "result.json")
	if err := os.WriteFile(path, []byte(`{"slug":"x","phase":"review","status":"ok","inputs":[],"outputs":[],"validated_in":true,"validated_out":true,"open_issues":2,"summary":"s"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := ReadResult(path)
	if err != nil || r == nil {
		t.Fatalf("ReadResult: (%v,%v)", r, err)
	}
	if r.Status != "ok" || r.OpenIssues != 2 || r.Phase != "review" {
		t.Errorf("parsed = %+v", r)
	}
}
