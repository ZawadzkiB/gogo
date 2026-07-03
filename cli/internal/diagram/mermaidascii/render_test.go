package mermaidascii

import (
	"strings"
	"testing"
)

// A flowchart renders as Unicode box-drawing by default (useAscii=false).
func TestRenderUnicodeFlowchart(t *testing.T) {
	out, err := Render("flowchart LR\n  A[Start] --> B[End]", false)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.ContainsAny(out, "┌└│─") {
		t.Errorf("no Unicode box-drawing:\n%s", out)
	}
	if !strings.Contains(out, "Start") || !strings.Contains(out, "End") {
		t.Errorf("node labels lost:\n%s", out)
	}
}

// The ASCII option swaps the charset (no Unicode box-drawing).
func TestRenderASCIIOption(t *testing.T) {
	out, err := Render("flowchart LR\n  A[Start] --> B[End]", true)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if strings.ContainsAny(out, "┌└│─►▼") {
		t.Errorf("ASCII mode should not emit Unicode box chars:\n%s", out)
	}
	if !strings.Contains(out, "Start") {
		t.Errorf("node label lost in ASCII mode:\n%s", out)
	}
}

// Garbage input must return an error (recovered), never panic.
func TestRenderNeverPanics(t *testing.T) {
	if _, err := Render("flowchart TD\n@@@ ((( ]]] {{{ -->>", false); err != nil {
		t.Logf("garbage returned err (acceptable): %v", err)
	}
	// The point of the test: reaching here means no panic escaped Render.
}
