package main

import (
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// TestChooseBoardTwoMode pins the two-mode runBoard decision (UAT round 1): the
// mode is a small pure function (project store + root-discovery result + the
// global-home "initialized" check are all injected), so every branch is unit-tested
// without opening a TTY.
//
//  1. inside a repo → THAT repo's single board, always (even when the repo IS a
//     registered project's source — per-repo stays simple, no auto-route)
//  2. outside any repo + initialized + ≥1 project → the global cockpit
//  3. outside + initialized + 0 projects → an "add a project" hint (none)
//  4. outside + NOT initialized → a "gogo global init" hint (none)
func TestChooseBoardTwoMode(t *testing.T) {
	const root = "/repos/gogo"
	// A store where `root` IS a registered project's source — mode 1 must STILL
	// resolve to the single repo board (the dropped case-1 auto-route).
	owning := func() ([]projects.Project, error) {
		return []projects.Project{{
			Name:    "gogo",
			Sources: []projects.Source{{Path: root, Name: "gogo"}},
		}}, nil
	}
	someProjects := func() ([]projects.Project, error) {
		return []projects.Project{{
			Name:    "other",
			Sources: []projects.Source{{Path: "/repos/other", Name: "other"}},
		}}, nil
	}
	noProjects := func() ([]projects.Project, error) { return nil, nil }
	yes := func() bool { return true }
	no := func() bool { return false }

	// 1. Inside a repo → the single-repo board, EVEN when the repo is a registered
	//    source of a project (the two-mode drop of the old case-1 project auto-route).
	if got := chooseBoard(root, true, owning, yes); got.kind != "single" || got.model == nil {
		t.Errorf("in-repo (is a project source): kind=%q model=%v, want single/non-nil", got.kind, got.model)
	}
	// The single board also holds with an empty / uninitialized store — a repo is a repo.
	if got := chooseBoard(root, true, noProjects, no); got.kind != "single" || got.model == nil {
		t.Errorf("in-repo (empty store): kind=%q model=%v, want single/non-nil", got.kind, got.model)
	}

	// 2. Outside any repo + initialized + ≥1 project → the global cockpit.
	if got := chooseBoard("", false, someProjects, yes); got.kind != "project" || got.model == nil {
		t.Errorf("outside/initialized/≥1: kind=%q model=%v, want project/non-nil", got.kind, got.model)
	}

	// 3. Outside + initialized + 0 projects → none (add-a-project hint, no model).
	got := chooseBoard("", false, noProjects, yes)
	if got.kind != "none" || got.model != nil {
		t.Errorf("outside/initialized/0: kind=%q model=%v, want none/nil", got.kind, got.model)
	}
	if got.err == "" {
		t.Error("outside/initialized/0 must carry a stderr hint")
	}

	// 4. Outside + NOT initialized → none (a `gogo global init` hint, no model).
	got = chooseBoard("", false, someProjects, no)
	if got.kind != "none" || got.model != nil {
		t.Errorf("outside/uninitialized: kind=%q model=%v, want none/nil", got.kind, got.model)
	}
	if got.err == "" {
		t.Error("outside/uninitialized must carry a stderr hint")
	}
}
