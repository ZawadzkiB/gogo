// Package mermaidascii is a vendored, trimmed copy of the flowchart/graph
// render path from github.com/AlexanderGrooff/mermaid-ascii (MIT, tag 1.3.0,
// commit aca241698ec7, 2026-06).
//
// # Why vendored (and not imported)
//
// Upstream keeps its SEQUENCE renderer in the clean pkg/sequence package, which
// gogo imports directly (verified: importing pkg/sequence + pkg/diagram links
// zero gin/sonic/cobra into the binary). The GRAPH renderer, however, lives in
// upstream's `cmd` package alongside cmd/web.go, which imports gin-gonic. Go
// links at package granularity, so importing the graph renderer would pull
// gin + bytedance/sonic (asm) into a binary that must start in milliseconds —
// the exact reason implement round 3 dropped the library. The graph API is also
// effectively private (unexported mermaidFileToMap/drawMap in package cmd).
// Vendoring the graph path is therefore the sanctioned fallback.
//
// # Changes from upstream
//
//   - package cmd -> package mermaidascii
//   - the cobra/gin surface (root.go, web.go, main.go, cmd/diagram.go factory)
//     is NOT vendored — only the graph render files.
//   - logrus is replaced by a no-op logger (log.go) — the render path only
//     logged Debug/Warn.
//   - gookit/color is dropped: wrapTextInColor emits plain text (no ANSI) so
//     diagrams stay clean inside terminal viewports and markdown code blocks.
//   - the render entry point (Render) is a small local re-creation of upstream
//     GraphDiagram.Parse+Render (entry.go).
//
// Remaining external deps of this package: github.com/elliotchance/orderedmap/v2
// and github.com/mattn/go-runewidth (both already in the gogo module graph,
// gin-free, pure Go). Upstream MIT license: ./LICENSE.
//
// Sequence diagrams are NOT handled here — the caller (internal/diagram) uses
// upstream pkg/sequence directly.
package mermaidascii
