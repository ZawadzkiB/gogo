package plans

import (
	"strings"
	"testing"
	"time"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
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

// --- project-UAT gate (FR3) -------------------------------------------------------

// memberRepo builds an in-memory board repo of (source, status, correlation) tuples so
// the pure MembersShippedIn / DerivedStatus / MarkDone tests never touch a source's
// on-disk .gogo/.
func memberRepo(feats ...*contract.Feature) *contract.Repo { return &contract.Repo{Features: feats} }

func feat(source, status, planID string) *contract.Feature {
	return &contract.Feature{Slug: source + "-item", Source: source, Status: status, Correlations: []string{planID}}
}

// TestMembersShippedInGuard: MembersShippedIn is all-shipped only when EVERY member's
// work item reads state.md status shipped/done; it names any unshipped member, and a
// memberless plan is never accepted.
func TestMembersShippedInGuard(t *testing.T) {
	p := Plan{ID: "plan-abcd1234", Status: StatusActive, Members: []Member{
		{Source: "web", SlugHint: "web-item"}, {Source: "api", SlugHint: "api-item"},
	}}

	// One member still building → not all shipped, names the api member.
	repo := memberRepo(feat("web", "shipped", p.ID), feat("api", "implementing", p.ID))
	if all, unshipped := MembersShippedIn("", p, repo); all || len(unshipped) != 1 || !strings.Contains(unshipped[0], "api") {
		t.Errorf("partial: all=%v unshipped=%v, want not-all + [api:…]", all, unshipped)
	}

	// Both shipped (one on the legacy `done` status) → all shipped.
	repo = memberRepo(feat("web", "shipped", p.ID), feat("api", "done", p.ID))
	if all, unshipped := MembersShippedIn("", p, repo); !all || len(unshipped) != 0 {
		t.Errorf("all-shipped: all=%v unshipped=%v, want all + none", all, unshipped)
	}

	// A wrong correlation id is not a member match → unshipped.
	repo = memberRepo(feat("web", "shipped", "plan-other"), feat("api", "shipped", p.ID))
	if all, _ := MembersShippedIn("", p, repo); all {
		t.Error("a feature carrying a DIFFERENT plan id must not count as this plan's member")
	}

	// A memberless plan is never all-shipped (nothing to accept).
	if all, _ := MembersShippedIn("", Plan{ID: p.ID, Status: StatusActive}, memberRepo()); all {
		t.Error("a memberless plan must not report all-shipped")
	}
}

// TestMembersShippedInCrossProjectGuard (REV-002): on a workspace-spanning repo a source
// NAME can collide across projects, so MembersShippedIn only counts a member whose feature
// belongs to the PLAN's project. A same-named source's SHIPPED feature in a DIFFERENT
// project must NOT satisfy the member (mirroring the guard tui.spawnedFeature carries),
// while a project-less feature (single-repo seam) still matches by (source, correlation).
func TestMembersShippedInCrossProjectGuard(t *testing.T) {
	p := Plan{ID: "plan-deadbeef", Status: StatusActive, Members: []Member{
		{Source: "web", SlugHint: "web-item"},
	}}
	withProject := func(source, project, status, planID string) *contract.Feature {
		f := feat(source, status, planID)
		f.Project = project
		return f
	}

	// The ONLY shipped `web` feature carrying the plan id lives in ANOTHER project — it must
	// not satisfy a member of a plan scoped to "app".
	crossOnly := memberRepo(withProject("web", "other", "shipped", p.ID))
	if all, unshipped := MembersShippedIn("app", p, crossOnly); all || len(unshipped) != 1 {
		t.Errorf("a same-named source's shipped feature in another project must NOT count: all=%v unshipped=%v", all, unshipped)
	}

	// The plan's OWN project's shipped feature (same source name) DOES satisfy it, even with
	// a sibling in another project still building.
	rightProject := memberRepo(
		withProject("web", "app", "shipped", p.ID),
		withProject("web", "other", "implementing", p.ID),
	)
	if all, unshipped := MembersShippedIn("app", p, rightProject); !all || len(unshipped) != 0 {
		t.Errorf("the plan's own project's shipped member must count: all=%v unshipped=%v", all, unshipped)
	}

	// Byte-for-byte fallback: a project-less feature (single-repo seam) still matches by
	// (source, correlation) regardless of the project arg.
	noProject := memberRepo(feat("web", "shipped", p.ID))
	if all, _ := MembersShippedIn("app", p, noProject); !all {
		t.Error("a project-less feature must still match by (source, correlation) — byte-for-byte fallback")
	}
}

// TestDerivedStatus: only an active plan with ≥1 member and all shipped derives
// awaiting-project-uat; every other combination returns the persisted status.
func TestDerivedStatus(t *testing.T) {
	withMember := Plan{Status: StatusActive, Members: []Member{{Source: "web", SlugHint: "web-item"}}}
	if got := DerivedStatus(withMember, true); got != StatusAwaitingProjectUAT {
		t.Errorf("active+member+shipped derived %q, want %s", got, StatusAwaitingProjectUAT)
	}
	if got := DerivedStatus(withMember, false); got != StatusActive {
		t.Errorf("active+member not-all-shipped derived %q, want active", got)
	}
	if got := DerivedStatus(Plan{Status: StatusReady, Members: []Member{{Source: "web"}}}, true); got != StatusReady {
		t.Errorf("a non-active plan must never derive the gate: got %q", got)
	}
	if got := DerivedStatus(Plan{Status: StatusActive}, true); got != StatusActive {
		t.Errorf("a memberless active plan must not derive the gate: got %q", got)
	}
}

// TestMarkDone: MarkDone appends a `## Project UAT` round to the plan body and flips the
// persisted status to done; a second accept increments the round number.
func TestMarkDone(t *testing.T) {
	seedDataHome(t)
	p, _ := New("proj", "Cross-repo migration", "the brief body")
	SetStatus("proj", p.ID, StatusActive)

	got, err := MarkDone("proj", p.ID)
	if err != nil {
		t.Fatalf("MarkDone: %v", err)
	}
	if got.Status != StatusDone {
		t.Errorf("after MarkDone status = %q, want done", got.Status)
	}
	if !strings.Contains(got.Description, "## Project UAT") || !strings.Contains(got.Description, "UAT round 1") {
		t.Errorf("MarkDone did not append the project-UAT round:\n%s", got.Description)
	}
	// The original brief is preserved (the round is appended, never a clobber).
	if !strings.Contains(got.Description, "the brief body") {
		t.Errorf("MarkDone clobbered the plan body:\n%s", got.Description)
	}
	// A second accept increments the round number (idempotent-ish append).
	again, _ := MarkDone("proj", p.ID)
	if !strings.Contains(again.Description, "UAT round 2") {
		t.Errorf("second MarkDone did not increment the round:\n%s", again.Description)
	}
}

// TestBriefFor pins the 0.25.0 FR2 per-source brief extractor: it returns the text
// under a `### <sourceName>` subsection of the `## Source briefs` section (the shape the
// gogo-project-plan analyst writes), "" when absent, ignores unrelated sections, and is
// case-insensitive on the source name.
func TestBriefFor(t *testing.T) {
	body := `## Goal
Cross-repo token migration.

## Source briefs
### web
Swap the web token client to the new store.
Touch: src/auth/*.ts. Accept: login still works.

### api
Expose the new token endpoint.

## Out of scope
Nothing here.`
	p := Plan{Description: body}

	cases := []struct {
		name, source, wantHas string
		wantEmpty             bool
	}{
		{"present-web", "web", "Swap the web token client", false},
		{"present-api", "api", "Expose the new token endpoint", false},
		{"case-insensitive", "WEB", "Swap the web token client", false},
		{"absent-source", "worker", "", true},
	}
	for _, c := range cases {
		got := BriefFor(p, c.source)
		if c.wantEmpty {
			if got != "" {
				t.Errorf("%s: BriefFor(%q) = %q, want empty", c.name, c.source, got)
			}
			continue
		}
		if !strings.Contains(got, c.wantHas) {
			t.Errorf("%s: BriefFor(%q) = %q, want it to contain %q", c.name, c.source, got, c.wantHas)
		}
		// The web brief must not bleed the api subsection or the Out-of-scope section.
		if c.source == "web" && (strings.Contains(got, "token endpoint") || strings.Contains(got, "Out of scope")) {
			t.Errorf("%s: BriefFor(web) leaked a neighbouring section: %q", c.name, got)
		}
	}

	// A plan with NO `## Source briefs` section yields "" for every source (the
	// hand-authored / n-drafted fallback path).
	plain := Plan{Description: "## Goal\nJust a goal, no briefs."}
	if got := BriefFor(plain, "web"); got != "" {
		t.Errorf("BriefFor on a brief-less plan = %q, want empty", got)
	}
}
