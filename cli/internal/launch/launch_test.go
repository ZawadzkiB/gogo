package launch

import (
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

func TestTmuxNewSessionArgs(t *testing.T) {
	in := BuildIntent(ActionGo, []string{"slug-x"}, "")
	got := TmuxNewSessionArgs("/repo/root", in)
	// -c anchors the claude session to the repo root: launching from the
	// board's cwd (e.g. cli/) made Claude Code treat it as a NEW project and
	// park on first-run MCP/trust prompts (TEST-013).
	want := []string{"new-session", "-d", "-s", "gogo-go-slug-x", "-c", "/repo/root", "claude", "/gogo:go slug-x"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("args = %v, want %v", got, want)
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
