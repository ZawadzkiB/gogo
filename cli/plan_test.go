package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/plans"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// writeCorrelatedFeature drops a source work item on disk under
// <root>/.gogo/work/feature-<slug>/state.md carrying `correlation: [<planID>]` — the
// out-of-band spawn the plans-tab guards on but no member records (REV-002 fixture).
func writeCorrelatedFeature(t *testing.T, root, slug, planID string) {
	t.Helper()
	dir := filepath.Join(root, ".gogo", "work", "feature-"+slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "- **feature:** " + slug + "\n" +
		"- **phase:** implement\n" +
		"- **status:** implementing\n" +
		"- **created:** 2026-07-22\n" +
		"- **correlation:** [" + planID + "]\n"
	if err := os.WriteFile(filepath.Join(dir, "state.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

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

// TestCmdPlanReadyFansOut pins the 0.25.0 FR2 CLI mirror: `gogo plan ready <id>` on a
// plan with targets fires the launch seam ONCE per un-spawned target (each carrying its
// per-source brief + `--correlation`, and `--skip-acceptance` for a plan-acceptance-skip
// source), records a member per target, and flips the plan active — the headless twin of
// the plans-tab `r` auto-spawn.
func TestCmdPlanReadyFansOut(t *testing.T) {
	seedDataHome(t)
	if _, err := projects.Add(projects.Project{Name: "app", Sources: []projects.Source{
		{Name: "web", Path: "/repos/web"},
		{Name: "api", Path: "/repos/api", PlanAcceptanceSkip: true},
	}}); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	body := `## Goal
Cross-repo rollout.

## Source briefs
### web
Do the web side.

### api
Do the api side.`
	p, _ := plans.New("app", "Rollout", body)
	plans.AddTarget("app", p.ID, "web")
	plans.AddTarget("app", p.ID, "api")

	var calls int
	cmds := map[string]string{}
	stubPlanLauncher(t, func(root string, in launch.Intent) (launch.Result, error) {
		calls++
		cmds[root] = in.Command
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	})

	if code := cmdPlanStore([]string{"ready", p.ID}); code != 0 {
		t.Fatalf("plan ready: exit %d, want 0", code)
	}
	if calls != 2 {
		t.Fatalf("launcher fired %d times, want exactly 2 (one per target)", calls)
	}
	if !strings.Contains(cmds["/repos/web"], "Do the web side") || !strings.Contains(cmds["/repos/web"], "--correlation "+p.ID) {
		t.Errorf("web spawn = %q, want its brief + --correlation", cmds["/repos/web"])
	}
	if !strings.Contains(cmds["/repos/api"], "--skip-acceptance") {
		t.Errorf("api (planAcceptanceSkip) spawn = %q, want --skip-acceptance", cmds["/repos/api"])
	}
	if strings.Contains(cmds["/repos/web"], "--skip-acceptance") {
		t.Errorf("web (no skip) spawn = %q, must not carry --skip-acceptance", cmds["/repos/web"])
	}
	if got, _ := plans.Get("app", p.ID); got.Status != plans.StatusActive || len(got.Members) != 2 {
		t.Errorf("after ready plan = %+v, want active with 2 members", got)
	}
}

// TestCmdPlanReadyTargetlessMarksReady pins the additive fallback: `gogo plan ready` on a
// TARGETLESS plan just advances draft → ready with ZERO launches.
func TestCmdPlanReadyTargetlessMarksReady(t *testing.T) {
	seedDataHome(t)
	seedPlanProject(t, "web", "/repos/web")
	p, _ := plans.New("web", "Solo idea", "x")

	fired := false
	stubPlanLauncher(t, func(string, launch.Intent) (launch.Result, error) {
		fired = true
		return launch.Result{}, nil
	})
	if code := cmdPlanStore([]string{"ready", p.ID}); code != 0 {
		t.Fatalf("plan ready (targetless): exit %d, want 0", code)
	}
	if fired {
		t.Error("targetless ready fired the launcher (want zero launches)")
	}
	if got, _ := plans.Get("web", p.ID); got.Status != plans.StatusReady {
		t.Errorf("targetless ready: status = %q, want ready", got.Status)
	}
}

// TestCmdPlanReadyIdempotent pins the idempotency: a re-run skips a target that already
// has a member (never re-launching it).
func TestCmdPlanReadyIdempotent(t *testing.T) {
	seedDataHome(t)
	if _, err := projects.Add(projects.Project{Name: "app", Sources: []projects.Source{
		{Name: "web", Path: "/repos/web"},
		{Name: "api", Path: "/repos/api"},
	}}); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	p, _ := plans.New("app", "Rollout", "body")
	plans.AddTarget("app", p.ID, "web")
	plans.AddTarget("app", p.ID, "api")
	plans.AddMember("app", p.ID, plans.Member{Source: "web", SlugHint: "rollout"}) // web already spawned

	var roots []string
	stubPlanLauncher(t, func(root string, in launch.Intent) (launch.Result, error) {
		roots = append(roots, root)
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	})
	if code := cmdPlanStore([]string{"ready", p.ID}); code != 0 {
		t.Fatalf("plan ready: exit %d, want 0", code)
	}
	if len(roots) != 1 || roots[0] != "/repos/api" {
		t.Fatalf("fired into %v, want exactly [/repos/api] (web skipped)", roots)
	}
}

// TestCmdPlanReadyIdempotentOnBoardFeature pins REV-002: `gogo plan ready` also skips a
// target already spawned OUT OF BAND — a work item in the source stamped with the plan's
// correlation id but NEVER recorded as a member — matching the plans-tab member-OR-feature
// guard, so a re-run never re-launches a duplicate into that source.
func TestCmdPlanReadyIdempotentOnBoardFeature(t *testing.T) {
	seedDataHome(t)
	webRoot := t.TempDir()
	apiRoot := t.TempDir()
	if _, err := projects.Add(projects.Project{Name: "app", Sources: []projects.Source{
		{Name: "web", Path: webRoot},
		{Name: "api", Path: apiRoot},
	}}); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	p, _ := plans.New("app", "Rollout", "body")
	plans.AddTarget("app", p.ID, "web")
	plans.AddTarget("app", p.ID, "api")
	// web already carries a correlated work item on disk (no recorded member); api is fresh.
	writeCorrelatedFeature(t, webRoot, "rollout", p.ID)

	var roots []string
	stubPlanLauncher(t, func(root string, in launch.Intent) (launch.Result, error) {
		roots = append(roots, root)
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	})
	if code := cmdPlanStore([]string{"ready", p.ID}); code != 0 {
		t.Fatalf("plan ready: exit %d, want 0", code)
	}
	if len(roots) != 1 || roots[0] != apiRoot {
		t.Fatalf("fired into %v, want exactly [%s] (web already spawned out of band, skipped)", roots, apiRoot)
	}
}

// TestCmdPlanReadyInvalidTargetsReported pins REV-003: when spawned==0 is caused by an
// unresolved target (a targets: entry that is not a source), `gogo plan ready` names it
// on stderr and exits non-zero — it must NOT print the misleading "all targets already
// spawned - nothing to do" success (the old swallow), and it must not launch or mutate.
func TestCmdPlanReadyInvalidTargetsReported(t *testing.T) {
	seedDataHome(t)
	if _, err := projects.Add(projects.Project{Name: "app", Sources: []projects.Source{
		{Name: "web", Path: "/repos/web"},
	}}); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	p, _ := plans.New("app", "Rollout", "body")
	plans.AddTarget("app", p.ID, "web")   // valid, but already spawned (member below)
	plans.AddTarget("app", p.ID, "ghost") // NOT a source
	plans.AddMember("app", p.ID, plans.Member{Source: "web", SlugHint: "rollout"})

	fired := false
	stubPlanLauncher(t, func(string, launch.Intent) (launch.Result, error) {
		fired = true
		return launch.Result{}, nil
	})
	out, code := captureStderr(t, func() int { return cmdPlanStore([]string{"ready", p.ID}) })
	if code == 0 {
		t.Errorf("plan ready with an unresolved target: exit 0, want non-zero (must not misreport 'already spawned')")
	}
	if fired {
		t.Error("an invalid target fired the launcher (must not)")
	}
	if !strings.Contains(out, "ghost") || !strings.Contains(out, "unresolved") {
		t.Errorf("stderr = %q, want it to name the unresolved target 'ghost'", out)
	}
	if got, _ := plans.Get("app", p.ID); got.Status == plans.StatusActive || len(got.Members) != 1 {
		t.Errorf("invalid/already-spawned run mutated the plan: %+v", got)
	}
}

// TestCmdPlanReadyLaunchFailuresReported pins TEST-001: when spawned==0 is caused by a
// genuine LAUNCH FAILURE (planLauncher returns err for every un-spawned target — no claude
// on PATH, tmux down, etc.) rather than idempotency, `gogo plan ready` must exit non-zero
// and name the failed target(s) on stderr — it must NOT print the misleading "all targets
// already spawned - nothing to do" success (which would tell a CI $? check zero work items
// were actually created). The plan file stays unmutated (no phantom member; REV-005).
func TestCmdPlanReadyLaunchFailuresReported(t *testing.T) {
	seedDataHome(t)
	if _, err := projects.Add(projects.Project{Name: "app", Sources: []projects.Source{
		{Name: "web", Path: "/repos/web"},
		{Name: "api", Path: "/repos/api"},
	}}); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	p, _ := plans.New("app", "Rollout", "body")
	plans.AddTarget("app", p.ID, "web")
	plans.AddTarget("app", p.ID, "api")

	var calls int
	stubPlanLauncher(t, func(string, launch.Intent) (launch.Result, error) {
		calls++
		return launch.Result{}, errors.New("claude CLI not found on PATH")
	})
	out, code := captureStderr(t, func() int { return cmdPlanStore([]string{"ready", p.ID}) })
	if code == 0 {
		t.Errorf("plan ready with only launch failures: exit 0, want non-zero (must not misreport 'already spawned')")
	}
	if calls != 2 {
		t.Errorf("launcher fired %d times, want 2 (one attempt per un-spawned target)", calls)
	}
	if strings.Contains(out, "nothing to do") {
		t.Errorf("stderr = %q, must NOT print the 'already spawned - nothing to do' success line when launches actually failed", out)
	}
	if !strings.Contains(out, "web") || !strings.Contains(out, "api") || !strings.Contains(out, "failed to launch") {
		t.Errorf("stderr = %q, want it to name the failed targets web + api", out)
	}
	// REV-005: a failed launch records no member and leaves the plan a draft (no phantom).
	if got, _ := plans.Get("app", p.ID); got.Status == plans.StatusActive || len(got.Members) != 0 {
		t.Errorf("failed run mutated the plan: %+v, want draft with zero members", got)
	}
}

// TestCmdPlanReadyMissingProjectSources pins REV-003 (the load path): a targeted plan in
// a project with NO sources cannot resolve any target, so `gogo plan ready` surfaces a
// clear error + non-zero rather than swallowing an empty source set into the false
// "all already spawned" summary.
func TestCmdPlanReadyMissingProjectSources(t *testing.T) {
	seedDataHome(t)
	if _, err := projects.Add(projects.Project{Name: "app"}); err != nil { // project, zero sources
		t.Fatalf("seed project: %v", err)
	}
	p, _ := plans.New("app", "Rollout", "body")
	plans.AddTarget("app", p.ID, "web")

	fired := false
	stubPlanLauncher(t, func(string, launch.Intent) (launch.Result, error) {
		fired = true
		return launch.Result{}, nil
	})
	out, code := captureStderr(t, func() int { return cmdPlanStore([]string{"ready", p.ID}) })
	if code == 0 {
		t.Errorf("plan ready in a source-less project: exit 0, want non-zero")
	}
	if fired {
		t.Error("a source-less project fired the launcher (must not)")
	}
	if !strings.Contains(out, "no sources") {
		t.Errorf("stderr = %q, want a 'no sources' error", out)
	}
}
