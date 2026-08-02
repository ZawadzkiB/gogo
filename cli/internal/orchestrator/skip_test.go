package orchestrator

import "testing"

// TestGoSkipSuffix pins the FR4 gate-skip suffix + the plan-leg invariant (REV-001): the
// `go` leg renders the exact ` --skip-acceptance`/` --skip-uat` tokens per flag combination
// (and "" when neither is set — byte-for-byte today's command), while the `plan` leg ALWAYS
// returns "" even with both flags set, so /gogo:plan never carries the params and
// --correlation stays its final token.
func TestGoSkipSuffix(t *testing.T) {
	cases := []struct {
		name              string
		kind              string
		planSkip, uatSkip bool
		fast              bool
		want              string
	}{
		{"go neither → byte-for-byte", "go", false, false, false, ""},
		{"go plan-only", "go", true, false, false, " --skip-acceptance"},
		{"go uat-only", "go", false, true, false, " --skip-uat"},
		{"go both", "go", true, true, false, " --skip-acceptance --skip-uat"},
		// Fast mode: its param renders after the gate-skip params (orthogonal flags).
		{"go fast-only", "go", false, false, true, " --fast"},
		{"go both + fast", "go", true, true, true, " --skip-acceptance --skip-uat --fast"},
		// The plan-leg invariant: never carries the params, whatever the flags.
		{"plan both (invariant)", "plan", true, true, false, ""},
		{"plan plan-only (invariant)", "plan", true, false, false, ""},
		{"plan uat-only (invariant)", "plan", false, true, false, ""},
		{"plan fast (invariant)", "plan", false, false, true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &Session{Kind: c.kind, SkipAcceptance: c.planSkip, SkipUAT: c.uatSkip, Fast: c.fast}
			if got := s.goSkipSuffix(); got != c.want {
				t.Errorf("goSkipSuffix(kind=%s plan=%v uat=%v fast=%v) = %q, want %q", c.kind, c.planSkip, c.uatSkip, c.fast, got, c.want)
			}
		})
	}
}
