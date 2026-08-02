package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/plans"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// card-selection-border: focus/selection choose only the card FRAME — the body is
// ONE styled render for every state. These guards pin FR1-FR7 of that plan.

// forceColors forces a real colour profile for the duration of the test (go test
// has no TTY, so lipgloss would otherwise strip every colour and an "identical
// bodies" assertion would be vacuous). Restored via t.Cleanup.
func forceColors(t *testing.T) {
	t.Helper()
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(old) })
}

// borderRunes are the three side-border glyphs a card line can carry (rounded,
// double, gate stripe). All three encode to 3 bytes in UTF-8.
const borderRunes = "│║┃"

// edgeAnsi trims ANSI SGR fragments left at the cut edges — the border glyph's
// own reset/colour sequences, which legitimately differ between frame styles.
var edgeAnsi = regexp.MustCompile(`^(?:\x1b\[[0-9;]*m)+|(?:\x1b\[[0-9;]*m)+$`)

// cardInnerLines cuts a rendered card down to its per-line INNER content: the
// top/bottom border rows dropped, each middle line cut between its side-border
// glyphs, edge ANSI (the border's own styling) trimmed. What remains is exactly
// the body the frame wraps — the thing FR1 says must be focus-independent.
func cardInnerLines(t *testing.T, card string) []string {
	t.Helper()
	lines := strings.Split(card, "\n")
	if len(lines) < 3 {
		t.Fatalf("card render has %d lines, want >= 3:\n%s", len(lines), card)
	}
	var out []string
	for _, ln := range lines[1 : len(lines)-1] {
		first := strings.IndexAny(ln, borderRunes)
		last := strings.LastIndexAny(ln, borderRunes)
		if first < 0 || last <= first {
			t.Fatalf("card line carries no side borders: %q", ln)
		}
		out = append(out, edgeAnsi.ReplaceAllString(ln[first+3:last], ""))
	}
	return out
}

// colorfulFeature is a fixture whose card carries every styled span at once:
// status pill, live session ● dot, correlation chip.
func colorfulFeature() *contract.Feature {
	return &contract.Feature{
		Slug: "inprogress", Title: "In progress", Status: "implementing",
		Phase: "implement", Class: contract.ClassInProgress,
		Correlations: []string{"plan-7f3a"},
	}
}

// TestFocusedCardKeepsInnerColors (FR1): with a real colour profile forced, the
// focused card's inner content is byte-identical to the unfocused one — pill
// tint, session dot, correlation chip and slug styling all survive focus.
func TestFocusedCardKeepsInnerColors(t *testing.T) {
	forceColors(t)
	m := newModel(t)
	m.sessions = []string{"gogo-go-inprogress"}
	f := colorfulFeature()

	unfocused := cardInnerLines(t, m.renderCard(1, f, false, 44))
	focused := cardInnerLines(t, m.renderCard(1, f, true, 44))

	// Anti-vacuity: the unfocused body must actually carry ANSI colour, or the
	// equality below would prove nothing.
	if !strings.Contains(strings.Join(unfocused, "\n"), "\x1b[") {
		t.Fatalf("unfocused body carries no ANSI at ANSI256 — vacuous comparison:\n%q", unfocused)
	}
	if len(focused) != len(unfocused) {
		t.Fatalf("focused body has %d lines, unfocused %d", len(focused), len(unfocused))
	}
	for i := range focused {
		if focused[i] != unfocused[i] {
			t.Errorf("line %d differs under focus:\nfocused:   %q\nunfocused: %q", i, focused[i], unfocused[i])
		}
	}
}

// TestFocusedCardFrameCarriesNoFill (FR1 invariant): no card frame style carries
// a fg/bg fill — asserted through the lipgloss API so it cannot regress quietly.
func TestFocusedCardFrameCarriesNoFill(t *testing.T) {
	for i := 0; i < 4; i++ {
		for name, st := range map[string]lipgloss.Style{
			"cardFocused":         columnStyles[i].cardFocused,
			"cardFocusedSelected": columnStyles[i].cardFocusedSelected,
			"cardSelected":        columnStyles[i].cardSelected,
			"card":                columnStyles[i].card,
		} {
			if bg := st.GetBackground(); bg != (lipgloss.NoColor{}) {
				t.Errorf("columnStyles[%d].%s carries a background fill: %v", i, name, bg)
			}
			if fg := st.GetForeground(); fg != (lipgloss.NoColor{}) {
				t.Errorf("columnStyles[%d].%s carries a foreground override: %v", i, name, fg)
			}
		}
	}
}

// TestFocusedCardMarkedByBorderGlyph (FR2): under the default colourless `go
// test` render, the focused card is still identifiable — by its double-border
// glyphs — and an unfocused card keeps the rounded set.
func TestFocusedCardMarkedByBorderGlyph(t *testing.T) {
	m := newModel(t)
	f := colorfulFeature()
	focused := m.renderCard(1, f, true, 40)
	unfocused := m.renderCard(1, f, false, 40)
	for _, glyph := range []string{"║", "═", "╔"} {
		if !strings.Contains(focused, glyph) {
			t.Errorf("focused card missing the double-border glyph %q:\n%s", glyph, focused)
		}
	}
	if strings.Contains(unfocused, "║") || strings.Contains(unfocused, "═") {
		t.Errorf("unfocused card wrongly carries double-border glyphs:\n%s", unfocused)
	}
	if !strings.Contains(unfocused, "│") {
		t.Errorf("unfocused card lost its rounded border:\n%s", unfocused)
	}
}

// TestSelectedAndFocusedBothVisible (FR3): a space-selected card keeps its ✓
// mark when the cursor arrives, gains the double focus frame, and renderCard
// actually PICKS the select-accent frame for the both-states card (today focus
// swallowed the selection colour). The frame-pick assertion compares rendered
// top borders under a forced colour profile, so dropping the focused&&selected
// arm (falling back to cardFocused) fails here — not just the style value
// (REV-001: the earlier form survived exactly that mutation).
func TestSelectedAndFocusedBothVisible(t *testing.T) {
	forceColors(t)
	m := newModel(t)
	f := &contract.Feature{Slug: "ready", Title: "R", Status: "awaiting-uat", Phase: "done", Class: contract.ClassReadyToShip}
	if m.selected == nil {
		m.selected = map[string]bool{}
	}
	m.selected[featureKey(f)] = true

	out := m.renderCard(2, f, true, 40)
	if !strings.Contains(out, "✓") {
		t.Errorf("focused+selected card lost its ✓ mark:\n%s", out)
	}
	if !strings.Contains(out, "║") {
		t.Errorf("focused+selected card missing the focus border:\n%s", out)
	}
	// The frame pick: the both-states top border (select accent) must differ from
	// the focused-only top border (column accent) — content-free lines, so the
	// only difference is the frame colour renderCard chose. Proven on column 0
	// (blue): the ready column's OWN accent equals selectAccent by palette design
	// (#5db97a), so on column 2 the two frames legitimately match.
	topBoth := strings.SplitN(m.renderCard(0, f, true, 40), "\n", 2)[0]
	m.selected = map[string]bool{}
	topFocused := strings.SplitN(m.renderCard(0, f, true, 40), "\n", 2)[0]
	if topBoth == topFocused {
		t.Errorf("focused+selected card renders the focused-only frame — the select accent was swallowed:\n%q", topBoth)
	}
	for i := 0; i < 4; i++ {
		if got := columnStyles[i].cardFocusedSelected.GetBorderTopForeground(); got != lipgloss.TerminalColor(selectAccent) {
			t.Errorf("columnStyles[%d].cardFocusedSelected border colour = %v, want the select accent", i, got)
		}
	}
}

// TestFocusedGateCardKeepsStripeAndFocusBorder (FR4): the ┃ gate stripe is
// composed OVER the focus border — a focused gate card carries both; a focused
// flowing card carries the focus glyphs and no ┃.
func TestFocusedGateCardKeepsStripeAndFocusBorder(t *testing.T) {
	m := newModel(t)
	uat := &contract.Feature{Slug: "u", Title: "U", Status: "awaiting-uat", Phase: "knowledge", Class: contract.ClassReadyToShip}
	out := m.renderCard(2, uat, true, 40)
	if !strings.Contains(out, gateStripe) {
		t.Errorf("focused gate card lost the ┃ stripe:\n%s", out)
	}
	if !strings.Contains(out, "═") {
		t.Errorf("focused gate card lost the focus border glyphs:\n%s", out)
	}
	flow := &contract.Feature{Slug: "f", Title: "F", Status: "implementing", Phase: "implement", Class: contract.ClassInProgress}
	if out := m.renderCard(1, flow, true, 40); strings.Contains(out, gateStripe) {
		t.Errorf("focused flowing card wrongly shows the ┃ stripe:\n%s", out)
	}
}

// TestCardHeightIsFocusIndependent (FR7): rendering any card focused vs
// unfocused never changes its height — the column windowing (cardHeights /
// reflowColumns) depends on it.
func TestCardHeightIsFocusIndependent(t *testing.T) {
	m := newModel(t)
	m.sessions = []string{"gogo-go-inprogress"}
	fixtures := []*contract.Feature{
		colorfulFeature(),
		{Slug: "u", Title: "Gate", Status: "awaiting-uat", Phase: "knowledge", Class: contract.ClassReadyToShip},
		{Slug: "long", Title: "A very long title that will surely truncate somewhere", Status: "plan-accepted", Phase: "plan", Class: contract.ClassUnfinished, Correlations: []string{"plan-7f3a", "plan-9c2e"}},
	}
	for _, f := range fixtures {
		for _, w := range []int{24, 40, 60} {
			hf := lipgloss.Height(m.renderCard(1, f, true, w))
			hu := lipgloss.Height(m.renderCard(1, f, false, w))
			if hf != hu {
				t.Errorf("%s @%d: focused height %d != unfocused %d", f.Slug, w, hf, hu)
			}
		}
	}
}

// TestFocusedPlanCardKeepsInnerColors (FR5): the plans-tab kanban card mirrors
// FR1 — one styled body, focus changes only the frame.
func TestFocusedPlanCardKeepsInnerColors(t *testing.T) {
	forceColors(t)
	m := newModel(t)
	p := plans.Plan{ID: "plan-x", Title: "Rollout", Status: plans.StatusActive, Targets: []string{"web", "api"}}

	unfocused := cardInnerLines(t, m.renderPlanCard(2, p, false, 44))
	focused := cardInnerLines(t, m.renderPlanCard(2, p, true, 44))
	if !strings.Contains(strings.Join(unfocused, "\n"), "\x1b[") {
		t.Fatalf("unfocused plan-card body carries no ANSI at ANSI256 — vacuous comparison:\n%q", unfocused)
	}
	if len(focused) != len(unfocused) {
		t.Fatalf("focused plan-card body has %d lines, unfocused %d", len(focused), len(unfocused))
	}
	for i := range focused {
		if focused[i] != unfocused[i] {
			t.Errorf("plan-card line %d differs under focus:\nfocused:   %q\nunfocused: %q", i, focused[i], unfocused[i])
		}
	}
	// And the focus FRAME must actually be applied (REV-002: the bodies are
	// focus-independent by construction now, so without this glyph check a
	// renderPlanCard that ignores `focused` entirely would still pass).
	if raw := m.renderPlanCard(2, p, true, 44); !strings.Contains(raw, "║") || !strings.Contains(raw, "═") {
		t.Errorf("focused plan card missing the double focus border:\n%s", raw)
	}
	if raw := m.renderPlanCard(2, p, false, 44); strings.Contains(raw, "║") {
		t.Errorf("unfocused plan card wrongly carries the focus border:\n%s", raw)
	}
}

// TestFocusedConfigRowsKeepColors (FR6, config-tab half — REV-003): the focused
// project and source rows keep their individually styled origin dots (no row
// fill), marked by the ▸ cursor. A restored fill would style the whole row in
// one sequence, so the dot would no longer carry its OWN immediately-preceding
// SGR — which is exactly what the regexp requires.
func TestFocusedConfigRowsKeepColors(t *testing.T) {
	forceColors(t)
	seedDataHome(t)
	repo := gogoRepoDir(t)
	p := projects.Project{Name: "app", Sources: []projects.Source{{Name: "svc", Path: repo}}}
	if err := projects.Save(&p); err != nil {
		t.Fatal(err)
	}
	m := configTab(sizedWorkspace(t, &contract.Repo{}, p))
	out := m.viewConfigLeft()
	styledDot := regexp.MustCompile(`\x1b\[[0-9;]*m●`)
	bgFill := regexp.MustCompile(`\x1b\[[0-9;]*48;`)
	rows := 0
	for _, ln := range strings.Split(out, "\n") {
		if !strings.Contains(ln, "▸") {
			continue
		}
		rows++
		if !styledDot.MatchString(ln) {
			t.Errorf("focused config row lost its styled origin dot (fill regression?):\n%q", ln)
		}
		// A fill-only regression (row background restored, dots still tinted) would
		// slip the dot check — no focused row may carry a background SGR at all.
		if bgFill.MatchString(ln) {
			t.Errorf("focused config row carries a background fill:\n%q", ln)
		}
	}
	if rows == 0 {
		t.Fatalf("no focused (▸) rows found in the config tab render:\n%s", out)
	}
}

// TestFocusedChangelogRowKeepsColors (FR6): a focused changelog row keeps its
// origin/session dot colours (no fill) and is marked by the ▸ cursor.
func TestFocusedChangelogRowKeepsColors(t *testing.T) {
	forceColors(t)
	m := newModel(t)
	f := &contract.Feature{Slug: "shipped-x", Source: "web", Project: "app", Status: "shipped", Completed: "2026-08-01", Class: contract.ClassShipped}

	focused := m.changelogRow(f, 40, true, true)
	unfocused := m.changelogRow(f, 40, false, true)
	if !strings.Contains(focused, "▸") {
		t.Errorf("focused changelog row missing the ▸ cursor:\n%q", focused)
	}
	if !strings.Contains(focused, "\x1b[") {
		t.Errorf("focused changelog row carries no colour (fill regression?):\n%q", focused)
	}
	// The origin/session dots must render with the SAME styling focused and not:
	// strip the cursor gutter + slug styling difference by asserting every ANSI-
	// styled dot sequence of the unfocused row also appears in the focused one.
	for _, frag := range regexp.MustCompile(`\x1b\[[0-9;]*m●\x1b\[[0-9;]*m`).FindAllString(unfocused, -1) {
		if !strings.Contains(focused, frag) {
			t.Errorf("focused changelog row lost a styled dot %q:\nfocused: %q", frag, focused)
		}
	}
}
