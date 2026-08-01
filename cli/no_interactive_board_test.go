package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoInteractiveBoardInSkills is the durable guard behind 0.33.0's standing
// rule: **no gogo command opens an interactive terminal UI as a side effect of
// another action.** The /gogo:done board (the vendored python curses kanban +
// its intent protocol) was retired for exactly that reason — bare /gogo:done
// now ships via the in-chat table — and this scan is what keeps the rule a TEST
// rather than a sentence (the shape of TestSkillsBashNoUnsafeRm): it fails if
// any skill or command references the retired machinery again.
//
// Scanned tokens: `board.py` (the curses TUI), `board-intent` (the intent
// protocol), and `resources/kanban` (its runtime scratch). The `gogo` binary's
// own Bubble Tea board is NOT covered — that is the deliberately separate,
// user-launched cockpit, never a side effect.
func TestNoInteractiveBoardInSkills(t *testing.T) {
	banned := []string{"board.py", "board-intent", "resources/kanban"}

	var files []string
	for _, pattern := range []string{
		filepath.Join("..", "skills", "*", "SKILL.md"),
		filepath.Join("..", "commands", "*.md"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		files = append(files, matches...)
	}
	// Anti-vacuity floor: the guard must never pass because it scanned nothing
	// (a moved tree or wrong cwd would otherwise silence it). The repo ships
	// well over 20 skills + commands.
	if len(files) < 20 {
		t.Fatalf("scanned only %d skill/command files (%v…) — wrong cwd or moved tree?", len(files), files)
	}

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, token := range banned {
				if strings.Contains(line, token) {
					t.Errorf("%s:%d — retired done-board machinery referenced again (%q): %q\n"+
						"  no gogo command may open an interactive board as a side effect of another action (0.33.0);"+
						" the interactive cockpit is the separate `gogo` binary",
						path, i+1, token, strings.TrimSpace(line))
				}
			}
		}
	}
}
