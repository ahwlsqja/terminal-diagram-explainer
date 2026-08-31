package sequence_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/sequence"
)

func TestActivationParserBuildsParticipantLocalLIFO(t *testing.T) {
	source := `sequenceDiagram
participant A
participant B
activate A
activate A
activate B
A ->> B: nested
deactivate A
B -->> A: outer
deactivate A
deactivate B`
	diagram, err := sequence.Parse(source, sequence.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if diagram.Messages != nil || len(diagram.Steps) != 8 {
		t.Fatalf("activation timeline=%#v", diagram)
	}
	wantKinds := []sequence.StepKind{
		sequence.ActivateStep, sequence.ActivateStep, sequence.ActivateStep,
		sequence.MessageStep, sequence.DeactivateStep, sequence.MessageStep,
		sequence.DeactivateStep, sequence.DeactivateStep,
	}
	wantParticipants := []int{0, 0, 1, 0, 0, 0, 0, 1}
	for index, kind := range wantKinds {
		if diagram.Steps[index].Kind != kind || diagram.Steps[index].Participant != wantParticipants[index] {
			t.Fatalf("step %d=%+v", index, diagram.Steps[index])
		}
	}
}

func TestActivationKeywordsRemainMessageParticipantIDs(t *testing.T) {
	source := `sequenceDiagram
participant activate
participant deactivate
participant A
activate ->> A: request
deactivate -->> A: response`
	diagram, err := sequence.Parse(source, sequence.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if diagram.Steps != nil || len(diagram.Messages) != 2 {
		t.Fatalf("message precedence drifted: %#v", diagram)
	}
}

func TestActivationParserRejectsInvalidState(t *testing.T) {
	tests := []struct {
		name string
		body string
		line int
	}{
		{name: "unmatched", body: "deactivate A\nA ->> A: x", line: 3},
		{name: "zero message", body: "activate A\ndeactivate A\nA ->> A: x", line: 4},
		{name: "unclosed", body: "activate A\nA ->> A: x", line: 3},
		{name: "cross fragment open", body: "activate A\nloop x\nA ->> A: x\nend\ndeactivate A", line: 4},
		{name: "cross alt branch", body: "alt yes\nactivate A\nA ->> A: x\nelse no\nA ->> A: y\nend", line: 6},
		{name: "cross fragment end", body: "loop x\nactivate A\nA ->> A: x\nend\ndeactivate A", line: 6},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagram, err := sequence.Parse("sequenceDiagram\nparticipant A\n"+test.body, sequence.DefaultLimits())
			if err == nil || diagram != nil {
				t.Fatalf("Parse()=%#v %v", diagram, err)
			}
			var parseErr *sequence.ParseError
			if !errors.As(err, &parseErr) || parseErr.Line != test.line {
				t.Fatalf("error=%v want line=%d", err, test.line)
			}
		})
	}
}

func TestActivationParserLimits(t *testing.T) {
	limits := sequence.DefaultLimits()
	if limits.MaxActivations != 96 || limits.MaxActivationDepth != 8 {
		t.Fatalf("activation limits=%+v", limits)
	}
	var depth8 string
	for index := 0; index < 8; index++ {
		depth8 += "activate A\n"
	}
	depth8 += "A ->> A: x\n"
	for index := 0; index < 8; index++ {
		depth8 += "deactivate A\n"
	}
	if _, err := sequence.Parse("sequenceDiagram\nparticipant A\n"+depth8, limits); err != nil {
		t.Fatalf("depth 8 rejected: %v", err)
	}
	depth9 := "activate A\n" + depth8
	if diagram, err := sequence.Parse("sequenceDiagram\nparticipant A\n"+depth9, limits); err == nil || diagram != nil {
		t.Fatalf("depth 9 accepted: %#v %v", diagram, err)
	}

	activationSource := func(count int) string {
		var source strings.Builder
		source.WriteString("sequenceDiagram\n")
		participantCount := (count + 7) / 8
		for participant := 0; participant < participantCount; participant++ {
			fmt.Fprintf(&source, "participant P%d\n", participant)
		}
		for activation := 0; activation < count; activation++ {
			fmt.Fprintf(&source, "activate P%d\n", activation/8)
		}
		source.WriteString("P0 ->> P0: x\n")
		for activation := count - 1; activation >= 0; activation-- {
			fmt.Fprintf(&source, "deactivate P%d\n", activation/8)
		}
		return source.String()
	}
	if _, err := sequence.Parse(activationSource(limits.MaxActivations), limits); err != nil {
		t.Fatalf("%d activations rejected: %v", limits.MaxActivations, err)
	}
	if diagram, err := sequence.Parse(activationSource(limits.MaxActivations+1), limits); err == nil || diagram != nil {
		t.Fatalf("%d activations accepted: %#v %v", limits.MaxActivations+1, diagram, err)
	}
}
