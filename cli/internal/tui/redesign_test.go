package tui

import (
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
)

// --- cockpit redesign (1b + 1c): the new pure helpers + board elements ---

// TestActiveAgent pins the FR-6 phase→agent map: knowledge/report both mean the
// report step (reporter); done/unknown yield no label; and a blank phase falls
// back to the status so a live card whose telemetry lags still names its agent.
func TestActiveAgent(t *testing.T) {
	cases := []struct {
		f    *contract.Feature
		want string
	}{
		{&contract.Feature{Phase: "plan"}, "analyst"},
		{&contract.Feature{Phase: "implement"}, "developer"},
		{&contract.Feature{Phase: "review"}, "reviewer"},
		{&contract.Feature{Phase: "test"}, "tester"},
		{&contract.Feature{Phase: "knowledge"}, "reporter"},
		{&contract.Feature{Phase: "report"}, "reporter"},
		{&contract.Feature{Phase: "done"}, ""},
		{&contract.Feature{Phase: ""}, ""},
		{&contract.Feature{Phase: "bogus"}, ""},
		// Phase absent → derive from status (the telemetry-lag fallback).
		{&contract.Feature{Phase: "", Status: "implementing"}, "developer"},
		{&contract.Feature{Phase: "", Status: "reviewing"}, "reviewer"},
		{&contract.Feature{Phase: "", Status: "testing"}, "tester"},
	}
	for _, tc := range cases {
		if got := activeAgent(tc.f); got != tc.want {
			t.Errorf("activeAgent(phase=%q status=%q) = %q, want %q", tc.f.Phase, tc.f.Status, got, tc.want)
		}
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

// TestChangelogFocusCursor: the collapsed changelog list carries an in-list focus
// indicator (▸ cursor + selection bar) on the focused row — but only when the
// changelog column itself holds board focus, and only on that one row.
func TestChangelogFocusCursor(t *testing.T) {
	m := newModel(t)
	a := &contract.Feature{Slug: "alpha", Class: contract.ClassShipped, Completed: "2026-07-01"}
	b := &contract.Feature{Slug: "bravo", Class: contract.ClassShipped, Completed: "2026-07-02"}
	m.cols[3] = []*contract.Feature{a, b}

	m.colIdx = 3
	m.cardIdx[3] = 1
	out := m.renderColumn(3, m.boardColWidth())
	if !strings.Contains(out, "▸ ✓ bravo") {
		t.Errorf("focused changelog row missing the ▸ cursor:\n%s", out)
	}
	if strings.Contains(out, "▸ ✓ alpha") {
		t.Errorf("non-focused changelog row wrongly shows the cursor:\n%s", out)
	}

	// Focus a different column → no changelog row shows the cursor.
	m.colIdx = 0
	if out := m.renderColumn(3, m.boardColWidth()); strings.Contains(out, "▸ ✓") {
		t.Errorf("changelog rows show a cursor while another column is focused:\n%s", out)
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

// --- lean cards: the agent chip + the removed strip / phase dots ---

// TestAgentChipOnlyWhenLive (FR-6, D1): a live in-progress card renders the green
// `● <agent>` chip; the same card with no live session, and a live *gate* card
// (WaitingForInput), render NO chip — the chip is a "who's on it now" signal.
func TestAgentChipOnlyWhenLive(t *testing.T) {
	m := newModel(t)
	f := &contract.Feature{
		Slug: "build", Title: "Building it", Phase: "implement", Status: "implementing",
		Class: contract.ClassInProgress,
	}

	// Live session on an in-progress card → the chip names its agent.
	m.sessions = []string{"gogo-go-build"}
	if out := m.renderCard(1, f, false, 40); !strings.Contains(out, "● developer") {
		t.Errorf("live in-progress card missing the ● developer chip:\n%s", out)
	}
	// Focused (plain-fill) render still carries the chip.
	if out := m.renderCard(1, f, true, 40); !strings.Contains(out, "● developer") {
		t.Errorf("focused live card missing the ● developer chip:\n%s", out)
	}

	// No live session → no chip.
	m.sessions = nil
	if out := m.renderCard(1, f, false, 40); strings.Contains(out, "developer") {
		t.Errorf("idle card wrongly shows the agent chip:\n%s", out)
	}

	// A live GATE card (WaitingForInput) shows no chip — nobody is working a card
	// that is parked on the user.
	gate := &contract.Feature{Slug: "gate", Title: "G", Phase: "plan", Status: "awaiting-plan-acceptance", Class: contract.ClassUnfinished}
	m.sessions = []string{"gogo-go-gate"}
	if out := m.renderCard(0, gate, false, 40); strings.Contains(out, "analyst") {
		t.Errorf("a live gate card wrongly shows the agent chip:\n%s", out)
	}
}

// TestNoPhaseDots: no rendered card carries the removed phase-dot glyphs ①②③④⑤.
func TestNoPhaseDots(t *testing.T) {
	m := newModel(t)
	f := &contract.Feature{Slug: "x", Title: "X", Phase: "review", Status: "reviewing", Class: contract.ClassInProgress}
	for _, focused := range []bool{false, true} {
		out := m.renderCard(1, f, focused, 40)
		for _, dot := range []string{"①", "②", "③", "④", "⑤"} {
			if strings.Contains(out, dot) {
				t.Errorf("card (focused=%v) still shows a phase dot %q:\n%s", focused, dot, out)
			}
		}
	}
}

// TestNoNeedsYouStrip: the board view no longer renders the NEEDS YOU inbox box;
// the gate count survives only as the header pill (⏸ K need you).
func TestNoNeedsYouStrip(t *testing.T) {
	m := newModel(t)
	out := m.View()
	if strings.Contains(out, "NEEDS YOU") {
		t.Errorf("board still renders the needs-you strip:\n%s", out)
	}
	if !strings.Contains(out, "⏸ 1 need you") {
		t.Errorf("header lost the need-you count pill:\n%s", out)
	}
}
