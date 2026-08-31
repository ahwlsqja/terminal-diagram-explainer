package render

import (
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/sequence"
)

const parFixture = `sequenceDiagram
participant API
participant Email
participant SMS
par notify
API ->> Email: email
and sms branch
API ->> SMS: text-message
and audit branch
API ->> API: record
end`

func TestSequenceParMakesDisplayOrderExplicit(t *testing.T) {
	diagram, err := sequence.Parse(parFixture, sequence.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	output := mustRenderSequence(t, diagram, Options{MaxWidth: 180, MaxHeight: 60})
	for _, text := range []string{"par (display order only): notify", "and: sms branch", "and: audit branch", "email", "text-message", "record"} {
		assertSequenceTextOnce(t, output, text)
	}
	if strings.Count(output, "├") < 2 || strings.Count(output, "┤") < 2 {
		t.Fatalf("par separators missing:\n%s", output)
	}
}

func TestSequenceParASCIIAndNestedActivation(t *testing.T) {
	source := `sequenceDiagram
participant A
participant B
par one
activate B
A ->> B: call
deactivate B
and two
loop retry
B -->> A: done
end
end`
	diagram, err := sequence.Parse(source, sequence.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	output := mustRenderSequence(t, diagram, Options{ASCII: true, MaxWidth: 140, MaxHeight: 60})
	if strings.ContainsAny(output, "┌┐└┘├┤─│┄┊┼▶◀") || !strings.Contains(output, "par (display order only): one") || !strings.Contains(output, "and: two") {
		t.Fatalf("ASCII par output invalid:\n%s", output)
	}
}

func TestSequenceParDirectModelRejectsElseAndAltRejectsAnd(t *testing.T) {
	participant := []sequence.Participant{{ID: "A", Label: "A"}}
	message := sequence.Message{From: 0, To: 0, Label: "x", Kind: sequence.Request}
	parWithElse := &sequence.Diagram{Participants: participant, Steps: []sequence.Step{
		{Kind: sequence.FragmentStartStep, Fragment: sequence.ParFragment, Label: "parallel"},
		{Kind: sequence.MessageStep, Message: message},
		{Kind: sequence.FragmentBranchStep, Branch: sequence.ElseBranch, Label: "wrong"},
		{Kind: sequence.MessageStep, Message: message},
		{Kind: sequence.FragmentEndStep},
	}}
	altWithAnd := &sequence.Diagram{Participants: participant, Steps: []sequence.Step{
		{Kind: sequence.FragmentStartStep, Fragment: sequence.AltFragment, Label: "choice"},
		{Kind: sequence.MessageStep, Message: message},
		{Kind: sequence.FragmentBranchStep, Branch: sequence.AndBranch, Label: "wrong"},
		{Kind: sequence.MessageStep, Message: message},
		{Kind: sequence.FragmentEndStep},
	}}
	for name, diagram := range map[string]*sequence.Diagram{"par with else": parWithElse, "alt with and": altWithAnd} {
		t.Run(name, func(t *testing.T) {
			assertSequenceErrorNoPanic(t, diagram, Options{MaxWidth: 512, MaxHeight: 512}, ErrInvalidSequence)
		})
	}
}
