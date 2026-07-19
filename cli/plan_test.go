package main

import (
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/plans"
)

// stubPlanLauncher swaps the spawn launch seam for a fake, restored after the test.
func stubPlanLauncher(t *testing.T, fn func(root string, in launch.Intent) (launch.Result, error)) {
	t.Helper()
	orig := planLauncher
	planLauncher = fn
	t.Cleanup(func() { planLauncher = orig })
}

// TestCmdPlanNewShowReadyDelete: the project-scoped plan CRUD walks the lifecycle
// through the CLI (writes ~/.gogo/ only).
func TestCmdPlanNewShowReadyDelete(t *testing.T) {
	seedDataHome(t)
	seedPlanProject(t, "web", "/repos/web")

	if code := cmdPlanStore([]string{"new", "Token migration", "--desc", "move the store"}); code != 0 {
		t.Fatalf("plan new: exit %d, want 0", code)
	}
	list, _ := plans.List("web")
	if len(list) != 1 || list[0].Status != plans.StatusDraft {
		t.Fatalf("after new: %+v, want one draft plan", list)
	}
	id := list[0].ID

	if code := cmdPlanStore([]string{"show", id}); code != 0 {
		t.Errorf("plan show: exit %d, want 0", code)
	}
	if code := cmdPlanStore([]string{"ready", id}); code != 0 {
		t.Fatalf("plan ready: exit %d, want 0", code)
	}
	if p, _ := plans.Get("web", id); p.Status != plans.StatusReady {
		t.Errorf("after ready: status = %q, want ready", p.Status)
	}

	out := FormatPlans("web", list)
	for _, want := range []string{id, "Token migration", "1 in"} {
		if !strings.Contains(out, want) {
			t.Errorf("FormatPlans missing %q:\n%s", want, out)
		}
	}

	if code := cmdPlanStore([]string{"delete", id}); code != 0 {
		t.Fatalf("plan delete: exit %d, want 0", code)
	}
	if l, _ := plans.List("web"); len(l) != 0 {
		t.Errorf("after delete: %d plans, want 0", len(l))
	}
}

// TestCmdPlanNewNeedsTitle: a titleless `new` is a usage error (exit 2).
func TestCmdPlanNewNeedsTitle(t *testing.T) {
	seedDataHome(t)
	seedPlanProject(t, "web", "/repos/web")
	if code := cmdPlanStore([]string{"new"}); code != 2 {
		t.Errorf("plan new (no title): exit %d, want 2", code)
	}
}

// TestCmdPlanPromoteSpawns pins FR11/FR15/D3: `gogo plan promote <id> <source>` fires
// the launch seam EXACTLY ONCE, anchored at the source root, with
// `/gogo:plan <body> --correlation plan-XXXX`; it records a member + activates the
// plan, and never writes the source's .gogo/.
func TestCmdPlanPromoteSpawns(t *testing.T) {
	seedDataHome(t)
	seedPlanProject(t, "web", "/repos/web")
	p, _ := plans.New("web", "Token migration", "move the shared token store")

	var calls int
	var gotRoot string
	var gotIntent launch.Intent
	stubPlanLauncher(t, func(root string, in launch.Intent) (launch.Result, error) {
		calls++
		gotRoot, gotIntent = root, in
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	})

	if code := cmdPlanStore([]string{"promote", p.ID, "web"}); code != 0 {
		t.Fatalf("plan promote: exit %d, want 0", code)
	}
	if calls != 1 {
		t.Fatalf("launcher fired %d times, want exactly 1", calls)
	}
	if gotRoot != "/repos/web" {
		t.Errorf("spawned in %q, want the source root /repos/web", gotRoot)
	}
	if !strings.HasPrefix(gotIntent.Command, "/gogo:plan move the shared token store") {
		t.Errorf("command = %q, want the plan body seeded whole", gotIntent.Command)
	}
	if !strings.HasSuffix(gotIntent.Command, "--correlation "+p.ID) {
		t.Errorf("command = %q, want the --correlation %s param", gotIntent.Command, p.ID)
	}
	if got, _ := plans.Get("web", p.ID); got.Status != plans.StatusActive || len(got.Members) != 1 {
		t.Errorf("after promote plan = %+v, want active with one member", got)
	}
}

// TestCmdPlanPromoteBadSource: an unknown source aborts before any launch.
func TestCmdPlanPromoteBadSource(t *testing.T) {
	seedDataHome(t)
	seedPlanProject(t, "web", "/repos/web")
	p, _ := plans.New("web", "Idea", "x")
	fired := false
	stubPlanLauncher(t, func(string, launch.Intent) (launch.Result, error) {
		fired = true
		return launch.Result{}, nil
	})
	if code := cmdPlanStore([]string{"promote", p.ID, "ghost"}); code != 1 {
		t.Errorf("promote into an unknown source: exit %d, want 1", code)
	}
	if fired {
		t.Error("launcher fired for an unknown source (must not)")
	}
}
