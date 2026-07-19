package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/plans"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what it wrote.
func captureStdout(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	code := fn()
	_ = w.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(r)
	return string(out), code
}

// TestCmdEpicNewAndAddForwardsToPlan (D9): `gogo epic new` creates a plan and
// `gogo epic add <id> <source>:<slug>` links a work item (a plan with members == an
// epic) — both forward to the project-scoped plan store.
func TestCmdEpicNewAndAddForwardsToPlan(t *testing.T) {
	seedDataHome(t)
	seedPlanProject(t, "web", "/repos/web")

	if code := cmdEpic([]string{"new", "Cross-repo token migration"}); code != 0 {
		t.Fatalf("epic new: exit %d, want 0", code)
	}
	list, _ := plans.List("web")
	if len(list) != 1 {
		t.Fatalf("after epic new: %d plans, want 1", len(list))
	}
	id := list[0].ID

	// add a work item (member) — the retroactive many-to-many link.
	if code := cmdEpic([]string{"add", id, "web:login-flow"}); code != 0 {
		t.Fatalf("epic add: exit %d, want 0", code)
	}
	p, _ := plans.Get("web", id)
	if len(p.Members) != 1 || p.Members[0] != (plans.Member{Source: "web", SlugHint: "login-flow"}) {
		t.Fatalf("member = %+v, want {web, login-flow}", p.Members)
	}

	// rm unlinks it.
	if code := cmdEpic([]string{"rm", id, "web:login-flow"}); code != 0 {
		t.Fatalf("epic rm: exit %d, want 0", code)
	}
	if p, _ := plans.Get("web", id); len(p.Members) != 0 {
		t.Errorf("after rm: members = %+v, want none", p.Members)
	}
}

// TestCmdEpicShowAndDelete: `show` prints the plan; `delete` removes it.
func TestCmdEpicShowAndDelete(t *testing.T) {
	seedDataHome(t)
	seedPlanProject(t, "web", "/repos/web")
	p, _ := plans.New("web", "Doomed", "temporary")

	if code := cmdEpic([]string{"show", p.ID}); code != 0 {
		t.Errorf("epic show: exit %d, want 0", code)
	}
	if code := cmdEpic([]string{"delete", p.ID}); code != 0 {
		t.Fatalf("epic delete: exit %d, want 0", code)
	}
	if _, ok := plans.Get("web", p.ID); ok {
		t.Error("plan still present after epic delete")
	}
}

// TestCmdEpicListShowsMemberBearingRegardlessOfStatus pins REV-003: `epic add` links a
// member WITHOUT flipping status (the plan stays draft/ready), so `epic list` must
// narrow by MEMBERSHIP, not `status==active` — otherwise a just-linked epic vanishes
// from its own list. A plain draft (no members, no targets) is still excluded.
func TestCmdEpicListShowsMemberBearingRegardlessOfStatus(t *testing.T) {
	seedDataHome(t)
	seedPlanProject(t, "web", "/repos/web")

	// A plan that gets a member via `epic add` — stays DRAFT (add never flips status).
	linked, _ := plans.New("web", "Linked epic", "")
	if code := cmdEpic([]string{"add", linked.ID, "web:login-flow"}); code != 0 {
		t.Fatalf("epic add: exit %d, want 0", code)
	}
	if got, _ := plans.Get("web", linked.ID); got.Status != plans.StatusDraft {
		t.Fatalf("epic add flipped status to %q, want it to stay draft (setup sanity)", got.Status)
	}

	// A plain draft with no members and no targets — must NOT appear in `epic list`.
	plain, _ := plans.New("web", "Just a draft", "")

	out, code := captureStdout(t, func() int { return cmdEpic([]string{"list"}) })
	if code != 0 {
		t.Fatalf("epic list: exit %d, want 0", code)
	}
	if !strings.Contains(out, linked.ID) {
		t.Errorf("epic list omitted the member-bearing draft %s (REV-003):\n%s", linked.ID, out)
	}
	if strings.Contains(out, plain.ID) {
		t.Errorf("epic list included the memberless/targetless draft %s (should be excluded):\n%s", plain.ID, out)
	}
}

// TestCmdEpicUnknownSubcommand: an unknown verb is a usage error (exit 2).
func TestCmdEpicUnknownSubcommand(t *testing.T) {
	seedDataHome(t)
	seedPlanProject(t, "web", "/repos/web")
	if code := cmdEpic([]string{"frobnicate"}); code != 2 {
		t.Errorf("epic frobnicate: exit %d, want 2", code)
	}
}
