package launch

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBuildIntentGo(t *testing.T) {
	in := BuildIntent(ActionGo, []string{"cli-cockpit-and-events"}, "")
	if in.Command != "/gogo:go cli-cockpit-and-events" {
		t.Errorf("command = %q", in.Command)
	}
	if in.Session != "gogo-go-cli-cockpit-and-events" {
		t.Errorf("session = %q", in.Session)
	}
}

func TestBuildIntentDoneSingle(t *testing.T) {
	in := BuildIntent(ActionDone, []string{"my-feature"}, "")
	if in.Command != "/gogo:done my-feature" {
		t.Errorf("command = %q", in.Command)
	}
	if in.Session != "gogo-done-my-feature" {
		t.Errorf("session = %q", in.Session)
	}
}

func TestBuildIntentDoneMerged(t *testing.T) {
	// Multiple ready picks = ONE merged entry: claude "/gogo:done a+b+c".
	in := BuildIntent(ActionDone, []string{"alpha", "beta", "gamma"}, "Summer Release 2026")
	if in.Command != "/gogo:done alpha+beta+gamma" {
		t.Errorf("command = %q", in.Command)
	}
	// Release name drives the session, sanitized to tmux-safe chars.
	if in.Session != "gogo-done-summer-release-2026" {
		t.Errorf("session = %q", in.Session)
	}
}

func TestBuildIntentDoneMergedNoRelease(t *testing.T) {
	in := BuildIntent(ActionDone, []string{"alpha", "beta"}, "")
	if in.Command != "/gogo:done alpha+beta" {
		t.Errorf("command = %q", in.Command)
	}
	if in.Session != "gogo-done-alpha" {
		t.Errorf("session = %q, want first-slug fallback", in.Session)
	}
}

func TestSessionSanitize(t *testing.T) {
	in := BuildIntent(ActionDone, []string{"x"}, "Weird.Name:With/Spaces & dots")
	if strings.ContainsAny(in.Session, ".: /&") {
		t.Errorf("session %q contains tmux-unsafe chars", in.Session)
	}
	if in.Session != "gogo-done-weird-name-with-spaces-dots" {
		t.Errorf("session = %q", in.Session)
	}
}

// TestSkipParams pins the FR4 gate-skip param rendering (REV-001): each flag maps to its
// exact fixed [a-z-] token appended INSIDE the single trailing argv element (exactly like
// --correlation — injection-safe), both flags render both tokens in order, and neither
// flag → "" (today's gated command byte-for-byte).
func TestSkipParams(t *testing.T) {
	cases := []struct {
		planSkip, uatSkip bool
		want              string
	}{
		{false, false, ""},
		{true, false, " --skip-acceptance"},
		{false, true, " --skip-uat"},
		{true, true, " --skip-acceptance --skip-uat"},
	}
	for _, c := range cases {
		if got := SkipParams(c.planSkip, c.uatSkip); got != c.want {
			t.Errorf("SkipParams(plan=%v uat=%v) = %q, want %q", c.planSkip, c.uatSkip, got, c.want)
		}
	}
}

// TestSessionMatchesSlug pins TEST-005: session ↔ slug matching is an exact
// boundary match on the sanitized-slug component, never a substring search.
func TestSessionMatchesSlug(t *testing.T) {
	cases := []struct {
		session, slug string
		want          bool
	}{
		// The live repro: "waiting-card" is a textual substring of
		// "gogo-done-awaiting-card" — the old strings.Contains matched, this must not.
		{"gogo-done-awaiting-card", "waiting-card", false},
		{"gogo-done-awaiting-card", "awaiting-card", true},
		// Substring-collision family (realistic slugs).
		{"gogo-go-oauth", "auth", false},
		{"gogo-go-oauth", "oauth", true},
		{"gogo-done-resync", "sync", false},
		// A slug is not matched by a session whose slug merely starts with it.
		{"gogo-go-a-b", "a", false},
		{"gogo-go-a-b", "a-b", true},
		// Exact match on either action prefix.
		{"gogo-go-my-feature", "my-feature", true},
		{"gogo-done-my-feature", "my-feature", true},
		// uniqueSession collision suffix ("-<n>") still matches its OWN slug…
		{"gogo-go-a-2", "a", true},
		{"gogo-done-my-feature-3", "my-feature", true},
		// …but a numeric suffix is not a wildcard for a different slug.
		{"gogo-go-a-2", "b", false},
		{"gogo-go-a-b", "a-2", false},
		// A non-numeric trailing segment is a different slug, not a suffix.
		{"gogo-go-a-bee", "a", false},
		// The slug is sanitized the same way the session name was.
		{"gogo-done-weird-name", "Weird.Name", true},
	}
	for _, c := range cases {
		if got := SessionMatchesSlug(c.session, c.slug); got != c.want {
			t.Errorf("SessionMatchesSlug(%q, %q) = %v, want %v", c.session, c.slug, got, c.want)
		}
	}
}

func TestTmuxNewSessionArgs(t *testing.T) {
	t.Setenv(PermissionModeEnv, "auto") // deterministic: the default classifier mode
	in := BuildIntent(ActionGo, []string{"slug-x"}, "")
	got := TmuxNewSessionArgs("/repo/root", in)
	// -c anchors the claude session to the repo root: launching from the
	// board's cwd (e.g. cli/) made Claude Code treat it as a NEW project and
	// park on first-run MCP/trust prompts (TEST-013). The permission flag (FR8)
	// sits as its OWN argv elements, and the slug stays a single separate element
	// (injection safety — never a shell string).
	want := []string{"new-session", "-d", "-s", "gogo-go-slug-x", "-c", "/repo/root", "claude", "--permission-mode", "auto", "/gogo:go slug-x"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
}

// TestPlanIntent pins the spawn seam (D3): the plan body is seeded whole as the
// /gogo:plan goal (a single argv element via TmuxNewSessionArgs — injection-safe,
// even with spaces/newlines), and the tmux session name is derived from the label
// (the plan title), sanitized to [a-z0-9-]. An empty correlation degrades to a
// plain body-seeded plan launch (no --correlation param).
func TestPlanIntent(t *testing.T) {
	t.Setenv(PermissionModeEnv, "auto")
	body := "Swap the linear scan for a prefix trie; keep lookups O(k) & safe"
	in := PlanIntent("My Plan Title", body, "")
	if in.Action != ActionPlan {
		t.Errorf("action = %v, want ActionPlan", in.Action)
	}
	if in.Command != "/gogo:plan "+body {
		t.Errorf("command = %q, want the body seeded whole (no --correlation)", in.Command)
	}
	if in.Session != "gogo-plan-my-plan-title" {
		t.Errorf("session = %q, want gogo-plan-my-plan-title", in.Session)
	}
	// The body stays ONE trailing argv element (no shell splitting) — injection-safe.
	got := TmuxNewSessionArgs("/repo/root", in)
	if last := got[len(got)-1]; last != "/gogo:plan "+body {
		t.Errorf("body was split across argv: last element = %q", last)
	}
}

// TestPlanIntentEmptyBodySeedsTitle (REV-002): a title-only plan (empty/blank body)
// spawns with its TITLE as the goal, never an empty `/gogo:plan `.
func TestPlanIntentEmptyBodySeedsTitle(t *testing.T) {
	for _, body := range []string{"", "   \n\t "} {
		in := PlanIntent("Add a retry budget", body, "")
		if in.Command != "/gogo:plan Add a retry budget" {
			t.Errorf("empty body: command = %q, want the title seeded as goal", in.Command)
		}
	}
}

// TestPlanIntentCorrelation pins FR15/D3=A: a non-empty correlation folds
// `--correlation plan-XXXX` onto the goal, staying a SINGLE trailing argv element
// (injection-safe) even when the body carries spaces AND newlines; an empty/blank
// correlation degrades to a plain body-seeded PlanIntent (no dangling flag).
func TestPlanIntentCorrelation(t *testing.T) {
	t.Setenv(PermissionModeEnv, "auto")
	body := "migrate the shared token store\nkeep the two apps in lock-step & safe"
	in := PlanIntent("Token migration", body, "plan-7f3a1b2c")
	if in.Command != "/gogo:plan "+body+" --correlation plan-7f3a1b2c" {
		t.Errorf("command = %q, want the --correlation param appended to the goal", in.Command)
	}
	// The whole command (body + newlines + --correlation) stays ONE trailing argv
	// element — injection-safe (no shell splitting on the spaces/newlines).
	got := TmuxNewSessionArgs("/repo/root", in)
	if last := got[len(got)-1]; last != in.Command {
		t.Errorf("command was split across argv: last element = %q", last)
	}
	// A blank correlation is exactly a plain PlanIntent (no dangling --correlation).
	plain := PlanIntent("Token migration", body, "  ")
	if plain.Command != PlanIntent("Token migration", body, "").Command {
		t.Errorf("blank correlation: command = %q, want a plain PlanIntent", plain.Command)
	}
}

// TestAuthorPlanIntentCarriesSkillAndSourcePaths pins the 0.25.0 FR1 analyst seed: the
// `A` authoring intent is a PLAIN prompt (not a slash command, no --correlation flag)
// that directs the session to LOAD the gogo-project-plan skill and carries the plan-file
// path, the .knowledge/ dir, and each source's LABEL + absolute PATH — all in ONE
// trailing argv element (injection-safe even with a space in a path).
func TestAuthorPlanIntentCarriesSkillAndSourcePaths(t *testing.T) {
	t.Setenv(PermissionModeEnv, "auto")
	sources := []SourceRef{
		{Label: "web", Path: "/repos/web app"}, // a space in the path exercises injection-safety
		{Label: "api", Path: "/repos/api"},
	}
	in := AuthorPlanIntent("Cross-repo migration", "Migrate the shared token store to the new service",
		"/home/.gogo/projects/app/.gogo/plans/plan-7f3a.md",
		"plan-7f3a", "/home/.gogo/projects/app/.knowledge", sources)

	if in.Action != ActionAuthor {
		t.Errorf("action = %v, want ActionAuthor", in.Action)
	}
	// A PLAIN prompt — never a slash-command launch, never a --correlation flag.
	if strings.HasPrefix(in.Command, "/") {
		t.Errorf("command = %q, must be a plain prompt (not a slash command)", in.Command)
	}
	if strings.Contains(in.Command, "--correlation") {
		t.Errorf("command = %q, must NOT carry a --correlation flag (a plain session)", in.Command)
	}
	// It directs the session to the analyst skill + carries every seeded input.
	for _, want := range []string{
		"gogo-project-plan", // the skill directive
		"Migrate the shared token store to the new service", // the user's GOAL, named explicitly (0.25.1)
		"/home/.gogo/projects/app/.gogo/plans/plan-7f3a.md", // the plan-file path
		"/home/.gogo/projects/app/.knowledge",               // the project .knowledge/ dir
		"/repos/web app", "/repos/api",                      // each source PATH (not just label)
		"web", "api", // each source label
		"targets:", "Source briefs", // the strict output contract
		"plan-7f3a", // the correlation id (in prose)
	} {
		if !strings.Contains(in.Command, want) {
			t.Errorf("command missing %q:\n%s", want, in.Command)
		}
	}
	// The whole prompt stays ONE trailing argv element — no shell splitting on the
	// space inside a source path (injection-safe).
	got := TmuxNewSessionArgs("/repos/web app", in)
	if last := got[len(got)-1]; last != in.Command {
		t.Errorf("author prompt was split across argv: last element = %q", last)
	}
}

// setEnv sets or unsets an env var for a test and restores it after. t.Setenv
// cannot represent "unset", which the default-mode case needs.
func setEnv(t *testing.T, key string, val *string) {
	t.Helper()
	orig, had := os.LookupEnv(key)
	if val == nil {
		os.Unsetenv(key)
	} else {
		os.Setenv(key, *val)
	}
	t.Cleanup(func() {
		if had {
			os.Setenv(key, orig)
		} else {
			os.Unsetenv(key)
		}
	})
}

// TestPermissionArgsMatrix pins the three permission-flag cases (FR8): env unset
// → the default auto mode; env set to a value → that value verbatim; env set to
// the empty string → the flag is omitted entirely.
func TestPermissionArgsMatrix(t *testing.T) {
	empty := ""
	accept := "acceptEdits"
	cases := []struct {
		name string
		env  *string
		want []string
	}{
		{"default auto (env unset)", nil, []string{"--permission-mode", "auto"}},
		{"override verbatim", &accept, []string{"--permission-mode", "acceptEdits"}},
		{"empty omits the flag", &empty, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setEnv(t, PermissionModeEnv, tc.env)
			if got := PermissionArgs(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("PermissionArgs() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestClaudePrintArgs verifies the no-tmux `claude -p` fallback carries the same
// permission flag as separate argv elements, ahead of -p.
func TestClaudePrintArgs(t *testing.T) {
	dflt := "auto"
	setEnv(t, PermissionModeEnv, &dflt)
	got := ClaudePrintArgs("/gogo:done a+b")
	want := []string{"--permission-mode", "auto", "-p", "/gogo:done a+b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ClaudePrintArgs = %v, want %v", got, want)
	}
	// Empty env → the flag is omitted, so -p leads.
	empty := ""
	setEnv(t, PermissionModeEnv, &empty)
	if got := ClaudePrintArgs("/gogo:go x"); !reflect.DeepEqual(got, []string{"-p", "/gogo:go x"}) {
		t.Errorf("omit-mode ClaudePrintArgs = %v", got)
	}
}

// TestCapturePaneArgs pins the read-only peek snapshot argv (FR7).
func TestCapturePaneArgs(t *testing.T) {
	got := CapturePaneArgs("gogo-go-x", 300)
	want := []string{"capture-pane", "-t", "gogo-go-x", "-p", "-S", "-300"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CapturePaneArgs = %v, want %v", got, want)
	}
}

// TestBackgroundLogFor finds the newest matching background log for a slug.
func TestBackgroundLogFor(t *testing.T) {
	root := t.TempDir()
	logs := filepath.Join(root, ".gogo", "resources", "cli", "logs")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	// A matching and a non-matching log.
	if err := os.WriteFile(filepath.Join(logs, "go-my-slug.log"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logs, "done-other.log"), []byte("no"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := BackgroundLogFor(root, "my-slug"); filepath.Base(got) != "go-my-slug.log" {
		t.Errorf("BackgroundLogFor = %q, want go-my-slug.log", got)
	}
	if got := BackgroundLogFor(root, "absent"); got != "" {
		t.Errorf("BackgroundLogFor(absent) = %q, want empty", got)
	}
}

func TestAttachArgs(t *testing.T) {
	t.Setenv("TMUX", "")
	if got := AttachArgs("gogo-go-x"); !reflect.DeepEqual(got, []string{"attach-session", "-t", "gogo-go-x"}) {
		t.Errorf("outside tmux: %v", got)
	}
	t.Setenv("TMUX", "/tmp/tmux-501/default,1234,0")
	if got := AttachArgs("gogo-go-x"); !reflect.DeepEqual(got, []string{"switch-client", "-t", "gogo-go-x"}) {
		t.Errorf("inside tmux: %v", got)
	}
}
