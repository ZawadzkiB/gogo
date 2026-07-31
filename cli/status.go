package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
)

// classRank fixes the status table's row order (goldenable, independent of
// dates): shipped → ready-to-ship → in-progress → unfinished.
func classRank(class string) int {
	switch class {
	case contract.ClassShipped:
		return 0
	case contract.ClassReadyToShip:
		return 1
	case contract.ClassInProgress:
		return 2
	default:
		return 3
	}
}

// cmdStatus prints the classifier table in a stable column order.
func cmdStatus(_ []string) int {
	root, ok := findRoot()
	if !ok {
		return 1
	}
	repo, _ := contract.LoadRepo(root)
	// FR14: the headless table must not lie either. `gogo status` called ListSessions()
	// ZERO times before 0.29.0, which is why a display-layer-only fix was rejected: a card
	// mid-build reads its pre-build status on disk, and with no session awareness the table
	// had no way to say otherwise. tmux is a SOFT dep, so an absent tmux returns nil here
	// and the table renders exactly as it did before (FormatStatus's own fallback).
	fmt.Print(FormatStatusSessions(repo, launch.ListSessions()))
	return 0
}

// FormatStatus renders the work-index as a deterministic plain-text table.
// Exposed so a golden test can pin it.
func FormatStatus(repo *contract.Repo) string { return FormatStatusSessions(repo, nil) }

// FormatStatusSessions is FormatStatus plus the live-session column (FR14). It takes the
// session list as an ARGUMENT (never calling ListSessions itself) so the table stays a
// pure function of its inputs and the golden test cannot be perturbed by whatever the
// developer happens to have running.
//
// An EMPTY session set renders the table byte-for-byte as pre-0.29.0 - no LIVE column at
// all. That is the tmux-absent / nothing-running fallback: a column of nothing but "-"
// would widen every row to report an absence, and the portability contract wants an absent
// soft dep to be invisible, not to reshape the output.
func FormatStatusSessions(repo *contract.Repo, sessions []string) string {
	feats := append([]*contract.Feature(nil), repo.Features...)
	sort.SliceStable(feats, func(i, j int) bool {
		if ri, rj := classRank(feats[i].Class), classRank(feats[j].Class); ri != rj {
			return ri < rj
		}
		return feats[i].Slug < feats[j].Slug
	})

	var counts [4]int
	for _, f := range feats {
		counts[classRank(f.Class)]++
	}

	var b strings.Builder
	fmt.Fprintf(&b, "gogo — %d features  (shipped %d · ready %d · in-progress %d · unfinished %d)\n\n",
		len(feats), counts[0], counts[1], counts[2], counts[3])
	if len(sessions) == 0 {
		fmt.Fprintf(&b, "%-5s %-14s %-10s %-18s %-28s %s\n", "WAIT", "CLASS", "PHASE", "STATUS", "ITERATIONS", "SLUG")
		b.WriteString(strings.Repeat("─", 102) + "\n")
		for _, f := range feats {
			fmt.Fprintf(&b, "%-5s %-14s %-10s %-18s %-28s %s\n",
				waitMarker(f), f.Class, dash(f.Phase), dash(f.Status), dash(f.Iterations), f.Slug)
		}
		return b.String()
	}
	fmt.Fprintf(&b, "%-5s %-9s %-14s %-10s %-18s %-28s %s\n", "WAIT", "LIVE", "CLASS", "PHASE", "STATUS", "ITERATIONS", "SLUG")
	b.WriteString(strings.Repeat("─", 112) + "\n")
	for _, f := range feats {
		fmt.Fprintf(&b, "%-5s %-9s %-14s %-10s %-18s %-28s %s\n",
			waitMarker(f), liveMarker(f, sessions), f.Class, dash(f.Phase), dash(f.Status), dash(f.Iterations), f.Slug)
	}
	return b.String()
}

// liveMarker names what a feature's live session is DOING (FR14), so the headless table
// can show work in flight that the feature's own state.md has not caught up with yet:
// `building` for a live `gogo-go-<slug>` session, `authoring` for a `gogo-plan-<slug>` one,
// `live` for any other attributed session, `-` for none. Words, not glyphs, because this
// table is greppable output - `gogo status | grep building` is the point.
func liveMarker(f *contract.Feature, sessions []string) string {
	switch {
	case launch.HasSessionAction(f.Slug, sessions, launch.ActionGo):
		return "building"
	case launch.HasSessionAction(f.Slug, sessions, launch.ActionPlan):
		return "authoring"
	}
	for _, s := range sessions {
		if launch.SessionMatchesSlug(s, f.Slug) {
			return "live"
		}
	}
	return "-"
}

// waitMarker flags a feature parked at a genuine user gate (WaitingForInput) in
// the status table's dedicated leading WAIT column — a greppable, golden-stable
// signal that the item blocks on the user (FR-B3). An AUTHORING item (0.29.0) is not a
// user gate, so it reads `-` here: nothing is waiting on the user until the plan exists.
func waitMarker(f *contract.Feature) string {
	if f.WaitingForInput() {
		return "WAIT"
	}
	return "-"
}

func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
