package tui

import (
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
)

// TestWaitingCardCue pins FR-B2: a card that WaitingForInput() carries the
// distinct ⏸ cue — including awaiting-plan-acceptance, which had none before —
// on both focused and unfocused cards; a flowing card carries none.
func TestWaitingCardCue(t *testing.T) {
	m := newModel(t)

	apa := &contract.Feature{
		Slug: "plan-pending", Title: "Plan pending",
		Phase: "plan", Status: "awaiting-plan-acceptance", Class: contract.ClassUnfinished,
	}
	out := m.renderCard(0, apa, false, 40)
	if !strings.Contains(out, waitingMarker) {
		t.Errorf("awaiting-plan-acceptance card missing the waiting cue %q:\n%s", waitingMarker, out)
	}
	// FR-3 folds ⏸ into the status pill, whose answer-first label reads
	// "accept plan" (badge() stays canonical — see TestBadgeAwaitingPlanAcceptance).
	if !strings.Contains(out, "accept plan") {
		t.Errorf("awaiting-plan-acceptance card missing its pill text:\n%s", out)
	}
	// The cue does not depend on focus.
	if focused := m.renderCard(0, apa, true, 40); !strings.Contains(focused, waitingMarker) {
		t.Errorf("focused waiting card lost the cue:\n%s", focused)
	}

	// A flowing (auto) card carries NO cue.
	flow := &contract.Feature{Slug: "building", Phase: "implement", Status: "implementing", Class: contract.ClassInProgress}
	if out := m.renderCard(1, flow, false, 40); strings.Contains(out, waitingMarker) {
		t.Errorf("flowing card wrongly shows the waiting cue:\n%s", out)
	}
}

// TestBadgeAwaitingPlanAcceptance pins FR-B2's badge change: a plan-pending card
// reads as its gate state name, not the misleading "plan r1" phase round.
func TestBadgeAwaitingPlanAcceptance(t *testing.T) {
	f := &contract.Feature{
		Slug: "p", Phase: "plan", Status: "awaiting-plan-acceptance",
		Iterations: "plan=1 · implement=0 · review=0 · test=0",
	}
	if got := badge(f); got != "awaiting-plan-acceptance" {
		t.Errorf("badge = %q, want awaiting-plan-acceptance (not the plan round)", got)
	}
}

// TestColumnSeparatorRendered pins FR-B4: the board draws a vertical separator
// between the four columns.
func TestColumnSeparatorRendered(t *testing.T) {
	m := newModel(t)
	if out := m.View(); !strings.Contains(out, "│") {
		t.Errorf("viewBoard did not render the column separator glyph:\n%s", out)
	}
}
