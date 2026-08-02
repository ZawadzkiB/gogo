package launch

import "testing"

// TestSetFastParamRoundTrips pins SetFastParam's contract (the launch confirm's
// one command producer, FR1/FR4/FR5): idempotent in both directions and
// round-tripping on exactly the command shapes intentFor really produces —
// including one already carrying the gate-skip params.
func TestSetFastParamRoundTrips(t *testing.T) {
	for _, c := range []string{
		"/gogo:go wf",
		"/gogo:go wf --skip-acceptance --skip-uat",
		"/gogo:go wf --fast",
		"/gogo:go wf --skip-acceptance --skip-uat --fast",
	} {
		off := SetFastParam(c, false)
		if HasFastParam(off) {
			t.Errorf("SetFastParam(%q, false) = %q still carries --fast", c, off)
		}
		on := SetFastParam(off, true)
		if !HasFastParam(on) {
			t.Errorf("SetFastParam(%q, true) = %q lost --fast", off, on)
		}
		if again := SetFastParam(on, true); again != on {
			t.Errorf("SetFastParam not idempotent adding: %q -> %q", on, again)
		}
		if again := SetFastParam(off, false); again != off {
			t.Errorf("SetFastParam not idempotent removing: %q -> %q", off, again)
		}
	}

	// Round-trip equality on the producer's own outputs: the token is appended at
	// the END (FastParam's position), so off->on->off and on->off->on are exact.
	orig := "/gogo:go wf --skip-acceptance --skip-uat"
	if got := SetFastParam(SetFastParam(orig, true), false); got != orig {
		t.Errorf("off->on->off = %q, want %q", got, orig)
	}
	fast := orig + " --fast"
	if got := SetFastParam(SetFastParam(fast, false), true); got != fast {
		t.Errorf("on->off->on = %q, want %q", got, fast)
	}

	// One spelling: the config resolver's token (FastParam) IS the token the
	// per-launch pair matches, so the seed and the override can never drift.
	if !HasFastParam("/gogo:go wf" + FastParam(true)) {
		t.Error("FastParam(true)'s token is not recognised by HasFastParam - the spellings drifted")
	}
}

// TestFastParamMatchedExactlyNeverBySubstring is the TEST-005 rule applied to a
// token: a command whose text merely CONTAINS "fast" (a slug like `fast-path`, a
// longer flag like `--fastest`) is never read as fast mode and never modified.
func TestFastParamMatchedExactlyNeverBySubstring(t *testing.T) {
	for _, c := range []string{
		"/gogo:go fast-path",
		"/gogo:go fast",
		"/gogo:go build --fastest",
		"/gogo:done fast-path+other",
	} {
		if HasFastParam(c) {
			t.Errorf("HasFastParam(%q) = true - a substring match, want field-exact", c)
		}
		if got := SetFastParam(c, false); got != c {
			t.Errorf("SetFastParam(%q, false) = %q - modified a command carrying no --fast token", c, got)
		}
	}
}
