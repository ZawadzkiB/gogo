package tui

import (
	"hash/fnv"
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

// colorFor resolves a stored color hex + a stable fallback index into a terminal color
// that is NEVER blank (D2=A):
//   - hex matches a palette swatch → lipgloss.AdaptiveColor{Light,Dark} (adaptive,
//     consistent with styles.go)
//   - hex is an arbitrary user value → lipgloss.Color(hex) (back-compat, direct)
//   - hex == "" → the swatch at ColorForIndex(fallbackIdx), rendered adaptively
//
// So a source/project dot never renders grey/blank, whatever the stored value.
func colorFor(hex string, fallbackIdx int) lipgloss.TerminalColor {
	if hex == "" {
		hex = projects.ColorForIndex(fallbackIdx)
	}
	if sw, ok := projects.LookupSwatch(hex); ok {
		return lipgloss.AdaptiveColor{Light: sw.Light, Dark: sw.Dark}
	}
	return lipgloss.Color(hex)
}

// stableIndex hashes a label to a deterministic non-negative index — the fallback index
// for an entity whose color is blank AND which is not in the (position-indexed) color
// map (e.g. a stray feature source on the aggregate board). ColorForIndex wraps it into
// range, so the same label always resolves to the same swatch.
func stableIndex(label string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(label))
	return int(h.Sum32())
}

// sourceColor resolves the terminal color for a source LABEL (never blank). The
// project-board constructor pre-resolves every source into m.sourceColors (own color, or
// ColorForIndex(position) when blank); an unknown label falls back to a hashed swatch.
func (m Model) sourceColor(label string) lipgloss.TerminalColor {
	return colorFor(m.sourceColors[label], stableIndex(label))
}

// projectColor resolves the terminal color for a project NAME (never blank), mirroring
// sourceColor over the pre-resolved m.projectColors map.
func (m Model) projectColor(name string) lipgloss.TerminalColor {
	return colorFor(m.projectColors[name], stableIndex(name))
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
// else the deterministic ColorForIndex(position) fallback, so a colorless source NEVER
// yields a grey/blank dot. Keyed by the source label (Name, else the path base). Shared
// by the constructor and the config-tab refresh so the two never drift.
func sourceColorMap(sources []projects.Source) map[string]string {
	colors := make(map[string]string, len(sources))
	for i, s := range sources {
		name := s.Name
		if name == "" {
			name = filepath.Base(s.Path)
		}
		if s.Color != "" {
			colors[name] = s.Color
		} else {
			colors[name] = projects.ColorForIndex(i)
		}
	}
	return colors
}

// projectColorMap builds the project-name → RESOLVED-never-blank color-hex lookup for the
// multi-project combination (the config switcher, D5): a project's own Color when set,
// else ColorForIndex(position). Never yields a blank dot.
func projectColorMap(projs []projects.Project) map[string]string {
	colors := make(map[string]string, len(projs))
	for i, p := range projs {
		if p.Color != "" {
			colors[p.Name] = p.Color
		} else {
			colors[p.Name] = projects.ColorForIndex(i)
		}
	}
	return colors
}
