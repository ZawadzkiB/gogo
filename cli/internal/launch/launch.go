// Package launch delegates every state-changing action to Claude by spawning
// the real slash commands (/gogo:go, /gogo:done). It NEVER mutates pipeline
// state itself — a card moves columns only when the contract files actually
// change. Preferred mode is an attachable tmux session (gates stay
// answerable); with no tmux it falls back to a backgrounded `claude -p` + log.
// The CLI writes only under .gogo/resources/.
package launch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

// Action is the pipeline verb a column move triggers.
type Action string

const (
	ActionGo   Action = "go"   // plan→implement / resume in-progress → /gogo:go
	ActionDone Action = "done" // ready→changelog (single or merged) → /gogo:done
)

// Intent is a fully-resolved, quoted plan for a launch — built purely from a
// move so it can be shown in a huh confirmation before anything runs.
type Intent struct {
	Action  Action
	Slugs   []string // one for go; one-or-more for a merged done
	Release string   // release name for a merged done ("" otherwise)
	Command string   // the claude slash command, e.g. "/gogo:done a+b+c"
	Session string   // sanitized tmux session name, e.g. "gogo-done-my-release"
}

// Result records what was actually launched so the TUI can surface it.
type Result struct {
	Mode    string // "tmux" | "background"
	Session string // tmux session name (tmux mode)
	LogPath string // log file (background mode)
	PID     int    // process id (background mode)
	Command string // the claude command that ran
}

var sessionUnsafe = regexp.MustCompile(`[^a-z0-9-]+`)

// BuildIntent resolves an action + slugs (+ optional release) into the exact
// command and tmux session name. Pure — the unit-tested core of the launcher.
func BuildIntent(action Action, slugs []string, release string) Intent {
	in := Intent{Action: action, Slugs: slugs, Release: release}
	switch action {
	case ActionGo:
		slug := ""
		if len(slugs) > 0 {
			slug = slugs[0]
		}
		in.Command = "/gogo:go " + slug
		in.Session = sessionName("go", slug)
	case ActionDone:
		// Multiple ready picks = ONE merged entry: claude "/gogo:done a+b+c".
		in.Command = "/gogo:done " + strings.Join(slugs, "+")
		label := release
		if label == "" && len(slugs) > 0 {
			label = slugs[0]
		}
		in.Session = sessionName("done", label)
	}
	return in
}

// sessionName builds "gogo-<action>-<sanitized>" (tmux-safe: lowercase,
// [a-z0-9-] only). tmux forbids '.' and ':' in session names.
func sessionName(action, label string) string {
	s := sessionUnsafe.ReplaceAllString(strings.ToLower(label), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "run"
	}
	return "gogo-" + action + "-" + s
}

// TmuxNewSessionArgs are the argv for `tmux <args>` that starts a detached,
// attachable session running the interactive claude command. No shell quoting
// is needed: tmux execs the command + its single argument directly.
// The session is anchored to the repo root (`-c root`): launching claude from
// wherever the board happened to run (e.g. cli/) makes Claude Code treat that
// dir as a NEW project — first-run MCP/trust prompts park the session
// (TEST-013). The repo root carries the user's existing approvals.
func TmuxNewSessionArgs(root string, in Intent) []string {
	return []string{"new-session", "-d", "-s", in.Session, "-c", root, "claude", in.Command}
}

// Detection helpers (soft deps — detected at use, never required).
func HasTmux() bool   { return has("tmux") }
func HasClaude() bool { return has("claude") }
func HasGlow() bool   { return has("glow") }

func has(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// ListSessions returns running tmux session names matching "gogo-*". Empty
// when tmux is absent or none exist.
func ListSessions() []string {
	if !HasTmux() {
		return nil
	}
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		return nil
	}
	var sessions []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "gogo-") {
			sessions = append(sessions, line)
		}
	}
	return sessions
}

// HasSession reports whether a tmux session with this exact name exists.
func HasSession(name string) bool {
	if !HasTmux() {
		return false
	}
	return exec.Command("tmux", "has-session", "-t", name).Run() == nil
}

// uniqueSession appends -2, -3, … until the name is free (best-effort).
func uniqueSession(base string) string {
	if !HasSession(base) {
		return base
	}
	for i := 2; i < 100; i++ {
		cand := fmt.Sprintf("%s-%d", base, i)
		if !HasSession(cand) {
			return cand
		}
	}
	return base
}

// AttachArgs returns argv for attaching to a session, honoring whether we are
// already inside tmux (switch-client) or outside (attach-session).
func AttachArgs(session string) []string {
	if os.Getenv("TMUX") != "" {
		return []string{"switch-client", "-t", session}
	}
	return []string{"attach-session", "-t", session}
}

// Launch spawns the intent. With tmux → a detached, attachable session that
// stays alive after the command exits (remain-on-exit) so the user can attach
// to answer gates. Without tmux → a backgrounded `claude -p` writing to a log
// under .gogo/resources/cli/logs/. NEVER call without a prior confirmation.
func Launch(root string, in Intent) (Result, error) {
	if !HasClaude() {
		return Result{}, fmt.Errorf("claude CLI not found on PATH — cannot launch %q", in.Command)
	}

	if HasTmux() {
		session := uniqueSession(in.Session)
		in.Session = session
		args := TmuxNewSessionArgs(root, in)
		if err := exec.Command("tmux", args...).Run(); err != nil {
			return Result{}, fmt.Errorf("tmux new-session failed: %w", err)
		}
		// Keep the session alive after claude exits, so it stays attachable.
		_ = exec.Command("tmux", "set-option", "-t", session, "remain-on-exit", "on").Run()
		return Result{Mode: "tmux", Session: session, Command: in.Command}, nil
	}

	// No tmux: background claude -p with a log file (gates surfaced as
	// "waiting for user — resume in chat").
	logPath, err := backgroundLogPath(root, in)
	if err != nil {
		return Result{}, err
	}
	logFile, err := os.Create(logPath)
	if err != nil {
		return Result{}, err
	}
	cmd := exec.Command("claude", "-p", in.Command)
	cmd.Dir = root // same anchoring as the tmux path (TEST-013)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // detach from the CLI's process group
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return Result{}, fmt.Errorf("claude -p failed to start: %w", err)
	}
	return Result{Mode: "background", LogPath: logPath, PID: cmd.Process.Pid, Command: in.Command}, nil
}

func backgroundLogPath(root string, in Intent) (string, error) {
	dir := filepath.Join(root, ".gogo", "resources", "cli", "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := strings.TrimPrefix(in.Session, "gogo-")
	return filepath.Join(dir, name+".log"), nil
}
