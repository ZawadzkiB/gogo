package tui

import (
	"path/filepath"

	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	"github.com/charmbracelet/lipgloss"
)

// palette.go is the tui side of the cockpit-colors model: it turns a persisted color
// hex (from projects.Source.Color / projects.Project.Color, or a blank) into a terminal
// color that is ADAPTIVE when the hex matches a palette swatch, DIRECT for an arbitrary
// hand-typed hex, and a deterministic never-blank fallback when blank (D2=A). The single
// source of truth for the swatches is projects.Swatches (pure strings) — this file only
// pairs them into lipgloss colors.

// colorFor resolves a stored color hex + a name-stable fallback into a terminal color
// that is NEVER blank (D2=A):
//   - hex matches a palette swatch → lipgloss.AdaptiveColor{Light,Dark} (adaptive,
//     consistent with styles.go)
//   - hex is an arbitrary user value → lipgloss.Color(hex) (back-compat, direct)
//   - hex == "" → the swatch at ColorForName(fallbackName), rendered adaptively — a
//     name-stable fallback (REV-002), so a colorless entity's dot never shifts on reorder
//
// So a source/project dot never renders grey/blank, whatever the stored value.
func colorFor(hex, fallbackName string) lipgloss.TerminalColor {
	if hex == "" {
		hex = projects.ColorForName(fallbackName)
	}
	if sw, ok := projects.LookupSwatch(hex); ok {
		return lipgloss.AdaptiveColor{Light: sw.Light, Dark: sw.Dark}
	}
	return lipgloss.Color(hex)
}

// sourceColor resolves the terminal color for a source LABEL (never blank). The
// project-board constructor pre-resolves every source into m.sourceColors (own color, or
// the name-stable ColorForName fallback when blank); an unknown label falls back to the
// same name-stable swatch via colorFor.
func (m Model) sourceColor(label string) lipgloss.TerminalColor {
	return colorFor(m.sourceColors[label], label)
}

// projectColor resolves the terminal color for a project NAME (never blank), mirroring
// sourceColor over the pre-resolved m.projectColors map.
func (m Model) projectColor(name string) lipgloss.TerminalColor {
	return colorFor(m.projectColors[name], name)
}

// originDots renders the D5 origin cue: a project dot then a source dot (`● ●`) for a
// multi-project surface (the config switcher / the future unified board), or a single
// dot when projectColor is nil (single-project surface). `plain` drops the tint for a
// focus-filled row (whose one fg/bg fill would otherwise punch a hole through a colored
// dot). No TTY under `go test` → lipgloss emits plain text, so the dots stay
// substring-assertable.
func originDots(projectColor, sourceColor lipgloss.TerminalColor, plain bool) string {
	dot := func(c lipgloss.TerminalColor) string {
		if plain || c == nil {
			return "●"
		}
		return lipgloss.NewStyle().Foreground(c).Render("●")
	}
	if projectColor == nil {
		return dot(sourceColor)
	}
	return dot(projectColor) + " " + dot(sourceColor)
}

// swatchName names a resolved hex for the config "label color · <name>" field: the
// swatch name when it matches the palette, else the raw hex (a hand-typed value).
func swatchName(hex string) string {
	if sw, ok := projects.LookupSwatch(hex); ok {
		return sw.Name
	}
	return hex
}

// sourceColorMap builds the source-label → RESOLVED-never-blank color-hex lookup the
// project board tints cards with (cockpit-colors FR2): a source's own Color when set,
// else the name-stable ColorForName(label) fallback (REV-002), so a colorless source
// NEVER yields a grey/blank dot AND keeps its hue when the source list reorders. Keyed by
// the source label (Name, else the path base). Shared by the constructor and the
// config-tab refresh so the two never drift.
func sourceColorMap(sources []projects.Source) map[string]string {
	colors := make(map[string]string, len(sources))
	for _, s := range sources {
		name := s.Name
		if name == "" {
			name = filepath.Base(s.Path)
		}
		if s.Color != "" {
			colors[name] = s.Color
		} else {
			colors[name] = projects.ColorForName(name)
		}
	}
	return colors
}

// projectColorMap builds the project-name → RESOLVED-never-blank color-hex lookup for the
// multi-project combination (the config switcher, D5): a project's own Color when set,
// else the name-stable ColorForName(name) fallback (REV-002). Never yields a blank dot,
// and a colorless project keeps its hue across a reorder.
func projectColorMap(projs []projects.Project) map[string]string {
	colors := make(map[string]string, len(projs))
	for _, p := range projs {
		if p.Color != "" {
			colors[p.Name] = p.Color
		} else {
			colors[p.Name] = projects.ColorForName(p.Name)
		}
	}
	return colors
}
