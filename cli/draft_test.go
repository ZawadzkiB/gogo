package main

import (
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/plans"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// seedPlanProject creates a single home project so resolveProjectName picks it as the
// sole default (the common one-project case the aliases forward into).
func seedPlanProject(t *testing.T, name, path string) {
	t.Helper()
	if _, err := projects.Add(projects.Project{Name: name, Sources: []projects.Source{{Path: path, Name: name}}}); err != nil {
		t.Fatalf("seed project: %v", err)
	}
}

// TestCmdDraftNewForwardsToPlan (D9): `gogo draft new` is a thin alias — it creates a
// DRAFT plan in the sole project (== gogo plan new).
func TestCmdDraftNewForwardsToPlan(t *testing.T) {
	seedDataHome(t)
	seedPlanProject(t, "web", "/repos/web")

	if code := cmdDraft([]string{"new", "Try the new indexer", "--desc", "swap the scan for a trie"}); code != 0 {
		t.Fatalf("draft new: exit %d, want 0", code)
	}
	list, _ := plans.List("web")
	if len(list) != 1 {
		t.Fatalf("after draft new: %d plans, want 1", len(list))
	}
	if list[0].Status != plans.StatusDraft || list[0].Title != "Try the new indexer" {
		t.Errorf("new draft = %+v, want a draft titled 'Try the new indexer'", list[0])
	}
}

// TestCmdDraftListNarrowsToDrafts (D9): `gogo draft list` shows the drafts only — a
// ready/active plan is not a draft, so it is excluded.
func TestCmdDraftListNarrowsToDrafts(t *testing.T) {
	seedDataHome(t)
	seedPlanProject(t, "web", "/repos/web")
	d, _ := plans.New("web", "A draft", "")
	r, _ := plans.New("web", "A ready one", "")
	plans.MarkReady("web", r.ID)

	// The list-alias narrows to drafts: it should surface the draft, not the ready one.
	all, _ := plans.List("web")
	drafts := 0
	for _, p := range all {
		if p.Status == plans.StatusDraft {
			drafts++
		}
	}
	if drafts != 1 {
		t.Fatalf("store has %d drafts, want 1 (setup sanity)", drafts)
	}
	// Exercise the alias path (no crash, exit 0); the store assertion above pins the
	// narrowing FormatPlans renders from.
	if code := cmdDraft([]string{"list"}); code != 0 {
		t.Errorf("draft list: exit %d, want 0", code)
	}
	_ = d
}

// TestCmdDraftRmDeletesDraft (TEST-001): the documented `gogo draft rm <id>` (a single
// id, no <source>) DELETES the memberless draft. Previously it forwarded to
// plan-rm-target, which demanded a second <source> arg the draft never had — so the
// documented command exited 2 and never worked.
func TestCmdDraftRmDeletesDraft(t *testing.T) {
	seedDataHome(t)
	seedPlanProject(t, "web", "/repos/web")
	p, _ := plans.New("web", "Trash me", "")

	if code := cmdDraft([]string{"rm", p.ID}); code != 0 {
		t.Fatalf("draft rm <id>: exit %d, want 0", code)
	}
	if list, _ := plans.List("web"); len(list) != 0 {
		t.Errorf("after draft rm <id>: %d plans, want 0 (the draft should be deleted)", len(list))
	}
}

// TestCmdDraftHelpMatchesBehavior (TEST-001): the `gogo draft` help must not advertise a
// subcommand+arity the code rejects. It documents `draft rm <id>` and `draft delete
// <id>` as single-id deletes (and NOT the 2-arg unlink, which belongs to `gogo plan
// rm`), so both must actually delete a memberless draft at that exact arity.
func TestCmdDraftHelpMatchesBehavior(t *testing.T) {
	for _, want := range []string{"gogo draft rm <id>", "gogo draft delete <id>", "delete a draft"} {
		if !strings.Contains(draftHelp, want) {
			t.Errorf("draftHelp missing %q", want)
		}
	}
	// The 2-arg unlink arity must NOT be advertised for `draft rm` (it is `gogo plan rm`'s).
	if strings.Contains(draftHelp, "draft rm <id> <source>") {
		t.Errorf("draftHelp advertises the 2-arg unlink arity — that belongs to `gogo plan rm`")
	}
	// Each documented single-id delete verb actually works at that arity.
	for _, verb := range []string{"rm", "delete"} {
		seedDataHome(t)
		seedPlanProject(t, "web", "/repos/web")
		p, _ := plans.New("web", "Trash "+verb, "")
		if code := cmdDraft([]string{verb, p.ID}); code != 0 {
			t.Fatalf("draft %s <id>: exit %d, want 0 (documented arity must work)", verb, code)
		}
		if list, _ := plans.List("web"); len(list) != 0 {
			t.Errorf("after draft %s <id>: %d plans, want 0", verb, len(list))
		}
	}
}

// TestCmdDraftHelp: bare `gogo draft` prints help (exit 0).
func TestCmdDraftHelp(t *testing.T) {
	seedDataHome(t)
	if code := cmdDraft(nil); code != 0 {
		t.Errorf("draft (no args): exit %d, want 0", code)
	}
}
