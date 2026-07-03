# Review — feature `cli-distribution` · round 1

Reviewer: gogo-reviewer (fresh eyes) · phase ③ · 2026-07-03
Scope: `.github/workflows/release.yml` (NEW — the repo's first workflow) +
README.md "The gogo CLI" install section. Reviewed against the accepted
`plan.md` (round 2 — otp-cli RAW-asset pattern: no goreleaser / checksums /
stamping / install.sh, by explicit user decision) and `decisions.md`.

The workflow cannot be executed before the real tag push, so it was reviewed
line-by-line and by reasoning about the runner semantics.

## Verdict: APPROVE

No open blockers or majors. Two low-severity findings (1 minor, 1 nit), both
AGENT-FIXABLE, neither blocking the advance to ④ test.

## Findings

| id | sev | prio | status | title |
|---|---|---|---|---|
| REV-001 | minor | P3 | new | CI release gate runs `go test ./...` without `-race` |
| REV-002 | nit | P3 | new | No-sudo curl one-liner installs to `~/bin` with no PATH reminder |

### REV-001 — CI release gate runs `go test ./...` without `-race` (minor · AGENT-FIXABLE)
The `test` step in `release.yml` is the ONLY automated gate before the binaries
publish, yet it omits `-race`. `coding-rules.md` calls `go test -race ./...`
non-negotiable for cli/ changes, and the CLI is concurrent (bubbletea
value-copy Model + fsnotify — the TEST-001 bug class). Plan FR1 does literally
name `go test ./...` as "the one gate", so the code is plan-faithful — this is a
standards tension, not a deviation.
**Fix:** switch to `go test -race ./...` (one gate, strengthened; gcc on
ubuntu-latest satisfies the race detector's CGO need; the build step's
`CGO_ENABLED=0` is separate and unaffected). If the plan's literal wording is
preferred, mark wontfix.

### REV-002 — no PATH reminder for the `~/bin` install (nit · AGENT-FIXABLE)
The no-sudo one-liner drops `gogo` into `~/bin`, which is commonly not on PATH
on a fresh machine → `command not found` right after a successful install. The
sudo variant (`/usr/local/bin`) is fine.
**Fix:** add a one-line PATH note after the no-sudo block.

## What was checked and passed (no findings)

**Workflow correctness (line-by-line).**
- Trigger `tags: ["v*"]` fires on version tags; `permissions: contents: write`
  is exactly the scope `gh release create`/`upload` need via `github.token`.
- `actions/checkout@v4` + `actions/setup-go@v5` pinned majors; `go-version-file:
  cli/go.mod` resolves from repo root (unaffected by step `working-directory`)
  and installs Go 1.25.x per the `go 1.25.0` directive.
- Build loop parameter expansions `${target%/*}` / `${target#*/}` are POSIX and
  correct under the runner's default `bash -eo pipefail`; the four targets map
  to `gogo-darwin-arm64 · gogo-darwin-amd64 · gogo-linux-amd64 · gogo-linux-arm64`,
  matching the README asset table exactly.
- `CGO_ENABLED=0` is a per-command env prefix (correct placement). The module is
  pure Go (`import "C"`: none), so the internal linker is used — which ad-hoc
  signs the cross-compiled `darwin/arm64` binary, so it runs on Apple Silicon.
  The CGO-off choice is not just fine here, it is what keeps darwin/arm64
  runnable when cross-compiled from Linux.
- Path consistency: build runs `working-directory: cli` writing `dist/gogo-*`
  (i.e. `cli/dist/...`); publish runs at repo root referencing `cli/dist/gogo-*`.
  Consistent. `gh` uploads by basename, so asset names are `gogo-<os>-<arch>`.
- `gh` is preinstalled on `ubuntu-latest`; `GH_TOKEN: ${{ github.token }}`
  authenticates it non-interactively.

**Half-published-assets risk — none silent.** `gh release create <tag> <assets>
--verify-tag --generate-notes || gh release upload <tag> <assets> --clobber`:
first push creates + uploads all four; a pre-existing release (hand-made or
re-run) makes `create` fail → the `|| upload --clobber` re-uploads all four; a
partial-upload failure in `create` also trips the `||` recovery. `gh release
create` returns non-zero on any asset failure (never a zero-exit partial), and
if the `upload` fallback itself fails the step exits non-zero → visible red run,
not a silent half-publish. Re-running the job converges.

**`--verify-tag` with annotated tags.** `--verify-tag` only checks the tag ref
exists on the remote (it does on a tag push — it triggered the run); it does not
require annotation or signing, so the repo's annotated-tag convention is fine.

**`GITHUB_REF_NAME`.** On a tag push it holds the bare tag (`v0.10.0`, not
`refs/tags/...`) — exactly what `gh release create/upload` expect.

**dist hygiene.** `.gitignore:31` ignores `cli/dist/`; the runner's dist is
ephemeral and freshly `mkdir -p`'d — no commit risk, no collision.

**README install section.** Both one-liners are copy-paste-safe: `$(uname ...)`
command substitution runs inside the double-quoted URL; the sed map
(`x86_64→amd64`, `aarch64→arm64`, darwin/arm64 pass-through) covers all four
assets; `releases/latest/download/<asset>` and the pinned `download/vX.Y.Z`
forms are correct GitHub URLs; `curl -fsSL` + `&&` chaining fails safe. Curl is
first, source second. No stale claim anywhere that installing the CLI requires
Go (the only Go mentions are the intentional "build from source" alternative and
the plugin-install lines).

**Scope.** Delta is exactly the two files plus the feature folder; no
`plugin.json` bump (correct — infra only, no plugin behavior change). The other
working-tree changes belong to the prior v0.10.0-train features and are out of
scope. `events.jsonl` lines validate against `events.schema.json`.
