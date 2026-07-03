package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/glamour"
)

var ansiSeq = regexp.MustCompile("\x1b\\[[0-9;]*m")

const styleSample = "# Title\n\n" +
	"A **bold** word and some `code` in a paragraph.\n\n" +
	"## Section\n\n" +
	"- one\n- two\n\n" +
	"> a quote\n"

// TEST-009: the custom style config carries the article tuning — a document
// margin, blank-line block spacing, a distinct heading hierarchy, a subtle
// inline-code background, and bold emphasis — in BOTH variants, and the dark and
// light palettes differ.
func TestMarkdownStyleConfig(t *testing.T) {
	for _, dark := range []bool{true, false} {
		cfg := markdownStyle(dark)
		if cfg.Document.Margin == nil || *cfg.Document.Margin != 2 {
			t.Errorf("dark=%v: document margin is not ~2", dark)
		}
		if cfg.Paragraph.BlockSuffix != "\n" {
			t.Errorf("dark=%v: no blank line after paragraphs", dark)
		}
		if cfg.List.BlockSuffix != "\n" {
			t.Errorf("dark=%v: no blank line after lists", dark)
		}
		if cfg.Heading.BlockPrefix != "\n" {
			t.Errorf("dark=%v: no blank line above headings", dark)
		}
		if cfg.H1.Color == nil || cfg.H2.Color == nil || cfg.H3.Color == nil {
			t.Errorf("dark=%v: heading hierarchy missing accent colors", dark)
		}
		if cfg.H1.Color != nil && cfg.H2.Color != nil && *cfg.H1.Color == *cfg.H2.Color {
			t.Errorf("dark=%v: H1 and H2 share a color — no hierarchy", dark)
		}
		if cfg.Code.BackgroundColor == nil {
			t.Errorf("dark=%v: inline code has no subtle background", dark)
		}
		if cfg.Strong.Bold == nil || !*cfg.Strong.Bold {
			t.Errorf("dark=%v: strong is not bold", dark)
		}
	}
	if *markdownStyle(true).H1.Color == *markdownStyle(false).H1.Color {
		t.Errorf("dark and light H1 accents are identical — variants not distinct")
	}
	// Building the styles must not have mutated glamour's shared package configs
	// (fresh-pointer discipline): the stock dark H1 keeps its inverse background.
	if darkMarkdownStyle.H1.BackgroundColor != nil {
		t.Errorf("custom dark H1 kept a background — expected a clean accent title")
	}
}

// TEST-009: rendering with the custom style emits ANSI styling and airier
// blank-line spacing, and preserves the prose.
func TestMarkdownStyleRendersAnsiAndSpacing(t *testing.T) {
	r, err := glamour.NewTermRenderer(glamour.WithStyles(markdownStyle(true)), glamour.WithWordWrap(80))
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Render(styleSample)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("no ANSI styling emitted:\n%q", out)
	}
	// glamour pads every line (incl. blank ones) with the document margin AND
	// wraps each in color/reset ANSI, so a blank spacer line is not a bare "\n".
	// Strip ANSI + trailing whitespace before asserting the airier block spacing
	// actually produced empty lines.
	if !strings.Contains(stripTrailingWS(ansiSeq.ReplaceAllString(out, "")), "\n\n") {
		t.Errorf("no blank-line block spacing in the rendered output:\n%q", out)
	}
	for _, w := range []string{"Title", "Section", "bold", "code", "quote"} {
		if !strings.Contains(out, w) {
			t.Errorf("rendered output missing %q:\n%s", w, out)
		}
	}
}

// stripTrailingWS trims trailing spaces from every line so margin padding on
// otherwise-blank spacer lines doesn't hide the block spacing.
func stripTrailingWS(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimRight(ln, " \t")
	}
	return strings.Join(lines, "\n")
}
