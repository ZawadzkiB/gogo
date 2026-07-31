package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/orchestrator"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	tea "github.com/charmbracelet/bubbletea"
)

// authoringFeature is a card whose plan.md is not written while its status still reads the
// plan-acceptance gate - the folder the board used to offer for acceptance. Dir is real, so
// the refusals resolve their exact reason from the disk the production code reads.
func authoringFeature(t *testing.T, slug string, planSections int) *contract.Feature {
	t.Helper()
	dir := t.TempDir()
	if planSections > 0 {
		var b strings.Builder
		b.WriteString("# Plan\n\n")
		for i := 0; i < planSections; i++ {
			b.WriteString("## S")
			b.WriteByte(byte('a' + i))
			b.WriteString("\n\nx\n\n")
		}
		if err := os.WriteFile(filepath.Join(dir, "plan.md"), []byte(b.String()), 0o644); err != nil {
			t.Fatalf("write plan.md: %v", err)
		}
	}
	return &contract.Feature{
		Slug: slug, Dir: dir, Root: fixtureRoot,
		Status: "awaiting-plan-acceptance", Phase: "plan",
		Class:         contract.ClassUnfinished,
		PlanUnwritten: planSections < contract.PlanSectionsRequired,
	}
}

// focusOn puts a single feature in the plan column and focuses it.
func focusOn(m Model, f *contract.Feature) Model {
	m.cols[0] = []*contract.Feature{f}
	m.colIdx, m.cardIdx[0] = 0, 0
	return m
}

// cappedRoot is the source path the cap fixtures register (with ConcurrentWorkItems 1).
const cappedRoot = "/r/capped"

// cappedModel builds a unified-board Model over feats whose single registered source is
// capped at ONE concurrent work item, with the given live session list - the fixture the
// cap-bounce assertions need. It never touches a real repo or a real tmux.
func cappedModel(t *testing.T, sessions []string, feats ...*contract.Feature) Model {
	t.Helper()
	proj := projects.Project{Name: "app", Sources: []projects.Source{
		{Name: "capped", Path: cappedRoot, ConcurrentWorkItems: 1},
	}}
	m := NewWorkspaceAll(&contract.Repo{Features: feats}, []projects.Project{proj})
	m.hasClaude = true
	m.sessions = sessions
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	return nm.(Model)
}

// --- FR3: the pill, the stripe, the badge -------------------------------------------

// TestAuthoringPillIsDimAndDistinct pins FR3: an authoring card reads `✎ authoring` in the
// DIM style, not the red `⏸ accept plan` - glyph AND word, so the distinction survives a
// colourless terminal (FR14b). It must be impossible to mistake for a plan the user can act
// on, which is the whole failure being fixed.
func TestAuthoringPillIsDimAndDistinct(t *testing.T) {
	auth := authoringFeature(t, "demo", 0)
	if got := badge(auth); got != "authoring" {
		t.Errorf("badge = %q, want %q", got, "authoring")
	}
	if got := pillLabel(auth); got != authoringMarker+" authoring" {
		t.Errorf("pillLabel = %q, want %q", got, authoringMarker+" authoring")
	}
	if got := pillLabel(auth); strings.Contains(got, waitingMarker) {
		t.Errorf("pillLabel = %q - an authoring card must NOT carry the ⏸ gate glyph", got)
	}
	// Compare the style's PROPERTIES, not its rendered output: `go test` has no TTY, so
	// lipgloss strips colour and `pillRed.Render("x")` and `pillDim.Render("x")` are the
	// SAME string. A rendered-output assertion here looks like a check and is a no-op -
	// it let a revert of this very arm survive a mutation sweep.
	got, dim, red := pillStyleFor(auth), pillDim, pillRed
	if got.GetForeground() != dim.GetForeground() || got.GetBackground() != dim.GetBackground() {
		t.Errorf("pillStyleFor(authoring) fg/bg = %v/%v, want pillDim's %v/%v - an authoring card is not a gate",
			got.GetForeground(), got.GetBackground(), dim.GetForeground(), dim.GetBackground())
	}
	if got.GetForeground() == red.GetForeground() {
		t.Error("pillStyleFor(authoring) uses the RED gate foreground - it must not look like a card awaiting the user")
	}
	if got.GetBold() {
		t.Error("pillStyleFor(authoring) is bold - the gate pills are bold, an authoring pill is quiet")
	}
	// No gate stripe: the heavy `┃` accent is the board's sole "act now" cue.
	if col, ok := stripeAccent(auth); ok {
		t.Errorf("stripeAccent = (%v, true) - an authoring card must carry NO gate stripe", col)
	}

	// The written-plan gate is untouched: red pill, ⏸ glyph, stripe.
	gate := authoringFeature(t, "ready", 8)
	if got := pillLabel(gate); got != waitingMarker+" accept plan" {
		t.Errorf("a WRITTEN plan's pill = %q, want the accept-plan chip (the gate itself regressed)", got)
	}
	if _, ok := stripeAccent(gate); !ok {
		t.Error("a WRITTEN plan awaiting acceptance lost its gate stripe")
	}
}

// TestAuthoringStubNamesItsShortfall: a STUB plan (1 of 2 sections) reads authoring too, and
// every refusal about it names the number rather than saying "too small" (FR14b).
func TestAuthoringStubReadsAuthoring(t *testing.T) {
	stub := authoringFeature(t, "demo", 1)
	if !stub.Authoring() {
		t.Fatal("a 1-section plan.md is not authoring - the stub half of the check does not bite")
	}
	if got := pillLabel(stub); got != authoringMarker+" authoring" {
		t.Errorf("pillLabel = %q, want the authoring chip", got)
	}
	if got := planReadinessBounce(stub); !strings.Contains(got, "1 of the 2 sections") {
		t.Errorf("the stub refusal = %q, want it to NAME ITS NUMBER (\"1 of the 2 sections\")", got)
	}
}

// TestTemplateScaffoldRendersNoCorrelationChip is TEST-001 at the surface the user actually
// saw. A work item scaffolded straight from the SHIPPED template rendered `⛓ ×3`, claiming
// membership in three cross-source plans, because the template's optional-correlation legend
// wraps an example `- **correlation:**` line in a multi-line comment that the parser read as a
// real field. This test loads the real template (not a copy, so it cannot drift from what
// ships) through the real reader and asserts the CARD, since the chip is what made the defect
// visible - and it is what the original bug report's stray `x3` turned out to be.
func TestTemplateScaffoldRendersNoCorrelationChip(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "templates", "state.template.md"))
	if err != nil {
		t.Fatalf("read the shipped template: %v", err)
	}
	if !strings.Contains(string(raw), "- **correlation:**") {
		t.Fatal("the shipped template no longer carries a commented-out `- **correlation:**` example, so " +
			"this guard no longer exercises TEST-001")
	}
	root := t.TempDir()
	dir := filepath.Join(root, ".gogo", "work", "feature-zzscaffold")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "state.md"), raw, 0o644); err != nil {
		t.Fatalf("write state.md: %v", err)
	}
	repo, err := contract.LoadRepo(root)
	if err != nil {
		t.Fatalf("LoadRepo: %v", err)
	}
	f := repo.Feature("zzscaffold")
	if f == nil {
		t.Fatal("scaffolded feature missing")
	}

	card := (&Model{}).renderCard(0, f, false, 60)
	if strings.Contains(card, "⛓") {
		t.Errorf("a template scaffold renders a correlation chip:\n%s", card)
	}
	// It reads as authoring (no plan.md), and its name falls back to the slug rather than the
	// `<one-line title>` placeholder - the same family of legend leak, both closed.
	if !strings.Contains(card, authoringMarker+" authoring") {
		t.Errorf("the scaffold card does not read authoring:\n%s", card)
	}
	if strings.Contains(card, "<one-line title>") {
		t.Errorf("the scaffold card leaks the title placeholder:\n%s", card)
	}
}

// TestAuthoringNotCountedInNeedsYou pins FR2 at the header pill: `⏸ K need you` counts
// WaitingForInput across all four columns, and an authoring item used to inflate it - the
// observed `⏸ 3 need you`. A real gate still counts.
func TestAuthoringNotCountedInNeedsYou(t *testing.T) {
	m := newModel(t)
	auth := authoringFeature(t, "demo", 0)
	gate := authoringFeature(t, "ready", 8)
	m.cols[0] = []*contract.Feature{auth}
	m.cols[1], m.cols[2], m.cols[3] = nil, nil, nil
	if got := m.needsYouCount(); got != 0 {
		t.Errorf("needsYouCount with one authoring card = %d, want 0", got)
	}
	m.cols[0] = []*contract.Feature{auth, gate}
	if got := m.needsYouCount(); got != 1 {
		t.Errorf("needsYouCount with authoring + a real gate = %d, want 1 (only the real gate)", got)
	}
	// And it is visible in the rendered header, not just on the model.
	m.cardIdx[0] = 0
	if out := m.View(); !strings.Contains(out, "⏸ 1 need you") {
		t.Errorf("View() header does not read \"⏸ 1 need you\"; got:\n%s", firstLines(out, 4))
	}
}

// --- FR4 / FR4a / FR8: the refusals --------------------------------------------------

// TestAuthoringMoveBounces pins FR4 through the REAL guard: `m` on an authoring card must
// produce NO launch intent, and the bounce must name plan.md and the unblock.
func TestAuthoringMoveBounces(t *testing.T) {
	m := focusOn(newModel(t), authoringFeature(t, "demo", 0))
	in, ship, bounce := m.attemptAction(false)
	if bounce == "" {
		t.Fatalf("m on an authoring card produced intent %+v (ship=%v) instead of a bounce", in, ship)
	}
	if in.Action != "" || in.Command != "" {
		t.Errorf("a bounced move still built an intent: %+v", in)
	}
	for _, want := range []string{"plan.md not written yet", "demo is still being authored", "no plan.md on disk yet", "gogo plan demo"} {
		if !strings.Contains(bounce, want) {
			t.Errorf("bounce %q missing %q", bounce, want)
		}
	}
}

// TestAuthoringMoveBounceIsBlockedSeverityAndRendered pins FR14a/FR14b AND review check #8:
// a status a handler sets must be RENDERED by the mode's View(). It drives the real `m`
// keystroke through Update, then asserts the amber `⚠ ` glyph and the unblock in View()
// output under a TTY-less test - where colour alone is flattened to nothing.
func TestAuthoringMoveBounceIsBlockedSeverityAndRendered(t *testing.T) {
	m := focusOn(newModel(t), authoringFeature(t, "demo", 0))
	got := send(m, runes("m"))

	if got.statusLevel != statusLevelWarn {
		t.Errorf("statusLevel = %v, want warn - a refusal is BLOCKED, not failed and not done", got.statusLevel)
	}
	if got.mode == modeForm {
		t.Fatal("m on an authoring card opened a confirm form - with the confirm-default convention " +
			"(a forward move seeds confirm:true) a bare Enter would then launch /gogo:accept at a plan " +
			"that does not exist")
	}
	out := got.View()
	if !strings.Contains(out, statusWarnMarker) {
		t.Errorf("View() carries no %q glyph, so the severity is invisible without colour; status line region:\n%s",
			statusWarnMarker, lastLines(out, 4))
	}
	for _, want := range []string{"plan.md not written yet", "gogo plan demo"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() output missing %q (a status set on the model but never rendered is a silent no-op)", want)
		}
	}
}

// TestForceCannotOverrideMissingPlan is FR4a, the sharpest edge in this change. `M` skips the
// per-source cap "and only that guard - every other legality rule still applies". A missing
// plan is a LEGALITY rule, so the refusal is evaluated OUTSIDE the !force conditions: if it
// were written `!ready && !force`, `M` would become a way to force-accept a plan that does not
// exist, weakening the exact invariant this release strengthens.
func TestForceCannotOverrideMissingPlan(t *testing.T) {
	cases := []struct {
		name string
		f    *contract.Feature
	}{
		{"authoring (FR4)", authoringFeature(t, "demo", 0)},
		{"stub plan (FR4)", authoringFeature(t, "demo", 1)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := focusOn(newModel(t), c.f)
			in, _, bounce := m.attemptActionForce(false, true) // force=true
			if bounce == "" {
				t.Fatalf("M forced past the missing plan and produced %+v", in)
			}
			if !strings.Contains(bounce, "plan.md") {
				t.Errorf("forced bounce %q does not name plan.md", bounce)
			}
			// And through the real keystroke: no form, nothing launched.
			got := send(m, runes("M"))
			if got.mode == modeForm {
				t.Error("M opened a confirm form for a card with no written plan")
			}
			if got.statusLevel != statusLevelWarn {
				t.Errorf("M refusal level = %v, want warn", got.statusLevel)
			}
		})
	}
}

// TestDecisionGateIsNotLaunchable pins TEST-002: a card paused at a decision gate must not be
// launchable from the board, by `m` OR by `M`.
//
// Before this, `waiting-for-user` was consulted for DISPLAY only (the badge, the ⏸ count, the
// stripe) and never by the move guard, so a paused card whose stale phase still classified
// in-progress opened a real `/gogo:go` confirmation - leaving a STOP instruction inside the
// spawned session's own prompt as the only thing between a keypress and a relaunch. That is
// prose enforcement, which this release's own evidence (FR11, n=3) shows does not hold.
func TestDecisionGateIsNotLaunchable(t *testing.T) {
	paused := func() *contract.Feature {
		return &contract.Feature{Slug: "demo", Root: fixtureRoot, Phase: "review",
			Status: "waiting-for-user", OpenDecision: "D3", Class: contract.ClassInProgress}
	}
	// Sanity: the fixture must really be the in-progress shape, or this would pass because the
	// card fell into some other arm rather than because the gate check fired.
	if f := paused(); !f.WaitingForUser() || f.Class != contract.ClassInProgress {
		t.Fatalf("fixture is not a paused in-progress card: waiting=%v class=%q", f.WaitingForUser(), f.Class)
	}

	for _, force := range []bool{false, true} {
		name := "m"
		if force {
			name = "M (force)"
		}
		t.Run(name, func(t *testing.T) {
			f := paused()
			m := newModel(t)
			m.cols[0], m.cols[1] = nil, []*contract.Feature{f}
			m.colIdx, m.cardIdx[1] = 1, 0
			in, ship, bounce := m.attemptActionForce(false, force)
			if bounce == "" {
				t.Fatalf("%s on a paused card produced intent %+v (ship=%v) instead of a bounce - a decision "+
					"gate is a legality rule, so force must not override it (FR4a's shape)", name, in, ship)
			}
			if in.Action != "" || in.Command != "" {
				t.Errorf("a bounced move still built an intent: %+v", in)
			}
			// Diagnosable: names the gate BY ID and the legal move.
			for _, want := range []string{"demo is paused on D3", "decisions.md", "/gogo:resume demo"} {
				if !strings.Contains(bounce, want) {
					t.Errorf("bounce %q missing %q", bounce, want)
				}
			}
		})
	}

	// Rendered + severity, through the real keystrokes.
	for _, key := range []string{"m", "M"} {
		f := paused()
		m := newModel(t)
		m.cols[0], m.cols[1] = nil, []*contract.Feature{f}
		m.colIdx, m.cardIdx[1] = 1, 0
		got := send(m, runes(key))
		if got.mode == modeForm {
			t.Errorf("%s opened a launch confirm for a paused card - with the confirm-default convention a "+
				"bare Enter would then relaunch it", key)
		}
		if got.statusLevel != statusLevelWarn {
			t.Errorf("%s refusal level = %v, want warn (blocked, not failed)", key, got.statusLevel)
		}
		out := got.View()
		if !strings.Contains(out, statusWarnMarker) || !strings.Contains(out, "/gogo:resume demo") {
			t.Errorf("%s: View() does not render the warn glyph and the unblock; status line:\n%s",
				key, lastLines(out, 3))
		}
	}

	// A gate you cannot LEAVE would be worse than one you can bypass. Once /gogo:resume records
	// the answer and the status moves on, the same card is launchable again - the gate is closed,
	// not sealed. (This is also the half that agrees with REV-022's scoped exit write: §④ never
	// overwrites a gate status, so the card stays paused until something legitimately resumes it.)
	resumed := paused()
	resumed.Status, resumed.OpenDecision = "implementing", "none"
	m := newModel(t)
	m.cols[0], m.cols[1] = nil, []*contract.Feature{resumed}
	m.colIdx, m.cardIdx[1] = 1, 0
	in, _, bounce := m.attemptActionForce(false, false)
	if bounce != "" || in.Action != launch.ActionGo {
		t.Errorf("after resuming, the card is still refused: intent=%+v bounce=%q - a decision gate must be "+
			"escapable, or the board becomes a trap", in, bounce)
	}

	// And a card with no open-decision id still gets a usable refusal.
	noID := paused()
	noID.OpenDecision = "none"
	if b := decisionGateBounce(noID); !strings.Contains(b, "paused on a decision") || !strings.Contains(b, "/gogo:resume demo") {
		t.Errorf("bounce without a decision id = %q, want the generic wording plus the unblock", b)
	}
}

// TestDecisionGateKeepsItsDisplaySignals pins the interaction with REV-022's scoped exit write:
// a paused card keeps the signals that tell the user it needs them, at the same time as being
// un-launchable. If the two disagreed, a card could be refused by the board while showing no
// reason for it.
func TestDecisionGateKeepsItsDisplaySignals(t *testing.T) {
	f := &contract.Feature{Slug: "demo", Root: fixtureRoot, Phase: "review",
		Status: "waiting-for-user", OpenDecision: "D3", Class: contract.ClassInProgress}
	if !f.WaitingForInput() {
		t.Error("a paused card dropped out of WaitingForInput, so it would not be counted in ⏸ K need you")
	}
	if _, ok := stripeAccent(f); !ok {
		t.Error("a paused card lost its gate stripe")
	}
	m := newModel(t)
	// Isolate the board: newModel loads the fixture repo, which already contains a card at the
	// UAT gate, so counting against it would measure the fixture rather than this card.
	m.cols[0], m.cols[1], m.cols[2], m.cols[3] = nil, []*contract.Feature{f}, nil, nil
	m.colIdx, m.cardIdx[1] = 1, 0
	if n := m.needsYouCount(); n != 1 {
		t.Errorf("needsYouCount = %d, want 1 - a paused card is exactly what that pill is for", n)
	}
	if card := m.renderCard(1, f, false, 60); !strings.Contains(card, waitingMarker) {
		t.Errorf("the paused card renders no ⏸ gate cue:\n%s", card)
	}
}

// TestForceStillOverridesTheCap is the other half of FR4a: narrowing `M` must not BREAK it.
// A pure cap bounce is still forced past, so the same key that cannot conjure a plan can
// still start a second build in a busy source.
func TestForceStillOverridesTheCap(t *testing.T) {
	// A written, accepted plan in a source capped at 1 that is already building something.
	target := authoringFeature(t, "next", 8)
	target.Status, target.Root = "plan-accepted", cappedRoot
	busy := &contract.Feature{Slug: "busy", Root: cappedRoot, Class: contract.ClassInProgress, Status: "implementing"}
	m := cappedModel(t, []string{"gogo-go-busy"}, target, busy)
	m = focusOn(m, target)

	if _, _, bounce := m.attemptAction(false); bounce == "" {
		t.Fatal("the fixture is not actually over its cap, so this test would pass for the wrong reason")
	} else if !strings.Contains(bounce, "cap 1 reached") || !strings.Contains(bounce, "busy") {
		t.Fatalf("the unforced bounce is not the CAP bounce: %q", bounce)
	}
	in, _, bounce := m.attemptActionForce(false, true)
	if bounce != "" {
		t.Fatalf("M did not force past the cap: %q", bounce)
	}
	if in.Action != launch.ActionGo {
		t.Errorf("forced intent action = %q, want go", in.Action)
	}
}

// TestCapBounceStatesTheNewRule pins FR12a: 0.28.0 wrote the cap's rule into the
// user-visible bounce, FR12 changed the rule, so the sentence had to change with it. The
// cockpit's most legible message must not become its most wrong.
func TestCapBounceStatesTheNewRule(t *testing.T) {
	target := authoringFeature(t, "next", 8)
	target.Status, target.Root = "plan-accepted", cappedRoot
	// DELIBERATELY ClassUnfinished + plan-accepted: the pre-first-write window. Before FR12
	// the class filter made this live build invisible, so a passing bounce here also proves
	// the filter is gone.
	busy := &contract.Feature{Slug: "busy", Root: cappedRoot, Class: contract.ClassUnfinished, Status: "plan-accepted"}
	m := cappedModel(t, []string{"gogo-go-busy"}, target, busy)

	bounce := m.capBounce(target)
	if bounce == "" {
		t.Fatal("no cap bounce - note that `busy` is deliberately ClassUnfinished here: before FR12 the " +
			"class filter made this live build invisible, so this test also proves the filter is gone")
	}
	// Assert the bounce quotes the SHARED clause, not a hand-written copy of today's
	// wording: hand-copying is what left three of the four cap surfaces stating the old rule
	// (REV-002), and a test that pins its own copy of the sentence has the same defect.
	if !strings.Contains(bounce, orchestrator.CapRuleClause) {
		t.Errorf("bounce does not quote orchestrator.CapRuleClause: %q", bounce)
	}
	if strings.Contains(bounce, "in-progress work items with a live session") {
		t.Errorf("bounce still states the OLD (now wrong) rule: %q", bounce)
	}
	if !strings.Contains(bounce, "press M to force") {
		t.Errorf("bounce lost the M affordance 0.28.0 added: %q", bounce)
	}
	// Every blocker must be actionable (REV-008). Since the cap counts a live build session
	// regardless of the feature's status, a blocker can be an ALREADY-SHIPPED item whose
	// session outlived a failed ship-reap - and for that one "ship one" is useless advice, so
	// the bounce must also name a sweep.
	for _, unblock := range []string{"ship one", "--force"} {
		if !strings.Contains(bounce, unblock) {
			t.Errorf("bounce does not name the %q unblock: %q", unblock, bounce)
		}
	}
	// The blocking slug is named, which is what makes a stale blocker diagnosable at all.
	if !strings.Contains(bounce, "busy") {
		t.Errorf("bounce does not name the blocking slug: %q", bounce)
	}
	// REV-011: the sweep must be the TARGETED form. A bare `gogo sweep` is host-global - it
	// judges every gogo-* session on the machine against THIS repo's features, so on a
	// multi-source host it kills another source's in-flight build as an "orphan", with no
	// confirmation. Assert the exact targeted substring AND the absence of the bare form, so
	// a later re-copy cannot reintroduce the destructive advice.
	if !strings.Contains(bounce, "`gogo sweep busy`") {
		t.Errorf("bounce does not name the TARGETED sweep `gogo sweep busy`: %q", bounce)
	}
	if strings.Contains(bounce, "`gogo sweep`") {
		t.Errorf("bounce recommends the BARE, host-global `gogo sweep`, which reaps other sources' "+
			"live builds as orphans: %q", bounce)
	}
}

// TestConfigTabCapSurfacesRenderTheRule is the behavioural half of REV-012's fix. config_tab.go
// owns TWO cap surfaces, so a FILE-level grep for the shared constant could be satisfied by
// one of them plus three comments naming it - which is exactly how a fresh hand-written
// sentence in the field the user reads WHILE SETTING the cap passed the whole suite.
//
// A rendered assertion cannot be satisfied by a comment or by a sibling surface, so assert each
// producer's actual output. Every pre-0.29.0 phrasing contains "in-progress", which is why that
// one substring covers them all without duplicating the stale-phrase list across packages.
func TestConfigTabCapSurfacesRenderTheRule(t *testing.T) {
	surfaces := map[string]string{
		"the source-edit cap field (capFieldDescription)": capFieldDescription(),
		"the source-detail note (capScopeNote)":           capScopeNote(1),
	}
	for name, got := range surfaces {
		if !strings.Contains(got, orchestrator.CapRuleClause) {
			t.Errorf("%s does not state the shared cap rule.\n  got:  %q\n  want it to contain: %q",
				name, got, orchestrator.CapRuleClause)
		}
		if strings.Contains(got, "in-progress") {
			t.Errorf("%s still states a pre-0.29.0 class-filtered rule: %q", name, got)
		}
	}
	// The unlimited arm deliberately says nothing about counting.
	if note := capScopeNote(0); note != "(unlimited)" {
		t.Errorf("capScopeNote(0) = %q, want %q", note, "(unlimited)")
	}

	// --- and the CALL SITES must actually use those producers (REV-016) ---
	//
	// Asserting a producer's return value proves the producer is right, not that the surface
	// uses it. Both wirings could be hand-written back while the producers stayed correct and
	// referenced, leaving the per-surface count at 2 and every assertion above green - review
	// standard #11(b) verbatim, one level up from REV-012, on the surface REV-002 went stale
	// on. So pin each wiring by the strongest means available to it.

	// capScopeNote's site is a Model method returning a string, so assert the RENDERED detail
	// pane. A rendered assertion cannot be satisfied by a sibling producer.
	seedDataHome(t)
	repo := gogoRepoDir(t)
	proj := projects.Project{Name: "app", Sources: []projects.Source{
		{Name: "svc", Path: repo, ConcurrentWorkItems: 1},
	}}
	if err := projects.Save(&proj); err != nil {
		t.Fatalf("save project: %v", err)
	}
	detail := configTab(sizedWorkspace(t, &contract.Repo{}, proj)).viewConfigRight()
	if !strings.Contains(detail, orchestrator.CapRuleClause) {
		t.Errorf("the config tab's source-detail pane does not RENDER the shared cap rule - capScopeNote "+
			"can be correct and simply not called. Detail:\n%s", detail)
	}
	if strings.Contains(detail, "in-progress") {
		t.Errorf("the source-detail pane states a pre-0.29.0 class-filtered rule:\n%s", detail)
	}

	// The huh field's Description is not readable back off the built form, so pin that wiring
	// STRUCTURALLY with the comment-stripping helper already in this file - the same shape as
	// TestFooterChipDoesNoDiskIO. Both call sites get a structural check, so neither wiring
	// rests on a SINGLE assertion: a two-part mutation (hand-write the surface AND disable one
	// guard) showed capScopeNote's site was pinned only by the rendered assertion above.
	wirings := []struct {
		fn, mustCall, mustNotContain string
	}{
		{"startSourceForm", "capFieldDescription(", `"0 = unlimited`},
		{"viewConfigRight", "capScopeNote(", `"(counts`},
	}
	for _, w := range wirings {
		body := tuiFuncBody(t, "config_tab.go", w.fn)
		if !strings.Contains(body, w.mustCall) {
			t.Errorf("%s no longer calls %s - the cap surface it renders can then carry fresh, unreviewed "+
				"copy while the producer stays correct and the whole suite stays green (REV-016). Body:\n%s",
				w.fn, w.mustCall, body)
		}
		if strings.Contains(body, w.mustNotContain) {
			t.Errorf("%s hand-writes its cap text (found %s) instead of calling %s. Body:\n%s",
				w.fn, w.mustNotContain, w.mustCall, body)
		}
	}
}

// TestCapRefusalsComposeThroughCapRefusal pins REV-019's fix structurally, because it cannot be
// pinned behaviourally: both call sites sit behind CapExceeded, which needs a non-empty blocker
// list, so the empty-remedy case they now handle is unreachable from either - and with a
// non-empty list the composed and the hand-interpolated forms render identically.
//
// The composition is what makes CapSweepRemedy's documented "" contract real rather than
// merely stated, so a silent revert to unconditional interpolation would restore the ", ,"
// hazard for the first future caller (a plans-tab cap message, a `gogo status` hint) without
// failing anything. Same technique as TestFooterChipDoesNoDiskIO: read the source, comments
// stripped.
func TestCapRefusalsComposeThroughCapRefusal(t *testing.T) {
	body := tuiFuncBody(t, "move.go", "capBounce")
	if !strings.Contains(body, "orchestrator.CapRefusal(") {
		t.Errorf("capBounce no longer composes its remedies through orchestrator.CapRefusal, so an empty "+
			"remedy would render as double punctuation (REV-019). Body:\n%s", body)
	}
	if strings.Contains(body, `", "+orchestrator.CapSweepRemedy(`) {
		t.Errorf("capBounce interpolates the sweep remedy unconditionally again. Body:\n%s", body)
	}
}

// TestCapSweepRemedyIsAlwaysTargeted pins REV-011 at the shared producer, so neither cap
// surface can emit a bare `gogo sweep` however it composes its sentence.
func TestCapSweepRemedyIsAlwaysTargeted(t *testing.T) {
	cases := []struct {
		blocking []string
		want     string
	}{
		{[]string{"one"}, "run `gogo sweep one` if a blocker already shipped"},
		// Several blockers: the space-separated form is still the TARGETED one.
		{[]string{"one", "two"}, "run `gogo sweep one two` if a blocker already shipped"},
		// Nothing to name → no remedy at all, rather than a bare sweep.
		{nil, ""},
	}
	for _, c := range cases {
		if got := orchestrator.CapSweepRemedy(c.blocking); got != c.want {
			t.Errorf("CapSweepRemedy(%v) = %q, want %q", c.blocking, got, c.want)
		}
	}
	if got := orchestrator.CapSweepRemedy([]string{"one"}); strings.Contains(got, "`gogo sweep`") {
		t.Errorf("CapSweepRemedy emitted a bare sweep: %q", got)
	}

	// REV-019: the "" return only means something if callers DROP it. Both refusals compose
	// through CapRefusal, so an empty remedy disappears instead of rendering ", ,". Unreachable
	// from either site today (both sit behind CapExceeded, which needs a non-empty active set),
	// but the "" exists precisely so the function cannot degrade into naming the bare sweep -
	// and a future caller would meet the empty case first.
	got := orchestrator.CapRefusal("press M to force", "ship one", orchestrator.CapSweepRemedy(nil), "or --force")
	if got != "press M to force, ship one, or --force" {
		t.Errorf("CapRefusal with an empty remedy = %q, want the empty part dropped", got)
	}
	if strings.Contains(got, ", ,") {
		t.Errorf("CapRefusal rendered double punctuation: %q", got)
	}
	if all := orchestrator.CapRefusal("", "", ""); all != "" {
		t.Errorf("CapRefusal of all-empty = %q, want \"\"", all)
	}
}

// TestAcceptedButUnwrittenPlanRefused pins FR8 on the board path, and pins that the two gates
// give DIFFERENT diagnoses: "still being authored" (nothing to accept) vs "plan-accepted but
// its plan.md is not written" (nothing to build). A test that only asserted "it refused"
// would pass even if the wrong gate fired.
func TestAcceptedButUnwrittenPlanRefused(t *testing.T) {
	f := authoringFeature(t, "demo", 0)
	f.Status = "plan-accepted"
	m := focusOn(newModel(t), f)
	in, _, bounce := m.attemptAction(false)
	if bounce == "" {
		t.Fatalf("m on a plan-accepted card with no plan.md produced %+v", in)
	}
	for _, want := range []string{"demo is plan-accepted", "plan.md is not written", "no plan.md on disk yet", "nothing to build", "gogo plan demo"} {
		if !strings.Contains(bounce, want) {
			t.Errorf("FR8 bounce %q missing %q", bounce, want)
		}
	}
	if strings.Contains(bounce, "still being authored") {
		t.Errorf("FR8 bounce used the AUTHORING wording - the two gates must be distinguishable: %q", bounce)
	}
	// force must not override this one either.
	if _, _, forced := m.attemptActionForce(false, true); forced == "" {
		t.Error("M forced past FR8's missing-plan refusal")
	}
}

// TestWrittenPlanStillAccepts guards TestAcceptMoveGuard from regressing through the new
// gate: a WRITTEN plan awaiting acceptance still routes `m` to /gogo:accept, uncapped.
func TestWrittenPlanStillAccepts(t *testing.T) {
	m := focusOn(newModel(t), authoringFeature(t, "demo", 8))
	in, ship, bounce := m.attemptAction(false)
	if bounce != "" || ship || in.Action != launch.ActionAccept {
		t.Fatalf("m on a WRITTEN plan-pending card: intent=%+v ship=%v bounce=%q, want ActionAccept", in, ship, bounce)
	}
	if in.Command != "/gogo:accept demo" {
		t.Errorf("accept command = %q", in.Command)
	}
}

// TestFooterDoesNotOfferAnIllegalMove: the footer chip is the board's promise about what
// `m` will do, so it must not advertise "accept" on a card whose `m` bounces - that is the
// same say-one-thing-do-another defect in miniature. Asserted in the RENDERED footer.
func TestFooterDoesNotOfferAnIllegalMove(t *testing.T) {
	cases := []struct {
		name    string
		f       *contract.Feature
		want    string
		notWant string
	}{
		{"authoring", authoringFeature(t, "demo", 0), "[m] ✗ plan not written", "[m] accept"},
		{"stub", authoringFeature(t, "demo", 1), "[m] ✗ plan not written", "[m] accept"},
		{"written gate", authoringFeature(t, "demo", 8), "[m] accept", "plan not written"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := focusOn(newModel(t), c.f).View()
			if !strings.Contains(out, c.want) {
				t.Errorf("footer missing %q; footer:\n%s", c.want, lastLines(out, 2))
			}
			if strings.Contains(out, c.notWant) {
				t.Errorf("footer still offers %q; footer:\n%s", c.notWant, lastLines(out, 2))
			}
		})
	}
	// The FR8 case (plan-accepted, no plan) must not offer "[m] go" either.
	f := authoringFeature(t, "demo", 0)
	f.Status = "plan-accepted"
	out := focusOn(newModel(t), f).View()
	if strings.Contains(out, "[m] go") {
		t.Errorf("footer offers `[m] go` for a plan-accepted card with no plan.md; footer:\n%s", lastLines(out, 2))
	}
}

// --- FR14 / FR15: the live-vs-file cues ----------------------------------------------

// TestBuildingCueOnStaleCard pins FR14: in the launch-to-first-write window a live
// `gogo-go-<slug>` session contradicts a file that still says plan-accepted. The card keeps
// its FILE-derived column (D6=A, one source of truth for placement) and gains the amber
// `● building` chip.
func TestBuildingCueOnStaleCard(t *testing.T) {
	f := authoringFeature(t, "demo", 8)
	f.Status = "plan-accepted"
	sessions := []string{"gogo-go-demo"}
	if !buildingDisagreement(f, sessions) {
		t.Fatal("buildingDisagreement = false for a live go session on a plan-accepted card")
	}
	if got := cardStateCue(f, sessions); got != buildingMarker+" building" {
		t.Errorf("cardStateCue = %q, want %q", got, buildingMarker+" building")
	}
	if f.Column() != contract.ColPlan {
		t.Errorf("column = %q, want the FILE-derived plan column (D6=A cues, it does not move the card)", f.Column())
	}
	// Rendered, not just computed.
	m := focusOn(newModel(t), f)
	m.sessions = sessions
	if out := m.View(); !strings.Contains(out, buildingMarker+" building") {
		t.Errorf("View() carries no `● building` chip; board:\n%s", out)
	}
}

// TestAuthoringNeverReadsBuilding: an AUTHORING card with a live gogo-plan session shows the
// authoring pill and NEVER the building chip - the discrimination that stops Slice B from
// papering over Slice A.
func TestAuthoringNeverReadsBuilding(t *testing.T) {
	auth := authoringFeature(t, "demo", 0)
	sessions := []string{"gogo-plan-demo"}
	if buildingDisagreement(auth, sessions) {
		t.Error("an authoring card with a live PLAN session reads as building")
	}
	if got := cardStateCue(auth, sessions); got != "" {
		t.Errorf("cardStateCue = %q, want \"\" for an authoring card", got)
	}
	if got := sessionAgent(auth, sessions); got != "analyst" {
		t.Errorf("sessionAgent = %q, want analyst", got)
	}
}

// TestAgentChipFollowsTheSession pins FR14's third symptom: renderCard derived its agent from
// f.Phase, still `plan` for the whole of a build, so a card being BUILT displayed `● analyst`.
func TestAgentChipFollowsTheSession(t *testing.T) {
	f := authoringFeature(t, "demo", 8)
	f.Status, f.Phase = "plan-accepted", "plan"
	sessions := []string{"gogo-go-demo"}
	if got := activeAgent(f); got != "analyst" {
		t.Fatalf("activeAgent = %q - the fixture no longer reproduces the stale-phase case", got)
	}
	if got := sessionAgent(f, sessions); got != "developer" {
		t.Errorf("sessionAgent = %q, want developer (never analyst, for a card being built)", got)
	}
	// A phase that a warm `go` session genuinely runs keeps its more precise label.
	f.Phase, f.Status = "review", "reviewing"
	if got := sessionAgent(f, sessions); got != "reviewer" {
		t.Errorf("sessionAgent for a reviewing card = %q, want reviewer (the phase line is more precise)", got)
	}
	// Rendered: the chip on the card, not just the model.
	f.Phase, f.Status = "plan", "plan-accepted"
	m := focusOn(newModel(t), f)
	m.sessions = sessions
	out := m.View()
	if !strings.Contains(out, "● developer") {
		t.Errorf("View() has no `● developer` chip; board:\n%s", out)
	}
	if strings.Contains(out, "● analyst") {
		t.Errorf("View() still shows `● analyst` on a card being built:\n%s", out)
	}
}

// TestStalledCueOnKilledPhase pins FR15, the mirror direction. Once the phases write their
// status at ENTRY, a killed build LEAVES `implementing` on disk - so the honest writer would
// trade one silent lie for another unless a session-less working status reads as stalled.
func TestStalledCueOnKilledPhase(t *testing.T) {
	f := &contract.Feature{Slug: "demo", Root: fixtureRoot, Status: "implementing", Phase: "implement", Class: contract.ClassInProgress}
	if got := cardStateCue(f, nil); got != stalledMarker+" stalled" {
		t.Errorf("cardStateCue with no live session = %q, want %q", got, stalledMarker+" stalled")
	}
	if got := cardStateCue(f, []string{"gogo-go-demo"}); got != "" {
		t.Errorf("cardStateCue with a LIVE session = %q, want \"\" (it is running, not stalled)", got)
	}
	for _, s := range []string{"reviewing", "testing"} {
		f.Status = s
		if got := cardStateCue(f, nil); got != stalledMarker+" stalled" {
			t.Errorf("cardStateCue(%s, no session) = %q, want stalled", s, got)
		}
	}
	// A non-working status is never "stalled" - a plan-accepted card is just parked.
	f.Status = "plan-accepted"
	if got := cardStateCue(f, nil); got != "" {
		t.Errorf("cardStateCue(plan-accepted, no session) = %q, want \"\"", got)
	}
	// Rendered, with no ● dot (there is no session to dot).
	f.Status = "implementing"
	m := newModel(t)
	m.cols[1] = []*contract.Feature{f}
	m.colIdx, m.cardIdx[1] = 1, 0
	out := m.View()
	if !strings.Contains(out, stalledMarker+" stalled") {
		t.Errorf("View() carries no `· stalled` cue; board:\n%s", out)
	}
}

// TestPhaseLineLagsCue is REV-006's reader-side guard: FR11's entry write is LLM prose and was
// skipped on its very first live run (gogo-review kept implement/implementing for a whole
// round), and neither of the other two cues can see that - `● building` and the cap both key on
// the live `gogo-go` session, which is present for the WHOLE warm run through the later phases.
// The deterministic evidence is the telemetry: a `phase-done` for the phase state.md still
// claims to be in, while a build session is live.
func TestPhaseLineLagsCue(t *testing.T) {
	// The exact shape observed live: implement finished (its phase-done is the last event),
	// review started and did NOT write its occupancy, and the warm gogo-go session is alive.
	lagging := &contract.Feature{
		Slug: "demo", Root: fixtureRoot, Phase: "implement", Status: "implementing",
		Class:       contract.ClassInProgress,
		LatestEvent: &contract.Event{Event: "phase-done", Phase: "implement"},
	}
	sessions := []string{"gogo-go-demo"}
	if !phaseLineLags(lagging, sessions) {
		t.Fatal("phaseLineLags = false for a phase-done + live build session - the skipped entry write is invisible")
	}
	if got := cardStateCue(lagging, sessions); got != stalledMarker+" state lags" {
		t.Errorf("cardStateCue = %q, want %q", got, stalledMarker+" state lags")
	}
	// Rendered on the card, not just computed.
	m := newModel(t)
	m.cols[1] = []*contract.Feature{lagging}
	m.colIdx, m.cardIdx[1] = 1, 0
	m.sessions = sessions
	if out := m.View(); !strings.Contains(out, stalledMarker+" state lags") {
		t.Errorf("View() carries no `· state lags` cue; board:\n%s", out)
	}

	// The healthy hand-off blink, pinned so the trade-off is deliberate. Since the exit write
	// was restored (each phase writes phase/status at its END as well as its start - belt and
	// braces, because the entry write is prose that has been skipped n=3 times), every healthy
	// hand-off passes through arm A's exact shape for the gap between one phase's exit write and
	// the next phase's entry write. Strictly it is true there - review HAS ended and nothing has
	// claimed test yet - and it self-clears on the next write, exactly like `● building` over the
	// launch-to-first-write window. The SAME shape stays lit for a whole phase when the entry
	// write is genuinely skipped, which is the difference the user sees; deleting arm A to
	// silence the blink would blind the detector to the failure it exists for.
	handoff := &contract.Feature{
		Slug: "demo", Root: fixtureRoot, Phase: "review", Status: "reviewing",
		Class:       contract.ClassInProgress,
		LatestEvent: &contract.Event{Event: "phase-done", Phase: "review"},
	}
	if !phaseLineLags(handoff, sessions) {
		t.Error("the exit-write hand-off window no longer matches arm A - if that was deliberate, the " +
			"long-lived skipped-entry-write case it shares a shape with must still be covered")
	}

	// --- arm B (REV-009): an ENTRY event naming a phase the line does not ---
	lagShapes := []struct {
		name string
		f    *contract.Feature
	}{
		{
			// The pipeline's MOST COMMON re-entry: review found issues, implement re-entered
			// with --issues and appended its fix-round event, but skipped the occupancy write.
			// state.md says review/reviewing while implement is editing code.
			"fix-round/implement while the line still reads review",
			&contract.Feature{Slug: "demo", Root: fixtureRoot, Phase: "review", Status: "reviewing",
				Class:       contract.ClassInProgress,
				LatestEvent: &contract.Event{Event: "fix-round", Phase: "implement"}},
		},
		{
			// A PARTIAL step 1: the event half landed, the state.md half did not.
			"phase-started/review while the line still reads implement",
			&contract.Feature{Slug: "demo", Root: fixtureRoot, Phase: "implement", Status: "implementing",
				Class:       contract.ClassInProgress,
				LatestEvent: &contract.Event{Event: "phase-started", Phase: "review"}},
		},
		{
			// The forward hand-off into ⑤ with the test line left behind (knowledge↔report).
			"phase-started/report while the line still reads test",
			&contract.Feature{Slug: "demo", Root: fixtureRoot, Phase: "test", Status: "testing",
				Class:       contract.ClassInProgress,
				LatestEvent: &contract.Event{Event: "phase-started", Phase: "report"}},
		},
	}
	for _, c := range lagShapes {
		t.Run(c.name, func(t *testing.T) {
			if !phaseLineLags(c.f, sessions) {
				t.Errorf("phaseLineLags = false - the telemetry names a different phase than the phase " +
					"line, which is stronger evidence of a skipped occupancy write than the phase-done shape")
			}
			if got := cardStateCue(c.f, sessions); got != stalledMarker+" state lags" {
				t.Errorf("cardStateCue = %q, want the state-lags cue", got)
			}
		})
	}

	// --- and it must be SILENT in every healthy shape ---
	healthy := []struct {
		name string
		f    *contract.Feature
		sess []string
	}{
		{
			// Arm B must NOT fire on an entry event that AGREES with the phase line - that is
			// simply a phase mid-flight, the healthiest shape there is.
			"an entry event matching the phase line",
			&contract.Feature{Slug: "demo", Phase: "review", Status: "reviewing", Class: contract.ClassInProgress,
				LatestEvent: &contract.Event{Event: "phase-started", Phase: "review"}},
			sessions,
		},
		{
			// The boundary that makes arm B PRECISE rather than merely broad: a MID-PHASE event
			// from an earlier phase is ordinary telemetry lag (docs/cli-contract.md §5 calls it
			// normal), not a skipped write - implement here re-entered and DID write its state,
			// so reporting a lag would blame a writer that did everything right.
			"a mid-phase event from the previous phase (issues-found/review)",
			&contract.Feature{Slug: "demo", Phase: "implement", Status: "implementing", Class: contract.ClassInProgress,
				LatestEvent: &contract.Event{Event: "issues-found", Phase: "review"}},
			sessions,
		},
		{
			// REV-010: a TERMINAL item with a lingering build session is not lagging - nothing
			// is running. Reachable: exactly REV-008's scenario (a session that outlived a
			// failed ship-reap), and an aborted item whose phase is implement classifies
			// ClassInProgress, so it renders as a real card rather than a changelog row.
			"aborted with a lingering build session",
			&contract.Feature{Slug: "demo", Phase: "implement", Status: "aborted", Class: contract.ClassInProgress,
				LatestEvent: &contract.Event{Event: "phase-done", Phase: "implement"}},
			sessions,
		},
		{
			"shipped with a lingering build session",
			&contract.Feature{Slug: "demo", Phase: "implement", Status: "shipped", Class: contract.ClassInProgress,
				LatestEvent: &contract.Event{Event: "phase-done", Phase: "implement"}},
			sessions,
		},
		{
			// The entry write HAPPENED: state.md moved to review, so the last phase-done
			// (implement) no longer names the current phase.
			"the entry write was honoured",
			&contract.Feature{Slug: "demo", Phase: "review", Status: "reviewing", Class: contract.ClassInProgress,
				LatestEvent: &contract.Event{Event: "phase-done", Phase: "implement"}},
			sessions,
		},
		{
			// Mid-phase: the last event is the phase's START, not its end.
			"phase-started is not phase-done",
			&contract.Feature{Slug: "demo", Phase: "implement", Status: "implementing", Class: contract.ClassInProgress,
				LatestEvent: &contract.Event{Event: "phase-started", Phase: "implement"}},
			sessions,
		},
		{
			// A user gate legitimately sits after a completed phase - state.md is CORRECT.
			"awaiting-uat after report's phase-done",
			&contract.Feature{Slug: "demo", Phase: "knowledge", Status: "awaiting-uat", Class: contract.ClassReadyToShip,
				LatestEvent: &contract.Event{Event: "phase-done", Phase: "report"}},
			sessions,
		},
		{
			// No session: nothing is continuing, so there is no lag - that is `· stalled`.
			"no live build session",
			&contract.Feature{Slug: "demo", Phase: "implement", Status: "implementing", Class: contract.ClassInProgress,
				LatestEvent: &contract.Event{Event: "phase-done", Phase: "implement"}},
			nil,
		},
		{
			// Telemetry is best-effort and absent on older features - degrade silently.
			"no events.jsonl at all",
			&contract.Feature{Slug: "demo", Phase: "implement", Status: "implementing", Class: contract.ClassInProgress},
			sessions,
		},
		{
			// An AUTHORING session is not a build, so it never implies a phase moved on.
			"a live plan session only",
			&contract.Feature{Slug: "demo", Phase: "implement", Status: "implementing", Class: contract.ClassInProgress,
				LatestEvent: &contract.Event{Event: "phase-done", Phase: "implement"}},
			[]string{"gogo-plan-demo"},
		},
	}
	for _, c := range healthy {
		t.Run(c.name, func(t *testing.T) {
			if phaseLineLags(c.f, c.sess) {
				t.Errorf("phaseLineLags = true - a false positive here would make the cue noise")
			}
		})
	}
	// The knowledge→report phase-name mapping is load-bearing: state.md's fifth phase is
	// `knowledge` while events call it `report`, so a naive string compare would miss ⑤.
	report := &contract.Feature{Slug: "demo", Phase: "knowledge", Status: "testing", Class: contract.ClassInProgress,
		LatestEvent: &contract.Event{Event: "phase-done", Phase: "report"}}
	if !phaseLineLags(report, sessions) {
		t.Error("phaseLineLags missed the knowledge/report phase-name mapping (contract.EventsPhase)")
	}
}

// TestCueArmsAreMutuallyExclusive is REV-013's guard, and it pins the invariant that actually
// holds rather than the one the comment used to claim.
//
// The review found that cardStateCue's documented "Order matters" was unpinned: swapping the
// arms survived the whole suite. Investigating it showed WHY - after REV-010 gave phaseLineLags
// a working-status whitelist, its status set (implementing/reviewing/testing) became disjoint
// from buildingDisagreement's (plan-accepted/awaiting-plan-acceptance), so no swap CAN change
// behaviour and no test could ever fail on one. The reviewer's overlap fixture (a UAT-rerun
// card at plan-accepted) is likewise no longer an overlap.
//
// So the honest guard is the exclusivity itself, asserted over a cross-product. It cannot be
// escaped: widening any arm's status set into another's makes this fail, forcing the precedence
// to be decided deliberately instead of inherited from the switch order.
func TestCueArmsAreMutuallyExclusive(t *testing.T) {
	statuses := []string{
		"awaiting-plan-acceptance", "plan-accepted", "implementing", "reviewing", "testing",
		"waiting-for-user", "awaiting-uat", "done", "shipped", "aborted", "",
	}
	phases := []string{"plan", "implement", "review", "test", "knowledge", ""}
	events := []*contract.Event{
		nil,
		{Event: "phase-done", Phase: "implement"},
		{Event: "phase-done", Phase: "review"},
		{Event: "phase-done", Phase: "report"},
		{Event: "phase-started", Phase: "review"},
		{Event: "phase-started", Phase: "report"},
		{Event: "fix-round", Phase: "implement"},
		{Event: "issues-found", Phase: "review"},
		{Event: "plan-accepted", Phase: "plan"},
	}
	sessionSets := [][]string{nil, {"gogo-go-demo"}, {"gogo-plan-demo"}, {"gogo-go-demo", "gogo-plan-demo"}}

	overlaps := 0
	for _, st := range statuses {
		for _, ph := range phases {
			for _, ev := range events {
				for _, ss := range sessionSets {
					f := &contract.Feature{Slug: "demo", Status: st, Phase: ph, LatestEvent: ev}
					n := 0
					for _, on := range []bool{buildingDisagreement(f, ss), phaseLineLags(f, ss), stalledPhase(f, ss)} {
						if on {
							n++
						}
					}
					if n > 1 {
						overlaps++
						if overlaps <= 3 { // report a few, not thousands
							t.Errorf("%d cue arms are true at once for status=%q phase=%q event=%+v sessions=%v - "+
								"the switch ORDER now silently decides which cue the user sees. Decide the "+
								"precedence explicitly (and say so in cardStateCue's comment) rather than "+
								"leaving it to arm order.", n, st, ph, ev, ss)
						}
					}
				}
			}
		}
	}
	if overlaps > 0 {
		t.Errorf("%d overlapping combinations in total", overlaps)
	}

	// Sanity: the matrix must actually REACH each arm, or the exclusivity above would be
	// vacuously true (an all-false matrix trivially never overlaps) - the compensated-mutation
	// trap this feature has now hit six times.
	reached := map[string]bool{}
	for _, st := range statuses {
		for _, ph := range phases {
			for _, ev := range events {
				for _, ss := range sessionSets {
					f := &contract.Feature{Slug: "demo", Status: st, Phase: ph, LatestEvent: ev}
					if buildingDisagreement(f, ss) {
						reached["building"] = true
					}
					if phaseLineLags(f, ss) {
						reached["lags"] = true
					}
					if stalledPhase(f, ss) {
						reached["stalled"] = true
					}
				}
			}
		}
	}
	for _, arm := range []string{"building", "lags", "stalled"} {
		if !reached[arm] {
			t.Errorf("the cross-product never made %q true, so the exclusivity assertion is vacuous there", arm)
		}
	}
}

// TestStalledStaysRunnable pins FR15's promise that the cue is a CUE, not a new gate: a
// stalled feature is still resumable, so `gogo go` picks it up.
func TestStalledStaysRunnable(t *testing.T) {
	f := &contract.Feature{Slug: "demo", Root: fixtureRoot, Status: "implementing", Class: contract.ClassInProgress}
	m := newModel(t)
	m.cols[1] = []*contract.Feature{f}
	m.colIdx, m.cardIdx[1] = 1, 0
	in, _, bounce := m.attemptAction(false)
	if bounce != "" || in.Action != launch.ActionGo {
		t.Errorf("m on a stalled card: intent=%+v bounce=%q, want a plain resume (ActionGo)", in, bounce)
	}
}

// --- the drill-in note ---------------------------------------------------------------

// TestQuickViewNamesTheMissingPlan: `v` on a plan-column card with no plan.md used to fall
// SILENTLY to the file list, so the single most confusing card on the board answered a
// keypress with no message at all.
func TestQuickViewNamesTheMissingPlan(t *testing.T) {
	m := focusOn(newModel(t), authoringFeature(t, "demo", 0))
	got := send(m, runes("v"))
	if got.status == "" {
		t.Fatal("v on an authoring card set no status - the silent no-op is still there")
	}
	if got.statusLevel != statusLevelWarn {
		t.Errorf("statusLevel = %v, want warn (it is a gate and it names its unblock)", got.statusLevel)
	}
	for _, want := range []string{"plan.md not written yet", "gogo plan demo"} {
		if !strings.Contains(got.status, want) {
			t.Errorf("status %q missing %q", got.status, want)
		}
	}
	if got.mode != modeDrill {
		t.Errorf("mode = %v, want the file-list fallback (modeDrill)", got.mode)
	}
	// RENDERED, not just set (review check #8 / the 0.16.0 drill-card finding): the drill
	// status line is literally the path that once shipped a silent no-op, because the unit
	// test asserted Model.status while viewDrill() never rendered it. Assert the glyph, the
	// reason and the unblock in the drill's own View() output.
	out := got.View()
	for _, want := range []string{statusWarnMarker, "plan.md not written yet", "no plan.md on disk yet", "gogo plan demo"} {
		if !strings.Contains(out, want) {
			t.Errorf("viewDrill() output missing %q - a status set on the model but never rendered is a "+
				"silent no-op; drill:\n%s", want, out)
		}
	}
	// The note is the SAME sentence the move guard bounces with (one producer, no drift).
	if bounce := planReadinessBounce(m.focusedCard()); bounce != got.status {
		t.Errorf("the v note and the m bounce diverged:\n  v: %q\n  m: %q", got.status, bounce)
	}
}

// TestPlanUnreadyAgreesWithTheBounce pins the REV-007 split: planUnready is the pure,
// no-I/O predicate the RENDER path uses, and planReadinessBounce is the sentence the
// keystroke paths show. They must answer the same question, or the footer chip and the
// refusal disagree about the same card - so assert the equivalence directly rather than
// trusting two copies of the condition to stay aligned.
func TestPlanUnreadyAgreesWithTheBounce(t *testing.T) {
	authoring := authoringFeature(t, "demo", 0)
	stub := authoringFeature(t, "demo", 1)
	written := authoringFeature(t, "demo", 8)
	acceptedNoPlan := authoringFeature(t, "demo", 0)
	acceptedNoPlan.Status = "plan-accepted"
	acceptedWritten := authoringFeature(t, "demo", 8)
	acceptedWritten.Status = "plan-accepted"
	building := &contract.Feature{Slug: "demo", Status: "implementing", PlanUnwritten: true}

	cases := []struct {
		name string
		f    *contract.Feature
		want bool
	}{
		{"authoring (no plan)", authoring, true},
		{"authoring (stub)", stub, true},
		{"written plan at the gate", written, false},
		{"plan-accepted, no plan (FR8)", acceptedNoPlan, true},
		{"plan-accepted, written", acceptedWritten, false},
		{"mid-pipeline with no plan (resumable, not refused)", building, false},
		{"nil", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := planUnready(c.f); got != c.want {
				t.Errorf("planUnready = %v, want %v", got, c.want)
			}
			if bounced := planReadinessBounce(c.f) != ""; bounced != c.want {
				t.Errorf("planReadinessBounce non-empty = %v, want %v - the pure predicate and the "+
					"sentence producer disagree, so the footer chip and the refusal would too", bounced, c.want)
			}
		})
	}
}

// TestFooterChipDoesNoDiskIO pins REV-007 STRUCTURALLY, because it cannot be pinned
// behaviourally. planUnready and planReadinessBounce answer the same question off the same
// model fields (only the reason CLAUSE reads plan.md), so swapping one for the other in
// footerChips changes no output - a mutation sweep confirmed the behavioural version of this
// test survived that exact revert. What differs is that the render path would open and scan
// plan.md, and build a refusal sentence it discards, on EVERY View(): every keystroke, every
// fsnotify reload. So assert the source: footerChips must not reach for the message producer.
//
// This is test-strategy.md's "prefer a guard that cannot be escaped" applied to a
// non-functional property - the only kind of assertion that can hold it.
func TestFooterChipDoesNoDiskIO(t *testing.T) {
	// The decision moved from footerChips into moveChip when the two class arms were
	// collapsed into one producer (REV-030), so guard BOTH: neither may reach the
	// disk-touching producer, and moveChip must still be the one deciding.
	for _, fn := range []string{"footerChips", "moveChip"} {
		body := tuiFuncBody(t, "view.go", fn)
		if strings.Contains(body, "planReadinessBounce") {
			t.Errorf("%s calls planReadinessBounce, which opens and scans plan.md and formats a "+
				"refusal sentence it then throws away - on every View(). Use the pure planUnready(f) "+
				"predicate in render paths (REV-007). Body:\n%s", fn, body)
		}
	}
	if body := tuiFuncBody(t, "view.go", "moveChip"); !strings.Contains(body, "planUnready(") {
		t.Errorf("moveChip no longer decides the [m] chip with planUnready - if it stopped deciding "+
			"this at all, the cross-product sweep should have failed too. Body:\n%s", body)
	}
	// And the behavioural half that IS observable: the loaded model decides, so deleting
	// plan.md under a Feature whose PlanUnwritten is already false changes nothing.
	f := authoringFeature(t, "demo", 8) // written plan → PlanUnwritten false
	if err := os.Remove(filepath.Join(f.Dir, "plan.md")); err != nil {
		t.Fatalf("remove plan.md: %v", err)
	}
	out := focusOn(newModel(t), f).View()
	if !strings.Contains(out, "[m] accept") {
		t.Errorf("the footer changed after plan.md was deleted from disk, so the render answer is not "+
			"coming from the loaded Feature; footer:\n%s", lastLines(out, 2))
	}
}

// tuiFuncBody returns the CODE of a top-level func in a tui source file - its text between
// the opening line and the next closing brace in column 0, with `//` comments stripped.
//
// Stripping comments is load-bearing, not tidiness: a structural guard must assert what the
// code DOES. Without it, a comment explaining "planUnready, not planReadinessBounce" trips a
// guard that bans planReadinessBounce - the guard would fail on prose describing the fix it
// is enforcing.
func tuiFuncBody(t *testing.T, file, name string) string {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	src := string(raw)
	i := strings.Index(src, "\nfunc "+name+"(")
	if i < 0 {
		i = strings.Index(src, "\nfunc (m Model) "+name+"(")
	}
	if i < 0 {
		i = strings.Index(src, "\nfunc (m *Model) "+name+"(")
	}
	if i < 0 {
		t.Fatalf("func %s not found in %s", name, file)
	}
	rest := src[i+1:]
	end := strings.Index(rest, "\n}\n")
	if end < 0 {
		t.Fatalf("could not find the end of func %s", name)
	}
	return stripLineComments(rest[:end])
}

// stripLineComments removes `//` line comments so a structural guard reads code, not prose.
// It ignores a `//` inside a string literal (naive but sufficient for these guards, and it
// errs toward KEEPING text, so it can only ever make a guard stricter, never blind).
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
				i = len(line) // stop
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// TestQuickViewUnchangedForAWrittenPlan: the note must not fire where the old behaviour was
// right - a written plan still opens in the viewer.
func TestQuickViewUnchangedForAWrittenPlan(t *testing.T) {
	m := focusOn(newModel(t), authoringFeature(t, "demo", 8))
	got := send(m, runes("v"))
	if got.mode != modeViewer {
		t.Errorf("mode = %v, want modeViewer for a written plan", got.mode)
	}
	if strings.Contains(got.status, "plan.md not written") {
		t.Errorf("a written plan produced the unwritten note: %q", got.status)
	}
}

// firstLines / lastLines trim a rendered board down to the interesting region for failures.
func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

func lastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// --- REV-027: a stale selection must not bypass the move guards ----------------------

// TestStaleSelectionCannotShip pins REV-027. The selection branch in attemptActionForce
// runs BEFORE every guard, and m.selected deliberately survives a reload. Selection-time
// (toggleSelect) and display-time (renderCard's ✓) both filtered on ClassReadyToShip;
// the ACTION path filtered on nothing. So a card selected while ready and then
// reclassified - a UAT round locking it to waiting-for-user is the reachable case -
// stayed selected, vanished from the display because the renderer filtered it, and STILL
// shipped: `m` returned /gogo:done for a card the user could not see was selected,
// straight past the decision-gate guard that runs later in the same function.
//
// The three surfaces now share selectableForShip. This asserts the behaviour, not the
// predicate: reverting any one of the three to its own literal must fail here.
func TestStaleSelectionCannotShip(t *testing.T) {
	ready := &contract.Feature{
		Slug: "shipme", Root: cappedRoot, Class: contract.ClassReadyToShip,
		Phase: "knowledge", Status: "awaiting-uat",
	}
	m := cappedModel(t, nil, ready)
	m.selected = map[string]bool{featureKey(ready): true}

	// Baseline: while genuinely ready, the selection ships.
	if in, isShip, bounce := m.attemptActionForce(false, false); bounce != "" || !isShip ||
		in.Action != launch.ActionDone {
		t.Fatalf("ready selection: action=%v isShip=%v bounce=%q, want a /gogo:done ship",
			in.Action, isShip, bounce)
	}
	if !selectableForShip(ready) {
		t.Fatal("a ready-to-ship card must be selectable")
	}

	// The card is now locked at a decision gate (a UAT re-plan round). The selection is
	// stale: the renderer already hides its ✓, so the user has no way to see it is armed.
	ready.Class = contract.ClassInProgress
	ready.Status = "waiting-for-user"

	if selectableForShip(ready) {
		t.Error("a card at a decision gate must not be selectable for ship")
	}
	if got := m.selectedFeatures(); len(got) != 0 {
		t.Errorf("selectedFeatures = %d stale card(s), want 0 - a reclassified card must "+
			"drop out of the selection at the READ, since selection survives reload", len(got))
	}
	// The whole point: `m` must now fall through to the guards and refuse, not ship.
	in, isShip, bounce := m.attemptActionForce(false, false)
	if bounce == "" {
		t.Errorf("stale selection shipped %v (isShip=%v) with no bounce - it bypassed the "+
			"decision-gate guard entirely", in, isShip)
	}
	// And `M` must not force past it either, for the same reason FR4a's guard cannot be forced.
	if _, _, forced := m.attemptActionForce(false, true); forced == "" {
		t.Error("M forced a stale selection past the decision gate")
	}

	// The RENDERER is the second surface: a stale selection must lose its ✓, so the user
	// is never shown a card as armed that the action path will refuse (and vice versa).
	if card := m.renderCard(0, ready, false, 60); strings.Contains(card, "✓") {
		t.Errorf("renderCard still marks a reclassified card as selected:\n%s", card)
	}

	// The TOGGLE is the third: `space` on a non-ready card must refuse and must NOT arm it,
	// or a stale selection could simply be recreated by hand.
	tm := focusOn(cappedModel(t, nil, ready), ready)
	tm.selected = map[string]bool{}
	tm.toggleSelect()
	if tm.selected[featureKey(ready)] {
		t.Error("space armed a card at a decision gate")
	}
	if tm.status == "" {
		t.Error("space on a non-selectable card refused silently - it must say why")
	}
}

// TestNotRunnableStatusIsRefusedByName pins REV-026. `/gogo:go` only runs a RUNNABLE
// status (plan-accepted / implementing / reviewing / testing). An `awaiting-uat` card
// whose report/ has not landed classifies `unfinished`, so it fell through the
// ClassUnfinished arm - past the accept route, which only matches
// awaiting-plan-acceptance - and returned ActionGo with an EMPTY bounce: `m` silently
// launched a /gogo:go on a card parked at the user's UAT gate.
//
// The guard sits on the GO return, not the branch, because the accept route above it is
// legal for awaiting-plan-acceptance, which is deliberately NOT runnable. Asserted by
// behaviour and by the message naming the legal move (Diagnosability bar).
func TestNotRunnableStatusIsRefusedByName(t *testing.T) {
	// The class matters: classify() derives ClassInProgress from the `phase:` line ALONE,
	// so a LAGGING phase line (this release's own subject) puts a not-runnable status on
	// the ClassInProgress arm. Round 05 guarded only ClassUnfinished, so that combination
	// returned ActionGo with an empty bounce while `gogo go` refused it. Both arms here.
	for _, c := range []struct {
		name, phase, status, want string
		class                     string
	}{
		{"awaiting-uat (unfinished)", "knowledge", "awaiting-uat", "/gogo:done", contract.ClassUnfinished},
		{"shipped (unfinished)", "knowledge", "shipped", "no move", contract.ClassUnfinished},
		{"awaiting-uat on a LAGGING phase line (in-progress)", "review", "awaiting-uat", "/gogo:done", contract.ClassInProgress},
		{"waiting on a lagging phase line (in-progress)", "test", "awaiting-plan-acceptance", "", contract.ClassInProgress},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := &contract.Feature{
				Slug: "uatcard", Root: cappedRoot, Class: c.class,
				Phase: c.phase, Status: c.status,
			}
			m := focusOn(cappedModel(t, nil, f), f)
			in, _, bounce := m.attemptActionForce(false, false)
			if c.want == "" { // awaiting-plan-acceptance routes to ACCEPT, not a bounce
				if in.Action != launch.ActionAccept {
					t.Fatalf("%s: action=%v, want accept - the accept route must survive "+
						"the runnable guard", c.status, in.Action)
				}
				return
			}
			if bounce == "" {
				t.Fatalf("%s returned %v with an EMPTY bounce - a silent launch on a card "+
					"the user is holding", c.status, in.Action)
			}
			if !strings.Contains(bounce, c.want) {
				t.Errorf("bounce %q does not name the legal move (%q)", bounce, c.want)
			}
			// M must not force past it either - not-runnable is legality, not the cap.
			if _, _, forced := m.attemptActionForce(false, true); forced == "" {
				t.Errorf("M forced a launch on a %s card", c.status)
			}
		})
	}
}

// TestStaleSelectionDoesNotResurrect pins REV-033. REV-027 filtered a stale selection at
// the READ, which stopped it shipping - but the entry SURVIVED in m.selected. So a card
// that went ready -> waiting-for-user (✓ correctly vanished) -> ready again came back
// SELECTED, ✓ restored, `m` returning /gogo:done, without the user ever pressing space.
// A selection the user did not make and cannot remember making is the same invisible-arm
// failure REV-027 was about; filtering hid it, pruneSelection (on every rebuild, so every
// reload) removes it.
func TestStaleSelectionDoesNotResurrect(t *testing.T) {
	f := &contract.Feature{
		Slug: "shipme", Root: cappedRoot, Class: contract.ClassReadyToShip,
		Phase: "knowledge", Status: "awaiting-uat",
	}
	m := cappedModel(t, nil, f)
	m.selected = map[string]bool{featureKey(f): true}

	// A UAT round locks the card. rebuild() runs on reload; the entry must be dropped,
	// not merely filtered out of the read.
	f.Class, f.Status = contract.ClassInProgress, "waiting-for-user"
	m.rebuild()
	if m.selected[featureKey(f)] {
		t.Error("a reclassified card is still in m.selected - filtering hides it, but it " +
			"comes back the moment the card is ready again")
	}

	// Back to ready: it must NOT be armed, because the user never re-selected it.
	f.Class, f.Status = contract.ClassReadyToShip, "awaiting-uat"
	m.rebuild()
	if got := m.selectedFeatures(); len(got) != 0 {
		t.Errorf("selection resurrected: %d card(s) armed with no keystroke", len(got))
	}
	if in, isShip, _ := m.attemptActionForce(false, false); isShip &&
		in.Action == launch.ActionDone && len(in.Slugs) > 0 {
		t.Errorf("m shipped %v from a resurrected selection", in.Slugs)
	}
}

// TestDecisionGateBounceNamesTheRightArtifact pins REV-025. `waiting-for-user` covers two
// gates: a plain decision (decisions.md, exits via /gogo:resume) and a UAT re-plan round
// (uat.md, exits ONLY by re-acceptance). Round 06 first branched on a substring match for
// "uat", which was wrong in BOTH directions - an ordinary decision merely MENTIONING uat
// was sent to the wrong file, and the message disagreed with the card's own pill.
// isUATReplan requires a digit after "uat", so bounce and pill agree by construction.
func TestDecisionGateBounceNamesTheRightArtifact(t *testing.T) {
	for _, c := range []struct{ name, openDecision, wantFile, wantNot string }{
		{"a plain decision", "D3 - which shape for the cue", "decisions.md", "uat.md"},
		{"a decision that merely mentions uat", "D4 - uat asked for a different header",
			"decisions.md", "uat.md"},
		{"a real UAT re-plan round", "UAT round 2", "uat.md", "decisions.md"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := &contract.Feature{
				Slug: "demo", Root: cappedRoot, Class: contract.ClassInProgress,
				Phase: "implement", Status: "waiting-for-user", OpenDecision: c.openDecision,
			}
			got := decisionGateBounce(f)
			if !strings.Contains(got, c.wantFile) {
				t.Errorf("bounce %q does not name %s", got, c.wantFile)
			}
			if strings.Contains(got, c.wantNot) {
				t.Errorf("bounce %q names %s - the wrong artifact for this gate", got, c.wantNot)
			}
			// The bounce and the card's own pill must agree about which gate this is.
			if isUATReplan(f) != strings.Contains(got, "uat.md") {
				t.Errorf("bounce disagrees with isUATReplan (pill) for %q", c.openDecision)
			}
		})
	}
}

// TestBounceArmsMatchRunnableHint pins REV-031. The board's notRunnableBounce and the
// CLI's runnableHint answer the same question ("why can't this run?") and had drifted:
// `done` is terminal for the CLI but hit the board's generic arm, and an empty status
// rendered "x is  - not a runnable status", naming nothing at all. Same shape as the
// cap-rule drift (REV-002/012/016) - two prose copies of one rule.
func TestBounceArmsMatchRunnableHint(t *testing.T) {
	for _, st := range []string{"shipped", "done", "aborted", ""} {
		f := &contract.Feature{Slug: "x", Status: st}
		got := notRunnableBounce(f)
		if strings.Contains(got, "  ") {
			t.Errorf("status %q renders a double space (names nothing): %q", st, got)
		}
		switch st {
		case "shipped", "done":
			if !strings.Contains(got, "already shipped") {
				t.Errorf("status %q = %q, want the terminal wording", st, got)
			}
		case "aborted":
			// REV-031: an aborted feature is NOT shipped. Round 06 said it was, by
			// aligning to a runnableHint arm that cmdGo never reaches.
			if strings.Contains(got, "shipped") {
				t.Errorf("aborted = %q - claims the feature shipped, which is false", got)
			}
			if !strings.Contains(got, "aborted") {
				t.Errorf("aborted = %q, want it to name the real status", got)
			}
		case "":
			if !strings.Contains(got, "no status on disk") {
				t.Errorf("empty status = %q, want it to name what is missing", got)
			}
		}
	}
}

// TestFooterChipMatchesWhatMActuallyDoes pins REV-030. The footer must never promise a
// move that bounces, nor refuse one that is legal - advertising an affordance that does
// something else is the same say-one-thing-do-another defect this release is about.
//
// It sweeps the CROSS-PRODUCT of both go-capable classes rather than a hand-listed set:
// the original fix enumerated shapes by hand and missed two (`in-progress` +
// `awaiting-plan-acceptance`, which legally accepts, and `in-progress` + `plan-accepted`
// with an unwritten plan, which bounces), because the two arms were hand-kept copies.
func TestFooterChipMatchesWhatMActuallyDoes(t *testing.T) {
	classes := []string{contract.ClassUnfinished, contract.ClassInProgress}
	statuses := []string{
		"awaiting-plan-acceptance", "plan-accepted", "implementing", "reviewing",
		"testing", "waiting-for-user", "awaiting-uat", "done", "shipped", "aborted", "",
	}
	phases := []string{"plan", "implement", "review", "test", "knowledge"}
	checked := 0
	for _, cl := range classes {
		for _, st := range statuses {
			for _, ph := range phases {
				for _, written := range []bool{true, false} {
					f := authoringFeature(t, "demo", 2)
					if !written {
						f = authoringFeature(t, "demo", 0)
					}
					f.Class, f.Status, f.Phase, f.Root = cl, st, ph, cappedRoot
					if st == "waiting-for-user" {
						f.OpenDecision = "D1 - a fork"
					}
					m := focusOn(cappedModel(t, nil, f), f)
					_, _, bounce := m.attemptActionForce(false, false)
					chip := moveChip(f)
					refuses := strings.Contains(chip, "✗")
					if bounce != "" && !refuses {
						t.Errorf("class=%s status=%q phase=%s written=%v: chip %q promises a "+
							"move but m bounces: %q", cl, st, ph, written, chip, bounce)
					}
					if bounce == "" && refuses {
						t.Errorf("class=%s status=%q phase=%s written=%v: chip %q refuses but "+
							"m is legal", cl, st, ph, written, chip)
					}
					checked++
				}
			}
		}
	}
	if checked < 200 {
		t.Fatalf("swept only %d shapes - the cross-product collapsed, so this assertion is "+
			"weaker than it looks", checked)
	}
}
