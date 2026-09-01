package render

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/sequence"
)

const activationFixture = `sequenceDiagram
participant A
participant B
activate B
activate B
A ->> B: inner
deactivate B
B -->> A: outer
deactivate B`

func TestSequenceActivationBarsAndEndpoints(t *testing.T) {
	diagram, err := sequence.Parse(activationFixture, sequence.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	output := mustRenderSequence(t, diagram, Options{MaxWidth: 100, MaxHeight: 30})
	grid := newSequenceGrid(t, output)
	b := sequenceHeaderBox(t, grid, "B")
	innerY := sequenceArrowRow(t, grid, "inner")
	outerY := sequenceArrowRow(t, grid, "outer")
	if grid.at(b.center()+1, innerY) != "▶" {
		t.Fatalf("nested activation endpoint missing at (%d,%d):\n%s", b.center()+1, innerY, output)
	}
	if grid.at(b.center(), outerY) != "┘" {
		t.Fatalf("outer activation did not return to base bar:\n%s", output)
	}
	if !strings.Contains(output, "│") {
		t.Fatalf("solid activation bar missing:\n%s", output)
	}
}

func TestSequenceActivationInsideFragmentAndASCII(t *testing.T) {
	source := `sequenceDiagram
participant A
participant B
alt yes
activate B
A ->> B: call
deactivate B
else no
B -->> A: skip
end`
	diagram, err := sequence.Parse(source, sequence.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	output := mustRenderSequence(t, diagram, Options{ASCII: true, MaxWidth: 100, MaxHeight: 40})
	if strings.ContainsAny(output, "┌┐└┘├┤─│┄┊┼▶◀") || !strings.Contains(output, "|") || !strings.Contains(output, ":") {
		t.Fatalf("ASCII activation output invalid:\n%s", output)
	}
}

func TestSequenceActiveSelfMessageUsesTightBounds(t *testing.T) {
	source := `sequenceDiagram
participant A
participant B
activate B
B ->> B: active self
deactivate B`
	diagram, err := sequence.Parse(source, sequence.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	output := mustRenderSequence(t, diagram, Options{MaxWidth: 100, MaxHeight: 30})
	width := sequenceOutputWidth(t, output)
	height := len(strings.Split(output, "\n"))
	if _, err := Sequence(diagram, Options{MaxWidth: width, MaxHeight: height}); err != nil {
		t.Fatalf("tight bounds rejected: %v", err)
	}
	if _, err := Sequence(diagram, Options{MaxWidth: width - 1, MaxHeight: height}); !errors.Is(err, ErrOutputBounds) {
		t.Fatalf("short width error=%v", err)
	}
}

func TestSequenceUnrelatedDeepLastActivationUsesExactWidthPreflight(t *testing.T) {
	diagram := &sequence.Diagram{}
	for participant := 0; participant < 16; participant++ {
		diagram.Participants = append(diagram.Participants, sequence.Participant{ID: fmt.Sprintf("P%d", participant), Label: fmt.Sprintf("P%d", participant)})
	}
	for depth := 0; depth < 8; depth++ {
		diagram.Steps = append(diagram.Steps, sequence.Step{Kind: sequence.ActivateStep, Participant: 15})
	}
	diagram.Steps = append(diagram.Steps, sequence.Step{Kind: sequence.MessageStep, Message: sequence.Message{From: 0, To: 1, Label: "unrelated", Kind: sequence.Request}})
	for depth := 0; depth < 8; depth++ {
		diagram.Steps = append(diagram.Steps, sequence.Step{Kind: sequence.DeactivateStep, Participant: 15})
	}
	output := mustRenderSequence(t, diagram, Options{MaxWidth: 512, MaxHeight: 30})
	width := sequenceOutputWidth(t, output)
	height := len(strings.Split(output, "\n"))
	if _, err := Sequence(diagram, Options{MaxWidth: width, MaxHeight: height}); err != nil {
		t.Fatalf("exact activation width rejected: %v", err)
	}
	if _, err := Sequence(diagram, Options{MaxWidth: width - 1, MaxHeight: height}); !errors.Is(err, ErrOutputBounds) {
		t.Fatalf("one-cell-short activation width error=%v", err)
	}
}
