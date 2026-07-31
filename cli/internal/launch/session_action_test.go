package launch

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestSessionActionAllSixActions pins FR13's core promise: SessionAction returns the
// ACTION component of the gogo-<action>-<label> convention, for every action sessionName
// can mint. Before 0.29.0 the convention encoded the action and nothing returned it, so no
// consumer could tell an AUTHORING session (gogo-plan-<slug>) from a BUILD one
// (gogo-go-<slug>) - which is exactly the discrimination the concurrency cap needs.
func TestSessionActionAllSixActions(t *testing.T) {
	for _, want := range sessionActions {
		session := "gogo-" + string(want) + "-my-feature"
		got, ok := SessionAction(session, "my-feature")
		if !ok {
			t.Errorf("SessionAction(%q, %q) did not attribute", session, "my-feature")
			continue
		}
		if got != want {
			t.Errorf("SessionAction(%q) action = %q, want %q", session, got, want)
		}
	}
}

// TestSessionActionUnambiguous pins the ARGUMENT that makes the Action return safe: the
// match is on the literal "gogo-<action>-", and no action name contains a `-`, so a label
// that merely STARTS with another action's name cannot steal the action. `gogo-go-plan-foo`
// is action `go` on label `plan-foo`, never action `plan`.
func TestSessionActionUnambiguous(t *testing.T) {
	cases := []struct {
		session, slug string
		want          Action
	}{
		{"gogo-go-plan-foo", "plan-foo", ActionGo},
		{"gogo-plan-go-foo", "go-foo", ActionPlan},
		{"gogo-done-accept-me", "accept-me", ActionDone},
		{"gogo-accept-done-me", "done-me", ActionAccept},
		{"gogo-author-resume-x", "resume-x", ActionAuthor},
		{"gogo-resume-author-x", "author-x", ActionResume},
	}
	for _, c := range cases {
		got, ok := SessionAction(c.session, c.slug)
		if !ok || got != c.want {
			t.Errorf("SessionAction(%q, %q) = (%q, %v), want (%q, true)", c.session, c.slug, got, ok, c.want)
		}
	}
}

// TestSessionActionLabelCandidates pins that BOTH label transforms are tried, in 0.28.0's
// order (REV-009's back-compat widening must survive the refactor): a >MaxSessionLabel slug
// matches BOTH the bounded name 0.28.0+ mints and the unbounded name every release up to
// 0.27.0 minted - and the action is right in both.
func TestSessionActionLabelCandidates(t *testing.T) {
	// 60 chars, dash-free so sanitizeLabel's word-boundary cut cannot fire (a hard cut at
	// MaxSessionLabel), making the two candidates provably different strings.
	slug := strings.Repeat("abcdefghij", 6)
	if len(slug) <= MaxSessionLabel {
		t.Fatalf("fixture slug is %d chars, must exceed MaxSessionLabel (%d)", len(slug), MaxSessionLabel)
	}
	bounded := "gogo-go-" + sanitizeLabel(slug)
	unbounded := "gogo-go-" + unboundedLabel(slug)
	if bounded == unbounded {
		t.Fatalf("fixture did not produce two distinct candidates (%q)", bounded)
	}
	for _, session := range []string{bounded, unbounded} {
		got, ok := SessionAction(session, slug)
		if !ok {
			t.Errorf("SessionAction(%q) lost attribution for its own slug (the REV-009 upgrade path)", session)
		}
		if got != ActionGo {
			t.Errorf("SessionAction(%q) action = %q, want go", session, got)
		}
	}
}

// TestSessionActionMatchesLegacyBool is the byte-for-byte no-behaviour-change guard: over
// the whole TEST-005 case family (substring collisions, the -N collision suffix, the
// sanitize transform), SessionAction's bool return equals what SessionMatchesSlug answered
// before the refactor. The expectations are copied from TestSessionMatchesSlug, so the two
// can never drift.
func TestSessionActionMatchesLegacyBool(t *testing.T) {
	cases := []struct {
		session, slug string
		want          bool
	}{
		{"gogo-done-awaiting-card", "waiting-card", false},
		{"gogo-done-awaiting-card", "awaiting-card", true},
		{"gogo-go-oauth", "auth", false},
		{"gogo-go-oauth", "oauth", true},
		{"gogo-done-resync", "sync", false},
		{"gogo-go-a-b", "a", false},
		{"gogo-go-a-b", "a-b", true},
		{"gogo-go-my-feature", "my-feature", true},
		{"gogo-done-my-feature", "my-feature", true},
		{"gogo-go-a-2", "a", true},
		{"gogo-done-my-feature-3", "my-feature", true},
		{"gogo-go-a-2", "b", false},
		{"gogo-go-a-b", "a-2", false},
		{"gogo-go-a-bee", "a", false},
		{"gogo-done-weird-name", "Weird.Name", true},
		// Not a gogo session at all, and a truncated convention.
		{"my-own-tmux-session", "my-own", false},
		{"gogo-go-", "", false},
	}
	for _, c := range cases {
		_, ok := SessionAction(c.session, c.slug)
		if ok != c.want {
			t.Errorf("SessionAction(%q, %q) ok = %v, want %v", c.session, c.slug, ok, c.want)
		}
		if legacy := SessionMatchesSlug(c.session, c.slug); legacy != ok {
			t.Errorf("SessionMatchesSlug(%q, %q) = %v but SessionAction ok = %v - the delegation drifted",
				c.session, c.slug, legacy, ok)
		}
	}
}

// TestSessionActionCollisionSuffixKeepsAction: uniqueSession's "-<n>" suffix leaves the
// slug attribution fuzzy (gogo-go-foo-2 is either slug foo run 2, or slug foo-2) - but the
// ACTION is identical either way, which is the whole reason the Action return is safe where
// the bool inherits that fuzziness.
func TestSessionActionCollisionSuffixKeepsAction(t *testing.T) {
	for _, slug := range []string{"foo", "foo-2"} {
		got, ok := SessionAction("gogo-go-foo-2", slug)
		if !ok || got != ActionGo {
			t.Errorf("SessionAction(gogo-go-foo-2, %q) = (%q, %v), want (go, true)", slug, got, ok)
		}
	}
}

// TestHasSessionAction pins the shared read primitive the cap and the board cues use: it
// scans EVERY session (not just the first attributed one), because a slug can hold both an
// authoring and a build session at once and tmux's list order is not a contract.
func TestHasSessionAction(t *testing.T) {
	// The build session listed SECOND, behind an authoring session for the same slug: a
	// "first attributed match wins" implementation would answer "not building" here.
	sessions := []string{"gogo-plan-demo", "gogo-go-demo", "gogo-go-other"}
	if !HasSessionAction("demo", sessions, ActionGo) {
		t.Error("HasSessionAction(demo, go) = false - a build session behind an authoring one must still count")
	}
	if !HasSessionAction("demo", sessions, ActionPlan) {
		t.Error("HasSessionAction(demo, plan) = false")
	}
	if HasSessionAction("demo", sessions, ActionDone) {
		t.Error("HasSessionAction(demo, done) = true - no done session exists")
	}
	if HasSessionAction("other", sessions, ActionPlan) {
		t.Error("HasSessionAction(other, plan) = true - other only has a go session")
	}
	if HasSessionAction("demo", nil, ActionGo) {
		t.Error("HasSessionAction with no sessions = true (tmux-absent must be false)")
	}
}

// --- structural guards (unescapable by construction) ---------------------------------
//
// Two of FR13's rules are cross-site AGREEMENTS, so they are asserted against the SOURCE
// rather than by example: a future copy-paste then cannot reopen the hole (test-strategy.md,
// "prefer a guard that cannot be escaped").

// actionConstRe finds every `ActionX Action = "value"` line in launch.go's const block.
var actionConstRe = regexp.MustCompile(`(?m)^\s*(Action\w+)\s+Action\s*=\s*"([^"]*)"`)

// TestActionConstantsHaveNoDash is the FR13 invariant guard. SessionAction's parse is
// unambiguous ONLY because the action is the token between the first and second `-` after
// `gogo`, which requires that no action name contains a `-`. A future action like `plan-b`
// would silently reopen the ambiguity, so it fails HERE, at the const block, rather than
// producing quietly mis-parsed sessions.
func TestActionConstantsHaveNoDash(t *testing.T) {
	consts := actionConstants(t)
	if len(consts) < 6 {
		t.Fatalf("parsed only %d Action constants from launch.go (%v) - parser drift?", len(consts), consts)
	}
	for name, value := range consts {
		if strings.Contains(value, "-") {
			t.Errorf("Action constant %s = %q contains a '-'. SessionAction parses the action as the token "+
				"between the first and second '-' after \"gogo\", so a dashed action name makes the parse "+
				"ambiguous (it could not be told from a label that starts with another action's name). "+
				"Rename it, or rewrite SessionAction to parse a delimited action first.", name, value)
		}
		if value == "" {
			t.Errorf("Action constant %s is empty - \"gogo--<label>\" would match everything", name)
		}
	}
}

// TestSessionActionsCoversEveryAction guards the defect 0.28.0 shipped: sessionName can
// mint a session for ANY Action, but SessionAction only recognises the ones in
// sessionActions - an action missing from that list is a session no reader can attribute
// (no ● dot, `a` says "no running session", and the cap UNDER-counts it, which is the
// working-tree-clobber risk). Derived from the const block, so adding an Action without
// listing it fails immediately.
func TestSessionActionsCoversEveryAction(t *testing.T) {
	listed := map[Action]bool{}
	for _, a := range sessionActions {
		if listed[a] {
			t.Errorf("sessionActions lists %q twice", a)
		}
		listed[a] = true
	}
	for name, value := range actionConstants(t) {
		if !listed[Action(value)] {
			t.Errorf("Action %s (%q) is not in sessionActions, so SessionAction/SessionMatchesSlug cannot "+
				"attribute a gogo-%s-<slug> session: it would be invisible to the board's ● dot, to attach/peek, "+
				"and to the concurrency cap.", name, value, value)
		}
		delete(listed, Action(value))
	}
	for a := range listed {
		t.Errorf("sessionActions contains %q, which is not an Action constant in launch.go", a)
	}
}

// TestSessionMatchesSlugDelegates pins TEST-005's "exactly ONE parser" rule structurally:
// SessionMatchesSlug must be a delegation to SessionAction, not a second copy of the
// six-action × two-label loop. Re-inlining the loop is precisely how the two would drift
// (0.28.0 had to widen the label candidates in one place; a copy would have kept the old
// behaviour on one path).
func TestSessionMatchesSlugDelegates(t *testing.T) {
	body := funcBody(t, "SessionMatchesSlug")
	if !strings.Contains(body, "SessionAction(") {
		t.Errorf("SessionMatchesSlug no longer delegates to SessionAction; body:\n%s", body)
	}
	for _, banned := range []string{"for ", "range ", `"gogo-"`, "sanitizeLabel", "unboundedLabel", "sessionActions"} {
		if strings.Contains(body, banned) {
			t.Errorf("SessionMatchesSlug re-inlines the convention parse (found %q) - there must be exactly ONE "+
				"parser (TEST-005). Body:\n%s", banned, body)
		}
	}
}

// actionConstants reads launch.go and returns constName → value for every Action constant.
func actionConstants(t *testing.T) map[string]string {
	t.Helper()
	src := readLaunchSource(t)
	out := map[string]string{}
	for _, m := range actionConstRe.FindAllStringSubmatch(src, -1) {
		out[m[1]] = m[2]
	}
	return out
}

// funcBody returns the CODE of a top-level func in launch.go - its text between the opening
// `{` line and the closing `}` in column 0, with `//` comments stripped. Stripping matters:
// a structural guard must assert what the code DOES, and a comment that merely NAMES a banned
// construct (e.g. explaining why the loop lives elsewhere) would otherwise trip it.
func funcBody(t *testing.T, name string) string {
	t.Helper()
	src := readLaunchSource(t)
	i := strings.Index(src, "\nfunc "+name+"(")
	if i < 0 {
		t.Fatalf("func %s not found in launch.go", name)
	}
	rest := src[i+1:]
	if end := strings.Index(rest, "\n}\n"); end >= 0 {
		return stripLineComments(rest[:end])
	}
	t.Fatalf("could not find the end of func %s", name)
	return ""
}

// stripLineComments removes `//` line comments (ignoring a `//` inside a string literal) so
// the structural guards above read code rather than prose. It errs toward KEEPING text, so it
// can only make a guard stricter, never blind.
func stripLineComments(src string) string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		inStr := false
		for i := 0; i < len(line)-1; i++ {
			switch {
			case line[i] == '"' && (i == 0 || line[i-1] != '\\'):
				inStr = !inStr
			case !inStr && line[i] == '/' && line[i+1] == '/':
				line = line[:i]
				i = len(line)
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func readLaunchSource(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("launch.go")
	if err != nil {
		t.Fatalf("read launch.go: %v", err)
	}
	return string(raw)
}
