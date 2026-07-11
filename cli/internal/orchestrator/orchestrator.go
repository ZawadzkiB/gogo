// Package orchestrator is the CLI process-orchestrator (gogo run): a deterministic
// Go sequencer that drives ②→③→④(→⑤) by spawning each phase as its own `claude -p`
// session — the DEVELOPER session kept warm across fix rounds via --resume (never
// re-reads the codebase), REVIEW and TEST spawned fresh (fresh eyes). It coexists
// with the in-chat `/gogo:go` orchestrator over the ONE shared core (the phase
// skills + the typed contracts); it re-implements no phase logic and reads the same
// issues.json/result.json to route (contract.Route). All judgment and every decision
// gate stay with the claude phase-sessions + the human — this package only
// sequences, resumes, routes, bounds, and books its own session registry.
package orchestrator

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
)

// Result classifies how a `gogo run` finished.
const (
	ResultAwaitingUAT = "awaiting-uat" // green through ⑤ → the user's UAT gate
	ResultGated       = "gated"        // paused for a human decision (FR8)
)

// Outcome is what Run returns: a terminal result plus, when gated, the reason.
type Outcome struct {
	Result string
	Gate   string
}

// Default loop bounds (FR7); overridable via env (see ConfigFromEnv).
const (
	DefaultMaxRounds   = 3    // implement↔review fix rounds on the same finding (mirrors the in-chat bound)
	DefaultCostCeiling = 10.0 // per-feature USD ceiling; 0 disables
)

// Env knobs for the bounds (FR7 / D6).
const (
	EnvMaxRounds   = "GOGO_RUN_MAX_ROUNDS"
	EnvCostCeiling = "GOGO_RUN_COST_CEILING"
)

// Invocation is one phase run the orchestrator hands to the runner. Exactly one of
// SessionID (a NEW session with a pre-assigned uuid) or Resume (continue a WARM
// session) is set; both empty is a one-shot (⑤ report).
type Invocation struct {
	Kind      string // implement | review | test | report (telemetry + label)
	Command   string // the full slash command, e.g. "/gogo:implement my-slug --in-session"
	SessionID string // --session-id <uuid>
	Resume    string // --resume <uuid>
}

// PhaseRunner runs one phase invocation and blocks until it exits. The production
// runner spawns `claude -p` (ClaudeRunner); tests inject a fake so the loop is
// asserted without spawning claude.
type PhaseRunner interface {
	Run(inv Invocation) (launch.RunResult, error)
}

// ClaudeRunner is the production PhaseRunner: it spawns `claude -p` via launch and
// waits for the process to exit (the race-free phase-done signal, D2).
type ClaudeRunner struct{ Root string }

// Run spawns the phase over `claude -p` and returns its parsed json envelope.
func (r ClaudeRunner) Run(inv Invocation) (launch.RunResult, error) {
	return launch.RunPhase(r.Root, inv.Command, launch.PhaseOpts{
		SessionID: inv.SessionID,
		Resume:    inv.Resume,
	})
}

// Orchestrator drives one feature's ②→③→④(→⑤) loop.
type Orchestrator struct {
	Root        string
	Slug        string
	Runner      PhaseRunner
	Reg         *Registry
	Out         io.Writer // notices + gate messages (os.Stdout in production)
	MaxRounds   int
	CostCeiling float64 // 0 = no ceiling
	Attach      bool    // D3 opt-in: on a gate, launch an interactive /gogo:resume session
}

// New builds an Orchestrator with a production ClaudeRunner and the feature's
// registry loaded (FR9 — resumes the same warm dev session on a re-run).
func New(root, slug string, cfg Config) *Orchestrator {
	return &Orchestrator{
		Root:        root,
		Slug:        slug,
		Runner:      ClaudeRunner{Root: root},
		Reg:         LoadRegistry(root, slug),
		Out:         cfg.Out,
		MaxRounds:   cfg.MaxRounds,
		CostCeiling: cfg.CostCeiling,
		Attach:      cfg.Attach,
	}
}

// Config carries the resolved run configuration.
type Config struct {
	Out         io.Writer
	MaxRounds   int
	CostCeiling float64
	Attach      bool
}

// ConfigFromEnv resolves the loop bounds from the environment, falling back to the
// defaults, and directs output to out (FR7 / D6).
func ConfigFromEnv(out io.Writer, attach bool) Config {
	cfg := Config{Out: out, MaxRounds: DefaultMaxRounds, CostCeiling: DefaultCostCeiling, Attach: attach}
	if v := os.Getenv(EnvMaxRounds); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxRounds = n
		}
	}
	if v := os.Getenv(EnvCostCeiling); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			cfg.CostCeiling = f
		}
	}
	return cfg
}

// RunnableStatus reports whether a feature's state.md status permits `gogo run` —
// the SAME acceptance gate `/gogo:go` enforces (FR1). `awaiting-uat` and
// `waiting-for-user` are NOT runnable (the user's UAT gate / a paused decision).
func RunnableStatus(status string) bool {
	switch status {
	case "plan-accepted", "implementing", "reviewing", "testing":
		return true
	}
	return false
}

// Run drives the loop and returns the Outcome. It never invents a decision: a fork
// surfaced by a phase (result.status waiting-for-user, a needs-user-decision
// finding, or a blown bound/ceiling) returns a gated Outcome for the human.
func (o *Orchestrator) Run() (Outcome, error) {
	// Pre-flight the COST ceiling: if a prior run already spent it, gate immediately
	// — re-running can only spend more, never less, so spawning a session just to
	// re-gate is a money sink (REV-003). The round bound is deliberately NOT
	// pre-flighted: a re-run lets review re-check, so a human who resolved the
	// findings can complete without any more fixes; only if MORE fixes are needed
	// does the exhausted round budget re-gate (with a raise-the-budget hint).
	if over, why := o.overBudget(); over {
		return o.gateBudget(why), nil
	}

	// ① Ensure a warm developer build exists. First ever run → fresh dev session;
	//    a re-run (registry has the dev uuid) skips straight to review (the dev is
	//    already warm and will be --resumed on the next fix).
	if o.Reg.DevUUID == "" {
		o.Reg.DevUUID = newUUID()
		if _, err := o.exec(Invocation{
			Kind:      "implement",
			Command:   o.cmd("/gogo:implement", "--in-session"),
			SessionID: o.Reg.DevUUID,
		}); err != nil {
			return Outcome{}, err
		}
		if gated, why := o.implementGate(); gated {
			return o.gateDecision(why), nil
		}
		if over, why := o.overBudget(); over {
			return o.gateBudget(why), nil
		}
	}

	// ② review ↔ implement ↔ test loop.
	for {
		// review sub-loop: review (fresh) until clean or gated, re-implementing (warm) on fixables.
		for {
			if _, err := o.exec(Invocation{
				Kind:      "review",
				Command:   o.cmd("/gogo:review"),
				SessionID: newUUID(),
			}); err != nil {
				return Outcome{}, err
			}
			if over, why := o.overBudget(); over {
				return o.gateBudget(why), nil
			}
			issues, result, err := o.readTrack(contract.TrackReview)
			if err != nil {
				return Outcome{}, err
			}
			if result == nil && issues == nil {
				return o.gateDecision("review produced no result.json or issues.json — cannot confirm it ran; treating as a failure, not a pass"), nil
			}
			switch contract.Route(contract.TrackReview, result, issues) {
			case contract.Gate:
				return o.gateDecision("review raised a finding that needs your decision"), nil
			case contract.ReImplement:
				if out, err := o.reImplement(contract.TrackReview); err != nil {
					return Outcome{}, err
				} else if out != nil {
					return *out, nil
				}
				continue // re-review (fresh eyes) after the warm fix
			case contract.Advance:
			}
			break // review clean → test
		}

		// test (fresh) → route.
		if _, err := o.exec(Invocation{
			Kind:      "test",
			Command:   o.cmd("/gogo:test"),
			SessionID: newUUID(),
		}); err != nil {
			return Outcome{}, err
		}
		if over, why := o.overBudget(); over {
			return o.gateBudget(why), nil
		}
		issues, result, err := o.readTrack(contract.TrackTest)
		if err != nil {
			return Outcome{}, err
		}
		if result == nil && issues == nil {
			return o.gateDecision("test produced no result.json or issues.json — cannot confirm it ran; treating as a failure, not a pass"), nil
		}
		switch contract.Route(contract.TrackTest, result, issues) {
		case contract.Gate:
			return o.gateDecision("test raised a finding (or a blocked hands-on check) that needs your decision"), nil
		case contract.ReImplement:
			if out, err := o.reImplement(contract.TrackTest); err != nil {
				return Outcome{}, err
			} else if out != nil {
				return *out, nil
			}
			continue // a test fix re-enters the review sub-loop, then re-tests
		case contract.Advance:
		}

		// ③ all green → report (fresh one-shot) → stop at the UAT gate.
		if _, err := o.exec(Invocation{
			Kind:    "report",
			Command: o.cmd("/gogo:report"),
		}); err != nil {
			return Outcome{}, err
		}
		o.printf("✓ %s — pipeline green; report written, stopped at awaiting-uat. Run `/gogo:done %s` to ship.\n", o.Slug, o.Slug)
		return Outcome{Result: ResultAwaitingUAT}, nil
	}
}

// reImplement runs a WARM dev fix round (--resume) for a track's open issues, after
// enforcing the round bound (FR7). It returns a non-nil *Outcome (a gate, with the
// right kind + message already applied) when the loop must stop, or (nil,nil) to
// continue. The round bound is a TOTAL fix-round budget per feature (not per finding
// — see FR7 note / REV-005); it errs safe, gating to a human rather than looping.
func (o *Orchestrator) reImplement(track string) (*Outcome, error) {
	if o.Reg.Round >= o.MaxRounds {
		out := o.gateBudget(fmt.Sprintf("the fix-round budget (%d total for this feature) is exhausted and findings remain", o.MaxRounds))
		return &out, nil
	}
	o.Reg.Round++
	issuesRel := o.relPath(track, "issues.json")
	if _, e := o.exec(Invocation{
		Kind:    "implement",
		Command: o.cmd("/gogo:implement", "--issues", issuesRel, "--in-session"),
		Resume:  o.Reg.DevUUID,
	}); e != nil {
		return nil, e
	}
	// A warm fix round can itself stop at a fork (blocked / waiting-for-user) — check
	// it here too, not only after the first build (REV-002). That is a DECISION gate.
	if g, w := o.implementGate(); g {
		out := o.gateDecision(w)
		return &out, nil
	}
	if over, w := o.overBudget(); over {
		out := o.gateBudget(w)
		return &out, nil
	}
	return nil, nil
}

// exec runs one invocation via the runner, records its telemetry into the registry,
// and persists the registry (best-effort). The registry is the CLI's own bookkeeping
// under .gogo/resources/ — never pipeline state.
func (o *Orchestrator) exec(inv Invocation) (launch.RunResult, error) {
	o.printf("→ %s\n", inv.Command)
	res, err := o.Runner.Run(inv)
	if err != nil {
		return res, err
	}
	// Book this run's telemetry BEFORE any halt: a run that finished but reported
	// is_error still spent tokens, so the cost accounting (and REV-003's cost
	// pre-flight) must see it (REV-006).
	uuid := inv.SessionID
	if inv.Resume != "" {
		uuid = inv.Resume
	}
	o.Reg.Phase = inv.Kind
	o.Reg.record(SessionInfo{
		Kind:       inv.Kind,
		UUID:       uuid,
		Round:      o.Reg.Round,
		Resumed:    inv.Resume != "",
		CostUSD:    res.CostUSD,
		NumTurns:   res.NumTurns,
		DurationMS: res.DurationMS,
	})
	o.save()
	if res.IsError {
		// The `claude -p` run finished but reported an internal error (is_error=true).
		// Halt rather than march on a failed phase as if it were green (REV-002).
		return res, fmt.Errorf("phase %q reported an error (claude is_error) — halting; not advancing on a failed phase", inv.Kind)
	}
	return res, nil
}

// implementGate reports whether the initial build stopped short (result.status
// blocked / waiting-for-user) so the loop gates instead of reviewing a non-build.
func (o *Orchestrator) implementGate() (bool, string) {
	res, err := contract.ReadResult(filepath.Join(o.featureDir(), "implement", "result.json"))
	if err != nil || res == nil {
		return false, ""
	}
	switch res.Status {
	case "blocked":
		return true, "implement is blocked (a gate failed and could not be repaired)"
	case "waiting-for-user":
		return true, "implement stopped at a decision that needs you"
	}
	return false, ""
}

// readTrack reads a review/test track's issues.json + result.json (both optional;
// absent → nil, routed defensively).
func (o *Orchestrator) readTrack(track string) (*contract.IssuesList, *contract.PhaseResult, error) {
	issues, err := contract.ReadIssues(filepath.Join(o.featureDir(), track, "issues.json"))
	if err != nil {
		return nil, nil, err
	}
	result, err := contract.ReadResult(filepath.Join(o.featureDir(), track, "result.json"))
	if err != nil {
		return nil, nil, err
	}
	return issues, result, nil
}

// overBudget reports whether the summed session cost crossed the ceiling (FR7).
func (o *Orchestrator) overBudget() (bool, string) {
	if o.CostCeiling > 0 && o.Reg.CostUSD > o.CostCeiling {
		return true, fmt.Sprintf("cost ceiling $%.2f exceeded (spent $%.2f)", o.CostCeiling, o.Reg.CostUSD)
	}
	return false, ""
}

// gateDecision pauses for a genuine fork (a needs-user-decision finding, a phase
// that stopped at waiting-for-user/blocked, or a phase that produced no output). The
// human resolves it — via /gogo:resume, or with --attach an interactive session
// (reusing the existing attachable-tmux path) — then re-runs `gogo run` to continue
// the warm session (FR8 / D3).
func (o *Orchestrator) gateDecision(why string) Outcome {
	o.save()
	o.printf("\n⏸  gogo run paused — %s\n", why)
	o.printf("   the feature is parked; see %s/decisions.md.\n", o.featureRel())
	if o.Attach && launch.HasTmux() && launch.HasClaude() {
		if res, err := launch.Launch(o.Root, launch.ResumeIntent(o.Slug)); err == nil {
			o.printf("   attach to answer:  tmux %s\n", joinArgs(launch.AttachArgs(res.Session)))
			return Outcome{Result: ResultGated, Gate: why}
		}
	}
	o.printf("   resolve it (e.g. `/gogo:resume %s` in a Claude session), then re-run `gogo run %s` to continue the warm session.\n", o.Slug, o.Slug)
	return Outcome{Result: ResultGated, Gate: why}
}

// gateBudget pauses because the loop's own budget is spent (the round bound or the
// cost ceiling). Unlike a decision gate, re-running as-is cannot make progress on
// the same findings: the human must raise the budget or resolve the findings — so
// the hint is different (REV-003), never the misleading "re-run to continue".
func (o *Orchestrator) gateBudget(why string) Outcome {
	o.save()
	o.printf("\n⏸  gogo run paused — %s\n", why)
	o.printf("   the loop budget for this feature is spent. Raise %s / %s and re-run,\n", EnvMaxRounds, EnvCostCeiling)
	o.printf("   or resolve the open findings first (see %s/review and %s/test).\n", o.featureRel(), o.featureRel())
	return Outcome{Result: ResultGated, Gate: why}
}

// --- small helpers -----------------------------------------------------------

// cmd builds a slash command string: "/gogo:<verb> <slug> [extra...]".
func (o *Orchestrator) cmd(verb string, extra ...string) string {
	s := verb + " " + o.Slug
	for _, e := range extra {
		s += " " + e
	}
	return s
}

func (o *Orchestrator) featureRel() string { return ".gogo/work/feature-" + o.Slug }
func (o *Orchestrator) featureDir() string {
	return filepath.Join(o.Root, ".gogo", "work", "feature-"+o.Slug)
}

// relPath is a repo-relative path (forward-slashed) to a track file, safe to pass
// in a slash command since the phase session runs with cwd=root.
func (o *Orchestrator) relPath(track, file string) string {
	return o.featureRel() + "/" + track + "/" + file
}

func (o *Orchestrator) save() {
	if o.Reg != nil {
		_ = o.Reg.Save(o.Root) // best-effort; CLI-only bookkeeping
	}
}

func (o *Orchestrator) printf(format string, a ...any) {
	if o.Out != nil {
		fmt.Fprintf(o.Out, format, a...)
	}
}

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
