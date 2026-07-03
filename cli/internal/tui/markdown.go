package tui

import (
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/diagram"
)

// preprocessMermaid rewrites ```mermaid fenced code blocks in a markdown source
// BEFORE it reaches glamour — which would otherwise show the raw DSL (TEST-005).
// Flowchart AND sequence diagrams become the mermaid-ascii render inside a plain
// code block (plus a derivable title); every other kind (class/state/…) keeps a
// labeled source block that points at `w` (the browser view). It never panics on
// malformed DSL — the fallback is always the labeled source.
// PreprocessMermaid is the exported entry point — used by the non-interactive
// `gogo view` stdout path too, so every markdown surface renders fences the
// same way (drill-in viewer, glamour view, and plain stdout).
func PreprocessMermaid(md string, width int) string { return preprocessMermaid(md, width) }

func preprocessMermaid(md string, width int) string {
	lines := strings.Split(md, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		marker, ok := mermaidFenceOpen(lines[i])
		if !ok {
			out = append(out, lines[i])
			i++
			continue
		}
		i++ // consume the opening ```mermaid
		var body []string
		for i < len(lines) && !fenceClose(lines[i], marker) {
			body = append(body, lines[i])
			i++
		}
		if i < len(lines) {
			i++ // consume the closing fence
		}
		out = append(out, renderMermaidBlock(strings.Join(body, "\n"), width)...)
	}
	return strings.Join(out, "\n")
}

// mermaidFenceOpen reports whether line opens a ```mermaid / ~~~mermaid fence,
// returning the fence marker (``` or ~~~) so the matching close can be found.
func mermaidFenceOpen(line string) (marker string, ok bool) {
	t := strings.TrimSpace(line)
	for _, ch := range []string{"```", "~~~"} {
		if strings.HasPrefix(t, ch) {
			info := strings.TrimSpace(strings.TrimLeft(t, ch[:1]))
			return ch, strings.EqualFold(info, "mermaid")
		}
	}
	return "", false
}

// fenceClose reports whether line is a bare closing fence for the given marker.
func fenceClose(line, marker string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, marker) && strings.TrimSpace(strings.TrimLeft(t, marker[:1])) == ""
}

// renderMermaidBlock turns one mermaid source into the markdown lines that
// replace its fence: a mermaid-ascii render for flowchart AND sequence kinds,
// else a labeled source block (class/state/… or an unrenderable diagram).
// diagram.Render never panics, so no local recover is needed.
func renderMermaidBlock(src string, width int) []string {
	if rendered, err := diagram.Render(src, width); err == nil {
		out := make([]string, 0, 8)
		if title := mermaidTitle(src); title != "" {
			out = append(out, "**"+title+"**", "")
		}
		out = append(out, "```")
		out = append(out, strings.Split(strings.TrimRight(rendered, "\n"), "\n")...)
		out = append(out, "```")
		return out
	}
	kind := diagram.Kind(src)
	if kind == "" {
		kind = "diagram"
	}
	out := []string{"> [mermaid " + kind + " — press w for the browser view]", "", "```"}
	out = append(out, strings.Split(strings.TrimRight(src, "\n"), "\n")...)
	out = append(out, "```")
	return out
}

// mermaidTitle extracts a title from a `--- title: X ---` frontmatter or a
// `%% title: X` comment, when present.
func mermaidTitle(src string) string {
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "title:"):
			return strings.TrimSpace(strings.TrimPrefix(line, "title:"))
		case strings.HasPrefix(line, "%% title:"):
			return strings.TrimSpace(strings.TrimPrefix(line, "%% title:"))
		}
	}
	return ""
}
