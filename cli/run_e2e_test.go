package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/orchestrator"
)

// TestRunE2EStubClaude is THE end-to-end dry run for `gogo run`: it drives the
// REAL command dispatch (cmdRun -> orchestrator.New -> ClaudeRunner ->
// launch.RunPhase -> exec.Command("claude", ...)) with a stub `claude` on PATH
// standing in for the model, so the full process-spawning wiring is exercised
// without spawning a real, billable claude session (plan Tests §, D2). This is
// the ONE seam the fake-PhaseRunner unit tests (orchestrator_test.go) never
// cross: they inject orchestrator.PhaseRunner directly and so never touch
// launch.RunPhase's actual argv construction or exec.Command boundary.
//
// Hermetic: everything lives under t.TempDir(); PATH/cwd/env are restored via
// t.Setenv/t.Chdir; no network; no real claude.
func TestRunE2EStubClaude(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub claude is a bash script; skip on windows")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not on PATH — cannot run the stub claude script")
	}

	binDir := writeStubClaude(t)

	t.Run("happy path: implement->review->test->report, registry has stable dev_uuid + telemetry", func(t *testing.T) {
		root, slug := setupScratchFeature(t, "e2ehappyt")
		stateDir := t.TempDir()
		wireStub(t, binDir, stateDir, slug, "happy", "")

		code := cmdRun([]string{slug})
		if code != 0 {
			t.Fatalf("cmdRun = %d, want 0 (green -> awaiting-uat)", code)
		}

		gotOrder := readCallPhases(t, stateDir)
		wantOrder := []string{"implement", "review", "test", "report"}
		if strings.Join(gotOrder, ",") != strings.Join(wantOrder, ",") {
			t.Errorf("phase call order = %v, want %v", gotOrder, wantOrder)
		}

		reg := orchestrator.LoadRegistry(root, slug)
		if reg.DevUUID == "" {
			t.Error("registry dev_uuid is empty, want a stable uuid")
		}
		if len(reg.Sessions) != 4 {
			t.Fatalf("registry has %d sessions, want 4 (implement/review/test/report)", len(reg.Sessions))
		}
		for _, s := range reg.Sessions {
			if s.CostUSD <= 0 {
				t.Errorf("session %q has cost_usd=%v, want > 0 (telemetry booked)", s.Kind, s.CostUSD)
			}
		}
		if reg.Sessions[0].UUID != reg.DevUUID {
			t.Errorf("first (implement) session uuid = %q, want it to equal the registry dev_uuid %q", reg.Sessions[0].UUID, reg.DevUUID)
		}
	})

	t.Run("warm resume: open major on round 1 -> --resume <dev-uuid> fix round -> fresh --session-id per review", func(t *testing.T) {
		root, slug := setupScratchFeature(t, "e2ewarmt")
		stateDir := t.TempDir()
		wireStub(t, binDir, stateDir, slug, "warm-resume", "")

		code := cmdRun([]string{slug})
		if code != 0 {
			t.Fatalf("cmdRun = %d, want 0 (fixed on round 2 -> green)", code)
		}

		gotOrder := readCallPhases(t, stateDir)
		wantOrder := []string{"implement", "review", "implement", "review", "test", "report"}
		if strings.Join(gotOrder, ",") != strings.Join(wantOrder, ",") {
			t.Fatalf("phase call order = %v, want %v (fix round must re-enter implement then re-review)", gotOrder, wantOrder)
		}

		reg := orchestrator.LoadRegistry(root, slug)
		if reg.DevUUID == "" {
			t.Fatal("registry dev_uuid is empty")
		}

		argv := readArgvLog(t, stateDir)
		if len(argv) != 6 {
			t.Fatalf("argv log has %d calls, want 6", len(argv))
		}
		// Call 0 (first implement build): a NEW session, --session-id == dev uuid.
		assertArg(t, argv[0], "--session-id", reg.DevUUID)
		assertNoArg(t, argv[0], "--resume")
		// Call 1 (first review): a fresh session, never --resume.
		firstReviewSessionID := argValue(t, argv[1], "--session-id")
		assertNoArg(t, argv[1], "--resume")
		// Call 2 (the warm fix round): MUST --resume the SAME dev uuid, never a fresh --session-id.
		assertArg(t, argv[2], "--resume", reg.DevUUID)
		assertNoArg(t, argv[2], "--session-id")
		// Call 3 (second, re-review): a FRESH session-id, different from the first review's.
		secondReviewSessionID := argValue(t, argv[3], "--session-id")
		assertNoArg(t, argv[3], "--resume")
		if secondReviewSessionID == firstReviewSessionID {
			t.Errorf("second review reused the first review's session id %q — review must be fresh-eyes every round", secondReviewSessionID)
		}
		if secondReviewSessionID == reg.DevUUID || firstReviewSessionID == reg.DevUUID {
			t.Errorf("a review session id collided with the dev uuid %q — review must never share the dev session", reg.DevUUID)
		}

		// Registry: the warm fix round must be recorded as resumed=true under the dev uuid.
		var sawResumedFix bool
		for _, s := range reg.Sessions {
			if s.Kind == "implement" && s.Resumed {
				sawResumedFix = true
				if s.UUID != reg.DevUUID {
					t.Errorf("resumed implement session uuid = %q, want dev uuid %q", s.UUID, reg.DevUUID)
				}
			}
		}
		if !sawResumedFix {
			t.Error("no implement session in the registry is marked resumed=true — the fix round was not recorded as warm")
		}
	})

	t.Run("is_error halts the loop (exit 1), not a false green", func(t *testing.T) {
		root, slug := setupScratchFeature(t, "e2eerrt")
		stateDir := t.TempDir()
		wireStub(t, binDir, stateDir, slug, "error", "review")

		code := cmdRun([]string{slug})
		if code != 1 {
			t.Fatalf("cmdRun = %d, want 1 (is_error must halt, never advance as green)", code)
		}

		gotOrder := readCallPhases(t, stateDir)
		wantOrder := []string{"implement", "review"}
		if strings.Join(gotOrder, ",") != strings.Join(wantOrder, ",") {
			t.Fatalf("phase call order = %v, want %v (must stop at the failed review, never reach test/report)", gotOrder, wantOrder)
		}

		// REV-006: the failed phase's own telemetry must still be booked before halting.
		reg := orchestrator.LoadRegistry(root, slug)
		if len(reg.Sessions) != 2 {
			t.Fatalf("registry has %d sessions, want 2 (implement + the failed review, cost booked before halt)", len(reg.Sessions))
		}
		if reg.Sessions[1].Kind != "review" || reg.Sessions[1].CostUSD <= 0 {
			t.Errorf("failed review session telemetry not booked: %+v", reg.Sessions[1])
		}
	})
}

// setupScratchFeature builds a minimal throwaway plan-accepted feature under a
// fresh t.TempDir() and t.Chdir()s into it (auto-restored). Returns the root and
// the slug.
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
		"- **branch:** n/a\n" +
		"- **iterations:** plan=1\n" +
		"- **resume:** implement — start build\n" +
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
// (all auto-restored by t.Setenv).
func wireStub(t *testing.T, binDir, stateDir, slug, mode, errorPhase string) {
	t.Helper()
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GOGO_STUB_STATE", stateDir)
	t.Setenv("GOGO_STUB_SLUG", slug)
	t.Setenv("GOGO_STUB_MODE", mode)
	t.Setenv("GOGO_STUB_ERROR_PHASE", errorPhase)
	t.Setenv(orchestrator.EnvCostCeiling, "10")
	t.Setenv(orchestrator.EnvMaxRounds, "3")
}

// readCallPhases reads the stub's calls.log (one "phase=<kind>" line per call,
// interleaved with the raw command line) and returns just the ordered phase kinds.
func readCallPhases(t *testing.T, stateDir string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(stateDir, "calls.log"))
	if err != nil {
		t.Fatalf("read calls.log: %v", err)
	}
	var phases []string
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if p, ok := strings.CutPrefix(line, "phase="); ok {
			phases = append(phases, p)
		}
	}
	return phases
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

// argValue returns the value following flag in argv, failing the test if absent.
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
// stands in for the real model: it logs its full argv + the phase it was invoked
// for, writes the canned contract files a real phase session would (gated by
// GOGO_STUB_MODE / GOGO_STUB_ERROR_PHASE), and prints a minimal
// `--output-format json` envelope to stdout — matching the RunResult shape
// (session_id/total_cost_usd/num_turns/duration_ms/is_error) launch.RunPhase parses.
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
# Stub claude for TestRunE2EStubClaude — records argv and writes canned contract
# files instead of spawning a real model. See cli/run_e2e_test.go.
set -euo pipefail

STATE_DIR="${GOGO_STUB_STATE:?GOGO_STUB_STATE not set}"
SLUG="${GOGO_STUB_SLUG:?GOGO_STUB_SLUG not set}"
MODE="${GOGO_STUB_MODE:-happy}"
ERROR_PHASE="${GOGO_STUB_ERROR_PHASE:-implement}"

mkdir -p "$STATE_DIR"
ARGV_LOG="$STATE_DIR/argv.log"
CALL_LOG="$STATE_DIR/calls.log"

{
  for a in "$@"; do printf '<<%s>>' "$a"; done
  printf '\n'
} >> "$ARGV_LOG"

CMD="${*: -1}"
echo "$CMD" >> "$CALL_LOG"

FEATURE_DIR=".gogo/work/feature-$SLUG"

phase="unknown"
case "$CMD" in
  *"/gogo:implement"*) phase="implement" ;;
  *"/gogo:review"*) phase="review" ;;
  *"/gogo:test"*) phase="test" ;;
  *"/gogo:report"*) phase="report" ;;
esac
echo "phase=$phase" >> "$CALL_LOG"

is_error="false"

case "$phase" in
  implement)
    mkdir -p "$FEATURE_DIR/implement"
    cat > "$FEATURE_DIR/implement/result.json" <<JSON
{"slug":"$SLUG","phase":"implement","status":"ok","round":1,"inputs":[],"outputs":[],"validated_in":true,"validated_out":true,"summary":"stub implement run"}
JSON
    if [ "$MODE" = "error" ] && [ "$ERROR_PHASE" = "implement" ]; then is_error="true"; fi
    ;;
  review)
    mkdir -p "$FEATURE_DIR/review"
    n=0
    [ -f "$STATE_DIR/review_calls" ] && n=$(cat "$STATE_DIR/review_calls")
    n=$((n+1))
    echo "$n" > "$STATE_DIR/review_calls"
    if [ "$MODE" = "warm-resume" ] && [ "$n" -eq 1 ]; then
      cat > "$FEATURE_DIR/review/issues.json" <<JSON
{"slug":"$SLUG","track":"review","round":1,"updated":"2026-07-11","issues":[{"id":"REV-901","title":"stub open major finding","description":"stub finding for the e2e dry run (warm-resume variant)","proposed_solution":"AGENT-FIXABLE: stub fix","severity":"major","priority":"P1","status":"open","origin":"review","found_in_round":1}]}
JSON
      cat > "$FEATURE_DIR/review/result.json" <<JSON
{"slug":"$SLUG","phase":"review","status":"ok","round":1,"inputs":[],"outputs":[],"validated_in":true,"validated_out":true,"open_issues":1,"summary":"stub review round 1 - one open major"}
JSON
    else
      cat > "$FEATURE_DIR/review/issues.json" <<JSON
{"slug":"$SLUG","track":"review","round":$n,"updated":"2026-07-11","issues":[]}
JSON
      cat > "$FEATURE_DIR/review/result.json" <<JSON
{"slug":"$SLUG","phase":"review","status":"ok","round":$n,"inputs":[],"outputs":[],"validated_in":true,"validated_out":true,"open_issues":0,"summary":"stub review clean"}
JSON
    fi
    if [ "$MODE" = "error" ] && [ "$ERROR_PHASE" = "review" ]; then is_error="true"; fi
    ;;
  test)
    mkdir -p "$FEATURE_DIR/test"
    cat > "$FEATURE_DIR/test/issues.json" <<JSON
{"slug":"$SLUG","track":"test","round":1,"updated":"2026-07-11","issues":[]}
JSON
    cat > "$FEATURE_DIR/test/result.json" <<JSON
{"slug":"$SLUG","phase":"test","status":"ok","round":1,"inputs":[],"outputs":[],"validated_in":true,"validated_out":true,"open_issues":0,"summary":"stub test clean"}
JSON
    if [ "$MODE" = "error" ] && [ "$ERROR_PHASE" = "test" ]; then is_error="true"; fi
    ;;
  report)
    : # the report phase's outputs are not read by the orchestrator; nothing to write
    ;;
esac

session_id="stub-session-$$-$RANDOM"
printf '{"session_id":"%s","total_cost_usd":0.01,"num_turns":1,"duration_ms":10,"is_error":%s}\n' "$session_id" "$is_error"
`
