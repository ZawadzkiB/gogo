package orchestrator

import (
	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// The per-project concurrency cap (Phase 2). This is the ONE pure helper both
// launch paths share — `gogo go` (cmdGo) and the board `m`→go path (tui/move.go)
// — so they enforce the SAME rule without drift. It writes nothing (a read-side
// check that composes with the one-owner lock); the session list is passed in, so
// counting is fully unit-testable off a fake with no real tmux (FR8).

// ActiveWorkSlugs returns the DISTINCT slugs of root's features that are being
// actively worked: in-progress class (implement/review/test) AND carrying a live
// gogo-* session (exact SessionMatchesSlug attribution, never substring —
// TEST-005), excluding the target slug (so a resume of the feature being launched
// never counts against its own cap). Order follows repo.Features (newest-first).
//
// The clobber risk is a LIVE build session: a parked in-progress feature (no
// session) is not fighting over the working tree, so it is deliberately not
// counted. Features are matched by their OWN Root == root, so the aggregate board
// counts only the target project's features (a same-named live slug in another
// repo can still over-count — the known Phase-1 cross-repo limitation, P4's
// correlation id; documented, not solved here).
func ActiveWorkSlugs(repo *contract.Repo, root string, sessions []string, exclude string) []string {
	if repo == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, f := range repo.Features {
		if f == nil || f.Root != root || f.Slug == exclude {
			continue
		}
		if f.Class != contract.ClassInProgress {
			continue
		}
		if seen[f.Slug] || !liveSession(f.Slug, sessions) {
			continue
		}
		seen[f.Slug] = true
		out = append(out, f.Slug)
	}
	return out
}

// ActiveWorkCount is the number of distinct in-progress+live features in root,
// excluding the target — the count CapExceeded compares against the cap.
func ActiveWorkCount(repo *contract.Repo, root string, sessions []string, exclude string) int {
	return len(ActiveWorkSlugs(repo, root, sessions, exclude))
}

// CapForSource returns the ConcurrentWorkItems of the SOURCE whose Path == root,
// or 0 (unlimited) when root is not a registered source — the corrected
// project→sources model's cap resolver (FR2): the launch guard reads the source's
// per-source cap. 0 keeps the single-repo / unregistered fallback byte-for-byte.
// Both cap paths — the CLI `gogo go` guard and the board `m`→go path — resolve
// their sources from the projects store (projects.AllSources / the focused
// project's Sources) and share this one resolver so they never drift.
func CapForSource(sources []projects.Source, root string) int {
	for _, s := range sources {
		if s.Path == root {
			return s.ConcurrentWorkItems
		}
	}
	return 0
}

// CapExceeded reports whether a new go-launch would breach the cap: a positive
// cap with the active count already at or above it. cap == 0 (unlimited) NEVER
// blocks — the fallback-preserving sentinel (D2).
func CapExceeded(cap, active int) bool {
	return cap > 0 && active >= cap
}

// liveSession reports whether any session name attributes to slug by the exact
// gogo-<action>-<slug> convention (never a substring — TEST-005).
func liveSession(slug string, sessions []string) bool {
	for _, s := range sessions {
		if launch.SessionMatchesSlug(s, slug) {
			return true
		}
	}
	return false
}
