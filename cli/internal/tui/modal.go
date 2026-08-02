package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// A terminal has no native modal: a TUI "modal" is a COMPOSITE — render the
// background, render a box, splice the box's cells over the background's.
// These helpers are pure (strings in, strings out, no Model) so the treatment
// can be extended to other form sites later (D2=B composited the board launch
// confirm only for now).
//
// Named minimums + margins (FR10-FR12). Below either minimum — or before the
// first WindowSizeMsg has arrived (width/height 0) — no modal is attempted:
// modalFormSize answers ok == false and the caller renders today's full-screen
// form byte-for-byte, handing the form the raw terminal size. ONE producer for
// layout and render, so the two can never disagree.
const (
	modalMinTermW = 60 // below these a framed dialog is worse than the full screen
	// modalMinTermH was 12 and MEASURED optimistic (REV-006): the confirm's title
	// is charged to the same clamped row budget as the options, and a real-length
	// repo root in the `at <root>` tail wraps it tall enough that the options (or
	// the merged-ship Launch/Cancel row) lost their rows up to height 14. At 15
	// every measured cell renders all its options; below it the full-screen
	// fallback shows them, exactly as 0.35.0 did at the same sizes.
	modalMinTermH = 15
	// modalMaxFormW is deliberately generous: existing tests assert substrings of
	// View() while a launch confirm is open, and a narrow wrap could break one
	// mid-phrase (the plan's named blast radius).
	modalMaxFormW = 120
	modalMarginW  = 8 // total dimmed-backdrop columns guaranteed beside the box
	modalMarginH  = 2 // total dimmed-backdrop rows guaranteed above+below the box
	modalChromeW  = 4 // the box's rounded border (2) + Padding(0,1) (2)
	modalChromeH  = 2 // the box's border rows
)

// modalBoxStyle frames the composited form (FR8). subtleBorder so the frame
// reads as chrome, not content; the border GLYPHS are the colour-free cue that
// survives a no-colour terminal and `go test` (FR13).
var modalBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(subtleBorder).
	Padding(0, 1)

// modalFormSize returns the INNER size to lay the modal's form out at — fed to
// huh as a WindowSizeMsg (P2: the one path huh recomputes width AND height
// together; Form.WithWidth is measured-broken, P1) — or ok == false when the
// terminal is too small (or unsized) for a modal at all: the FR12 fallback rule.
// The height handed to huh bounds the form and the box adds the fixed chrome,
// so the composite can never exceed the terminal (FR11) by construction.
func modalFormSize(termW, termH int) (w, h int, ok bool) {
	if termW < modalMinTermW || termH < modalMinTermH {
		return 0, 0, false
	}
	w = termW - modalMarginW - modalChromeW
	if w > modalMaxFormW {
		w = modalMaxFormW
	}
	h = termH - modalMarginH - modalChromeH
	return w, h, true
}

// overlayCenter composites box over bg on a termW x termH canvas (FR8, D3=A:
// strip + dim). Every background line is ANSI-STRIPPED and re-rendered through
// dimStyle — a plain line has no open SGR state, so nothing can bleed into the
// box, deleting the mid-escape splice bug class outright — then the box's rows
// are spliced over the middle: dim(left) + boxRow + dim(right). Column cuts use
// x/ansi, and lipgloss.Width IS ansi.StringWidth, so the cuts land on exactly
// the cells lipgloss drew. Every output line is exactly termW columns and the
// total height exactly termH (FR11).
func overlayCenter(bg, box string, termW, termH int) string {
	bgLines := strings.Split(bg, "\n")
	boxLines := strings.Split(box, "\n")
	boxW := lipgloss.Width(box)
	boxH := len(boxLines)

	top := (termH - boxH) / 2
	if top < 0 {
		top = 0
	}
	left := (termW - boxW) / 2
	if left < 0 {
		left = 0
	}

	out := make([]string, termH)
	for row := 0; row < termH; row++ {
		plain := ""
		if row < len(bgLines) {
			plain = ansi.Strip(bgLines[row])
		}
		plain = padToWidth(plain, termW)
		if bi := row - top; bi >= 0 && bi < boxH {
			l := ansi.Truncate(plain, left, "")
			r := ansi.TruncateLeft(plain, left+boxW, "")
			out[row] = dimStyle.Render(l) + padToWidth(boxLines[bi], boxW) + dimStyle.Render(r)
			continue
		}
		out[row] = dimStyle.Render(plain)
	}
	return strings.Join(out, "\n")
}

// padToWidth pads s with spaces to exactly `width` display columns (truncating
// when wider) — the canvas invariant every composite line holds.
func padToWidth(s string, width int) string {
	w := lipgloss.Width(s)
	switch {
	case w < width:
		return s + strings.Repeat(" ", width-w)
	case w > width:
		return ansi.Truncate(s, width, "")
	}
	return s
}
