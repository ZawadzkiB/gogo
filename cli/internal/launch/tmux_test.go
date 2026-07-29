package launch

import (
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// --- 0.28.0 launch diagnostics (plans-tab-launch-diagnostics-and-view) --------
//
// Every test here is pure argv/string work - no tmux is spawned, which is exactly
// the seam the package already exposes (BuildIntent / *Args / SessionMatchesSlug).

// TestTmuxErrorCarriesStderr pins FR1.1: the typed error names the tmux
// subcommand, the exit status AND tmux's own stderr words, and Unwrap still
// reaches the underlying *exec.ExitError so errors.As/Is keep working.
func TestTmuxErrorCarriesStderr(t *testing.T) {
	// A real, cheap non-zero exit so the wrapped error is a genuine *exec.ExitError.
	inner := exec.Command("sh", "-c", "exit 1").Run()
	if inner == nil {
		t.Fatal("probe command unexpectedly succeeded")
	}
	err := &TmuxError{Sub: "new-session", Args: []string{"new-session", "-d"}, Stderr: "command too long\n", Err: inner}

	got := err.Error()
	for _, want := range []string{"tmux new-session failed", "exit status 1", "command too long"} {
		if !strings.Contains(got, want) {
			t.Errorf("TmuxError.Error() = %q, missing %q", got, want)
		}
	}
	// The exact reading the plan pins (FR1.1).
	if got != "tmux new-session failed: exit status 1: command too long" {
		t.Errorf("TmuxError.Error() = %q, want the sub + exit + stderr reading", got)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("errors.As did not reach the *exec.ExitError through Unwrap")
	}

	// No stderr captured (tmux said nothing) → the error still reads cleanly.
	bare := &TmuxError{Sub: "kill-session", Err: inner}
	if bare.Error() != "tmux kill-session failed: exit status 1" {
		t.Errorf("bare TmuxError = %q", bare.Error())
	}
}

// TestBoundedBufferCapsCapture: the stderr capture is bounded (FR1.1) and never
// fails the child's write, so a chatty/garbled tmux can neither exhaust memory nor
// break an invocation that would otherwise have succeeded.
func TestBoundedBufferCapsCapture(t *testing.T) {
	b := &boundedBuffer{limit: 8}
	n, err := b.Write([]byte("0123456789abcdef"))
	if err != nil {
		t.Fatalf("bounded write returned an error: %v", err)
	}
	if n != 16 {
		t.Errorf("Write reported %d bytes consumed, want the full 16 (never short-write the child)", n)
	}
	if b.String() != "01234567" {
		t.Errorf("buffer = %q, want the first 8 bytes only", b.String())
	}
	b.Write([]byte("more"))
	if b.String() != "01234567" {
		t.Errorf("buffer grew past the limit: %q", b.String())
	}
}

// TestTmuxCommandBytesAndLimit pins FR1.2: the measured limit and the byte
// accounting the preflight uses (every argv element plus one separator between
// them - the size of the command line tmux actually receives).
func TestTmuxCommandBytesAndLimit(t *testing.T) {
	if MaxTmuxCommandBytes != 16317 {
		t.Errorf("MaxTmuxCommandBytes = %d, want the measured 16317 (tmux 3.7b: last OK 16317, first failure 16318)", MaxTmuxCommandBytes)
	}
	cases := []struct {
		argv []string
		want int
	}{
		{nil, 0},
		{[]string{"abc"}, 3},      // one element, no separator
		{[]string{"ab", "cd"}, 5}, // 2+2 + one separator
		{[]string{"new-session", "-d", "-s", "gogo"}, 22}, // 11+2+2+4 + 3 separators
	}
	for _, c := range cases {
		if got := TmuxCommandBytes(c.argv); got != c.want {
			t.Errorf("TmuxCommandBytes(%v) = %d, want %d", c.argv, got, c.want)
		}
	}
	// A real spawn argv is measured whole - the inlined brief dominates it.
	in := PlanIntent("Some plan", strings.Repeat("x", 20000), "plan-abc123")
	if n := TmuxCommandBytes(TmuxNewSessionArgs("/repos/x", in)); n <= MaxTmuxCommandBytes {
		t.Errorf("a 20 KB brief measured %d bytes - expected it to blow the %d-byte budget", n, MaxTmuxCommandBytes)
	}
}

// TestFoldToPointerDropsBodyKeepsPointer pins FR1.3/D1=A: an over-budget spawn
// loses the inlined body, gains the absolute plan path + the per-source section,
// KEEPS its --correlation param, and lands under the limit.
func TestFoldToPointerDropsBodyKeepsPointer(t *testing.T) {
	t.Setenv(PermissionModeEnv, "auto")
	body := strings.Repeat("a very long analyst brief. ", 800) // ~21 KB
	in := PlanIntent("Catalogue side of the matching engine", body, "plan-30939c06")
	in.Root = "/repos/dotai"
	if intentFits(in) {
		t.Fatalf("fixture is not over budget (%d bytes)", TmuxCommandBytes(TmuxNewSessionArgs(in.Root, in)))
	}

	planPath := "/Users/x/.gogo/projects/dotai/.gogo/plans/plan-30939c06.md"
	folded := FoldToPointer(in, planPath, "gogo")

	if strings.Contains(folded.Command, body) {
		t.Error("folded command still inlines the whole body")
	}
	for _, want := range []string{"read your brief at " + planPath, "### gogo", "--correlation plan-30939c06"} {
		if !strings.Contains(folded.Command, want) {
			t.Errorf("folded command = %q, missing %q", folded.Command, want)
		}
	}
	if !strings.HasPrefix(folded.Command, "/gogo:plan ") {
		t.Errorf("folded command lost its slash command: %q", folded.Command)
	}
	if folded.Body != "" {
		t.Error("folded intent still records a Body - nothing is left to fold")
	}
	if !intentFits(folded) {
		t.Errorf("folded command is STILL over budget: %d bytes", TmuxCommandBytes(TmuxNewSessionArgs(folded.Root, folded)))
	}
	// Everything except the command is untouched.
	if folded.Session != in.Session || folded.Action != in.Action || folded.Root != in.Root {
		t.Errorf("fold disturbed the intent envelope: %+v", folded)
	}
}

// TestFoldToPointerFoldsAnAuthoringGoal: the `A` authoring prompt folds too - the
// pasted goal is replaced by a whole-file pointer (no `## Source briefs` section:
// the goal IS the plan body) while the skill instruction survives intact.
func TestFoldToPointerFoldsAnAuthoringGoal(t *testing.T) {
	t.Setenv(PermissionModeEnv, "auto")
	goal := strings.Repeat("paste of a multi-KB spec. ", 800)
	planPath := "/Users/x/.gogo/projects/dotai/.gogo/plans/plan-1948afcd.md"
	in := AuthorPlanIntent("Big spec", goal, planPath, "plan-1948afcd", "", []SourceRef{{Label: "gogo", Path: "/repos/gogo"}})
	in.Root = "/repos/gogo"
	if intentFits(in) {
		t.Fatalf("fixture is not over budget")
	}

	folded := FoldToPointer(in, planPath, "")
	if strings.Contains(folded.Command, goal) {
		t.Error("folded author command still inlines the whole goal")
	}
	for _, want := range []string{"Load and follow the gogo-project-plan skill", "read your brief at " + planPath, "gogo -> /repos/gogo"} {
		if !strings.Contains(folded.Command, want) {
			t.Errorf("folded author command missing %q", want)
		}
	}
	// A whole-file pointer names no per-source subsection (the prompt's own output
	// contract still mentions `## Source briefs` - that is the instruction, not the
	// pointer, so assert on the pointer sentence itself).
	if strings.Contains(folded.Command, "read your brief at "+planPath+", section") {
		t.Error("a whole-file pointer must not name a per-source section")
	}
	if !intentFits(folded) {
		t.Error("folded author command is still over budget")
	}
}

// TestFoldToPointerNoopUnderBudget pins the non-negotiable happy path: an
// in-budget intent comes back BYTE-FOR-BYTE, so nothing about today's launches
// moves. Same for an intent with nothing to fold or nowhere to point.
func TestFoldToPointerNoopUnderBudget(t *testing.T) {
	t.Setenv(PermissionModeEnv, "auto")
	in := PlanIntent("Token migration", "move the shared token store", "plan-7f3a1b2c")
	in.Root = "/repos/web"

	got := FoldToPointer(in, "/Users/x/.gogo/projects/web/.gogo/plans/plan-7f3a1b2c.md", "web")
	if !reflect.DeepEqual(got, in) {
		t.Errorf("under budget the intent changed:\n got %+v\nwant %+v", got, in)
	}

	// No plan path to point at, and no recorded body → unchanged even when oversized.
	huge := PlanIntent("x", strings.Repeat("y", 20000), "")
	if got := FoldToPointer(huge, "", "web"); !reflect.DeepEqual(got, huge) {
		t.Error("an empty planPath must leave the intent untouched")
	}
	bodyless := Intent{Action: ActionGo, Command: "/gogo:go " + strings.Repeat("z", 20000)}
	if got := FoldToPointer(bodyless, "/p/plan.md", "web"); !reflect.DeepEqual(got, bodyless) {
		t.Error("an intent with no recorded Body must be left untouched")
	}
}

// TestUnderBudgetArgvIsUnaffectedByFoldPlumbing guards the non-negotiable D1=A
// property against the plumbing added for it: neither Intent.Body nor Intent.Root
// may reach an argv. Root in particular is now set at the CLI spawn doors (so the
// budget is measured against the real anchor path) - if it ever leaked into
// TmuxNewSessionArgs, every under-budget launch would change shape.
func TestUnderBudgetArgvIsUnaffectedByFoldPlumbing(t *testing.T) {
	t.Setenv(PermissionModeEnv, "auto")
	bare := PlanIntent("Token migration", "move the shared token store", "plan-7f3a1b2c")

	// The same intent carrying the fold metadata must produce IDENTICAL argv.
	withMeta := bare
	withMeta.Root = "/repos/web"
	if got, want := TmuxNewSessionArgs("/repos/web", withMeta), TmuxNewSessionArgs("/repos/web", bare); !reflect.DeepEqual(got, want) {
		t.Errorf("Intent.Root leaked into the argv:\n got %v\nwant %v", got, want)
	}
	// And a FoldToPointer round-trip under budget leaves the argv byte-identical.
	folded := FoldToPointer(withMeta, "/Users/x/.gogo/projects/web/.gogo/plans/plan-7f3a1b2c.md", "web")
	if got, want := TmuxNewSessionArgs("/repos/web", folded), TmuxNewSessionArgs("/repos/web", bare); !reflect.DeepEqual(got, want) {
		t.Errorf("an under-budget fold changed the argv:\n got %v\nwant %v", got, want)
	}
	// The exact pre-0.28 command shape, spelled out so a drift is unmissable.
	if bare.Command != "/gogo:plan move the shared token store --correlation plan-7f3a1b2c" {
		t.Errorf("under-budget command drifted: %q", bare.Command)
	}
}

// TestExactTargetOnProbesNotOnCreate pins FR1.5 - the split that makes this safe:
// tmux `-t` TARGETS get the `=` exact-match prefix (has-session / kill-session /
// capture-pane), while `new-session -s` - which takes a NAME, not a target - must
// NOT, or the `=` becomes part of the session name (verified: a session literally
// called `=gogo-eqtest` was created).
func TestExactTargetOnProbesNotOnCreate(t *testing.T) {
	t.Setenv(PermissionModeEnv, "auto")
	const name = "gogo-plan-exact"

	if got, want := HasSessionArgs(name), []string{"has-session", "-t", "=gogo-plan-exact"}; !reflect.DeepEqual(got, want) {
		t.Errorf("HasSessionArgs = %v, want %v", got, want)
	}
	if got, want := KillSessionArgs(name), []string{"kill-session", "-t", "=gogo-plan-exact"}; !reflect.DeepEqual(got, want) {
		t.Errorf("KillSessionArgs = %v, want %v", got, want)
	}
	// capture-pane's -t is a PANE target: it needs the trailing `:` (a bare
	// `=<name>` is a SESSION target and is rejected - `can't find pane`).
	if got := CapturePaneArgs(name, 300); got[1] != "-t" || got[2] != "=gogo-plan-exact:" {
		t.Errorf("CapturePaneArgs target = %v, want the exact PANE form =<name>:", got[:3])
	}

	// new-session: the -s VALUE must be the bare name on both create paths.
	in := BuildIntent(ActionGo, []string{"slug-x"}, "")
	for label, argv := range map[string][]string{
		"TmuxNewSessionArgs": TmuxNewSessionArgs("/repos/x", in),
		"TmuxPersistentArgs": TmuxPersistentArgs("/repos/x", in, PhaseOpts{}),
	} {
		for i, a := range argv {
			if a == "-s" {
				if got := argv[i+1]; got != "gogo-go-slug-x" {
					t.Errorf("%s: -s = %q, want the BARE session name (an `=` would become part of it)", label, got)
				}
			}
			if strings.HasPrefix(a, "=") {
				t.Errorf("%s: argv element %q carries an exact-match prefix - new-session takes no targets", label, a)
			}
		}
	}
}

// TestSanitizeLabelBounded pins FR1.4: the session label is capped, cut on a `-`
// boundary, never trailing-dashed, idempotent - and SessionMatchesSlug still
// attributes a capped session to its (uncapped) slug, because both sides run the
// same transform.
func TestSanitizeLabelBounded(t *testing.T) {
	long := "Catalogue side of the matching engine - normalise, store, embed, hard-filter"
	got := sanitizeLabel(long)
	if len(got) > MaxSessionLabel {
		t.Errorf("sanitizeLabel(%q) = %q (%d bytes), want <= %d", long, got, len(got), MaxSessionLabel)
	}
	if strings.HasSuffix(got, "-") || strings.HasPrefix(got, "-") {
		t.Errorf("capped label %q has a dangling dash", got)
	}
	if got != sanitizeLabel(got) {
		t.Errorf("sanitizeLabel is not idempotent: %q -> %q", got, sanitizeLabel(got))
	}
	// Cut on a `-` boundary: the last segment must be a whole word of the original.
	if last := got[strings.LastIndex(got, "-")+1:]; !strings.Contains(sanitizeLabelUncapped(long), "-"+last+"-") {
		t.Errorf("capped label %q ends mid-word (%q is not a whole segment)", got, last)
	}

	// A single long word with no `-` still gets capped (hard cut, no boundary to use).
	oneWord := strings.Repeat("z", 200)
	if g := sanitizeLabel(oneWord); len(g) != MaxSessionLabel {
		t.Errorf("one-word label = %d bytes, want a hard cut at %d", len(g), MaxSessionLabel)
	}

	// REV-001: the `-` boundary must never collapse the label to a stub. These are
	// realistic titles (a verb plus one long CamelCase identifier) whose ONLY dash
	// sits near the front; honouring it unconditionally yields "refactor" / "a" / "x",
	// and two plans sharing a first word then mint the SAME session base.
	floor := MaxSessionLabel / 2
	for _, degenerate := range []string{
		"Refactor NotificationDeliveryOrchestrationPipelineForRealtimeEvents",
		"A supercalifragilisticexpialidociousreallylongidentifiername",
		"x abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz",
	} {
		g := sanitizeLabel(degenerate)
		if len(g) < floor {
			t.Errorf("sanitizeLabel(%q) = %q (%d bytes) - collapsed below the %d-byte floor; the dash boundary must not win here",
				degenerate, g, len(g), floor)
		}
		if len(g) > MaxSessionLabel {
			t.Errorf("sanitizeLabel(%q) = %d bytes, over the %d cap", degenerate, len(g), MaxSessionLabel)
		}
	}

	// The property that floor protects: titles that differ only AFTER their shared
	// first word must stay distinguishable, or SessionMatchesSlug cannot tell two
	// plans apart (TEST-005's ambiguity, via a lossy transform).
	a := sanitizeLabel("Refactor NotificationDeliveryOrchestrationPipeline")
	b := sanitizeLabel("Refactor PaymentSettlementReconciliationPipeline")
	if a == b {
		t.Errorf("two titles sharing only their first word collapsed to the same label %q - session names and SlugHints now collide", a)
	}
	if SessionMatchesSlug(sessionName("plan", "Refactor NotificationDeliveryOrchestrationPipeline"), "Refactor PaymentSettlementReconciliationPipeline") {
		t.Error("a session minted for one long title attributes to a DIFFERENT title sharing its first word")
	}

	// Short labels are untouched (byte-for-byte with pre-cap behaviour).
	if g := sanitizeLabel("My Plan Title"); g != "my-plan-title" {
		t.Errorf("short label changed: %q", g)
	}
	if g := sanitizeLabel(""); g != "run" {
		t.Errorf("empty label = %q, want the run fallback", g)
	}

	// The capped session still attributes to its slug (both sides sanitize).
	session := sessionName("plan", long)
	if !SessionMatchesSlug(session, long) {
		t.Errorf("SessionMatchesSlug(%q, <the same long label>) = false - a capped name lost its attribution", session)
	}
	// SlugFromLabel is the exported form of exactly this transform (FR1.7).
	if SlugFromLabel(long) != got {
		t.Errorf("SlugFromLabel drifted from sanitizeLabel: %q vs %q", SlugFromLabel(long), got)
	}
}

// sanitizeLabelUncapped is the pre-cap reduction, for asserting that the cap lands
// on a whole `-` segment of the full label.
func sanitizeLabelUncapped(label string) string {
	return "-" + strings.Trim(sessionUnsafe.ReplaceAllString(strings.ToLower(label), "-"), "-") + "-"
}

// TestSessionMatchesSlugCoversAuthorAndResume pins FR1.7: the action list covers
// EVERY action sessionName can mint, and the TEST-005 non-matches still fail.
func TestSessionMatchesSlugCoversAuthorAndResume(t *testing.T) {
	cases := []struct {
		session, slug string
		want          bool
	}{
		{"gogo-author-x", "x", true},
		{"gogo-resume-x", "x", true},
		{"gogo-author-my-feature-2", "my-feature", true},
		// TEST-005 must keep holding for the new actions too - no substring matching.
		{"gogo-author-awaiting-card", "waiting-card", false},
		{"gogo-resume-oauth", "auth", false},
		{"gogo-author-a-bee", "a", false},
		// A non-gogo action prefix is not a match.
		{"gogo-ship-x", "x", false},
	}
	for _, c := range cases {
		if got := SessionMatchesSlug(c.session, c.slug); got != c.want {
			t.Errorf("SessionMatchesSlug(%q, %q) = %v, want %v", c.session, c.slug, got, c.want)
		}
	}
}

// TestSessionMatchesSlugSurvivesTheUpgrade pins REV-009: FR1.4's cap moved the
// SLUG side of this comparison too, so a session minted by an OLDER gogo with a
// >48-char label would stop matching the moment the user upgraded - losing its ●
// dot, its `a` attach, its `l` peek, and (the dangerous one) its place in
// ActiveWorkSlugs, so the per-source cap would let a second build clobber the same
// working tree.
//
// The fixtures are the two sessions that were ACTUALLY running on the dev host
// during this work, read from `tmux list-sessions`.
func TestSessionMatchesSlugSurvivesTheUpgrade(t *testing.T) {
	// The 83-char session a pre-0.28.0 gogo minted, and the title it came from.
	const liveLong = "gogo-plan-catalogue-side-of-the-matching-engine---normalise-store-embed-hard-filter"
	const longTitle = "Catalogue side of the matching engine - normalise, store, embed, hard-filter"
	// The 46-char one, already under the cap and unaffected either way.
	const liveShort = "gogo-author-for-gogo-project-lets-add-few-new-tasks-to-plan"
	const shortTitle = "for gogo project lets add few new tasks to plan"

	if !SessionMatchesSlug(liveLong, longTitle) {
		t.Errorf("the live 83-char pre-0.28.0 session lost its attribution after the upgrade:\n  session %q\n  title   %q\n  0.28.0 would mint %q",
			liveLong, longTitle, sessionName("plan", longTitle))
	}
	if !SessionMatchesSlug(liveShort, shortTitle) {
		t.Errorf("the live 46-char session (under the cap) stopped matching: %q vs %q", liveShort, shortTitle)
	}
	// A session minted by 0.28.0 (bounded) must of course still match.
	if got := sessionName("plan", longTitle); !SessionMatchesSlug(got, longTitle) {
		t.Errorf("a freshly-minted bounded session %q does not match its own title", got)
	}
	// The collision suffix works on the legacy base too.
	if !SessionMatchesSlug(liveLong+"-2", longTitle) {
		t.Error("the legacy base does not accept uniqueSession's numeric collision suffix")
	}

	// The widening must NOT reintroduce REV-001's ambiguity: matching stays a WHOLE
	// -base comparison, so a bounded name still must not match a different slug that
	// merely shares a prefix, and the legacy base must not match a foreign slug.
	for _, c := range []struct {
		session, slug string
	}{
		{liveLong, "Catalogue side of the matching engine - normalise, store, embed"}, // a DIFFERENT, shorter title
		{liveLong, "Catalogue side"},
		{sessionName("plan", longTitle), "Catalogue side"},
		{sessionName("plan", "Refactor NotificationDeliveryOrchestrationPipeline"), "Refactor PaymentSettlementReconciliationPipeline"},
		// TEST-005's originals must be untouched by the widening.
		{"gogo-done-awaiting-card", "waiting-card"},
		{"gogo-go-oauth", "auth"},
		{"gogo-go-a-b", "a"},
	} {
		if SessionMatchesSlug(c.session, c.slug) {
			t.Errorf("the legacy-base widening cross-attributed %q to slug %q", c.session, c.slug)
		}
	}

	// Minting is UNAFFECTED - the widening is read-side only (FR1.4 intact).
	if got := sessionName("plan", longTitle); len(got) > len("gogo-plan-")+MaxSessionLabel {
		t.Errorf("minting produced an unbounded name %q - the widening must not relax FR1.4", got)
	}
}

// TestPreflightRefusesOversizedCommand pins the FR1.2 backstop (D1's option B):
// when even a folded command does not fit, the launch is refused BEFORE tmux sees
// it, with a typed error naming the byte count and the limit.
func TestPreflightRefusesOversizedCommand(t *testing.T) {
	argv := []string{"new-session", strings.Repeat("q", MaxTmuxCommandBytes)}
	err := preflight("new-session", argv)
	if err == nil {
		t.Fatal("preflight accepted an over-budget command line")
	}
	var tooLong *CommandTooLongError
	if !errors.As(err, &tooLong) {
		t.Fatalf("preflight returned %T, want a *CommandTooLongError", err)
	}
	if tooLong.Limit != MaxTmuxCommandBytes || tooLong.Bytes <= tooLong.Limit {
		t.Errorf("CommandTooLongError = %+v, want Bytes > Limit == %d", tooLong, MaxTmuxCommandBytes)
	}
	for _, want := range []string{"new-session", "16317"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
	// Exactly at the limit is accepted (the bisected boundary: 16317 OK, 16318 not).
	atLimit := []string{strings.Repeat("q", MaxTmuxCommandBytes)}
	if err := preflight("new-session", atLimit); err != nil {
		t.Errorf("preflight refused a command line exactly at the limit: %v", err)
	}
}
