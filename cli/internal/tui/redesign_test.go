package tui

import (
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
)

// --- cockpit redesign (1b + 1c): the new pure helpers + board elements ---

// TestPhaseProgressVector pins the single shared phase-progress model (D2): every
// phase before the current is done, the current is current, the rest pending;
// gate/terminal states map meaningfully.
func TestPhaseProgressVector(t *testing.T) {
	d, c, p := phaseDone, phaseCurrent, phasePending
	cases := []struct {
		name string
		f    *contract.Feature
		want [5]phaseState
	}{
		{"review is current", &contract.Feature{Phase: "review", Class: contract.ClassInProgress}, [5]phaseState{d, d, c, p, p}},
		{"plan gate", &contract.Feature{Phase: "plan", Status: "awaiting-plan-acceptance", Class: contract.ClassUnfinished}, [5]phaseState{c, p, p, p, p}},
		{"uat gate (report current)", &contract.Feature{Phase: "knowledge", Status: "awaiting-uat", Class: contract.ClassReadyToShip}, [5]phaseState{d, d, d, d, c}},
		{"shipped is all done", &contract.Feature{Phase: "done", Status: "shipped", Class: contract.ClassShipped}, [5]phaseState{d, d, d, d, d}},
		{"implementing via status", &contract.Feature{Status: "implementing", Class: contract.ClassInProgress}, [5]phaseState{d, c, p, p, p}},
		{"unknown → all pending", &contract.Feature{Class: contract.ClassUnfinished}, [5]phaseState{p, p, p, p, p}},
	}
	for _, tc := range cases {
		if got := phaseProgress(tc.f); got != tc.want {
			t.Errorf("%s: phaseProgress = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestPhaseDotsAndBarPlainText: the FR-4 dots and FR-9 bar render the SAME vector
// and stay substring-assertable (no TTY under go test → lipgloss emits plain text).
func TestPhaseDotsAndBarPlainText(t *testing.T) {
	f := &contract.Feature{Phase: "review", Class: contract.ClassInProgress}
	if dots := phaseDots(f); dots != "①②③④⑤" {
		t.Errorf("phaseDots = %q, want the five glyphs ①②③④⑤", dots)
	}
	if bar := phaseBar(f); !strings.Contains(bar, "▓") || !strings.Contains(bar, "░") {
		t.Errorf("phaseBar should carry filled ▓ (done/current) + faint ░ (pending): %q", bar)
	}
}

// TestPillLabel pins the FR-3 chip text (badge() stays canonical; this transform
// maps the gate states to answer-first wording, passes everything else through).
// "running" is NOT a pill value: a live session is a separate signal, so the pill
// always reads the true status (a phase card stays on its phase).
func TestPillLabel(t *testing.T) {
	cases := []struct {
		f    *contract.Feature
		want string
	}{
		{&contract.Feature{Status: "awaiting-plan-acceptance", Phase: "plan"}, "⏸ accept plan"},
		{&contract.Feature{Status: "awaiting-uat", Phase: "knowledge"}, "⏸ awaiting-uat"},
		{&contract.Feature{Status: "waiting-for-user"}, "⏸ decision"},
		// A UAT re-plan (waiting-for-user carrying a "UAT round N" open-decision)
		// reads as re-planning, not a stuck decision.
		{&contract.Feature{Status: "waiting-for-user", Resume: "plan", OpenDecision: "UAT round 2"}, "⏸ re-planning · UAT 2"},
		{&contract.Feature{Slug: "r", Phase: "review", Status: "reviewing", Iterations: "plan=1 · implement=1 · review=2 · test=0"}, "review r2"},
		{&contract.Feature{Slug: "run", Phase: "implement"}, "implement"},
		// A shipped feature reads "shipped" even with a lingering session (the pill
		// never says "running" — running-vs-status decoupling).
		{&contract.Feature{Slug: "done", Phase: "done", Status: "shipped", Class: contract.ClassShipped}, "shipped"},
	}
	for _, tc := range cases {
		if got := pillLabel(tc.f); got != tc.want {
			t.Errorf("pillLabel(%+v) = %q, want %q", tc.f, got, tc.want)
		}
	}
}

// TestStripeAccentGatesOnly (FR-5): the left accent stripe is purple for the uat
// gate, red for any other gate, and absent for a flowing card.
func TestStripeAccentGatesOnly(t *testing.T) {
	if col, ok := stripeAccent(&contract.Feature{Status: "awaiting-uat"}); !ok || col != uatAccent {
		t.Errorf("uat gate stripe = (%v,%v), want the uat purple", col, ok)
	}
	if col, ok := stripeAccent(&contract.Feature{Status: "awaiting-plan-acceptance"}); !ok || col != waitAccent {
		t.Errorf("plan gate stripe = (%v,%v), want the wait red", col, ok)
	}
	if _, ok := stripeAccent(&contract.Feature{Status: "implementing", Phase: "implement"}); ok {
		t.Errorf("a flowing card must carry no stripe")
	}
}

// TestGateCardStripeGlyph (FR-5): a gate card renders the heavy ┃ left-stripe
// glyph independent of focus; a flowing card keeps the plain │ border (no ┃).
func TestGateCardStripeGlyph(t *testing.T) {
	m := newModel(t)
	uat := &contract.Feature{Slug: "u", Title: "U", Status: "awaiting-uat", Phase: "knowledge", Class: contract.ClassReadyToShip}
	if out := m.renderCard(2, uat, false, 40); !strings.Contains(out, gateStripe) {
		t.Errorf("uat gate card missing the ┃ stripe:\n%s", out)
	}
	if out := m.renderCard(2, uat, true, 40); !strings.Contains(out, gateStripe) {
		t.Errorf("focused uat gate card lost the ┃ stripe (must be focus-independent):\n%s", out)
	}
	flow := &contract.Feature{Slug: "f", Title: "F", Status: "implementing", Phase: "implement", Class: contract.ClassInProgress}
	if out := m.renderCard(1, flow, false, 40); strings.Contains(out, gateStripe) {
		t.Errorf("flowing card wrongly shows the ┃ stripe:\n%s", out)
	}
}

// TestGatesEnumeration (FR-8): gates() surfaces the board's WaitingForInput()
// cards. The fixture's sole gate is the awaiting-uat "ready" feature.
func TestGatesEnumeration(t *testing.T) {
	m := newModel(t)
	gs := m.gates()
	if len(gs) != 1 {
		t.Fatalf("gates() = %d, want 1 (the awaiting-uat 'ready' feature)", len(gs))
	}
	if gs[0].kind != "uat gate" || gs[0].feature.Slug != "ready" {
		t.Errorf("gate[0] = {%s, %s}, want {uat gate, ready}", gs[0].kind, gs[0].feature.Slug)
	}
}

// TestNeedsYouStripRender (FR-8): the strip surfaces the gate as a titled,
// answer-first inbox row above the board.
func TestNeedsYouStripRender(t *testing.T) {
	m := newModel(t)
	out := m.View()
	for _, want := range []string{"⏸ NEEDS YOU (1)", "uat gate", "ready", "[1] read report · [d] ship"} {
		if !strings.Contains(out, want) {
			t.Errorf("needs-you strip missing %q:\n%s", want, out)
		}
	}
}

// TestHeaderAttentionSummary (FR-1): the header shows the red need-you pill and,
// when a session is live, the green session count.
func TestHeaderAttentionSummary(t *testing.T) {
	m := newModel(t)
	if out := m.View(); !strings.Contains(out, "⏸ 1 need you") {
		t.Errorf("header missing the '⏸ 1 need you' pill:\n%s", out)
	}
	m.colIdx = 1
	f := m.focusedCard()
	m.sessions = []string{"gogo-go-" + f.Slug}
	if out := m.View(); !strings.Contains(out, "● 1 session") {
		t.Errorf("header missing the '● 1 session' count:\n%s", out)
	}
}

// TestContextualFooterChips (FR-7): the footer shows the focused card's action
// key-chips + a right-aligned [?] all keys.
func TestContextualFooterChips(t *testing.T) {
	m := newModel(t)
	m.colIdx = 2 // the ready column → [d] ship is the legal move
	out := m.View()
	for _, want := range []string{"[d] ship", "[enter] drill", "[?] all keys"} {
		if !strings.Contains(out, want) {
			t.Errorf("contextual footer missing %q:\n%s", want, out)
		}
	}
}

// TestAllKeysToggle (FR-10): ? swaps the contextual footer for the full key list,
// and ? again hides it.
func TestAllKeysToggle(t *testing.T) {
	m := newModel(t)
	if strings.Contains(m.View(), "space select") {
		t.Fatalf("full key list shown before any ?:\n%s", m.View())
	}
	m = send(m, runes("?"))
	if !strings.Contains(m.View(), "space select") {
		t.Errorf("? did not reveal the full key list:\n%s", m.View())
	}
	m = send(m, runes("?"))
	if strings.Contains(m.View(), "space select") {
		t.Errorf("a second ? did not hide the full key list")
	}
}

// TestNumberKeyReadsGate (FR-10): pressing 1 answers gate 1 — it focuses that
// gate's card AND opens its primary view ("read report" for the uat gate).
func TestNumberKeyReadsGate(t *testing.T) {
	m := newModel(t)
	m = keyPress(t, m, runes("1"))
	if f := m.focusedCard(); f == nil || f.Slug != "ready" {
		t.Fatalf("pressing 1 did not focus gate 1 (ready): %v", f)
	}
	if m.mode != modeViewer {
		t.Fatalf("pressing 1 did not open the gate's primary view (mode=%d)", m.mode)
	}
	if !strings.HasSuffix(m.viewerTitle, "report.md") {
		t.Errorf("gate 1 opened %q, want the report", m.viewerTitle)
	}
}

// TestNumberKeyOutOfRange (FR-10): a number with no matching gate is a status
// hint, not a jump or a crash.
func TestNumberKeyOutOfRange(t *testing.T) {
	m := newModel(t)
	nm, cmd := m.Update(runes("5")) // only 1 gate in the fixture
	m = nm.(Model)
	if cmd != nil {
		t.Errorf("out-of-range gate number returned a command")
	}
	if m.mode != modeBoard || !strings.Contains(m.status, "no gate 5") {
		t.Errorf("out-of-range gate number: mode=%d status=%q", m.mode, m.status)
	}
}

// sanity: the digit parser is exact (1..9 only).
func TestGateNumberKeyParse(t *testing.T) {
	for _, s := range []string{"1", "9"} {
		if _, ok := gateNumberKey(s); !ok {
			t.Errorf("gateNumberKey(%q) should parse", s)
		}
	}
	for _, s := range []string{"0", "a", "12", "", "enter"} {
		if _, ok := gateNumberKey(s); ok {
			t.Errorf("gateNumberKey(%q) should NOT parse", s)
		}
	}
}

// --- running-vs-status decoupling + UAT re-plan label (both issues) ---

// TestShippedNeverRunning (Issue 2): a shipped feature whose just-finished
// gogo-done-<slug> host still lingers (the documented self-reap limitation) reads
// its true "shipped" status — the lingering session is a SEPARATE signal, never a
// "running" that masquerades as a status.
func TestShippedNeverRunning(t *testing.T) {
	f := &contract.Feature{Slug: "cockpit", Phase: "done", Status: "shipped", Class: contract.ClassShipped}
	sessions := []string{"gogo-done-cockpit"}
	if !hasLiveSession(f.Slug, sessions) {
		t.Fatal("precondition: the lingering done session should match the slug")
	}
	if got := badge(f); got != "shipped" {
		t.Errorf("badge = %q, want shipped (a lingering session is not a status)", got)
	}
	if got := pillLabel(f); got != "shipped" {
		t.Errorf("pillLabel = %q, want shipped", got)
	}
	if f.WaitingForInput() {
		t.Error("a shipped feature is terminal — never a needs-you gate")
	}
}

// TestRunningIsNotAStatus proves the decoupling end to end at the card layer: an
// in-progress card with a LIVE session renders its true phase pill (never
// "running") AND the separate ● liveness dot.
func TestRunningIsNotAStatus(t *testing.T) {
	m := newModel(t)
	m.sessions = []string{"gogo-go-build"}
	f := &contract.Feature{
		Slug: "build", Title: "Building it", Phase: "implement", Status: "implementing",
		Iterations: "plan=1 · implement=2 · review=0 · test=0", Class: contract.ClassInProgress,
	}
	out := m.renderCard(1, f, false, 40)
	if strings.Contains(out, "running") {
		t.Errorf("card must not show a 'running' status:\n%s", out)
	}
	if !strings.Contains(out, "implement r2") {
		t.Errorf("card lost its true phase pill:\n%s", out)
	}
	if !strings.Contains(out, "●") {
		t.Errorf("live card lost the separate ● session dot:\n%s", out)
	}
}

// TestUATReplanGate (Issue 1): a mid-UAT re-plan surfaces as a "uat re-plan" gate
// with re-planning wording, distinct from a generic decision fork.
func TestUATReplanGate(t *testing.T) {
	f := &contract.Feature{Slug: "redo", Status: "waiting-for-user", Resume: "plan", OpenDecision: "UAT round 2"}
	g := gateFor(f)
	if g.kind != "uat re-plan" {
		t.Errorf("gate kind = %q, want uat re-plan", g.kind)
	}
	if !strings.Contains(g.blocked, "re-planning after UAT round 2") {
		t.Errorf("gate blocked = %q, want the re-planning blurb", g.blocked)
	}
	d := gateFor(&contract.Feature{Slug: "fork", Status: "waiting-for-user", Resume: "review", OpenDecision: "D3"})
	if d.kind != "decision gate" {
		t.Errorf("generic gate kind = %q, want decision gate", d.kind)
	}
}

// TestUATRound pins the open-decision round parser (0 for a non-UAT decision).
func TestUATRound(t *testing.T) {
	cases := []struct {
		od   string
		want int
	}{
		{"UAT round 2", 2}, {"UAT round 10", 10}, {"uat round 1", 1},
		{"D3", 0}, {"", 0}, {"UAT round", 0},
	}
	for _, c := range cases {
		if got := uatRound(&contract.Feature{OpenDecision: c.od}); got != c.want {
			t.Errorf("uatRound(%q) = %d, want %d", c.od, got, c.want)
		}
	}
}
