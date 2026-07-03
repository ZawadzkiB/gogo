package mermaidascii

import "fmt"

// Render draws a mermaid flowchart/graph source as text. useAscii selects the
// plain-ASCII charset; false (the default the caller passes) yields Unicode
// box-drawing. It mirrors upstream GraphDiagram.Parse+Render (cmd/diagram.go)
// minus the gin-tainted cmd package.
//
// It NEVER panics: any panic inside the best-effort layout algorithm is
// recovered and returned as an error so the caller can fall back to the
// labeled source.
func Render(input string, useAscii bool) (out string, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = "", fmt.Errorf("mermaid-ascii render panic: %v", r)
		}
	}()

	properties, err := mermaidFileToMap(input, "cli")
	if err != nil {
		return "", err
	}
	properties.boxBorderPadding = boxBorderPadding
	properties.paddingX = paddingBetweenX
	properties.paddingY = paddingBetweenY
	properties.styleType = "cli"
	properties.useAscii = useAscii
	return drawMap(properties), nil
}
