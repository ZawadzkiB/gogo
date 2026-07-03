# Report — feature `cli-distribution`

- **feature:** CLI distribution — raw GitHub Release binaries on tag push + otp-cli-style curl install
- **status:** done
- **completed:** 2026-07-03
- **branch / commits:** `main` · rides the pending v0.10.0 train (uncommitted)

## Run status / gaps

**All phases ran; zero open issues.** A deliberately small feature: plan (2 rounds — the user redirected round 1's goreleaser design to the house otp-cli pattern), inline implement, 1 review round (**APPROVE**, 2 low findings fixed inline + verified), test folded into implement verification (the workflow's exact build loop executed locally). The **one untestable-before-live step** — the Actions run itself — is flagged: its first execution is the v0.10.0 tag push.

## Summary

**`gogo` becomes installable with one curl command, no Go toolchain.** Every `v*` tag push now triggers the repo's **first GitHub Actions workflow**: a `go test -race` gate, then four plain cross-compiles (darwin/linux × arm64/amd64, CGO off), attaching **raw binaries** (`gogo-darwin-arm64`, …) straight to the GitHub Release — the **otp-cli pattern** by explicit decision: *no goreleaser, no checksums, no archives, no stamping, no installer script*. The README leads with two copy-paste one-liners (no-sudo `~/bin` + PATH note, and sudo `/usr/local/bin`) whose `uname` substitutions select the right asset on every supported platform.

## Planned vs shipped

Shipped as the round-2 plan (the round-1 goreleaser design was **rejected by the user at the gate** — recorded in [adjustments.md](../adjustments.md)). Review added two refinements: the gate runs **`-race`** (coding-rules non-negotiable) and the `~/bin` variant gained a PATH reminder.

| Change | File |
|---|---|
| Release workflow (test gate → 4 builds → raw asset publish, create-or-upload recovery) | `.github/workflows/release.yml` |
| Curl install one-liners + asset table + pinned-version note | `README.md` ("The gogo CLI") |

## Decisions

| D | Choice |
|---|---|
| **D1 tooling** | **custom — the otp-cli pattern** (raw assets + README curl; automation only for the cross-compile Go forces) |
| **D2 train** | **A** — rides v0.10.0; the tag push mints the first installable release |

## Review & test

**Review: APPROVE** — REV-001 (`-race` in the gate) + REV-002 (PATH note), both fixed + verified; the reviewer separately confirmed the four live-risk questions: no silent half-publish path (`gh release create … || upload --clobber` converges on re-run), `--verify-tag` passes annotated tags, `GITHUB_REF_NAME` is the bare tag, `gh` is preinstalled+authenticated on ubuntu-latest. **Test:** the workflow's exact build loop ran locally — 4 binaries, correct architectures (`file`-verified), host binary runs, `uname` derivations correct for all four assets, YAML parses. See [review-01.md](../review-01.md) · [review/issues.json](../review/issues.json).

## Diagrams

[flow.mmd](flow.mmd) — the shipped release flow; before/ = the go-build-only baseline.

## Knowledge updates

None needed — repo infrastructure only; the README (proxy upstream) was updated directly as the product change itself.

## Follow-ups

- **Watch the first live run** on the v0.10.0 tag push (the one thing no local test covers); fix-forward if the runner surprises.
- Later, on demand: Homebrew tap, Windows assets, checksums if distribution ever needs hardening.

## Summary (TL;DR)

- **What:** tag push → raw `gogo` binaries on the GitHub Release + README curl one-liners.
- **Why:** install anywhere with curl, zero toolchain, zero ceremony — the house pattern.
- **Verdicts:** review APPROVE (2 lows fixed); local build-loop test green; first live Actions run = the v0.10.0 tag.
- **Next:** commit + tag v0.10.0 — the release train publishes the first artifacts.
