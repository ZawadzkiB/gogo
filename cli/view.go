package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/pages"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/ZawadzkiB/gogo/cli/internal/tui"
)

// cmdView renders a plan/report: glamour to stdout by default, or the
// self-contained interactive HTML page with --web (+ --open to launch it).
func cmdView(args []string) int {
	var web, open bool
	var target string
	for _, a := range args {
		switch a {
		case "--web":
			web = true
		case "--open":
			open = true
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintf(os.Stderr, "gogo view: unknown flag %q\n", a)
				return 2
			}
			target = a
		}
	}
	if target == "" {
		fmt.Fprintln(os.Stderr, "gogo view: missing <slug|slug:plan|slug:report|date-name>")
		return 2
	}

	root, ok := findRoot()
	if !ok {
		return 1
	}
	repo, _ := contract.LoadRepo(root)

	bundle, err := resolveBundle(root, repo, target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gogo view:", err)
		return 1
	}

	if !web {
		return printMarkdown(bundle.MarkdownPath)
	}

	page, err := pages.WritePage(root, bundle)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gogo view:", err)
		return 1
	}
	fmt.Printf("built %s\n", page)
	if open {
		openInBrowser(page)
	}
	return 0
}

// resolveBundle maps a view target to a page bundle: a feature (plan/report by
// kind) or a changelog entry.
func resolveBundle(root string, repo *contract.Repo, target string) (pages.Bundle, error) {
	slug, kind := contract.SlugFromArg(target)
	if f := repo.Feature(slug); f != nil {
		return featureBundle(f, kind)
	}
	// Not a feature slug — try a changelog entry name (folder base).
	if ce := findChangelog(repo, target); ce != nil {
		if !ce.HasReport {
			return pages.Bundle{}, fmt.Errorf("changelog entry %q has no report.md", ce.Base)
		}
		return pages.Bundle{
			Name:         ce.Base,
			Title:        "gogo — " + ce.Base,
			MarkdownPath: filepath.Join(ce.Dir, "report.md"),
			DiagramDir:   ce.Dir,
			BeforeDir:    optionalDir(filepath.Join(ce.Dir, "before")),
			ManifestPath: filepath.Join(ce.Dir, "manifest.json"),
		}, nil
	}
	return pages.Bundle{}, fmt.Errorf("no feature or changelog entry matches %q", target)
}

func featureBundle(f *contract.Feature, kind string) (pages.Bundle, error) {
	wantReport := kind == "report" || (kind == "" && f.ReportPath != "")
	if wantReport {
		if f.ReportPath == "" {
			return pages.Bundle{}, fmt.Errorf("%s has no report yet (try :plan)", f.Slug)
		}
		dir := filepath.Dir(f.ReportPath)
		diagDir := dir
		if filepath.Base(dir) != "report" { // legacy root report → charts/
			diagDir = filepath.Join(f.Dir, "charts")
		}
		return pages.Bundle{
			Name:         f.Slug,
			Title:        "gogo — " + f.Slug,
			MarkdownPath: f.ReportPath,
			DiagramDir:   diagDir,
			BeforeDir:    optionalDir(filepath.Join(dir, "before")),
			ManifestPath: filepath.Join(dir, "manifest.json"),
		}, nil
	}
	// plan bundle
	plan := filepath.Join(f.Dir, "plan.md")
	if _, err := os.Stat(plan); err != nil {
		return pages.Bundle{}, fmt.Errorf("%s has no plan.md", f.Slug)
	}
	charts := filepath.Join(f.Dir, "charts")
	return pages.Bundle{
		Name:         f.Slug + "-plan",
		Title:        "gogo — " + f.Slug + " (plan)",
		MarkdownPath: plan,
		DiagramDir:   charts,
		BeforeDir:    optionalDir(filepath.Join(charts, "before")),
		ManifestPath: filepath.Join(charts, "manifest.json"),
	}, nil
}

func findChangelog(repo *contract.Repo, target string) *contract.ChangelogEntry {
	for _, ce := range repo.Changelog {
		if ce.Base == target || ce.Name == target {
			return ce
		}
	}
	return nil
}

func optionalDir(p string) string {
	if info, err := os.Stat(p); err == nil && info.IsDir() {
		return p
	}
	return ""
}

func printMarkdown(path string) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gogo view:", err)
		return 1
	}
	// Same fence handling as the TUI viewers (TEST-005/TEST-008): mermaid
	// flow/sequence fences render as Unicode diagrams, others get the labeled
	// source fallback — never raw DSL on stdout.
	src := tui.PreprocessMermaid(string(raw), 100)
	// TEST-009: render with the exact same custom "article" style as the in-TUI
	// viewer (shared via tui.MarkdownStyle) so stdout and the cockpit match.
	// Detect the background once (no Bubble Tea owns stdin here, so the query is
	// safe) and fall back to dark on a non-tty.
	r, err := glamour.NewTermRenderer(glamour.WithStyles(tui.MarkdownStyle(lipgloss.HasDarkBackground())), glamour.WithWordWrap(100))
	if err != nil {
		fmt.Print(src)
		return 0
	}
	out, err := r.Render(src)
	if err != nil {
		fmt.Print(src)
		return 0
	}
	fmt.Print(out)
	return 0
}

func openInBrowser(path string) {
	if _, err := exec.LookPath("open"); err == nil {
		_ = exec.Command("open", path).Start()
		return
	}
	if _, err := exec.LookPath("xdg-open"); err == nil {
		_ = exec.Command("xdg-open", path).Start()
		return
	}
	fmt.Printf("open it manually: file://%s\n", path)
}
