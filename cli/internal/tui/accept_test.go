package tui

import (
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
)

// TestAcceptMoveGuard pins FR-C2: an awaiting-plan-acceptance card (ClassUnfinished)
// routes `m` to ActionAccept (/gogo:accept), closing the dead end where it bounced
// into /gogo:go — while a plan-accepted card (also ClassUnfinished) still routes to
// ActionGo. The guard branches on status, not class, because both share the class.
func TestAcceptMoveGuard(t *testing.T) {
	m := newModel(t)
	apa := &contract.Feature{Slug: "plan-pending", Status: "awaiting-plan-acceptance", Class: contract.ClassUnfinished}
	pa := &contract.Feature{Slug: "accepted", Status: "plan-accepted", Class: contract.ClassUnfinished}
	m.cols[0] = []*contract.Feature{apa, pa}
	m.colIdx = 0

	m.cardIdx[0] = 0
	in, ship, bounce := m.attemptAction(false)
	if bounce != "" || ship || in.Action != launch.ActionAccept {
		t.Fatalf("plan-pending m: intent=%+v ship=%v bounce=%q, want ActionAccept", in, ship, bounce)
	}
	if in.Command != "/gogo:accept plan-pending" {
		t.Errorf("accept command = %q", in.Command)
	}

	m.cardIdx[0] = 1
	in, _, bounce = m.attemptAction(false)
	if bounce != "" || in.Action != launch.ActionGo {
		t.Errorf("plan-accepted m: intent=%+v bounce=%q, want ActionGo (still)", in, bounce)
	}
}

// TestAcceptSessionAttribution: a live /gogo:accept session is attributed to its
// card (so `a` attach / `l` peek work), but liveness is a SEPARATE signal from the
// status pill — the badge stays the true gate status (awaiting-plan-acceptance),
// never "running" (FR-C1; running-vs-status decoupling).
func TestAcceptSessionAttribution(t *testing.T) {
	sessions := []string{"gogo-accept-plan-pending"}
	if !hasLiveSession("plan-pending", sessions) {
		t.Errorf("gogo-accept session not attributed to its slug")
	}
	f := &contract.Feature{Slug: "plan-pending", Phase: "plan", Status: "awaiting-plan-acceptance"}
	if got := badge(f); got != "awaiting-plan-acceptance" {
		t.Errorf("badge should be the true gate status, not the session; got %q", got)
	}
	if got := pillLabel(f); got != waitingMarker+" accept plan" {
		t.Errorf("pillLabel = %q, want the accept-plan chip", got)
	}
}
