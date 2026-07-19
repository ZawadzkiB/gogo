package main

import (
	"path/filepath"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// TestCmdSourceAddDefaultsToSoleProject: `gogo source add <repo>` with a single
// project appends the repo as a second source (no --project needed).
func TestCmdSourceAddDefaultsToSoleProject(t *testing.T) {
	seedDataHome(t)
	repo1 := gogoRepo(t)
	repo2 := gogoRepo(t)
	if code := cmdProject([]string{"add", repo1, "--name", "gogo"}); code != 0 {
		t.Fatalf("project add: exit %d", code)
	}
	if code := cmdSource([]string{"add", repo2}); code != 0 {
		t.Fatalf("source add (sole project): exit %d, want 0", code)
	}
	p, _ := projects.Load("gogo")
	if len(p.Sources) != 2 {
		t.Fatalf("sources = %d, want 2", len(p.Sources))
	}
	if !hasSourcePath(p.Sources, repo2) {
		t.Errorf("source %s not linked: %+v", repo2, p.Sources)
	}
}

// TestCmdSourceAddAmbiguous: with >1 project and no --project, `source add` errors
// (ambiguous) and writes nothing; --project resolves it.
func TestCmdSourceAddAmbiguous(t *testing.T) {
	seedDataHome(t)
	repoA := gogoRepo(t)
	repoB := gogoRepo(t)
	extra := gogoRepo(t)
	cmdProject([]string{"add", repoA, "--name", "alpha"})
	cmdProject([]string{"add", repoB, "--name", "beta"})

	if code := cmdSource([]string{"add", extra}); code == 0 {
		t.Errorf("ambiguous source add: exit 0, want non-zero")
	}
	// Neither project gained the source.
	for _, name := range []string{"alpha", "beta"} {
		if p, _ := projects.Load(name); hasSourcePath(p.Sources, extra) {
			t.Errorf("ambiguous add still wrote %s into %q", extra, name)
		}
	}
	// --project resolves it.
	if code := cmdSource([]string{"add", extra, "--project", "beta"}); code != 0 {
		t.Fatalf("source add --project beta: exit %d, want 0", code)
	}
	if p, _ := projects.Load("beta"); !hasSourcePath(p.Sources, extra) {
		t.Errorf("source not added to beta: %+v", p.Sources)
	}
}

// TestCmdSourceRm: remove a source by name and by path.
func TestCmdSourceRm(t *testing.T) {
	seedDataHome(t)
	repo1 := gogoRepo(t)
	repo2 := gogoRepo(t)
	cmdProject([]string{"add", repo1, "--name", "gogo"})
	cmdSource([]string{"add", repo2})

	// rm by source name (basename of repo2).
	if code := cmdSource([]string{"rm", filepath.Base(repo2)}); code != 0 {
		t.Fatalf("source rm by name: exit %d, want 0", code)
	}
	if p, _ := projects.Load("gogo"); hasSourcePath(p.Sources, repo2) {
		t.Errorf("source %s survived rm: %+v", repo2, p.Sources)
	}
	// rm a no-match → non-zero.
	if code := cmdSource([]string{"rm", "nope"}); code == 0 {
		t.Errorf("source rm no-match: exit 0, want non-zero")
	}
}

// TestCmdSourceNoProjects: source ops error cleanly when no project exists.
func TestCmdSourceNoProjects(t *testing.T) {
	seedDataHome(t)
	repo := gogoRepo(t)
	if code := cmdSource([]string{"add", repo}); code == 0 {
		t.Errorf("source add with no projects: exit 0, want non-zero")
	}
}

// TestCmdSourceUnknownSubcommand: an unknown subcommand errors.
func TestCmdSourceUnknownSubcommand(t *testing.T) {
	seedDataHome(t)
	if code := cmdSource([]string{"frobnicate"}); code == 0 {
		t.Errorf("unknown subcommand: exit 0, want non-zero")
	}
}
