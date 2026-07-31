package tui

import "github.com/charmbracelet/lipgloss"

// waitingMarker is the leading glyph on a card that is WaitingForInput() — an
// unmissable "blocked on you" cue (matches the intended-design ⏸USER states,
// FR-B2). One rune, like the existing ●/○/✓ card markers, so truncate's
// rune-count math is unaffected.
const waitingMarker = "⏸"

// Derived-state card cues (0.29.0). Each is GLYPH + WORD, never colour alone: the
// Diagnosability bar says a distinction that colour carries is flattened by a no-colour
// terminal AND by TTY-less `go test`, so pairing the glyph with the word is what makes
// the cue survive anywhere and stay assertable in View().
//
//   - authoringMarker - the plan.md is not written yet, so this card is being AUTHORED.
//     Deliberately NOT the ⏸ gate glyph: an authoring item is not waiting on the user,
//     it is waiting on its own analyst, and it must not read like an acceptable plan.
//   - buildingMarker  - a live `gogo-go-<slug>` session contradicts a file-derived state
//     that has not caught up yet (the launch-to-first-write window). Reuses the green
//     `●` liveness glyph the name-row dot already uses, on the amber "in flight" tint.
//   - stalledMarker   - the mirror case: a WORKING status (implementing/reviewing/
//     testing) with NO live session, i.e. the session died mid-phase. `·` is a
//     deliberately quiet glyph - a stalled card is not an error, it is resumable.
const (
	authoringMarker = "✎"
	buildingMarker  = "●"
	stalledMarker   = "·"
)

// Status-line severity markers (FR3.2). Every cockpit outcome - a cap bounce, a
// dangling plan target, a tmux failure and a plain success - used to render
// through ONE faint-grey voice, so the user could not tell a blocked launch from
// a failed one. Colour alone would not fix it (a no-colour terminal, and `go
// test`'s TTY-less lipgloss, both flatten it), so each severity also carries a
// leading glyph: the distinction survives anywhere and is assertable in View().
// Success keeps NO marker, so today's success line is byte-for-byte unchanged.
const (
	statusErrMarker  = "✗ " // a failure - always carries the real error's words
	statusWarnMarker = "⚠ " // blocked / a gate - always carries the unblock
)

// TONE palette — per-column accents ported from the xplan/web tones, as lipgloss
// adaptive colors so light terminals stay readable (TEST-007). All styles here
// are precomputed ONCE (package init), never rebuilt per frame.
var columnAccent = [4]lipgloss.AdaptiveColor{
	{Light: "#2f6fe0", Dark: "#7aa8ff"}, // plan — blue
	{Light: "#b9721c", Dark: "#e6a14a"}, // in progress — amber
	{Light: "#2e8b57", Dark: "#5db97a"}, // ready — green
	{Light: "#6b6b6b", Dark: "#9aa0aa"}, // changelog — muted
}

var (
	subtleBorder = lipgloss.AdaptiveColor{Light: "#c9cdd6", Dark: "#3a3f4b"}
	focusBg      = lipgloss.AdaptiveColor{Light: "#eaf0fb", Dark: "#222834"}
	focusFg      = lipgloss.AdaptiveColor{Light: "#111418", Dark: "#f2f4f8"}
	selectAccent = lipgloss.AdaptiveColor{Light: "#1f9d55", Dark: "#5db97a"}
	sessionDot   = lipgloss.AdaptiveColor{Light: "#1f9d55", Dark: "#57d977"}
	dimText      = lipgloss.AdaptiveColor{Light: "#6b6b6b", Dark: "#9aa0aa"}
	titleText    = lipgloss.AdaptiveColor{Light: "#111418", Dark: "#e6e9ef"}
	waitAccent   = lipgloss.AdaptiveColor{Light: "#c0392b", Dark: "#ff6b6b"}
	uatAccent    = lipgloss.AdaptiveColor{Light: "#8250df", Dark: "#b392f0"} // awaiting-uat — purple

	// Redesign tokens (cockpit-redesign): the palette was already present; these
	// are the few genuinely-new ones the mockup's new elements need.
	secondaryText = lipgloss.AdaptiveColor{Light: "#3a4048", Dark: "#b7bdc9"} // light body on changelog/footer
	faintText     = lipgloss.AdaptiveColor{Light: "#9aa0aa", Dark: "#5f6572"} // changelog dates

	// Faint tinted chip backgrounds for the status pills — colored text on a faint
	// accent wash (the mockup's rounded chips). No TTY under `go test` → lipgloss
	// emits plain text, so the pill LABEL stays substring-assertable.
	redTint    = lipgloss.AdaptiveColor{Light: "#fbe9e7", Dark: "#2a1719"}
	amberTint  = lipgloss.AdaptiveColor{Light: "#fbf1e0", Dark: "#2a2113"}
	purpleTint = lipgloss.AdaptiveColor{Light: "#f0e9fb", Dark: "#1f1830"}
	dimTint    = lipgloss.AdaptiveColor{Light: "#eceef2", Dark: "#1b1f27"}
)

var (
	// Status-pill chips (FR-3): a colored label on a faint
	// tinted wash. Padding(0,1) gives the rounded-chip breathing room.
	pillRed    = lipgloss.NewStyle().Bold(true).Foreground(waitAccent).Background(redTint).Padding(0, 1)
	pillAmber  = lipgloss.NewStyle().Bold(true).Foreground(columnAccent[1]).Background(amberTint).Padding(0, 1)
	pillPurple = lipgloss.NewStyle().Bold(true).Foreground(uatAccent).Background(purpleTint).Padding(0, 1)
	pillDim    = lipgloss.NewStyle().Foreground(dimText).Background(dimTint).Padding(0, 1)

	// pillBuilding tints the `● building` disagreement chip (FR14): amber, the "work in
	// flight" family the in-progress column and the in-flight status pill already use -
	// it says "this is running", not "this is broken". Distinct from pillAmber only in
	// that it is not bold, so the chip reads as a note ON the pill rather than a second
	// pill competing with it.
	pillBuilding = lipgloss.NewStyle().Foreground(columnAccent[1]).Background(amberTint).Padding(0, 1)

	secondaryStyle = lipgloss.NewStyle().Foreground(secondaryText)
	faintStyle     = lipgloss.NewStyle().Foreground(faintText)

	// Status-line severity styles (FR3.2), over the palette the pills already use:
	// red = failed, amber = blocked/gate. The dim success voice stays statusStyle
	// (view.go) so every untouched call site is unchanged.
	statusErrStyle  = lipgloss.NewStyle().Bold(true).Foreground(waitAccent)
	statusWarnStyle = lipgloss.NewStyle().Bold(true).Foreground(columnAccent[1])

	// correlationChipStyle tints a card's ⛓ plan-… correlation chip(s) (FR14) — a muted
	// purple, distinct from the source tag's registry color, echoing the uat/gate
	// purple family without implying a gate. Plain text under `go test` (no TTY), so the
	// chip stays substring-assertable.
	correlationChipStyle = lipgloss.NewStyle().Foreground(uatAccent)

	// changelogFocusStyle is the selection bar for the focused collapsed-changelog
	// row: one focus fg+bg fill across the row (accent bg + bright fg), the analog
	// of the focused work card's highlight for a borderless list row.
	changelogFocusStyle = lipgloss.NewStyle().Foreground(focusFg).Background(focusBg).Bold(true)
	keyChipStyle        = lipgloss.NewStyle().Foreground(secondaryText).Background(focusBg).Padding(0, 1) // footer key-chips

	// Tab bar (FR8): the active tab is a bold accent chip, the others dim. Plain
	// text under `go test` (no TTY), so the tab LABELS stay substring-assertable.
	tabActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(focusFg).Background(focusBg).Padding(0, 1)
	tabStyle       = lipgloss.NewStyle().Foreground(dimText).Padding(0, 1)

	// Source filter chips (FR7): the active chip is a tinted accent wash, the others
	// dim. Substring-assertable like the tab bar.
	chipActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(focusFg).Background(focusBg).Padding(0, 1)
	chipStyle       = lipgloss.NewStyle().Foreground(dimText).Padding(0, 1)
)

// gateBorder is the card border for a card that needs the user: a heavy `┃` left
// edge (the mockup's left-accent stripe) recolored red (plan/decision) or purple
// (uat) via BorderLeftForeground. The heavy glyph is what makes the stripe
// substring-assertable (a flowing card keeps the plain `│`), independent of focus.
var gateBorder = func() lipgloss.Border {
	b := lipgloss.RoundedBorder()
	b.Left = "┃"
	return b
}()

// gateStripe is the left-stripe glyph a gate card carries (used by the test +
// the assertable-element check); "" for a flowing card.
const gateStripe = "┃"

// colStyleSet is the precomputed card/header frame styles for one column.
type colStyleSet struct {
	header       lipgloss.Style
	card         lipgloss.Style // normal (subtle border)
	cardFocused  lipgloss.Style // full-card highlight: accent border + subtle bg
	cardSelected lipgloss.Style // selected-for-ship: accent border
}

var (
	columnStyles [4]colStyleSet

	slugStyle    = lipgloss.NewStyle().Bold(true).Foreground(titleText)
	dimStyle     = lipgloss.NewStyle().Foreground(dimText)
	sessionStyle = lipgloss.NewStyle().Bold(true).Foreground(sessionDot)
	selMarkStyle = lipgloss.NewStyle().Bold(true).Foreground(selectAccent)
	helpStyle    = lipgloss.NewStyle().Foreground(dimText)
	sepStyle     = lipgloss.NewStyle().Foreground(subtleBorder) // vertical column separators (FR-B4)
)

func init() {
	base := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	for i := 0; i < 4; i++ {
		accent := columnAccent[i]
		columnStyles[i] = colStyleSet{
			header:       lipgloss.NewStyle().Bold(true).Foreground(accent),
			card:         base.BorderForeground(subtleBorder),
			cardFocused:  base.BorderForeground(accent).Background(focusBg).Foreground(focusFg).Bold(true),
			cardSelected: base.BorderForeground(selectAccent),
		}
	}
}
