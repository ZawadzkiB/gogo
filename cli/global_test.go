package main

import (
	"os"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// TestGlobalInitCreatesMarkerAndProjectsDir: `gogo global init` initializes the
// global cockpit home — it creates ~/.gogo/projects/ and writes the
// ~/.gogo/config.json marker (Initialized() true), is idempotent (a re-run exits 0
// and leaves it initialized), and never touches anything outside ~/.gogo/ (the whole
// store is a t.TempDir() via the GOGO_DATA_HOME seam).
func TestGlobalInitCreatesMarkerAndProjectsDir(t *testing.T) {
	seedDataHome(t)

	if projects.Initialized() {
		t.Fatal("fresh home should not be initialized before `global init`")
	}
	if code := cmdGlobal([]string{"init"}); code != 0 {
		t.Fatalf("global init: exit %d, want 0", code)
	}
	if !projects.Initialized() {
		t.Error("global init did not mark the home initialized")
	}
	if _, err := os.Stat(projects.HomeConfigPath()); err != nil {
		t.Errorf("global init did not write the ~/.gogo/config.json marker: %v", err)
	}
	if info, err := os.Stat(projects.ProjectsDir()); err != nil || !info.IsDir() {
		t.Errorf("global init did not create ~/.gogo/projects/: err=%v", err)
	}

	// Idempotent: a second init still exits 0 and stays initialized.
	if code := cmdGlobal([]string{"init"}); code != 0 {
		t.Fatalf("re-run global init: exit %d, want 0", code)
	}
	if !projects.Initialized() {
		t.Error("re-run global init un-initialized the home")
	}
}

// TestGlobalBoardUninitializedHints: `gogo global` on an uninitialized home hints
// `gogo global init` (non-zero, no crash, no TTY).
func TestGlobalBoardUninitializedHints(t *testing.T) {
	seedDataHome(t)
	if code := cmdGlobal(nil); code == 0 {
		t.Errorf("global on uninitialized home: exit 0, want non-zero hint")
	}
}

// TestGlobalBoardInitializedNoProjectsHints: `gogo global` on an initialized home
// with 0 projects hints `gogo project add` (non-zero, no crash, no TTY) — the ≥1
// project branch opens a TTY and is exercised via chooseBoard's pure seam instead.
func TestGlobalBoardInitializedNoProjectsHints(t *testing.T) {
	seedDataHome(t)
	if _, err := projects.EnsureHome(); err != nil {
		t.Fatal(err)
	}
	if code := cmdGlobal([]string{"board"}); code == 0 {
		t.Errorf("global board with 0 projects: exit 0, want non-zero hint")
	}
}

func TestGlobalUnknownSubcommand(t *testing.T) {
	seedDataHome(t)
	if code := cmdGlobal([]string{"frobnicate"}); code == 0 {
		t.Errorf("global unknown subcommand: exit 0, want non-zero")
	}
}

func TestGlobalHelp(t *testing.T) {
	seedDataHome(t)
	if code := cmdGlobal([]string{"-h"}); code != 0 {
		t.Errorf("global -h: exit %d, want 0", code)
	}
}
