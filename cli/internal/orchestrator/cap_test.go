package orchestrator

import (
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// feat is a compact in-progress/other feature builder for the cap tests.
func feat(slug, root, class string) *contract.Feature {
	return &contract.Feature{Slug: slug, Root: root, Class: class}
}

// TestActiveWorkCount drives the cap counter off an injected session list (no real
// tmux — FR8): it counts a root's DISTINCT in-progress features that carry a live
// gogo-* session, excluding the target slug.
func TestActiveWorkCount(t *testing.T) {
	const root = "/repos/app"
	repo := &contract.Repo{Features: []*contract.Feature{
		feat("alpha", root, contract.ClassInProgress),               // in-progress + live → counts
		feat("beta", root, contract.ClassInProgress),                // in-progress + live → counts
		feat("parked", root, contract.ClassInProgress),              // in-progress, NO session → not counted
		feat("ready", root, contract.ClassReadyToShip),              // wrong class → not counted
		feat("other", "/repos/elsewhere", contract.ClassInProgress), // different root → not counted
	}}
	// Live sessions: alpha, beta, other (other is in a different root). Sessions are
	// named by their leg action (go/plan/done/accept, SessionMatchesSlug) — a feature
	// in the review phase is still driven by its warm gogo-go-<slug> session, so beta's
	// session is gogo-go-beta (not a "review" action, which is not a session kind).
	sessions := []string{"gogo-go-alpha", "gogo-go-beta", "gogo-go-other"}

	cases := []struct {
		name    string
		exclude string
		want    int
	}{
		{"counts alpha+beta, excludes nothing", "", 2},
		{"excludes the target slug (resume not blocked)", "alpha", 1},
		{"excluding a non-active slug leaves the count", "parked", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ActiveWorkCount(repo, root, sessions, c.exclude); got != c.want {
				t.Errorf("ActiveWorkCount(exclude=%q) = %d, want %d", c.exclude, got, c.want)
			}
		})
	}
}

// TestActiveWorkCountDistinct: two live sessions for the SAME slug (a base name +
// its collision suffix) count that feature ONCE (distinct features only).
func TestActiveWorkCountDistinct(t *testing.T) {
	const root = "/repos/app"
	repo := &contract.Repo{Features: []*contract.Feature{feat("dup", root, contract.ClassInProgress)}}
	sessions := []string{"gogo-go-dup", "gogo-go-dup-2"}
	if got := ActiveWorkCount(repo, root, sessions, ""); got != 1 {
		t.Errorf("distinct count = %d, want 1", got)
	}
}

// TestActiveWorkParkedNotCounted: an in-progress feature with no live session is
// not clobbering the tree, so it is not counted (the count is in-progress ∩ live).
func TestActiveWorkParkedNotCounted(t *testing.T) {
	const root = "/repos/app"
	repo := &contract.Repo{Features: []*contract.Feature{
		feat("live", root, contract.ClassInProgress),
		feat("parked", root, contract.ClassInProgress),
	}}
	if got := ActiveWorkCount(repo, root, []string{"gogo-go-live"}, ""); got != 1 {
		t.Errorf("count = %d, want 1 (parked feature not counted)", got)
	}
	if got := ActiveWorkCount(repo, root, nil, ""); got != 0 {
		t.Errorf("count with no sessions = %d, want 0", got)
	}
}

// TestCapForSource: the per-source cap is resolved by exact Path match; a repo
// that is not a registered source is 0 (unlimited fallback) — the corrected model.
func TestCapForSource(t *testing.T) {
	sources := []projects.Source{
		{Name: "app", Path: "/repos/app", ConcurrentWorkItems: 2},
		{Name: "lib", Path: "/repos/lib"}, // cap 0
	}
	if got := CapForSource(sources, "/repos/app"); got != 2 {
		t.Errorf("CapForSource(app) = %d, want 2", got)
	}
	if got := CapForSource(sources, "/repos/lib"); got != 0 {
		t.Errorf("CapForSource(lib) = %d, want 0", got)
	}
	if got := CapForSource(sources, "/repos/unregistered"); got != 0 {
		t.Errorf("CapForSource(unregistered) = %d, want 0 (fallback)", got)
	}
}

// TestCapExceeded: below/at/over the cap, and cap 0 never blocks.
func TestCapExceeded(t *testing.T) {
	cases := []struct {
		cap, active int
		want        bool
	}{
		{0, 0, false},  // unlimited
		{0, 99, false}, // unlimited even with many active
		{1, 0, false},  // below (a first launch)
		{1, 1, true},   // at cap → next launch refused
		{2, 1, false},  // below
		{2, 2, true},   // at
		{2, 3, true},   // over
	}
	for _, c := range cases {
		if got := CapExceeded(c.cap, c.active); got != c.want {
			t.Errorf("CapExceeded(cap=%d, active=%d) = %v, want %v", c.cap, c.active, got, c.want)
		}
	}
}
