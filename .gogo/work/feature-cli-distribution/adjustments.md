# Adjustments — feature `cli-distribution`

Running log of user-requested changes / clarifications during planning.

## 2026-07-03 — origin (from the request)

> "we need also thing how we want to deliver this cli, maybe as an artifact in
> github, so users can cUrl it into bin and use from anywhere?"

Exactly the report's queued follow-up (goreleaser/brew). Scoped: GitHub Release
artifacts per version tag + a curl-able install script + the repo's first CI.

## 2026-07-03 — D1 redirected at the gate (user)

> "I do not want any checksums, stamping or anything, just release file as
> artifact in gh and add cUrl command to install it like we did in
> https://github.com/ZawadzkiB/otp-cli"

otp-cli pattern verified: raw binaries attached to the GitHub release + plain
curl one-liners in the README (sudo /usr/local/bin and ~/bin variants). Plan
slimmed accordingly: DROPPED goreleaser, checksums.txt, archives, install.sh,
ldflags stamping, and the separate ci.yml. KEPT: a minimal tag-push workflow
(Go must be cross-compiled per OS/arch — 4 plain `go build`s on one runner +
asset upload, gated by `go test`), README curl one-liners (uname-based so one
line works on every platform + a per-platform table).
