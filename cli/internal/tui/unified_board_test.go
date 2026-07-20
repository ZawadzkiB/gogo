package tui

import (
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	tea "github.com/charmbracelet/bubbletea"
)

// TestUnifiedSameSlugAcrossProjects (REV-001): two projects that share a slug (both
// have `feature-cli`) must not collide. Asserts (a) launching from project-B's card
// anchors the intent at B's OWN root (not the first slug-match, A's); (b) `space`
// selects exactly ONE card (composite key, not the shared slug); (c) selecting the two
// same-slug cards from different projects trips the cross-project ship guard.
func TestUnifiedSameSlugAcrossProjects(t *testing.T) {
	fa := &contract.Feature{Slug: "cli", Title: "A cli", Project: "alpha", Source: "alpha", Root: "/r/a", Class: contract.ClassReadyToShip, Status: "awaiting-uat"}
	fb := &contract.Feature{Slug: "cli", Title: "B cli", Project: "beta", Source: "beta", Root: "/r/b", Class: contract.ClassReadyToShip, Status: "awaiting-uat"}
	repo := &contract.Repo{Features: []*contract.Feature{fa, fb}}
	projs := []projects.Project{proj("alpha", src("alpha", "/r/a")), proj("beta", src("beta", "/r/b"))}

	// Locate beta's card in the ready column (both cli cards land there).
	betaIdx := func(m Model) int {
		for i, c := range m.cols[2] {
			if c.Root == "/r/b" {
				return i
			}
		}
		return -1
	}

	// (a) Launch from beta's focused card → the intent anchors at /r/b, not /r/a.
	m := sizedWorkspaceAll(t, repo, projs)
	m.colIdx = 2
	idx := betaIdx(m)
	if idx < 0 {
		t.Fatal("beta's cli card is not in the ready column")
	}
	m.cardIdx[2] = idx
	if got := m.focusedCard(); got == nil || got.Root != "/r/b" {
		t.Fatalf("focused card root = %v, want /r/b", got)
	}
	in, isShip, bounce := m.attemptAction(true)
	if bounce != "" || !isShip {
		t.Fatalf("ship of beta's cli: bounce=%q isShip=%v", bounce, isShip)
	}
	if in.Root != "/r/b" {
		t.Errorf("launch anchored at %q, want /r/b (beta's OWN root, not the first slug-match /r/a)", in.Root)
	}

	// (b) space on beta's card selects exactly ONE card (composite key), not both cli's.
	m2 := sizedWorkspaceAll(t, repo, projs)
	m2.colIdx = 2
	m2.cardIdx[2] = betaIdx(m2)
	m2 = send(m2, tea.KeyMsg{Type: tea.KeySpace})
	if n := len(m2.selectedFeatures()); n != 1 {
		t.Errorf("space selected %d cards, want exactly 1 (same-slug cards keyed distinctly)", n)
	}
	if sel := m2.selectedFeatures(); len(sel) == 1 && sel[0].Root != "/r/b" {
		t.Errorf("space selected the wrong same-slug card (root %q, want /r/b)", sel[0].Root)
	}

	// (c) selecting BOTH same-slug cards from different projects trips the guard.
	m3 := sizedWorkspaceAll(t, repo, projs)
	m3.selected[featureKey(fa)] = true
	m3.selected[featureKey(fb)] = true
	if !selectionSpansProjects(m3.selectedFeatures()) {
		t.Error("same-slug cross-project selection did not trip selectionSpansProjects")
	}
	if _, isShip, bounce := m3.attemptAction(true); isShip || bounce == "" {
		t.Errorf("cross-project merged ship: isShip=%v bounce=%q, want refused", isShip, bounce)
	}
}

// TestUnifiedHeaderCountsAllProjects (FR1): the unified board header `N features · M
// projects` counts EVERY project's features (N) and every project (M) — the mismatch
// the pre-0.23 board carried (N was one project's, M was all) is gone.
func TestUnifiedHeaderCountsAllProjects(t *testing.T) {
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "a", Project: "alpha", Source: "web", Root: "/r/a", Class: contract.ClassUnfinished, Status: "plan-accepted"},
		{Slug: "b", Project: "beta", Source: "api", Root: "/r/b", Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"},
		{Slug: "c", Project: "beta", Source: "api", Root: "/r/b", Class: contract.ClassReadyToShip, Status: "awaiting-uat"},
	}}
	projs := []projects.Project{proj("alpha", src("web", "/r/a")), proj("beta", src("api", "/r/b"))}
	m := sizedWorkspaceAll(t, repo, projs)

	out := m.View()
	for _, want := range []string{"3 features", "2 projects"} {
		if !strings.Contains(out, want) {
			t.Errorf("unified header missing %q:\n%s", want, out)
		}
	}
}

// TestMatchFilterMatchesProjectOrSource (FR3, D3=A): the free-text `@name` token narrows
// to a feature's PROJECT or SOURCE (fixing the drift where it only matched Source).
func TestMatchFilterMatchesProjectOrSource(t *testing.T) {
	f := &contract.Feature{Slug: "login", Title: "Login", Project: "alpha", Source: "webapp"}
	cases := []struct {
		q    string
		want bool
	}{
		{"@alpha", true},       // project match
		{"@web", true},         // source match (substring of webapp)
		{"@beta", false},       // neither
		{"@alpha login", true}, // AND: origin + text, both hit
		{"@alpha zzz", false},  // AND: origin hits, text misses
		{"@ALPHA", true},       // case-insensitive
	}
	for _, c := range cases {
		if got := matchFilter(f, c.q, true, nil); got != c.want {
			t.Errorf("matchFilter(%q) = %v, want %v", c.q, got, c.want)
		}
	}
}

// TestUnifiedCardTwoNameOriginTag (FR2, D5=B): a card whose feature carries both a
// project + a source renders the two-name `●project ●source` origin tag on the name row
// (both names spelled out, two dots), focused and unfocused.
func TestUnifiedCardTwoNameOriginTag(t *testing.T) {
	m := sizedWorkspaceAll(t, &contract.Repo{}, []projects.Project{proj("gogo", src("gogo-cli", "/r/cli"))})
	m.sessions = nil
	f := &contract.Feature{Slug: "unified-board", Title: "Unified", Project: "gogo", Source: "gogo-cli",
		Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"}

	for _, focused := range []bool{false, true} {
		card := m.renderCard(1, f, focused, 60)
		var nameRow string
		for _, ln := range strings.Split(card, "\n") {
			if strings.Contains(ln, "unified-board") {
				nameRow = ln
			}
		}
		if nameRow == "" {
			t.Fatalf("focused=%v: could not locate the name row:\n%s", focused, card)
		}
		if !strings.Contains(nameRow, "gogo") || !strings.Contains(nameRow, "gogo-cli") {
			t.Errorf("focused=%v: two-name origin tag missing project+source:\n%q", focused, nameRow)
		}
		if n := strings.Count(nameRow, "●"); n < 2 {
			t.Errorf("focused=%v: name row has %d dots, want ≥2 (●project ●source):\n%q", focused, n, nameRow)
		}
	}

	// A feature with only a Source (no Project) still degrades to the single `● source`
	// tag — byte-for-byte parity with the legacy single-project card.
	single := &contract.Feature{Slug: "x", Title: "X", Source: "gogo-cli",
		Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"}
	card := m.renderCard(1, single, false, 60)
	var row string
	for _, ln := range strings.Split(card, "\n") {
		if strings.Contains(ln, "x") && strings.Contains(ln, "●") {
			row = ln
		}
	}
	if strings.Count(row, "●") != 1 {
		t.Errorf("Project-less card should carry a single source dot:\n%q", row)
	}
}

// TestUnifiedCardDedupsEqualProjectSource (D5=B dedup): when a feature's Project ==
// Source (a single-source project named after its repo — the common case), the origin
// tag shows the name ONCE with a single dot, not the doubled `●name ●name`.
func TestUnifiedCardDedupsEqualProjectSource(t *testing.T) {
	m := sizedWorkspaceAll(t, &contract.Repo{}, []projects.Project{proj("very-nice-mermaid", src("very-nice-mermaid", "/r/vnm"))})
	m.sessions = nil
	f := &contract.Feature{Slug: "add-arrows", Title: "Arrows", Project: "very-nice-mermaid", Source: "very-nice-mermaid",
		Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"}

	for _, focused := range []bool{false, true} {
		card := m.renderCard(1, f, focused, 60)
		var nameRow string
		for _, ln := range strings.Split(card, "\n") {
			if strings.Contains(ln, "add-arrows") {
				nameRow = ln
			}
		}
		if nameRow == "" {
			t.Fatalf("focused=%v: could not locate the name row:\n%s", focused, card)
		}
		// The equal name appears exactly ONCE (deduped), never twice.
		if c := strings.Count(nameRow, "very-nice-mermaid"); c != 1 {
			t.Errorf("focused=%v: equal project/source should show the name once, got %d:\n%q", focused, c, nameRow)
		}
		// A single origin dot (no live session in this test).
		if d := strings.Count(nameRow, "●"); d != 1 {
			t.Errorf("focused=%v: deduped origin should be a single dot, got %d:\n%q", focused, d, nameRow)
		}
	}
}

// TestUnifiedTagSlugStaysReadable (slug-first fit): on a narrow card a long origin tag
// must NOT crush the ticket slug — the slug keeps its readable floor while the tag
// collapses (truncates / drops), and the name row never wraps.
func TestUnifiedTagSlugStaysReadable(t *testing.T) {
	m := sizedWorkspaceAll(t, &contract.Repo{}, []projects.Project{proj("very-nice-mermaid", src("very-nice-mermaid", "/r/vnm"))})
	m.sessions = nil
	long := &contract.Feature{Slug: "render-the-whole-thing", Title: "Render", Project: "very-nice-mermaid", Source: "very-nice-mermaid",
		Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"}

	const narrow = 30
	card := m.renderCard(1, long, false, narrow)
	var nameRow string
	for _, ln := range strings.Split(card, "\n") {
		if strings.Contains(ln, "render") {
			nameRow = ln
		}
	}
	// The slug keeps a healthy readable prefix (≥ ~12 runes), not crushed to ~4.
	if !strings.Contains(nameRow, "render-the-w") {
		t.Errorf("origin tag crushed the slug (should stay readable, ≥ its floor):\n%q", nameRow)
	}
	// No wrap: the long-slug card is the SAME height as a short-slug one at this width.
	short := &contract.Feature{Slug: "x", Title: "Render", Project: "very-nice-mermaid", Source: "very-nice-mermaid",
		Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"}
	if a, b := len(strings.Split(card, "\n")), len(strings.Split(m.renderCard(1, short, false, narrow), "\n")); a != b {
		t.Errorf("card height changed (wrap?): long=%d lines, short=%d", a, b)
	}
}

// TestUnifiedTwoNameTagNoWrap (FR2 / REV-006): a wide `●project ●source` pair on a
// narrow card must NOT wrap the name row (a wrap desyncs the per-card window math). The
// long-origin card renders at the SAME height as a short-origin one — the tag is
// truncated (or dropped) to fit, never pushed onto its own line.
func TestUnifiedTwoNameTagNoWrap(t *testing.T) {
	projs := []projects.Project{
		proj("a", src("s", "/r/s")),
		proj("backend-platform", src("really-long-service", "/r/b")),
	}
	m := sizedWorkspaceAll(t, &contract.Repo{}, projs)
	m.sessions = nil

	const narrow = 22
	short := &contract.Feature{Slug: "auth-flow", Title: "Auth", Project: "a", Source: "s",
		Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"}
	long := &contract.Feature{Slug: "auth-flow", Title: "Auth", Project: "backend-platform", Source: "really-long-service",
		Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"}

	for _, focused := range []bool{false, true} {
		shortLines := len(strings.Split(m.renderCard(1, short, focused, narrow), "\n"))
		longCard := m.renderCard(1, long, focused, narrow)
		longLines := len(strings.Split(longCard, "\n"))
		if longLines != shortLines {
			t.Errorf("focused=%v: two-name origin tag wrapped — long=%d lines, short=%d (want equal, no wrap):\n%s",
				focused, longLines, shortLines, longCard)
		}
	}
}

// TestUnifiedChangelogRowTwoDots (FR2, D5=B): a UNIFIED-board changelog row leads with
// the two-dot `●project ●source` origin (`● ● ✓ slug`); a live session adds a THIRD
// trailing dot.
func TestUnifiedChangelogRowTwoDots(t *testing.T) {
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "shipped-x", Project: "alpha", Source: "web", Root: "/r/web", Class: contract.ClassShipped, Completed: "2026-07-01"},
	}}
	m := sizedWorkspaceAll(t, repo, []projects.Project{proj("alpha", src("web", "/r/web"))})
	m.sessions = nil

	out := m.renderColumn(3, m.boardColWidth())
	if !strings.Contains(out, "● ● ✓ shipped-x") {
		t.Errorf("unified changelog row missing the two-dot origin lead (`● ● ✓`):\n%s", out)
	}
	row := changelogRowText(out, "shipped-x")
	if n := strings.Count(row, "●"); n != 2 {
		t.Errorf("changelog row has %d dots, want 2 (project + source, no session):\n%q", n, row)
	}

	// A live session on the shipped card → the relocated trailing green session dot: 3 dots.
	m.sessions = []string{"gogo-go-shipped-x"}
	rowLive := changelogRowText(m.renderColumn(3, m.boardColWidth()), "shipped-x")
	if n := strings.Count(rowLive, "●"); n != 3 {
		t.Errorf("live unified changelog row has %d dots, want 3 (project + source + session):\n%q", n, rowLive)
	}
}

// TestUnifiedCapBounceSpansProjects (FR5 REGRESSION): the focus is alpha, but the capped
// card lives in BETA — a non-focused project. capBounce must STILL bounce (it resolves
// projects.AllSources, not just the focused project's sources — the pre-fix bug left a
// cross-project card uncapped).
func TestUnifiedCapBounceSpansProjects(t *testing.T) {
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "target", Project: "beta", Source: "api", Root: "/r/b", Class: contract.ClassUnfinished, Status: "plan-accepted"},
		{Slug: "other", Project: "beta", Source: "api", Root: "/r/b", Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"},
	}}
	projs := []projects.Project{
		proj("alpha", src("web", "/r/a")),
		proj("beta", src("api", "/r/b", 1)), // beta's source capped at 1
	}
	m := NewWorkspaceAll(repo, projs) // focus defaults to alpha (projs[0])
	if m.project.Name != "alpha" {
		t.Fatalf("focus = %q, want alpha (a NON-owning project for the capped card)", m.project.Name)
	}
	m.sessions = []string{"gogo-go-other"}

	bounce := m.capBounce(repo.Features[0]) // target, whose source is in beta
	if bounce == "" {
		t.Fatalf("cross-project card not capped — capBounce went inert (the FR5 regression)")
	}
	for _, want := range []string{"cap 1", "other"} {
		if !strings.Contains(bounce, want) {
			t.Errorf("bounce %q missing %q", bounce, want)
		}
	}
}

// TestUnifiedWatchDirsSpansProjects (FR5): watchDirs arms EVERY project's source tree —
// even a non-focused project's — so live refresh spans projects.
func TestUnifiedWatchDirsSpansProjects(t *testing.T) {
	projs := []projects.Project{
		proj("alpha", src("web", "/r/a")),
		proj("beta", src("api", "/r/b")),
	}
	m := NewWorkspaceAll(&contract.Repo{}, projs) // focus = alpha
	joined := strings.Join(m.watchDirs(), "\n")
	for _, want := range []string{"/r/a/.gogo/work", "/r/b/.gogo/work"} {
		if !strings.Contains(joined, want) {
			t.Errorf("watchDirs missing %q (FR5: must span every project's sources):\n%s", want, joined)
		}
	}
}

// TestUnifiedConfigSwitcherSharesFocus (FR4, D4): the config-tab `p` switcher moves the
// shared focus AND narrows the board's project chip to match — the board chip + config
// switcher share ONE m.project.
func TestUnifiedConfigSwitcherSharesFocus(t *testing.T) {
	projs := []projects.Project{proj("alpha", src("web", "/r/a")), proj("beta", src("api", "/r/b"))}
	m := sizedWorkspaceAll(t, &contract.Repo{}, projs)
	if m.project.Name != "alpha" {
		t.Fatalf("default focus = %q, want alpha (allProjects[0])", m.project.Name)
	}
	m.switchProject(m.projIdx + 1) // the config `p` switcher
	if m.project.Name != "beta" {
		t.Errorf("switchProject did not move the focus: %q, want beta", m.project.Name)
	}
	if m.projectChip != "beta" {
		t.Errorf("config switcher did not share the board chip: projectChip=%q, want beta (D4)", m.projectChip)
	}
}

// TestUnifiedSingleProjectDegrades (FR5): a single registered project still opens the
// unified board — the project chip row collapses to `all` + one, `p` cycles it, no crash.
func TestUnifiedSingleProjectDegrades(t *testing.T) {
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "a", Project: "solo", Source: "web", Root: "/r/a", Class: contract.ClassUnfinished, Status: "plan-accepted"},
	}}
	m := sizedWorkspaceAll(t, repo, []projects.Project{proj("solo", src("web", "/r/a"))})

	if !strings.Contains(m.View(), "1 project") {
		t.Errorf("single-project unified header missing `1 project`:\n%s", m.View())
	}
	if chips := m.viewProjectChips(); !strings.Contains(chips, "all") || !strings.Contains(chips, "solo") {
		t.Errorf("single-project chip row missing all+solo:\n%q", chips)
	}
	m = send(m, runes("p"))
	if m.projectChip != "solo" || len(m.cols[0]) != 1 {
		t.Errorf("after p: chip=%q col0=%d, want solo → 1 (no regression)", m.projectChip, len(m.cols[0]))
	}
}
