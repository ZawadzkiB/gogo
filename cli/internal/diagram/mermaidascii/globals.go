package mermaidascii

// These package-level defaults lived on the cobra root command upstream
// (cmd/root.go, which we do NOT vendor). mermaidFileToMap reads them as the
// initial graph properties; Render overrides padding/ascii per call.
var (
	boxBorderPadding = 1
	paddingBetweenX  = 5
	paddingBetweenY  = 5

	// Coords toggles upstream coordinate-debug overlays; always off here.
	Coords = false
)
