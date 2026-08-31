package render

import (
	"fmt"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/sequence"
)

func TestSequenceActivationDirectValidation(t *testing.T) {
	participant := []sequence.Participant{{ID: "A", Label: "A"}}
	message := sequence.Message{From: 0, To: 0, Label: "x", Kind: sequence.Request}
	tests := map[string][]sequence.Step{
		"negative participant":     {{Kind: sequence.ActivateStep, Participant: -1}, {Kind: sequence.MessageStep, Message: message}, {Kind: sequence.DeactivateStep, Participant: -1}},
		"out of range participant": {{Kind: sequence.ActivateStep, Participant: 1}, {Kind: sequence.MessageStep, Message: message}, {Kind: sequence.DeactivateStep, Participant: 1}},
		"unmatched deactivate":     {{Kind: sequence.DeactivateStep}, {Kind: sequence.MessageStep, Message: message}},
		"zero interval":            {{Kind: sequence.ActivateStep}, {Kind: sequence.DeactivateStep}, {Kind: sequence.MessageStep, Message: message}},
		"unclosed":                 {{Kind: sequence.ActivateStep}, {Kind: sequence.MessageStep, Message: message}},
		"cross fragment":           {{Kind: sequence.ActivateStep}, {Kind: sequence.FragmentStartStep, Fragment: sequence.LoopFragment, Label: "x"}, {Kind: sequence.MessageStep, Message: message}, {Kind: sequence.FragmentEndStep}, {Kind: sequence.DeactivateStep}},
		"control carries label":    {{Kind: sequence.ActivateStep, Label: "ignored"}, {Kind: sequence.MessageStep, Message: message}, {Kind: sequence.DeactivateStep}},
	}
	for name, steps := range tests {
		t.Run(name, func(t *testing.T) {
			assertSequenceErrorNoPanic(t, &sequence.Diagram{Participants: participant, Steps: steps}, Options{MaxWidth: 512, MaxHeight: 512}, ErrInvalidSequence)
		})
	}
}

func TestSequenceActivationRepresentativeAllocationIsBounded(t *testing.T) {
	diagram := &sequence.Diagram{}
	for participant := 0; participant < 12; participant++ {
		diagram.Participants = append(diagram.Participants, sequence.Participant{ID: fmt.Sprintf("P%d", participant), Label: fmt.Sprintf("P%d", participant)})
	}
	for activation := 0; activation < 96; activation++ {
		diagram.Steps = append(diagram.Steps, sequence.Step{Kind: sequence.ActivateStep, Participant: activation / 8})
	}
	diagram.Steps = append(diagram.Steps, sequence.Step{Kind: sequence.MessageStep, Message: sequence.Message{From: 0, To: 11, Label: "active", Kind: sequence.Request}})
	for activation := 95; activation >= 0; activation-- {
		diagram.Steps = append(diagram.Steps, sequence.Step{Kind: sequence.DeactivateStep, Participant: activation / 8})
	}
	options := Options{MaxWidth: 512, MaxHeight: 512}
	if _, err := Sequence(diagram, options); err != nil {
		t.Fatal(err)
	}
	allocations := testing.AllocsPerRun(10, func() {
		if _, err := Sequence(diagram, options); err != nil {
			panic(err)
		}
	})
	if allocations > 2_500 {
		t.Fatalf("activation allocations/run=%.0f", allocations)
	}
	t.Logf("activation allocations/run=%.0f", allocations)
}

func TestSequenceActivationDirectHardLimits(t *testing.T) {
	participants := make([]sequence.Participant, 13)
	for index := range participants {
		participants[index] = sequence.Participant{ID: fmt.Sprintf("P%d", index), Label: fmt.Sprintf("P%d", index)}
	}
	message := sequence.Step{Kind: sequence.MessageStep, Message: sequence.Message{From: 0, To: 0, Label: "x", Kind: sequence.Request}}

	tooMany := &sequence.Diagram{Participants: participants}
	for activation := 0; activation < 97; activation++ {
		tooMany.Steps = append(tooMany.Steps, sequence.Step{Kind: sequence.ActivateStep, Participant: activation / 8})
	}
	tooMany.Steps = append(tooMany.Steps, message)
	for activation := 96; activation >= 0; activation-- {
		tooMany.Steps = append(tooMany.Steps, sequence.Step{Kind: sequence.DeactivateStep, Participant: activation / 8})
	}

	tooDeep := &sequence.Diagram{Participants: participants[:1]}
	for depth := 0; depth < 9; depth++ {
		tooDeep.Steps = append(tooDeep.Steps, sequence.Step{Kind: sequence.ActivateStep})
	}
	tooDeep.Steps = append(tooDeep.Steps, message)
	for depth := 0; depth < 9; depth++ {
		tooDeep.Steps = append(tooDeep.Steps, sequence.Step{Kind: sequence.DeactivateStep})
	}

	for name, diagram := range map[string]*sequence.Diagram{"ninety seven activations": tooMany, "depth nine": tooDeep} {
		t.Run(name, func(t *testing.T) {
			assertSequenceErrorNoPanic(t, diagram, Options{MaxWidth: 512, MaxHeight: 512}, ErrInvalidSequence)
		})
	}
}
