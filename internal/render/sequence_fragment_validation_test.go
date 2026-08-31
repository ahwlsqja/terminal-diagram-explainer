package render

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/sequence"
)

func TestSequenceFragmentValidationRejectsMalformedDirectTimelines(t *testing.T) {
	participant := []sequence.Participant{{ID: "A", Label: "A"}}
	message := sequence.Message{From: 0, To: 0, Label: "x", Kind: sequence.Request}
	validSteps := []sequence.Step{
		{Kind: sequence.FragmentStartStep, Fragment: sequence.LoopFragment, Label: "loop"},
		{Kind: sequence.MessageStep, Message: message},
		{Kind: sequence.FragmentEndStep},
	}
	tests := []struct {
		name    string
		diagram *sequence.Diagram
	}{
		{name: "both modes", diagram: &sequence.Diagram{Participants: participant, Messages: []sequence.Message{message}, Steps: validSteps}},
		{name: "empty peer slice", diagram: &sequence.Diagram{Participants: participant, Messages: []sequence.Message{message}, Steps: []sequence.Step{}}},
		{name: "invalid step kind", diagram: &sequence.Diagram{Participants: participant, Steps: []sequence.Step{{Kind: sequence.StepKind(99)}}}},
		{name: "invalid fragment kind", diagram: &sequence.Diagram{Participants: participant, Steps: []sequence.Step{{Kind: sequence.FragmentStartStep, Fragment: sequence.FragmentKind(99), Label: "x"}, {Kind: sequence.MessageStep, Message: message}, {Kind: sequence.FragmentEndStep}}}},
		{name: "root branch", diagram: &sequence.Diagram{Participants: participant, Steps: []sequence.Step{{Kind: sequence.FragmentBranchStep, Label: "x"}, {Kind: sequence.MessageStep, Message: message}}}},
		{name: "root end", diagram: &sequence.Diagram{Participants: participant, Steps: []sequence.Step{{Kind: sequence.FragmentEndStep}, {Kind: sequence.MessageStep, Message: message}}}},
		{name: "empty loop", diagram: &sequence.Diagram{Participants: participant, Steps: []sequence.Step{{Kind: sequence.FragmentStartStep, Fragment: sequence.LoopFragment, Label: "x"}, {Kind: sequence.FragmentEndStep}}}},
		{name: "alt without else", diagram: &sequence.Diagram{Participants: participant, Steps: []sequence.Step{{Kind: sequence.FragmentStartStep, Fragment: sequence.AltFragment, Label: "x"}, {Kind: sequence.MessageStep, Message: message}, {Kind: sequence.FragmentEndStep}}}},
		{name: "empty label", diagram: &sequence.Diagram{Participants: participant, Steps: []sequence.Step{{Kind: sequence.FragmentStartStep, Fragment: sequence.OptFragment}, {Kind: sequence.MessageStep, Message: message}, {Kind: sequence.FragmentEndStep}}}},
		{name: "hostile label", diagram: &sequence.Diagram{Participants: participant, Steps: []sequence.Step{{Kind: sequence.FragmentStartStep, Fragment: sequence.OptFragment, Label: "bad\x1b"}, {Kind: sequence.MessageStep, Message: message}, {Kind: sequence.FragmentEndStep}}}},
		{name: "wide label", diagram: &sequence.Diagram{Participants: participant, Steps: []sequence.Step{{Kind: sequence.FragmentStartStep, Fragment: sequence.OptFragment, Label: strings.Repeat("x", 97)}, {Kind: sequence.MessageStep, Message: message}, {Kind: sequence.FragmentEndStep}}}},
		{name: "message carries control label", diagram: &sequence.Diagram{Participants: participant, Steps: []sequence.Step{{Kind: sequence.MessageStep, Message: message, Label: "ignored"}}}},
		{name: "message carries fragment", diagram: &sequence.Diagram{Participants: participant, Steps: []sequence.Step{{Kind: sequence.MessageStep, Message: message, Fragment: sequence.AltFragment}}}},
		{name: "start carries message", diagram: &sequence.Diagram{Participants: participant, Steps: []sequence.Step{{Kind: sequence.FragmentStartStep, Fragment: sequence.LoopFragment, Label: "x", Message: message}, {Kind: sequence.MessageStep, Message: message}, {Kind: sequence.FragmentEndStep}}}},
		{name: "branch carries message", diagram: &sequence.Diagram{Participants: participant, Steps: []sequence.Step{{Kind: sequence.FragmentStartStep, Fragment: sequence.AltFragment, Label: "x"}, {Kind: sequence.MessageStep, Message: message}, {Kind: sequence.FragmentBranchStep, Label: "y", Message: message}, {Kind: sequence.MessageStep, Message: message}, {Kind: sequence.FragmentEndStep}}}},
		{name: "end carries label", diagram: &sequence.Diagram{Participants: participant, Steps: []sequence.Step{{Kind: sequence.FragmentStartStep, Fragment: sequence.LoopFragment, Label: "x"}, {Kind: sequence.MessageStep, Message: message}, {Kind: sequence.FragmentEndStep, Label: "ignored"}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertSequenceErrorNoPanic(t, test.diagram, Options{MaxWidth: 512, MaxHeight: 512}, ErrInvalidSequence)
		})
	}
}

func TestSequenceLegacyAndExtendedModesRemainDistinct(t *testing.T) {
	legacy := &sequence.Diagram{
		Participants: []sequence.Participant{{ID: "A", Label: "A"}},
		Messages:     []sequence.Message{{From: 0, To: 0, Label: "legacy", Kind: sequence.Request}},
	}
	legacyOutput, err := Sequence(legacy, Options{MaxWidth: 80, MaxHeight: 20})
	if err != nil {
		t.Fatal(err)
	}
	extended := &sequence.Diagram{
		Participants: legacy.Participants,
		Steps: []sequence.Step{
			{Kind: sequence.FragmentStartStep, Fragment: sequence.LoopFragment, Label: "one"},
			{Kind: sequence.MessageStep, Message: legacy.Messages[0]},
			{Kind: sequence.FragmentEndStep},
		},
	}
	extendedOutput, err := Sequence(extended, Options{MaxWidth: 80, MaxHeight: 30})
	if err != nil {
		t.Fatal(err)
	}
	if legacyOutput == extendedOutput || !strings.Contains(extendedOutput, "loop: one") {
		t.Fatalf("mode outputs not distinct:\nlegacy:\n%s\nextended:\n%s", legacyOutput, extendedOutput)
	}
	if _, err := Sequence(&sequence.Diagram{Participants: legacy.Participants, Messages: nil, Steps: nil}, Options{MaxWidth: 80, MaxHeight: 20}); !errors.Is(err, ErrInvalidSequence) {
		t.Fatalf("empty modes error=%v", err)
	}
}

func TestSequenceFragmentRepresentativeAllocationIsBounded(t *testing.T) {
	diagram := &sequence.Diagram{Participants: []sequence.Participant{{ID: "A", Label: "A"}}}
	for index := 0; index < 32; index++ {
		diagram.Steps = append(diagram.Steps,
			sequence.Step{Kind: sequence.FragmentStartStep, Fragment: sequence.LoopFragment, Label: fmt.Sprintf("loop-%02d", index)},
			sequence.Step{Kind: sequence.MessageStep, Message: sequence.Message{From: 0, To: 0, Label: fmt.Sprintf("inside-%02d", index), Kind: sequence.Request}},
			sequence.Step{Kind: sequence.FragmentEndStep},
		)
	}
	for index := 32; index < 96; index++ {
		diagram.Steps = append(diagram.Steps, sequence.Step{
			Kind:    sequence.MessageStep,
			Message: sequence.Message{From: 0, To: 0, Label: fmt.Sprintf("outside-%02d", index), Kind: sequence.Request},
		})
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
		t.Fatalf("fragment allocations/run=%.0f, limit=2500", allocations)
	}
	t.Logf("fragment allocations/run=%.0f", allocations)
}

func TestSequenceFragmentDirectHardLimits(t *testing.T) {
	participant := []sequence.Participant{{ID: "A", Label: "A"}}
	message := sequence.Message{From: 0, To: 0, Label: "x", Kind: sequence.Request}

	tooManyFragments := &sequence.Diagram{Participants: participant}
	for index := 0; index < 33; index++ {
		tooManyFragments.Steps = append(tooManyFragments.Steps,
			sequence.Step{Kind: sequence.FragmentStartStep, Fragment: sequence.LoopFragment, Label: fmt.Sprintf("F%02d", index)},
			sequence.Step{Kind: sequence.MessageStep, Message: message},
			sequence.Step{Kind: sequence.FragmentEndStep},
		)
	}

	tooDeep := &sequence.Diagram{Participants: participant}
	for index := 0; index < 9; index++ {
		tooDeep.Steps = append(tooDeep.Steps, sequence.Step{Kind: sequence.FragmentStartStep, Fragment: sequence.LoopFragment, Label: fmt.Sprintf("D%02d", index)})
	}
	tooDeep.Steps = append(tooDeep.Steps, sequence.Step{Kind: sequence.MessageStep, Message: message})
	for index := 0; index < 9; index++ {
		tooDeep.Steps = append(tooDeep.Steps, sequence.Step{Kind: sequence.FragmentEndStep})
	}

	tooManySteps := &sequence.Diagram{Participants: participant, Steps: make([]sequence.Step, 193)}
	for index := range tooManySteps.Steps {
		tooManySteps.Steps[index] = sequence.Step{Kind: sequence.MessageStep, Message: message}
	}

	for name, diagram := range map[string]*sequence.Diagram{
		"thirty three fragments":         tooManyFragments,
		"depth nine":                     tooDeep,
		"one hundred ninety three steps": tooManySteps,
	} {
		t.Run(name, func(t *testing.T) {
			assertSequenceErrorNoPanic(t, diagram, Options{MaxWidth: 512, MaxHeight: 512}, ErrInvalidSequence)
		})
	}
}

func FuzzSequenceFragmentNoPanic(f *testing.F) {
	for _, seed := range []uint8{0, 1, 2, 3, 4, 5, 255} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, mode uint8) {
		message := sequence.Message{From: 0, To: 0, Label: "x", Kind: sequence.Request}
		diagram := &sequence.Diagram{Participants: []sequence.Participant{{ID: "A", Label: "A"}}}
		switch mode % 6 {
		case 0:
			diagram.Steps = []sequence.Step{{Kind: sequence.FragmentStartStep, Fragment: sequence.LoopFragment, Label: "x"}, {Kind: sequence.MessageStep, Message: message}, {Kind: sequence.FragmentEndStep}}
		case 1:
			diagram.Steps = []sequence.Step{{Kind: sequence.FragmentStartStep, Fragment: sequence.AltFragment, Label: "x"}, {Kind: sequence.MessageStep, Message: message}, {Kind: sequence.FragmentBranchStep, Label: "y"}, {Kind: sequence.MessageStep, Message: message}, {Kind: sequence.FragmentEndStep}}
		case 2:
			diagram.Steps = []sequence.Step{{Kind: sequence.StepKind(99)}}
		case 3:
			diagram.Steps = []sequence.Step{{Kind: sequence.FragmentEndStep}}
		case 4:
			diagram.Messages = []sequence.Message{message}
			diagram.Steps = []sequence.Step{}
		case 5:
			diagram.Steps = []sequence.Step{{Kind: sequence.MessageStep, Message: message, Label: "ignored"}}
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("Sequence panicked: %v", recovered)
			}
		}()
		output, err := Sequence(diagram, Options{MaxWidth: 512, MaxHeight: 512})
		if err != nil && output != "" {
			t.Fatalf("error returned partial output: %q", output)
		}
	})
}
