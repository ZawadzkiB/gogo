// Package tui is the Bubble Tea cockpit: a 4-column kanban board over the
// deterministic contract reader (plan | in progress | ready | changelog),
// drill-in file viewing (glamour / issues table / events timeline / ASCII
// diagrams), native `w` page builds, and column moves that launch Claude via
// the launch package (never mutating pipeline state directly). The model's
// Update/View are pure and unit-tested without a tty.
package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

type mode int

const (
	modeBoard mode = iota
	modeDrill
	modeViewer
	modeForm
)

// formBinding holds the huh field targets behind a pointer so the bindings stay
// valid as the value-type Model is copied between Update calls. Binding huh's
// .Value() directly to a field of the Model (a value receiver copies the struct
// on every Update) would leave the form writing to an orphaned copy, so a
// confirmed launch would read a stale false and silently cancel. TEST-001.
type formBinding struct {
	release string
	confirm bool
}

// columnOrder / columnTitles fix the 4-column layout left→right.
var (
	columnOrder  = [4]string{contract.ColPlan, contract.ColInProgress, contract.ColReady, contract.ColChangelog}
	columnTitles = [4]string{"plan", "in progress", "ready", "changelog"}
)

// Model is the whole cockpit state.
type Model struct {
	root string
	repo *contract.Repo

	cols      [4][]*contract.Feature
	colIdx    int
	cardIdx   [4]int
	colOffset [4]int          // per-column scroll offset (first visible card) — TEST-014
	selected  map[string]bool // selected ready-to-ship slugs (space)

	filter    string
	filtering bool
	status    string

	width, height int
	mode          mode

	sessions []string

	// drill-in
	drill     *contract.Feature
	artifacts []contract.Artifact
	artIdx    int

	// viewer
	viewport      viewport.Model
	viewerTitle   string
	viewerReady   bool
	viewerLoading bool              // TEST-003: async render in flight (spinner shown)
	curArtifact   contract.Artifact // the artifact currently open/loading (for width re-render)
	spinner       spinner.Model     // loading spinner while a viewer render runs
	renderCache   map[string]string // rendered content by (kind|path|width) — instant reopen
	dark          bool              // terminal background, detected ONCE before the program starts

	// form
	form        *huh.Form
	pending     launch.Intent
	pendingShip bool
	binding     *formBinding // heap-stable targets for the live huh fields

	// launcher spawns a confirmed intent. A seam (defaults to launch.Launch) so
	// the form lifecycle can be driven with a fake in tests — never nil once a
	// Model comes from New.
	launcher func(root string, in launch.Intent) (launch.Result, error)

	hasTmux, hasClaude, hasGlow bool
	reloadCh                    chan struct{}
	watch                       *watchSet // long-lived fsnotify handle (set by Init)
}

// New loads the repo at root and builds the initial board. It does NOT start
// fsnotify (that happens in Init) so tests can drive Update directly.
func New(root string) Model {
	repo, _ := contract.LoadRepo(root)
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	sp.Style = lipgloss.NewStyle().Foreground(columnAccent[0])
	m := Model{
		root:        root,
		repo:        repo,
		selected:    map[string]bool{},
		mode:        modeBoard,
		hasTmux:     launch.HasTmux(),
		hasClaude:   launch.HasClaude(),
		hasGlow:     launch.HasGlow(),
		launcher:    launch.Launch,
		reloadCh:    make(chan struct{}, 1),
		viewport:    viewport.New(0, 0),
		spinner:     sp,
		renderCache: map[string]string{},
		// Detect the terminal background ONCE here — before tea.Program grabs the
		// TTY — and pass an explicit glamour style thereafter. The freeze (TEST-003)
		// was glamour's WithAutoStyle re-querying the terminal (termenv OSC-11 +
		// 5s timeout) on EVERY view while Bubble Tea owned stdin; this makes it a
		// single, safe, cached detection.
		dark: lipgloss.HasDarkBackground(),
	}
	m.sessions = launch.ListSessions()
	m.rebuild()
	return m
}

// Init starts the fsnotify watch loop, the reload waiter, and the session
// ticker (TEST-006 — keeps the card session dots fresh between reloads).
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.startWatchCmd(), waitForReload(m.reloadCh), sessionTick())
}

// rebuild partitions the (filtered) features into the four columns and clamps
// focus indices.
func (m *Model) rebuild() {
	var cols [4][]*contract.Feature
	for _, f := range m.repo.Features {
		if m.filter != "" && !matchFilter(f, m.filter) {
			continue
		}
		switch f.Column() {
		case contract.ColPlan:
			cols[0] = append(cols[0], f)
		case contract.ColInProgress:
			cols[1] = append(cols[1], f)
		case contract.ColReady:
			cols[2] = append(cols[2], f)
		case contract.ColChangelog:
			cols[3] = append(cols[3], f)
		}
	}
	m.cols = cols
	for i := range m.cardIdx {
		m.cardIdx[i] = clamp(m.cardIdx[i], 0, len(cols[i])-1)
	}
	m.colIdx = clamp(m.colIdx, 0, 3)
}

// reload re-reads the repo + sessions and rebuilds, preserving filter/focus.
func (m *Model) reload() {
	if repo, err := contract.LoadRepo(m.root); err == nil {
		m.repo = repo
	}
	m.sessions = launch.ListSessions()
	m.rebuild()
}

// refocus restores the cursor to a slug within the currently focused column
// after a reload (features can be added/removed, shifting indices). If the slug
// still lives in the column the cursor follows it (so the window keeps it
// visible after the reflow); otherwise the index is clamped into range. TEST-014.
func (m *Model) refocus(slug string) {
	col := m.cols[m.colIdx]
	if slug != "" {
		for j, f := range col {
			if f.Slug == slug {
				m.cardIdx[m.colIdx] = j
				return
			}
		}
	}
	m.cardIdx[m.colIdx] = clamp(m.cardIdx[m.colIdx], 0, len(col)-1)
}

func (m *Model) focusedCard() *contract.Feature {
	col := m.cols[m.colIdx]
	if len(col) == 0 {
		return nil
	}
	return col[clamp(m.cardIdx[m.colIdx], 0, len(col)-1)]
}

func (m *Model) selectedSlugs() []string {
	var out []string
	for slug, on := range m.selected {
		if on {
			out = append(out, slug)
		}
	}
	sort.Strings(out)
	return out
}

func matchFilter(f *contract.Feature, q string) bool {
	q = strings.ToLower(q)
	return strings.Contains(strings.ToLower(f.Slug+" "+f.Title), q)
}

// badge returns the card's live status badge. Precedence:
//
//  1. waiting-for-user — a parked decision gate (state.md status) always wins.
//  2. running — a live tmux/claude session for this slug.
//  3. state.md is the current-phase source of truth (it drives the card's
//     column). The latest events.jsonl line only ENRICHES the badge with a
//     round, and only when its phase agrees with state.md's current phase
//     (mapping state.md's fifth phase "knowledge" → events' "report"). When the
//     telemetry lags state.md — a gap docs/cli-contract.md §5 calls normal — the
//     badge is derived from state.md alone (phase + the iterations round), so it
//     never disagrees with its own column.
//
// Older/raw features with no state.md phase fall back to the latest event, then
// to the state.md status, so a bare events-only feature still shows something.
func badge(f *contract.Feature, sessions []string) string {
	if f.WaitingForUser() {
		return "waiting-for-user"
	}
	if hasLiveSession(f.Slug, sessions) {
		return "running"
	}
	phase := f.Phase
	if phase == "" {
		if e := f.LatestEvent; e != nil {
			return phaseRound(e.Phase, e.Round, e.HasRound)
		}
		return f.Status
	}
	// Round: prefer the latest event's round when it agrees with state.md's
	// current phase; otherwise the round recorded in state.md's iterations line.
	if e := f.LatestEvent; e != nil && e.HasRound && contract.EventsPhase(phase) == e.Phase {
		return phaseRound(phase, e.Round, true)
	}
	if r := f.RoundFor(phase); r > 0 {
		return phaseRound(phase, r, true)
	}
	if f.Status != "" && f.Status != phase {
		return f.Status
	}
	return phase
}

// phaseRound renders a phase badge with an optional "rN" round suffix.
func phaseRound(phase string, round int, hasRound bool) string {
	if hasRound {
		return fmt.Sprintf("%s r%d", phase, round)
	}
	return phase
}

func hasLiveSession(slug string, sessions []string) bool {
	for _, s := range sessions {
		if strings.Contains(s, slug) {
			return true
		}
	}
	return false
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// colTitleStyle is the top/section title style. Per-column card + accent styles
// live in styles.go (precomputed once). View stays substring-assertable because
// go test has no TTY, so lipgloss emits plain text.
var colTitleStyle = lipgloss.NewStyle().Bold(true)
