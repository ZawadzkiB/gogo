package projects

import (
	"hash/fnv"
	"strings"
)

// palette.go is the ONE source of truth for the cockpit's per-project / per-source
// origin colors (cockpit-colors, D2/D2.1). It lives in the `projects` package — the
// data layer that already owns Source.Color, Project.Color and the legacy color
// migration — as pure strings with NO lipgloss dependency, so the three consumers
// (this package, `main` for assignment, and `tui` for rendering) share it with no
// enum-sync drift. The tui side turns a persisted hex into an adaptive terminal
// color; here we only mint + look up plain hexes.

// Swatch is one named palette color: a Dark hex (the design's canonical value, what
// gets persisted) and a Light hex (the adaptive light-terminal variant the tui pairs
// it with). Pure strings — no lipgloss here.
type Swatch struct{ Name, Light, Dark string }

// Swatches is the curated, hue-spaced palette (D2.1). The design's teal/pink/blue are
// verbatim; the rest reuse styles.go hues. Deliberately avoids a pure alert-red so a
// source dot never reads as "needs you". Order is stable — AssignColor / ColorForIndex
// round-robin over it, so this order is part of the deterministic contract.
var Swatches = []Swatch{
	{Name: "blue", Dark: "#58a6ff", Light: "#2f6fe0"},
	{Name: "teal", Dark: "#35c9b5", Light: "#0f9e8c"},
	{Name: "cyan", Dark: "#4fc3e0", Light: "#0e8bb0"},
	{Name: "green", Dark: "#5db97a", Light: "#2e8b57"},
	{Name: "amber", Dark: "#e6a14a", Light: "#b9721c"},
	{Name: "coral", Dark: "#f4826b", Light: "#cf5136"},
	{Name: "pink", Dark: "#eb7bb5", Light: "#c14b8a"},
	{Name: "purple", Dark: "#b392f0", Light: "#8250df"},
}

// AssignColor returns the first palette Dark hex not already in taken (a deterministic
// round-robin that skips collisions), so freshly-added projects/sources fan out across
// the palette. When every swatch is taken (more entities than swatches) it wraps
// deterministically by the taken count, so the result is stable across runs.
func AssignColor(taken []string) string {
	used := make(map[string]bool, len(taken))
	for _, c := range taken {
		if c = normalizeHex(c); c != "" {
			used[c] = true
		}
	}
	for _, sw := range Swatches {
		if !used[normalizeHex(sw.Dark)] {
			return sw.Dark
		}
	}
	return ColorForIndex(len(taken)) // all taken → wrap deterministically
}

// ColorForIndex is the deterministic never-blank fallback: the palette Dark hex at
// position i (wrapping), so an entity created before this feature (blank color) still
// renders a stable, non-blank origin dot. Negative indices wrap too.
func ColorForIndex(i int) string {
	n := len(Swatches)
	return Swatches[((i%n)+n)%n].Dark
}

// ColorForName is the NAME-stable never-blank fallback (REV-002): the palette Dark hex at
// fnv(name) % len(Swatches). Unlike ColorForIndex (keyed on slice POSITION, so a colorless
// entity's dot shifts hue when the project/source list reorders), it derives the color
// from the entity's own name, so a colorless project/source keeps the SAME origin color
// across a reorder. Deterministic and never blank.
func ColorForName(name string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return ColorForIndex(int(h.Sum32()))
}

// TakenColors gathers every non-blank origin color in use across projs — each
// Project.Color and each of its Source.Color — the `taken` set AssignColor round-robins
// around so a freshly-added project/source fans out to the next free palette swatch
// (cockpit-colors FR2). ONE shared walk (REV-001) for `gogo project add`, `gogo source
// add`, and the config tab's blank-on-add auto-assign, so the gather logic lives in a
// single place. A nil/empty input yields nil.
func TakenColors(projs []Project) []string {
	var out []string
	for _, p := range projs {
		if p.Color != "" {
			out = append(out, p.Color)
		}
		for _, s := range p.Sources {
			if s.Color != "" {
				out = append(out, s.Color)
			}
		}
	}
	return out
}

// LookupSwatch resolves a persisted hex back to its Swatch (matching either the Dark or
// the Light variant, case-insensitively), so the tui can render it adaptively and the
// config tab can name it ("label color · teal"). A hand-typed hex that matches nothing
// returns ok=false (the tui renders it directly).
func LookupSwatch(hex string) (Swatch, bool) {
	h := normalizeHex(hex)
	if h == "" {
		return Swatch{}, false
	}
	for _, sw := range Swatches {
		if normalizeHex(sw.Dark) == h || normalizeHex(sw.Light) == h {
			return sw, true
		}
	}
	return Swatch{}, false
}

// SwatchByName resolves a swatch NAME (e.g. "teal") to its Dark hex — so the config
// label-color field accepts a friendly name as well as a raw hex. ok=false for an
// unknown name (the caller keeps whatever the user typed).
func SwatchByName(name string) (string, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, sw := range Swatches {
		if sw.Name == n {
			return sw.Dark, true
		}
	}
	return "", false
}

// normalizeHex lowercases + trims a hex string for case-insensitive comparison.
func normalizeHex(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
