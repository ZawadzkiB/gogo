package tui

// Session-binding ops (0.32.0): board/drill P (plan in an attached session),
// board K (the promoted kill picker + the pickerOrigin cancel fix), R (re-assign
// a live session onto the item it is really driving), the unbound-session count,
// and the footer chips that follow the symptom. Driven through Update with the
// launcher/killer/renamer seams — fire-exactly-once assertions, no real tmux.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	tea "github.com/charmbracelet/bubbletea"
)

// recordingRenamer records every (old, base) pair the R adopt path asks to
// rename, returning the base as the final name — the renamer-seam analog of
// recordingKiller/recordingLauncher.
type recordingRenamer struct{ calls [][2]string }

func (r *recordingRenamer) rename(old, base string) (string, error) {
	r.calls = append(r.calls, [2]string{old, base})
	return base, nil
}

// focusSlug points the board cursor at the card with slug in column col.
func focusSlug(t *testing.T, m *Model, col int, slug string) *contract.Feature {
	t.Helper()
	m.colIdx = col
	for j, f := range m.cols[col] {
		if f.Slug == slug {
			m.cardIdx[col] = j
			return f
		}
	}
	t.Fatalf("slug %q not in column %d", slug, col)
	return nil
}

// --- bindAction (D4=A) --------------------------------------------------------

// TestBindAction: the action a moved session takes is DERIVED from the target's
// status — RunnableStatus → go (counted by the source cap), else plan — never
// preserved from the old name (a preserved gogo-done-<B> would leave a real
// build invisible to the cap).
func TestBindAction(t *testing.T) {
	cases := []struct {
		status string
		want   launch.Action
	}{
		{"plan-accepted", launch.ActionGo},
		{"implementing", launch.ActionGo},
		{"reviewing", launch.ActionGo},
		{"testing", launch.ActionGo},
		{"awaiting-plan-acceptance", launch.ActionPlan},
		{"waiting-for-user", launch.ActionPlan},
		{"awaiting-uat", launch.ActionPlan},
		{"", launch.ActionPlan},
	}
	for _, c := range cases {
		if got := bindAction(&contract.Feature{Status: c.status}); got != c.want {
			t.Errorf("bindAction(%q) = %v, want %v", c.status, got, c.want)
		}
	}
	if got := bindAction(nil); got != launch.ActionPlan {
		t.Errorf("bindAction(nil) = %v, want plan", got)
	}
}

// --- P: plan this item in an attached session (FR1) ---------------------------

// TestPlanSessionLaunchesOnceThenAttaches: P on a non-terminal card opens a
// confirm; Enter (forward move → affirmative default) fires the launcher EXACTLY
// once with `/gogo:plan <slug>` anchored at the card's OWN root, then the model
// attaches — the chosen session name is the status observable (attach has no
// seam, matching the attach-picker tests).
func TestPlanSessionLaunchesOnceThenAttaches(t *testing.T) {
	m, rl := launchable(t)
	f := focusSlug(t, &m, 0, "unfinished")

	nm, _ := m.Update(runes("P"))
	m = nm.(Model)
	if m.mode != modeForm || m.pendingPlanSession == nil {
		t.Fatalf("P did not open the plan-session confirm (mode=%d pending=%v)", m.mode, m.pendingPlanSession)
	}
	m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if len(rl.calls) != 1 {
		t.Fatalf("launcher called %d times, want exactly 1", len(rl.calls))
	}
	in := rl.calls[0]
	if in.Action != launch.ActionPlan || in.Command != "/gogo:plan unfinished" {
		t.Errorf("launched %+v, want plan /gogo:plan unfinished", in)
	}
	if in.Root != f.Root {
		t.Errorf("launch root = %q, want the card's own root %q", in.Root, f.Root)
	}
	if !strings.Contains(m.status, "attaching gogo-plan-unfinished") {
		t.Errorf("status = %q, want the attach to name gogo-plan-unfinished", m.status)
	}
	if m.pendingPlanSession != nil {
		t.Errorf("pendingPlanSession not cleared: %v", m.pendingPlanSession)
	}
}

// TestPlanSessionRefusals: each P refusal names its reason — a terminal card has
// nothing to plan; no claude blocks; and an existing live gogo-plan-<slug> is
// ATTACHED instead of duplicated (no launch).
func TestPlanSessionRefusals(t *testing.T) {
	t.Run("terminal card", func(t *testing.T) {
		m, rl := launchable(t)
		focusSlug(t, &m, 0, "aborted")
		nm, _ := m.Update(runes("P"))
		m = nm.(Model)
		if !strings.Contains(m.status, "nothing to plan") || m.statusLevel != statusLevelWarn {
			t.Errorf("status = %q (level %d), want a named 'nothing to plan' block", m.status, m.statusLevel)
		}
		if len(rl.calls) != 0 || m.mode != modeBoard {
			t.Errorf("terminal P launched anyway (calls=%d mode=%d)", len(rl.calls), m.mode)
		}
	})

	t.Run("no claude", func(t *testing.T) {
		m, rl := launchable(t)
		m.hasClaude = false
		focusSlug(t, &m, 0, "unfinished")
		nm, _ := m.Update(runes("P"))
		m = nm.(Model)
		if !strings.Contains(m.status, "claude CLI not on PATH") {
			t.Errorf("status = %q, want the missing-claude block", m.status)
		}
		if len(rl.calls) != 0 {
			t.Errorf("launched with no claude: %d calls", len(rl.calls))
		}
	})

	t.Run("already live attaches instead of duplicating", func(t *testing.T) {
		m, rl := launchable(t)
		focusSlug(t, &m, 0, "unfinished")
		m.sessions = []string{"gogo-plan-unfinished"}
		nm, _ := m.Update(runes("P"))
		m = nm.(Model)
		if len(rl.calls) != 0 {
			t.Fatalf("P minted a second session for a slug with a live plan session (%d launches)", len(rl.calls))
		}
		if !strings.Contains(m.status, "attaching gogo-plan-unfinished") {
			t.Errorf("status = %q, want an attach to the EXISTING session", m.status)
		}
	})
}

// --- K on the board (FR2) -----------------------------------------------------

// TestBoardKill: the drill's kill path, promoted to the board — 0 sessions → a
// blocked hint; ≥2 → the picker, killing exactly the chosen session by exact
// name; and completion returns to the BOARD, not a drill.
func TestBoardKill(t *testing.T) {
	t.Run("no session", func(t *testing.T) {
		m := newModel(t)
		m.hasTmux = true
		focusSlug(t, &m, 1, "inprogress")
		nm, _ := m.Update(runes("K"))
		m = nm.(Model)
		if !strings.Contains(m.status, "no live session to kill") {
			t.Errorf("status = %q, want the 0-session block", m.status)
		}
	})

	t.Run("picker kills exactly the chosen one and returns to the board", func(t *testing.T) {
		m := newModel(t)
		m.hasTmux = true
		m.registry = fakeReg(nil)
		rk := &recordingKiller{}
		m.killer = rk.kill
		focusSlug(t, &m, 1, "inprogress")
		m.sessions = []string{"gogo-go-inprogress", "gogo-plan-inprogress"}

		nm, _ := m.Update(runes("K"))
		m = nm.(Model)
		if m.mode != modeForm || m.pendingKill == nil || m.pickerOrigin != modeBoard {
			t.Fatalf("board K did not open a board-origin kill picker (mode=%d origin=%d)", m.mode, m.pickerOrigin)
		}
		m = keyPress(t, m, runes("j"))                     // 2nd option
		m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // select + submit
		if len(rk.calls) != 1 || rk.calls[0] != "gogo-plan-inprogress" {
			t.Fatalf("killed %v, want exactly [gogo-plan-inprogress]", rk.calls)
		}
		if m.mode != modeBoard {
			t.Errorf("board K completion left mode=%d, want the board (the pre-fix code landed in the drill)", m.mode)
		}
	})

	// FR2's distinguishing clause (REV-002): `K` works on a focused SHIPPED /
	// changelog card — where a lingering session shows up — and the board-level
	// 1-session arm drives the Enter-safe confirm. One subtest pins both: the
	// exact session name is asserted (not a bare call count) so a fixture that
	// stops resolving fails loudly, and a mutation that refuses terminal cards
	// inside killFeature now breaks the suite.
	t.Run("shipped changelog card: single-session confirm kills exactly it", func(t *testing.T) {
		m := newModel(t)
		m.hasTmux = true
		m.registry = fakeReg(nil)
		rk := &recordingKiller{}
		m.killer = rk.kill
		m.colIdx = 3 // changelog — shipped cards
		f := m.focusedCard()
		if f == nil {
			t.Fatal("no changelog card focused")
		}
		session := "gogo-done-" + f.Slug
		m.sessions = []string{session}

		nm, _ := m.Update(runes("K"))
		m = nm.(Model)
		if m.mode != modeForm || m.pendingKill == nil {
			t.Fatalf("board K on a shipped card did not open the kill confirm (mode=%d pending=%v)", m.mode, m.pendingKill)
		}
		m = keyPress(t, m, runes("y")) // pick Kill and complete
		if len(rk.calls) != 1 || rk.calls[0] != session {
			t.Fatalf("killer calls = %v, want exactly [%s]", rk.calls, session)
		}
		if m.mode != modeBoard {
			t.Errorf("after the board kill, mode=%d, want the board", m.mode)
		}
	})

	// The FR2 in-passing fix: a board-originated picker's CANCEL must return to
	// the board even when a STALE drill exists from an earlier visit — the old
	// `m.drill != nil` inference landed the user in a drill they never opened.
	t.Run("cancel with a stale drill returns to the board", func(t *testing.T) {
		m := newModel(t)
		m.hasTmux = true
		m.registry = fakeReg(nil)
		focusSlug(t, &m, 1, "inprogress")
		m = send(m, tea.KeyMsg{Type: tea.KeyEnter}) // drill in…
		m = send(m, tea.KeyMsg{Type: tea.KeyEsc})   // …and back: m.drill is now stale
		if m.mode != modeBoard || m.drill == nil {
			t.Fatalf("precondition failed: mode=%d drill=%v", m.mode, m.drill)
		}
		m.sessions = []string{"gogo-go-inprogress", "gogo-plan-inprogress"}
		nm, _ := m.Update(runes("K"))
		m = nm.(Model)
		m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEsc})
		if m.mode != modeBoard {
			t.Errorf("board-origin kill cancel left mode=%d, want the board (pickerOrigin fix)", m.mode)
		}
	})
}

// --- R: re-assign (FR3) -------------------------------------------------------

// adoptable builds a board model with the renamer seam + one live foreign-named
// session anchored at the target card's own root.
func adoptable(t *testing.T, col int, slug, session string) (Model, *recordingRenamer, *contract.Feature) {
	t.Helper()
	m := newModel(t)
	m.hasTmux = true
	rr := &recordingRenamer{}
	m.renamer = rr.rename
	f := focusSlug(t, &m, col, slug)
	m.sessions = []string{session}
	m.sessionMeta = []launch.SessionMeta{{Name: session, Path: filepath.Clean(m.rootFor(f)), Attached: true}}
	return m, rr, f
}

// TestAdoptRenamesToTheDerivedAction: choosing a session in the R picker renames
// it to gogo-<derived action>-<slug> — a runnable target mints a BUILD name (the
// cap counts it), a pre-acceptance target an authoring one — and the picker's
// description SHOWS the resulting name (D4).
func TestAdoptRenamesToTheDerivedAction(t *testing.T) {
	t.Run("runnable target mints a build session", func(t *testing.T) {
		m, rr, _ := adoptable(t, 1, "inprogress", "gogo-go-item-a") // status reviewing → go
		nm, _ := m.Update(runes("R"))
		m = nm.(Model)
		if m.mode != modeForm || m.pendingAdopt == nil {
			t.Fatalf("R did not open the adopt picker (mode=%d)", m.mode)
		}
		v := m.form.View()
		if !strings.Contains(v, "gogo-go-inprogress") {
			t.Errorf("picker does not SHOW the resulting name (D4):\n%s", v)
		}
		// REV-005: a session with a live client reads `attached` in its row, so
		// renaming it out from under a human is a deliberate act.
		if !strings.Contains(v, "attached") {
			t.Errorf("picker row does not mark the attached session:\n%s", v)
		}
		m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // first option = the session
		if len(rr.calls) != 1 || rr.calls[0] != [2]string{"gogo-go-item-a", "gogo-go-inprogress"} {
			t.Fatalf("renamer calls = %v, want exactly [gogo-go-item-a → gogo-go-inprogress]", rr.calls)
		}
		if !strings.Contains(m.status, "re-assigned gogo-go-item-a → gogo-go-inprogress") {
			t.Errorf("status = %q, want the re-assign confirmation", m.status)
		}
		if m.mode != modeBoard {
			t.Errorf("adopt completion left mode=%d, want the board", m.mode)
		}
	})

	t.Run("non-runnable target mints an authoring session", func(t *testing.T) {
		m, rr, _ := adoptable(t, 0, "malformed", "gogo-done-item-a") // no status → plan
		nm, _ := m.Update(runes("R"))
		m = keyPress(t, nm.(Model), tea.KeyMsg{Type: tea.KeyEnter})
		if len(rr.calls) != 1 || rr.calls[0][1] != "gogo-plan-malformed" {
			t.Fatalf("renamer calls = %v, want the derived gogo-plan-malformed base", rr.calls)
		}
	})
}

// TestAdoptRefusals: every R refusal is NAMED — terminal target (and the real
// move it points at), foreign session anchor, already-bound, no sessions, no
// tmux — and none of them renames anything.
func TestAdoptRefusals(t *testing.T) {
	t.Run("terminal target names the real move", func(t *testing.T) {
		m := newModel(t)
		m.hasTmux = true
		rr := &recordingRenamer{}
		m.renamer = rr.rename
		m.colIdx = 3 // changelog — shipped cards
		nm, _ := m.Update(runes("R"))
		m = nm.(Model)
		if !strings.Contains(m.status, "nothing to drive") || !strings.Contains(m.status, "press R on the card") {
			t.Errorf("status = %q, want 'nothing to drive' naming the unblock", m.status)
		}
		if len(rr.calls) != 0 || m.mode != modeBoard {
			t.Errorf("terminal R did something (calls=%v mode=%d)", rr.calls, m.mode)
		}
	})

	t.Run("foreign anchor", func(t *testing.T) {
		m, rr, _ := adoptable(t, 1, "inprogress", "gogo-go-item-a")
		m.sessionMeta[0].Path = "/somewhere/else" // anchored at another repo
		nm, _ := m.Update(runes("R"))
		m = keyPress(t, nm.(Model), tea.KeyMsg{Type: tea.KeyEnter})
		if len(rr.calls) != 0 {
			t.Fatalf("foreign-anchor adopt renamed anyway: %v", rr.calls)
		}
		if !strings.Contains(m.status, "anchored at /somewhere/else") || m.statusLevel != statusLevelWarn {
			t.Errorf("status = %q (level %d), want the named anchor refusal", m.status, m.statusLevel)
		}
	})

	t.Run("already bound", func(t *testing.T) {
		m, rr, _ := adoptable(t, 1, "inprogress", "gogo-go-inprogress")
		nm, _ := m.Update(runes("R"))
		m = keyPress(t, nm.(Model), tea.KeyMsg{Type: tea.KeyEnter})
		if len(rr.calls) != 0 {
			t.Fatalf("already-bound adopt renamed anyway: %v", rr.calls)
		}
		if !strings.Contains(m.status, "already bound") {
			t.Errorf("status = %q, want the already-bound refusal", m.status)
		}
	})

	t.Run("no sessions", func(t *testing.T) {
		m := newModel(t)
		m.hasTmux = true
		focusSlug(t, &m, 1, "inprogress")
		nm, _ := m.Update(runes("R"))
		m = nm.(Model)
		if !strings.Contains(m.status, "no live gogo-* session") {
			t.Errorf("status = %q, want the no-sessions block", m.status)
		}
	})

	t.Run("no tmux", func(t *testing.T) {
		m := newModel(t)
		m.hasTmux = false
		focusSlug(t, &m, 1, "inprogress")
		m.sessionMeta = []launch.SessionMeta{{Name: "gogo-go-x", Path: "/x"}}
		nm, _ := m.Update(runes("R"))
		m = nm.(Model)
		if !strings.Contains(m.status, "tmux not installed") {
			t.Errorf("status = %q, want the no-tmux block", m.status)
		}
	})

	t.Run("cancel renames nothing", func(t *testing.T) {
		m, rr, _ := adoptable(t, 1, "inprogress", "gogo-go-item-a")
		nm, _ := m.Update(runes("R"))
		m = keyPress(t, nm.(Model), tea.KeyMsg{Type: tea.KeyEsc})
		if len(rr.calls) != 0 || m.mode != modeBoard {
			t.Errorf("cancelled adopt renamed/misrouted (calls=%v mode=%d)", rr.calls, m.mode)
		}
	})
}

// --- unbound sessions (FR4) ---------------------------------------------------

// TestUnboundSessions: a live gogo-* session anchored in a board root that
// attributes to no feature is unbound; a bound one and one anchored in a repo
// this board does not show are not.
func TestUnboundSessions(t *testing.T) {
	features := []*contract.Feature{{Slug: "auth"}}
	metas := []launch.SessionMeta{
		{Name: "gogo-plan-catalogue-side-of-the-matching-engine", Path: "/repo"},
		{Name: "gogo-go-auth", Path: "/repo"},       // bound → not counted
		{Name: "gogo-go-elsewhere", Path: "/other"}, // another board's repo → not counted
	}
	got := unboundSessions(metas, features, map[string]bool{"/repo": true})
	if len(got) != 1 || got[0].Name != "gogo-plan-catalogue-side-of-the-matching-engine" {
		t.Fatalf("unboundSessions = %+v, want exactly the unbound plans-tab spawn", got)
	}
}

// TestUnboundCountOnStatusLine: the idle board status line reports a session
// bound to nothing (FR4) instead of leaving it invisible.
func TestUnboundCountOnStatusLine(t *testing.T) {
	m := newModel(t)
	m.sessionMeta = []launch.SessionMeta{{Name: "gogo-plan-some-plan-title", Path: filepath.Clean(fixtureRoot)}}
	out := m.View()
	if !strings.Contains(out, "1 unbound session") {
		t.Errorf("board view does not count the unbound session:\n%s", out)
	}

	// Bound (or foreign-root) sessions never inflate the count.
	m.sessionMeta = []launch.SessionMeta{
		{Name: "gogo-go-inprogress", Path: filepath.Clean(fixtureRoot)},
		{Name: "gogo-go-foreign", Path: "/somewhere/else"},
	}
	m.sessions = []string{"gogo-go-inprogress"}
	if out := m.View(); strings.Contains(out, "unbound") {
		t.Errorf("bound/foreign sessions counted as unbound:\n%s", out)
	}
}

// --- footer chips follow the symptom (FR4) ------------------------------------

func TestSessionBindingFooterChips(t *testing.T) {
	t.Run("stalled card offers adopt", func(t *testing.T) {
		m := newModel(t)
		focusSlug(t, &m, 1, "inprogress") // reviewing + no session → · stalled
		if out := m.View(); !strings.Contains(out, "[R] adopt") {
			t.Errorf("stalled card footer missing [R] adopt:\n%s", out)
		}
	})

	t.Run("shipped card holding a session offers kill — and never a lying re-assign", func(t *testing.T) {
		m := newModel(t)
		m.colIdx = 3
		f := m.focusedCard()
		if f == nil {
			t.Fatal("no changelog card focused")
		}
		m.sessions = []string{"gogo-done-" + f.Slug}
		out := m.View()
		if !strings.Contains(out, "[K] kill") {
			t.Errorf("shipped card with a live session missing [K] kill:\n%s", out)
		}
		// REV-003: R refuses a terminal TARGET by design (FR3), so a shipped card
		// must never advertise `[R] re-assign` — a chip that is a guaranteed
		// bounce. The re-assign move lives on the card the session should DRIVE.
		if strings.Contains(out, "[R] re-assign") {
			t.Errorf("shipped card advertises [R] re-assign, a key that always refuses there:\n%s", out)
		}
		if strings.Contains(out, "[P] plan") {
			t.Errorf("terminal card offers [P] plan:\n%s", out)
		}
	})

	t.Run("non-terminal card offers plan", func(t *testing.T) {
		m := newModel(t)
		focusSlug(t, &m, 0, "unfinished")
		if out := m.View(); !strings.Contains(out, "[P] plan") {
			t.Errorf("non-terminal card footer missing [P] plan:\n%s", out)
		}
	})
}
