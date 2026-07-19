package tui

import (
	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/trash"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// deleteFocused (x on the board — FR6) moves the focused WORK card's folder to
// .gogo/trash/ behind an explicit huh confirm. Changelog-column (shipped) cards
// are the append-only archive and bounce — never deletable from the board.
func (m Model) deleteFocused() (tea.Model, tea.Cmd) {
	f := m.focusedCard()
	if f == nil {
		m.status = "no card to delete"
		return m, nil
	}
	if f.Column() == contract.ColChangelog {
		m.status = "changelog is append-only — cannot delete " + f.Slug
		return m, nil
	}
	m.startDeleteForm(f)
	return m, m.form.Init()
}

// startDeleteForm opens the destructive-action confirm. It defaults to Cancel
// (confirm=false) so Enter is safe: the user must deliberately pick Delete.
func (m *Model) startDeleteForm(f *contract.Feature) {
	m.pendingDelete = f
	m.binding = &formBinding{confirm: false}
	title := "Move " + f.Slug + " (" + f.Class + ") to .gogo/trash/?"
	desc := "recoverable — restore with:  gogo trash restore <entry>"
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(title).
			Description(desc).
			Affirmative("Delete").
			Negative("Cancel").
			Value(&m.binding.confirm),
	))
	m.mode = modeForm
}

// finishDelete runs after a completed delete form. A confirmed form moves the
// folder to trash and reloads the board; a cancelled one just returns.
func (m Model) finishDelete() (tea.Model, tea.Cmd) {
	f := m.pendingDelete
	confirmed := m.binding != nil && m.binding.confirm
	m.pendingDelete = nil
	m.binding = nil
	m.form = nil
	m.mode = modeBoard
	if !confirmed || f == nil {
		m.status = "cancelled"
		return m, nil
	}
	entry, err := trash.MoveToTrash(m.rootFor(f), f.Dir)
	if err != nil {
		m.status = "delete failed: " + err.Error()
		return m, nil
	}
	m.status = "moved " + f.Slug + " → .gogo/trash/" + entry.Base + "  (gogo trash restore " + entry.Base + ")"
	// Immediate feedback: drop the card now (fsnotify would also fire a reload).
	m.reload()
	m.refocus("")
	m.reflowColumns()
	return m, nil
}
