package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// TestParseForceFlag: parseSessionFlags parses the concurrency-cap escape hatch
// (--force), distinct from --takeover, and defaults it to false when absent.
func TestParseForceFlag(t *testing.T) {
	slug, _, _, force, helped, code := parseSessionFlags("gogo go", goHelp, []string{"myfeat", "--force"})
	if helped || code != 0 {
		t.Fatalf("parse with --force: helped=%v code=%d", helped, code)
	}
	if slug != "myfeat" || !force {
		t.Errorf("parse = slug %q force %v, want myfeat/true", slug, force)
	}
	if _, _, _, force2, _, _ := parseSessionFlags("gogo go", goHelp, []string{"myfeat"}); force2 {
		t.Errorf("force defaulted to true with no --force flag")
	}
}

// TestCapBlock drives the pure over-cap decision off an injected session list (no
// real tmux): over the cap → a refusal naming the cap + the live feature + the
// --force hint; --force → bypass; a resume of the active feature itself → allowed
// (excluded from its own count); an uncapped / unregistered root → allowed.
func TestCapBlock(t *testing.T) {
	t.Setenv("GOGO_DATA_HOME", t.TempDir())
	const root = "/repos/app"
	if _, err := projects.Add(projects.Project{Name: "app", Sources: []projects.Source{
		{Path: root, Name: "app", ConcurrentWorkItems: 1},
	}}); err != nil {
		t.Fatal(err)
	}
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "active", Root: root, Class: contract.ClassInProgress},
		{Slug: "target", Root: root, Class: contract.ClassUnfinished},
	}}
	orig := sessionLister
	sessionLister = func() []string { return []string{"gogo-go-active"} }
	defer func() { sessionLister = orig }()

	if msg := capBlock(root, repo, "target", false); msg == "" || !strings.Contains(msg, "--force") {
		t.Errorf("over-cap capBlock = %q, want a refusal naming --force", msg)
	}
	if msg := capBlock(root, repo, "target", true); msg != "" {
		t.Errorf("--force capBlock = %q, want an empty (bypassed) message", msg)
	}
	if msg := capBlock(root, repo, "active", false); msg != "" {
		t.Errorf("resume-of-active capBlock = %q, want allowed (excluded from its own count)", msg)
	}
	if msg := capBlock("/repos/unregistered", repo, "target", false); msg != "" {
		t.Errorf("unregistered-root capBlock = %q, want allowed (fallback)", msg)
	}
}

// TestGoCapGuardE2E is the end-to-end `gogo go` cap guard over the real command
// dispatch with a stub claude on PATH: an over-cap launch is refused (exit 1)
// BEFORE any session spawns; --force bypasses it and launches; an uncapped project
// behaves exactly as today (launches despite an active feature).
func TestGoCapGuardE2E(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub claude is a bash script; skip on windows")
	}
	binDir := writeStubClaude(t)

	t.Run("over cap → refused (exit 1) and nothing is launched", func(t *testing.T) {
		root, slug := setupScratchFeature(t, "captarget")
		addActiveFeature(t, root, "capother")
		stateDir := t.TempDir()
		wireStub(t, binDir, stateDir, slug, "happy")
		capRegister(t, root, 1)
		restore := stubSessions("gogo-go-capother")
		defer restore()

		stderr, code := captureStderr(t, func() int { return cmdGo([]string{slug}) })
		if code != 1 {
			t.Fatalf("over-cap cmdGo = %d, want 1 (refused)", code)
		}
		if !strings.Contains(stderr, "--force") {
			t.Errorf("refusal must name the --force escape; stderr: %q", stderr)
		}
		// Refused before launch → the stub was never exec'd (no argv log).
		if _, err := os.Stat(filepath.Join(stateDir, "argv.log")); err == nil {
			t.Error("a session was launched despite the over-cap refusal")
		}
	})

	t.Run("--force bypasses the cap and launches", func(t *testing.T) {
		root, slug := setupScratchFeature(t, "capforce")
		addActiveFeature(t, root, "capforceother")
		stateDir := t.TempDir()
		wireStub(t, binDir, stateDir, slug, "happy")
		capRegister(t, root, 1)
		restore := stubSessions("gogo-go-capforceother")
		defer restore()

		if code := cmdGo([]string{slug, "--force"}); code != 0 {
			t.Fatalf("--force cmdGo = %d, want 0 (bypassed → launched → awaiting-uat)", code)
		}
		if argv := readArgvLog(t, stateDir); len(argv) != 1 {
			t.Errorf("--force must launch exactly once; argv calls = %d", len(argv))
		}
	})

	t.Run("uncapped project → today's behaviour (launches despite an active feature)", func(t *testing.T) {
		root, slug := setupScratchFeature(t, "capuncapped")
		addActiveFeature(t, root, "capuncappedother")
		stateDir := t.TempDir()
		wireStub(t, binDir, stateDir, slug, "happy")
		capRegister(t, root, 0) // 0 = unlimited
		restore := stubSessions("gogo-go-capuncappedother")
		defer restore()

		if code := cmdGo([]string{slug}); code != 0 {
			t.Fatalf("uncapped cmdGo = %d, want 0 (no guard fires)", code)
		}
	})
}

// addActiveFeature writes a minimal in-progress (implementing) feature under root
// so it classifies as ClassInProgress — the concurrency the cap counts when it
// also carries a live session.
func addActiveFeature(t *testing.T, root, slug string) {
	t.Helper()
	dir := filepath.Join(root, ".gogo", "work", "feature-"+slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir active feature: %v", err)
	}
	state := "# State — feature `" + slug + "`\n\n" +
		"- **feature:** cap-guard active feature\n" +
		"- **phase:** implement\n" +
		"- **status:** implementing\n" +
		"- **created:** 2026-07-15\n" +
		"- **open-decision:** none\n"
	if err := os.WriteFile(filepath.Join(dir, "state.md"), []byte(state), 0o644); err != nil {
		t.Fatalf("write active state.md: %v", err)
	}
}

// capRegister points GOGO_DATA_HOME at a fresh dir and registers root as a
// project's source with the given per-source cap (0 = unlimited).
func capRegister(t *testing.T, root string, cap int) {
	t.Helper()
	t.Setenv("GOGO_DATA_HOME", t.TempDir())
	name := filepath.Base(root)
	if _, err := projects.Add(projects.Project{Name: name, Sources: []projects.Source{
		{Path: root, Name: name, ConcurrentWorkItems: cap},
	}}); err != nil {
		t.Fatalf("register capped source: %v", err)
	}
}

// stubSessions overrides the cap guard's session lister with a fixed set (no real
// tmux) and returns a restore func.
func stubSessions(sessions ...string) func() {
	orig := sessionLister
	sessionLister = func() []string { return sessions }
	return func() { sessionLister = orig }
}
