// Package plans is the CLI-owned reader/writer for PROJECT-SCOPED plans — the one
// store that collapses the former global drafts (P3) and epics (P4) stores into a
// single lifecycle entity (D8). A PLAN belongs to ONE home-folder project and lives
// as a hand-editable markdown file at
// ~/.gogo/projects/<project>/.gogo/plans/<plan-id>.md (front-matter + free-text
// body, no YAML dependency — the same shape the drafts store used). A plan owns a
// stable correlation id (plan-<hash>), a status in the lifecycle
// draft → ready → active → done, a set of TARGET sources, and a MANY-TO-MANY set of
// MEMBERS (the work items it spawned/linked into sources).
//
// Like the config/projects stores (and UNLIKE the read-only contract reader of a
// source's .gogo/ file surface) this package is deliberately WRITE-capable: a plan
// is CLI-owned data, NOT pipeline state, so `gogo plan …` (+ the `draft`/`epic`
// aliases) and the plans tab mutate it. The hard "CLI reads, skills write" invariant
// is about a SOURCE's .gogo/work/ — which this package NEVER writes: it writes ONLY
// the project's own home folder under ~/.gogo/. Spawning a work item into a source
// is a `claude -p` `gogo:plan` launch (the skill writes the work item + stamps the
// correlation), never a CLI write.
//
// Every read is defensive: a missing/malformed dir or file degrades to an empty
// result with NO error (mirroring config.Load and the contract parsers), never a
// crash.
package plans

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// Plan status lifecycle (D8). A plan advances draft → ready (targets chosen /
// user-marked) → active (≥1 member spawned) → done (all members shipped; terminal /
// optional in v1). `draft` == the old "draft"; `active` == the old "epic".
const (
	StatusDraft  = "draft"
	StatusReady  = "ready"
	StatusActive = "active"
	StatusDone   = "done"
)

// StatusAwaitingProjectUAT is the DERIVED (display-only, never persisted) status of a
// plan whose every member work item is shipped and which is awaiting the project-UAT
// accept (FR3). The persisted lifecycle stays draft → ready → active → done; this is
// computed at read time by DerivedStatus and shown by the plans tab, so nothing new is
// stored until `gogo plan done` (MarkDone) flips the plan to the persisted `done`.
const StatusAwaitingProjectUAT = "awaiting-project-uat"

// frontFence is the `---` line that opens and closes the front-matter block.
const frontFence = "---"

// Member is one work item a plan spawned or linked into a source: the source name
// (the label the board tags cards with) and the feature slug hint. For a spawn-time
// member the slug is the advisory SlugFromTitle (the analyst derives the real one);
// for a retroactive link it is the real on-disk slug. Membership is a SET keyed on
// (Source, SlugHint).
type Member struct {
	Source   string
	SlugHint string
}

// Plan is one project-scoped plan entity (the collapsed draft+epic).
type Plan struct {
	ID          string   // plan-<hash> — the stable correlation id stamped into state.md
	Title       string   //
	Description string   // the free-text body (the plan brief / goal)
	Status      string   // draft | ready | active | done (D8)
	Targets     []string // source names this plan targets (spawnable into)
	Members     []Member // work items spawned/linked (many-to-many)
	Created     string   // RFC3339 — drives newest-first
}

// HasMembers reports whether the plan has ≥1 member (the old "epic" shape).
func (p Plan) HasMembers() bool { return len(p.Members) > 0 }

// Dir is a project's plans directory ~/.gogo/projects/<project>/.gogo/plans.
func Dir(project string) string {
	return filepath.Join(projects.Dir(project), ".gogo", "plans")
}

// Path is the absolute .md path for a plan id under a project's plans dir.
func Path(project, id string) string { return filepath.Join(Dir(project), id+".md") }

var idUnsafe = regexp.MustCompile(`[^a-z0-9-]+`)

// validID reports whether id is a safe single path component (plan-[a-z0-9-]) — the
// write-scope guard so an id carrying `/`, `\` or `..` can never escape the plans
// dir.
func validID(id string) bool {
	return id != "" && id != "." && id != ".." &&
		!strings.ContainsAny(id, `/\`) && filepath.Base(id) == id &&
		!idUnsafe.MatchString(id)
}

// MintID mints the stable correlation id for a title created at now: the pure
// helper "plan-" + a short hex of sha256(title + "\x00" + timestamp). The timestamp
// is PASSED IN (never read via time.Now inside here) so the helper is deterministic
// and unit-testable / migration-idempotent from a fixed instant. The result is
// [a-z0-9-] only (filter- and tmux-safe): hex is a subset, "plan-" is literal.
func MintID(title string, now time.Time) string {
	seed := strings.TrimSpace(title) + "\x00" + now.UTC().Format(time.RFC3339Nano)
	sum := sha256.Sum256([]byte(seed))
	return "plan-" + hex.EncodeToString(sum[:])[:8]
}

// uniqueID returns base, or base-2 / base-3 / … — the first id not already in
// existing — so a (astronomically unlikely) hash collision never overwrites a plan.
func uniqueID(base string, existing map[string]bool) string {
	if !existing[base] {
		return base
	}
	for i := 2; i < 10000; i++ {
		cand := base + "-" + strconv.Itoa(i)
		if !existing[cand] {
			return cand
		}
	}
	return base
}

// parsePlan reads one plan .md leniently: a fenced front-matter block of
// `key: value` lines between the first two `---` fences, then the free-text body.
// A file with no opening fence is treated as all body, a malformed line is skipped,
// and unknown keys are ignored — never a crash (the drafts parser's defensive
// style). id is the filename stem (the on-disk identity).
func parsePlan(id string, raw []byte) Plan {
	p := Plan{ID: id, Status: StatusDraft}
	lines := strings.Split(string(raw), "\n")

	i := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == frontFence {
		close := -1
		for j := 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == frontFence {
				close = j
				break
			}
		}
		if close >= 0 {
			for _, ln := range lines[1:close] {
				key, val, ok := strings.Cut(ln, ":")
				if !ok {
					continue // skip a malformed front-matter line, never crash
				}
				key = strings.ToLower(strings.TrimSpace(key))
				val = strings.TrimSpace(val)
				switch key {
				case "id":
					if val != "" {
						p.ID = val
					}
				case "title":
					p.Title = val
				case "status":
					p.Status = normalizeStatus(val)
				case "created":
					p.Created = val
				case "targets":
					p.Targets = parseList(val)
				case "members":
					p.Members = parseMembers(val)
				}
			}
			i = close + 1
		}
	}
	p.Description = strings.Trim(strings.Join(lines[i:], "\n"), "\n")
	return p
}

// normalizeStatus maps a parsed status onto the known lifecycle, defaulting an
// empty/unknown value to draft (the safest resting state).
func normalizeStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case StatusReady:
		return StatusReady
	case StatusActive:
		return StatusActive
	case StatusDone:
		return StatusDone
	default:
		return StatusDraft
	}
}

// parseList splits a `a, b, c` (optionally `[a, b]`) value into trimmed, non-empty
// tokens.
func parseList(v string) []string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "[")
	v = strings.TrimSuffix(v, "]")
	var out []string
	for _, tok := range strings.Split(v, ",") {
		if tok = strings.TrimSpace(tok); tok != "" {
			out = append(out, tok)
		}
	}
	return out
}

// parseMembers parses a `source:slug, source2:slug2` value into a Member set. A
// token with no ':' is skipped (a target belongs in `targets`, not `members`).
func parseMembers(v string) []Member {
	var out []Member
	for _, tok := range parseList(v) {
		if s, slug, ok := strings.Cut(tok, ":"); ok {
			s, slug = strings.TrimSpace(s), strings.TrimSpace(slug)
			if s != "" && slug != "" {
				out = append(out, Member{Source: s, SlugHint: slug})
			}
		}
	}
	return out
}

// render serializes a plan to its on-disk markdown form: a fenced front-matter
// block (set keys, stable order) then a blank line + the body + a trailing newline.
func (p Plan) render() []byte {
	var b strings.Builder
	b.WriteString(frontFence + "\n")
	fmt.Fprintf(&b, "id: %s\n", p.ID)
	fmt.Fprintf(&b, "title: %s\n", p.Title)
	status := p.Status
	if status == "" {
		status = StatusDraft
	}
	fmt.Fprintf(&b, "status: %s\n", status)
	if p.Created != "" {
		fmt.Fprintf(&b, "created: %s\n", p.Created)
	}
	if len(p.Targets) > 0 {
		fmt.Fprintf(&b, "targets: %s\n", strings.Join(p.Targets, ", "))
	}
	if len(p.Members) > 0 {
		parts := make([]string, len(p.Members))
		for i, m := range p.Members {
			parts[i] = m.Source + ":" + m.SlugHint
		}
		fmt.Fprintf(&b, "members: %s\n", strings.Join(parts, ", "))
	}
	b.WriteString(frontFence + "\n")
	if body := strings.Trim(p.Description, "\n"); body != "" {
		b.WriteString("\n" + body + "\n")
	}
	return []byte(b.String())
}

// List returns every plan of a project newest-first (Created desc, tie-broken by ID
// asc — deterministic). A missing/unreadable dir degrades to an EMPTY list with NO
// error (the no-plans fallback), and an unreadable individual file is skipped.
func List(project string) ([]Plan, error) {
	entries, err := os.ReadDir(Dir(project))
	if err != nil {
		return nil, nil // missing / unreadable dir → empty, never a crash
	}
	var out []Plan
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(Dir(project), e.Name()))
		if rerr != nil {
			continue
		}
		out = append(out, parsePlan(strings.TrimSuffix(e.Name(), ".md"), raw))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Created != out[j].Created {
			return out[i].Created > out[j].Created // newest-first
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// Get returns the plan with id in project, and whether it was found.
func Get(project, id string) (Plan, bool) {
	if !validID(id) {
		return Plan{}, false
	}
	raw, err := os.ReadFile(Path(project, id))
	if err != nil {
		return Plan{}, false
	}
	return parsePlan(id, raw), true
}

// Save writes p under the project's plans dir, creating it when absent. A plan with
// an invalid ID is refused (the write-scope guard).
func Save(project string, p Plan) error {
	if !validID(p.ID) {
		return fmt.Errorf("plans: invalid plan id %q", p.ID)
	}
	if err := os.MkdirAll(Dir(project), 0o755); err != nil {
		return err
	}
	return os.WriteFile(Path(project, p.ID), p.render(), 0o644)
}

// New mints a fresh DRAFT plan from title + optional description, stamps Created =
// now (RFC3339, UTC — drives newest-first), assigns a deduped plan-<hash> id, and
// persists it. Targets/members are added afterward via AddTarget/AddMember.
func New(project, title, description string) (Plan, error) {
	existing := existingIDs(project)
	p := Plan{
		ID:          uniqueID(MintID(title, time.Now()), existing),
		Title:       strings.TrimSpace(title),
		Description: strings.TrimSpace(description),
		Status:      StatusDraft,
		Created:     time.Now().UTC().Format(time.RFC3339),
	}
	if err := Save(project, p); err != nil {
		return Plan{}, err
	}
	return p, nil
}

// existingIDs is the set of plan ids already present in a project (for id-dedup).
func existingIDs(project string) map[string]bool {
	ids := map[string]bool{}
	all, _ := List(project)
	for _, p := range all {
		ids[p.ID] = true
	}
	return ids
}

// Delete removes the plan with id from project. removed=false (no error, no write)
// when it was absent — a graceful no-op mirroring config.Remove.
func Delete(project, id string) (removed bool, err error) {
	if !validID(id) {
		return false, nil
	}
	p := Path(project, id)
	if _, serr := os.Stat(p); serr != nil {
		return false, nil
	}
	if err := os.Remove(p); err != nil {
		return false, err
	}
	return true, nil
}

// AddTarget adds source to the plan's Targets (a SET op — an existing target is an
// idempotent no-op). Returns the updated plan; a missing plan is an error.
func AddTarget(project, id, source string) (Plan, error) {
	p, ok := Get(project, id)
	if !ok {
		return Plan{}, os.ErrNotExist
	}
	source = strings.TrimSpace(source)
	if source == "" {
		return p, nil
	}
	for _, t := range p.Targets {
		if t == source {
			return p, nil // already a target — idempotent
		}
	}
	p.Targets = append(p.Targets, source)
	return p, Save(project, p)
}

// RemoveTarget removes source from the plan's Targets. removed=false (no write) when
// the plan or the target is absent.
func RemoveTarget(project, id, source string) (removed bool, err error) {
	p, ok := Get(project, id)
	if !ok {
		return false, nil
	}
	kept := make([]string, 0, len(p.Targets))
	for _, t := range p.Targets {
		if !removed && t == source {
			removed = true
			continue
		}
		kept = append(kept, t)
	}
	if !removed {
		return false, nil
	}
	p.Targets = kept
	return true, Save(project, p)
}

// AddMember connects m to the plan with id (a SET op keyed on (Source, SlugHint) —
// an existing member is an idempotent no-op). The member's source is also ensured
// to be a target. Returns the updated plan; a missing plan is an error.
func AddMember(project, id string, m Member) (Plan, error) {
	p, ok := Get(project, id)
	if !ok {
		return Plan{}, os.ErrNotExist
	}
	for _, ex := range p.Members {
		if ex == m {
			return p, nil // already a member — idempotent
		}
	}
	p.Members = append(p.Members, m)
	if m.Source != "" && !containsStr(p.Targets, m.Source) {
		p.Targets = append(p.Targets, m.Source)
	}
	return p, Save(project, p)
}

// RemoveMember disconnects m from the plan with id. removed=false (no error, no
// write) when the plan or the member is absent — a graceful no-op.
func RemoveMember(project, id string, m Member) (removed bool, err error) {
	p, ok := Get(project, id)
	if !ok {
		return false, nil
	}
	kept := make([]Member, 0, len(p.Members))
	for _, ex := range p.Members {
		if !removed && ex == m {
			removed = true
			continue
		}
		kept = append(kept, ex)
	}
	if !removed {
		return false, nil
	}
	p.Members = kept
	return true, Save(project, p)
}

// SetStatus sets the plan's lifecycle status (normalized to the known set) and
// persists it. Returns the updated plan; a missing plan is an error.
func SetStatus(project, id, status string) (Plan, error) {
	p, ok := Get(project, id)
	if !ok {
		return Plan{}, os.ErrNotExist
	}
	p.Status = normalizeStatus(status)
	return p, Save(project, p)
}

// MarkReady advances a plan to `ready` (the draft → ready transition the user
// triggers when a plan's targets are chosen and it is spawnable). A no-op-status
// convenience over SetStatus.
func MarkReady(project, id string) (Plan, error) {
	return SetStatus(project, id, StatusReady)
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// --- project-UAT gate (FR3) — derive at read; accept via MarkDone ----------------

// MembersShipped reports whether EVERY member work item of p is shipped, plus the
// labels (source:slug) of any that are not — the project-UAT guard (FR3). It READS
// each member's source repo through the contract reader (contract.LoadProject) and
// matches the member by the plan's correlation id (like the board's spawnedFeature),
// NEVER writing a source's .gogo/. A plan with NO members returns (false, nil) —
// nothing to accept yet. The member-shipped check keys on state.md status (shipped),
// not artifact presence (TEST-004).
func MembersShipped(project string, p Plan) (allShipped bool, unshipped []string) {
	proj, _ := projects.Load(project)
	return MembersShippedIn(project, p, contract.LoadProject(*proj))
}

// MembersShippedIn is the pure core of MembersShipped over an ALREADY-loaded board
// repo (so the plans tab can pass its in-memory m.repo without a re-read). It matches
// each member in repo by (Source, plan correlation id) — SCOPED to project — and checks
// Shipped(). On a workspace-spanning repo (the unified board's contract.LoadWorkspace) a
// source NAME can collide across projects, so project is threaded in and a feature whose
// Project is set and differs is skipped (mirroring tui.spawnedFeature, REV-002). A
// project-less feature (the single-project LoadProject / single-repo seam) stays inert,
// so those callers behave byte-for-byte; project "" disables the scope guard entirely.
func MembersShippedIn(project string, p Plan, repo *contract.Repo) (allShipped bool, unshipped []string) {
	if len(p.Members) == 0 {
		return false, nil
	}
	for _, m := range p.Members {
		if f := memberFeature(repo, project, m.Source, p.ID); f == nil || !f.Shipped() {
			unshipped = append(unshipped, m.Source+":"+m.SlugHint)
		}
	}
	return len(unshipped) == 0, unshipped
}

// memberFeature finds the work item a plan spawned into source: a feature tagged with
// that source whose state.md correlation list carries the plan id, or nil. The
// cross-project scope guard (REV-002) skips a feature whose Project is set and differs
// from the plan's project — on the workspace-spanning repo a same-named source in ANOTHER
// project must not match. A feature with no Project (single-project/single-repo seam) or
// an empty project arg leaves the match keyed on (Source, correlation) alone.
func memberFeature(repo *contract.Repo, project, source, planID string) *contract.Feature {
	if repo == nil {
		return nil
	}
	for _, f := range repo.Features {
		if f == nil || f.Source != source {
			continue
		}
		if project != "" && f.Project != "" && f.Project != project {
			continue
		}
		for _, c := range f.Correlations {
			if c == planID {
				return f
			}
		}
	}
	return nil
}

// DerivedStatus returns the DISPLAY status of p given whether all its members are
// shipped (FR3, derive-at-read): `awaiting-project-uat` when p is `active` with ≥1
// member and all shipped, else the persisted status. Display-only — nothing new is
// persisted until MarkDone. Pure (the shipped decision is passed in, computed by
// MembersShipped/MembersShippedIn) so it stays unit-testable without disk.
func DerivedStatus(p Plan, allShipped bool) string {
	if p.Status == StatusActive && len(p.Members) > 0 && allShipped {
		return StatusAwaitingProjectUAT
	}
	return p.Status
}

// MarkDone records the project-UAT acceptance on a CLI-owned plan (FR3, accept-only
// v1): it appends a `## Project UAT` round to the plan body and flips the persisted
// status to `done`. It does NOT itself re-check members-shipped — the caller
// (`gogo plan done` / plans-tab `D`) guards with MembersShipped first. Writes ONLY the
// plan file under ~/.gogo/ (never a source's .gogo/). A missing plan is an error.
func MarkDone(project, id string) (Plan, error) {
	p, ok := Get(project, id)
	if !ok {
		return Plan{}, os.ErrNotExist
	}
	round := strings.Count(p.Description, "## UAT round") + 1
	stamp := time.Now().UTC().Format("2006-01-02")
	block := fmt.Sprintf("## Project UAT\n## UAT round %d - accepted (user, %s) - via gogo plan done", round, stamp)
	body := strings.TrimRight(p.Description, "\n")
	if body != "" {
		body += "\n\n"
	}
	p.Description = body + block
	p.Status = StatusDone
	return p, Save(project, p)
}
