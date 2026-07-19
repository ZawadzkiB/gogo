package plans

import (
	"testing"
	"time"
)

// seedDataHome points the DATA home (projects.Home, and thus the plans dir) at a
// fresh t.TempDir() via the GOGO_DATA_HOME seam so no test touches the real ~/.gogo.
func seedDataHome(t *testing.T) {
	t.Helper()
	t.Setenv("GOGO_DATA_HOME", t.TempDir())
}

// TestMintIDDeterministic: the id is a deterministic plan-<hex8> from (title,
// instant) — the same inputs mint the same id (migration idempotence + testability),
// and it is [a-z0-9-] only (filter/tmux-safe).
func TestMintIDDeterministic(t *testing.T) {
	instant := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	a := MintID("Cross-repo token migration", instant)
	b := MintID("Cross-repo token migration", instant)
	if a != b {
		t.Fatalf("MintID not deterministic: %q vs %q", a, b)
	}
	if len(a) != len("plan-")+8 || a[:5] != "plan-" {
		t.Errorf("id = %q, want plan-<hex8>", a)
	}
	if MintID("Different title", instant) == a {
		t.Error("different titles must mint different ids")
	}
}

// TestNewListGetRoundTrip: New mints a draft plan, List/Get round-trip it back
// intact, and a fresh store lists nothing (defensive empty).
func TestNewListGetRoundTrip(t *testing.T) {
	seedDataHome(t)
	if list, _ := List("proj"); len(list) != 0 {
		t.Fatalf("fresh store lists %d plans, want 0", len(list))
	}
	p, err := New("proj", "Token migration", "move the shared token store")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Status != StatusDraft || p.ID[:5] != "plan-" {
		t.Errorf("new plan = %+v, want a draft with a plan-<hash> id", p)
	}
	got, ok := Get("proj", p.ID)
	if !ok {
		t.Fatal("Get did not find the new plan")
	}
	if got.Title != "Token migration" || got.Description != "move the shared token store" {
		t.Errorf("round-trip = %+v, want title+body preserved", got)
	}
	if list, _ := List("proj"); len(list) != 1 {
		t.Errorf("List = %d plans, want 1", len(list))
	}
}

// TestManyToManyMembers: AddMember/RemoveMember are idempotent SET ops keyed on
// (Source, SlugHint), and adding a member ensures the source is a target.
func TestManyToManyMembers(t *testing.T) {
	seedDataHome(t)
	p, _ := New("proj", "Wire up", "")
	m1 := Member{Source: "web", SlugHint: "login"}
	m2 := Member{Source: "api", SlugHint: "token"}

	if _, err := AddMember("proj", p.ID, m1); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	AddMember("proj", p.ID, m2)
	AddMember("proj", p.ID, m1) // duplicate — idempotent no-op

	got, _ := Get("proj", p.ID)
	if len(got.Members) != 2 {
		t.Fatalf("members = %+v, want exactly 2 (idempotent add)", got.Members)
	}
	// AddMember ensures the source became a target.
	if !containsStr(got.Targets, "web") || !containsStr(got.Targets, "api") {
		t.Errorf("targets = %v, want web+api ensured from members", got.Targets)
	}

	// Remove one member; the other stays.
	removed, _ := RemoveMember("proj", p.ID, m1)
	if !removed {
		t.Fatal("RemoveMember(m1) = false, want removed")
	}
	if got, _ := Get("proj", p.ID); len(got.Members) != 1 || got.Members[0] != m2 {
		t.Errorf("after remove: members = %+v, want just m2", got.Members)
	}
	// Removing an absent member is a graceful no-op.
	if removed, _ := RemoveMember("proj", p.ID, Member{Source: "ghost", SlugHint: "x"}); removed {
		t.Error("RemoveMember(absent) = true, want a no-op")
	}
}

// TestStatusTransitions: SetStatus/MarkReady walk the lifecycle and a bogus status
// normalizes to draft; Delete is a graceful no-op when absent.
func TestStatusTransitions(t *testing.T) {
	seedDataHome(t)
	p, _ := New("proj", "Lifecycle", "")
	if p.Status != StatusDraft {
		t.Fatalf("new plan status = %q, want draft", p.Status)
	}
	if got, _ := MarkReady("proj", p.ID); got.Status != StatusReady {
		t.Errorf("MarkReady status = %q, want ready", got.Status)
	}
	if got, _ := SetStatus("proj", p.ID, StatusActive); got.Status != StatusActive {
		t.Errorf("SetStatus(active) = %q, want active", got.Status)
	}
	if got, _ := SetStatus("proj", p.ID, "bogus"); got.Status != StatusDraft {
		t.Errorf("SetStatus(bogus) = %q, want normalized to draft", got.Status)
	}

	if removed, _ := Delete("proj", p.ID); !removed {
		t.Fatal("Delete = false, want removed")
	}
	if _, ok := Get("proj", p.ID); ok {
		t.Error("plan still present after Delete")
	}
	if removed, _ := Delete("proj", p.ID); removed {
		t.Error("Delete(already gone) = true, want a no-op")
	}
}

// TestTargets: AddTarget/RemoveTarget are idempotent SET ops.
func TestTargets(t *testing.T) {
	seedDataHome(t)
	p, _ := New("proj", "Targets", "")
	AddTarget("proj", p.ID, "web")
	AddTarget("proj", p.ID, "web") // idempotent
	AddTarget("proj", p.ID, "api")
	if got, _ := Get("proj", p.ID); len(got.Targets) != 2 {
		t.Fatalf("targets = %v, want exactly [web api]", got.Targets)
	}
	if removed, _ := RemoveTarget("proj", p.ID, "web"); !removed {
		t.Error("RemoveTarget(web) = false, want removed")
	}
	if got, _ := Get("proj", p.ID); len(got.Targets) != 1 || got.Targets[0] != "api" {
		t.Errorf("after remove: targets = %v, want [api]", got.Targets)
	}
}

// TestSaveRefusesInvalidID: the write-scope guard refuses a path-escaping id.
func TestSaveRefusesInvalidID(t *testing.T) {
	seedDataHome(t)
	if err := Save("proj", Plan{ID: "../pwn", Title: "x"}); err == nil {
		t.Error("Save with a path-escaping id must be refused")
	}
	if _, ok := Get("proj", "../pwn"); ok {
		t.Error("Get with a path-escaping id must be refused")
	}
}
