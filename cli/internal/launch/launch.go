// Package launch delegates every state-changing action to Claude by spawning
// the real slash commands (/gogo:go, /gogo:done). It NEVER mutates pipeline
// state itself — a card moves columns only when the contract files actually
// change. Preferred mode is an attachable tmux session (gates stay
// answerable); with no tmux it falls back to a backgrounded `claude -p` + log.
// The CLI writes only under .gogo/resources/.
package launch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Action is the pipeline verb a column move triggers.
type Action string

const (
	ActionGo     Action = "go"     // plan→implement / resume in-progress → /gogo:go
	ActionPlan   Action = "plan"   // plan a feature → /gogo:plan (persistent-session gogo plan leg)
	ActionDone   Action = "done"   // ready→changelog (single or merged) → /gogo:done
	ActionResume Action = "resume" // answer a paused decision gate → /gogo:resume (orchestrator --attach)
	ActionAccept Action = "accept" // clear the plan-acceptance gate → /gogo:accept (board `m` on a plan-pending card)
	ActionAuthor Action = "author" // author a PROJECT plan brief in place → a plain `claude` session (plans-tab `A`)
)

// Intent is a fully-resolved, quoted plan for a launch — built purely from a
// move so it can be shown in a huh confirmation before anything runs.
type Intent struct {
	Action  Action
	Slugs   []string // one for go; one-or-more for a merged done
	Release string   // release name for a merged done ("" otherwise)
	Command string   // the claude slash command, e.g. "/gogo:done a+b+c"
	Session string   // sanitized tmux session name, e.g. "gogo-done-my-release"
	// Root is the repo root the launch must anchor at — the target card's OWN root,
	// captured when the intent is built. The board threads it through so a launch never
	// re-resolves the root by a (possibly colliding) slug on the unified board (REV-001);
	// "" when the caller roots the launch itself (e.g. the CLI inside a repo).
	Root string
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

// --- launched-session permission mode (FR8) ---------------------------------
//
// Board-launched claude sessions run in Claude's AUTO (classifier-based)
// permission mode so the shipped skills' safe file/read/write/copy steps stop
// nagging for approval inside a session nobody is watching (decision D2 —
// classifier auto, NOT a full bypass). The exact flag + value were verified
// against `claude --help` at dev time:
//
//	--permission-mode <mode>   choices: acceptEdits, auto, bypassPermissions,
//	                           manual, dontAsk, plan
//
// "auto" is the classifier mode that auto-approves classified-safe actions
// (claude also ships a `claude auto-mode` inspector for it) — the closest match
// to "auto-approve classified-safe actions" without the danger of
// bypassPermissions, so that is the pick. The value is overridable per the env
// knob below; the flag + value are always SEPARATE argv elements (never a shell
// string), so a value can never reach a shell (injection safety).

// PermissionModeEnv overrides the launched-session permission mode. Set it to
// any claude --permission-mode value to change it, or to the empty string to
// omit the flag entirely (claude then uses its own default — interactive
// prompting). Unset → DefaultPermissionMode.
const PermissionModeEnv = "GOGO_CLAUDE_PERMISSION_MODE"

// DefaultPermissionMode is the classifier auto mode (verified via claude --help).
const DefaultPermissionMode = "auto"

// PermissionMode resolves the effective mode. Unset env → DefaultPermissionMode.
// Present-but-empty → omit=true (the caller drops the flag). Present-nonempty →
// that value verbatim.
func PermissionMode() (mode string, omit bool) {
	v, ok := os.LookupEnv(PermissionModeEnv)
	if !ok {
		return DefaultPermissionMode, false
	}
	if v == "" {
		return "", true
	}
	return v, false
}

// PermissionArgs returns the permission flag as separate argv elements
// (["--permission-mode", "<mode>"]), or nil when the flag must be omitted. Never
// a shell string — safe to splice directly into an exec/tmux argv.
func PermissionArgs() []string {
	mode, omit := PermissionMode()
	if omit {
		return nil
	}
	return []string{"--permission-mode", mode}
}

// PermissionSummary is a one-line description of the effective mode for the huh
// confirmation (so the user sees what a launch will run under).
func PermissionSummary() string {
	mode, omit := PermissionMode()
	if omit {
		return "permission: claude default (prompts — flag omitted via " + PermissionModeEnv + ")"
	}
	if mode == DefaultPermissionMode {
		return "permission: auto (classifier)"
	}
	return "permission: " + mode + " (via " + PermissionModeEnv + ")"
}

// SkipParams returns the per-source gate-skip params to append to a `/gogo:go`
// command for a source that opted out of the plan-acceptance / UAT gate (FR4):
// ` --skip-acceptance` and/or ` --skip-uat`. Each is a single fixed [a-z-] token
// appended INSIDE the one trailing argv element (exactly like the --correlation
// param), so it never reaches a shell — injection-safe. The gogo skills honor the
// params (auto-record the acceptance / auto-pass UAT); absent flags → "" (today's
// gated command byte-for-byte).
func SkipParams(planSkip, uatSkip bool) string {
	s := ""
	if planSkip {
		s += " --skip-acceptance"
	}
	if uatSkip {
		s += " --skip-uat"
	}
	return s
}

// ClaudePrintArgs builds the argv for a backgrounded `claude -p <command>` run
// (the no-tmux fallback), with the permission flag spliced in as separate argv
// elements ahead of -p.
func ClaudePrintArgs(command string) []string {
	args := append([]string(nil), PermissionArgs()...)
	return append(args, "-p", command)
}

// --- persistent-session runner: the one `claude -p` run of the whole skill -----
//
// The orchestrator (cli/internal/orchestrator) launches ONE persistent session per
// feature leg — `claude -p "/gogo:go <slug>"` (or `/gogo:plan`) running the entire
// skill (implement in-context + Task review/test + report) — and WAITS for it to
// exit (the race-free leg-done signal). A first leg starts a NEW session
// (--session-id <uuid>); a later leg RESUMES the same warm session (--resume
// <uuid>). These are the session-aware argv builder + the foreground
// wait-for-exit runner it uses; they are separate from Launch (which backgrounds
// an attachable session for the board) and from LaunchPersistent (the --attach
// path).

// PhaseOpts configures one persistent-session invocation over `claude -p`. At most
// one of SessionID (start a NEW session with a pre-assigned uuid — the first leg)
// or Resume (continue the WARM session — a later leg / gate resume) is set; both
// empty is a plain one-shot.
type PhaseOpts struct {
	SessionID string // --session-id <uuid>
	Resume    string // --resume <uuid>
	JSON      bool   // --output-format json (capture session_id + total_cost_usd + …)
}

// PhaseArgs builds the argv for `claude <flags> -p "<command>"`. Flag+value pairs
// are always SEPARATE argv elements (injection-safe, matching PermissionArgs); the
// command is the single final element after -p, never a shell string. Resume wins
// over SessionID if both are set (you can't pre-assign an id to an existing session).
// Pure — the unit-tested core of the session runner.
func PhaseArgs(command string, opts PhaseOpts) []string {
	args := append([]string(nil), PermissionArgs()...)
	switch {
	case opts.Resume != "":
		args = append(args, "--resume", opts.Resume)
	case opts.SessionID != "":
		args = append(args, "--session-id", opts.SessionID)
	}
	if opts.JSON {
		args = append(args, "--output-format", "json")
	}
	return append(args, "-p", command)
}

// RunResult is the parsed `--output-format json` envelope of a finished `claude -p`
// persistent-session run (only the fields the orchestrator uses; unknown fields are
// ignored, forward-compatible). Verified against claude 2.1.206 (spike).
type RunResult struct {
	SessionID  string  `json:"session_id"`
	CostUSD    float64 `json:"total_cost_usd"`
	NumTurns   int     `json:"num_turns"`
	DurationMS int     `json:"duration_ms"`
	IsError    bool    `json:"is_error"`
}

// RunPhase spawns `claude -p "<command>"` with opts, BLOCKS until it exits, and
// parses the `--output-format json` envelope. This is the foreground, wait-for-exit
// primitive the orchestrator uses to detect the persistent session's leg completion
// (D2: `-p` exit is the race-free leg-done signal); warm continuity survives the
// exit because claude persists the session by uuid (spike-proven). Anchored to root
// (TEST-013). It always requests JSON so the envelope (session_id, cost) is capturable.
func RunPhase(root, command string, opts PhaseOpts) (RunResult, error) {
	if !HasClaude() {
		return RunResult{}, fmt.Errorf("claude CLI not found on PATH — cannot run %q", command)
	}
	opts.JSON = true
	cmd := exec.Command("claude", PhaseArgs(command, opts)...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return RunResult{}, fmt.Errorf("claude -p %q failed: %w", command, err)
	}
	var r RunResult
	if jerr := json.Unmarshal(out, &r); jerr != nil {
		return RunResult{}, fmt.Errorf("parse claude json output for %q: %w", command, jerr)
	}
	return r, nil
}

// --- session log peek (FR7) --------------------------------------------------

// PeekLines is how many lines of a session's pane (scrollback + screen) a peek
// captures — read-only, never an attach.
const PeekLines = 300

// CapturePaneArgs is the argv for `tmux capture-pane`: a read-only snapshot of a
// session's active pane — the last `lines` lines (`-S -<lines>`), printed (`-p`).
func CapturePaneArgs(session string, lines int) []string {
	return []string{"capture-pane", "-t", session, "-p", "-S", "-" + strconv.Itoa(lines)}
}

// CapturePane returns a read-only snapshot of a live session's pane (best-effort;
// no tmux → an error, never a panic).
func CapturePane(session string, lines int) (string, error) {
	if !HasTmux() {
		return "", fmt.Errorf("tmux not installed")
	}
	out, err := exec.Command("tmux", CapturePaneArgs(session, lines)...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// BackgroundLogFor returns the newest background log under
// .gogo/resources/cli/logs whose file name contains slug, or "" if none — the
// no-tmux `claude -p` fallback writes <action>-<label>.log there (see
// backgroundLogPath). Used by the log peek when there is no live tmux session.
func BackgroundLogFor(root, slug string) string {
	dir := filepath.Join(root, ".gogo", "resources", "cli", "logs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var best string
	var bestMod time.Time
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") || !strings.Contains(e.Name(), slug) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if best == "" || info.ModTime().After(bestMod) {
			best = filepath.Join(dir, e.Name())
			bestMod = info.ModTime()
		}
	}
	return best
}

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
	case ActionPlan:
		// The persistent-session `gogo plan` leg: launch-or-resume ONE session
		// running the /gogo:plan skill. Its own tracked session, distinct from the
		// feature's `go` leg (they are separate legs of the same feature's work).
		slug := ""
		if len(slugs) > 0 {
			slug = slugs[0]
		}
		in.Command = "/gogo:plan " + slug
		in.Session = sessionName("plan", slug)
	case ActionAccept:
		// Clear the plan-acceptance gate from the board: a thin launched
		// /gogo:accept presents the plan and records acceptance through gogo-plan's
		// existing recording (the CLI never mutates state itself). Same shape as go.
		slug := ""
		if len(slugs) > 0 {
			slug = slugs[0]
		}
		in.Command = "/gogo:accept " + slug
		in.Session = sessionName("accept", slug)
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

// ResumeIntent builds the Intent for an interactive `/gogo:resume <slug>` session
// — the opt-in `--attach` path the orchestrator uses to let a human answer a paused
// decision gate live, reusing the same attachable-tmux machinery as a board launch.
func ResumeIntent(slug string) Intent {
	return Intent{
		Action:  ActionResume,
		Slugs:   []string{slug},
		Command: "/gogo:resume " + slug,
		Session: sessionName("resume", slug),
	}
}

// PlanIntent builds the Intent for a fire-once, body-seeded `/gogo:plan <body>`
// launch — the plan-detail SPAWN path (FR11/FR15/D3). Unlike BuildIntent(ActionPlan,
// …), which passes a kebab SLUG as the goal (the persistent `gogo plan` leg), this
// seeds the plan's full free-text BODY as the goal so the analyst plans from the
// real idea and DERIVES the final feature slug itself (the CLI cannot pin it). When
// correlation is non-empty it is folded onto the command as a trailing
// `--correlation plan-XXXX` param the `gogo:plan` skill parses + stamps into the new
// work item's state.md (FR15) — the deterministic, injection-safe alternative to an
// advisory prose hint. The whole command reaches claude as ONE trailing argv element
// (no shell — injection-safe, exactly like TmuxNewSessionArgs, even with
// newlines/spaces), and the tmux session name is derived from label, sanitized to
// tmux-safe [a-z0-9-]. Spawn is fire-once, so this is the non-persistent
// Launch(targetRoot, intent) path — no lock/registry, no resume key. An empty
// correlation degrades to a plain body-seeded plan launch (byte-for-byte).
func PlanIntent(label, body, correlation string) Intent {
	// A title-only plan (empty body) seeds its LABEL as the goal, so a spawn never
	// launches an empty `/gogo:plan ` — the title is the one thing such a plan carries.
	goal := body
	if strings.TrimSpace(goal) == "" {
		goal = label
	}
	cmd := "/gogo:plan " + goal
	if c := strings.TrimSpace(correlation); c != "" {
		// The correlation id is [a-z0-9-] (plan-<hex8>), appended AFTER the goal as
		// the explicit `--correlation` param. It stays inside the single trailing argv
		// element (never a shell string), so a body with spaces/newlines is safe.
		cmd += " --correlation " + c
	}
	return Intent{
		Action:  ActionPlan,
		Command: cmd,
		Session: sessionName("plan", label),
	}
}

// SourceRef pairs a source's display label with its absolute repo PATH — the pairs
// the AuthorPlanIntent analyst seed lists so the loaded gogo-project-plan skill can
// READ each source repo (by absolute path, read-only) and key its per-source brief
// (by label). Carrying the path (not just the label) is what lets the session ANALYZE
// the real repos and auto-select targets (0.25.0 FR1).
type SourceRef struct {
	Label string
	Path  string
}

// AuthorPlanIntent builds the Intent for the plans-tab `A` "plan-with-claude"
// authoring trigger — now an analyst-grade session (0.25.0 FR1). Unlike PlanIntent (a
// /gogo:plan SPAWN that has the skill scaffold a SOURCE work item), this AUTHORS a
// PROJECT-LEVEL plan, so it launches a PLAIN interactive `claude` session — NOT a slash
// command — and directs it to LOAD + FOLLOW the `gogo-project-plan` skill (via the
// Skill tool). /gogo:plan Step 1 would unconditionally scaffold a source
// `.gogo/work/feature-<slug>/` (the wrong thing for a project-plan file, and the
// advisory-prose-ignored failure D3 rejected), so the session is seeded to READ +
// ANALYZE the project's SOURCE repos (by absolute path, read-only), decide WHICH the
// plan needs, and write the CLI-owned project-plan markdown at planPath IN PLACE with
// the strict output contract the FR2 auto-spawn parses: a front-matter `targets:` line
// (only the chosen sources) plus a `## Source briefs` body section keyed `### <name>`.
// It keeps the front-matter correlation id and creates NO `.gogo/work/` scaffolding.
//
// The whole prompt reaches claude as ONE trailing argv element (no shell —
// injection-safe, exactly like PlanIntent, even with spaces/newlines in planPath or a
// source path). correlation rides in the PROSE only (the plan file already carries it in
// front-matter); there is NO `--correlation` flag — that param is a /gogo:plan spawn
// contract, meaningless to a plain session. Session name is derived from label
// (sessionName("author", label)); like PlanIntent this is the fire-once, non-persistent
// Launch path (no lock/registry, no resume key).
//
// sources carries each source's LABEL + absolute PATH (SourceRef) so the analyst can
// read the actual repos; knowledgePath (when non-empty) points the session at the
// project's cross-repo .knowledge/ FIRST so the whole-domain context grounds the
// analysis. Both are spliced into the same single trailing argv element.
//
// goal is the user's plan goal (what to build/change across the sources), captured by the
// plans-tab `A` form before minting. It is NAMED explicitly in the prompt (and also lives
// in the plan file's body) so the analyst plans FOR THAT GOAL instead of guessing from the
// repos alone; an empty goal degrades to the pre-0.25.1 prose (the plan file still carries
// the goal). It rides in the same single trailing argv element (injection-safe).
func AuthorPlanIntent(label, goal, planPath, correlation, knowledgePath string, sources []SourceRef) Intent {
	var b strings.Builder
	b.WriteString("Load and follow the gogo-project-plan skill (use the Skill tool) to author this gogo PROJECT PLAN in place.")
	if g := strings.TrimSpace(goal); g != "" {
		b.WriteString(" The user's goal for this plan: ")
		b.WriteString(g)
		b.WriteString(". Analyze the project's sources and write the plan FOR THIS GOAL.")
	}
	b.WriteString(" The project-plan markdown file is at ")
	b.WriteString(planPath)
	b.WriteString(" - read and edit ONLY that one file.")
	if kp := strings.TrimSpace(knowledgePath); kp != "" {
		b.WriteString(" First READ the project's cross-repo domain knowledge under ")
		b.WriteString(kp)
		b.WriteString(" (e.g. project-knowledge.md) - how the sources connect, the shared glossary, and the integration contracts.")
	}
	if len(sources) > 0 {
		b.WriteString(" ANALYZE these SOURCE repos read-only by absolute path (code = source of truth) and decide which the plan needs - name -> path: ")
		parts := make([]string, len(sources))
		for i, s := range sources {
			lbl := strings.TrimSpace(s.Label)
			if lbl == "" {
				lbl = s.Path
			}
			parts[i] = lbl + " -> " + s.Path
		}
		b.WriteString(strings.Join(parts, "; "))
		b.WriteString(".")
	}
	if c := strings.TrimSpace(correlation); c != "" {
		b.WriteString(" The plan's correlation id is ")
		b.WriteString(c)
		b.WriteString(" - it is already in the file's front-matter; keep it.")
	}
	b.WriteString(" Output contract: set the front-matter targets: line to only the chosen source NAMES, and write a `## Source briefs` body section with a `### <source-name>` subsection per target giving that source's work-item brief.")
	b.WriteString(" This is a PROJECT-LEVEL plan under the gogo data home: edit ONLY that one markdown file. Do NOT write any source's .gogo/ and do NOT scaffold a .gogo/work/ - a work item is spawned separately, later, per target source.")
	return Intent{
		Action:  ActionAuthor,
		Command: b.String(),
		Session: sessionName("author", label),
	}
}

// sessionName builds "gogo-<action>-<sanitized>" (tmux-safe: lowercase,
// [a-z0-9-] only). tmux forbids '.' and ':' in session names.
func sessionName(action string, label string) string {
	return "gogo-" + action + "-" + sanitizeLabel(label)
}

// sanitizeLabel lowercases a slug/label and reduces it to tmux-safe [a-z0-9-]
// (the exact transform sessionName applies). Empty → "run".
func sanitizeLabel(label string) string {
	s := sessionUnsafe.ReplaceAllString(strings.ToLower(label), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "run"
	}
	return s
}

// SessionMatchesSlug reports whether a running tmux session name was created for
// slug, following the "gogo-<action>-<sanitized-slug>" convention (sessionName)
// plus uniqueSession's collision suffix ("-<n>"). This is an EXACT boundary
// match on the sanitized-slug component — NOT a substring search — so one
// feature's session is never misattributed to another whose sanitized name is a
// textual substring of it (e.g. session "gogo-done-awaiting-card" must not match
// slug "waiting-card"; TEST-005).
func SessionMatchesSlug(session, slug string) bool {
	sanitized := sanitizeLabel(slug)
	for _, action := range []Action{ActionGo, ActionPlan, ActionDone, ActionAccept} {
		base := "gogo-" + string(action) + "-" + sanitized
		if session == base {
			return true
		}
		// uniqueSession appends "-<n>" (n≥2) on a name collision — accept the
		// base name followed by a purely-numeric suffix, nothing else.
		if rest, ok := strings.CutPrefix(session, base+"-"); ok && allDigits(rest) {
			return true
		}
	}
	return false
}

// allDigits reports whether s is a non-empty run of ASCII digits.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// TmuxNewSessionArgs are the argv for `tmux <args>` that starts a detached,
// attachable session running the interactive claude command. No shell quoting
// is needed: tmux execs the command + its single argument directly, and the
// permission flag (FR8) is spliced in as its own argv elements — the slug is
// always a single, separate argv element, never a shell string.
// The session is anchored to the repo root (`-c root`): launching claude from
// wherever the board happened to run (e.g. cli/) makes Claude Code treat that
// dir as a NEW project — first-run MCP/trust prompts park the session
// (TEST-013). The repo root carries the user's existing approvals.
func TmuxNewSessionArgs(root string, in Intent) []string {
	args := []string{"new-session", "-d", "-s", in.Session, "-c", root, "claude"}
	args = append(args, PermissionArgs()...)
	return append(args, in.Command)
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

// CurrentSession returns the name of the tmux session this process is running
// inside (via `tmux display-message -p '#S'`), or "" when not inside tmux or
// tmux is absent. The sweeper's self-guard (FR3) uses it so `gogo sweep` never
// reaps the very session it is hosted in — e.g. a board-launched
// gogo-done-<slug> running /gogo:done, which flips its own member to shipped and
// then sweeps.
func CurrentSession() string {
	if os.Getenv("TMUX") == "" || !HasTmux() {
		return ""
	}
	out, err := exec.Command("tmux", "display-message", "-p", "#S").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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

// Launch spawns the intent. With tmux → a detached, attachable session running
// interactive claude, which the user attaches to in order to answer gates: claude
// stays alive (and the pane open) while parked at a gate, and when claude exits
// the pane closes by construction — no remain-on-exit, matching LaunchPersistent
// and the headless `-p` path, so a finished board launch leaves no dead pane
// (FR4; the remain-on-exit leak the incident hit). Without tmux → a backgrounded
// `claude -p` writing to a log under .gogo/resources/cli/logs/. NEVER call
// without a prior confirmation.
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
	cmd := exec.Command("claude", ClaudePrintArgs(in.Command)...)
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

// --- reaper + liveness (persistent-session lifecycle, FR6/FR8) ----------------

// KillSession stops a tmux session by name (`tmux kill-session -t <name>`,
// best-effort — single argv, no shell, injection-safe). The reaper (`gogo sweep`
// + the opportunistic reap) uses it to kill a tracked or orphaned `gogo-*`
// session so panes never pile up — the remain-on-exit leak the incident hit
// (7 orphaned sessions) is what this repairs (FR8). No tmux, no such session, or
// an empty name → a returned error; never a panic.
func KillSession(name string) error {
	if name == "" {
		return fmt.Errorf("empty tmux session name")
	}
	if !HasTmux() {
		return fmt.Errorf("tmux not installed")
	}
	return exec.Command("tmux", "kill-session", "-t", name).Run()
}

// PidAlive reports whether a process is alive via signal 0 (no signal is
// delivered — the call only probes whether the pid is signalable). The owner
// lock's liveness cross-check uses it (FR6): a lock whose recorded PID no longer
// answers signal-0 AND has no matching live `gogo-*` tmux session is stale and
// reclaimable. A non-positive pid is never alive; EPERM (exists but not ours to
// signal) still counts as alive.
func PidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// TmuxPersistentArgs builds the tmux argv for the `--attach` persistent session:
// `tmux new-session -d -s <name> -c root claude [--resume <uuid>|--session-id <uuid>] [perm] <command>`.
// The session flag + value are separate argv elements (injection-safe, like
// PermissionArgs); the slash command is the single final element. Unlike the
// board's TmuxNewSessionArgs this drives an INTERACTIVE claude (no `-p`) so the
// user can answer gates live in the warm session, and the caller never sets
// remain-on-exit — the pane closes when claude exits (no orphan by construction).
func TmuxPersistentArgs(root string, in Intent, opts PhaseOpts) []string {
	args := []string{"new-session", "-d", "-s", in.Session, "-c", root, "claude"}
	switch {
	case opts.Resume != "":
		args = append(args, "--resume", opts.Resume)
	case opts.SessionID != "":
		args = append(args, "--session-id", opts.SessionID)
	}
	args = append(args, PermissionArgs()...)
	return append(args, in.Command)
}

// LaunchPersistent starts a feature's ONE persistent session as a detached,
// attachable tmux session running interactive `claude` (the `--attach` path,
// D4=C): the user attaches to answer decision/UAT gates live in the warm session.
// It resumes the tracked uuid (opts.Resume) or starts a fresh one
// (opts.SessionID). Unlike the board's Launch it NEVER sets remain-on-exit, so
// when claude exits the pane closes and the session is gone by construction —
// the leak the incident hit is absent on this path (FR8). Needs tmux (that is
// what "attach" means); without it, drop `--attach` for the headless `-p` path.
func LaunchPersistent(root string, in Intent, opts PhaseOpts) (Result, error) {
	if !HasClaude() {
		return Result{}, fmt.Errorf("claude CLI not found on PATH — cannot launch %q", in.Command)
	}
	if !HasTmux() {
		return Result{}, fmt.Errorf("tmux not installed — --attach needs tmux (drop --attach to run headless -p)")
	}
	session := uniqueSession(in.Session)
	in.Session = session
	if err := exec.Command("tmux", TmuxPersistentArgs(root, in, opts)...).Run(); err != nil {
		return Result{}, fmt.Errorf("tmux new-session failed: %w", err)
	}
	return Result{Mode: "tmux", Session: session, Command: in.Command}, nil
}

func backgroundLogPath(root string, in Intent) (string, error) {
	dir := filepath.Join(root, ".gogo", "resources", "cli", "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := strings.TrimPrefix(in.Session, "gogo-")
	return filepath.Join(dir, name+".log"), nil
}
