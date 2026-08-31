package sequence_test

import (
	"errors"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/sequence"
)

func TestParParserBuildsSourceOrderBranches(t *testing.T) {
	source := `sequenceDiagram
participant API
participant Email
participant SMS
par notify by email
API ->> Email: send email
and notify by sms
API ->> SMS: send sms
and audit
API ->> API: record
end`
	diagram, err := sequence.Parse(source, sequence.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	wantKinds := []sequence.StepKind{sequence.FragmentStartStep, sequence.MessageStep, sequence.FragmentBranchStep, sequence.MessageStep, sequence.FragmentBranchStep, sequence.MessageStep, sequence.FragmentEndStep}
	if diagram.Messages != nil || len(diagram.Steps) != len(wantKinds) {
		t.Fatalf("timeline=%#v", diagram)
	}
	for index, kind := range wantKinds {
		if diagram.Steps[index].Kind != kind {
			t.Fatalf("step %d=%+v", index, diagram.Steps[index])
		}
	}
	if diagram.Steps[0].Fragment != sequence.ParFragment || diagram.Steps[2].Branch != sequence.AndBranch || diagram.Steps[2].Label != "notify by sms" || diagram.Steps[4].Branch != sequence.AndBranch || diagram.Steps[4].Label != "audit" {
		t.Fatalf("par payload=%#v", diagram.Steps)
	}
}

func TestParKeywordsRemainMessageParticipantIDs(t *testing.T) {
	source := `sequenceDiagram
participant par
participant and
participant A
par ->> A: request
and -->> A: response`
	diagram, err := sequence.Parse(source, sequence.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if diagram.Steps != nil || len(diagram.Messages) != 2 {
		t.Fatalf("message precedence drifted: %#v", diagram)
	}
}

func TestParParserRejectsMalformedBranches(t *testing.T) {
	tests := []struct {
		name string
		body string
		line int
	}{
		{name: "root and", body: "and x\nA ->> A: x", line: 3},
		{name: "par without and", body: "par x\nA ->> A: x\nend", line: 5},
		{name: "empty first branch", body: "par x\nand y\nA ->> A: y\nend", line: 4},
		{name: "empty later branch", body: "par x\nA ->> A: x\nand y\nend", line: 6},
		{name: "and in alt", body: "alt x\nA ->> A: x\nand y\nA ->> A: y\nend", line: 5},
		{name: "else in par", body: "par x\nA ->> A: x\nelse y\nA ->> A: y\nend", line: 5},
		{name: "active at and", body: "par x\nactivate A\nA ->> A: x\nand y\nA ->> A: y\nend", line: 6},
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
