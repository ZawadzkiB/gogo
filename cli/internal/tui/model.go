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
	"strconv"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/orchestrator"
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

	showAllKeys bool // FR-10: ? toggles the full key list under the contextual footer

	width, height int
	mode          mode

	sessions []string

	// drill-in
	drill     *contract.Feature
	artifacts []contract.Artifact
	artIdx    int

	// drill-in CARD detail (Slice B — FR-B1/B2/B4): the card's session rows
	// (registry ⨯ live-tmux cross-check) and a compact recent-events tail,
	// (re)computed by openDrill/loadDrillCard. Description / folder / status are
	// derived from m.drill at render time (no cache — they already live there).
	drillSessions   []sessionRow
	drillEventsTail string

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
	form          *huh.Form
	pending       launch.Intent
	pendingShip   bool
	pendingDelete *contract.Feature // FR6: the card a confirmed `x` moves to trash
	pendingKill   []string          // FR-B3: the drill card's live session(s) a confirmed `K` kills
	binding       *formBinding      // heap-stable targets for the live huh fields

	// peek (FR7): a read-only session-log viewer reusing the async viewer.
	peeking     bool   // the open viewer is a session-log peek (r re-captures)
	peekSlug    string // the card being peeked
	peekSession string // live tmux session name, or "" for a background-log peek
	peekLog     string // background -p log path, or ""

	// capturer snapshots a session's pane for a peek. A seam (defaults to
	// launch.CapturePane) so peek can be driven in tests without real tmux.
	capturer func(session string, lines int) (string, error)

	// launcher spawns a confirmed intent. A seam (defaults to launch.Launch) so
	// the form lifecycle can be driven with a fake in tests — never nil once a
	// Model comes from New.
	launcher func(root string, in launch.Intent) (launch.Result, error)

	// killer kills a live tmux session by exact name (defaults to
	// launch.KillSession) and registry loads a feature's persistent-session
	// registry (defaults to orchestrator.LoadRegistry). Seams (FR-B3/B5) so the
	// drill-in kill wiring + the session-row reader are asserted with fakes, no
	// real tmux/registry file — never nil once a Model comes from New.
	killer   func(session string) error
	registry func(root, slug string) *orchestrator.Registry

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
		capturer:    launch.CapturePane,
		killer:      launch.KillSession,
		registry:    orchestrator.LoadRegistry,
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

// badge returns the card's true pipeline STATUS — never a session-liveness word.
// "running" is NOT a status: whether a tmux/claude session is live is a separate
// signal (the green ● name-row dot + the header "● N session" count), decoupled
// here so the pill always reads the real state (a shipped card reads "shipped"
// even while its just-finished gogo-done-<slug> pane lingers; an in-flight card
// reads "review r2", not a "running" that hides its phase). Precedence:
//
//  1. waiting-for-user — a parked decision gate / mid-UAT re-plan (status always
//     wins; a re-plan stays waiting-for-user for the whole stretch, REV-004).
//  2. awaiting-uat — the UAT gate (0.11.0): phase ⑤ left the feature ready but
//     unshipped, pending the user's sign-off (state.md status awaiting-uat).
//  3. awaiting-plan-acceptance — the plan-acceptance gate: surfaced as its own
//     state name so a plan-pending card reads as a gate, not "plan r1" (FR-B2).
//  4. state.md is the current-phase source of truth (it drives the card's
//     column). The latest events.jsonl line only ENRICHES the badge with a
//     round, and only when its phase agrees with state.md's current phase
//     (mapping state.md's fifth phase "knowledge" → events' "report"). When the
//     telemetry lags state.md — a gap docs/cli-contract.md §5 calls normal — the
//     badge is derived from state.md alone (phase + the iterations round), so it
//     never disagrees with its own column. A shipped feature falls through to its
//     "shipped" status here (this is what un-hides it from the old "running").
//
// Older/raw features with no state.md phase fall back to the latest event, then
// to the state.md status, so a bare events-only feature still shows something.
func badge(f *contract.Feature) string {
	if f.WaitingForUser() {
		return "waiting-for-user"
	}
	if f.AwaitingUAT() {
		return "awaiting-uat"
	}
	// The plan-acceptance gate: surface its state name like the other two gates
	// (it had no distinct badge before — FR-B2). Mutually exclusive with the
	// statuses above, so this does not disturb their precedence.
	if f.Status == "awaiting-plan-acceptance" {
		return "awaiting-plan-acceptance"
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

// --- redesign: phase progress, status pills, gate enumeration (cockpit-redesign) ---
//
// One shared model, several thin renderers. phaseProgress(f) is the single source
// of truth (D2); the FR-4 dots and the FR-9 segmented bar both render it. badge()
// stays the canonical status producer; pillLabel/pillStyleFor transform it into
// the FR-3 chip. gates() enumerates the FR-8 needs-you inbox. All pure, all
// substring-assertable (no TTY under `go test` → lipgloss emits plain text).

// phaseState is one slot of the 5-phase progress vector.
type phaseState int

const (
	phasePending phaseState = iota // not started (zero value)
	phaseCurrent                   // the feature's current phase
	phaseDone                      // completed
)

// phaseGlyphs are the FR-4 dots — one circled digit per pipeline phase
// (plan/implement/review/test/report). An intentional non-ASCII exception
// (coding-rules "Style").
var phaseGlyphs = [5]string{"①", "②", "③", "④", "⑤"}

// phaseIndex maps a state.md phase name to its slot in the 5-phase vector, or -1
// when unknown. "done" (the terminal label) returns 5 → every slot done. events'
// "report" and state.md's "knowledge" are the same fifth phase.
func phaseIndex(phase string) int {
	switch phase {
	case "plan":
		return 0
	case "implement":
		return 1
	case "review":
		return 2
	case "test":
		return 3
	case "knowledge", "report":
		return 4
	case "done":
		return 5
	}
	return -1
}

// phaseIndexFromStatus derives the current phase slot from state.md's status when
// the phase line is absent (older/raw features), so the dots still light up.
func phaseIndexFromStatus(status string) int {
	switch status {
	case "awaiting-plan-acceptance", "plan-accepted":
		return 0
	case "implementing":
		return 1
	case "reviewing":
		return 2
	case "testing":
		return 3
	case "awaiting-uat":
		return 4
	}
	return -1
}

// phaseProgress is the single source of truth for a card's phase display (D2):
// every phase before the current is done, the current is current, the rest
// pending; a shipped feature is all-done; an unknown phase is all-pending (a
// safe, meaningful default — never a panic).
func phaseProgress(f *contract.Feature) [5]phaseState {
	var out [5]phaseState // zero value == phasePending
	if f.Class == contract.ClassShipped {
		for i := range out {
			out[i] = phaseDone
		}
		return out
	}
	idx := phaseIndex(f.Phase)
	if idx < 0 {
		idx = phaseIndexFromStatus(f.Status)
	}
	if idx < 0 {
		return out // unknown → all pending
	}
	if idx >= len(out) { // terminal "done" phase
		for i := range out {
			out[i] = phaseDone
		}
		return out
	}
	for i := 0; i < idx; i++ {
		out[i] = phaseDone
	}
	out[idx] = phaseCurrent
	return out
}

// phaseStyleFor maps a phase slot to its glyph/segment style.
func phaseStyleFor(s phaseState) lipgloss.Style {
	switch s {
	case phaseDone:
		return phaseDoneStyle
	case phaseCurrent:
		return phaseCurrentStyle
	default:
		return phasePendingStyle
	}
}

// phaseDots renders the FR-4 per-card element: five colored glyphs `①②③④⑤`.
func phaseDots(f *contract.Feature) string {
	var b strings.Builder
	for i, s := range phaseProgress(f) {
		b.WriteString(phaseStyleFor(s).Render(phaseGlyphs[i]))
	}
	return b.String()
}

// phaseDotsPlain is the uncolored dot run for a FOCUSED card, whose frame carries
// a single fg+bg fill (per-glyph colors would punch background holes).
func phaseDotsPlain() string { return strings.Join(phaseGlyphs[:], "") }

// isChangelogCol reports whether board column i is the collapsed changelog list.
func isChangelogCol(i int) bool { return columnOrder[i] == contract.ColChangelog }

// phaseBar renders the FR-9 flavor: a 5-segment progress bar over the SAME
// phaseProgress vector (D2). Filled `▓` for done/current (green/amber), faint
// `░` for pending — both substring-assertable.
func phaseBar(f *contract.Feature) string {
	parts := make([]string, 0, 5)
	for _, s := range phaseProgress(f) {
		glyph := "▓▓▓"
		if s == phasePending {
			glyph = "░░░"
		}
		parts = append(parts, phaseStyleFor(s).Render(glyph))
	}
	return strings.Join(parts, " ")
}

// pillLabel is the FR-3 chip text: badge() stays the canonical status producer
// (its tests + the drill/status line depend on it); this transform maps the gate
// states to their answer-first chip wording and passes everything else through.
// A mid-UAT re-plan (waiting-for-user carrying a "UAT round N" open-decision)
// reads "re-planning · UAT N" instead of the generic "decision", so the card says
// what the analyst is doing rather than looking like a stuck decision gate.
func pillLabel(f *contract.Feature) string {
	switch b := badge(f); b {
	case "awaiting-plan-acceptance":
		return waitingMarker + " accept plan"
	case "awaiting-uat":
		return waitingMarker + " awaiting-uat"
	case "waiting-for-user":
		if isUATReplan(f) {
			if n := uatRound(f); n > 0 {
				return fmt.Sprintf("%s re-planning · UAT %d", waitingMarker, n)
			}
			return waitingMarker + " re-planning"
		}
		return waitingMarker + " decision"
	default:
		return b // implement r1 · review r2 · plan-accepted · shipped · phase names
	}
}

// uatRound parses the round N from a mid-UAT re-plan's open-decision, which the
// orchestrator sets to "UAT round N" when it locks the gate (skills/gogo/SKILL.md).
// 0 when absent/unparseable — including a generic decision gate (open-decision
// "D<n>"), which is exactly how isUATReplan tells the two apart.
func uatRound(f *contract.Feature) int {
	od := strings.ToLower(f.OpenDecision)
	i := strings.Index(od, "uat")
	if i < 0 {
		return 0
	}
	// Return the first whole integer that appears after "uat". Scanning one past
	// the end closes a trailing digit run; start<0 keeps the run's first index so a
	// multi-digit round (e.g. "UAT round 10") is read whole.
	rest := od[i+3:]
	start := -1
	for j := 0; j <= len(rest); j++ {
		isDigit := j < len(rest) && rest[j] >= '0' && rest[j] <= '9'
		switch {
		case isDigit && start < 0:
			start = j
		case !isDigit && start >= 0:
			if n, err := strconv.Atoi(rest[start:j]); err == nil {
				return n
			}
			start = -1
		}
	}
	return 0
}

// isUATReplan reports whether a waiting-for-user card is parked in a UAT re-plan
// stretch (analysis → revision → re-acceptance) rather than a generic decision
// fork. The "UAT round N" open-decision is the precise discriminator.
func isUATReplan(f *contract.Feature) bool {
	return f.WaitingForUser() && uatRound(f) > 0
}

// pillStyleFor picks the tinted chip style for a card's status pill, mirroring
// badge()'s own precedence so the color always agrees with pillLabel: red for a
// decision/plan gate (incl. a UAT re-plan), purple for the uat gate, amber for an
// in-flight phase round, dim otherwise. Session liveness is NOT a status, so it
// does not tint the pill — the green ● name-row dot carries that signal instead.
func pillStyleFor(f *contract.Feature) lipgloss.Style {
	switch {
	case f.WaitingForUser():
		return pillRed
	case f.AwaitingUAT():
		return pillPurple
	case f.Status == "awaiting-plan-acceptance":
		return pillRed
	}
	switch f.Phase {
	case "implement", "review", "test":
		return pillAmber
	}
	return pillDim
}

// stripeAccent is the FR-5 left-stripe decision, independent of focus: purple for
// the uat gate, red for any other gate (plan-acceptance / decision), (nil,false)
// for a flowing card. The renderer recolors the heavy-`┃` gateBorder with it.
func stripeAccent(f *contract.Feature) (lipgloss.TerminalColor, bool) {
	switch {
	case f.AwaitingUAT():
		return uatAccent, true
	case f.WaitingForInput():
		return waitAccent, true
	}
	return nil, false
}

// gate is one needs-you inbox row (FR-8): a card parked at a user gate, its gate
// type + tinted pill, the one-line "what's blocked", the read-key label, the
// answer key, and where the card sits so a number key can jump focus to it.
type gate struct {
	feature   *contract.Feature
	kind      string         // "plan gate" | "uat gate" | "decision gate"
	kindStyle lipgloss.Style // the gate-type pill style
	blocked   string         // one-line what-s-blocked
	read      string         // the [N] read-key label, e.g. "read plan"
	answer    string         // the answer key, e.g. "[m] accept"
	col, row  int            // board coordinates (column, card index) for the jump
}

// gates enumerates the board's WaitingForInput() cards as inbox rows in board
// order (plan → in progress → ready → changelog, newest-first within each — the
// same order the columns render), so the number keys 1..N are stable.
func (m Model) gates() []gate {
	var out []gate
	for i := 0; i < 4; i++ {
		for j, f := range m.cols[i] {
			if !f.WaitingForInput() {
				continue
			}
			g := gateFor(f)
			g.col, g.row = i, j
			out = append(out, g)
		}
	}
	return out
}

// gateFor classifies one gate card into its inbox row (kind, pill, blurb, keys).
func gateFor(f *contract.Feature) gate {
	switch {
	case f.AwaitingUAT():
		return gate{feature: f, kind: "uat gate", kindStyle: pillPurple,
			blocked: "report done, awaiting your verification", read: "read report", answer: "[d] ship"}
	case f.Status == "awaiting-plan-acceptance":
		return gate{feature: f, kind: "plan gate", kindStyle: pillRed,
			blocked: "plan ready for your acceptance", read: "read plan", answer: "[m] accept"}
	case isUATReplan(f):
		// A mid-UAT re-plan: the analyst is revising the plan (or it is parked for
		// re-acceptance) — not a stuck decision fork. Say so, and route the read to
		// the plan being revised.
		return gate{feature: f, kind: "uat re-plan", kindStyle: pillRed,
			blocked: fmt.Sprintf("re-planning after UAT round %d — revising the plan", uatRound(f)),
			read:    "read plan", answer: "[m] resume"}
	default: // waiting-for-user — a parked decision fork
		// `m` is the board's go/resume key (move.go: in-progress → go/resume); `g`
		// has no board handler, so advertise the real key here (REV-001).
		return gate{feature: f, kind: "decision gate", kindStyle: pillRed,
			blocked: "parked on a decision — resume with your answer", read: "read state", answer: "[m] resume"}
	}
}

func hasLiveSession(slug string, sessions []string) bool {
	return liveSessionFor(slug, sessions) != ""
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
