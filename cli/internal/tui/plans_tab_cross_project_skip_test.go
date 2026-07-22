package tui

import (
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/plans"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// TestPlansTabAcceptSpawnSkipScopedToFocusedProject pins REV-001 (smart-project-plans,
// 0.25.0): on the UNIFIED cockpit board, TWO projects can each link a source at the SAME
// repo path with OPPOSITE PlanAcceptanceSkip flags (a repo shared across two gogo
// projects). finishPlanSpawn's `r` auto-spawn must ride the FOCUSED project's OWN flag
// for that path — never the other project's — because it resolves the skip through
// m.sourceByName, which is scoped to m.project.Sources (the single focused Project), not
// a flattened cross-project lookup. A cross-project bleed here would silently apply the
// wrong project's --skip-acceptance to a spawned work item's plan-acceptance gate.
func TestPlansTabAcceptSpawnSkipScopedToFocusedProject(t *testing.T) {
	seedDataHome(t)
	const sharedPath = "/repos/shared"

	alpha := projects.Project{Name: "alpha", Sources: []projects.Source{
		{Name: "shared", Path: sharedPath, PlanAcceptanceSkip: true}, // alpha opts OUT
	}}
	beta := projects.Project{Name: "beta", Sources: []projects.Source{
		{Name: "shared", Path: sharedPath, PlanAcceptanceSkip: false}, // beta does NOT opt out
	}}

	// A plan in EACH project, both targeting the same-named "shared" source.
	pAlpha, _ := plans.New("alpha", "Alpha rollout", "alpha body")
	plans.AddTarget("alpha", pAlpha.ID, "shared")
	pBeta, _ := plans.New("beta", "Beta rollout", "beta body")
	plans.AddTarget("beta", pBeta.ID, "shared")

	run := func(project string, focusIdx int, planID string) string {
		m := NewWorkspaceAll(&contract.Repo{}, []projects.Project{alpha, beta})
		m.hasClaude = true
		m.tab = tabPlans
		m.project = &m.allProjects[focusIdx] // focus the named project (never index 0 by accident)

		var gotCommand string
		m.launcher = func(root string, in launch.Intent) (launch.Result, error) {
			gotCommand = in.Command
			return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
		}

		p, ok := plans.Get(project, planID)
		if !ok {
			t.Fatalf("plan %s not found in %s", planID, project)
		}
		m.pendingPlanSpawn = &planSpawnEdit{project: project, id: p.ID, title: p.Title, targets: []string{"shared"}}
		m.binding = &formBinding{confirm: true}
		fm, cmd := m.finishPlanSpawn()
		_ = fm
		if cmd == nil {
			t.Fatalf("finishPlanSpawn(%s) returned a nil cmd", project)
		}
		cmd()
		return gotCommand
	}

	betaCmd := run("beta", 1, pBeta.ID)
	if strings.Contains(betaCmd, "--skip-acceptance") {
		t.Errorf("beta (PlanAcceptanceSkip=false) spawn carries --skip-acceptance — bled alpha's flag: %q", betaCmd)
	}

	alphaCmd := run("alpha", 0, pAlpha.ID)
	if !strings.Contains(alphaCmd, "--skip-acceptance") {
		t.Errorf("alpha (PlanAcceptanceSkip=true) spawn missing --skip-acceptance: %q", alphaCmd)
	}
}
