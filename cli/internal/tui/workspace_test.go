package tui

import (
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	tea "github.com/charmbracelet/bubbletea"
)

// proj is a compact single-project-with-named-sources builder for the source-native
// board tests: each name→path pair becomes one source of the project.
func proj(name string, srcs ...projects.Source) projects.Project {
	return projects.Project{Name: name, Sources: srcs}
}

// src is a compact source builder (name at path, optional cap via the variadic).
func src(name, path string, cap ...int) projects.Source {
	s := projects.Source{Name: name, Path: path}
	if len(cap) > 0 {
		s.ConcurrentWorkItems = cap[0]
	}
	return s
}

// selKey returns the composite selection key (featureKey = Root\x00Slug) for the first
// feature with slug in m.repo — the workspace-unique key m.selected is keyed by after
// REV-001. Test convenience so a test can select a card by slug.
func selKey(m Model, slug string) string {
	return featureKey(m.repo.Feature(slug))
}

// sizedWorkspace builds a source-native project board over an in-memory repo + a
// focused project and gives it a render size.
func sizedWorkspace(t *testing.T, repo *contract.Repo, p projects.Project) Model {
	t.Helper()
	m := NewWorkspace(repo, p)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	return nm.(Model)
}

// sizedWorkspaceAll builds the UNIFIED cockpit board (0.23.0) over an in-memory MERGED
// repo (features already carrying Project + Source, as LoadWorkspace stamps them) + the
// full project set, and gives it a render size.
func sizedWorkspaceAll(t *testing.T, repo *contract.Repo, projs []projects.Project) Model {
	t.Helper()
	m := NewWorkspaceAll(repo, projs)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	return nm.(Model)
}

func TestNewWorkspacePopulatesColumns(t *testing.T) {
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "a", Title: "Alpha", Source: "projA", Root: "/repos/a", Class: contract.ClassUnfinished, Status: "plan-accepted"},
		{Slug: "b", Title: "Beta", Source: "projB", Root: "/repos/b", Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"},
		{Slug: "c", Title: "Gamma", Source: "projA", Root: "/repos/a", Class: contract.ClassReadyToShip, Status: "awaiting-uat"},
	}}
	m := sizedWorkspace(t, repo, proj("myproj", src("projA", "/repos/a"), src("projB", "/repos/b")))

	if !m.global() {
		t.Fatalf("project board should be global (root empty), got root=%q", m.root)
	}
	if got := [4]int{len(m.cols[0]), len(m.cols[1]), len(m.cols[2]), len(m.cols[3])}; got != [4]int{1, 1, 1, 0} {
		t.Errorf("columns = %v, want [1 1 1 0]", got)
	}

	out := m.View()
	// Header carries the feature + project count (FR7); cards carry their source tags;
	// the tab bar + source chips only show on the project board.
	for _, want := range []string{"3 features", "1 project", "board", "plans", "config", "● projA", "● projB"} {
		if !strings.Contains(out, want) {
			t.Errorf("project view missing %q", want)
		}
	}
}

func TestRenderCardSourceTagPresentAndAbsent(t *testing.T) {
	m := sizedWorkspace(t, &contract.Repo{}, proj("myproj", src("projA", "/a")))

	tagged := &contract.Feature{Slug: "x", Title: "Ex", Source: "projA", Class: contract.ClassUnfinished, Status: "plan-accepted"}
	if card := m.renderCard(0, tagged, false, 40); !strings.Contains(card, "projA") {
		t.Errorf("card with Source should show the tag:\n%s", card)
	}
	// The focused card (single fg/bg fill) still renders the (plain) tag.
	if card := m.renderCard(0, tagged, true, 40); !strings.Contains(card, "projA") {
		t.Errorf("focused card with Source should show the tag:\n%s", card)
	}

	// Source == "" (single-repo parity): no source tag at all.
	untagged := &contract.Feature{Slug: "y", Title: "Why", Class: contract.ClassUnfinished, Status: "plan-accepted"}
	if card := m.renderCard(0, untagged, false, 40); strings.Contains(card, "projA") {
		t.Errorf("card without Source must show no source tag:\n%s", card)
	}
}

// TestSourceTagOnNameRow pins Phase-B nit #1 (design TURN-3a): the per-card source tag
// rides the NAME row (right-aligned after the slug), NOT the description row — and it
// never leaks onto a lone-repo card (Source == "").
func TestSourceTagOnNameRow(t *testing.T) {
	m := sizedWorkspace(t, &contract.Repo{}, proj("app", src("web", "/r/web")))
	f := &contract.Feature{Slug: "auth", Title: "Auth flow", Source: "web",
		Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"}

	for _, focused := range []bool{false, true} {
		card := m.renderCard(1, f, focused, 44)
		var nameRow, descRow string
		for _, ln := range strings.Split(card, "\n") {
			if strings.Contains(ln, "auth") { // the slug (lowercase) — the name row
				nameRow = ln
			}
			if strings.Contains(ln, "Auth flow") { // the title — the description row
				descRow = ln
			}
		}
		if nameRow == "" || descRow == "" {
			t.Fatalf("focused=%v: could not locate the name/description rows:\n%s", focused, card)
		}
		if !strings.Contains(nameRow, "web") {
			t.Errorf("focused=%v: source tag not on the name row:\n%s", focused, card)
		}
		if strings.Contains(descRow, "web") {
			t.Errorf("focused=%v: source tag leaked onto the description row:\n%s", focused, card)
		}
		// Right-aligned: the tag sits AFTER the slug on the name row.
		if strings.Index(nameRow, "auth") >= strings.Index(nameRow, "web") {
			t.Errorf("focused=%v: tag not right-aligned after the slug:\n%q", focused, nameRow)
		}
	}

	// Lone-repo parity: a Source-less card carries NO tag on either row.
	lone := &contract.Feature{Slug: "solo", Title: "Solo work", Class: contract.ClassInProgress, Phase: "implement"}
	if card := m.renderCard(1, lone, false, 44); strings.Contains(card, "web") {
		t.Errorf("Source-less card leaked a tag:\n%s", card)
	}
}

// TestRenderCardNarrowLongSourceTagNoWrap pins REV-006: a long source name on a narrow
// card must NOT wrap the name row (a wrap desyncs the per-card window-height math). The
// long-source card must render at the SAME height as a short-source one at the same
// width — the tag is truncated (or dropped) to fit, never pushed onto its own line.
func TestRenderCardNarrowLongSourceTagNoWrap(t *testing.T) {
	m := sizedWorkspace(t, &contract.Repo{}, proj("app",
		src("s", "/r/s"), src("backend-service-really-long", "/r/b")))
	m.sessions = nil // deterministic: no live-session dot regardless of the dev box

	const narrow = 20 // ~80-col terminal split across 4 columns → a very tight card
	short := &contract.Feature{Slug: "auth-flow", Title: "Auth", Source: "s",
		Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"}
	long := &contract.Feature{Slug: "auth-flow", Title: "Auth", Source: "backend-service-really-long",
		Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"}

	for _, focused := range []bool{false, true} {
		shortLines := len(strings.Split(m.renderCard(1, short, focused, narrow), "\n"))
		longCard := m.renderCard(1, long, focused, narrow)
		longLines := len(strings.Split(longCard, "\n"))
		if longLines != shortLines {
			t.Errorf("focused=%v: long source tag wrapped the card — long=%d lines, short=%d (want equal, no wrap):\n%s",
				focused, longLines, shortLines, longCard)
		}
	}

	// The lone-repo path (no source tag) is untouched — byte-for-byte parity.
	lone := &contract.Feature{Slug: "auth-flow", Title: "Auth",
		Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"}
	if strings.Contains(m.renderCard(1, lone, false, narrow), "●") {
		t.Errorf("lone-repo card grew a tag dot at narrow width")
	}
}

func TestMatchFilterProjectToken(t *testing.T) {
	fa := &contract.Feature{Slug: "auth-login", Title: "Login flow", Source: "webapp"}
	fb := &contract.Feature{Slug: "cache-warm", Title: "Warm cache", Source: "backend"}
	cases := []struct {
		q    string
		a, b bool
	}{
		{"@web", true, false},           // @source only
		{"login", true, false},          // text only (single-repo parity)
		{"@web login", true, false},     // AND: source + text, both hit a
		{"@web cache", false, false},    // AND: source a, text b → neither
		{"@backend cache", false, true}, // AND: hits b
		{"@WEB LOGIN", true, false},     // case-insensitive
		{"zzz", false, false},           // no match
	}
	for _, c := range cases {
		// The @source token is a project-board concept (global == true).
		if got := matchFilter(fa, c.q, true, nil); got != c.a {
			t.Errorf("matchFilter(a, %q) = %v, want %v", c.q, got, c.a)
		}
		if got := matchFilter(fb, c.q, true, nil); got != c.b {
			t.Errorf("matchFilter(b, %q) = %v, want %v", c.q, got, c.b)
		}
	}
}

// TestMatchFilterSingleRepoLiteralAt pins REV-002: in single-repo mode (global ==
// false) a filter starting with `@` must be matched LITERALLY over slug+title, not
// treated as a source token — otherwise, since single-repo features always have
// Source == "", an `@`-query would hide every card (an FR7 byte-for-byte parity
// gap). The SAME `@`-query in project-board mode is a source token.
func TestMatchFilterSingleRepoLiteralAt(t *testing.T) {
	f := &contract.Feature{Slug: "email-parser", Title: "Handle @user mentions"}
	cases := []struct {
		q      string
		global bool
		want   bool
	}{
		{"@user", false, true},    // single-repo: literal — the title contains "@user"
		{"@user", true, false},    // project board: @user is a source token, Source=="" → miss
		{"@zzz", false, false},    // single-repo: literal miss (no such substring)
		{"mentions", false, true}, // bare text — unchanged in both modes
		{"mentions", true, true},  // project board: text-only still matches slug+title
	}
	for _, c := range cases {
		if got := matchFilter(f, c.q, c.global, nil); got != c.want {
			t.Errorf("matchFilter(%q, global=%v) = %v, want %v", c.q, c.global, got, c.want)
		}
	}
}

// TestAttemptActionRefusesCrossProjectShip pins REV-001: on the project board, ready
// cards from different sources share the one 'ready' column. A merged ship launches a
// SINGLE /gogo:done rooted at the first slug's repo, so a selection spanning >1 source
// would mis-root the rest. attemptAction must refuse a cross-source selection (no
// intent) while a same-source multi-select still ships as a merge.
func TestAttemptActionRefusesCrossProjectShip(t *testing.T) {
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "a", Title: "A", Source: "projA", Root: "/repos/a", Class: contract.ClassReadyToShip, Status: "awaiting-uat"},
		{Slug: "b", Title: "B", Source: "projB", Root: "/repos/b", Class: contract.ClassReadyToShip, Status: "awaiting-uat"},
		{Slug: "c", Title: "C", Source: "projA", Root: "/repos/a", Class: contract.ClassReadyToShip, Status: "awaiting-uat"},
	}}
	m := NewWorkspace(repo, proj("myproj", src("projA", "/repos/a"), src("projB", "/repos/b")))

	// Two roots selected (a @ /repos/a, b @ /repos/b) → refused, no ship intent.
	m.selected = map[string]bool{selKey(m, "a"): true, selKey(m, "b"): true}
	if _, isShip, bounce := m.attemptAction(true); isShip || bounce == "" {
		t.Errorf("cross-source ship: isShip=%v bounce=%q, want refused with a bounce", isShip, bounce)
	}

	// Same-source multi-select (a + c, both @ /repos/a) → still ships as a merge.
	m.selected = map[string]bool{selKey(m, "a"): true, selKey(m, "c"): true}
	in, isShip, bounce := m.attemptAction(true)
	if !isShip || bounce != "" {
		t.Fatalf("same-source ship: isShip=%v bounce=%q, want a clean ship", isShip, bounce)
	}
	if in.Action != launch.ActionDone || len(in.Slugs) != 2 {
		t.Errorf("same-source ship intent = %+v, want ActionDone over 2 slugs", in)
	}
}

// TestDoLaunchBouncesOnEmptyRoot pins REV-004: on the project board (m.root == ""), a
// confirmed launch whose target feature has vanished from the merged repo resolves to
// an EMPTY root. doLaunch must bounce (launching nothing) rather than run relative to
// the process cwd.
func TestDoLaunchBouncesOnEmptyRoot(t *testing.T) {
	launched := false
	m := NewWorkspace(&contract.Repo{}, proj("a", src("a", "/a")))
	m.launcher = func(root string, in launch.Intent) (launch.Result, error) {
		launched = true
		return launch.Result{}, nil
	}
	m.pending = launch.BuildIntent(launch.ActionDone, []string{"ghost"}, "")
	cmd := m.doLaunch()
	if cmd == nil {
		t.Fatal("doLaunch returned a nil cmd")
	}
	msg := cmd()
	done, ok := msg.(launchDoneMsg)
	if !ok {
		t.Fatalf("doLaunch msg = %T, want launchDoneMsg", msg)
	}
	if launched {
		t.Error("launcher must NOT run when the resolved root is empty")
	}
	if !strings.Contains(done.status, "no longer present") {
		t.Errorf("bounce status = %q, want a 'no longer present' hint", done.status)
	}
}

func TestRootFor(t *testing.T) {
	m := Model{root: "/single"}
	if got := m.rootFor(&contract.Feature{Root: "/feat"}); got != "/feat" {
		t.Errorf("rootFor(feature with own root) = %q, want /feat", got)
	}
	if got := m.rootFor(&contract.Feature{}); got != "/single" {
		t.Errorf("rootFor(feature, empty root) = %q, want /single (fallback)", got)
	}
	if got := m.rootFor(nil); got != "/single" {
		t.Errorf("rootFor(nil) = %q, want /single (fallback)", got)
	}
	// Project board (m.root == ""): a feature's own root is the only source.
	agg := Model{}
	if got := agg.rootFor(&contract.Feature{Root: "/repos/x"}); got != "/repos/x" {
		t.Errorf("project rootFor = %q, want /repos/x", got)
	}
}

// TestBoardCapBounce: the board `m`→go path bounces when the target source is over
// its per-source cap, naming the cap + the live feature + the `--force` escape, and
// launches (returns a go intent) when it is not — the same rule cmdGo enforces via
// CapForSource.
func TestBoardCapBounce(t *testing.T) {
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "target", Title: "T", Source: "app", Root: "/repos/app", Class: contract.ClassUnfinished, Status: "plan-accepted"},
		{Slug: "other", Title: "O", Source: "app", Root: "/repos/app", Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"},
	}}
	capped := proj("myproj", src("app", "/repos/app", 1))

	// Over cap: "other" is in-progress + live, so a go on "target" would be the 2nd
	// concurrent build → bounce.
	m := NewWorkspace(repo, capped)
	m.sessions = []string{"gogo-go-other"}
	m.colIdx = 0 // "target" is the focused plan card
	if f := m.focusedCard(); f == nil || f.Slug != "target" {
		t.Fatalf("focused card = %v, want target", f)
	}
	_, isShip, bounce := m.attemptAction(false)
	if isShip || bounce == "" {
		t.Fatalf("over-cap go: isShip=%v bounce=%q, want a bounce", isShip, bounce)
	}
	for _, want := range []string{"cap 1", "other", "--force"} {
		if !strings.Contains(bounce, want) {
			t.Errorf("bounce %q missing %q", bounce, want)
		}
	}

	// No live session → under cap → the go intent is produced (launches).
	m2 := NewWorkspace(repo, capped)
	m2.sessions = nil
	m2.colIdx = 0
	in, isShip, bounce := m2.attemptAction(false)
	if isShip || bounce != "" {
		t.Fatalf("under-cap go: isShip=%v bounce=%q, want a clean go", isShip, bounce)
	}
	if in.Action != launch.ActionGo {
		t.Errorf("under-cap intent action = %v, want ActionGo", in.Action)
	}
}

// TestCapBounceUncappedInert pins FR7 (single-repo / unregistered parity): with no
// per-source cap for the target's root, capBounce is inert — byte-for-byte as today,
// even when other features are actively building.
func TestCapBounceUncappedInert(t *testing.T) {
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "target", Root: "/repos/app", Class: contract.ClassUnfinished, Status: "plan-accepted"},
		{Slug: "other", Root: "/repos/app", Class: contract.ClassInProgress, Phase: "implement"},
	}}
	target := repo.Features[0]

	// Lone repo (no project → no sources) → no cap → inert (single-repo fallback).
	m := Model{root: "/repos/app", repo: repo, sessions: []string{"gogo-go-other"}}
	if b := m.capBounce(target); b != "" {
		t.Errorf("lone repo bounced: %q, want inert", b)
	}

	// Registered but uncapped (ConcurrentWorkItems 0 = unlimited) → still inert.
	p := proj("myproj", src("app", "/repos/app", 0))
	m.project = &p
	if b := m.capBounce(target); b != "" {
		t.Errorf("uncapped source bounced: %q, want inert", b)
	}
}
