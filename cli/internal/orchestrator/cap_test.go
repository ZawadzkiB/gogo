package orchestrator

import (
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// feat is a compact in-progress/other feature builder for the cap tests.
func feat(slug, root, class string) *contract.Feature {
	return &contract.Feature{Slug: slug, Root: root, Class: class}
}

// TestActiveWorkCount drives the cap counter off an injected session list (no real
// tmux - FR8): it counts a root's DISTINCT features that carry a live BUILD session
// (gogo-go-<slug>), excluding the target slug. Since 0.29.0 the feature's file-derived
// CLASS is not part of the rule - the class filter lied for the whole of a build, because
// state.md is written at a phase's exit (see ActiveWorkSlugs). The classes in this fixture
// are therefore incidental; what decides each row is its session.
func TestActiveWorkCount(t *testing.T) {
	const root = "/repos/app"
	repo := &contract.Repo{Features: []*contract.Feature{
		feat("alpha", root, contract.ClassInProgress),               // live BUILD session → counts
		feat("beta", root, contract.ClassInProgress),                // live BUILD session → counts
		feat("parked", root, contract.ClassInProgress),              // no session → not counted
		feat("ready", root, contract.ClassReadyToShip),              // a live DONE session → not counted (not a build)
		feat("other", "/repos/elsewhere", contract.ClassInProgress), // different root → not counted
	}}
	// Live sessions: alpha, beta, other (other is in a different root) as BUILDS, plus a
	// gogo-done-ready - so `ready` proves the ACTION test rather than the deleted class
	// filter (it is attributed to its slug, and still not counted). Sessions are named by
	// their leg action (go/plan/done/accept, SessionAction) - a feature in the review phase
	// is still driven by its warm gogo-go-<slug> session, so beta's session is gogo-go-beta
	// ("review" is not a session kind).
	sessions := []string{"gogo-go-alpha", "gogo-go-beta", "gogo-go-other", "gogo-done-ready"}

	// `ready` is what makes the fixture prove the ACTION test rather than the deleted class
	// filter, so assert that explicitly: its gogo-done session IS attributed to it (so it is
	// not being skipped for want of a session) and yet it is NOT a build. Without this pair
	// the extra fixture session is decoration - removing it would change no assertion, which
	// is exactly the "comment claims coverage its assertions do not provide" defect.
	if !liveSessionAttributed("ready", sessions) {
		t.Fatal("the gogo-done-ready fixture session is not attributed to `ready`, so this test cannot " +
			"distinguish 'excluded because it is not a build' from 'excluded because it has no session'")
	}
	if liveBuildSession("ready", sessions) {
		t.Error("liveBuildSession(ready) = true - a gogo-done session must not count as a build")
	}

	cases := []struct {
		name    string
		exclude string
		want    []string
	}{
		{"counts alpha+beta, excludes nothing", "", []string{"alpha", "beta"}},
		{"excludes the target slug (resume not blocked)", "alpha", []string{"beta"}},
		{"excluding a non-active slug leaves the count", "parked", []string{"alpha", "beta"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Assert the exact SLUGS, not just the tally: a count alone cannot tell "counted
			// alpha+beta" from "counted alpha+ready", so it would pass for the wrong reason.
			got := ActiveWorkSlugs(repo, root, sessions, c.exclude)
			if len(got) != len(c.want) {
				t.Fatalf("ActiveWorkSlugs(exclude=%q) = %v, want %v", c.exclude, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("ActiveWorkSlugs(exclude=%q) = %v, want %v", c.exclude, got, c.want)
					break
				}
			}
			if n := ActiveWorkCount(repo, root, sessions, c.exclude); n != len(c.want) {
				t.Errorf("ActiveWorkCount(exclude=%q) = %d, want %d", c.exclude, n, len(c.want))
			}
		})
	}
}

// liveSessionAttributed reports whether ANY session attributes to slug by the exact
// convention, regardless of action - the "is it even this feature's session" half that
// liveBuildSession then narrows to a build. Test-only: production code never wants the
// weaker question, which is why it is not in cap.go.
func liveSessionAttributed(slug string, sessions []string) bool {
	for _, s := range sessions {
		if launch.SessionMatchesSlug(s, slug) {
			return true
		}
	}
	return false
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

// TestActiveWorkParkedNotCounted: a feature with no live session is not clobbering the
// tree, so it is not counted (the count is "has a live BUILD session", per root).
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

// TestCapCountsLiveBuildRegardlessOfClass is FR12's safety fix, and the reason this plan
// exists. state.md is written at each phase's EXIT, so for the WHOLE of a build the file
// still reads `plan-accepted` and the feature classifies ClassUnfinished. The old
// `Class == ClassInProgress` filter therefore made a running build INVISIBLE to the cap, and
// a second `gogo go` in the same repo was allowed - two Claude sessions editing one working
// tree, the exact corruption the cap exists to prevent (the owner lock does not cover it: it
// is per-SLUG, not per-repo).
//
// It asserts the TALLY and the COUNTED SLUG, not merely "something was counted": a test that
// only checked "the second launch is refused" would still pass if it refused for the wrong
// reason (a different feature counted, or a cap misresolved).
func TestCapCountsLiveBuildRegardlessOfClass(t *testing.T) {
	const root = "/repos/app"
	// The pre-first-write window: demo is mid-build but its state.md still says
	// plan-accepted, so its class is unfinished - precisely the case that used to vanish.
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "demo", Root: root, Class: contract.ClassUnfinished, Status: "plan-accepted"},
		{Slug: "next", Root: root, Class: contract.ClassUnfinished, Status: "plan-accepted"},
	}}
	sessions := []string{"gogo-go-demo"}

	active := ActiveWorkSlugs(repo, root, sessions, "next")
	if len(active) != 1 || active[0] != "demo" {
		t.Fatalf("ActiveWorkSlugs = %v, want exactly [demo] - the live build under a stale "+
			"plan-accepted status must be counted", active)
	}
	if got := ActiveWorkCount(repo, root, sessions, "next"); got != 1 {
		t.Errorf("ActiveWorkCount = %d, want 1", got)
	}
	// …and with ConcurrentWorkItems 1 that is enough to refuse the second launch.
	if !CapExceeded(1, ActiveWorkCount(repo, root, sessions, "next")) {
		t.Error("CapExceeded(cap=1, active=1) = false - the second build in a busy repo was allowed")
	}
	// A resume of the building feature ITSELF is never blocked (it is excluded from its own
	// count), so dropping the class filter cannot make a resume unrunnable.
	if got := ActiveWorkCount(repo, root, sessions, "demo"); got != 0 {
		t.Errorf("ActiveWorkCount(exclude=demo) = %d, want 0 (a resume must not block itself)", got)
	}
}

// TestCapCountsOnlyBuildSessions pins the other half of FR12: only a `go` session is a
// BUILD. An AUTHORING session (gogo-plan-<slug>) must not count, or Slice B (count live
// sessions) would paper over Slice A (an item whose plan is still being written) and the cap
// would block a repo because an analyst is typing. Neither do accept / done / author /
// resume: none of them is a build.
func TestCapCountsOnlyBuildSessions(t *testing.T) {
	const root = "/repos/app"
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "authoring", Root: root, Class: contract.ClassUnfinished, Status: "awaiting-plan-acceptance"},
		{Slug: "accepting", Root: root, Class: contract.ClassUnfinished, Status: "awaiting-plan-acceptance"},
		{Slug: "shipping", Root: root, Class: contract.ClassReadyToShip, Status: "awaiting-uat"},
		{Slug: "authored", Root: root, Class: contract.ClassUnfinished},
		{Slug: "resumed", Root: root, Class: contract.ClassInProgress, Status: "implementing"},
		{Slug: "building", Root: root, Class: contract.ClassInProgress, Status: "implementing"},
	}}
	sessions := []string{
		"gogo-plan-authoring", "gogo-accept-accepting", "gogo-done-shipping",
		"gogo-author-authored", "gogo-resume-resumed", "gogo-go-building",
	}
	active := ActiveWorkSlugs(repo, root, sessions, "")
	if len(active) != 1 || active[0] != "building" {
		t.Errorf("ActiveWorkSlugs = %v, want exactly [building] - only a live `go` session is a build", active)
	}
}

// TestCapCountsATerminalFeatureStillHoldingABuildSession pins the deliberate consequence of
// dropping the class filter (REV-008): a shipped/done/aborted feature whose gogo-go session is
// STILL ALIVE is counted, where the old class filter always excluded it.
//
// That is intended, not an oversight. Review standard #9 records that "reaping a terminal
// feature's session is safe by definition" is FALSE - a just-shipped feature can still hold a
// live session - so a live BUILD session in that tree is a real claude editing a real working
// tree, which is the clobber risk the cap exists to name. Gating on TerminalStatus instead
// would put a file-derived condition back into the one guard FR12 made deterministic, with the
// same failure mode (an LLM-written status at a phase boundary).
func TestCapCountsATerminalFeatureStillHoldingABuildSession(t *testing.T) {
	const root = "/repos/app"
	shipped := &contract.Feature{Slug: "shipped-thing", Root: root, Class: contract.ClassShipped, Status: "shipped"}
	if !TerminalStatus(shipped.Status) {
		t.Fatal("the fixture is not terminal, so this test would not exercise the case it is named for")
	}
	repo := &contract.Repo{Features: []*contract.Feature{shipped}}

	active := ActiveWorkSlugs(repo, root, []string{"gogo-go-shipped-thing"}, "")
	if len(active) != 1 || active[0] != "shipped-thing" {
		t.Errorf("ActiveWorkSlugs = %v, want [shipped-thing] - a live BUILD session in the tree is the "+
			"clobber risk regardless of the feature's status; /gogo:done's ship-reap (or `gogo sweep`) is "+
			"what removes it", active)
	}
	// Once the session is gone - the normal case, after the ship-reap - it stops counting,
	// so a shipped feature never permanently consumes a slot.
	if got := ActiveWorkCount(repo, root, nil, ""); got != 0 {
		t.Errorf("ActiveWorkCount with the session reaped = %d, want 0", got)
	}
	// And a terminal feature's NON-build session (the gogo-done-<slug> that ships it) is
	// still not counted, so shipping something does not itself block the source.
	if got := ActiveWorkSlugs(repo, root, []string{"gogo-done-shipped-thing"}, ""); len(got) != 0 {
		t.Errorf("ActiveWorkSlugs with only a done session = %v, want none (a ship is not a build)", got)
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
