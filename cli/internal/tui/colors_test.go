package tui

import (
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	"github.com/charmbracelet/lipgloss"
)

// TestColorForNeverBlank (cockpit-colors D2=A): colorFor resolves a swatch hex
// adaptively, an arbitrary hex directly, and a blank via the never-blank ColorForIndex
// fallback (also adaptive) — it never returns a nil/zero color.
func TestColorForNeverBlank(t *testing.T) {
	if colorFor("", 0) == nil {
		t.Fatal("colorFor(\"\", …) returned nil — must never be blank")
	}
	if _, ok := colorFor("", 3).(lipgloss.AdaptiveColor); !ok {
		t.Errorf("colorFor(blank) = %T, want an AdaptiveColor (swatch fallback)", colorFor("", 3))
	}
	if _, ok := colorFor("#58a6ff", 0).(lipgloss.AdaptiveColor); !ok {
		t.Errorf("colorFor(swatch dark) = %T, want an AdaptiveColor", colorFor("#58a6ff", 0))
	}
	got := colorFor("#abcdef", 0)
	if c, ok := got.(lipgloss.Color); !ok || string(c) != "#abcdef" {
		t.Errorf("colorFor(arbitrary hex) = %#v, want a direct lipgloss.Color(#abcdef)", got)
	}
}

// TestSourceColorResolvesEverySource (cockpit-colors FR2): a colorless source still maps
// to a stable, distinct, palette-backed color — the board never falls back to the old
// grey "no color" dot.
func TestSourceColorResolvesEverySource(t *testing.T) {
	m := sizedWorkspace(t, &contract.Repo{}, proj("app", src("web", "/r/web"), src("api", "/r/api")))
	web, api := m.sourceColors["web"], m.sourceColors["api"]
	if web == "" || api == "" {
		t.Fatalf("a colorless source resolved to a blank color: web=%q api=%q", web, api)
	}
	if web == api {
		t.Errorf("two colorless sources share the fallback %q — want distinct by position", web)
	}
	for _, hex := range []string{web, api} {
		if _, ok := projects.LookupSwatch(hex); !ok {
			t.Errorf("resolved fallback %q is not a palette swatch", hex)
		}
	}
	if m.sourceColor("web") == nil || m.sourceColor("ghost-unregistered") == nil {
		t.Error("sourceColor returned nil (even an unknown label must resolve non-blank)")
	}
}

// TestBoardSurfacesCarryColoredDots (cockpit-colors FR4): the board source tag, the
// filter chips, and the plans-tab source dot each render a `●` for every source (no grey
// no-color path remains).
func TestBoardSurfacesCarryColoredDots(t *testing.T) {
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "a", Title: "Alpha", Source: "web", Root: "/r/web", Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"},
	}}
	m := sizedWorkspace(t, repo, proj("app", src("web", "/r/web"), src("api", "/r/api")))
	// Filter chips carry a dot per source.
	if chips := m.viewSourceChips(); !strings.Contains(chips, "● web") || !strings.Contains(chips, "● api") {
		t.Errorf("source chips missing a colored dot per source:\n%s", chips)
	}
	// The board card source tag carries its dot.
	if out := m.viewBoard(); !strings.Contains(out, "● web") {
		t.Errorf("board card missing the source dot:\n%s", out)
	}
	// The plans-tab source dot resolves (non-empty render).
	if dot := m.sourceDot("web"); !strings.Contains(dot, "●") {
		t.Errorf("sourceDot = %q, want a ● glyph", dot)
	}
}

// TestChangelogSourceDot (cockpit-colors D3=A): a PROJECT-board changelog row leads with
// a source dot (`● ✓ slug`); a live session adds a SECOND (trailing) dot; a SINGLE-REPO
// changelog row carries NO leading source dot (byte-for-byte).
func TestChangelogSourceDot(t *testing.T) {
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "shipped-web", Source: "web", Root: "/r/web", Class: contract.ClassShipped, Completed: "2026-07-01"},
	}}
	m := sizedWorkspace(t, repo, proj("app", src("web", "/r/web")))

	out := m.renderColumn(3, m.boardColWidth())
	if !strings.Contains(out, "● ✓ shipped-web") {
		t.Errorf("project-board changelog row missing the leading source dot (`● ✓`):\n%s", out)
	}
	// No live session → exactly one dot on the row (the source origin dot).
	row := changelogRowText(out, "shipped-web")
	if n := strings.Count(row, "●"); n != 1 {
		t.Errorf("changelog row has %d dots, want 1 (source only, no session):\n%q", n, row)
	}

	// A live session on the shipped card → the relocated trailing green session dot: 2 dots.
	m.sessions = []string{"gogo-go-shipped-web"}
	rowLive := changelogRowText(m.renderColumn(3, m.boardColWidth()), "shipped-web")
	if n := strings.Count(rowLive, "●"); n != 2 {
		t.Errorf("live changelog row has %d dots, want 2 (source + trailing session):\n%q", n, rowLive)
	}

	// Single-repo changelog: no source → NO leading source dot (byte-for-byte).
	sm := newModel(t)
	if out := sm.renderColumn(3, sm.boardColWidth()); strings.Contains(out, "● ✓") {
		t.Errorf("single-repo changelog gained a source dot (must stay byte-for-byte):\n%s", out)
	}
}

// changelogRowText pulls the single rendered changelog line containing slug out of a
// column render (so a dot-count assertion targets just that row).
func changelogRowText(colRender, slug string) string {
	for _, ln := range strings.Split(colRender, "\n") {
		if strings.Contains(ln, slug) {
			return ln
		}
	}
	return ""
}

// TestOriginDotsTwoVsOne (cockpit-colors D5): originDots renders TWO dots (project +
// source) for a multi-project surface and a SINGLE dot when projectColor is nil.
func TestOriginDotsTwoVsOne(t *testing.T) {
	two := originDots(lipgloss.Color("#111111"), lipgloss.Color("#222222"), false)
	if n := strings.Count(two, "●"); n != 2 {
		t.Errorf("originDots(project, source) = %q, want 2 dots", two)
	}
	one := originDots(nil, lipgloss.Color("#222222"), false)
	if n := strings.Count(one, "●"); n != 1 {
		t.Errorf("originDots(nil, source) = %q, want 1 dot", one)
	}
}
