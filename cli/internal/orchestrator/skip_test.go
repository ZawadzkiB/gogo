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
		want              string
	}{
		{"go neither → byte-for-byte", "go", false, false, ""},
		{"go plan-only", "go", true, false, " --skip-acceptance"},
		{"go uat-only", "go", false, true, " --skip-uat"},
		{"go both", "go", true, true, " --skip-acceptance --skip-uat"},
		// The plan-leg invariant: never carries the params, whatever the flags.
		{"plan both (invariant)", "plan", true, true, ""},
		{"plan plan-only (invariant)", "plan", true, false, ""},
		{"plan uat-only (invariant)", "plan", false, true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &Session{Kind: c.kind, SkipAcceptance: c.planSkip, SkipUAT: c.uatSkip}
			if got := s.goSkipSuffix(); got != c.want {
				t.Errorf("goSkipSuffix(kind=%s plan=%v uat=%v) = %q, want %q", c.kind, c.planSkip, c.uatSkip, got, c.want)
			}
		})
	}
}
