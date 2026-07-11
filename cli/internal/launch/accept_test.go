package launch

import "testing"

// TestBuildIntentAccept pins FR-C1: ActionAccept resolves to /gogo:accept <slug>
// and a gogo-accept-<slug> session, attributable back to its card (a/l), with the
// same TEST-005 exact-boundary matching as go/done.
func TestBuildIntentAccept(t *testing.T) {
	in := BuildIntent(ActionAccept, []string{"my-feature"}, "")
	if in.Command != "/gogo:accept my-feature" {
		t.Errorf("command = %q", in.Command)
	}
	if in.Session != "gogo-accept-my-feature" {
		t.Errorf("session = %q", in.Session)
	}
	if !SessionMatchesSlug("gogo-accept-my-feature", "my-feature") {
		t.Errorf("accept session not attributable to its slug")
	}
	// The uniqueSession collision suffix still attributes to its OWN slug…
	if !SessionMatchesSlug("gogo-accept-my-feature-2", "my-feature") {
		t.Errorf("suffixed accept session not attributed")
	}
	// …but never to a different slug that is a textual substring (TEST-005).
	if SessionMatchesSlug("gogo-accept-awaiting-card", "waiting-card") {
		t.Errorf("accept session cross-attributed to a substring slug")
	}
}
