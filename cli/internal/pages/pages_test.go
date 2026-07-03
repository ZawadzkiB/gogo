package pages

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fileExistsTest(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func sampleBundle() Bundle {
	dir := filepath.Join("testdata", "bundle")
	return Bundle{
		Name:         "sample-plan",
		Title:        "gogo — sample (plan)",
		MarkdownPath: filepath.Join(dir, "plan.md"),
		DiagramDir:   filepath.Join(dir, "charts"),
		BeforeDir:    filepath.Join(dir, "charts", "before"),
		ManifestPath: filepath.Join(dir, "charts", "manifest.json"),
	}
}

func TestBuildHTMLStructure(t *testing.T) {
	html, err := BuildHTML(sampleBundle())
	if err != nil {
		t.Fatalf("BuildHTML: %v", err)
	}

	// No unreplaced template placeholders. (window.GOGO_LAYOUT is a legit JS
	// variable in the template, not a placeholder — so we check the exact set.)
	for _, tok := range []string{
		"GOGO_VIEW_TITLE", "GOGO_VIEW_SUMMARY", "GOGO_VIEW_DIAGRAMS", "GOGO_VIEW_LAYOUT",
		"GOGO_MERMAID_SRC", "GOGO_GEOMETRY_SRC", "GOGO_VIEWPORT_SRC",
		"GOGO_MERMAID_PARSE_SRC", "GOGO_RENDER_SRC", "GOGO_VIEWER_SRC", "GOGO_VIEWER_CSS",
	} {
		if strings.Contains(html, tok) {
			t.Errorf("unreplaced placeholder: %q", tok)
		}
	}
	// Fully offline — no http(s) references.
	if strings.Contains(html, "http://") || strings.Contains(html, "https://") {
		t.Errorf("page contains a network reference")
	}
	// Summary rendered: heading + GFM table + list; mermaid fence stripped.
	for _, want := range []string{"<h1>", "<table>", "<ul>", "<code>fast</code>"} {
		if !strings.Contains(html, want) {
			t.Errorf("summary missing %q", want)
		}
	}
	if strings.Contains(html, "class=\"language-mermaid\"") || strings.Contains(html, "X --&gt; Y") {
		t.Errorf("mermaid fence leaked into summary")
	}
	// Status line stripped from summary.
	if strings.Contains(html, "Status:") {
		t.Errorf("Status: line not stripped from summary")
	}
}

func TestBuildHTMLCompareMode(t *testing.T) {
	html, err := BuildHTML(sampleBundle())
	if err != nil {
		t.Fatalf("BuildHTML: %v", err)
	}

	// One compare row exists for flow (before+after both present).
	compareRows := strings.Count(html, `<div class="compare">`)
	if compareRows != 2 { // flow (paired) + sequence (solo/added)
		t.Errorf("compare rows = %d, want 2", compareRows)
	}
	// flow is paired: before + after data-diagram keys must not collide.
	if !strings.Contains(html, `data-diagram="before-flow"`) {
		t.Errorf("missing before-flow figure")
	}
	if !strings.Contains(html, `data-diagram="flow"`) {
		t.Errorf("missing after flow figure")
	}
	if !strings.Contains(html, `class="diagram compare-before"`) || !strings.Contains(html, `class="diagram compare-after"`) {
		t.Errorf("compare-before/after classes missing")
	}
	// sequence exists only on the after side → solo "Added".
	if !strings.Contains(html, `class="diagram compare-solo"`) {
		t.Errorf("solo figure missing for sequence")
	}
	if !strings.Contains(html, "Added — Sample sequence") {
		t.Errorf("solo caption from manifest title missing")
	}
	// Captions from the manifest.
	if !strings.Contains(html, "Before — Sample flow") || !strings.Contains(html, "After — Sample flow") {
		t.Errorf("before/after captions from manifest missing")
	}
	// mermaid source inlined verbatim (escaped &).
	if !strings.Contains(html, "reader &amp; parser") {
		t.Errorf("figure source not inlined/escaped")
	}
}

func TestBuildHTMLNoBefore(t *testing.T) {
	b := sampleBundle()
	b.BeforeDir = ""
	html, err := BuildHTML(b)
	if err != nil {
		t.Fatalf("BuildHTML: %v", err)
	}
	// No compare rows without a before/ set; two plain figures.
	if strings.Contains(html, `<div class="compare">`) {
		t.Errorf("unexpected compare row without before/")
	}
	if n := strings.Count(html, "<figure "); n != 2 {
		t.Errorf("figure count = %d, want 2 (flow + sequence)", n)
	}
	if !strings.Contains(html, `class="diagram"`) {
		t.Errorf("plain diagram class missing")
	}
}

func TestBuildHTMLFigureCount(t *testing.T) {
	html, err := BuildHTML(sampleBundle())
	if err != nil {
		t.Fatalf("BuildHTML: %v", err)
	}
	// compare mode: flow(before+after)=2 + sequence(solo)=1 = 3 figures.
	if n := strings.Count(html, "<figure "); n != 3 {
		t.Errorf("figure count = %d, want 3", n)
	}
}

func TestWritePageAndResources(t *testing.T) {
	root := t.TempDir()
	b := sampleBundle()
	page, err := WritePage(root, b)
	if err != nil {
		t.Fatalf("WritePage: %v", err)
	}
	if !strings.HasSuffix(page, filepath.Join(".gogo", "resources", "view", "sample-plan.html")) {
		t.Errorf("page path = %q", page)
	}
	// Resources materialised for offline file:// use.
	for _, rel := range []string{
		filepath.Join(".gogo", "resources", "mermaid.min.js"),
		filepath.Join(".gogo", "resources", "viewer", "interactive.js"),
		filepath.Join(".gogo", "resources", "viewer", "viewer.css"),
	} {
		if !fileExistsTest(filepath.Join(root, rel)) {
			t.Errorf("resource not written: %s", rel)
		}
	}
}
