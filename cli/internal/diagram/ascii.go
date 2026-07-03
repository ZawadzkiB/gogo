// Package diagram renders mermaid sources as clean terminal text using the
// mermaid-ascii engine (github.com/AlexanderGrooff/mermaid-ascii, MIT):
//
//   - flowchart / graph  -> Unicode box-drawing, via the vendored graph render
//     path in ./mermaidascii (vendored because upstream keeps the graph
//     renderer in a gin-tainted `cmd` package — see mermaidascii/doc.go).
//   - sequenceDiagram     -> a real sequence render, via upstream's clean
//     pkg/sequence (imported directly: it links no gin/sonic/cobra — verified).
//   - every other kind (class/state/er/…) -> ErrUnsupported, so the caller
//     shows the labeled source + "press w for the browser view".
//
// Render never panics and never crashes on malformed input: on any parse/render
// error it returns an error and the caller falls back to the labeled source.
package diagram

import (
	"errors"
	"fmt"
	"strings"

	upstream "github.com/AlexanderGrooff/mermaid-ascii/pkg/diagram"
	"github.com/AlexanderGrooff/mermaid-ascii/pkg/sequence"

	"github.com/ZawadzkiB/gogo/cli/internal/diagram/mermaidascii"
)

// ErrUnsupported is returned for diagram kinds without a terminal renderer
// (class/state/er/gantt/…). The caller degrades to the labeled source.
var ErrUnsupported = errors.New("diagram: no terminal renderer for this kind")

// Render turns a mermaid source into terminal text: Unicode box-drawing for
// flowchart/graph, a sequence render for sequenceDiagram. width is advisory —
// the mermaid-ascii engine lays out to content and does not accept a width
// bound; the caller's viewport scrolls wide diagrams. Unsupported kinds return
// ErrUnsupported. It never panics.
func Render(source string, width int) (out string, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = "", fmt.Errorf("diagram render panic: %v", r)
		}
	}()

	switch Kind(source) {
	case "flowchart":
		out, err = mermaidascii.Render(sanitizeFlowchart(source), false)
	case "sequence":
		out, err = renderSequence(source)
	default:
		return "", ErrUnsupported
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out) == "" {
		return "", ErrUnsupported
	}
	return strings.TrimRight(out, "\n"), nil
}

// renderSequence parses+renders a sequenceDiagram via upstream pkg/sequence,
// after sanitizing directives that pkg/sequence doesn't model (actor, Note,
// control-flow blocks) so real-world sequences still render their participants
// and arrows instead of erroring.
func renderSequence(source string) (string, error) {
	sd, err := sequence.Parse(sanitizeSequence(source))
	if err != nil {
		return "", err
	}
	return sequence.Render(sd, upstream.DefaultConfig())
}

// sanitizeFlowchart strips mermaid styling directives (classDef/class/style/
// linkStyle) that the graph renderer would otherwise draw as stray nodes. The
// engine already handles `:::class` node suffixes, subgraphs, and `%%` comments.
func sanitizeFlowchart(src string) string {
	var b strings.Builder
	for _, raw := range strings.Split(src, "\n") {
		t := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(t, "classDef"),
			strings.HasPrefix(t, "class "),
			strings.HasPrefix(t, "style "),
			strings.HasPrefix(t, "linkStyle"):
			continue
		}
		b.WriteString(raw)
		b.WriteByte('\n')
	}
	return b.String()
}

// sequenceDropPrefixes are line-leading keywords pkg/sequence cannot parse; we
// drop them (rather than error the whole diagram) so the core interaction still
// renders. Notes and grouping blocks are lost — acceptable for a terminal view.
var sequenceDropPrefixes = []string{
	"note ", "activate ", "deactivate ", "loop ", "loop:", "alt ", "alt:",
	"opt ", "opt:", "par ", "par:", "and ", "else", "end", "critical",
	"rect ", "rect:", "break", "box ", "box:", "title ", "title:",
	"link ", "links ", "create ", "destroy ",
}

// sanitizeSequence maps `actor` -> `participant` (pkg/sequence models both the
// same) and drops directives it can't parse.
func sanitizeSequence(src string) string {
	var b strings.Builder
	for _, raw := range strings.Split(src, "\n") {
		t := strings.TrimSpace(raw)
		low := strings.ToLower(t)
		if strings.HasPrefix(low, "actor ") {
			raw = strings.Replace(raw, "actor ", "participant ", 1)
			b.WriteString(raw)
			b.WriteByte('\n')
			continue
		}
		drop := false
		for _, p := range sequenceDropPrefixes {
			if strings.HasPrefix(low, p) {
				drop = true
				break
			}
		}
		if drop {
			continue
		}
		b.WriteString(raw)
		b.WriteByte('\n')
	}
	return b.String()
}

// IsFlowchart reports whether src is a flowchart-family diagram (flowchart/graph).
func IsFlowchart(src string) bool {
	head := firstMeaningfulLine(src)
	return strings.HasPrefix(head, "flowchart") || strings.HasPrefix(head, "graph")
}

// Kind classifies a mermaid source by its first meaningful line — "flowchart",
// "sequence", "class", "state", "er", "gantt", "pie", "journey", "gitGraph",
// "mindmap", "timeline" — used to label the source-fallback block. Empty when
// the source is blank; the raw first word otherwise (never panics).
func Kind(src string) string {
	head := firstMeaningfulLine(src)
	switch {
	case strings.HasPrefix(head, "flowchart"), strings.HasPrefix(head, "graph"):
		return "flowchart"
	case strings.HasPrefix(head, "sequenceDiagram"):
		return "sequence"
	case strings.HasPrefix(head, "classDiagram"):
		return "class"
	case strings.HasPrefix(head, "stateDiagram"):
		return "state"
	case strings.HasPrefix(head, "erDiagram"):
		return "er"
	case strings.HasPrefix(head, "gantt"):
		return "gantt"
	case strings.HasPrefix(head, "pie"):
		return "pie"
	case strings.HasPrefix(head, "journey"):
		return "journey"
	case strings.HasPrefix(head, "gitGraph"):
		return "gitGraph"
	case strings.HasPrefix(head, "mindmap"):
		return "mindmap"
	case strings.HasPrefix(head, "timeline"):
		return "timeline"
	}
	if f := strings.Fields(head); len(f) > 0 {
		return f[0]
	}
	return ""
}

func firstMeaningfulLine(src string) string {
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "%%") {
			continue
		}
		return line
	}
	return ""
}
