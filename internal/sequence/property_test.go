package sequence_test

import (
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/sequence"
)

func FuzzParseNoPanic(f *testing.F) {
	for _, seed := range []string{
		"sequenceDiagram\nparticipant A\nA ->> A: x",
		"sequenceDiagram\nparticipant API-Gateway as Gateway\nparticipant DB\nAPI-Gateway->>DB: GET /v1:status",
		"sequenceDiagram\nparticipant A\nA -->> A: return",
		"sequenceDiagram\nparticipant A\nloop retry\nA ->> A: x\nend",
		"sequenceDiagram\nparticipant A\nalt ok\nA ->> A: yes\nelse no\nA -->> A: no\nend",
		"sequenceDiagram;\nparticipant A\nA ->> A: x",
		"\xff\xfe",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, source string) {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("Parse() panicked: %v", recovered)
			}
		}()
		diagram, err := sequence.Parse(source, sequence.DefaultLimits())
		if err != nil {
			if diagram != nil {
				t.Fatalf("Parse() error returned partial diagram: %+v", diagram)
			}
			return
		}
		limits := sequence.DefaultLimits()
		if len(diagram.Participants) == 0 || len(diagram.Participants) > limits.MaxParticipants {
			t.Fatalf("participants=%d", len(diagram.Participants))
		}
		messages := diagram.Messages
		switch {
		case diagram.Steps == nil && diagram.Messages != nil:
		case diagram.Messages == nil && diagram.Steps != nil:
			if len(diagram.Steps) == 0 || len(diagram.Steps) > limits.MaxSteps {
				t.Fatalf("steps=%d", len(diagram.Steps))
			}
			for _, step := range diagram.Steps {
				if step.Kind == sequence.MessageStep {
					messages = append(messages, step.Message)
				}
			}
		default:
			t.Fatalf("invalid timeline mode: Messages=%#v Steps=%#v", diagram.Messages, diagram.Steps)
		}
		if len(messages) == 0 || len(messages) > limits.MaxMessages {
			t.Fatalf("messages=%d", len(messages))
		}
		for _, message := range messages {
			if message.From < 0 || message.From >= len(diagram.Participants) || message.To < 0 || message.To >= len(diagram.Participants) {
				t.Fatalf("message endpoint out of range: %+v", message)
			}
		}
	})
}
