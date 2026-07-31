package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
)

// TestStatusGolden pins the `gogo status` output on the fixture tree so a
// change to the classifier or the table format is caught (contract stability).
func TestStatusGolden(t *testing.T) {
	repo, err := contract.LoadRepo(filepath.Join("internal", "contract", "testdata", "repo"))
	if err != nil {
		t.Fatalf("LoadRepo: %v", err)
	}
	got := FormatStatus(repo)

	goldenPath := filepath.Join("testdata", "status.golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Errorf("status output drifted from golden.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// statusRow returns the table row whose SLUG column is slug (the last field).
func statusRow(t *testing.T, table, slug string) string {
	t.Helper()
	for _, line := range strings.Split(table, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[len(fields)-1] == slug {
			return line
		}
	}
	t.Fatalf("no row for %q in:\n%s", slug, table)
	return ""
}

// authoringStatusRepo builds a fixture repo with one AUTHORING item (a template state.md at
// the plan gate, no plan.md), one genuine plan gate (a written plan.md), and one item at
// plan-accepted whose build session is live but whose state.md has not caught up.
func authoringStatusRepo(t *testing.T) *contract.Repo {
	t.Helper()
	root := t.TempDir()
	mk := func(slug, status string, planSections int) {
		dir := filepath.Join(root, ".gogo", "work", "feature-"+slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		state := "- **feature:** <one-line title>\n- **phase:** plan\n- **status:** " + status +
			"\n- **created:** 2026-07-30\n"
		if err := os.WriteFile(filepath.Join(dir, "state.md"), []byte(state), 0o644); err != nil {
			t.Fatalf("write state.md: %v", err)
		}
		if planSections > 0 {
			body := "# Plan\n\n"
			for i := 0; i < planSections; i++ {
				body += "## S" + string(rune('a'+i)) + "\n\nx\n\n"
			}
			if err := os.WriteFile(filepath.Join(dir, "plan.md"), []byte(body), 0o644); err != nil {
				t.Fatalf("write plan.md: %v", err)
			}
		}
	}
	mk("authoring", "awaiting-plan-acceptance", 0)
	mk("realgate", "awaiting-plan-acceptance", 8)
	mk("building", "plan-accepted", 8)
	repo, err := contract.LoadRepo(root)
	if err != nil {
		t.Fatalf("LoadRepo: %v", err)
	}
	return repo
}

// TestStatusWaitColumnExcludesAuthoring pins FR2 on the HEADLESS surface: `gogo status`
// marks a genuine user gate WAIT, and an authoring item `-`. The table is the greppable
// signal an unattended operator reads, so it must not report a gate that does not exist.
func TestStatusWaitColumnExcludesAuthoring(t *testing.T) {
	table := FormatStatus(authoringStatusRepo(t))

	if got := statusRow(t, table, "authoring"); !strings.HasPrefix(got, "-  ") {
		t.Errorf("authoring row does not start with a `-` WAIT cell: %q", got)
	}
	if got := statusRow(t, table, "realgate"); !strings.HasPrefix(got, "WAIT") {
		t.Errorf("a WRITTEN plan awaiting acceptance lost its WAIT marker: %q", got)
	}
	// And the header count agrees (the three unfinished items, one of them waiting).
	if !strings.Contains(table, "unfinished 3") {
		t.Errorf("header count drifted:\n%s", table)
	}
}

// TestStatusLiveColumn pins FR14 on the headless surface: with live sessions passed in, the
// table gains a LIVE column that names what each session is DOING - so a mid-build item whose
// state.md still reads plan-accepted cannot silently look idle. It asserts the exact cell,
// not just "the word appears somewhere".
func TestStatusLiveColumn(t *testing.T) {
	repo := authoringStatusRepo(t)
	table := FormatStatusSessions(repo, []string{"gogo-go-building", "gogo-plan-authoring", "gogo-done-realgate"})

	if !strings.Contains(table, "LIVE") {
		t.Fatalf("no LIVE column header:\n%s", table)
	}
	cases := map[string]string{
		"building":  "building",  // a live `go` session = a build in flight
		"authoring": "authoring", // a live `plan` session = the analyst, NOT a build
		"realgate":  "live",      // any other attributed session
	}
	for slug, want := range cases {
		row := statusRow(t, table, slug)
		if fields := strings.Fields(row); len(fields) < 2 || fields[1] != want {
			t.Errorf("%s LIVE cell = %q, want %q (row: %q)", slug, fields[1], want, row)
		}
	}
}

// TestStatusNoSessionsIsByteForByteToday pins the portability fallback: tmux is a SOFT dep,
// so an empty session set must render exactly the pre-0.29.0 table - no LIVE column, same
// widths, same rule. That is also what keeps the golden above meaningful.
func TestStatusNoSessionsIsByteForByteToday(t *testing.T) {
	repo := authoringStatusRepo(t)
	plain := FormatStatus(repo)
	if plain != FormatStatusSessions(repo, nil) {
		t.Error("FormatStatus and FormatStatusSessions(nil) disagree")
	}
	if strings.Contains(plain, "LIVE") {
		t.Errorf("the tmux-absent table grew a LIVE column:\n%s", plain)
	}
	if !strings.Contains(plain, `WAIT  CLASS`) {
		t.Errorf("the tmux-absent header is not the pre-0.29.0 one:\n%s", plain)
	}
}
