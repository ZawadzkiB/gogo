// Package pages is the native `w` page builder: it turns a plan/report bundle
// (markdown + its .mmd set) into the same self-contained, offline interactive
// HTML page the /gogo:view skill produces — markdown→HTML via goldmark, each
// .mmd inlined into a <figure> the vendored viewer makes interactive, with
// before/after compare pairing by filename stem. No LLM, no network.
package pages

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// Bundle describes what to render into one page.
type Bundle struct {
	Name         string // output basename (no extension), e.g. "cli-cockpit-and-events-plan"
	Title        string // plain-text tab title
	MarkdownPath string // the plan.md / report.md source
	DiagramDir   string // dir holding the after/normal .mmd set
	BeforeDir    string // optional before/ dir; "" = no compare mode
	ManifestPath string // optional manifest.json for diagram titles
}

// BuildHTML renders the bundle to a complete HTML document string. Pure: reads
// only the bundle's files, writes nothing — the golden-testable core.
func BuildHTML(b Bundle) (string, error) {
	summary, err := renderSummary(b.MarkdownPath)
	if err != nil {
		return "", err
	}
	manifest, _ := contract.ReadManifest(b.ManifestPath)
	diagrams := buildFigures(b.DiagramDir, b.BeforeDir, manifest)

	tmpl, err := assetsFS.ReadFile("assets/viewer.template.html")
	if err != nil {
		return "", err
	}
	r := strings.NewReplacer(
		"GOGO_VIEW_TITLE", escapeText(b.Title),
		"GOGO_VIEW_SUMMARY", summary,
		"GOGO_VIEW_DIAGRAMS", diagrams,
		"GOGO_VIEW_LAYOUT", "{}",
		"GOGO_MERMAID_SRC", "../mermaid.min.js",
		"GOGO_GEOMETRY_SRC", "../viewer/geometry.js",
		"GOGO_VIEWPORT_SRC", "../viewer/viewport.js",
		"GOGO_MERMAID_PARSE_SRC", "../viewer/mermaid-parse.js",
		"GOGO_RENDER_SRC", "../viewer/render.js",
		"GOGO_VIEWER_SRC", "../viewer/interactive.js",
		"GOGO_VIEWER_CSS", "../viewer/viewer.css",
	)
	return r.Replace(string(tmpl)), nil
}

// WritePage ensures the shared resources exist under <root>/.gogo/resources/
// (idempotent, mirroring the /gogo:view skill), builds the page, and writes it
// to .gogo/resources/view/<name>.html. Returns the absolute page path.
func WritePage(root string, b Bundle) (string, error) {
	if err := ensureResources(root); err != nil {
		return "", err
	}
	html, err := BuildHTML(b)
	if err != nil {
		return "", err
	}
	viewDir := filepath.Join(root, ".gogo", "resources", "view")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		return "", err
	}
	page := filepath.Join(viewDir, b.Name+".html")
	if err := os.WriteFile(page, []byte(html), 0o644); err != nil {
		return "", err
	}
	return filepath.Abs(page)
}

// ensureResources writes the vendored viewer JS/CSS every run (small, so
// updates propagate) and mermaid.min.js only if missing (large). Same policy
// as the skill.
func ensureResources(root string) error {
	res := filepath.Join(root, ".gogo", "resources")
	viewer := filepath.Join(res, "viewer")
	if err := os.MkdirAll(viewer, 0o755); err != nil {
		return err
	}
	entries, err := assetsFS.ReadDir("assets")
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		data, err := assetsFS.ReadFile("assets/" + name)
		if err != nil {
			return err
		}
		switch {
		case name == "mermaid.min.js":
			dst := filepath.Join(res, "mermaid.min.js")
			if _, err := os.Stat(dst); os.IsNotExist(err) {
				if err := os.WriteFile(dst, data, 0o644); err != nil {
					return err
				}
			}
		case name == "viewer.template.html":
			// template is embedded, not needed on disk
		case strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".css"):
			if err := os.WriteFile(filepath.Join(viewer, name), data, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

var (
	htmlComment  = regexp.MustCompile(`(?s)<!--.*?-->`)
	statusPrefix = regexp.MustCompile(`(?m)^\s*Status:\s.*$`)
)

// renderSummary reads the markdown, strips HTML comments and mermaid fences
// (diagrams render separately), then converts to HTML via goldmark (GFM).
func renderSummary(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	src := htmlComment.ReplaceAllString(string(raw), "")
	src = stripMermaidFences(src)
	src = statusPrefix.ReplaceAllString(src, "")

	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// stripMermaidFences removes fenced code blocks whose info string is "mermaid"
// so the intended-design diagram embedded in plan.md is not duplicated in the
// summary (the /gogo:view rule).
func stripMermaidFences(src string) string {
	lines := strings.Split(src, "\n")
	var out []string
	inMermaid := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if inMermaid {
			if strings.HasPrefix(trimmed, "```") {
				inMermaid = false
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") {
			info := strings.ToLower(strings.TrimSpace(strings.TrimLeft(trimmed, "`")))
			if info == "mermaid" {
				inMermaid = true
				continue
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// buildFigures produces the GOGO_VIEW_DIAGRAMS markup. With a before/ set it
// pairs by filename stem into compare rows; otherwise a single column.
func buildFigures(afterDir, beforeDir string, manifest *contract.Manifest) string {
	after := diagramMap(afterDir)
	before := diagramMap(beforeDir)

	if len(before) == 0 {
		// Normal single-column layout.
		var b strings.Builder
		for _, stem := range sortedKeys(after) {
			b.WriteString(figure("diagram", stem, caption("", stem, manifest), after[stem]))
		}
		return b.String()
	}

	// Compare mode: union of stems, sorted.
	stems := map[string]bool{}
	for s := range after {
		stems[s] = true
	}
	for s := range before {
		stems[s] = true
	}
	var keys []string
	for s := range stems {
		keys = append(keys, s)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, stem := range keys {
		aPath, hasA := after[stem]
		bPath, hasB := before[stem]
		b.WriteString("<div class=\"compare\">\n")
		switch {
		case hasA && hasB:
			b.WriteString(figure("diagram compare-before", "before-"+stem, caption("Before", stem, manifest), bPath))
			b.WriteString(figure("diagram compare-after", stem, caption("After", stem, manifest), aPath))
		case hasA:
			b.WriteString(figure("diagram compare-solo", stem, caption("Added", stem, manifest), aPath))
		default:
			b.WriteString(figure("diagram compare-solo", "before-"+stem, caption("Removed", stem, manifest), bPath))
		}
		b.WriteString("</div>\n")
	}
	return b.String()
}

// diagramMap indexes a dir's .mmd files by stem (basename without extension).
func diagramMap(dir string) map[string]string {
	out := map[string]string{}
	if dir == "" {
		return out
	}
	for _, p := range contract.ListDiagrams(dir) {
		stem := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		out[stem] = p
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func caption(prefix, stem string, manifest *contract.Manifest) string {
	title := manifest.TitleFor(stem)
	if title == "" {
		title = stem
	}
	if prefix != "" {
		return prefix + " — " + title
	}
	return title
}

// figure renders one <figure> with the .mmd source inlined into a
// <pre class="mermaid"> (HTML-critical chars escaped; the browser decodes them
// back for mermaid via textContent). data-diagram is the renderer's layout key.
func figure(class, dataDiagram, cap, mmdPath string) string {
	src, _ := os.ReadFile(mmdPath)
	var b strings.Builder
	b.WriteString("<figure class=\"")
	b.WriteString(class)
	b.WriteString("\" data-diagram=\"")
	b.WriteString(escapeText(dataDiagram))
	b.WriteString("\">\n  <figcaption>")
	b.WriteString(escapeText(cap))
	b.WriteString("</figcaption>\n  <pre class=\"mermaid\">\n")
	b.WriteString(escapeText(string(src)))
	b.WriteString("\n  </pre>\n</figure>\n")
	return b.String()
}

// escapeText escapes only the HTML-critical trio so the page stays well-formed
// while keeping the mermaid source otherwise verbatim.
func escapeText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
