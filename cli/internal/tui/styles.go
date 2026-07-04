package tui

import "github.com/charmbracelet/lipgloss"

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
)

// colStyleSet is the precomputed card/header frame styles for one column.
type colStyleSet struct {
	header       lipgloss.Style
	card         lipgloss.Style // normal (subtle border)
	cardFocused  lipgloss.Style // full-card highlight: accent border + subtle bg
	cardSelected lipgloss.Style // selected-for-ship: accent border
	badge        lipgloss.Style
}

var (
	columnStyles [4]colStyleSet

	slugStyle    = lipgloss.NewStyle().Bold(true).Foreground(titleText)
	dimStyle     = lipgloss.NewStyle().Foreground(dimText)
	sessionStyle = lipgloss.NewStyle().Bold(true).Foreground(sessionDot)
	selMarkStyle = lipgloss.NewStyle().Bold(true).Foreground(selectAccent)
	waitStyle    = lipgloss.NewStyle().Bold(true).Foreground(waitAccent)
	uatStyle     = lipgloss.NewStyle().Bold(true).Foreground(uatAccent)
	helpStyle    = lipgloss.NewStyle().Foreground(dimText)
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
			badge:        lipgloss.NewStyle().Foreground(accent),
		}
	}
}
