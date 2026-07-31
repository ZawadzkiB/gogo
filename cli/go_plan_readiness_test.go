package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
)

// scratchFeature writes a feature folder under a fresh repo root and chdirs into it,
// returning the root. planSections < 2 leaves the plan.md a stub; 0 writes none at all.
func scratchFeature(t *testing.T, slug, status string, planSections int) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".gogo", "work", "feature-"+slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	state := "# State - feature `" + slug + "`\n\n" +
		"- **feature:** scratch\n- **phase:** plan\n- **status:** " + status + "\n" +
		"- **created:** 2026-07-30\n- **open-decision:** none\n"
	if err := os.WriteFile(filepath.Join(dir, "state.md"), []byte(state), 0o644); err != nil {
		t.Fatalf("write state.md: %v", err)
	}
	if planSections > 0 {
		var b strings.Builder
		b.WriteString("# Plan\n\n")
		for i := 0; i < planSections; i++ {
			b.WriteString("## S")
			b.WriteByte(byte('a' + i))
			b.WriteString("\n\nx\n\n")
		}
		if err := os.WriteFile(filepath.Join(dir, "plan.md"), []byte(b.String()), 0o644); err != nil {
			t.Fatalf("write plan.md: %v", err)
		}
	}
	t.Chdir(root)
	return root
}

// TestCmdGoRefusesAcceptedButUnwrittenPlan pins FR8 on the CLI launch path: a plan-accepted
// feature whose plan.md is absent or a stub is refused BEFORE the cap check and before the
// owner lock, and the refusal names the exact reason (with its number for a stub) plus the
// unblock. Without this, `gogo go` would launch a build against a folder with no plan in it -
// reachable end to end, because the plan-acceptance gate can be cleared before the analyst
// writes the plan (state.md is written at a phase's exit).
func TestCmdGoRefusesAcceptedButUnwrittenPlan(t *testing.T) {
	cases := []struct {
		name         string
		planSections int
		wantReason   string
	}{
		{"no plan.md at all", 0, "no plan.md on disk yet"},
		{"a one-section stub", 1, "plan.md has 1 of the 2 sections a written plan needs"},
	}
	binDir := writeStubClaude(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			scratchFeature(t, "demo", "plan-accepted", c.planSections)
			// HERMETIC: cmdGo checks launch.HasClaude() BEFORE it loads the repo, so
			// without a stub on PATH this test passes only on a machine that happens to
			// have claude installed - and fails the release CI gate, which runs
			// `go test -race ./...` on a runner that does not (REV-001).
			stateDir := t.TempDir()
			wireStub(t, binDir, stateDir, "demo", "happy")
			stderr, code := captureStderr(t, func() int { return cmdGo([]string{"demo"}) })
			if code != 1 {
				t.Errorf("cmdGo = %d, want 1 (refused)", code)
			}
			for _, want := range []string{
				"demo is plan-accepted but its plan.md is not written",
				c.wantReason,
				"nothing to build",
				"gogo plan demo",
			} {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr missing %q; got:\n%s", want, stderr)
				}
			}
			// FR8 refuses BEFORE launching and before the owner lock, so the stub must
			// never have been exec'd. This is also what proves the claude stub did not
			// quietly turn the test into "claude ran and did nothing".
			if _, err := os.Stat(filepath.Join(stateDir, "argv.log")); err == nil {
				t.Error("a session was launched despite the unwritten-plan refusal")
			}
			if _, err := os.Stat(filepath.Join(".gogo", "resources", "cli", "locks")); err == nil {
				t.Error("the owner lock dir was created - the refusal must come before the lock")
			}
		})
	}
}

// TestCmdGoRunsAWrittenPlan is the compensation guard for the test above: the refusal must
// bite ONLY on an unwritten plan. With a written plan the readiness gate passes and cmdGo
// proceeds past it - proven by the failure it reaches instead (no claude on PATH), which is
// the NEXT guard in the function, not this one.
func TestCmdGoRunsAWrittenPlan(t *testing.T) {
	scratchFeature(t, "demo", "plan-accepted", 8)
	t.Setenv("PATH", t.TempDir()) // no claude → cmdGo stops at the claude guard, before the readiness gate
	stderr, _ := captureStderr(t, func() int { return cmdGo([]string{"demo"}) })
	if strings.Contains(stderr, "plan.md is not written") {
		t.Errorf("a WRITTEN 8-section plan was refused as unwritten:\n%s", stderr)
	}
	if !strings.Contains(stderr, "claude CLI not on PATH") {
		t.Errorf("expected to reach the claude guard (proving the readiness gate passed); stderr:\n%s", stderr)
	}
}

// TestRunnableHintForAuthoring pins the FR-level advice change: an AUTHORING item must not be
// told to "accept its plan" - that is the dead end this release closes. It is told to finish
// writing it, with the exact shortfall named.
func TestRunnableHintForAuthoring(t *testing.T) {
	authoring := &contract.Feature{Slug: "demo", Status: "awaiting-plan-acceptance", PlanUnwritten: true}
	got := runnableHint(authoring)
	for _, want := range []string{"still being written", "gogo plan demo"} {
		if !strings.Contains(got, want) {
			t.Errorf("runnableHint(authoring) = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "/gogo:accept") {
		t.Errorf("runnableHint(authoring) = %q - it must NOT send the user to accept a plan that does not exist", got)
	}
	// A real plan gate keeps the accept advice, byte-for-byte.
	gate := &contract.Feature{Slug: "demo", Status: "awaiting-plan-acceptance"}
	if got := runnableHint(gate); got != "accept its plan first (/gogo:accept), then re-run gogo go." {
		t.Errorf("runnableHint(a written plan gate) = %q - the existing hint regressed", got)
	}
	// Every other status is untouched.
	for status, want := range map[string]string{
		"awaiting-uat":     "UAT gate",
		"waiting-for-user": "paused on a decision",
		"shipped":          "already shipped",
		"":                 "run /gogo:plan and accept a plan first.",
	} {
		if got := runnableHint(&contract.Feature{Slug: "demo", Status: status}); !strings.Contains(got, want) {
			t.Errorf("runnableHint(%q) = %q, missing %q", status, got, want)
		}
	}
	if got := runnableHint(nil); got == "" {
		t.Error("runnableHint(nil) is empty - a vanished feature must still say something")
	}
}

// TestCmdGoAuthoringHintReachesTheUser wires the hint to the real command: an authoring item
// is not RunnableStatus, so `gogo go` refuses on status - and the hint it prints must be the
// authoring one, not the accept-a-plan dead end.
func TestCmdGoAuthoringHintReachesTheUser(t *testing.T) {
	scratchFeature(t, "demo", "awaiting-plan-acceptance", 0)
	// HERMETIC (REV-001): the claude-on-PATH guard runs before the repo is even loaded,
	// so without the stub this asserts nothing on a machine without claude.
	wireStub(t, writeStubClaude(t), t.TempDir(), "demo", "happy")
	stderr, code := captureStderr(t, func() int { return cmdGo([]string{"demo"}) })
	if code != 1 {
		t.Errorf("cmdGo = %d, want 1", code)
	}
	if !strings.Contains(stderr, "gogo plan demo") || !strings.Contains(stderr, "still being written") {
		t.Errorf("stderr does not carry the authoring hint:\n%s", stderr)
	}
	if strings.Contains(stderr, "/gogo:accept") {
		t.Errorf("stderr sends the user to accept a nonexistent plan:\n%s", stderr)
	}
}
