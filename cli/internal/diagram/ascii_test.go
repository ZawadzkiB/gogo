package diagram

import (
	"errors"
	"strings"
	"testing"
)

const flowSrc = `flowchart TD
  U["user terminal: gogo"]:::proc --> RD["contract reader (Go)\n.gogo/work"]:::art
  RD --> BRD["board TUI"]:::gate
  BRD -->|"enter"| FILES["item file list"]:::proc
  BRD -->|"move plan→implement"| GO["launch claude"]:::proc
  classDef proc fill:#e8ecff
  classDef art fill:#fff3d6`

// A flowchart renders as Unicode box-drawing (mermaid-ascii): boxes + arrowheads,
// node labels present, and styling directives (classDef) must NOT leak as nodes.
func TestRenderFlowchartUnicode(t *testing.T) {
	out, err := Render(flowSrc, 100)
	if err != nil {
		t.Fatalf("flowchart should render, got err=%v", err)
	}
	// Unicode box-drawing characters present.
	if !strings.ContainsAny(out, "┌└│─") {
		t.Errorf("no Unicode box characters in output:\n%s", out)
	}
	// An arrowhead (down or right) is drawn.
	if !strings.ContainsAny(out, "►▶▼◄") {
		t.Errorf("no arrowheads rendered:\n%s", out)
	}
	// Node labels present (\n flattened into the box).
	for _, want := range []string{"user terminal: gogo", "contract reader", "board TUI"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing node label %q in:\n%s", want, out)
		}
	}
	// classDef styling must not leak in as a node.
	if strings.Contains(out, "fill:#") || strings.Contains(out, "classDef") {
		t.Errorf("classDef styling leaked into render:\n%s", out)
	}
}

func TestRenderGraphLR(t *testing.T) {
	out, err := Render("graph LR\n  A[Start] --> B[End]", 80)
	if err != nil {
		t.Fatalf("graph LR should render: %v", err)
	}
	if !strings.Contains(out, "Start") || !strings.Contains(out, "End") {
		t.Errorf("graph LR render wrong:\n%s", out)
	}
	if !strings.ContainsAny(out, "┌└│─") {
		t.Errorf("graph LR not drawn with box chars:\n%s", out)
	}
}

// A sequence diagram now renders for real (participants + arrows), not the
// labeled-source fallback.
func TestRenderSequence(t *testing.T) {
	src := "sequenceDiagram\n  participant A as Alice\n  participant B as Bob\n  A->>B: Hello\n  B-->>A: Hi"
	out, err := Render(src, 80)
	if err != nil {
		t.Fatalf("sequence should render, got err=%v", err)
	}
	for _, want := range []string{"Alice", "Bob", "Hello", "Hi"} {
		if !strings.Contains(out, want) {
			t.Errorf("sequence missing %q:\n%s", want, out)
		}
	}
	if !strings.ContainsAny(out, "►▶◄") {
		t.Errorf("sequence arrows not drawn:\n%s", out)
	}
}

// Real-world sequences use `actor` and `Note over` — pkg/sequence can't parse
// those, so the facade sanitizes them; the interaction must still render.
func TestRenderSequenceActorAndNote(t *testing.T) {
	src := "sequenceDiagram\n  actor U as user\n  participant B as board\n" +
		"  U->>B: click\n  Note over U,B: this note is dropped\n  B-->>U: ok"
	out, err := Render(src, 80)
	if err != nil {
		t.Fatalf("actor/Note sequence should still render, got err=%v", err)
	}
	if !strings.Contains(out, "user") || !strings.Contains(out, "board") {
		t.Errorf("actor/participant lost:\n%s", out)
	}
	if !strings.Contains(out, "click") {
		t.Errorf("message lost:\n%s", out)
	}
}

// class/state/er keep the source fallback (ErrUnsupported).
func TestClassStateFallBack(t *testing.T) {
	for _, src := range []string{
		"classDiagram\n  class Animal",
		"stateDiagram-v2\n  [*] --> Idle",
		"erDiagram\n  A ||--o{ B : has",
	} {
		if _, err := Render(src, 80); !errors.Is(err, ErrUnsupported) {
			t.Errorf("%q should be ErrUnsupported, got %v", firstMeaningfulLine(src), err)
		}
	}
}

// Empty, comment-only, and pure-garbage inputs fall back without panicking.
func TestEmptyAndGarbage(t *testing.T) {
	if _, err := Render("", 80); err == nil {
		t.Errorf("empty must fall back")
	}
	if _, err := Render("flowchart TD\n%% only a comment", 80); err == nil {
		t.Errorf("flowchart with no nodes must fall back")
	}
	// A flowchart header with garbage body must not panic.
	if _, err := Render("flowchart TD\n@@@ (((  ]]] {{{ -->>", 80); err == nil {
		t.Logf("garbage flow rendered (acceptable); ensuring no panic")
	}
	// A non-mermaid first word never reaches a renderer.
	if _, err := Render("@@@ not a diagram", 80); !errors.Is(err, ErrUnsupported) {
		t.Errorf("garbage should be ErrUnsupported, got %v", err)
	}
}

func TestKind(t *testing.T) {
	cases := map[string]string{
		"flowchart TD\n A-->B":       "flowchart",
		"graph LR\n A-->B":           "flowchart",
		"sequenceDiagram\n A->>B: x": "sequence",
		"classDiagram\n class X":     "class",
		"stateDiagram-v2\n [*]-->A":  "state",
		"":                           "",
	}
	for src, want := range cases {
		if got := Kind(src); got != want {
			t.Errorf("Kind(%q) = %q, want %q", src, got, want)
		}
	}
}

func TestIsFlowchart(t *testing.T) {
	if !IsFlowchart("flowchart TD\n A-->B") {
		t.Error("flowchart not detected")
	}
	if IsFlowchart("sequenceDiagram\n A->>B: x") {
		t.Error("sequence wrongly detected as flowchart")
	}
}
