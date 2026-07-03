# Decisions — feature `cli-distribution`

## D1 — Release tooling

**Question:** what builds and publishes the artifacts on a tag push?
- **A (recommended):** **goreleaser** in a GitHub Actions workflow — standard tool:
  cross-compile matrix, ldflags version stamping, archives + raw binaries +
  `checksums.txt`, release upload, and a free brew-tap formula later.
- **B:** hand-rolled Actions matrix (`go build` × 4 + `gh release upload`) —
  fewer deps, but re-implements what goreleaser standardizes (checksums,
  archive naming, stamping) and gives no brew path.

**Recommendation:** A.

**RESOLVED (2026-07-03):** **custom — the otp-cli pattern.** No goreleaser, no
checksums, no stamping, no install.sh: a minimal tag-push workflow does the four
`go build`s and attaches RAW binaries as release assets; the README carries curl
one-liners (uname-selected asset). Automation kept only because Go binaries are
per-OS/arch.

## D2 — Which release train

**Question:** ship this in the pending v0.10.0 commit, or as a later 0.11.0?
- **A (recommended):** ride the **v0.10.0 train** — pure repo infrastructure (no
  plugin behavior change, no version bump), and the very tag that releases the
  CLI publishes its first installable binaries.
- **B:** separate 0.11.0 feature after 0.10.0 ships bare.

**Recommendation:** A.

**RESOLVED (2026-07-03):** **A** (ride the v0.10.0 train).

## Non-forks (recorded)

- **Platforms:** darwin/arm64 · darwin/amd64 · linux/amd64 · linux/arm64.
  Windows deferred (tmux/claude launch paths are unix-flavored; revisit on ask).
- **Install UX:** README curl one-liners only (sudo `/usr/local/bin` and
  no-sudo `~/bin` variants; asset picked via `uname`); `releases/latest/download`
  default + pinned-tag variant.
- **`go install` naming caveat** stays documented (no module rename churn).
- The first live workflow run happens on the v0.10.0 tag push (no local runner);
  snapshot builds verify everything verifiable locally first.
