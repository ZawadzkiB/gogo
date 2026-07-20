package tui

import (
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// TestBoardGoIntentCarriesSkipParams (FR4, REV-001): the board's go-launch intent carries a
// flagged source's --skip-acceptance/--skip-uat params — resolved by the card's OWN root
// through projects.SkipForSource, the SAME resolver `gogo go` uses (so the board and the CLI
// never drift). An UNFLAGGED source's go is byte-for-byte today's command, and a non-go
// action (a ship) never carries the gate-skip params even for a flagged source.
func TestBoardGoIntentCarriesSkipParams(t *testing.T) {
	seedDataHome(t)
	flagged := projects.Source{Name: "web", Path: "/repos/web", PlanAcceptanceSkip: true, UatAcceptanceSkip: true}
	plain := projects.Source{Name: "api", Path: "/repos/api"}
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "wf", Source: "web", Root: "/repos/web", Class: contract.ClassUnfinished, Status: "plan-accepted"},
		{Slug: "af", Source: "api", Root: "/repos/api", Class: contract.ClassUnfinished, Status: "plan-accepted"},
	}}
	m := sizedWorkspace(t, repo, proj("app", flagged, plain))

	// Flagged source → the go command carries both skip params as trailing tokens.
	if in := m.intentFor(launch.ActionGo, m.repo.Feature("wf")); in.Command != "/gogo:go wf --skip-acceptance --skip-uat" {
		t.Errorf("flagged go command = %q, want the two skip tokens appended", in.Command)
	}
	// Unflagged source → byte-for-byte today's command (no drift, no params).
	if in := m.intentFor(launch.ActionGo, m.repo.Feature("af")); in.Command != "/gogo:go af" {
		t.Errorf("unflagged go command = %q, want no skip params", in.Command)
	}
	// A non-go action (a ship) never carries the gate-skip params, even for a flagged source.
	if in := m.intentFor(launch.ActionDone, m.repo.Feature("wf")); strings.Contains(in.Command, "--skip-") {
		t.Errorf("done command = %q, must never carry gate-skip params", in.Command)
	}
}
