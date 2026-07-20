package tui

import (
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/orchestrator"
	tea "github.com/charmbracelet/bubbletea"
)

// --- Slice B: rich board drill-in card ---

// fakeReg returns a registry seam that yields a fixed Persistent map — no real
// .gogo/resources/cli/sessions file, so the reader + render are driven with fakes.
func fakeReg(persistent map[string]*orchestrator.PersistentSession) func(string, string) *orchestrator.Registry {
	return func(_, slug string) *orchestrator.Registry {
		return &orchestrator.Registry{Slug: slug, Persistent: persistent}
	}
}

// recordingKiller records the exact session names the drill `K` asks to kill,
// instead of shelling out to tmux — the fire-exactly-once assertion (FR-B3).
type recordingKiller struct{ calls []string }

func (r *recordingKiller) kill(name string) error { r.calls = append(r.calls, name); return nil }

// TestDrillCardDetailRender (FR-B1/B2/B4): enter on a card shows the description,
// folder (feature-<slug>), status, its session rows (live, with cost/turns), and
// a recent-events tail — above the file list.
func TestDrillCardDetailRender(t *testing.T) {
	m := newModel(t)
	m.colIdx = 1 // the in-progress feature (slug "inprogress") — has events.jsonl
	m.hasTmux = true
	m.sessions = []string{"gogo-go-inprogress"}
	m.registry = fakeReg(map[string]*orchestrator.PersistentSession{
		"go": {Kind: "go", Status: "running", Tmux: "gogo-go-inprogress", CostUSD: 0.12, NumTurns: 5},
	})

	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeDrill || m.drill.Slug != "inprogress" {
		t.Fatalf("enter did not drill into inprogress (mode=%d)", m.mode)
	}
	out := m.View()
	for _, want := range []string{
		"card — inprogress",
		"description", "folder", "feature-inprogress/", "status",
		"sessions", "go", "live", "running", "gogo-go-inprogress", "$0.12", "5 turns",
		"recent events", "files",
		"a attach · K kill", // the updated help line
	} {
		if !strings.Contains(out, want) {
			t.Errorf("drill card missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "no events recorded") {
		t.Errorf("events tail empty for a feature that has events.jsonl:\n%s", out)
	}
}

// TestSessionRowsTable is the FR-B5 core: (registry, live tmux, slug) → rows,
// exercising live/stale, tracked/untracked, the missing-registry degrade, and the
// exact-match attribution guard (TEST-005).
func TestSessionRowsTable(t *testing.T) {
	ps := func(status, tmux string) *orchestrator.PersistentSession {
		return &orchestrator.PersistentSession{Status: status, Tmux: tmux}
	}

	t.Run("live go leg", func(t *testing.T) {
		rows := sessionRows(&orchestrator.Registry{Persistent: map[string]*orchestrator.PersistentSession{
			"go": ps("running", "gogo-go-auth"),
		}}, []string{"gogo-go-auth"}, "auth")
		if len(rows) != 1 || rows[0].Kind != "go" || !rows[0].Live || !rows[0].Tracked || rows[0].Status != "running" {
			t.Fatalf("live go leg → %+v", rows)
		}
	})

	t.Run("reaped leg still shown, stale", func(t *testing.T) {
		rows := sessionRows(&orchestrator.Registry{Persistent: map[string]*orchestrator.PersistentSession{
			"go": ps("reaped", "gogo-go-auth"),
		}}, nil, "auth")
		if len(rows) != 1 || rows[0].Live || rows[0].Status != "reaped" || !rows[0].Tracked {
			t.Fatalf("reaped leg → %+v (want one stale, tracked, reaped row)", rows)
		}
	})

	t.Run("untracked live racer", func(t *testing.T) {
		rows := sessionRows(&orchestrator.Registry{}, []string{"gogo-go-auth"}, "auth")
		if len(rows) != 1 || rows[0].Tracked || !rows[0].Live || rows[0].Session != "gogo-go-auth" || rows[0].Kind != "" {
			t.Fatalf("untracked live → %+v", rows)
		}
	})

	t.Run("missing registry, no sessions → no rows", func(t *testing.T) {
		if rows := sessionRows(nil, nil, "auth"); len(rows) != 0 {
			t.Fatalf("empty → %+v (want none)", rows)
		}
		if rows := sessionRows(&orchestrator.Registry{}, nil, "auth"); len(rows) != 0 {
			t.Fatalf("empty reg → %+v (want none)", rows)
		}
	})

	t.Run("exact-match guard (TEST-005)", func(t *testing.T) {
		// oauth ⊄ auth, and awaiting-card ⊅ waiting-card — neither is this slug's.
		if rows := sessionRows(&orchestrator.Registry{}, []string{"gogo-done-oauth"}, "auth"); len(rows) != 0 {
			t.Errorf("oauth wrongly attributed to auth: %+v", rows)
		}
		if rows := sessionRows(&orchestrator.Registry{}, []string{"gogo-done-awaiting-card"}, "waiting-card"); len(rows) != 0 {
			t.Errorf("awaiting-card wrongly attributed to waiting-card: %+v", rows)
		}
	})

	t.Run("tracked live + tracked stale + untracked live", func(t *testing.T) {
		rows := sessionRows(&orchestrator.Registry{Persistent: map[string]*orchestrator.PersistentSession{
			"go":   ps("running", "gogo-go-auth"),
			"plan": ps("reaped", "gogo-plan-auth"),
		}}, []string{"gogo-go-auth", "gogo-go-auth-2"}, "auth")
		if len(rows) != 3 {
			t.Fatalf("want 3 rows (go live, plan stale, untracked live), got %d: %+v", len(rows), rows)
		}
		if rows[0].Kind != "go" || !rows[0].Live || rows[1].Kind != "plan" || rows[1].Live {
			t.Errorf("tracked leg order/liveness wrong: %+v", rows)
		}
		last := rows[2]
		if last.Tracked || !last.Live || last.Session != "gogo-go-auth-2" {
			t.Errorf("untracked-live row wrong: %+v", last)
		}
	})
}

// TestEventsTail: compact tail — empty in, trimming to the last n, and the
// hh:mm:ss event phase[ rN][ — note] shape.
func TestEventsTail(t *testing.T) {
	if got := eventsTail(nil, 5); got != "" {
		t.Errorf("empty events → %q, want empty", got)
	}
	evs := []contract.Event{
		{TSRaw: "2026-07-12T10:00:00Z", Event: "phase-started", Phase: "implement"},
		{TSRaw: "2026-07-12T10:01:00Z", Event: "issues-found", Phase: "review", Round: 2, HasRound: true, Note: "2 blockers"},
		{TSRaw: "2026-07-12T10:02:00Z", Event: "phase-done", Phase: "review"},
	}
	tail := eventsTail(evs, 2)
	if strings.Contains(tail, "phase-started") {
		t.Errorf("tail(2) kept a trimmed-off event:\n%s", tail)
	}
	if !strings.Contains(tail, "issues-found review r2 — 2 blockers") {
		t.Errorf("tail line shape wrong:\n%s", tail)
	}
	if lines := strings.Count(tail, "\n") + 1; lines != 2 {
		t.Errorf("tail(2) has %d lines, want 2:\n%s", lines, tail)
	}
}

// TestDrillAttachWiring (FR-B3): `a` in the drill over a card with a live session
// starts the attach path (a non-nil ExecProcess command); no live session → a
// status hint and no attach.
func TestDrillAttachWiring(t *testing.T) {
	base := newModel(t)
	base.colIdx = 1
	base.hasTmux = true
	base.registry = fakeReg(nil)

	// Live session present → attach fires (a command is returned).
	m := base
	m.sessions = []string{"gogo-go-inprogress"}
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	nm, cmd := m.Update(runes("a"))
	if cmd == nil {
		t.Errorf("attach over a live session returned no command")
	}
	if strings.Contains(nm.(Model).status, "no running session") {
		t.Errorf("attach over a live session reported no session: %q", nm.(Model).status)
	}

	// No live session (only a non-matching sibling) → a hint, no attach.
	m2 := base
	m2.sessions = []string{"gogo-go-inprogressX"} // not an exact match for "inprogress"
	m2 = send(m2, tea.KeyMsg{Type: tea.KeyEnter})
	nm2, cmd2 := m2.Update(runes("a"))
	if cmd2 != nil {
		t.Errorf("attach with no matching session should not return a command")
	}
	if !strings.Contains(nm2.(Model).status, "no running session") {
		t.Errorf("attach with no session status = %q, want the no-session hint", nm2.(Model).status)
	}
}

// TestDrillKillWiring (FR-B3): `K` → confirm → the killer is called EXACTLY once
// with the card's exact live session name (never a substring sibling); cancelling
// never calls the killer.
func TestDrillKillWiring(t *testing.T) {
	drillInLive := func(t *testing.T, k *recordingKiller) Model {
		t.Helper()
		m := newModel(t)
		m.colIdx = 1
		m.hasTmux = true
		m.registry = fakeReg(nil)
		m.killer = k.kill
		// The card's exact session PLUS a substring sibling that must NOT be killed.
		m.sessions = []string{"gogo-go-inprogress", "gogo-go-inprogressX"}
		m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
		return m
	}

	// Confirm → killer called once, with the exact name only.
	t.Run("confirm kills the exact session once", func(t *testing.T) {
		k := &recordingKiller{}
		m := drillInLive(t, k)
		nm, _ := m.Update(runes("K"))
		m = nm.(Model)
		if m.mode != modeForm || m.pendingKill == nil {
			t.Fatalf("K did not open a kill confirm (mode=%d pending=%v)", m.mode, m.pendingKill)
		}
		m = keyPress(t, m, runes("y")) // pick Kill and complete
		if len(k.calls) != 1 || k.calls[0] != "gogo-go-inprogress" {
			t.Fatalf("killer calls = %v, want exactly [gogo-go-inprogress]", k.calls)
		}
		if m.mode != modeDrill {
			t.Errorf("after kill, mode=%d, want back on the drill", m.mode)
		}
	})

	// Cancel (Esc) → killer never called, and the user stays on the drill card
	// (REV-001 — both cancel paths return to the drill, not the board).
	t.Run("esc cancels without killing, stays on the drill", func(t *testing.T) {
		k := &recordingKiller{}
		m := drillInLive(t, k)
		nm, _ := m.Update(runes("K"))
		m = keyPress(t, nm.(Model), tea.KeyMsg{Type: tea.KeyEsc})
		if len(k.calls) != 0 {
			t.Fatalf("esc still killed: %v", k.calls)
		}
		if m.mode != modeDrill {
			t.Errorf("esc-cancel of the kill confirm left mode=%d, want the drill (REV-001)", m.mode)
		}
	})

	// Cancel button (n) → killer never called, and the user stays on the drill.
	t.Run("cancel button does not kill, stays on the drill", func(t *testing.T) {
		k := &recordingKiller{}
		m := drillInLive(t, k)
		nm, _ := m.Update(runes("K"))
		m = keyPress(t, nm.(Model), runes("n"))
		if len(k.calls) != 0 {
			t.Fatalf("cancel-button still killed: %v", k.calls)
		}
		if m.mode != modeDrill {
			t.Errorf("cancel-button of the kill confirm left mode=%d, want the drill", m.mode)
		}
	})
}

// TestChangelogLiveSessionDot (FR-1): a collapsed changelog row whose shipped item
// has a live pipeline session shows a green ● just before its slug; an idle row does
// not — and the collapsed `✓ slug` list shape is preserved for both.
func TestChangelogLiveSessionDot(t *testing.T) {
	m := newModel(t)
	a := &contract.Feature{Slug: "alpha", Class: contract.ClassShipped, Completed: "2026-07-01"}
	b := &contract.Feature{Slug: "bravo", Class: contract.ClassShipped, Completed: "2026-07-02"}
	m.cols[3] = []*contract.Feature{a, b}
	m.colIdx = 0 // focus another column so neither row is the focus bar
	// A live gogo-done session for alpha only (exact-match convention — TEST-005).
	m.sessions = []string{"gogo-done-alpha"}

	out := m.renderColumn(3, m.boardColWidth())
	if !strings.Contains(out, "● alpha") {
		t.Errorf("live changelog row missing the ● dot before its slug:\n%s", out)
	}
	if strings.Contains(out, "● bravo") {
		t.Errorf("idle changelog row wrongly shows a live ● dot:\n%s", out)
	}
	// Collapsed list shape preserved: the live row keeps ✓ before the dot, the idle
	// row keeps the plain ✓ slug form.
	if !strings.Contains(out, "✓ ● alpha") {
		t.Errorf("live row lost the collapsed ✓ shape:\n%s", out)
	}
	if !strings.Contains(out, "✓ bravo") {
		t.Errorf("idle row lost the collapsed ✓ slug shape:\n%s", out)
	}
}

// TestDrillAttachPicker (FR-2): `a` over a card with ≥2 live sessions opens a picker
// (modeForm, pendingAttach set) listing every exact-match session; selecting one
// attaches exactly it (m.status names the chosen session); Cancel/esc attaches
// nothing and restores the drill. Exactly one session keeps the direct-attach UX
// (TestDrillAttachWiring) — untouched.
func TestDrillAttachPicker(t *testing.T) {
	drillMulti := func(t *testing.T) Model {
		t.Helper()
		m := newModel(t)
		m.colIdx = 1
		m.hasTmux = true
		m.registry = fakeReg(nil)
		// Two exact-match sessions PLUS a substring sibling that must NOT be offered.
		m.sessions = []string{"gogo-go-inprogress", "gogo-plan-inprogress", "gogo-go-inprogressX"}
		return send(m, tea.KeyMsg{Type: tea.KeyEnter})
	}

	t.Run("opens a picker listing each session, excludes the sibling", func(t *testing.T) {
		m := drillMulti(t)
		nm, cmd := m.Update(runes("a"))
		m = nm.(Model)
		if m.mode != modeForm || m.pendingAttach == nil {
			t.Fatalf("a over ≥2 sessions did not open the picker (mode=%d pending=%v)", m.mode, m.pendingAttach)
		}
		if cmd == nil {
			t.Errorf("picker open returned no init command")
		}
		out := m.View()
		for _, want := range []string{"Attach which session", "gogo-go-inprogress", "gogo-plan-inprogress", "Cancel"} {
			if !strings.Contains(out, want) {
				t.Errorf("attach picker missing %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, "gogo-go-inprogressX") {
			t.Errorf("attach picker offered the substring sibling:\n%s", out)
		}
	})

	t.Run("selecting a session attaches exactly it", func(t *testing.T) {
		m := drillMulti(t)
		nm, _ := m.Update(runes("a"))
		m = keyPress(t, nm.(Model), runes("j"))            // move to the 2nd option
		m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // select + submit
		if !strings.Contains(m.status, "attaching gogo-plan-inprogress") {
			t.Errorf("status = %q, want it to name the chosen (2nd) session", m.status)
		}
		if m.pendingAttach != nil {
			t.Errorf("pendingAttach not cleared after a completed pick: %v", m.pendingAttach)
		}
	})

	t.Run("cancel attaches nothing and restores the drill", func(t *testing.T) {
		m := drillMulti(t)
		nm, _ := m.Update(runes("a"))
		m = keyPress(t, nm.(Model), tea.KeyMsg{Type: tea.KeyEsc})
		if m.mode != modeDrill {
			t.Errorf("esc-cancel of the attach picker left mode=%d, want the drill", m.mode)
		}
		if strings.Contains(m.status, "attaching") {
			t.Errorf("esc-cancel still attached: %q", m.status)
		}
	})

	// The BOARD-origin picker (pickerFromDrill=false) is a distinct reachable branch:
	// cancel must restore modeBoard AND preserve the ready-ship multi-selection (the
	// attach-cancel is unrelated to the selection — formPreservesSelection now covers
	// pendingAttach, mirroring the kill/delete rule, REV-012).
	t.Run("board-origin cancel restores the board and keeps the ready-ship selection", func(t *testing.T) {
		m := newModel(t)
		m.colIdx = 2 // ready column, focus "ready"
		m.cardIdx[2] = 0
		m.hasTmux = true
		m.registry = fakeReg(nil)
		m.sessions = []string{"gogo-go-ready", "gogo-plan-ready"} // ≥2 exact matches for "ready"
		m.selected[selKey(m, "ready")] = true
		m.selected[selKey(m, "legacy-ready")] = true

		nm, _ := m.Update(runes("a")) // board a → attach picker (≥2)
		m = nm.(Model)
		if m.mode != modeForm || m.pendingAttach == nil || m.pickerFromDrill {
			t.Fatalf("board a did not open a board-origin attach picker (mode=%d pending=%v fromDrill=%v)", m.mode, m.pendingAttach, m.pickerFromDrill)
		}
		m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEsc})
		if m.mode != modeBoard {
			t.Errorf("board-origin cancel left mode=%d, want the board", m.mode)
		}
		if strings.Contains(m.status, "attaching") {
			t.Errorf("board-origin cancel still attached: %q", m.status)
		}
		if len(m.selectedSlugs()) != 2 {
			t.Errorf("attach-cancel wiped the ready-ship selection: %v", m.selectedSlugs())
		}
	})
}

// TestDrillKillPicker (FR-3): `K` over ≥2 live sessions opens a picker listing each
// session + an "all N" option + Cancel. Selecting one session kills exactly it (once,
// the exact name); "all N" kills each session once; Cancel kills nothing. The single
// session case keeps the confirm UX (TestDrillKillWiring) — untouched.
func TestDrillKillPicker(t *testing.T) {
	drillMulti := func(t *testing.T, k *recordingKiller) Model {
		t.Helper()
		m := newModel(t)
		m.colIdx = 1
		m.hasTmux = true
		m.registry = fakeReg(nil)
		m.killer = k.kill
		// Two exact-match sessions PLUS a substring sibling that must never be a target.
		m.sessions = []string{"gogo-go-inprogress", "gogo-plan-inprogress", "gogo-go-inprogressX"}
		return send(m, tea.KeyMsg{Type: tea.KeyEnter})
	}

	t.Run("picker lists each session + all + Cancel, excludes the sibling", func(t *testing.T) {
		k := &recordingKiller{}
		m := drillMulti(t, k)
		nm, _ := m.Update(runes("K"))
		m = nm.(Model)
		if m.mode != modeForm || m.pendingKill == nil {
			t.Fatalf("K over ≥2 sessions did not open the picker (mode=%d pending=%v)", m.mode, m.pendingKill)
		}
		out := m.View()
		for _, want := range []string{"Kill which session", "gogo-go-inprogress", "gogo-plan-inprogress", "all 2 sessions", "Cancel"} {
			if !strings.Contains(out, want) {
				t.Errorf("kill picker missing %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, "gogo-go-inprogressX") {
			t.Errorf("kill picker offered the substring sibling:\n%s", out)
		}
	})

	// Options order (liveSessionsFor iterates m.sessions): [0]=gogo-go-inprogress,
	// [1]=gogo-plan-inprogress, [2]=all 2 sessions, [3]=Cancel.
	t.Run("selecting one session kills exactly it once", func(t *testing.T) {
		k := &recordingKiller{}
		m := drillMulti(t, k)
		nm, _ := m.Update(runes("K"))
		m = keyPress(t, nm.(Model), tea.KeyMsg{Type: tea.KeyEnter}) // 1st option → select+submit
		if len(k.calls) != 1 || k.calls[0] != "gogo-go-inprogress" {
			t.Fatalf("killer calls = %v, want exactly [gogo-go-inprogress]", k.calls)
		}
		if m.mode != modeDrill {
			t.Errorf("after kill, mode=%d, want the drill", m.mode)
		}
	})

	t.Run("all N kills each session once", func(t *testing.T) {
		k := &recordingKiller{}
		m := drillMulti(t, k)
		nm, _ := m.Update(runes("K"))
		m = keyPress(t, nm.(Model), runes("j")) // → 2nd session
		m = keyPress(t, m, runes("j"))          // → all 2 sessions
		m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter})
		if len(k.calls) != 2 {
			t.Fatalf("all-kill calls = %v, want both sessions killed once each", k.calls)
		}
	})

	t.Run("Cancel kills nothing and stays on the drill", func(t *testing.T) {
		k := &recordingKiller{}
		m := drillMulti(t, k)
		nm, _ := m.Update(runes("K"))
		m = keyPress(t, nm.(Model), runes("j")) // → 2nd session
		m = keyPress(t, m, runes("j"))          // → all
		m = keyPress(t, m, runes("j"))          // → Cancel
		m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter})
		if len(k.calls) != 0 {
			t.Fatalf("Cancel still killed: %v", k.calls)
		}
		if m.mode != modeDrill {
			t.Errorf("Cancel left mode=%d, want the drill", m.mode)
		}
	})
}

// TestDrillDegradesNoSessions (FR-B5 no-LLM/degradation): a feature with no
// registry and no live sessions still renders a clean panel ("no tracked
// sessions"), and K/a with nothing live are safe no-ops with a hint.
func TestDrillDegradesNoSessions(t *testing.T) {
	m := newModel(t)
	m.colIdx = 1
	m.hasTmux = true
	m.registry = fakeReg(nil) // empty Persistent
	m.sessions = nil
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})

	if out := m.View(); !strings.Contains(out, "no tracked sessions") {
		t.Errorf("degraded drill did not render the no-sessions panel:\n%s", out)
	}

	// K with nothing live → a hint, no form, no crash.
	nm, _ := m.Update(runes("K"))
	km := nm.(Model)
	if km.mode == modeForm {
		t.Errorf("K opened a kill confirm with no live session")
	}
	if !strings.Contains(km.status, "no live session") {
		t.Errorf("K status = %q, want the no-live-session hint", km.status)
	}

	// a with nothing live → the no-session hint.
	nm2, cmd := m.Update(runes("a"))
	if cmd != nil || !strings.Contains(nm2.(Model).status, "no running session") {
		t.Errorf("a with no session: cmd=%v status=%q", cmd, nm2.(Model).status)
	}
}

// TestDrillStatusIsRendered pins TEST-001: viewDrill must actually RENDER m.status,
// not just set it — otherwise a/K hints and kill confirmations are silent no-ops in
// the live TUI (unit tests that only assert Model.status miss this). Drives the real
// `a`-with-no-session path and asserts the hint appears in View() output.
func TestDrillStatusIsRendered(t *testing.T) {
	m := newModel(t)
	m.colIdx = 1
	m.hasTmux = true
	m.registry = fakeReg(nil)
	m.sessions = nil
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter}) // drill into a no-session card

	// A freshly-opened drill has no status → no stale status line.
	if strings.Contains(m.View(), "no running session") {
		t.Fatalf("drill showed a session hint before any action:\n%s", m.View())
	}

	// Press `a` with nothing live → the hint must be VISIBLE in the drill view.
	m = send(m, runes("a"))
	if m.mode != modeDrill {
		t.Fatalf("a bounced off the drill (mode=%d)", m.mode)
	}
	if !strings.Contains(m.View(), "no running session") {
		t.Errorf("drill view did not render the a/K status hint (TEST-001):\n%s", m.View())
	}
}
