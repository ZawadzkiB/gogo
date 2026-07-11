package contract

import "strings"

// RouteDecision is what the CLI process-orchestrator does after a review (③) or
// test (④) phase. See Route.
type RouteDecision int

const (
	Advance     RouteDecision = iota // clean/green → advance to the next phase
	ReImplement                      // open agent-fixable findings → warm dev resume with --issues
	Gate                             // a human call is needed → pause the queue (FR8)
)

func (d RouteDecision) String() string {
	switch d {
	case Advance:
		return "advance"
	case ReImplement:
		return "re-implement"
	case Gate:
		return "gate"
	default:
		return "unknown"
	}
}

// needsUserDecisionMarker is how review/test tag a finding whose fix is a HUMAN
// call, not an agent edit: a text tag inside proposed_solution. There is no typed
// field for it — the authority is agents/gogo-reviewer.md ("a proposed_solution
// tagged AGENT-FIXABLE or NEEDS-USER-DECISION").
const needsUserDecisionMarker = "NEEDS-USER-DECISION"

// Track names for Route (they select the per-track routing severity, below).
const (
	TrackReview = "review"
	TrackTest   = "test"
)

// OpenIssueCount counts every finding still awaiting a fix (status open/new),
// regardless of severity, falling back to the result's open_issues when no list is
// present. This is the plain "how many are still open" count (telemetry / test);
// routing uses routableOpen, which applies each track's severity rule.
func OpenIssueCount(result *PhaseResult, issues *IssuesList) int {
	if issues != nil {
		n := 0
		for _, is := range issues.Issues {
			if is.Status == "open" || is.Status == "new" {
				n++
			}
		}
		return n
	}
	if result != nil {
		return result.OpenIssues
	}
	return 0
}

// routableOpen counts the open/new findings that, per the TRACK's "④ Route" step,
// send the loop back to implement. The two tracks deliberately differ (verified
// against the skills, which are the authority):
//   - review (gogo-review/SKILL.md §④) routes only on open/new **blockers/majors**
//     and "batches the minors" — a lone open minor/nit is CLEAN and advances.
//   - test (gogo-test/SKILL.md §④) routes on **any** open/new issue.
//
// With no issues list, it falls back to the result's open_issues (severity split
// unavailable → coarse count).
func routableOpen(track string, result *PhaseResult, issues *IssuesList) int {
	if issues != nil {
		n := 0
		for _, is := range issues.Issues {
			if is.Status != "open" && is.Status != "new" {
				continue
			}
			if track == TrackReview && is.Severity != "blocker" && is.Severity != "major" {
				continue // review batches minors/nits
			}
			n++
		}
		return n
	}
	if result != nil {
		return result.OpenIssues
	}
	return 0
}

// hasOpenNeedsUserDecision reports whether any still-open finding is tagged
// NEEDS-USER-DECISION — a scope/design fork the agent must not guess at. The tag is
// free text (no typed field); gogo-reviewer pins it to proposed_solution, but the
// tester is less strict, so scan title + description + proposed_solution to be robust
// to where the LLM placed it (REV-004).
func hasOpenNeedsUserDecision(issues *IssuesList) bool {
	if issues == nil {
		return false
	}
	for _, is := range issues.Issues {
		if is.Status != "open" && is.Status != "new" {
			continue
		}
		blob := strings.ToUpper(is.Title + "\n" + is.Description + "\n" + is.ProposedSolution)
		if strings.Contains(blob, needsUserDecisionMarker) {
			return true
		}
	}
	return false
}

// Route decides what the orchestrator does after a review (③) or test (④) phase.
// It implements the SAME per-track rule as those skills' "④ Route" step — the single
// source of truth is skills/gogo-review/SKILL.md and skills/gogo-test/SKILL.md — so
// the Go orchestrator and the in-chat orchestrator route identically (plan constraint
// 3 / FR6). NOTE the tracks differ on severity (see routableOpen): review batches
// minors, test does not — passing the right `track` is what keeps the two aligned.
//
// The rule, in order:
//  1. The phase itself stopped at a fork (result.status waiting-for-user) or could
//     not repair a gate (blocked) → Gate. The judgment already happened in-session.
//  2. Some open finding is tagged NEEDS-USER-DECISION → Gate (a human call).
//  3. Track-routable open findings remain → ReImplement (warm dev resume with --issues).
//  4. Nothing routable open → Advance.
func Route(track string, result *PhaseResult, issues *IssuesList) RouteDecision {
	if result != nil {
		switch result.Status {
		case "waiting-for-user", "blocked":
			return Gate
		}
	}
	if hasOpenNeedsUserDecision(issues) {
		return Gate
	}
	if routableOpen(track, result, issues) > 0 {
		return ReImplement
	}
	return Advance
}
