package projects

import (
	"strings"
	"testing"
)

// TestSwatchesShape pins the palette contract (D2.1): exactly the 8 curated swatches,
// each with a non-blank name + Dark + Light hex, and no duplicate Dark hexes (so the
// round-robin genuinely fans out).
func TestSwatchesShape(t *testing.T) {
	if len(Swatches) != 8 {
		t.Fatalf("Swatches has %d entries, want 8 (D2.1)", len(Swatches))
	}
	seen := map[string]bool{}
	for _, sw := range Swatches {
		if sw.Name == "" || sw.Dark == "" || sw.Light == "" {
			t.Errorf("swatch %+v has a blank field", sw)
		}
		if !strings.HasPrefix(sw.Dark, "#") || !strings.HasPrefix(sw.Light, "#") {
			t.Errorf("swatch %+v hexes must start with #", sw)
		}
		if seen[sw.Dark] {
			t.Errorf("duplicate Dark hex %q", sw.Dark)
		}
		seen[sw.Dark] = true
	}
	// The design's teal / pink / blue are included verbatim.
	for _, want := range []string{"#35c9b5", "#eb7bb5", "#58a6ff"} {
		if !seen[want] {
			t.Errorf("palette missing the design hue %q", want)
		}
	}
}

// TestAssignColorSkipsTaken: AssignColor round-robins deterministically and skips colors
// already in use, so sibling projects/sources fan out across the palette.
func TestAssignColorSkipsTaken(t *testing.T) {
	// Nothing taken → the first swatch.
	if got := AssignColor(nil); got != Swatches[0].Dark {
		t.Errorf("AssignColor(nil) = %q, want the first swatch %q", got, Swatches[0].Dark)
	}
	// First taken → the second.
	if got := AssignColor([]string{Swatches[0].Dark}); got != Swatches[1].Dark {
		t.Errorf("AssignColor([sw0]) = %q, want the second swatch %q", got, Swatches[1].Dark)
	}
	// First two taken (out of order, mixed case) → the third.
	taken := []string{strings.ToUpper(Swatches[1].Dark), Swatches[0].Dark}
	if got := AssignColor(taken); got != Swatches[2].Dark {
		t.Errorf("AssignColor(skip two) = %q, want the third swatch %q", got, Swatches[2].Dark)
	}
	// Deterministic: same input → same output.
	if AssignColor(taken) != AssignColor(taken) {
		t.Error("AssignColor is not deterministic for a fixed taken set")
	}
	// A non-swatch hand-typed color in `taken` does not consume a swatch.
	if got := AssignColor([]string{"#123456"}); got != Swatches[0].Dark {
		t.Errorf("AssignColor with only a non-palette color taken = %q, want the first swatch", got)
	}
}

// TestAssignColorWrapsWhenAllTaken: more entities than swatches → AssignColor still
// returns a non-blank palette hex (deterministic wrap), never "".
func TestAssignColorWrapsWhenAllTaken(t *testing.T) {
	all := make([]string, 0, len(Swatches))
	for _, sw := range Swatches {
		all = append(all, sw.Dark)
	}
	got := AssignColor(all)
	if got == "" {
		t.Fatal("AssignColor with every swatch taken returned blank")
	}
	if _, ok := LookupSwatch(got); !ok {
		t.Errorf("AssignColor wrap = %q, want a palette swatch hex", got)
	}
}

// TestColorForIndexNeverBlankAndWraps: ColorForIndex is deterministic, wraps (incl.
// negatives), and is never blank — the render-time fallback for a colorless entity.
func TestColorForIndexNeverBlankAndWraps(t *testing.T) {
	n := len(Swatches)
	for i := -2 * n; i < 3*n; i++ {
		got := ColorForIndex(i)
		if got == "" {
			t.Fatalf("ColorForIndex(%d) is blank", i)
		}
		if _, ok := LookupSwatch(got); !ok {
			t.Errorf("ColorForIndex(%d) = %q, not a palette swatch", i, got)
		}
	}
	if ColorForIndex(0) != ColorForIndex(n) || ColorForIndex(1) != ColorForIndex(1+n) {
		t.Error("ColorForIndex does not wrap deterministically")
	}
}

// TestColorForNameStableAndNeverBlank (REV-002): ColorForName is deterministic, never
// blank, always resolves to a palette swatch, and depends ONLY on the name (so a colorless
// entity keeps its hue when the list reorders — unlike the position-keyed ColorForIndex).
func TestColorForNameStableAndNeverBlank(t *testing.T) {
	for _, name := range []string{"web", "api", "gogo", "", "a-very-long-source-name"} {
		got := ColorForName(name)
		if got == "" {
			t.Fatalf("ColorForName(%q) is blank", name)
		}
		if _, ok := LookupSwatch(got); !ok {
			t.Errorf("ColorForName(%q) = %q, not a palette swatch", name, got)
		}
		if got != ColorForName(name) {
			t.Errorf("ColorForName(%q) is not deterministic", name)
		}
	}
	// Distinct, differently-hashing names land on different swatches (spot-check).
	if ColorForName("web") == ColorForName("api") {
		t.Error("ColorForName(web) == ColorForName(api) — expected distinct swatches")
	}
}

// TestTakenColors (REV-001): the one shared walk gathers every non-blank project + source
// color across the store, skips blanks, and yields nil for an empty input.
func TestTakenColors(t *testing.T) {
	if got := TakenColors(nil); got != nil {
		t.Errorf("TakenColors(nil) = %v, want nil", got)
	}
	projs := []Project{
		{Name: "a", Color: "#111111", Sources: []Source{{Color: "#222222"}, {Color: ""}}},
		{Name: "b", Color: "", Sources: []Source{{Color: "#333333"}}},
	}
	got := TakenColors(projs)
	want := []string{"#111111", "#222222", "#333333"}
	if len(got) != len(want) {
		t.Fatalf("TakenColors = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("TakenColors[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestLookupSwatchRoundTrip: every swatch's Dark AND Light hex resolve back to it (case-
// insensitively); a non-palette hex and a blank do not.
func TestLookupSwatchRoundTrip(t *testing.T) {
	for _, sw := range Swatches {
		if got, ok := LookupSwatch(sw.Dark); !ok || got.Name != sw.Name {
			t.Errorf("LookupSwatch(%q) = %+v ok=%v, want %s", sw.Dark, got, ok, sw.Name)
		}
		if got, ok := LookupSwatch(strings.ToUpper(sw.Light)); !ok || got.Name != sw.Name {
			t.Errorf("LookupSwatch(%q light, upper) = %+v ok=%v, want %s", sw.Light, got, ok, sw.Name)
		}
	}
	if _, ok := LookupSwatch("#010203"); ok {
		t.Error("LookupSwatch matched a non-palette hex")
	}
	if _, ok := LookupSwatch(""); ok {
		t.Error("LookupSwatch matched a blank")
	}
}

// TestSwatchByName: a swatch name resolves to its Dark hex (case-insensitive); an
// unknown name does not.
func TestSwatchByName(t *testing.T) {
	if got, ok := SwatchByName("Teal"); !ok || got != "#35c9b5" {
		t.Errorf("SwatchByName(Teal) = %q ok=%v, want #35c9b5", got, ok)
	}
	if _, ok := SwatchByName("chartreuse"); ok {
		t.Error("SwatchByName matched an unknown name")
	}
}
