# cli-distribution — shipped 2026-07-03

**`gogo` is now installable with a single curl command — no Go toolchain required.** Every `v*` tag push triggers the repo's first GitHub Actions workflow: a `go test -race` gate, then four plain cross-compiles (darwin/linux × arm64/amd64, CGO off) that attach **raw binaries** (`gogo-darwin-arm64`, …) straight to the GitHub Release. The README leads with two copy-paste one-liners — no-sudo `~/bin` (with a PATH reminder) and sudo `/usr/local/bin` — whose `uname` substitutions pick the right asset on every supported platform. Deliberately minimal by decision: no goreleaser, no checksums, no archives, no version stamping, no installer script.

## Decisions

- **Release tooling** — custom **otp-cli pattern** (raw assets + README curl), rejecting the recommended goreleaser design at the plan gate; automation kept only for the cross-compile Go forces.
- **Release train** — rides **v0.10.0**: the very tag that releases the CLI publishes its first installable binaries.

## Review & test

Review **APPROVE** (2 low findings — `-race` in the test gate, PATH note for `~/bin` — fixed and verified inline); the workflow's exact build loop ran green locally (4 binaries, `file`-verified architectures, `uname` derivations correct), with the one untestable-before-live step — the Actions run itself — flagged for the v0.10.0 tag push.

## Changed

| Change | File |
|---|---|
| Release workflow: test gate → 4 builds → raw asset publish, create-or-upload recovery | `.github/workflows/release.yml` |
| Curl install one-liners + asset table + pinned-version note | `README.md` ("The gogo CLI") |

## Follow-ups

- Watch the first live Actions run on the v0.10.0 tag push; fix-forward if the runner surprises.
- On demand later: Homebrew tap, Windows assets, checksums.

---

Full audit trail: [.gogo/work/feature-cli-distribution/](../../work/feature-cli-distribution/) (plan, decisions, review/test rounds, as-built report).
