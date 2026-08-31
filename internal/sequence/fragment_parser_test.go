package sequence_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/sequence"
)

func TestFragmentParserPreservesLegacyMessageMode(t *testing.T) {
	diagram, err := sequence.Parse("sequenceDiagram\nparticipant A\nA ->> A: legacy", sequence.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if diagram.Steps != nil || len(diagram.Messages) != 1 {
		t.Fatalf("legacy mode drifted: Messages=%#v Steps=%#v", diagram.Messages, diagram.Steps)
	}
}

func TestFragmentParserBuildsOrderedExtendedTimeline(t *testing.T) {
	source := `sequenceDiagram
participant A
participant B
loop retry
A ->> B: request
alt accepted
B -->> A: ok
else rejected
B -->> A: failed
end
end
opt audit
A ->> A: record
end`
	diagram, err := sequence.Parse(source, sequence.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if diagram.Messages != nil {
		t.Fatalf("extended mode retained legacy messages: %#v", diagram.Messages)
	}
	want := []struct {
		kind     sequence.StepKind
		fragment sequence.FragmentKind
		label    string
	}{
		{sequence.FragmentStartStep, sequence.LoopFragment, "retry"},
		{sequence.MessageStep, 0, "request"},
		{sequence.FragmentStartStep, sequence.AltFragment, "accepted"},
		{sequence.MessageStep, 0, "ok"},
		{sequence.FragmentBranchStep, 0, "rejected"},
		{sequence.MessageStep, 0, "failed"},
		{sequence.FragmentEndStep, 0, ""},
		{sequence.FragmentEndStep, 0, ""},
		{sequence.FragmentStartStep, sequence.OptFragment, "audit"},
		{sequence.MessageStep, 0, "record"},
		{sequence.FragmentEndStep, 0, ""},
	}
	if len(diagram.Steps) != len(want) {
		t.Fatalf("steps=%d want=%d: %#v", len(diagram.Steps), len(want), diagram.Steps)
	}
	for index, expected := range want {
		got := diagram.Steps[index]
		label := got.Label
		if got.Kind == sequence.MessageStep {
			label = got.Message.Label
		}
		if got.Kind != expected.kind || got.Fragment != expected.fragment || label != expected.label {
			t.Fatalf("step %d=%+v want=%+v", index, got, expected)
		}
	}
}

func TestFragmentKeywordsRemainValidParticipantIDsForMessages(t *testing.T) {
	source := `sequenceDiagram
participant loop
participant end
participant A
loop ->> A: existing request
end -->> A: existing return`
	diagram, err := sequence.Parse(source, sequence.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if diagram.Steps != nil || len(diagram.Messages) != 2 {
		t.Fatalf("keyword message precedence drifted: %#v", diagram)
	}
}

func TestFragmentParserRejectsMalformedStructures(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		line   int
		column int
	}{
		{name: "root else", body: "else no\nA ->> A: x", line: 3, column: 1},
		{name: "root end", body: "end\nA ->> A: x", line: 3, column: 1},
		{name: "else in loop", body: "loop retry\nA ->> A: x\nelse no\nA ->> A: y\nend", line: 5, column: 1},
		{name: "duplicate else", body: "alt x\nA ->> A: x\nelse y\nA ->> A: y\nelse z\nA ->> A: z\nend", line: 7, column: 1},
		{name: "alt without else", body: "alt x\nA ->> A: x\nend", line: 5, column: 1},
		{name: "empty branch", body: "alt x\nelse y\nA ->> A: y\nend", line: 4, column: 1},
		{name: "empty opt", body: "opt x\nend", line: 4, column: 1},
		{name: "unclosed", body: "loop x\nA ->> A: x", line: 3, column: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := "sequenceDiagram\nparticipant A\n" + test.body
			diagram, err := sequence.Parse(source, sequence.DefaultLimits())
			if err == nil || diagram != nil {
				t.Fatalf("Parse() diagram=%#v err=%v", diagram, err)
			}
			var parseErr *sequence.ParseError
			if !errors.As(err, &parseErr) {
				t.Fatalf("error type=%T", err)
			}
			if parseErr.Line != test.line || parseErr.Column != test.column {
				t.Fatalf("location=%d:%d want=%d:%d: %v", parseErr.Line, parseErr.Column, test.line, test.column, err)
			}
		})
	}
}

func TestFragmentParserLimits(t *testing.T) {
	limits := sequence.DefaultLimits()
	if limits.MaxSteps != 256 || limits.MaxFragments != 32 || limits.MaxFragmentDepth != 8 {
		t.Fatalf("fragment limits=%+v", limits)
	}

	t.Run("fragment count", func(t *testing.T) {
		limited := limits
		limited.MaxFragments = 1
		source := "sequenceDiagram\nparticipant A\nloop one\nA ->> A: x\nend\nopt two\nA ->> A: y\nend"
		if diagram, err := sequence.Parse(source, limited); err == nil || diagram != nil {
			t.Fatalf("fragment limit bypassed: %#v %v", diagram, err)
		}
	})

	t.Run("depth", func(t *testing.T) {
		limited := limits
		limited.MaxFragmentDepth = 1
		source := "sequenceDiagram\nparticipant A\nloop one\nopt two\nA ->> A: x\nend\nend"
		if diagram, err := sequence.Parse(source, limited); err == nil || diagram != nil {
			t.Fatalf("depth limit bypassed: %#v %v", diagram, err)
		}
	})

	t.Run("steps", func(t *testing.T) {
		limited := limits
		limited.MaxSteps = 3
		source := "sequenceDiagram\nparticipant A\nloop one\nA ->> A: x\nA ->> A: y\nend"
		if diagram, err := sequence.Parse(source, limited); err == nil || diagram != nil {
			t.Fatalf("step limit bypassed: %#v %v", diagram, err)
		}
	})

	t.Run("legacy messages also consume steps", func(t *testing.T) {
		limited := limits
		limited.MaxSteps = 1
		source := "sequenceDiagram\nparticipant A\nA ->> A: x\nA ->> A: y"
		if diagram, err := sequence.Parse(source, limited); err == nil || diagram != nil {
			t.Fatalf("legacy step limit bypassed: %#v %v", diagram, err)
		}
	})

	t.Run("label", func(t *testing.T) {
		source := "sequenceDiagram\nparticipant A\nloop " + strings.Repeat("한", 49) + "\nA ->> A: x\nend"
		if diagram, err := sequence.Parse(source, limits); err == nil || diagram != nil {
			t.Fatalf("fragment label limit bypassed: %#v %v", diagram, err)
		}
	})
}

func TestFragmentParserDefaultBoundaries(t *testing.T) {
	limits := sequence.DefaultLimits()

	nested := func(depth int) string {
		var source strings.Builder
		source.WriteString("sequenceDiagram\nparticipant A\n")
		for index := 0; index < depth; index++ {
			source.WriteString("loop depth\n")
		}
		source.WriteString("A ->> A: x\n")
		for index := 0; index < depth; index++ {
			source.WriteString("end\n")
		}
		return source.String()
	}
	if _, err := sequence.Parse(nested(limits.MaxFragmentDepth), limits); err != nil {
		t.Fatalf("depth %d rejected: %v", limits.MaxFragmentDepth, err)
	}
	if diagram, err := sequence.Parse(nested(limits.MaxFragmentDepth+1), limits); err == nil || diagram != nil {
		t.Fatalf("depth %d accepted: %#v %v", limits.MaxFragmentDepth+1, diagram, err)
	}

	siblings := func(count int) string {
		var source strings.Builder
		source.WriteString("sequenceDiagram\nparticipant A\n")
		for index := 0; index < count; index++ {
			source.WriteString("loop sibling\nA ->> A: x\nend\n")
		}
		return source.String()
	}
	if _, err := sequence.Parse(siblings(limits.MaxFragments), limits); err != nil {
		t.Fatalf("%d fragments rejected: %v", limits.MaxFragments, err)
	}
	if diagram, err := sequence.Parse(siblings(limits.MaxFragments+1), limits); err == nil || diagram != nil {
		t.Fatalf("%d fragments accepted: %#v %v", limits.MaxFragments+1, diagram, err)
	}
}
