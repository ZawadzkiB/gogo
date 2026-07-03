package tui

import (
	"strings"
	"testing"
)

const flowFence = "# Report\n\n" +
	"```mermaid\n" +
	"flowchart TD\n" +
	"  A[\"start\"] --> B[\"finish\"]\n" +
	"```\n\n" +
	"after\n"

const seqFence = "intro\n\n" +
	"```mermaid\n" +
	"sequenceDiagram\n" +
	"  participant U\n" +
	"  U->>S: go\n" +
	"```\n"

// An unrenderable flowchart (header + only comments, no nodes) exercises the
// labeled-source fallback; garbageFence is pure noise that must not panic.
const malformedFence = "```mermaid\n" +
	"flowchart TD\n" +
	"%% only comments here — no drawable nodes\n" +
	"```\n"

const garbageFence = "```mermaid\n" +
	"@@@ not a diagram at all ((( ]]] {{{\n" +
	"```\n"

// class kinds keep the labeled source + "press w" hint.
const classFence = "```mermaid\n" +
	"classDiagram\n" +
	"  class Animal { +int age }\n" +
	"```\n"

const testWidth = 100

// TEST-005 / TEST-008: a flowchart fence becomes Unicode box-drawing and the
// raw ```mermaid marker is gone; the surrounding prose survives.
func TestPreprocessFlowchartToUnicode(t *testing.T) {
	out := preprocessMermaid(flowFence, testWidth)
	if strings.Contains(out, "```mermaid") {
		t.Errorf("raw ```mermaid fence survived:\n%s", out)
	}
	if !strings.ContainsAny(out, "┌└│─") {
		t.Errorf("no Unicode box-drawing render present:\n%s", out)
	}
	if !strings.ContainsAny(out, "►▶▼◄") {
		t.Errorf("no arrowhead rendered:\n%s", out)
	}
	if !strings.Contains(out, "start") || !strings.Contains(out, "finish") {
		t.Errorf("node labels lost:\n%s", out)
	}
	if !strings.Contains(out, "after") {
		t.Errorf("prose after the fence was dropped:\n%s", out)
	}
}

// TEST-008: a sequence fence now RENDERS (participants + arrows) instead of
// degrading to the labeled source.
func TestPreprocessSequenceRenders(t *testing.T) {
	out := preprocessMermaid(seqFence, testWidth)
	if strings.Contains(out, "```mermaid") {
		t.Errorf("raw ```mermaid fence survived:\n%s", out)
	}
	if strings.Contains(out, "press w for the browser view") {
		t.Errorf("sequence fell back to labeled source instead of rendering:\n%s", out)
	}
	if !strings.ContainsAny(out, "┌└│─") || !strings.ContainsAny(out, "►▶◄") {
		t.Errorf("sequence not drawn as a diagram:\n%s", out)
	}
	if !strings.Contains(out, "go") || !strings.Contains(out, "U") || !strings.Contains(out, "S") {
		t.Errorf("sequence participants/message lost:\n%s", out)
	}
	// The surrounding prose survives.
	if !strings.Contains(out, "intro") {
		t.Errorf("prose before the fence dropped:\n%s", out)
	}
}

// TEST-005: a class diagram keeps a labeled source block pointing at `w`.
func TestPreprocessClassKeepsLabeledSource(t *testing.T) {
	out := preprocessMermaid(classFence, testWidth)
	if strings.Contains(out, "```mermaid") {
		t.Errorf("raw ```mermaid fence survived:\n%s", out)
	}
	if !strings.Contains(out, "[mermaid class — press w for the browser view]") {
		t.Errorf("missing class label:\n%s", out)
	}
	if !strings.Contains(out, "class Animal") {
		t.Errorf("class source not preserved:\n%s", out)
	}
}

// TEST-005: an unrenderable diagram degrades to a labeled source (never a raw
// fence), and pure garbage must not panic.
func TestPreprocessMalformedFallsBackToSource(t *testing.T) {
	out := preprocessMermaid(malformedFence, testWidth)
	if strings.Contains(out, "```mermaid") {
		t.Errorf("raw ```mermaid fence survived:\n%s", out)
	}
	if !strings.Contains(out, "[mermaid") {
		t.Errorf("unrenderable fence lost its label:\n%s", out)
	}
	if !strings.Contains(out, "only comments here") {
		t.Errorf("source not preserved:\n%s", out)
	}

	// Pure garbage: no panic, no raw fence, source kept.
	g := preprocessMermaid(garbageFence, testWidth)
	if strings.Contains(g, "```mermaid") {
		t.Errorf("garbage kept a raw fence:\n%s", g)
	}
	if !strings.Contains(g, "not a diagram at all") {
		t.Errorf("garbage source dropped:\n%s", g)
	}
}

// Text without any mermaid fences passes through unchanged.
func TestPreprocessNoFencePassthrough(t *testing.T) {
	in := "# Title\n\nsome text\n\n```go\nfmt.Println()\n```\n"
	if got := preprocessMermaid(in, testWidth); got != in {
		t.Errorf("non-mermaid markdown changed:\n%q\n->\n%q", in, got)
	}
}
