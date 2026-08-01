package tui

// TestBoardKeyHelpInSync — the enumeration-drift guard for the board/drill key
// surfaces. It reuses the switchKeys AST parser (plans_view_test.go) over
// updateBoard/updateDrill and asserts every handled key is documented in BOTH
// the in-TUI help (the `?` boardAllKeysLine / the drill footer drillKeysLine)
// and `cli/main.go`'s printed board/drill key blocks — so a new key can never
// ship undocumented in either place. Aliases of a documented key are exempted
// EXPLICITLY (with the key they alias), never silently.

import (
	"os"
	"strings"
	"testing"
)

// keyGlyph maps a handled key name to the glyph the help lines document it as.
var keyGlyph = map[string]string{
	"left": "←", "right": "→", "up": "↑", "down": "↓", " ": "space",
}

// helpKeyTokens reads a `·`-separated help line and returns the set of
// documented key tokens: each segment's FIRST word, with a glyph compound
// ("←→/h") split into runes and an ASCII alias group ("esc/q", "enter/v")
// split on "/". Matching the segment head — not a bare substring — is what
// makes the guard real ("move" contains a "v").
func helpKeyTokens(help string) map[string]bool {
	out := map[string]bool{}
	for _, seg := range strings.Split(help, "·") {
		fields := strings.Fields(seg)
		if len(fields) == 0 {
			continue
		}
		head := fields[0]
		ascii := true
		for _, r := range head {
			if r > 127 {
				ascii = false
				break
			}
		}
		if !ascii {
			for _, r := range head { // glyph compound → one token per rune
				out[string(r)] = true
			}
			continue
		}
		for _, part := range strings.Split(head, "/") { // esc/q → esc, q
			if part != "" {
				out[part] = true
			}
		}
	}
	return out
}

// mainHelpBlock extracts the printed help lines between a section header and
// the next blank line from cli/main.go's printHelp text.
func mainHelpBlock(t *testing.T, src, header string) string {
	t.Helper()
	i := strings.Index(src, header)
	if i < 0 {
		t.Fatalf("cli/main.go help has no %q section", header)
	}
	rest := src[i+len(header):]
	if end := strings.Index(rest, "\n\n"); end >= 0 {
		rest = rest[:end]
	}
	return strings.ReplaceAll(rest, "\n", " · ")
}

func TestBoardKeyHelpInSync(t *testing.T) {
	mainSrc, err := os.ReadFile("../../main.go")
	if err != nil {
		t.Fatalf("read cli/main.go: %v", err)
	}
	boardBlock := mainHelpBlock(t, string(mainSrc), "board keys:")
	drillBlock := mainHelpBlock(t, string(mainSrc), "drill-in keys")

	cases := []struct {
		fn     string
		inTUI  string
		inCLI  string
		exempt map[string]string // handled key → the documented key it aliases
	}{
		{
			fn:    "updateBoard",
			inTUI: boardAllKeysLine,
			inCLI: boardBlock,
		},
		{
			fn:    "updateDrill",
			inTUI: drillKeysLine,
			inCLI: drillBlock,
			exempt: map[string]string{
				"left": "esc", "h": "esc", // back aliases
				"right": "enter", "l": "enter", // open aliases
			},
		},
	}
	for _, c := range cases {
		// Anti-vacuity floors (the sibling plans-tab guard's rule, coding-rules.md:
		// a guard must never pass vacuously). A renamed/split handler, a key switch
		// that stops being `switch msg.String()`, a renamed help header or an
		// emptied const would zero one of these sets and turn every per-key
		// assertion below into a green no-op — fail loudly instead. updateBoard
		// handles 19 cases and updateDrill 9, so 8 is a safe floor for both.
		keys := switchKeys(t, "update.go", c.fn)
		if len(keys) < 8 {
			t.Fatalf("parsed only %d keys from %s (%v) - parser drift?", len(keys), c.fn, keys)
		}
		tuiDoc, cliDoc := helpKeyTokens(c.inTUI), helpKeyTokens(c.inCLI)
		if len(tuiDoc) == 0 || len(cliDoc) == 0 {
			t.Fatalf("%s: empty documented-key set (tui=%d cli=%d) - help line or main.go block drift?", c.fn, len(tuiDoc), len(cliDoc))
		}
		for _, k := range keys {
			if alias, ok := c.exempt[k]; ok {
				if !tuiDoc[alias] || !cliDoc[alias] {
					t.Errorf("%s: key %q is exempted as an alias of %q, but %q itself is undocumented", c.fn, k, alias, alias)
				}
				continue
			}
			tok := k
			if g, ok := keyGlyph[k]; ok {
				tok = g
			}
			if !tuiDoc[tok] {
				t.Errorf("%s key %q missing from the in-TUI help line\n  line: %s", c.fn, tok, c.inTUI)
			}
			if !cliDoc[tok] {
				t.Errorf("%s key %q missing from cli/main.go's key block\n  block: %s", c.fn, tok, c.inCLI)
			}
		}
	}
}
