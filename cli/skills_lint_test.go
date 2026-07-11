package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestSkillsBashNoUnsafeRm is the durable regression guard for Slice A
// (unattended-ops-input-signals, FR-A5): gogo's own mechanical file steps must
// never reintroduce a bash shape the harness's "dangerous rm" permission
// classifier flags, or /gogo:done starts halting on a false prompt again.
//
// It scans every skills/*/SKILL.md for the three unsafe idioms the plan
// enumerated and fails on any reappearance:
//   - glob-rm            e.g.  rm -f "$dst"/*.mmd
//   - rm -rf "$var…"     e.g.  rm -rf "$dst/before"
//   - rm -f "$var"       e.g.  rm -f "$res" "$code"  (bare-variable rm)
//
// The safe replacements are guarded, scoped `find <dir> … -delete` (no glob,
// no bare-variable rm) — see skills/gogo-done/SKILL.md and skills/gogo-build.
//
// The patterns anchor `rm` as an actual COMMAND (start-of-line or whitespace,
// then whitespace) so prose mentions like "dangerous rm", "glob-`rm`", or the
// words confirm / warm / term never match — only real invocations do.
func TestSkillsBashNoUnsafeRm(t *testing.T) {
	// rm as a command: preceded by start/whitespace, followed by whitespace.
	// This excludes word-internal "rm" (confirm, warm, term) and backtick-quoted
	// prose ("`rm`", "dangerous rm\"") which are never followed by a bare space.
	unsafe := []struct {
		name string
		re   *regexp.Regexp
	}{
		{"glob-rm (rm …*)", regexp.MustCompile(`(?:^|\s)rm\s[^\n]*\*`)},
		{`rm on a bare variable (rm … "$var")`, regexp.MustCompile(`(?:^|\s)rm\s+(?:-\S+\s+)*"?\$`)},
	}

	skills, err := filepath.Glob(filepath.Join("..", "skills", "*", "SKILL.md"))
	if err != nil {
		t.Fatalf("glob skills: %v", err)
	}
	if len(skills) == 0 {
		t.Fatal("no skills/*/SKILL.md found — wrong cwd? (expected to run from cli/)")
	}

	for _, path := range skills {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, u := range unsafe {
				if u.re.MatchString(line) {
					t.Errorf("%s:%d — unsafe %s reappeared: %q\n"+
						"  rewrite to guarded scoped `find <dir> … -delete` (no glob, no bare-variable rm)",
						path, i+1, u.name, strings.TrimSpace(line))
				}
			}
		}
	}
}
