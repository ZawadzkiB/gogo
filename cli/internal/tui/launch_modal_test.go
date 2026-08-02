package tui

import (
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- launch-confirm-modal-and-fast-toggle (0.36.0) ----------------------------
//
// D1=B: the board's go confirm is a THREE-OPTION SELECT (Launch / Launch --fast /
// Cancel), seeded from the source's fastMode config, with the exact command shown
// in the TITLE and re-evaluated live as the selection moves (REV-001: one-line
// option labels — a wrapped label pushed Cancel out of huh's row-per-option
// viewport). D2=B: that launch-confirm site is the ONE
// form rendered as a centered modal over the still-visible board; every other
// form keeps the full-screen takeover. D3=A: the backdrop is ANSI-stripped and
// dimmed. D4=A: formOrigin (recorded at entry) picks both the backdrop and the
// return mode.

// modalRepo builds a two-card single-source workspace: "next" is the launch
// target, "backdrop" exists to be visible BEHIND the modal.
func modalRepo() *contract.Repo {
	return &contract.Repo{Features: []*contract.Feature{
		{Slug: "backdrop", Title: "Backdrop card", Source: "web", Root: "/r/web", Class: contract.ClassUnfinished, Status: "plan-accepted"},
		{Slug: "next", Title: "Next", Source: "web", Root: "/r/web", Class: contract.ClassUnfinished, Status: "plan-accepted"},
	}}
}

// openGoConfirm focuses `slug` in the plan column and presses m, returning the
// model with the launch confirm open.
func openGoConfirm(t *testing.T, m Model, slug string) Model {
	t.Helper()
	m.colIdx = 0
	found := false
	for i, f := range m.cols[0] {
		if f.Slug == slug {
			m.cardIdx[0] = i
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("fixture: %q not in the plan column", slug)
	}
	m = send(m, runes("m"))
	if m.mode != modeForm || m.form == nil {
		t.Fatalf("m did not open the launch confirm (mode=%d status=%q)", m.mode, m.status)
	}
	return m
}

// TestLaunchConfirmSeedsFastFromSource (FR1): the Select opens pre-highlighted on
// whichever mode the source's fastMode config implies — the seed reads the command
// intentFor already resolved, so config and seed agree by construction — with the
// seeded command in the live title and both launch modes visible as options
// (D1=B discoverability).
func TestLaunchConfirmSeedsFastFromSource(t *testing.T) {
	t.Run("plain source seeds the full pipeline", func(t *testing.T) {
		m := sizedWorkspace(t, modalRepo(), proj("app", src("web", "/r/web")))
		m.hasClaude = true
		m = openGoConfirm(t, m, "next")
		if m.binding == nil || m.binding.launchMode != goLaunchFull {
			t.Fatalf("plain-source seed = %q, want %q", m.binding.launchMode, goLaunchFull)
		}
		out := m.View()
		// The TITLE carries the exact seeded command (REV-001 moved it out of the
		// option labels); both launch modes stay visible as one-line options.
		for _, want := range []string{`claude "/gogo:go next"`, "Launch --fast", "Cancel"} {
			if !strings.Contains(out, want) {
				t.Errorf("the confirm does not show %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, `claude "/gogo:go next --fast"`) {
			t.Errorf("the title already shows the --fast command while the seed is full:\n%s", out)
		}
		// FR2: moving the selection re-evaluates the title LIVE through the same
		// SetFastParam producer.
		m = keyPress(t, m, runes("j")) // Launch -> Launch --fast
		if out := m.View(); !strings.Contains(out, `claude "/gogo:go next --fast"`) {
			t.Errorf("after moving to Launch --fast the title still shows the full command:\n%s", out)
		}
	})

	t.Run("fastMode source seeds fast", func(t *testing.T) {
		speedy := projects.Source{Name: "web", Path: "/r/web", FastMode: true}
		m := sizedWorkspace(t, modalRepo(), proj("app", speedy))
		m.hasClaude = true
		m = openGoConfirm(t, m, "next")
		if m.binding == nil || m.binding.launchMode != goLaunchFast {
			t.Fatalf("fastMode-source seed = %q, want %q", m.binding.launchMode, goLaunchFast)
		}
	})
}

// TestFastChoiceChangesTheLaunchedCommand (FR2/FR4): moving the Select to the
// other launch option launches EXACTLY that option's command — with --fast from a
// plain source, and WITHOUT it from a fastMode source — fired exactly once.
func TestFastChoiceChangesTheLaunchedCommand(t *testing.T) {
	t.Run("plain source, choose fast", func(t *testing.T) {
		m := sizedWorkspace(t, modalRepo(), proj("app", src("web", "/r/web")))
		m.hasClaude = true
		rl := &recordingLauncher{}
		m.launcher = rl.launch
		m = openGoConfirm(t, m, "next")
		m = keyPress(t, m, runes("j")) // Launch -> Launch --fast
		m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter})
		if len(rl.calls) != 1 {
			t.Fatalf("launcher fired %d times, want exactly 1", len(rl.calls))
		}
		if got := rl.calls[0].Command; got != "/gogo:go next --fast" {
			t.Errorf("launched %q, want the --fast command the option showed", got)
		}
		if got := rl.calls[0].Session; got != launch.BuildIntent(launch.ActionGo, []string{"next"}, "").Session {
			t.Errorf("the fast choice changed the SESSION to %q - SetFastParam must touch only Command (FR4)", got)
		}
	})

	t.Run("fastMode source, choose full", func(t *testing.T) {
		speedy := projects.Source{Name: "web", Path: "/r/web", FastMode: true}
		m := sizedWorkspace(t, modalRepo(), proj("app", speedy))
		m.hasClaude = true
		rl := &recordingLauncher{}
		m.launcher = rl.launch
		m = openGoConfirm(t, m, "next")
		m = keyPress(t, m, runes("k")) // Launch --fast -> Launch
		m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter})
		if len(rl.calls) != 1 {
			t.Fatalf("launcher fired %d times, want exactly 1", len(rl.calls))
		}
		if got := rl.calls[0].Command; got != "/gogo:go next" {
			t.Errorf("launched %q, want the full-pipeline command with NO --fast", got)
		}
	})
}

// TestBareEnterStillLaunchesOnce (FR7): the CONFIRM-DEFAULT CONVENTION at the
// D1=B shape — the seeded option IS a launch, so `m` -> bare Enter fires the
// launcher exactly once with exactly the seeded (config-implied) command.
func TestBareEnterStillLaunchesOnce(t *testing.T) {
	cases := []struct {
		name string
		src  projects.Source
		want string
	}{
		{"plain source", src("web", "/r/web"), "/gogo:go next"},
		{"fastMode source", projects.Source{Name: "web", Path: "/r/web", FastMode: true}, "/gogo:go next --fast"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := sizedWorkspace(t, modalRepo(), proj("app", c.src))
			m.hasClaude = true
			rl := &recordingLauncher{}
			m.launcher = rl.launch
			m = openGoConfirm(t, m, "next")
			m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // BARE ENTER
			if len(rl.calls) != 1 {
				t.Fatalf("bare Enter fired the launcher %d times, want exactly 1", len(rl.calls))
			}
			if got := rl.calls[0].Command; got != c.want {
				t.Errorf("bare Enter launched %q, want the seeded command %q", got, c.want)
			}
		})
	}
}

// TestGoConfirmCancelOptionLaunchesNothing: the Select's third option is the
// cancel door (D1=B replaced the Confirm's Negative) — choosing it launches
// nothing and returns to the recorded origin.
func TestGoConfirmCancelOptionLaunchesNothing(t *testing.T) {
	m := sizedWorkspace(t, modalRepo(), proj("app", src("web", "/r/web")))
	m.hasClaude = true
	rl := &recordingLauncher{}
	m.launcher = rl.launch
	m = openGoConfirm(t, m, "next")
	m = keyPress(t, m, runes("j")) // -> Launch --fast
	m = keyPress(t, m, runes("j")) // -> Cancel
	m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if len(rl.calls) != 0 {
		t.Fatalf("Cancel fired the launcher %d times", len(rl.calls))
	}
	if m.mode != modeBoard {
		t.Errorf("Cancel left mode=%d, want the board (formOrigin)", m.mode)
	}
	if m.status != "cancelled" {
		t.Errorf("status = %q, want cancelled", m.status)
	}
}

// TestFastChoiceDoesNotWriteSourceConfig (FR5): the choice is per-launch only —
// flipping a fastMode source's launch to full and launching never rewrites the
// source's config.json; the next launch re-seeds from the config.
func TestFastChoiceDoesNotWriteSourceConfig(t *testing.T) {
	seedDataHome(t)
	speedy := projects.Source{Name: "web", Path: "/r/web", FastMode: true}
	p := proj("app", speedy)
	if _, err := projects.Add(p); err != nil {
		t.Fatal(err)
	}
	m := sizedWorkspace(t, modalRepo(), p)
	m.hasClaude = true
	rl := &recordingLauncher{}
	m.launcher = rl.launch
	m = openGoConfirm(t, m, "next")
	m = keyPress(t, m, runes("k")) // flip to the full pipeline for THIS launch
	m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if len(rl.calls) != 1 || launch.HasFastParam(rl.calls[0].Command) {
		t.Fatalf("expected one full-pipeline launch, got %+v", rl.calls)
	}
	got, err := projects.Load("app")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sources) != 1 || !got.Sources[0].FastMode {
		t.Errorf("the per-launch choice REWROTE the source config: %+v (fastMode must stay true)", got.Sources)
	}
}

// TestNoFastOptionOnNonGoConfirms (FR6): a ship (d), an accept (m on a
// plan-pending card) and the P plan-session confirm are byte-for-byte today's
// Launch/Cancel Confirms — no fast option, no launchMode discriminator.
func TestNoFastOptionOnNonGoConfirms(t *testing.T) {
	open := func(t *testing.T, m Model) Model {
		t.Helper()
		if m.mode != modeForm || m.form == nil {
			t.Fatalf("the confirm did not open (mode=%d status=%q)", m.mode, m.status)
		}
		if m.binding == nil || m.binding.launchMode != "" {
			t.Fatalf("a non-go confirm carries launchMode %q - the fast option leaked (FR6)", m.binding.launchMode)
		}
		if out := m.View(); strings.Contains(out, "Launch --fast") {
			t.Errorf("a non-go confirm lists a fast option:\n%s", out)
		}
		return m
	}

	t.Run("ship confirm", func(t *testing.T) {
		repo := &contract.Repo{Features: []*contract.Feature{
			{Slug: "shipme", Title: "Ship me", Source: "web", Root: "/r/web", Class: contract.ClassReadyToShip, Status: "awaiting-uat"},
		}}
		m := sizedWorkspace(t, repo, proj("app", src("web", "/r/web")))
		m.hasClaude = true
		m.colIdx = 2
		open(t, send(m, runes("d")))
	})

	t.Run("accept confirm", func(t *testing.T) {
		repo := &contract.Repo{Features: []*contract.Feature{
			{Slug: "pending", Title: "Pending", Source: "web", Root: "/r/web", Class: contract.ClassUnfinished, Status: "awaiting-plan-acceptance"},
		}}
		m := sizedWorkspace(t, repo, proj("app", src("web", "/r/web")))
		m.hasClaude = true
		m.colIdx = 0
		open(t, send(m, runes("m")))
	})

	t.Run("P plan-session confirm", func(t *testing.T) {
		m := sizedWorkspace(t, modalRepo(), proj("app", src("web", "/r/web")))
		m.hasClaude = true
		m.colIdx = 0
		open(t, send(m, runes("P")))
	})
}

// TestMergedShipReleaseInputStillReceivesF (FR6): `f` is not an intercepted key
// anywhere — typing it into a merged-ship release-name Input still types f.
func TestMergedShipReleaseInputStillReceivesF(t *testing.T) {
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "alpha-one", Source: "web", Root: "/r/web", Class: contract.ClassReadyToShip, Status: "awaiting-uat"},
		{Slug: "alpha-two", Source: "web", Root: "/r/web", Class: contract.ClassReadyToShip, Status: "awaiting-uat"},
	}}
	m := sizedWorkspace(t, repo, proj("app", src("web", "/r/web")))
	m.hasClaude = true
	m.selected = map[string]bool{selKey(m, "alpha-one"): true, selKey(m, "alpha-two"): true}
	m = send(m, runes("d"))
	if m.mode != modeForm || m.binding == nil {
		t.Fatalf("merged ship confirm did not open (mode=%d)", m.mode)
	}
	before := m.binding.release
	m = send(m, runes("f"))
	if m.binding.release != before+"f" {
		t.Errorf("typing f into the release Input produced %q (was %q) - the key was intercepted", m.binding.release, before)
	}
}

// TestFormRendersAsModalOverItsOrigin (FR8/FR13): at a real size the launch
// confirm is a bordered box composited over the still-visible board — the
// background card's text, the form's text, and the border glyphs are all
// present in one colourless View().
func TestFormRendersAsModalOverItsOrigin(t *testing.T) {
	m := sizedWorkspace(t, modalRepo(), proj("app", src("web", "/r/web")))
	m.hasClaude = true
	m = openGoConfirm(t, m, "next")
	out := m.View()
	for _, want := range []string{
		"backdrop",               // the board stays visible around the box
		`claude "/gogo:go next"`, // the form's own text
		"╭", "╰",                 // the box's border glyphs (FR13 — no colour needed)
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the modal composite lacks %q:\n%s", want, out)
		}
	}
	if strings.HasPrefix(out, "\n") {
		t.Error("the sized launch confirm still renders the full-screen form (leading newline)")
	}
}

// TestModalBackgroundFollowsRecordedOrigin (FR9): the backdrop is the RECORDED
// origin, never inferred — after a drill visit (esc back to the board), the
// modal composites over the BOARD, not the stale drill. The pickerFromDrill
// bug, generalised to the modal.
func TestModalBackgroundFollowsRecordedOrigin(t *testing.T) {
	m := sizedWorkspace(t, modalRepo(), proj("app", src("web", "/r/web")))
	m.hasClaude = true
	m.colIdx = 0
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter}) // drill in
	if m.mode != modeDrill {
		t.Fatalf("enter did not drill (mode=%d)", m.mode)
	}
	m = send(m, tea.KeyMsg{Type: tea.KeyEsc}) // back to the board
	m = openGoConfirm(t, m, "next")
	if m.formOrigin != modeBoard {
		t.Fatalf("formOrigin = %d after a drill visit, want the board", m.formOrigin)
	}
	out := m.View()
	if strings.Contains(out, "card — ") {
		t.Errorf("the modal composited over the stale DRILL view:\n%s", out)
	}
	if !strings.Contains(out, "backdrop") {
		t.Errorf("the modal's backdrop is not the board:\n%s", out)
	}
}

// TestModalNeverExceedsTheTerminal (FR11): over a size matrix and both modal
// forms (the go Select and the merged-ship Input+Confirm), every rendered line
// fits the terminal width and the total height fits the terminal height.
func TestModalNeverExceedsTheTerminal(t *testing.T) {
	sizes := [][2]int{{200, 40}, {120, 30}, {90, 24}, {80, 20}, {60, 15}}
	forms := map[string]func(t *testing.T, w, h int) Model{
		"go select": func(t *testing.T, w, h int) Model {
			m := NewWorkspace(modalRepo(), proj("app", src("web", "/r/web")))
			m.hasClaude = true
			m = send(m, tea.WindowSizeMsg{Width: w, Height: h})
			return openGoConfirm(t, m, "next")
		},
		"merged ship": func(t *testing.T, w, h int) Model {
			repo := &contract.Repo{Features: []*contract.Feature{
				{Slug: "alpha-one", Source: "web", Root: "/r/web", Class: contract.ClassReadyToShip, Status: "awaiting-uat"},
				{Slug: "alpha-two", Source: "web", Root: "/r/web", Class: contract.ClassReadyToShip, Status: "awaiting-uat"},
			}}
			m := NewWorkspace(repo, proj("app", src("web", "/r/web")))
			m.hasClaude = true
			m = send(m, tea.WindowSizeMsg{Width: w, Height: h})
			m.selected = map[string]bool{selKey(m, "alpha-one"): true, selKey(m, "alpha-two"): true}
			m = send(m, runes("d"))
			if m.mode != modeForm {
				t.Fatalf("merged ship confirm did not open (mode=%d)", m.mode)
			}
			return m
		},
	}
	for name, build := range forms {
		for _, s := range sizes {
			w, h := s[0], s[1]
			m := build(t, w, h)
			out := m.View()
			lines := strings.Split(out, "\n")
			if got := len(lines); got > h {
				t.Errorf("%s at %dx%d: view is %d lines tall, terminal is %d", name, w, h, got, h)
			}
			for i, ln := range lines {
				if lw := lipglossWidth(ln); lw > w {
					t.Errorf("%s at %dx%d: line %d is %d columns wide, terminal is %d", name, w, h, i, lw, w)
				}
			}
		}
	}
}

// TestGoSelectShowsAllThreeOptions (REV-001): every option is visible at
// realistic slugs AND widths. The round-1 defect: a wrapped multi-line option
// label pushed the tail option (Cancel) out of huh's row-per-option viewport —
// invisible for any slug over 34 chars at ANY width, and for a 4-char slug
// below ~102 columns. One-line labels + the command in the live title close it;
// this test pins the closure with this feature's own 36-char slug over the
// matrix the round-1 suite never exercised.
func TestGoSelectShowsAllThreeOptions(t *testing.T) {
	const longSlug = "launch-confirm-modal-and-fast-toggle" // 36 chars — the reviewer's probe
	// A realistic ROOT length too (REV-006): the `at <root>` tail wraps the title
	// taller, and the title is charged to the same clamped row budget as the
	// options — the round-1/2 suites never varied it.
	const longRoot = "/Users/bartlomiej/repos/gogo-workspaces/web"
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: longSlug, Title: "Long", Source: "web", Root: longRoot, Class: contract.ClassUnfinished, Status: "plan-accepted"},
	}}
	for _, s := range [][2]int{{200, 40}, {120, 30}, {100, 24}, {80, 24}, {70, 15}, {60, 15}} {
		w, h := s[0], s[1]
		m := NewWorkspace(repo, proj("app", src("web", longRoot)))
		m.hasClaude = true
		m = send(m, tea.WindowSizeMsg{Width: w, Height: h})
		m = openGoConfirm(t, m, longSlug)
		out := m.View()
		// "Launch" appears in both launch options (case-sensitive, so the slug's
		// own "launch-" never counts); Cancel is the tail option that used to
		// vanish; the title carries the exact command.
		if strings.Count(out, "Launch") < 2 || !strings.Contains(out, "Cancel") {
			t.Errorf("at %dx%d not all three options render (Launch x%d, Cancel %v):\n%s",
				w, h, strings.Count(out, "Launch"), strings.Contains(out, "Cancel"), out)
		}
		if !strings.Contains(out, "will run: claude") {
			t.Errorf("at %dx%d the title lost the command line:\n%s", w, h, out)
		}
	}
}

// TestForcedGoSelectShowsALaunchOptionAtTheModalMinimum (REV-001/REV-006): at
// the modal's own named minimum (60x15) WITH a FORCING description and a
// real-length repo root eating rows, the SEEDED option — always a launch — must
// still be visible (huh keeps the selected row in the viewport), so the confirm
// never renders option-less. One row BELOW the minimum the FR12 fallback owns
// the render and shows everything full-screen.
func TestForcedGoSelectShowsALaunchOptionAtTheModalMinimum(t *testing.T) {
	const longRoot = "/Users/bartlomiej/repos/gogo-workspaces/web"
	openForced := func(t *testing.T, w, h int) Model {
		t.Helper()
		repo := &contract.Repo{Features: []*contract.Feature{
			{Slug: "busy", Title: "Busy", Source: "web", Root: longRoot, Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"},
			{Slug: "next", Title: "Next", Source: "web", Root: longRoot, Class: contract.ClassUnfinished, Status: "plan-accepted"},
		}}
		m := NewWorkspace(repo, proj("app", src("web", longRoot, 1)))
		m.hasClaude = true
		m.sessions = []string{"gogo-go-busy"} // occupies the cap-1 source
		m = send(m, tea.WindowSizeMsg{Width: w, Height: h})
		m.colIdx = 0
		for i, f := range m.cols[0] {
			if f.Slug == "next" {
				m.cardIdx[0] = i
				break
			}
		}
		m = send(m, runes("M"))
		if m.mode != modeForm {
			t.Fatalf("M did not open the force confirm (mode=%d status=%q)", m.mode, m.status)
		}
		return m
	}

	t.Run("at the minimum the modal shows a launch option", func(t *testing.T) {
		m := openForced(t, modalMinTermW, modalMinTermH)
		out := m.View()
		if !strings.Contains(out, "FORCING past the source cap") {
			t.Fatalf("the force confirm lost its override note:\n%s", out)
		}
		if !strings.Contains(out, "Launch") {
			t.Errorf("no launch option renders at the modal minimum with a FORCING note:\n%s", out)
		}
	})

	t.Run("one row below the minimum the fallback shows everything", func(t *testing.T) {
		m := openForced(t, modalMinTermW, modalMinTermH-1)
		if got, want := m.View(), "\n"+m.form.View()+"\n"; got != want {
			t.Fatalf("below the minimum the view is not the full-screen form byte-for-byte")
		}
		out := m.View()
		if strings.Count(out, "Launch") < 2 || !strings.Contains(out, "Cancel") {
			t.Errorf("the full-screen fallback does not show all three options (Launch x%d, Cancel %v):\n%s",
				strings.Count(out, "Launch"), strings.Contains(out, "Cancel"), out)
		}
	})
}

// TestMergedShipModalAtTheMinimumRevealsConfirmOnEnter (TEST-001, phase ④): the
// merged-ship modal opens focused on the release-name Input, and at the modal's
// exact 60x15 minimum huh's focus-follow group viewport leaves the Launch/Cancel
// row below the fold on FIRST paint — one Enter (the hint bar's own "enter next")
// advances focus and reveals it. That is reachable-by-design (the forced
// go-Select's scrollable Cancel precedent accepted in REV-006), so this test pins
// the ACTUAL behavior at the boundary: what the first paint shows, that the
// reveal costs exactly one keypress which does NOT launch, and that one row
// below the minimum the FR12 fallback shows the action row at first paint.
func TestMergedShipModalAtTheMinimumRevealsConfirmOnEnter(t *testing.T) {
	const longRoot = "/Users/bartlomiej/repos/gogo-workspaces/web"
	openMergedShip := func(t *testing.T, w, h int) (Model, *recordingLauncher) {
		t.Helper()
		repo := &contract.Repo{Features: []*contract.Feature{
			{Slug: "alpha-one", Source: "web", Root: longRoot, Class: contract.ClassReadyToShip, Status: "awaiting-uat"},
			{Slug: "alpha-two", Source: "web", Root: longRoot, Class: contract.ClassReadyToShip, Status: "awaiting-uat"},
		}}
		m := NewWorkspace(repo, proj("app", src("web", longRoot)))
		m.hasClaude = true
		rl := &recordingLauncher{}
		m.launcher = rl.launch
		m = send(m, tea.WindowSizeMsg{Width: w, Height: h})
		m.selected = map[string]bool{selKey(m, "alpha-one"): true, selKey(m, "alpha-two"): true}
		m = send(m, runes("d"))
		if m.mode != modeForm {
			t.Fatalf("merged ship confirm did not open (mode=%d status=%q)", m.mode, m.status)
		}
		return m, rl
	}

	t.Run("at the minimum, first paint shows the input and one Enter reveals Launch/Cancel", func(t *testing.T) {
		m, rl := openMergedShip(t, modalMinTermW, modalMinTermH)
		out := m.View()
		for _, want := range []string{"Release name", "will run: claude"} {
			if !strings.Contains(out, want) {
				t.Errorf("first paint lacks %q:\n%s", want, out)
			}
		}
		m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // "enter next": Input -> Confirm
		if m.mode != modeForm {
			t.Fatalf("the first Enter closed the form (mode=%d) - it must only advance focus", m.mode)
		}
		if len(rl.calls) != 0 {
			t.Fatalf("the first Enter LAUNCHED (%d calls) - it must only advance focus", len(rl.calls))
		}
		out = m.View()
		if !strings.Contains(out, "Launch") || !strings.Contains(out, "Cancel") {
			t.Errorf("after one Enter the Launch/Cancel row is still not visible:\n%s", out)
		}
	})

	t.Run("one row below the minimum the fallback is the old form byte-for-byte", func(t *testing.T) {
		m, _ := openMergedShip(t, modalMinTermW, modalMinTermH-1)
		if got, want := m.View(), "\n"+m.form.View()+"\n"; got != want {
			t.Fatalf("below the minimum the view is not the full-screen form byte-for-byte")
		}
		// The fallback IS 0.35.0's render at this size — including huh's own
		// default-width (80) truncation of the Confirm row on a 60-col terminal
		// (a real terminal wraps that overlong line). Pre-existing, out of this
		// feature's scope: assert the content the old form actually shows.
		out := m.View()
		for _, want := range []string{"Release name", "will run: claude"} {
			if !strings.Contains(out, want) {
				t.Errorf("the full-screen fallback lacks %q:\n%s", want, out)
			}
		}
	})
}

// TestSmallTerminalFallsBackToFullScreen (FR12): below the named minimums — and
// on an unsized model — the launch confirm renders today's full-screen form
// BYTE-FOR-BYTE (the fallback is the old code path, not an approximation).
func TestSmallTerminalFallsBackToFullScreen(t *testing.T) {
	t.Run("46x9 terminal", func(t *testing.T) {
		m := NewWorkspace(modalRepo(), proj("app", src("web", "/r/web")))
		m.hasClaude = true
		m = send(m, tea.WindowSizeMsg{Width: 46, Height: 9})
		m = openGoConfirm(t, m, "next")
		if got, want := m.View(), "\n"+m.form.View()+"\n"; got != want {
			t.Errorf("small-terminal view is not the full-screen form byte-for-byte:\n%q", got)
		}
		if strings.Contains(m.View(), "╭") {
			t.Error("a box border rendered below the modal minimums")
		}
	})

	t.Run("unsized model", func(t *testing.T) {
		m := NewWorkspace(modalRepo(), proj("app", src("web", "/r/web")))
		m.hasClaude = true
		m = openGoConfirm(t, m, "next")
		if got, want := m.View(), "\n"+m.form.View()+"\n"; got != want {
			t.Errorf("unsized view is not the full-screen form byte-for-byte:\n%q", got)
		}
	})
}

// TestOverlayKeepsEveryFormLine (anti-clipping): every line of the form's own
// view appears in the composite. Anti-vacuity: the form must render at least 3
// lines, or the sweep proves nothing.
func TestOverlayKeepsEveryFormLine(t *testing.T) {
	m := sizedWorkspace(t, modalRepo(), proj("app", src("web", "/r/web")))
	m.hasClaude = true
	m = openGoConfirm(t, m, "next")
	formLines := strings.Split(strings.TrimRight(m.form.View(), "\n"), "\n")
	if len(formLines) < 3 {
		t.Fatalf("anti-vacuity: the form view has only %d lines", len(formLines))
	}
	out := m.View()
	for i, ln := range formLines {
		ln = strings.TrimRight(ln, " ")
		if ln == "" {
			continue
		}
		if !strings.Contains(out, ln) {
			t.Errorf("form line %d clipped from the composite: %q", i, ln)
		}
	}
}

// TestOverlayCentersAndPreservesBackgroundWidth is the pure overlayCenter unit
// (D3=A): a synthetic ANSI-styled background + a box → every output line is
// exactly termW columns, the height is exactly termH, the box sits centered,
// and the background's raw SGR state is STRIPPED (nothing can bleed into the
// box).
func TestOverlayCentersAndPreservesBackgroundWidth(t *testing.T) {
	const termW, termH = 60, 11
	bgLine := "\x1b[31mred-card\x1b[0m plain tail"
	bg := strings.TrimSuffix(strings.Repeat(bgLine+"\n", termH), "\n")
	box := modalBoxStyle.Render("hello\nworld")
	out := overlayCenter(bg, box, termW, termH)
	lines := strings.Split(out, "\n")
	if len(lines) != termH {
		t.Fatalf("composite is %d lines, want exactly %d", len(lines), termH)
	}
	for i, ln := range lines {
		if w := lipglossWidth(ln); w != termW {
			t.Errorf("line %d is %d columns, want exactly %d", i, w, termW)
		}
	}
	if strings.Contains(out, "\x1b[31m") {
		t.Error("the background's raw SGR state survived the strip (D3=A) - colour can bleed into the box")
	}
	if !strings.Contains(out, "red-card") {
		t.Error("the stripped background text vanished")
	}
	// Centered: the box row holding "hello" starts at ~ (termW-boxW)/2.
	boxW := lipglossWidth(strings.Split(box, "\n")[0])
	wantLeft := (termW - boxW) / 2
	for _, ln := range lines {
		if i := strings.Index(ln, "hello"); i >= 0 {
			// +2 for the border glyph + padding column inside the box.
			if got := lipglossWidth(ln[:i]); got < wantLeft || got > wantLeft+modalChromeW {
				t.Errorf("box content starts at column %d, want ~%d (centered)", got, wantLeft)
			}
			return
		}
	}
	t.Error("the box's content never rendered in the composite")
}

// TestLaunchConfirmReturnsToItsOrigin (FR15): esc AND the Cancel option both
// land back on the recorded formOrigin.
func TestLaunchConfirmReturnsToItsOrigin(t *testing.T) {
	t.Run("esc", func(t *testing.T) {
		m := sizedWorkspace(t, modalRepo(), proj("app", src("web", "/r/web")))
		m.hasClaude = true
		m = openGoConfirm(t, m, "next")
		m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEsc})
		if m.mode != modeBoard {
			t.Errorf("esc left mode=%d, want the recorded board origin", m.mode)
		}
	})
	// The Cancel-option half is TestGoConfirmCancelOptionLaunchesNothing above.
}

// lipglossWidth is a tiny local alias so the width assertions read as what they
// measure (lipgloss.Width IS ansi.StringWidth — the same ruler the overlay cuts
// with).
func lipglossWidth(s string) int { return lipgloss.Width(s) }
