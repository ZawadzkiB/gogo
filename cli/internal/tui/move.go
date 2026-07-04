package tui

import (
	"sort"
	"strconv"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// launchDoneMsg carries the outcome of a launch back to the model.
type launchDoneMsg struct{ status string }

// attemptAction resolves the requested action into a launch intent or a bounce
// reason (the status-line hint). Pure — the unit-tested move-guard core.
//
//	ship=false (m): plan/unfinished → go · in-progress → go(resume) · ready → done · shipped → bounce
//	ship=true  (d): ready(+selection) → done · everything else → bounce
func (m *Model) attemptAction(ship bool) (launch.Intent, bool, string) {
	sel := m.selectedSlugs()
	if len(sel) > 0 {
		// A ready selection always ships as one (merged if >1) entry.
		return launch.BuildIntent(launch.ActionDone, sel, ""), true, ""
	}

	f := m.focusedCard()
	if f == nil {
		return launch.Intent{}, false, "no card focused"
	}

	if ship {
		if f.Class != contract.ClassReadyToShip {
			return launch.Intent{}, false, "only ready cards can ship (d) — " + f.Slug + " is " + f.Class
		}
		return launch.BuildIntent(launch.ActionDone, []string{f.Slug}, ""), true, ""
	}

	switch f.Class {
	case contract.ClassUnfinished, contract.ClassInProgress:
		return launch.BuildIntent(launch.ActionGo, []string{f.Slug}, ""), false, ""
	case contract.ClassReadyToShip:
		return launch.BuildIntent(launch.ActionDone, []string{f.Slug}, ""), true, ""
	case contract.ClassShipped:
		return launch.Intent{}, false, "already shipped — no move (illegal)"
	}
	return launch.Intent{}, false, "no legal move for " + f.Slug
}

// launchAction runs the guard, then either bounces or opens the huh
// confirmation. NEVER launches without the confirmation.
func (m Model) launchAction(ship bool) (tea.Model, tea.Cmd) {
	intent, isShip, bounce := m.attemptAction(ship)
	if bounce != "" {
		m.status = bounce
		return m, nil
	}
	if !m.hasClaude {
		m.status = "claude CLI not on PATH — cannot launch " + intent.Command
		return m, nil
	}
	m.startForm(intent, isShip)
	return m, m.form.Init()
}

// startForm builds the huh confirmation (a release-name input first, for a
// merged ship of ≥2) and switches to form mode.
func (m *Model) startForm(intent launch.Intent, isShip bool) {
	m.pending = intent
	m.pendingShip = isShip
	// A fresh, heap-stable binding for this form's fields (see formBinding).
	// Defaults to the affirmative so the confirmation the user deliberately
	// opened is submittable with Enter/Tab; Esc/Ctrl+C or toggling to Cancel (n)
	// aborts it.
	m.binding = &formBinding{confirm: true}

	var fields []huh.Field
	merged := isShip && len(intent.Slugs) >= 2
	if merged {
		m.binding.release = suggestRelease(intent.Slugs)
		fields = append(fields, huh.NewInput().
			Title("Release name for the merged entry").
			Description(strings.Join(intent.Slugs, " + ")).
			Value(&m.binding.release))
	}
	fields = append(fields, huh.NewConfirm().
		Title(m.confirmSummary(intent)).
		Affirmative("Launch").
		Negative("Cancel").
		Value(&m.binding.confirm))

	m.form = huh.NewForm(huh.NewGroup(fields...))
	m.mode = modeForm
}

func (m *Model) confirmSummary(intent launch.Intent) string {
	where := "tmux session " + intent.Session
	if !m.hasTmux {
		where = "background (claude -p + log)"
	}
	// FR8: state the effective permission mode the launch runs under.
	return "will run: claude \"" + intent.Command + "\"  in " + where + "  · " + launch.PermissionSummary()
}

// doLaunch rebuilds the intent with the (possibly edited) release name and
// spawns Claude via the injected launcher. Returns a command that reports the
// outcome. The caller (updateForm) clears the consumed selection/pending state
// on the model it returns — this closure only captures the resolved intent.
func (m Model) doLaunch() tea.Cmd {
	intent := m.pending
	if m.pendingShip && len(intent.Slugs) >= 2 && m.binding != nil {
		intent = launch.BuildIntent(launch.ActionDone, intent.Slugs, m.binding.release)
	}
	root := m.root
	launcher := m.launcher
	return func() tea.Msg {
		res, err := launcher(root, intent)
		if err != nil {
			return launchDoneMsg{status: "launch failed: " + err.Error()}
		}
		if res.Mode == "tmux" {
			return launchDoneMsg{status: "launched " + res.Command + " → tmux " + res.Session + " (press a to attach)"}
		}
		return launchDoneMsg{status: "launched " + res.Command + " → background, log " + res.LogPath}
	}
}

// suggestRelease proposes a merged-entry name from a common theme across the
// slugs (the longest shared leading word run), else a generic fallback.
func suggestRelease(slugs []string) string {
	if len(slugs) == 0 {
		return "release"
	}
	if len(slugs) == 1 {
		return slugs[0]
	}
	parts := make([][]string, len(slugs))
	for i, s := range slugs {
		parts[i] = strings.Split(s, "-")
	}
	var common []string
	for i := 0; ; i++ {
		if i >= len(parts[0]) {
			break
		}
		word := parts[0][i]
		same := true
		for _, p := range parts[1:] {
			if i >= len(p) || p[i] != word {
				same = false
				break
			}
		}
		if !same {
			break
		}
		common = append(common, word)
	}
	if len(common) > 0 {
		return strings.Join(common, "-")
	}
	sorted := append([]string(nil), slugs...)
	sort.Strings(sorted)
	return sorted[0] + "-plus-" + strconv.Itoa(len(slugs)-1)
}
