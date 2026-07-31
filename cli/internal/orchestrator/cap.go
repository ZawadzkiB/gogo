package orchestrator

import (
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// The per-project concurrency cap (Phase 2). This is the ONE pure helper both
// launch paths share — `gogo go` (cmdGo) and the board `m`→go path (tui/move.go)
// — so they enforce the SAME rule without drift. It writes nothing (a read-side
// check that composes with the one-owner lock); the session list is passed in, so
// counting is fully unit-testable off a fake with no real tmux (FR8).

// CapRuleClause is the ONE user-visible statement of what the cap counts, shared by every
// surface that says it out loud: the board's cap bounce (tui/move.go), the source-edit
// form's cap field and the source-detail note (tui/config_tab.go), and `gogo --help`
// (main.go).
//
// It is a constant rather than four hand-written sentences because the four-copy version
// already failed: 0.28.0 deliberately wrote the rule in all four places, FR12 changed the
// rule, and only the bounce was updated - leaving the form the user reads WHILE SETTING the
// cap stating the opposite of what the code does. Single-sourcing it makes that class of
// drift impossible instead of merely forbidden. cap_rule_test.go pins that all four
// surfaces carry it and that no stale phrasing survives.
const CapRuleClause = "counts work items with a live build session (gogo-go-<slug>), per source; " +
	"an authoring session, any other session, and plans are never counted"

// CapSweepRemedy is the ONE phrasing of the "a blocker already shipped" unblock, shared by
// both cap refusals (the board's capBounce and `gogo go`'s capBlock) exactly as
// CapRuleClause is. Because the cap counts a live build session regardless of the feature's
// status, a blocker can be an already-SHIPPED item whose session outlived a failed
// ship-reap - and for that one "ship one" is impossible advice.
//
// It always names the TARGETED form, `gogo sweep <slug>...`, and never the bare one. That is
// not a style preference, it is a safety rule: a bare `gogo sweep` is HOST-GLOBAL. With
// Sweeper.Only empty, every `gogo-*` session on the machine is judged against THIS repo's
// feature list, so another source's in-flight build - which has no owning feature here - is
// classified "orphan - no owning feature" and killed, with no confirmation. Recommending the
// bare form inline as the fix for one named blocker strips exactly the context sweepHelp
// provides, and it costs nothing to be precise: both call sites already know the blocking
// slugs. `gogo sweep a b` is the same targeted form for several blockers.
//
// Returns "" for an empty slug list - there is no targeted command to name, and emitting a
// bare `gogo sweep` instead is the whole thing this function exists to prevent. Callers must
// therefore DROP an empty remedy rather than interpolating it, which is what CapRefusal below
// does for both of them (an unconditional `%s` between two commas would render ", ,").
func CapSweepRemedy(blocking []string) string {
	if len(blocking) == 0 {
		return ""
	}
	return "run `gogo sweep " + strings.Join(blocking, " ") + "` if a blocker already shipped"
}

// CapRefusal joins a cap refusal's remedies into one clause, dropping any that are empty. It
// is what makes CapSweepRemedy's "" contract real at both call sites rather than merely
// documented (REV-019): today the empty case is unreachable - both sites sit behind
// CapExceeded(cap, len(active)), which needs len(active) >= cap >= 1 - but the "" return
// exists precisely so the function cannot degrade into naming the host-global bare sweep, and
// a future caller (a plans-tab cap message, a `gogo status` hint) would meet the empty case
// first. Keeping the composition here means neither refusal can reintroduce the double comma.
func CapRefusal(remedies ...string) string {
	kept := make([]string, 0, len(remedies))
	for _, r := range remedies {
		if r != "" {
			kept = append(kept, r)
		}
	}
	return strings.Join(kept, ", ")
}

// ActiveWorkSlugs returns the DISTINCT slugs of root's features that are being actively
// BUILT: carrying a live `gogo-go-<slug>` BUILD session (exact SessionAction attribution,
// never substring - TEST-005), excluding the target slug (so a resume of the feature
// being launched never counts against its own cap). Order follows repo.Features
// (newest-first).
//
// The clobber risk is a LIVE BUILD SESSION, and that is now the whole rule (FR12, D5=A).
// A parked feature (no session) is not fighting over the working tree, so it is
// deliberately not counted; neither is an AUTHORING (`gogo-plan-<slug>`) session, nor an
// accept/done/author/resume one - none of them is a build.
//
// The `Class == ClassInProgress` filter this used to carry was REMOVED because it
// contradicted the rationale above and lied exactly when it mattered. state.md is
// written at each phase's EXIT, so for the whole of a build the file still reads
// `plan-accepted` and the feature classifies ClassUnfinished - it failed the class test,
// vanished from this count, and the per-source cap then let a SECOND build start in the
// same repo and clobber the working tree, which is the precise safety property this cap
// exists to protect. (The per-slug owner lock does not cover it: that lock is per-slug,
// not per-repo.) Moving the phases' state write to phase ENTRY shrinks that window but
// cannot close it - a launch-to-first-write gap always remains, and it is LLM prose -
// so the guard keys on the deterministic signal instead.
//
// A TERMINAL feature (shipped / done / aborted) still holding a live build session IS
// counted, and that is deliberate (REV-008). The repo's own review standard #9 records that
// "reaping a terminal feature's session is safe by definition" is FALSE - a just-shipped
// feature can still hold the very session that is shipping it - so a live `gogo-go-<slug>` in
// that tree is a real claude editing a real working tree, which is exactly the clobber risk
// this cap names. Gating on !TerminalStatus(f.Status) would re-introduce a file-derived
// condition with the same failure mode FR12 removed (a status written by an LLM at a phase
// boundary). Normally /gogo:done's targeted ship-reap removes the session; if that
// best-effort reap fails, a TARGETED `gogo sweep <slug>` clears it (never the bare form - see
// CapSweepRemedy) - and both cap refusals NAME the blocking slug, so a stale blocker is
// visible rather than mysterious.
//
// Features are matched by their OWN Root == root, so the aggregate board counts only the
// target project's features (a same-named live slug in another repo can still over-count
// - the known cross-repo limitation, documented and deliberately untouched here: it is
// the OPPOSITE direction from the under-count fixed above).
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
		if seen[f.Slug] || !liveBuildSession(f.Slug, sessions) {
			continue
		}
		seen[f.Slug] = true
		out = append(out, f.Slug)
	}
	return out
}

// ActiveWorkCount is the number of distinct live-build features in root,
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

// liveBuildSession reports whether slug holds a live BUILD session - a session named
// for the `go` action by the exact gogo-<action>-<slug> convention (never a substring -
// TEST-005). The action test is what keeps an AUTHORING (`gogo-plan-<slug>`) session out
// of the cap, so counting live sessions can never paper over a plan that is still being
// written.
func liveBuildSession(slug string, sessions []string) bool {
	return launch.HasSessionAction(slug, sessions, launch.ActionGo)
}
