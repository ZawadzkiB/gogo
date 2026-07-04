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
