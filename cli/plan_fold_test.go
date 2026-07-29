package main

import (
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/plans"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// --- REV-002: the headless doors fold an over-budget brief too ----------------
//
// `gogo plan go` / `gogo plan promote` build the IDENTICAL launch.PlanIntent the
// plans tab does, so they blow the identical tmux budget. Measured on the user's
// real shape: 20 951 bytes against the 16 317 limit. Without the fold the CLI
// door stayed unusable on realistic briefs while the cockpit worked - D1's
// rejected option B, arrived at by omission.

// hugeBrief is a per-source brief comfortably past MaxTmuxCommandBytes, matching
// the shape of the user's real 20 KB plan body.
func hugeBrief() string {
	return strings.Repeat("normalise, store, embed, hard-filter the catalogue. ", 400) // ~20 KB
}

// TestCmdPlanGoFoldsOversizedBrief pins the fan-out door (cli/plan.go planGo).
func TestCmdPlanGoFoldsOversizedBrief(t *testing.T) {
	t.Setenv(launch.PermissionModeEnv, "auto")
	seedDataHome(t)
	if _, err := projects.Add(projects.Project{Name: "app", Sources: []projects.Source{
		{Name: "web", Path: "/repos/web"},
	}}); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	body := "## Goal\nBig one.\n\n## Source briefs\n### web\n" + hugeBrief()
	p, _ := plans.New("app", "Catalogue side of the matching engine", body)
	plans.AddTarget("app", p.ID, "web")

	var got launch.Intent
	var gotRoot string
	stubPlanLauncher(t, func(root string, in launch.Intent) (launch.Result, error) {
		gotRoot, got = root, in
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	})
	if code := cmdPlanStore([]string{"go", p.ID}); code != 0 {
		t.Fatalf("plan go: exit %d, want 0", code)
	}

	argv := launch.TmuxNewSessionArgs(gotRoot, got)
	if n := launch.TmuxCommandBytes(argv); n > launch.MaxTmuxCommandBytes {
		t.Errorf("`gogo plan go` built a %d-byte command line - over tmux's %d-byte limit; the fold is not wired at this door",
			n, launch.MaxTmuxCommandBytes)
	}
	if strings.Contains(got.Command, hugeBrief()) {
		t.Error("the oversized brief is still inlined into the launched command")
	}
	for _, want := range []string{plans.Path("app", p.ID), "### web", "--correlation " + p.ID} {
		if !strings.Contains(got.Command, want) {
			t.Errorf("folded command missing %q:\n%s", want, got.Command)
		}
	}
}

// TestCmdPlanPromoteFoldsOversizedBrief pins the single-source door
// (cli/plan.go planPromote), which seeds the WHOLE plan body as the goal.
func TestCmdPlanPromoteFoldsOversizedBrief(t *testing.T) {
	t.Setenv(launch.PermissionModeEnv, "auto")
	seedDataHome(t)
	if _, err := projects.Add(projects.Project{Name: "app", Sources: []projects.Source{
		{Name: "web", Path: "/repos/web"},
	}}); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	p, _ := plans.New("app", "Catalogue side", hugeBrief())

	var got launch.Intent
	var gotRoot string
	stubPlanLauncher(t, func(root string, in launch.Intent) (launch.Result, error) {
		gotRoot, got = root, in
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	})
	if code := cmdPlanStore([]string{"promote", p.ID, "web"}); code != 0 {
		t.Fatalf("plan promote: exit %d, want 0", code)
	}

	argv := launch.TmuxNewSessionArgs(gotRoot, got)
	if n := launch.TmuxCommandBytes(argv); n > launch.MaxTmuxCommandBytes {
		t.Errorf("`gogo plan promote` built a %d-byte command line - over tmux's %d-byte limit; the fold is not wired at this door",
			n, launch.MaxTmuxCommandBytes)
	}
	if strings.Contains(got.Command, hugeBrief()) {
		t.Error("the oversized brief is still inlined into the launched command")
	}
	if !strings.Contains(got.Command, plans.Path("app", p.ID)) {
		t.Errorf("folded command does not point at the plan file:\n%s", got.Command)
	}
}

// TestCmdPlanDoorsUnderBudgetAreByteForByte is the other half of D1=A at these
// doors: a normal brief must launch EXACTLY as it did before the fold existed.
func TestCmdPlanDoorsUnderBudgetAreByteForByte(t *testing.T) {
	t.Setenv(launch.PermissionModeEnv, "auto")
	seedDataHome(t)
	if _, err := projects.Add(projects.Project{Name: "app", Sources: []projects.Source{
		{Name: "web", Path: "/repos/web"},
	}}); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	p, _ := plans.New("app", "Token migration", "move the shared token store")
	plans.AddTarget("app", p.ID, "web")

	var cmds []string
	stubPlanLauncher(t, func(root string, in launch.Intent) (launch.Result, error) {
		cmds = append(cmds, in.Command)
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	})
	if code := cmdPlanStore([]string{"go", p.ID}); code != 0 {
		t.Fatalf("plan go: exit %d", code)
	}
	want := "/gogo:plan move the shared token store --correlation " + p.ID
	if len(cmds) != 1 || cmds[0] != want {
		t.Errorf("under-budget `gogo plan go` command = %q, want the pre-fold command %q", cmds, want)
	}
}

// --- REV-004: one slug transform behind both doors ---------------------------

// TestPlanKebabMatchesTUITransform pins that the CLI's member-hint derivation and
// the launch package's session-label transform are the SAME function. They write
// the same field of the same store (plans.Member.SlugHint), so two hand-kept
// regexes were guaranteed to drift - and had already drifted on `" - "` and on
// any 48+ char title.
func TestPlanKebabMatchesTUITransform(t *testing.T) {
	for _, title := range []string{
		"Catalogue side - normalise",                                      // the `" - "` disagreement
		"Catalogue side of the matching engine - normalise, store, embed", // the 48+ cap disagreement
		strings.Repeat("segment-", 12),                                    // 60+ chars, all boundaries
		"Weird.Name:With/Spaces & dots",
	} {
		if got, want := planKebab(title), launch.SlugFromLabel(title); got != want {
			t.Errorf("planKebab(%q) = %q, but launch.SlugFromLabel = %q - the two doors write different SlugHints", title, got, want)
		}
	}
	// A blank title keeps the CLI's own "plan" fallback (not the launch "run" one).
	for _, blank := range []string{"", "   "} {
		if got := planKebab(blank); got != "plan" {
			t.Errorf("planKebab(%q) = %q, want the plan fallback", blank, got)
		}
	}
	// And the hint a spawn records still attributes to the session that spawn mints.
	title := "Catalogue side of the matching engine - normalise, store, embed"
	if !launch.SessionMatchesSlug("gogo-plan-"+launch.SlugFromLabel(title), planKebab(title)) {
		t.Error("the recorded SlugHint no longer attributes to the session the spawn mints")
	}
}
