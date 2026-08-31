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
		if len(diagram.Messages) == 0 || len(diagram.Messages) > limits.MaxMessages {
			t.Fatalf("messages=%d", len(diagram.Messages))
		}
		for _, message := range diagram.Messages {
			if message.From < 0 || message.From >= len(diagram.Participants) || message.To < 0 || message.To >= len(diagram.Participants) {
				t.Fatalf("message endpoint out of range: %+v", message)
			}
		}
	})
}
