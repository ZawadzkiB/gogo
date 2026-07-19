package tui

import (
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	tea "github.com/charmbracelet/bubbletea"
)

// TestMatchFilterCorrelationTokenAndsWithProject (FR14): the `#plan-XXXX`
// correlation token ANDs with the `@project` (source) token on the aggregate board —
// a member in the matching source passes, a member in a different source is excluded.
func TestMatchFilterCorrelationTokenAndsWithProject(t *testing.T) {
	known := map[string]bool{"plan-7f3a": true}
	memberWeb := &contract.Feature{Slug: "login", Title: "Login", Source: "web", Correlations: []string{"plan-7f3a"}}
	memberApi := &contract.Feature{Slug: "token", Title: "Token", Source: "api", Correlations: []string{"plan-7f3a"}}

	if !matchFilter(memberWeb, "#plan-7f3a @web", true, known) {
		t.Error("member in the matching source should pass #plan AND @source")
	}
	if matchFilter(memberApi, "#plan-7f3a @web", true, known) {
		t.Error("correlation member in a DIFFERENT source must be excluded by the @source AND")
	}
}

// TestMatchFilterUnknownCorrelationTokenIsLiteral (FR14 parity): an unknown `#token`
// (no such correlation on the board) degrades to a literal text match — it never
// over-hides a board that has no correlations.
func TestMatchFilterUnknownCorrelationTokenIsLiteral(t *testing.T) {
	f := &contract.Feature{Slug: "hash-parser", Title: "parse #tags"}
	// No known correlations → the token is matched literally over slug+title.
	if !matchFilter(f, "#tags", false, nil) {
		t.Error("unknown #token should match literally (title contains '#tags')")
	}
	if matchFilter(f, "#nope", false, nil) {
		t.Error("unknown #token with no literal hit should not match")
	}
}

// TestCardRendersTwoCorrelationChips (FR14): a work item stamped with TWO plan
// correlations renders BOTH ⛓ plan-… chips on its card (the plural, state.md-sourced
// overlay).
func TestCardRendersTwoCorrelationChips(t *testing.T) {
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "checkout", Title: "Checkout", Root: "/repos/web", Source: "web",
			Class: contract.ClassUnfinished, Status: "plan-accepted",
			Correlations: []string{"plan-aaa11111", "plan-bbb22222"}},
	}}
	m := NewWorkspace(repo, proj("web", src("web", "/repos/web")))
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 400, Height: 40})
	m = nm.(Model)

	out := m.View()
	for _, want := range []string{"⛓ plan-aaa11111", "⛓ plan-bbb22222"} {
		if !strings.Contains(out, want) {
			t.Errorf("card in two plans did not render chip %q:\n%s", want, out)
		}
	}
}

// TestCardNarrowCorrelationChipsCountFallback (TEST-002): when the full ⛓ plan-<id>
// chips don't fit a narrow card, the card renders a compact `⛓ ×N` count fallback
// (still saying "belongs to N plans") rather than an indistinguishable truncated id.
func TestCardNarrowCorrelationChipsCountFallback(t *testing.T) {
	m := sizedWorkspace(t, &contract.Repo{}, proj("app", src("web", "/r/web")))
	f := &contract.Feature{Slug: "checkout", Title: "Checkout", Source: "web",
		Class: contract.ClassUnfinished, Status: "plan-accepted",
		Correlations: []string{"plan-aaa11111", "plan-bbb22222"}}

	const narrow = 26 // too tight for two full `⛓ plan-<hex8>` chips
	out := m.renderCard(0, f, false, narrow)
	if !strings.Contains(out, "⛓ ×2") {
		t.Errorf("narrow card should show the `⛓ ×2` count fallback, not a truncated id:\n%s", out)
	}
	// The individual ids must NOT survive at this width (they'd be indistinguishable).
	for _, id := range []string{"plan-aaa11111", "plan-bbb22222"} {
		if strings.Contains(out, id) {
			t.Errorf("narrow card unexpectedly rendered full id %q:\n%s", id, out)
		}
	}

	// At a comfortable width both full ids render (no fallback).
	wide := m.renderCard(0, f, false, 400)
	for _, want := range []string{"⛓ plan-aaa11111", "⛓ plan-bbb22222"} {
		if !strings.Contains(wide, want) {
			t.Errorf("wide card should render full chip %q:\n%s", want, wide)
		}
	}
	if strings.Contains(wide, "×2") {
		t.Errorf("wide card should NOT use the count fallback:\n%s", wide)
	}
}

// TestCorrelationFilterNarrowsBoard (FR14): a `#plan-XXXX` filter narrows the board
// to that plan's members across sources; a non-member is hidden.
func TestCorrelationFilterNarrowsBoard(t *testing.T) {
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "a", Title: "A", Source: "web", Root: "/r/web", Class: contract.ClassUnfinished, Status: "plan-accepted", Correlations: []string{"plan-shared01"}},
		{Slug: "b", Title: "B", Source: "api", Root: "/r/api", Class: contract.ClassUnfinished, Status: "plan-accepted", Correlations: []string{"plan-shared01"}},
		{Slug: "c", Title: "C", Source: "web", Root: "/r/web", Class: contract.ClassUnfinished, Status: "plan-accepted"},
	}}
	m := sizedWorkspace(t, repo, proj("app", src("web", "/r/web"), src("api", "/r/api")))

	m.filter = "#plan-shared01"
	m.rebuild()
	if len(m.cols[0]) != 2 {
		t.Fatalf("#plan filter left %d plan cards, want 2 (the two members)", len(m.cols[0]))
	}
	slugs := map[string]bool{m.cols[0][0].Slug: true, m.cols[0][1].Slug: true}
	if !slugs["a"] || !slugs["b"] || slugs["c"] {
		t.Errorf("filtered slugs = %v, want the two correlation members a+b (not c)", slugs)
	}
}
