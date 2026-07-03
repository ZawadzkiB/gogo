package tui

import (
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
)

// TEST-009 — a custom, airier glamour style so plan/report views read like an
// article instead of a dense block. Derived from glamour's Dark/Light configs
// (so code-block chroma etc. stay sensible) and then tuned: generous block
// spacing (a blank line after paragraphs/lists and above every heading), a
// clear heading hierarchy on the board's TONE accents, stronger bold + a
// subtle-background inline code, a document margin, and readable quote/rule
// styling. Both variants are built ONCE at init and shared by the TUI viewer
// and the non-interactive `gogo view` stdout path via MarkdownStyle.

// mdPalette is the small accent set a style variant is built from — the same
// TONE colors the kanban board uses (styles.go), kept tasteful on both
// backgrounds.
type mdPalette struct {
	h1, h2, h3, h4 string // heading hierarchy: primary → secondary → dimmer → faint
	text, strong   string // body text + emphasized (bold) text
	codeFg, codeBg string // inline code: accent on a subtle fill
	quoteFg        string // blockquote text
	rule           string // horizontal rule
}

var (
	darkMDPalette = mdPalette{
		h1: "#7aa8ff", h2: "#5db97a", h3: "#e6a14a", h4: "#9aa0aa",
		text: "#d6dae2", strong: "#f4f6fb",
		codeFg: "#f2b263", codeBg: "#242a36",
		quoteFg: "#c3cbd9",
		rule:    "#3a3f4b",
	}
	lightMDPalette = mdPalette{
		h1: "#2f6fe0", h2: "#2e8b57", h3: "#b9721c", h4: "#6b6b6b",
		text: "#1c1f26", strong: "#0b0d12",
		codeFg: "#a8410f", codeBg: "#eef1f8",
		quoteFg: "#3a4152",
		rule:    "#c9cdd6",
	}
)

// Built once (package init) — cross-package init guarantees styles.* is ready.
var (
	darkMarkdownStyle  = buildMarkdownStyle(true)
	lightMarkdownStyle = buildMarkdownStyle(false)
)

// MarkdownStyle is the shared custom glamour style (TEST-009). Exported so the
// `gogo view` stdout path renders with the exact same article styling as the
// in-TUI viewer.
func MarkdownStyle(dark bool) ansi.StyleConfig { return markdownStyle(dark) }

func markdownStyle(dark bool) ansi.StyleConfig {
	if dark {
		return darkMarkdownStyle
	}
	return lightMarkdownStyle
}

// buildMarkdownStyle copies glamour's base config by value and overrides select
// fields with FRESH pointers only — never mutating through a shared pointer, so
// the upstream package vars are left untouched.
func buildMarkdownStyle(dark bool) ansi.StyleConfig {
	cfg := styles.LightStyleConfig
	p := lightMDPalette
	if dark {
		cfg = styles.DarkStyleConfig
		p = darkMDPalette
	}

	// Document: keep a comfortable ~2 margin and a calm base text color.
	cfg.Document.Margin = uintPtrMD(2)
	cfg.Document.Color = strPtrMD(p.text)

	// Generous block spacing — a trailing blank line after paragraphs and lists
	// so blocks breathe.
	cfg.Paragraph.BlockSuffix = "\n"
	cfg.List.BlockSuffix = "\n"

	// Headings: a blank line above and below every heading (cascades to H1–H6),
	// always bold.
	cfg.Heading.BlockPrefix = "\n"
	cfg.Heading.BlockSuffix = "\n"
	cfg.Heading.Bold = boolPtrMD(true)

	// H1 — the primary (plan) accent as a clean, bold, accent-colored title
	// (drop glamour's inverse-block H1 for a lighter, article feel).
	cfg.H1.Prefix = "# "
	cfg.H1.Suffix = ""
	cfg.H1.Color = strPtrMD(p.h1)
	cfg.H1.BackgroundColor = nil
	cfg.H1.Bold = boolPtrMD(true)

	// H2 — a distinct secondary accent; H3 dimmer; H4+ faint.
	cfg.H2.Prefix = "## "
	cfg.H2.Color = strPtrMD(p.h2)
	cfg.H2.Bold = boolPtrMD(true)

	cfg.H3.Prefix = "### "
	cfg.H3.Color = strPtrMD(p.h3)
	cfg.H3.Bold = boolPtrMD(true)

	cfg.H4.Prefix = "#### "
	cfg.H4.Color = strPtrMD(p.h4)
	cfg.H4.Bold = boolPtrMD(false)

	// Emphasis pops: bold is brighter/darker than the body; inline code sits on a
	// subtle fill in an accent color so it stands out from prose.
	cfg.Strong.Bold = boolPtrMD(true)
	cfg.Strong.Color = strPtrMD(p.strong)
	cfg.Emph.Italic = boolPtrMD(true)

	cfg.Code.Prefix = " "
	cfg.Code.Suffix = " "
	cfg.Code.Color = strPtrMD(p.codeFg)
	cfg.Code.BackgroundColor = strPtrMD(p.codeBg)
	cfg.Code.Bold = boolPtrMD(true)

	// Blockquote → a calm, italic callout with a colored bar.
	cfg.BlockQuote.Color = strPtrMD(p.quoteFg)
	cfg.BlockQuote.Italic = boolPtrMD(true)
	cfg.BlockQuote.Indent = uintPtrMD(1)
	cfg.BlockQuote.IndentToken = strPtrMD("┃ ")

	// A quieter rule and a bold accent table header for readable tables.
	cfg.HorizontalRule.Color = strPtrMD(p.rule)
	cfg.Table.BlockPrefix = "\n"
	cfg.Table.BlockSuffix = "\n"

	return cfg
}

func strPtrMD(s string) *string { return &s }
func boolPtrMD(b bool) *bool    { return &b }
func uintPtrMD(u uint) *uint    { return &u }
