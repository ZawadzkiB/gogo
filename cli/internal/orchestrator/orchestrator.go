// Package orchestrator is the CLI's persistent-session lifecycle manager. It does
// NOT re-implement the pipeline loop: `gogo go <slug>` / `gogo plan <slug>`
// launch-or-`--resume` ONE persistent `claude -p` session running the existing
// `/gogo:go` / `/gogo:plan` skill (implement warm in-context + review/test as
// nested Task subagents + report), and this package only manages that session's
// lifecycle — guard the one-owner lock, resolve fresh-vs-resume from the session
// registry, classify the child's exit, book telemetry, and reap. There is exactly
// ONE routing rule and it lives in the skill (deleting the drift bug the old Go
// per-phase loop + contract.Route created). All judgment and every decision gate
// stay with the claude session + the human.
package orchestrator

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
)

// Leg outcome results (FR4). The cmd layer maps these to exit codes.
const (
	ResultAwaitingUAT = "awaiting-uat" // green through ⑤ → the user's UAT gate (exit 0)
	ResultParked      = "parked"       // exited at a decision gate / waiting-for-user (exit 2)
	ResultRefused     = "refused"      // a live owner already holds the lock (exit 1)
	ResultAttached    = "attached"     // --attach: an attachable session was launched (exit 0)
	ResultTerminal    = "terminal"     // feature already shipped/aborted — nothing to run (exit 0)
	ResultOther       = "other"        // exited at some other status (surfaced; exit 2)
)

// Outcome is what LaunchOrResume returns: the leg result + the state.md status
// observed at exit (FR4).
type Outcome struct {
	Result string
	Status string
}

// Invocation is one persistent-session launch the manager hands to the runner.
// Exactly one of SessionID (a NEW session, pre-assigned uuid — the first leg) or
// Resume (continue the WARM session) is set.
type Invocation struct {
	Kind      string // go | plan (telemetry + label)
	Command   string // the full slash command, e.g. "/gogo:go my-slug"
	SessionID string // --session-id <uuid>
	Resume    string // --resume <uuid>
}

// SessionRunner runs one headless `-p` persistent-session invocation and BLOCKS
// until it exits (the race-free leg-complete signal, D2). Production =
// ClaudeRunner; tests inject a fake so the manager is asserted without spawning
// claude.
type SessionRunner interface {
	Run(inv Invocation) (launch.RunResult, error)
}

// ClaudeRunner is the production SessionRunner: it spawns `claude -p` via launch
// and waits for the process to exit.
type ClaudeRunner struct{ Root string }

// Run spawns the persistent session over `claude -p` and returns its parsed json
// envelope.
func (r ClaudeRunner) Run(inv Invocation) (launch.RunResult, error) {
	return launch.RunPhase(r.Root, inv.Command, launch.PhaseOpts{
		SessionID: inv.SessionID,
		Resume:    inv.Resume,
	})
}

// AttachFn launches the persistent session as an attachable tmux session (the
// --attach path). Production = launch.LaunchPersistent.
type AttachFn func(root string, in launch.Intent, opts launch.PhaseOpts) (launch.Result, error)

// Session is the lifecycle manager for one feature leg (`go` | `plan`).
type Session struct {
	Root     string
	Slug     string
	Kind     string // "go" | "plan"
	Reg      *Registry
	Runner   SessionRunner // headless -p runner (blocks); nil → ClaudeRunner
	Attacher AttachFn      // --attach launcher; nil → launch.LaunchPersistent
	Killer   func(name string) error
	Lister   func() []string // list live gogo-* sessions; nil → launch.ListSessions
	Live     LivenessFn      // liveness cross-check; nil → DefaultLiveness
	Out      io.Writer
	Attach   bool
	Takeover bool
}

// RunnableStatus reports whether a feature's state.md status permits `gogo go` —
// the SAME acceptance gate `/gogo:go` enforces (FR3). `awaiting-uat` and
// `waiting-for-user` are NOT runnable (the user's UAT gate / a paused decision).
func RunnableStatus(status string) bool {
	switch status {
	case "plan-accepted", "implementing", "reviewing", "testing":
		return true
	}
	return false
}

// PlannableStatus reports whether `gogo plan` may run for a feature at this status
// (FR3). Planning creates or revises a plan, so it is permitted for a brand-new
// feature ("") and any non-terminal one; a shipped/aborted feature has nothing to
// plan.
func PlannableStatus(status string) bool {
	return !TerminalStatus(status)
}

// TerminalStatus reports whether a feature is in a terminal (no-live-session)
// state — the kill-at-ship trigger for the opportunistic reap + sweep (FR8/FR9).
func TerminalStatus(status string) bool {
	switch status {
	case "shipped", "aborted", "done":
		return true
	}
	return false
}

// LaunchOrResume is the whole `gogo go`/`gogo plan` action (FR1/FR2/FR4/FR5/FR8):
// opportunistic-reap-if-terminal → acquire the owner lock (refuse / reclaim /
// takeover) → resolve fresh-vs-resume from the registry → run the persistent
// session (headless `-p`, or launch an attachable tmux with --attach) → classify
// the exit → book telemetry + update the registry → release the lock.
func (s *Session) LaunchOrResume() (Outcome, error) {
	s.defaults()

	// Opportunistic reap: a terminal feature keeps no live session. Reap any
	// tracked/leftover one and refuse to launch (FR8 — kill-at-ship backstop).
	if feat := s.feature(); feat != nil && TerminalStatus(feat.Status) {
		s.reapTracked()
		s.printf("… %s is %s — nothing to run; reaped any tracked session.\n", s.Slug, feat.Status)
		return Outcome{Result: ResultTerminal, Status: feat.Status}, nil
	}

	inv := ResolveInvocation(s.Reg, s.Kind, s.Slug)
	me := Owner{
		PID:       os.Getpid(),
		UUID:      invUUID(inv),
		Host:      hostname(),
		Kind:      s.Kind,
		StartedAt: now(),
	}
	if s.Attach {
		me.Tmux = s.intent().Session // provisional base name; SetTmux records the real one after launch
	}

	lock, taken, prior, err := Acquire(s.Root, s.Slug, me, s.Takeover, s.Live)
	if err != nil {
		return Outcome{}, fmt.Errorf("acquire owner lock for %q: %w", s.Slug, err)
	}
	if taken {
		s.printRefusal(prior)
		return Outcome{Result: ResultRefused, Status: ""}, nil
	}
	if prior != nil {
		// Reclaimed-stale, seized-via-takeover, or seized-over-an-untracked-board
		// session → reap the prior. Reap BY SLUG (every live gogo-* session for the
		// slug, exact SessionMatchesSlug) so a collision-suffixed or lockless
		// board session is killed, not the stale base name alone (REV-002).
		s.reapMatchingSessions(s.Slug)
		if prior.Tmux != "" {
			_ = s.Killer(prior.Tmux)
		}
	}

	// Book the persistent session (uuid, kind, status running) BEFORE launching, so
	// a crash mid-run still leaves the resume uuid on disk.
	ps := s.Reg.Ensure(s.Kind)
	ps.UUID = me.UUID
	ps.PID = me.PID
	ps.Status = SessRunning
	if ps.StartedAt == "" {
		ps.StartedAt = me.StartedAt
	}
	ps.UpdatedAt = now()
	if s.Attach {
		ps.Tmux = me.Tmux
	}
	s.save()

	if s.Attach {
		return s.launchAttached(inv, lock)
	}
	return s.runHeadless(inv, lock)
}

// runHeadless spawns the blocking `claude -p` session, books its telemetry (even
// on error — the cost was spent), releases the lock (the child has exited, this
// process no longer owns the feature), then classifies the exit.
func (s *Session) runHeadless(inv Invocation, lock *Lock) (Outcome, error) {
	s.printf("→ claude -p %q  (%s)\n", inv.Command, resumeOrFresh(inv))
	res, runErr := s.Runner.Run(inv)
	s.book(inv, res)
	_ = lock.Release()

	if runErr != nil {
		s.markStatus(SessParked)
		s.save()
		return Outcome{}, runErr
	}
	if res.IsError {
		// The `claude -p` run finished but reported an internal error. Halt rather
		// than march a failed leg on as if it were green (FR4).
		s.markStatus(SessParked)
		s.save()
		return Outcome{}, fmt.Errorf("the persistent session for %q reported an error (claude is_error) — halting; not advancing a failed leg as green", s.Slug)
	}
	out := s.classifyExit()
	s.save()
	return out, nil
}

// launchAttached starts the attachable tmux session (--attach), records the real
// (collision-suffixed) tmux name, and DELIBERATELY does not release the lock: the
// session lives on in tmux and the lock's tmux-liveness signal must keep the
// feature owned until it is reaped (`gogo sweep` / opportunistic).
func (s *Session) launchAttached(inv Invocation, lock *Lock) (Outcome, error) {
	res, err := s.Attacher(s.Root, s.intent(), launch.PhaseOpts{SessionID: inv.SessionID, Resume: inv.Resume})
	if err != nil {
		_ = lock.Release()
		s.markStatus(SessParked)
		s.save()
		return Outcome{}, err
	}
	ps := s.Reg.Ensure(s.Kind)
	ps.Tmux = res.Session // the real name reap will kill
	ps.Status = SessRunning
	ps.UpdatedAt = now()
	s.save()
	_ = lock.SetTmux(res.Session) // record the REAL (post-collision) name in the lock (REV-002)
	s.printf("▶ %s — attachable warm session launched: %s (%s)\n", s.Slug, res.Session, resumeOrFresh(inv))
	s.printf("   attach to drive it live:  tmux %s\n", joinArgs(launch.AttachArgs(res.Session)))
	s.printf("   it is reaped at close (no lingering pane); `gogo sweep` cleans up an orphan.\n")
	return Outcome{Result: ResultAttached, Status: "running"}, nil
}

// classifyExit reads state.md (the deterministic reader) after the `-p` child has
// exited and surfaces the leg's outcome (FR4).
func (s *Session) classifyExit() Outcome {
	feat := s.feature()
	status := ""
	if feat != nil {
		status = feat.Status
	}
	switch status {
	case "awaiting-uat":
		s.markStatus(SessAwaitingUAT)
		s.printf("✓ %s — pipeline green; stopped at awaiting-uat. Run `/gogo:done %s` to ship.\n", s.Slug, s.Slug)
		return Outcome{Result: ResultAwaitingUAT, Status: status}
	case "waiting-for-user":
		s.markStatus(SessParked)
		s.printParkedGate(feat)
		return Outcome{Result: ResultParked, Status: status}
	case "shipped", "aborted", "done":
		s.markStatus(SessShipped)
		s.reapTracked()
		s.printf("… %s is %s.\n", s.Slug, status)
		return Outcome{Result: ResultTerminal, Status: status}
	default:
		// The leg ended without reaching a gate (e.g. interrupted mid-phase, or the
		// plan leg left it awaiting-plan-acceptance). Surface the raw status + how to
		// resume the warm session; never claim green.
		s.markStatus(SessParked)
		s.printf("• %s — session ended at status %q. Re-run `gogo %s %s` to resume the warm session.\n", s.Slug, status, s.Kind, s.Slug)
		return Outcome{Result: ResultOther, Status: status}
	}
}

// Reap kills this feature's tracked session (tmux + registry mark) and releases
// its lock (FR8). Best-effort; safe when nothing is tracked.
func (s *Session) Reap() {
	s.defaults()
	s.reapTracked()
}

// reapMatchingSessions kills every live gogo-* tmux session that attributes to the
// slug by the exact convention parse (launch.SessionMatchesSlug — never substring,
// TEST-005). This is the robust reap the prior-owner path uses: it catches a
// collision-suffixed attach session and a lockless board-launched racer alike,
// where the lockfile's recorded name would be wrong or absent (REV-002).
func (s *Session) reapMatchingSessions(slug string) {
	for _, name := range s.Lister() {
		if launch.SessionMatchesSlug(name, slug) {
			_ = s.Killer(name)
		}
	}
}

// reapTracked kills every tracked persistent session's tmux (if any), marks them
// reaped, releases the lock, and persists. The headless `-p` path has no tmux to
// kill — there the mark + lock release is what matters.
func (s *Session) reapTracked() {
	for _, ps := range s.Reg.Persistent {
		if ps == nil {
			continue
		}
		if ps.Tmux != "" && ps.Status != SessReaped {
			_ = s.Killer(ps.Tmux)
		}
		ps.Status = SessReaped
		ps.UpdatedAt = now()
	}
	_ = releaseLock(s.Root, s.Slug)
	s.save()
}

// --- resolver (pure — the FR1/FR2 launch-or-resume decision) ------------------

// ResolveInvocation decides the fresh-vs-resume launch for a leg from the
// registry: no tracked persistent session for the kind → a fresh --session-id with
// a new uuid; a tracked one → --resume its uuid. This is the pure resolver the
// tests table-drive.
func ResolveInvocation(reg *Registry, kind, slug string) Invocation {
	cmd := commandFor(kind, slug)
	if ps := reg.Get(kind); ps != nil && ps.UUID != "" {
		return Invocation{Kind: kind, Command: cmd, Resume: ps.UUID}
	}
	return Invocation{Kind: kind, Command: cmd, SessionID: newUUID()}
}

func commandFor(kind, slug string) string {
	if kind == "plan" {
		return "/gogo:plan " + slug
	}
	return "/gogo:go " + slug
}

// --- telemetry + status bookkeeping ------------------------------------------

func (s *Session) book(inv Invocation, res launch.RunResult) {
	s.Reg.record(SessionInfo{
		Kind:       inv.Kind,
		UUID:       invUUID(inv),
		Resumed:    inv.Resume != "",
		CostUSD:    res.CostUSD,
		NumTurns:   res.NumTurns,
		DurationMS: res.DurationMS,
	})
	if ps := s.Reg.Get(s.Kind); ps != nil {
		ps.CostUSD += res.CostUSD
		ps.NumTurns += res.NumTurns
		ps.UpdatedAt = now()
	}
}

func (s *Session) markStatus(st string) {
	if ps := s.Reg.Get(s.Kind); ps != nil {
		ps.Status = st
		ps.UpdatedAt = now()
	}
}

// --- surfaces ----------------------------------------------------------------

func (s *Session) printRefusal(prior *Owner) {
	s.printf("✗ %s is already owned by a live session — refusing (D6: refuse-by-default).\n", s.Slug)
	switch {
	case prior == nil:
	case prior.Kind == "untracked":
		// A live gogo-* session holds the slug but wrote no lockfile — a board-launched
		// racer. Name it so the user can attach or takeover.
		if name := s.matchingSession(); name != "" {
			s.printf("   owner: an untracked live session (likely board-launched): %s\n", name)
			s.printf("   attach:  tmux %s\n", joinArgs(launch.AttachArgs(name)))
		} else {
			s.printf("   owner: an untracked live session (likely board-launched).\n")
		}
	default:
		s.printf("   owner: %s (pid %d", orDefault(prior.Kind, "session"), prior.PID)
		if prior.Tmux != "" {
			s.printf(", tmux %s", prior.Tmux)
		}
		if prior.Host != "" {
			s.printf(", host %s", prior.Host)
		}
		s.printf(", since %s)\n", prior.StartedAt)
		if prior.Tmux != "" {
			s.printf("   attach:  tmux %s\n", joinArgs(launch.AttachArgs(prior.Tmux)))
		}
	}
	s.printf("   re-run with --takeover to seize it (the prior is reaped), or `gogo sweep` if it is stale.\n")
}

// matchingSession returns the first live gogo-* session that attributes to this
// feature's slug, or "" — used to name an untracked (board-launched) owner.
func (s *Session) matchingSession() string {
	for _, name := range s.Lister() {
		if launch.SessionMatchesSlug(name, s.Slug) {
			return name
		}
	}
	return ""
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func (s *Session) printParkedGate(feat *contract.Feature) {
	dec := ""
	if feat != nil {
		dec = feat.OpenDecision
	}
	s.printf("⏸  %s — parked for you (waiting-for-user).\n", s.Slug)
	if dec != "" && dec != "none" {
		s.printf("   open decision: %s — see %s/decisions.md.\n", dec, s.featureRel())
	} else {
		s.printf("   see %s/decisions.md for the parked decision.\n", s.featureRel())
	}
	s.printf("   resolve it (`/gogo:resume %s`), then re-run `gogo %s %s` to resume the warm session.\n", s.Slug, s.Kind, s.Slug)
}

// --- small helpers -----------------------------------------------------------

func (s *Session) defaults() {
	if s.Reg == nil {
		s.Reg = LoadRegistry(s.Root, s.Slug)
	}
	if s.Kind == "" {
		s.Kind = "go"
	}
	if s.Runner == nil {
		s.Runner = ClaudeRunner{Root: s.Root}
	}
	if s.Attacher == nil {
		s.Attacher = launch.LaunchPersistent
	}
	if s.Killer == nil {
		s.Killer = launch.KillSession
	}
	if s.Lister == nil {
		s.Lister = launch.ListSessions
	}
	if s.Live == nil {
		s.Live = DefaultLiveness
	}
}

// feature reloads the feature from the deterministic reader (state.md), or nil.
func (s *Session) feature() *contract.Feature {
	repo, err := contract.LoadRepo(s.Root)
	if err != nil || repo == nil {
		return nil
	}
	return repo.Feature(s.Slug)
}

func (s *Session) intent() launch.Intent {
	action := launch.ActionGo
	if s.Kind == "plan" {
		action = launch.ActionPlan
	}
	return launch.BuildIntent(action, []string{s.Slug}, "")
}

func (s *Session) featureRel() string { return ".gogo/work/feature-" + s.Slug }

func (s *Session) save() {
	if s.Reg != nil {
		_ = s.Reg.Save(s.Root) // best-effort; CLI-only bookkeeping
	}
}

func (s *Session) printf(format string, a ...any) {
	if s.Out != nil {
		fmt.Fprintf(s.Out, format, a...)
	}
}

func invUUID(inv Invocation) string {
	if inv.Resume != "" {
		return inv.Resume
	}
	return inv.SessionID
}

func resumeOrFresh(inv Invocation) string {
	if inv.Resume != "" {
		return "resume warm session"
	}
	return "fresh session"
}

func hostname() string {
	h, _ := os.Hostname()
	return h
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// newUUID returns a random RFC-4122 v4 UUID (stdlib-only, no new dependency).
func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// joinArgs renders an argv slice as a space-joined command tail for a printed hint.
func joinArgs(args []string) string {
	s := ""
	for i, a := range args {
		if i > 0 {
			s += " "
		}
		s += a
	}
	return s
}
