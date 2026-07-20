package main

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// TestResolveSourceSkip (FR4, REV-001): resolveSourceSkip reads the projects store and
// returns a flagged source's (planSkip, uatSkip, label) for a matching root, and
// (false,false,"") for an unflagged or an UNREGISTERED root — the byte-for-byte fallback
// that keeps an unregistered / single repo's gates intact.
func TestResolveSourceSkip(t *testing.T) {
	t.Setenv("GOGO_DATA_HOME", t.TempDir())
	if _, err := projects.Add(projects.Project{Name: "app", Sources: []projects.Source{
		{Path: "/repos/flagged", Name: "flagged", PlanAcceptanceSkip: true, UatAcceptanceSkip: true},
		{Path: "/repos/plain", Name: "plain"},
	}}); err != nil {
		t.Fatal(err)
	}
	if p, u, label := resolveSourceSkip("/repos/flagged"); !p || !u || label != "flagged" {
		t.Errorf("flagged resolveSourceSkip = (%v,%v,%q), want (true,true,flagged)", p, u, label)
	}
	if p, u, _ := resolveSourceSkip("/repos/plain"); p || u {
		t.Errorf("plain resolveSourceSkip = (%v,%v), want (false,false)", p, u)
	}
	if p, u, label := resolveSourceSkip("/repos/ghost"); p || u || label != "" {
		t.Errorf("unregistered resolveSourceSkip = (%v,%v,%q), want (false,false,\"\")", p, u, label)
	}
}

// TestGoPathCarriesSkipParams is the end-to-end proof (FR4, REV-001) that `gogo go` appends
// the SOURCE's gate-skip params to the launched /gogo:go command EXACTLY when the source is
// flagged and NEVER otherwise — the regression net plan.md Phase-C promised. It drives the
// real cmdGo → LaunchOrResume → ClaudeRunner → exec path with a stub claude and reads the
// argv the persistent session was launched with. It also pins REV-005: the plan-acceptance
// note announces the source's opt-in without over-claiming an "auto-skipped" this leg.
func TestGoPathCarriesSkipParams(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub claude is a bash script; skip on windows")
	}
	binDir := writeStubClaude(t)

	register := func(t *testing.T, root string, s projects.Source) {
		t.Setenv("GOGO_DATA_HOME", t.TempDir())
		s.Path = root
		if _, err := projects.Add(projects.Project{Name: filepath.Base(root), Sources: []projects.Source{s}}); err != nil {
			t.Fatalf("register source: %v", err)
		}
	}
	launchedCommand := func(t *testing.T, stateDir string) string {
		argv := readArgvLog(t, stateDir)
		if len(argv) != 1 {
			t.Fatalf("argv log has %d calls, want 1", len(argv))
		}
		return argv[0][len(argv[0])-1] // the whole slash command is the single trailing element
	}

	t.Run("flagged source → both skip params appended as the single trailing element", func(t *testing.T) {
		root, slug := setupScratchFeature(t, "skipflagged")
		register(t, root, projects.Source{Name: "flagged", PlanAcceptanceSkip: true, UatAcceptanceSkip: true})
		stateDir := t.TempDir()
		wireStub(t, binDir, stateDir, slug, "happy")

		out, code := captureStdout(t, func() int { return cmdGo([]string{slug}) })
		if code != 0 {
			t.Fatalf("cmdGo = %d, want 0", code)
		}
		if got := launchedCommand(t, stateDir); got != "/gogo:go "+slug+" --skip-acceptance --skip-uat" {
			t.Errorf("launched command = %q, want both skip params", got)
		}
		// REV-005: the plan-acceptance note must NOT over-claim an "auto-skipped" on a leg
		// that skips nothing (a resume past the plan gate), but must still announce the opt-in.
		if strings.Contains(out, "plan-acceptance auto-skipped") {
			t.Errorf("plan note over-claims 'auto-skipped' on a leg past the plan gate:\n%s", out)
		}
		if !strings.Contains(out, "planAcceptanceSkip") {
			t.Errorf("plan note should still announce the source opt-in (planAcceptanceSkip):\n%s", out)
		}
		if !strings.Contains(out, "UAT auto-skipped") {
			t.Errorf("uat note should announce the UAT auto-skip:\n%s", out)
		}
	})

	t.Run("unflagged source → byte-for-byte today's command (no params, no note)", func(t *testing.T) {
		root, slug := setupScratchFeature(t, "skipplain")
		register(t, root, projects.Source{Name: "plain"})
		stateDir := t.TempDir()
		wireStub(t, binDir, stateDir, slug, "happy")

		out, code := captureStdout(t, func() int { return cmdGo([]string{slug}) })
		if code != 0 {
			t.Fatalf("cmdGo = %d, want 0", code)
		}
		if got := launchedCommand(t, stateDir); got != "/gogo:go "+slug {
			t.Errorf("launched command = %q, want no skip params", got)
		}
		if strings.Contains(out, "planAcceptanceSkip") || strings.Contains(out, "uatAcceptanceSkip") {
			t.Errorf("an unflagged source must print no skip note:\n%s", out)
		}
	})

	t.Run("unregistered root → no store entry → no params", func(t *testing.T) {
		_, slug := setupScratchFeature(t, "skipunreg")
		t.Setenv("GOGO_DATA_HOME", t.TempDir()) // empty store — the root is not a registered source
		stateDir := t.TempDir()
		wireStub(t, binDir, stateDir, slug, "happy")

		if code := cmdGo([]string{slug}); code != 0 {
			t.Fatalf("cmdGo = %d, want 0", code)
		}
		if got := launchedCommand(t, stateDir); got != "/gogo:go "+slug {
			t.Errorf("launched command = %q, want no skip params for an unregistered root", got)
		}
	})
}
