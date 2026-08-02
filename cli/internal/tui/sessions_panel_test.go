package tui

// The S sessions panel (0.33.0 FR3-FR5): S opens modeSessions from board AND
// drill, lists every live gogo-* session (adoptRow rows), R re-assigns the
// focused session onto a picked work item through the ONE shared reassign core,
// K closes it behind the destructive confirm, and esc/q return to wherever S was
// pressed. Driven through Update with the renamer/killer seams — no real tmux.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	tea "github.com/charmbracelet/bubbletea"
)

// panelModel builds a board model with the seams + a live session set: one
// bound to the in-progress card, one bound to a shipped card, one unbound.
func panelModel(t *testing.T) (Model, *recordingRenamer, *recordingKiller) {
	t.Helper()
	m := newModel(t)
	m.hasTmux = true
	m.registry = fakeReg(nil)
	rr := &recordingRenamer{}
	m.renamer = rr.rename
	rk := &recordingKiller{}
	m.killer = rk.kill
	root := filepath.Clean(fixtureRoot)
	m.sessions = []string{"gogo-go-inprogress", "gogo-done-ready", "gogo-plan-some-title"}
	m.sessionMeta = []launch.SessionMeta{
		{Name: "gogo-go-inprogress", Path: root, Attached: true},
		{Name: "gogo-done-ready", Path: root},
		{Name: "gogo-plan-some-title", Path: root},
	}
	return m, rr, rk
}

// TestSessionsPanelOpensFromBoardAndDrill (FR3.1/FR3.3, D2=C): S opens the
// panel from both surfaces, lists every session, and esc returns to the opener.
func TestSessionsPanelOpensFromBoardAndDrill(t *testing.T) {
	t.Run("from the board", func(t *testing.T) {
		m, _, _ := panelModel(t)
		nm, _ := m.Update(runes("S"))
		m = nm.(Model)
		if m.mode != modeSessions {
			t.Fatalf("S did not open the sessions panel (mode=%d)", m.mode)
		}
		out := m.View()
		for _, want := range []string{"gogo-go-inprogress", "gogo-done-ready", "gogo-plan-some-title", "attached", sessionsKeysLine} {
			if !strings.Contains(out, want) {
				t.Errorf("panel missing %q:\n%s", want, out)
			}
		}
		m = send(m, tea.KeyMsg{Type: tea.KeyEsc})
		if m.mode != modeBoard {
			t.Errorf("esc from a board-opened panel landed in mode=%d, want the board", m.mode)
		}
	})

	t.Run("from the drill", func(t *testing.T) {
		m, _, _ := panelModel(t)
		focusSlug(t, &m, 1, "inprogress")
		m = send(m, tea.KeyMsg{Type: tea.KeyEnter}) // drill in
		if m.mode != modeDrill {
			t.Fatalf("precondition: not drilled (mode=%d)", m.mode)
		}
		nm, _ := m.Update(runes("S"))
		m = nm.(Model)
		if m.mode != modeSessions {
			t.Fatalf("drill S did not open the panel (mode=%d)", m.mode)
		}
		m = send(m, runes("q"))
		if m.mode != modeDrill {
			t.Errorf("q from a drill-opened panel landed in mode=%d, want the drill (D2=C)", m.mode)
		}
	})
}

// TestSessionsPanelAlwaysOpens (FR3.2): an empty panel names why it is empty —
// the key never reads as a dead no-op.
func TestSessionsPanelAlwaysOpens(t *testing.T) {
	t.Run("no sessions", func(t *testing.T) {
		m := newModel(t)
		m.hasTmux = true
		nm, _ := m.Update(runes("S"))
		m = nm.(Model)
		if m.mode != modeSessions {
			t.Fatalf("S with no sessions did not open (mode=%d)", m.mode)
		}
		if out := m.View(); !strings.Contains(out, "no live gogo-* sessions") {
			t.Errorf("empty panel does not name its reason:\n%s", out)
		}
	})

	t.Run("no tmux", func(t *testing.T) {
		m := newModel(t)
		m.hasTmux = false
		nm, _ := m.Update(runes("S"))
		m = nm.(Model)
		if m.mode != modeSessions {
			t.Fatalf("S without tmux did not open (mode=%d)", m.mode)
		}
		if out := m.View(); !strings.Contains(out, "tmux not installed") {
			t.Errorf("no-tmux panel does not name its reason:\n%s", out)
		}
	})
}

// TestSessionsPanelReassign (FR4): R over the focused session opens the target
// picker (drivable items only, rows showing the RESULTING name), the choice
// renames through the ONE reassign core exactly once, and the user stays in the
// panel.
func TestSessionsPanelReassign(t *testing.T) {
	t.Run("renames onto the picked runnable item", func(t *testing.T) {
		m, rr, _ := panelModel(t)
		nm, _ := m.Update(runes("S"))
		m = nm.(Model)
		m = send(m, runes("j"))
		m = send(m, runes("j")) // cursor → the unbound gogo-plan-some-title
		nm, _ = m.Update(runes("R"))
		m = nm.(Model)
		if m.mode != modeForm || m.pendingReassign != "gogo-plan-some-title" {
			t.Fatalf("panel R did not open the target picker (mode=%d pending=%q)", m.mode, m.pendingReassign)
		}
		v := m.form.View()
		if !strings.Contains(v, "gogo-go-inprogress") {
			t.Errorf("picker rows do not show the resulting name for the runnable target:\n%s", v)
		}
		if strings.Contains(v, "aborted") {
			t.Errorf("picker offers a terminal item:\n%s", v)
		}
		// The first option is the newest non-terminal feature; navigate to
		// "inprogress" by its visible row, simplest via selecting the first and
		// checking what the renamer got — instead, walk options until Enter on
		// the row whose value is the inprogress feature. The fixture's feature
		// order is stable, so find it by driving j until the form completes on it:
		// simpler and robust — complete on the FIRST option and assert against
		// what the renamer records.
		m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter})
		if len(rr.calls) != 1 {
			t.Fatalf("renamer called %d times, want exactly 1 (calls=%v)", len(rr.calls), rr.calls)
		}
		if rr.calls[0][0] != "gogo-plan-some-title" {
			t.Errorf("renamed %q, want the focused session gogo-plan-some-title", rr.calls[0][0])
		}
		if !strings.HasPrefix(rr.calls[0][1], "gogo-") {
			t.Errorf("rename base %q does not follow the convention", rr.calls[0][1])
		}
		if m.mode != modeSessions {
			t.Errorf("after the re-assign, mode=%d, want the panel (stay-in-panel)", m.mode)
		}
		if !strings.Contains(m.status, "re-assigned gogo-plan-some-title") {
			t.Errorf("status = %q, want the re-assign confirmation", m.status)
		}
	})

	t.Run("already-bound refusal via the shared core", func(t *testing.T) {
		m, rr, _ := panelModel(t)
		nm, _ := m.Update(runes("S"))
		m = nm.(Model)
		// cursor on gogo-go-inprogress (row 0); pick the target it is ALREADY bound to.
		nm, _ = m.Update(runes("R"))
		m = nm.(Model)
		// Walk to the "inprogress" option: options are non-terminal features in
		// repo order. Drive with j until Enter completes on it — assert by outcome.
		var found bool
		for i := 0; i < 8; i++ {
			trial := keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter})
			if strings.Contains(trial.status, "already bound") {
				m, found = trial, true
				break
			}
			if len(rr.calls) > 0 {
				// A rename happened — wrong option; reset and try the next row.
				rr.calls = nil
				nm, _ = trial.Update(runes("S"))
				m = nm.(Model)
				nm, _ = m.Update(runes("R"))
				m = nm.(Model)
			}
			m = send(m, runes("j"))
		}
		if !found {
			t.Fatalf("never reached the already-bound refusal (status=%q)", m.status)
		}
		if m.statusLevel != statusLevelWarn {
			t.Errorf("already-bound refusal level = %d, want blocked (amber)", m.statusLevel)
		}
	})

	t.Run("cancel returns to the panel", func(t *testing.T) {
		m, rr, _ := panelModel(t)
		nm, _ := m.Update(runes("S"))
		m = nm.(Model)
		nm, _ = m.Update(runes("R"))
		m = keyPress(t, nm.(Model), tea.KeyMsg{Type: tea.KeyEsc})
		if m.mode != modeSessions {
			t.Errorf("esc-cancel of the target picker landed in mode=%d, want the panel", m.mode)
		}
		if len(rr.calls) != 0 {
			t.Errorf("cancel renamed anyway: %v", rr.calls)
		}
	})

	// The Cancel OPTION is the picker's second cancel door (the plan: formOrigin
	// on Cancel *and* Esc) — drive it too, so neither leg is unpinned (REV-005).
	t.Run("the Cancel option returns to the panel", func(t *testing.T) {
		m, rr, _ := panelModel(t)
		nm, _ := m.Update(runes("S"))
		m = nm.(Model)
		nm, _ = m.Update(runes("R"))
		m = nm.(Model)
		// The Cancel row is LAST, and huh's Select WRAPS — one `k` from the top
		// lands exactly on it, independent of how many feature rows exist.
		m = keyPress(t, m, runes("k"))
		m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter})
		if m.mode != modeSessions {
			t.Errorf("Cancel-option completion landed in mode=%d, want the panel", m.mode)
		}
		if len(rr.calls) != 0 {
			t.Errorf("Cancel option renamed anyway: %v", rr.calls)
		}
		// Cancel must read as a CANCEL (REV-010): without the sentinel branch the
		// flow falls through to the "no longer present" amber — wrong message,
		// wrong severity — while every outcome assertion above stays green.
		if m.status != "cancelled" {
			t.Errorf("Cancel option status = %q, want %q", m.status, "cancelled")
		}
	})

	// FR4.2 / REV-003: zero drivable items → a NAMED refusal, never a picker
	// whose only row is Cancel.
	t.Run("zero drivable items refuses with a named reason", func(t *testing.T) {
		m, rr, _ := panelModel(t)
		for _, f := range m.repo.Features {
			f.Status = "shipped" // force every card terminal
		}
		nm, _ := m.Update(runes("S"))
		m = nm.(Model)
		nm, _ = m.Update(runes("R"))
		m = nm.(Model)
		if m.mode != modeSessions {
			t.Fatalf("zero-target R left the panel (mode=%d) — want a refusal, not a picker", m.mode)
		}
		if !strings.Contains(m.status, "no drivable work item") || m.statusLevel != statusLevelWarn {
			t.Errorf("status = %q (level %d), want the named zero-target refusal", m.status, m.statusLevel)
		}
		// The refusal must REACH THE SCREEN (REV-009): viewSessions' status render
		// is the panel's only m.status→screen path, and without this assertion a
		// deleted render leg keeps the suite green while every named refusal turns
		// into a silent no-op.
		if !strings.Contains(m.View(), "no drivable work item") {
			t.Errorf("the zero-target refusal is not rendered by the panel:\n%s", m.View())
		}
		if len(rr.calls) != 0 {
			t.Errorf("zero-target R renamed anyway: %v", rr.calls)
		}
	})
}

// TestSessionsPanelKill (FR5): K opens the destructive confirm (Cancel default —
// a bare Enter kills nothing), a deliberate Kill kills exactly the focused
// session, and the user stays in the panel.
func TestSessionsPanelKill(t *testing.T) {
	t.Run("bare Enter is safe", func(t *testing.T) {
		m, _, rk := panelModel(t)
		nm, _ := m.Update(runes("S"))
		m = nm.(Model)
		nm, _ = m.Update(runes("K"))
		m = keyPress(t, nm.(Model), tea.KeyMsg{Type: tea.KeyEnter})
		if len(rk.calls) != 0 {
			t.Fatalf("Enter on the Cancel-default confirm killed: %v", rk.calls)
		}
		if m.mode != modeSessions {
			t.Errorf("after the cancelled kill, mode=%d, want the panel", m.mode)
		}
	})

	// The Esc-abort door (cancelForm) must ALSO return to the panel — this is the
	// pendingKillSession leg of cancelForm's returnMode list, which a mutation
	// proved unpinned (REV-005).
	t.Run("esc-abort returns to the panel and kills nothing", func(t *testing.T) {
		m, _, rk := panelModel(t)
		nm, _ := m.Update(runes("S"))
		m = nm.(Model)
		nm, _ = m.Update(runes("K"))
		m = keyPress(t, nm.(Model), tea.KeyMsg{Type: tea.KeyEsc})
		if m.mode != modeSessions {
			t.Errorf("esc-abort of the kill confirm landed in mode=%d, want the panel (the cancelForm returnMode leg)", m.mode)
		}
		if len(rk.calls) != 0 {
			t.Errorf("esc-abort killed anyway: %v", rk.calls)
		}
	})

	t.Run("deliberate Kill closes exactly the focused session", func(t *testing.T) {
		m, _, rk := panelModel(t)
		nm, _ := m.Update(runes("S"))
		m = nm.(Model)
		m = send(m, runes("j")) // cursor → gogo-done-ready
		nm, _ = m.Update(runes("K"))
		m = keyPress(t, nm.(Model), runes("y")) // pick Kill and complete
		if len(rk.calls) != 1 || rk.calls[0] != "gogo-done-ready" {
			t.Fatalf("killer calls = %v, want exactly [gogo-done-ready]", rk.calls)
		}
		if m.mode != modeSessions {
			t.Errorf("after the kill, mode=%d, want the panel", m.mode)
		}
		if !strings.Contains(m.status, "closed gogo-done-ready") {
			t.Errorf("status = %q, want the close confirmation", m.status)
		}
	})
}

// TestSessionsPanelCursorClamps (FR3.4): the list is live — a session tick that
// shrinks it clamps the cursor instead of pointing past the end.
func TestSessionsPanelCursorClamps(t *testing.T) {
	m, _, _ := panelModel(t)
	nm, _ := m.Update(runes("S"))
	m = nm.(Model)
	m = send(m, runes("j"))
	m = send(m, runes("j")) // cursor on row 2
	// The 5s tick delivers a shrunken set.
	m = send(m, sessionsMsg{
		sessions: []string{"gogo-go-inprogress"},
		meta:     []launch.SessionMeta{{Name: "gogo-go-inprogress", Path: filepath.Clean(fixtureRoot)}},
	})
	// Assert the FIELD the handler writes, not just the clamped read (REV-004):
	// focusedSession()/View() clamp on their own, so only this makes the tick's
	// production clamp a pinned behaviour.
	if m.sessIdx != 0 {
		t.Errorf("sessIdx = %d after the list shrank to 1, want 0 (the tick must clamp, not just the read)", m.sessIdx)
	}
	if s := m.focusedSession(); s == nil || s.Name != "gogo-go-inprogress" {
		t.Fatalf("cursor did not clamp onto the surviving session (got %v)", s)
	}
	if !strings.Contains(m.View(), "gogo-go-inprogress") {
		t.Errorf("panel does not render the refreshed set")
	}
}

// TestUnboundCountPointsAtThePanel (FR6.4): the unbound-count status line names
// S — the chip shares the handler's key.
func TestUnboundCountPointsAtThePanel(t *testing.T) {
	m := newModel(t)
	m.sessionMeta = []launch.SessionMeta{{Name: "gogo-plan-some-title", Path: filepath.Clean(fixtureRoot)}}
	if out := m.View(); !strings.Contains(out, "S sessions") {
		t.Errorf("unbound count does not point at the S panel:\n%s", out)
	}
}
