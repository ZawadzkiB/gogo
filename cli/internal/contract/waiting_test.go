package contract

import "testing"

// TestWaitingForInput pins the Slice-B predicate (unattended-ops-input-signals,
// FR-B1): exactly the three genuine user gates are "waiting for input"; every
// other status flows unattended.
func TestWaitingForInput(t *testing.T) {
	waiting := []string{"awaiting-plan-acceptance", "waiting-for-user", "awaiting-uat"}
	for _, s := range waiting {
		if !(&Feature{Status: s}).WaitingForInput() {
			t.Errorf("WaitingForInput(%q) = false, want true (user gate)", s)
		}
	}
	auto := []string{"plan-accepted", "implementing", "reviewing", "testing", "done", "shipped", "aborted", ""}
	for _, s := range auto {
		if (&Feature{Status: s}).WaitingForInput() {
			t.Errorf("WaitingForInput(%q) = true, want false (flows unattended)", s)
		}
	}
}
