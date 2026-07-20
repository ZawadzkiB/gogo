package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/orchestrator"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// launchDoneMsg carries the outcome of a launch back to the model.
type launchDoneMsg struct{ status string }

// attemptAction resolves the requested action into a launch intent or a bounce
// reason (the status-line hint). Pure — the unit-tested move-guard core.
//
//	ship=false (m): plan-pending → accept · other unfinished (plan-accepted) → go · in-progress → go(resume) · ready → done · shipped → bounce
//	ship=true  (d): ready(+selection) → done · everything else → bounce
func (m *Model) attemptAction(ship bool) (launch.Intent, bool, string) {
	sel := m.selectedFeatures()
	if len(sel) > 0 {
		// A ready selection ships as one (merged if >1) entry — but ONLY within a
		// single project. The merged /gogo:done launches in ONE repo (doLaunch anchors
		// it at the intent's Root), so a selection spanning >1 project would silently
		// run the other projects' slugs in the wrong repo. Cross-repo merged ship is
		// later-phase work (P4 correlation id); Phase 1 refuses it with a clear bounce
		// and launches nothing (REV-001). Computed from the actual selected features
		// (composite-keyed), never a slug re-lookup that collapses a same-slug pair.
		if selectionSpansProjects(sel) {
			return launch.Intent{}, false, "select ready cards from one project to ship together"
		}
		slugs := make([]string, len(sel))
		for i, f := range sel {
			slugs[i] = f.Slug
		}
		in := launch.BuildIntent(launch.ActionDone, slugs, "")
		in.Root = m.rootFor(sel[0]) // all share one root (the guard above)
		return in, true, ""
	}

	f := m.focusedCard()
	if f == nil {
		return launch.Intent{}, false, "no card focused"
	}

	if ship {
		if f.Class != contract.ClassReadyToShip {
			return launch.Intent{}, false, "only ready cards can ship (d) — " + f.Slug + " is " + f.Class
		}
		return m.intentFor(launch.ActionDone, f), true, ""
	}

	switch f.Class {
	case contract.ClassUnfinished:
		// A plan-pending card's legal `m` move is ACCEPT — route it to the launched
		// /gogo:accept (FR-C2), not the /gogo:go that bounces without plan-accepted
		// (the dead end this closes). Accept is uncapped (it doesn't drive build work).
		// Every other unfinished card (incl. an accepted plan awaiting its first build)
		// goes — behind the concurrency-cap guard. Branch on status, not class, because
		// both share ClassUnfinished.
		if f.Status == "awaiting-plan-acceptance" {
			return m.intentFor(launch.ActionAccept, f), false, ""
		}
		if bounce := m.capBounce(f); bounce != "" {
			return launch.Intent{}, false, bounce
		}
		return m.intentFor(launch.ActionGo, f), false, ""
	case contract.ClassInProgress:
		if bounce := m.capBounce(f); bounce != "" {
			return launch.Intent{}, false, bounce
		}
		return m.intentFor(launch.ActionGo, f), false, ""
	case contract.ClassReadyToShip:
		return m.intentFor(launch.ActionDone, f), true, ""
	case contract.ClassShipped:
		return launch.Intent{}, false, "already shipped — no move (illegal)"
	}
	return launch.Intent{}, false, "no legal move for " + f.Slug
}

// intentFor builds a single-card launch intent for f, stamping its OWN repo root (via
// rootFor) so the launch anchors at the FOCUSED card's source — never a slug re-lookup
// that could grab a same-slug card from another project on the unified board (REV-001).
// FR4: a go-launch also carries the SOURCE's gate-skip params (--skip-acceptance /
// --skip-uat), resolved by the card's own root through the same shared resolver the
// cap guard uses (so the board and `gogo go` never drift); they are visible in the
// launch confirmation. Absent flags → the command is byte-for-byte today's.
func (m *Model) intentFor(action launch.Action, f *contract.Feature) launch.Intent {
	in := launch.BuildIntent(action, []string{f.Slug}, "")
	in.Root = m.rootFor(f)
	if action == launch.ActionGo {
		planSkip, uatSkip := projects.SkipForSource(m.capWatchSources(), in.Root)
		in.Command += launch.SkipParams(planSkip, uatSkip)
	}
	return in
}

// selectionSpansProjects reports whether the selected ready features resolve to more
// than one distinct repo root. A merged ship builds a SINGLE /gogo:done anchored at one
// root, so a selection crossing project roots would mis-root every other project's slug.
// Phase 1 refuses such a ship (REV-001); a per-root fan-out is later-phase work. It reads
// the ACTUAL selected features (composite-keyed by the caller), never a slug re-lookup —
// on the unified board a slug re-lookup collapses a same-slug cross-project pair into one
// and would let the guard silently pass. Single-repo mode never trips this (every feature
// shares one root), so its ship path is byte-for-byte unchanged.
func selectionSpansProjects(feats []*contract.Feature) bool {
	root, seen := "", false
	for _, f := range feats {
		if f == nil {
			continue
		}
		if !seen {
			root, seen = f.Root, true
			continue
		}
		if f.Root != root {
			return true
		}
	}
	return false
}

// capBounce returns a status-line bounce when launching a `go` on f would exceed
// its source's concurrency cap (FR4/FR5) — the board analog of cmdGo's capBlock.
// BOTH launch paths enforce the SAME orchestrator.CapExceeded rule (over the one
// shared pure helper, CapForSource) so they never drift: two live build sessions
// clobber a repo's shared working tree. It resolves the cap from EVERY project's
// SOURCES on the unified board (else the focused project's — capWatchSources, FR5), by
// the target feature's OWN root (rootFor), and counts that
// root's active in-progress+live features EXCLUDING f itself (so resuming an
// already-active feature is never blocked). Empty when uncapped / unregistered — the
// byte-for-byte single-repo fallback (a lone repo has no sources, so CapForSource
// returns 0). Read-side only; it writes nothing and composes with the one-owner
// lock. Over the cap it drops the user to `gogo go --force` (the CLI escape hatch),
// matching the selectionSpansProjects bounce style.
func (m *Model) capBounce(f *contract.Feature) string {
	if f == nil {
		return ""
	}
	root := m.rootFor(f)
	// FR5: resolve the cap from EVERY project's sources on the unified board — a card's
	// source may live in a non-focused project — else the focused project's (byte-for-byte).
	cap := orchestrator.CapForSource(m.capWatchSources(), root)
	if cap <= 0 {
		return ""
	}
	active := orchestrator.ActiveWorkSlugs(m.repo, root, m.sessions, f.Slug)
	if !orchestrator.CapExceeded(cap, len(active)) {
		return ""
	}
	return fmt.Sprintf("cap %d reached — already building %s; ship one or run `gogo go %s --force`",
		cap, strings.Join(active, ", "), f.Slug)
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
	// Name the target REPO (intent.Root) so a mis-anchored launch is catchable at the
	// confirm — on the unified board a same-slug card in another project must never be
	// launched into the wrong repo unnoticed (REV-001). Empty root falls back silently
	// (single-repo mode, where the CLI roots the launch itself).
	at := ""
	if intent.Root != "" {
		at = "  at " + intent.Root
	}
	// FR8: state the effective permission mode the launch runs under.
	return "will run: claude \"" + intent.Command + "\"  in " + where + at + "  · " + launch.PermissionSummary()
}

// doLaunch rebuilds the intent with the (possibly edited) release name and
// spawns Claude via the injected launcher. Returns a command that reports the
// outcome. The caller (updateForm) clears the consumed selection/pending state
// on the model it returns — this closure only captures the resolved intent.
func (m Model) doLaunch() tea.Cmd {
	intent := m.pending
	// The launch anchors at the intent's OWN Root — captured from the FOCUSED / selected
	// card when the intent was built (attemptAction). NEVER re-resolve the root by a slug
	// re-lookup: on the unified board a slug is unique only per-source, so the first
	// match could be a same-slug card in the WRONG project and silently launch there
	// (REV-001). In single-repo mode this Root == m.root, so the resolution is unchanged.
	root := intent.Root
	if m.pendingShip && len(intent.Slugs) >= 2 && m.binding != nil {
		rebuilt := launch.BuildIntent(launch.ActionDone, intent.Slugs, m.binding.release)
		rebuilt.Root = root // preserve the captured root across the release-name rebuild
		intent = rebuilt
	}
	if root == "" {
		// No target root captured (the feature vanished between confirm and launch, or a
		// bare intent). Never launch relative to the process cwd — bounce, launching
		// nothing (REV-004).
		return func() tea.Msg {
			return launchDoneMsg{status: "feature no longer present, nothing launched"}
		}
	}
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
