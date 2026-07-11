package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/orchestrator"
)

// TestGoE2EStubClaude is THE end-to-end dry run for `gogo go`: it drives the REAL
// command dispatch (cmdGo -> Session.LaunchOrResume -> ClaudeRunner ->
// launch.RunPhase -> exec.Command("claude", ...)) with a stub `claude` on PATH
// standing in for the model, so the full process-spawning wiring — the argv the
// persistent session is launched with, and the state.md exit classification — is
// exercised without a real, billable claude session (plan Tests §8, D2). This is
// the ONE seam the fake-SessionRunner unit tests (orchestrator_test.go) never
// cross: they inject orchestrator.SessionRunner directly and so never touch
// launch.RunPhase's actual argv construction or the exec.Command boundary.
//
// Hermetic: everything lives under t.TempDir(); PATH/cwd/env are restored via
// t.Setenv/t.Chdir; no network; no real claude.
func TestGoE2EStubClaude(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub claude is a bash script; skip on windows")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not on PATH — cannot run the stub claude script")
	}
	binDir := writeStubClaude(t)

	t.Run("first launch: fresh --session-id, -p /gogo:go, permission flag, awaiting-uat -> exit 0", func(t *testing.T) {
		root, slug := setupScratchFeature(t, "e2egot")
		stateDir := t.TempDir()
		wireStub(t, binDir, stateDir, slug, "happy")

		if code := cmdGo([]string{slug}); code != 0 {
			t.Fatalf("cmdGo = %d, want 0 (green -> awaiting-uat)", code)
		}

		argv := readArgvLog(t, stateDir)
		if len(argv) != 1 {
			t.Fatalf("argv log has %d calls, want 1", len(argv))
		}
		// The persistent session is one -p run of the WHOLE /gogo:go skill.
		if last := argv[0][len(argv[0])-1]; last != "/gogo:go "+slug {
			t.Errorf("command = %q, want %q", last, "/gogo:go "+slug)
		}
		assertArg(t, argv[0], "--permission-mode", "auto")
		assertArg(t, argv[0], "--output-format", "json")
		assertNoArg(t, argv[0], "--resume")
		uuid := argValue(t, argv[0], "--session-id")

		// The registry persists that uuid as the feature's warm `go` session.
		reg := orchestrator.LoadRegistry(root, slug)
		if ps := reg.Get("go"); ps == nil || ps.UUID != uuid {
			t.Fatalf("registry go session uuid = %+v, want %q", ps, uuid)
		} else if ps.Status != orchestrator.SessAwaitingUAT {
			t.Errorf("registry go status = %q, want awaiting-uat", ps.Status)
		}
		if reg.CostUSD <= 0 {
			t.Errorf("run cost not booked, total = %v", reg.CostUSD)
		}
	})

	t.Run("re-run resumes the SAME warm uuid (--resume, no fresh --session-id)", func(t *testing.T) {
		root, slug := setupScratchFeature(t, "e2eresumet")
		stateDir := t.TempDir()
		wireStub(t, binDir, stateDir, slug, "resume-demo")

		// Call 1 leaves the feature mid-pipeline (implementing) — runnable, exit 2.
		if code := cmdGo([]string{slug}); code != 2 {
			t.Fatalf("first cmdGo = %d, want 2 (ended mid-pipeline at implementing)", code)
		}
		reg := orchestrator.LoadRegistry(root, slug)
		warm := reg.Get("go").UUID
		if warm == "" {
			t.Fatal("first run did not persist a go session uuid")
		}

		// Call 2 (feature still runnable) must --resume the SAME warm uuid.
		if code := cmdGo([]string{slug}); code != 0 {
			t.Fatalf("second cmdGo = %d, want 0 (resumed -> awaiting-uat)", code)
		}
		argv := readArgvLog(t, stateDir)
		if len(argv) != 2 {
			t.Fatalf("argv log has %d calls, want 2", len(argv))
		}
		assertArg(t, argv[0], "--session-id", warm)
		assertNoArg(t, argv[0], "--resume")
		assertArg(t, argv[1], "--resume", warm)
		assertNoArg(t, argv[1], "--session-id")
	})

	t.Run("is_error envelope halts (exit 1), never a false green", func(t *testing.T) {
		_, slug := setupScratchFeature(t, "e2eerrgot")
		stateDir := t.TempDir()
		wireStub(t, binDir, stateDir, slug, "error")

		if code := cmdGo([]string{slug}); code != 1 {
			t.Fatalf("cmdGo = %d, want 1 (is_error must halt)", code)
		}
	})

	t.Run("gogo run is a deprecated alias that forwards to gogo go", func(t *testing.T) {
		_, slug := setupScratchFeature(t, "e2ealiast")
		stateDir := t.TempDir()
		wireStub(t, binDir, stateDir, slug, "happy")

		stderr, code := captureStderr(t, func() int { return cmdRun([]string{slug}) })
		if code != 0 {
			t.Fatalf("cmdRun = %d, want 0 (forwards to gogo go on a happy stub)", code)
		}
		if !strings.Contains(strings.ToLower(stderr), "deprecated") {
			t.Errorf("gogo run must print a deprecation notice; stderr: %q", stderr)
		}
		argv := readArgvLog(t, stateDir)
		if len(argv) != 1 || argv[0][len(argv[0])-1] != "/gogo:go "+slug {
			t.Errorf("gogo run must forward to /gogo:go; argv = %v", argv)
		}
	})
}

// TestValidSlug guards the write-scope validator (REV-001): only kebab-case slugs
// pass; anything with a path separator or `..` is rejected before it can reach
// LockPath/RegistryPath and escape .gogo/resources/.
func TestValidSlug(t *testing.T) {
	ok := []string{"feat", "my-feature", "a1", "persistent-session-orchestrator"}
	bad := []string{"", "../foo", "a/b", "..", "./x", "Foo", "a--", "-a", "a_b", "a..b", "../../etc/passwd"}
	for _, s := range ok {
		if !validSlug(s) {
			t.Errorf("validSlug(%q) = false, want true", s)
		}
	}
	for _, s := range bad {
		if validSlug(s) {
			t.Errorf("validSlug(%q) = true, want false (write-scope escape)", s)
		}
	}
}

// TestCmdRejectsTraversalSlug: gogo go / gogo plan refuse a path-traversal slug
// before any filesystem path is built (no stub / no cwd needed — the guard is first).
func TestCmdRejectsTraversalSlug(t *testing.T) {
	for _, tc := range []struct {
		name string
		fn   func([]string) int
	}{
		{"gogo go", cmdGo},
		{"gogo plan", cmdPlan},
	} {
		stderr, code := captureStderr(t, func() int { return tc.fn([]string{"../../../../tmp/pwn"}) })
		if code != 1 {
			t.Errorf("%s with a traversal slug = %d, want 1", tc.name, code)
		}
		if !strings.Contains(stderr, "invalid slug") {
			t.Errorf("%s must reject the traversal slug with 'invalid slug'; stderr: %q", tc.name, stderr)
		}
	}
}

// setupScratchFeature builds a minimal throwaway plan-accepted feature under a
// fresh t.TempDir() and t.Chdir()s into it (auto-restored). Returns the root and slug.
func setupScratchFeature(t *testing.T, slug string) (root, gotSlug string) {
	t.Helper()
	root = t.TempDir()
	featureDir := filepath.Join(root, ".gogo", "work", "feature-"+slug)
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatalf("mkdir feature dir: %v", err)
	}
	state := "# State — feature `" + slug + "`\n\n" +
		"- **feature:** e2e dry-run scratch\n" +
		"- **phase:** implement\n" +
		"- **status:** plan-accepted\n" +
		"- **created:** 2026-07-11\n" +
		"- **accepted:** 2026-07-11 (user)\n" +
		"- **open-decision:** none\n"
	if err := os.WriteFile(filepath.Join(featureDir, "state.md"), []byte(state), 0o644); err != nil {
		t.Fatalf("write state.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, "plan.md"), []byte("PLACEHOLDER plan\n"), 0o644); err != nil {
		t.Fatalf("write plan.md: %v", err)
	}
	t.Chdir(root)
	return root, slug
}

// wireStub points PATH at the stub claude and configures its behaviour via env
// (all auto-restored by t.Setenv). Pins the permission mode so a host env var
// can't perturb the argv assertions.
func wireStub(t *testing.T, binDir, stateDir, slug, mode string) {
	t.Helper()
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GOGO_STUB_STATE", stateDir)
	t.Setenv("GOGO_STUB_SLUG", slug)
	t.Setenv("GOGO_STUB_MODE", mode)
	t.Setenv("GOGO_CLAUDE_PERMISSION_MODE", "auto")
}

// captureStderr runs fn with os.Stderr redirected to a pipe and returns what it wrote.
func captureStderr(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	code := fn()
	_ = w.Close()
	os.Stderr = orig
	out, _ := io.ReadAll(r)
	return string(out), code
}

// readArgvLog parses the stub's argv.log: one call per line, each argv element
// wrapped in <<...>>.
func readArgvLog(t *testing.T, stateDir string) [][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(stateDir, "argv.log"))
	if err != nil {
		t.Fatalf("read argv.log: %v", err)
	}
	var calls [][]string
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		var args []string
		for _, part := range strings.Split(line, "<<") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			args = append(args, strings.TrimSuffix(part, ">>"))
		}
		calls = append(calls, args)
	}
	return calls
}

func argValue(t *testing.T, argv []string, flag string) string {
	t.Helper()
	for i, a := range argv {
		if a == flag && i+1 < len(argv) {
			return argv[i+1]
		}
	}
	t.Fatalf("argv %v missing flag %q", argv, flag)
	return ""
}

func assertArg(t *testing.T, argv []string, flag, value string) {
	t.Helper()
	if got := argValue(t, argv, flag); got != value {
		t.Errorf("argv %v: %s = %q, want %q", argv, flag, got, value)
	}
}

func assertNoArg(t *testing.T, argv []string, flag string) {
	t.Helper()
	for _, a := range argv {
		if a == flag {
			t.Errorf("argv %v: unexpected flag %q present", argv, flag)
			return
		}
	}
}

// writeStubClaude writes a stub `claude` executable to a fresh temp bin dir. It
// stands in for the real model: it logs its full argv, writes the canned state.md
// the persistent /gogo:go session would leave (gated by GOGO_STUB_MODE), and
// prints a minimal `--output-format json` envelope to stdout — matching the
// RunResult shape (session_id/total_cost_usd/num_turns/duration_ms/is_error)
// launch.RunPhase parses.
func writeStubClaude(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "claude")
	if err := os.WriteFile(path, []byte(stubClaudeScript), 0o755); err != nil {
		t.Fatalf("write stub claude: %v", err)
	}
	return dir
}

const stubClaudeScript = `#!/usr/bin/env bash
# Stub claude for TestGoE2EStubClaude — records argv and writes a canned state.md
# instead of spawning a real model. See cli/go_e2e_test.go.
set -euo pipefail

STATE_DIR="${GOGO_STUB_STATE:?GOGO_STUB_STATE not set}"
SLUG="${GOGO_STUB_SLUG:?GOGO_STUB_SLUG not set}"
MODE="${GOGO_STUB_MODE:-happy}"

mkdir -p "$STATE_DIR"
{
  for a in "$@"; do printf '<<%s>>' "$a"; done
  printf '\n'
} >> "$STATE_DIR/argv.log"

FEATURE_DIR=".gogo/work/feature-$SLUG"
mkdir -p "$FEATURE_DIR"

is_error="false"
status="awaiting-uat"
case "$MODE" in
  happy)  status="awaiting-uat" ;;
  parked) status="waiting-for-user" ;;
  error)  status="plan-accepted"; is_error="true" ;;
  resume-demo)
    n=0; [ -f "$STATE_DIR/calls" ] && n=$(cat "$STATE_DIR/calls"); n=$((n+1)); echo "$n" > "$STATE_DIR/calls"
    if [ "$n" -eq 1 ]; then status="implementing"; else status="awaiting-uat"; fi
    ;;
esac

cat > "$FEATURE_DIR/state.md" <<MD
# State — feature $SLUG

- **feature:** e2e stub
- **phase:** implement
- **status:** $status
- **created:** 2026-07-11
- **open-decision:** none
MD

printf '{"session_id":"stub","total_cost_usd":0.02,"num_turns":1,"duration_ms":10,"is_error":%s}\n' "$is_error"
`
