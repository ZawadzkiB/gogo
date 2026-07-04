package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	tea "github.com/charmbracelet/bubbletea"
)

// --- FR9 badge: awaiting-uat + priority ---

func TestBadgeAwaitingUAT(t *testing.T) {
	// A report-complete feature parked at the UAT gate → the awaiting-uat badge.
	f := &contract.Feature{Slug: "uat-me", Phase: "knowledge", Status: "awaiting-uat"}
	if got := badge(f, nil); got != "awaiting-uat" {
		t.Errorf("awaiting-uat badge = %q", got)
	}

	// Mid-UAT re-plan: the status flips to waiting-for-user, which MUST win over
	// any awaiting-uat display (badge priority — REV-004).
	mid := &contract.Feature{Slug: "uat-me", Phase: "plan", Status: "waiting-for-user"}
	if got := badge(mid, nil); got != "waiting-for-user" {
		t.Errorf("mid-UAT badge = %q, want waiting-for-user (priority)", got)
	}

	// A running session outranks the resting awaiting-uat badge (something is
	// actively happening — e.g. a /gogo:done launch).
	f2 := &contract.Feature{Slug: "uat-me", Status: "awaiting-uat"}
	if got := badge(f2, []string{"gogo-done-uat-me"}); got != "running" {
		t.Errorf("running should outrank awaiting-uat, got %q", got)
	}
}

// TestLiveSessionExactMatch pins TEST-005 at the model layer: hasLiveSession /
// liveSessionFor match a card's session by exact slug boundary, so one feature's
// live session is never shown on — or peeked/attached as — a DIFFERENT feature
// whose slug is a textual substring of it.
func TestLiveSessionExactMatch(t *testing.T) {
	sessions := []string{"gogo-done-awaiting-card"}
	// The unrelated slug ("waiting-card" ⊂ "…awaiting-card") must NOT match.
	if hasLiveSession("waiting-card", sessions) {
		t.Errorf("waiting-card wrongly matched %v", sessions)
	}
	if s := liveSessionFor("waiting-card", sessions); s != "" {
		t.Errorf("liveSessionFor(waiting-card) = %q, want empty", s)
	}
	// The real owner matches.
	if !hasLiveSession("awaiting-card", sessions) {
		t.Errorf("awaiting-card should match its own session")
	}
	if s := liveSessionFor("awaiting-card", sessions); s != "gogo-done-awaiting-card" {
		t.Errorf("liveSessionFor(awaiting-card) = %q", s)
	}
	// A uniqueSession-suffixed session ("-<n>") matches only its own slug.
	suffixed := []string{"gogo-go-a-2"}
	if !hasLiveSession("a", suffixed) {
		t.Errorf("a should match its own suffixed session gogo-go-a-2")
	}
	if hasLiveSession("b", suffixed) {
		t.Errorf("b wrongly matched gogo-go-a-2")
	}
}

// TestAwaitingUATBadgeStyled: the rendered card shows the awaiting-uat badge text.
func TestAwaitingUATBadgeStyled(t *testing.T) {
	m := newModel(t)
	f := &contract.Feature{Slug: "uat-card", Title: "UAT card", Status: "awaiting-uat", Class: contract.ClassReadyToShip}
	out := m.renderCard(2, f, false, 40)
	if !strings.Contains(out, "awaiting-uat") {
		t.Errorf("card did not render the awaiting-uat badge:\n%s", out)
	}
}

// --- FR6 delete-to-trash ---

// TestDeleteChangelogBounces: `x` on a changelog-column (shipped) card is refused
// — the changelog archive is append-only.
func TestDeleteChangelogBounces(t *testing.T) {
	m := newModel(t)
	m.colIdx = 3 // changelog column
	nm, _ := m.deleteFocused()
	m = nm.(Model)
	if m.mode == modeForm {
		t.Fatalf("delete opened a confirm form for a changelog card (must bounce)")
	}
	if !strings.Contains(m.status, "append-only") {
		t.Errorf("changelog delete bounce = %q, want append-only hint", m.status)
	}
}

// writableRepo copies a single plan feature into a temp root so the delete move
// (a real os.Rename) can run without touching the read-only fixture tree.
func writableRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".gogo", "work", "feature-doomed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	state := "- **feature:** Doomed\n- **phase:** plan\n- **status:** plan-accepted\n- **created:** 2026-07-04\n"
	if err := os.WriteFile(filepath.Join(dir, "state.md"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plan.md"), []byte("# plan"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestDeleteToTrashFlow: `x` on a plan card → confirm form → toggle to Delete →
// the folder lands in .gogo/trash and the card leaves the board.
func TestDeleteToTrashFlow(t *testing.T) {
	root := writableRepo(t)
	m := New(root)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = nm.(Model)
	if len(m.cols[0]) != 1 {
		t.Fatalf("expected 1 plan card, got %d", len(m.cols[0]))
	}

	// x opens the destructive confirm (defaults to Cancel — explicit confirm).
	nm, _ = m.deleteFocused()
	m = nm.(Model)
	if m.mode != modeForm || m.pendingDelete == nil {
		t.Fatalf("x did not open a delete confirm, mode=%d pending=%v", m.mode, m.pendingDelete)
	}

	// Toggle to Delete (y / affirmative) and submit — huh's async completion chain
	// is pumped through Update via keyPress.
	m = keyPress(t, m, runes("y"))

	if m.mode != modeForm && m.mode != modeBoard {
		t.Fatalf("unexpected mode after confirm: %d", m.mode)
	}
	// The card is gone from the board, and its folder now lives in trash.
	if len(m.cols[0]) != 0 {
		t.Errorf("plan card still on the board after delete: %d", len(m.cols[0]))
	}
	if _, err := os.Stat(filepath.Join(root, ".gogo", "work", "feature-doomed")); !os.IsNotExist(err) {
		t.Errorf("feature folder was not moved out of .gogo/work")
	}
	entries, _ := os.ReadDir(filepath.Join(root, ".gogo", "trash"))
	if len(entries) != 1 || !strings.HasSuffix(entries[0].Name(), "-doomed") {
		t.Errorf("trash does not hold the deleted folder: %v", entries)
	}
	if !strings.Contains(m.status, "trash") {
		t.Errorf("status did not confirm the trash move: %q", m.status)
	}
}

// TestDeleteCancelKeepsCard: rejecting the confirm leaves the card in place.
func TestDeleteCancelKeepsCard(t *testing.T) {
	root := writableRepo(t)
	m := New(root)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = nm.(Model)

	nm, _ = m.deleteFocused()
	m = nm.(Model)
	m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEsc}) // abort

	if len(m.cols[0]) != 1 {
		t.Errorf("card vanished after a cancelled delete: %d", len(m.cols[0]))
	}
	if m.pendingDelete != nil {
		t.Errorf("pendingDelete not cleared after cancel")
	}
	if _, err := os.Stat(filepath.Join(root, ".gogo", "work", "feature-doomed")); err != nil {
		t.Errorf("folder moved despite a cancelled delete: %v", err)
	}
}

// TestDeleteCancelPreservesSelection pins REV-012: BOTH ways to cancel a DELETE
// confirm — Esc and the Cancel button — leave the unrelated ready-ship selection
// intact. The launch-form TEST-002 "clear the selection on cancel" rule applies
// only to SHIP forms; a delete's ship-selection is unrelated, so wiping it on Esc
// (the old behaviour) was an inconsistency with the Cancel-button (finishDelete)
// path that preserved it.
func TestDeleteCancelPreservesSelection(t *testing.T) {
	// --- Esc path ---
	m := newModel(t)
	m.selected["ready"] = true
	m.selected["legacy-ready"] = true
	m.colIdx = 0 // a plan (work) card — deletable, not the changelog column
	nm, _ := m.deleteFocused()
	m = nm.(Model)
	if m.mode != modeForm || m.pendingDelete == nil {
		t.Fatalf("x did not open a delete confirm, mode=%d pending=%v", m.mode, m.pendingDelete)
	}
	m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeBoard || m.pendingDelete != nil {
		t.Errorf("esc did not clear the delete cleanly: mode=%d pending=%v", m.mode, m.pendingDelete)
	}
	if got := m.selectedSlugs(); len(got) != 2 {
		t.Errorf("esc-cancel of a delete wiped the ready selection: %v (want 2)", got)
	}

	// --- Cancel-button path (n → finishDelete, confirmed=false) ---
	m2 := newModel(t)
	m2.selected["ready"] = true
	m2.selected["legacy-ready"] = true
	m2.colIdx = 0
	nm, _ = m2.deleteFocused()
	m2 = nm.(Model)
	m2 = keyPress(t, m2, runes("n")) // pick Cancel and complete
	if m2.mode != modeBoard || m2.pendingDelete != nil {
		t.Errorf("cancel button did not clear the delete cleanly: mode=%d pending=%v", m2.mode, m2.pendingDelete)
	}
	if got := m2.selectedSlugs(); len(got) != 2 {
		t.Errorf("cancel-button delete wiped the ready selection: %v (want 2)", got)
	}
}

// --- FR7 log peek ---

// TestPeekLiveSession: `l` on a card with a live gogo-* session opens a read-only
// peek showing the captured pane; `r` re-captures.
func TestPeekLiveSession(t *testing.T) {
	m := newModel(t)
	m.colIdx = 1 // in-progress card
	f := m.focusedCard()
	if f == nil {
		t.Fatal("no in-progress card")
	}
	m.sessions = []string{"gogo-go-" + f.Slug}
	captures := 0
	m.capturer = func(session string, lines int) (string, error) {
		captures++
		if session != "gogo-go-"+f.Slug {
			t.Errorf("capturer got session %q", session)
		}
		return "CAPTURED PANE OUTPUT", nil
	}

	m = keyPress(t, m, runes("l"))
	if m.mode != modeViewer || !m.peeking {
		t.Fatalf("l did not open a peek viewer, mode=%d peeking=%v", m.mode, m.peeking)
	}
	if got := m.viewport.View(); !strings.Contains(got, "CAPTURED PANE OUTPUT") {
		t.Errorf("peek viewer missing capture:\n%s", got)
	}
	if !strings.Contains(m.View(), "re-capture") {
		t.Errorf("peek footer missing the re-capture hint")
	}

	// r re-captures (a second capturer call).
	m = keyPress(t, m, runes("r"))
	if captures < 2 {
		t.Errorf("r did not re-capture: captures=%d", captures)
	}

	// q returns to the board and clears peek state.
	m = send(m, runes("q"))
	if m.mode != modeBoard || m.peeking {
		t.Errorf("q did not close the peek: mode=%d peeking=%v", m.mode, m.peeking)
	}
}

// TestPeekNoSession: `l` with no live session and no log → a hint, stays on board.
func TestPeekNoSession(t *testing.T) {
	m := newModel(t)
	m.colIdx = 0 // a plan card with no session/log
	m.sessions = nil
	nm, _ := m.peekFocused()
	m = nm.(Model)
	if m.mode == modeViewer || m.peeking {
		t.Fatalf("peek opened with no session/log")
	}
	if !strings.Contains(m.status, "no running session") {
		t.Errorf("peek no-session hint = %q", m.status)
	}
}

// TestPeekBackgroundLogTail: with no tmux session but a background -p log, `l`
// tails the log file.
func TestPeekBackgroundLogTail(t *testing.T) {
	root := writableRepo(t)
	// A background log for the slug under .gogo/resources/cli/logs.
	logs := filepath.Join(root, ".gogo", "resources", "cli", "logs")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logs, "go-doomed.log"), []byte("line1\nline2\nLOGTAIL\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(root)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = nm.(Model)
	m.sessions = nil // force the log-tail branch

	m = keyPress(t, m, runes("l"))
	if !m.peeking {
		t.Fatalf("l did not open a log-tail peek")
	}
	if got := m.viewport.View(); !strings.Contains(got, "LOGTAIL") {
		t.Errorf("log-tail peek missing content:\n%s", got)
	}
}

func TestTailLines(t *testing.T) {
	in := "a\nb\nc\nd\ne\n"
	if got := tailLines(in, 2); got != "d\ne" {
		t.Errorf("tailLines(2) = %q", got)
	}
	if got := tailLines(in, 100); got != "a\nb\nc\nd\ne" {
		t.Errorf("tailLines(all) = %q", got)
	}
}
