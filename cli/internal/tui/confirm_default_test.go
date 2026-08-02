package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/plans"
	tea "github.com/charmbracelet/bubbletea"
)

// --- the CONFIRM-DEFAULT CONVENTION (TEST-001) --------------------------------
//
// Every gogo confirm seeds binding.confirm explicitly, and WHICH value it seeds is
// a safety rule, not a style choice:
//
//	forward pipeline move (m: launch / spawn / accept)  -> confirm: true  -> Enter SUBMITS
//	destructive / irreversible (delete, kill)           -> confirm: false -> Enter is SAFE
//
// Both halves are asserted here, in one file, so the asymmetry reads as one
// deliberate rule rather than two unrelated defaults - and so a future "let's make
// these consistent" change fails loudly whichever direction it goes.
//
// Every case drives a BARE ENTER through the real huh lifecycle (keyPress pumps the
// async NextField/nextGroup/StateCompleted chain) and asserts the real side effect:
// an injected launcher/killer fired or did not, and the store/disk changed or did
// not. Asserting binding.confirm alone would not prove the keystroke's outcome.

// TestConfirmDefaultForwardMovesSubmitOnEnter is the FORWARD half: a bare Enter on
// a plans-tab `m` confirm must PROCEED, exactly as it does on the board. Before the
// fix the plans-tab bindings were unseeded, so the same keystroke that launches on
// the board silently cancelled here ("m -> enter should confirm").
func TestConfirmDefaultForwardMovesSubmitOnEnter(t *testing.T) {
	t.Run("board m launches", func(t *testing.T) {
		// The canonical reference the plans tab must match (move.go seeds confirm: true).
		m, rl := launchable(t)
		m.colIdx = 0 // the plan column, focused on an unfinished card
		nm, _ := m.Update(runes("m"))
		m = nm.(Model)
		if m.mode != modeForm {
			t.Fatalf("board m did not open a confirm (mode=%d)", m.mode)
		}
		m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter})
		if len(rl.calls) != 1 {
			t.Fatalf("bare Enter on the BOARD confirm fired the launcher %d times, want 1", len(rl.calls))
		}
	})

	t.Run("plans-tab m spawns", func(t *testing.T) {
		seedDataHome(t)
		p, _ := plans.New("app", "Rollout", "roll it out")
		plans.AddTarget("app", p.ID, "web")
		plans.MarkReady("app", p.ID) // ready -> `m` is the go/spawn move
		m := NewWorkspace(&contract.Repo{}, proj("app", src("web", "/r/web")))
		m.hasClaude = true
		m.tab = tabPlans
		m.planColIdx = 1 // the ready column
		calls := 0
		m.launcher = func(string, launch.Intent) (launch.Result, error) {
			calls++
			return launch.Result{Mode: "tmux"}, nil
		}

		m = send(m, runes("m"))
		if m.pendingPlanSpawn == nil || m.mode != modeForm {
			t.Fatalf("plans-tab m did not open the spawn confirm (mode=%d)", m.mode)
		}
		m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // BARE ENTER - must spawn

		if calls != 1 {
			t.Fatalf("bare Enter on the plans-tab spawn confirm fired the launcher %d times, want 1 - Enter cancelled instead of confirming", calls)
		}
		if got, _ := plans.Get("app", p.ID); got.Status != plans.StatusActive || len(got.Members) != 1 {
			t.Errorf("after a bare-Enter spawn the plan = %+v, want active with 1 member", got)
		}
		if m.status == "cancelled" {
			t.Errorf("bare Enter reported %q - the affirmative was not the default", m.status)
		}
	})

	t.Run("plans-tab m accepts the project-UAT", func(t *testing.T) {
		seedDataHome(t)
		p, _ := plans.New("app", "Cross-repo migration", "body")
		plans.AddTarget("app", p.ID, "web")
		plans.AddMember("app", p.ID, plans.Member{Source: "web", SlugHint: "web-item"})
		plans.SetStatus("app", p.ID, plans.StatusActive)
		// Every member shipped -> the plan sits at the project-UAT gate.
		repo := &contract.Repo{Features: []*contract.Feature{
			// Correlations is load-bearing: memberFeature matches a member by
			// (Source, plan id), so without it the member is never FOUND and the gate
			// would refuse for the wrong reason instead of opening the confirm.
			{Slug: "web-item", Source: "web", Root: "/r/web", Class: contract.ClassShipped, Status: "shipped", Correlations: []string{p.ID}},
		}}
		m := sizedWorkspace(t, repo, proj("app", src("web", "/r/web")))
		m = tab(m)
		m.planColIdx = 2 // the active column

		m = send(m, runes("m"))
		if m.pendingPlanDone == nil || m.mode != modeForm {
			t.Fatalf("plans-tab m did not open the project-UAT confirm (mode=%d)", m.mode)
		}
		m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // BARE ENTER - must accept

		if got, _ := plans.Get("app", p.ID); got.Status != plans.StatusDone {
			t.Errorf("after a bare-Enter project-UAT accept the plan is %q, want done - Enter cancelled instead of confirming", got.Status)
		}
		if m.status == "cancelled" {
			t.Errorf("bare Enter reported %q - the affirmative was not the default", m.status)
		}
	})
}

// TestConfirmDefaultDestructiveActionsNeedDeliberateChoice is the DESTRUCTIVE half:
// a bare Enter must NOT delete and must NOT kill. This is the guard that stops the
// forward-move fix above from being "helpfully" generalised to every confirm.
func TestConfirmDefaultDestructiveActionsNeedDeliberateChoice(t *testing.T) {
	t.Run("delete stays safe on Enter", func(t *testing.T) {
		root := writableRepo(t)
		m := New(root)
		nm, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
		m = nm.(Model)

		nm, _ = m.deleteFocused()
		m = nm.(Model)
		if m.pendingDelete == nil {
			t.Fatalf("x did not open the delete confirm")
		}
		m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // BARE ENTER - must NOT delete

		if _, err := os.Stat(filepath.Join(root, ".gogo", "work", "feature-doomed")); err != nil {
			t.Errorf("a bare Enter DELETED the feature (%v) - a destructive confirm must default to Cancel", err)
		}
		if len(m.cols[0]) != 1 {
			t.Errorf("the card left the board on a bare Enter: %d cards", len(m.cols[0]))
		}
		entries, _ := os.ReadDir(filepath.Join(root, ".gogo", "trash"))
		if len(entries) != 0 {
			t.Errorf("a bare Enter moved %d entr(ies) to trash", len(entries))
		}
	})

	t.Run("kill stays safe on Enter", func(t *testing.T) {
		f := &contract.Feature{Slug: "busy", Source: "web", Root: "/r/web", Class: contract.ClassInProgress, Status: "implementing"}
		m := sizedWorkspace(t, &contract.Repo{Features: []*contract.Feature{f}}, proj("app", src("web", "/r/web")))
		m.hasTmux = true
		m.sessions = []string{"gogo-go-busy"}
		killed := 0
		m.killer = func(string) error { killed++; return nil }
		m.openDrill(f)

		nm, _ := m.killDrill()
		m = nm.(Model)
		if m.pendingKill == nil || m.mode != modeForm {
			t.Fatalf("K did not open the kill confirm (mode=%d)", m.mode)
		}
		m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // BARE ENTER - must NOT kill

		if killed != 0 {
			t.Errorf("a bare Enter KILLED %d session(s) - a destructive confirm must default to Cancel", killed)
		}
	})
}

// TestConfirmDefaultsAreAlwaysExplicit is the structural guard: an UNSEEDED
// `&formBinding{}` behind a confirm is what caused TEST-001 (it silently means
// Cancel, which is right for a delete and wrong for a spawn). Every confirm-bearing
// constructor must state its default, so the choice is visible at the call site
// rather than inherited from Go's zero value.
func TestConfirmDefaultsAreAlwaysExplicit(t *testing.T) {
	// The constructors that build a confirm, and the default each MUST seed.
	want := map[string]bool{
		"startFormOverriding": true,  // board m/M - forward move
		"startPlanSpawnForm":  true,  // plans-tab m on ready - forward move
		"startPlanDoneForm":   true,  // plans-tab m on active - forward move
		"startDeleteForm":     false, // destructive
		"startKillForm":       false, // destructive
	}
	for fn, affirmative := range want {
		body, file := funcBody(t, []string{"move.go", "plans_tab.go", "delete.go", "update.go"}, fn)
		if body == "" {
			t.Errorf("could not find %s in the package sources - did it move or get renamed?", fn)
			continue
		}
		seeded := "&formBinding{confirm: " + boolLit(affirmative) + "}"
		if !containsAny(body, seeded, "&formBinding{confirm: "+boolLit(affirmative)+",") {
			t.Errorf("%s (%s) does not seed %s\n"+
				"  the confirm-default convention (TEST-001, canonical statement in move.go startFormOverriding):\n"+
				"    a FORWARD pipeline move (launch/spawn/accept) seeds confirm: true  -> Enter submits\n"+
				"    a DESTRUCTIVE action (delete/kill)            seeds confirm: false -> Enter is safe\n"+
				"  an UNSEEDED &formBinding{} is what caused TEST-001 - state the default explicitly",
				fn, file, seeded)
		}
	}

	// The go confirm's D1=B shape (0.36.0, REV-003): the Select's REAL default is
	// binding.launchMode, not binding.confirm (launchConfirmed never reads confirm
	// when launchMode is set) - so the convention's structural pin must bite on the
	// field that actually decides. startFormOverriding must seed a NON-cancel
	// launchMode: the full-pipeline base, optionally upgraded to fast by the
	// source's config - and never Cancel, which would make a bare Enter silently
	// abort the confirmation the user deliberately opened.
	body, file := funcBody(t, []string{"move.go"}, "startFormOverriding")
	if body == "" {
		t.Fatal("could not find startFormOverriding in move.go")
	}
	if !strings.Contains(body, "m.binding.launchMode = goLaunchFull") {
		t.Errorf("startFormOverriding (%s) does not seed the go Select's launchMode to goLaunchFull\n"+
			"  the D1=B half of the confirm-default convention: the seeded option IS a launch,\n"+
			"  so a bare Enter still submits the confirmation the user deliberately opened", file)
	}
	if strings.Contains(body, "m.binding.launchMode = goLaunchCancel") {
		t.Errorf("startFormOverriding (%s) seeds the go Select's launchMode to goLaunchCancel\n"+
			"  - a bare Enter would silently cancel a forward pipeline move (the TEST-001 shape)", file)
	}
}

// funcBody returns the source text of function fn from the first of files that
// declares it, plus that file's name. Read from disk (not reflection) because the
// thing under test is what the CONSTRUCTOR writes, which only the source shows.
func funcBody(t *testing.T, files []string, fn string) (string, string) {
	t.Helper()
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		body := string(src)
		marker := ") " + fn + "("
		i := strings.Index(body, marker)
		if i < 0 {
			continue
		}
		rest := body[i:]
		// The function ends at the first line that is exactly "}" at column 0.
		if end := strings.Index(rest, "\n}\n"); end >= 0 {
			return rest[:end], f
		}
		return rest, f
	}
	return "", ""
}

func boolLit(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
