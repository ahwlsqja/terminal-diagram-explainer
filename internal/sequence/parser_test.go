package sequence_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/sequence"
)

const maxSequenceSourceBytes = 256 * 1024

func TestParseSequenceKeepsDeclarationAndMessageSourceOrder(t *testing.T) {
	source := `%% before header
sequenceDiagram
participant API as Gateway
participant Auth as Auth Service
participant Store
API ->> Auth: validate token
API ->> Store: read session
Auth -->> API: accepted`

	diagram, err := sequence.Parse(source, sequence.DefaultLimits())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got, want := len(diagram.Participants), 3; got != want {
		t.Fatalf("participants=%d, want %d", got, want)
	}
	for index, want := range []struct{ id, label string }{
		{"API", "Gateway"},
		{"Auth", "Auth Service"},
		{"Store", "Store"},
	} {
		got := diagram.Participants[index]
		if got.ID != want.id || got.Label != want.label {
			t.Fatalf("participant %d=%+v, want ID=%q Label=%q", index, got, want.id, want.label)
		}
	}

	if got, want := len(diagram.Messages), 3; got != want {
		t.Fatalf("messages=%d, want %d", got, want)
	}
	for index, want := range []struct {
		from, to int
		label    string
		kind     sequence.Kind
	}{
		{0, 1, "validate token", sequence.Request},
		{0, 2, "read session", sequence.Request},
		{1, 0, "accepted", sequence.Return},
	} {
		got := diagram.Messages[index]
		if got.From != want.from || got.To != want.to || got.Label != want.label || got.Kind != want.kind {
			t.Fatalf("message %d=%+v, want from=%d to=%d label=%q kind=%v", index, got, want.from, want.to, want.label, want.kind)
		}
	}
}

func TestParseSequenceAcceptsCompactHyphenIDsSelfAndLiteralMessageSuffixes(t *testing.T) {
	source := `sequenceDiagram
participant API-Gateway as Gateway
participant DB
API-Gateway->>DB: GET /v1:status; %% literal
DB-->>API-Gateway: ok
API-Gateway ->> API-Gateway: cache hit
API-Gateway -->> API-Gateway: cached`

	diagram, err := sequence.Parse(source, sequence.DefaultLimits())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got, want := len(diagram.Messages), 4; got != want {
		t.Fatalf("messages=%d, want %d", got, want)
	}
	if got, want := diagram.Messages[0].Label, "GET /v1:status; %% literal"; got != want {
		t.Fatalf("first label=%q, want %q", got, want)
	}
	for _, index := range []int{2, 3} {
		message := diagram.Messages[index]
		if message.From != 0 || message.To != 0 {
			t.Fatalf("self message %d=%+v", index, message)
		}
	}
	if diagram.Messages[3].Kind != sequence.Return {
		t.Fatalf("self return kind=%v, want Return", diagram.Messages[3].Kind)
	}
}

func TestParseSequenceCommentsCRLFAndPhysicalByteColumns(t *testing.T) {
	t.Run("CRLF and comments", func(t *testing.T) {
		source := "%% ignore\r\nsequenceDiagram\r\n\tparticipant A\r\n  participant B as Bee\r\n\tA ->> B: hello\r\n"
		diagram, err := sequence.Parse(source, sequence.DefaultLimits())
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if len(diagram.Participants) != 2 || len(diagram.Messages) != 1 {
			t.Fatalf("diagram=%+v", diagram)
		}
	})

	t.Run("leading tab is physical byte column", func(t *testing.T) {
		source := "sequenceDiagram\n\tparticipant A\n\tparticipant B\n\tA ==> B: broken"
		assertParseError(t, source, 4, 4)
	})
}

func TestParseSequenceRejectsClosedGrammarAndStateViolations(t *testing.T) {
	tests := []struct {
		name   string
		source string
		line   int
		column int
	}{
		{
			name:   "header semicolon is not accepted",
			source: "sequenceDiagram;\nparticipant A\nA ->> A: x",
			line:   1, column: 1,
		},
		{
			name:   "participant must precede message",
			source: "sequenceDiagram\nparticipant A\nA ->> A: x\nparticipant B",
			line:   4, column: 1,
		},
		{
			name:   "missing sender declaration",
			source: "sequenceDiagram\nparticipant B\nA ->> B: x",
			line:   3, column: 1,
		},
		{
			name:   "missing receiver declaration",
			source: "sequenceDiagram\nparticipant A\nA ->> B: x",
			line:   3, column: 7,
		},
		{
			name:   "display alias is not endpoint",
			source: "sequenceDiagram\nparticipant API as Gateway\nparticipant DB\nGateway ->> DB: x",
			line:   4, column: 1,
		},
		{
			name:   "duplicate participant ID",
			source: "sequenceDiagram\nparticipant A\nparticipant A\nA ->> A: x",
			line:   3, column: 13,
		},
		{
			name:   "duplicate display label",
			source: "sequenceDiagram\nparticipant A as API\nparticipant B as API\nA ->> B: x",
			line:   3, column: 18,
		},
		{
			name:   "reserved participant ID",
			source: "sequenceDiagram\nparticipant participant\nparticipant A\nA ->> A: x",
			line:   2, column: 13,
		},
		{
			name:   "chain before message label is not supported",
			source: "sequenceDiagram\nparticipant A\nparticipant B\nparticipant C\nA ->> B ->> C: second",
			line:   5, column: 9,
		},
		{
			name:   "comma fanout is not supported",
			source: "sequenceDiagram\nparticipant A\nparticipant B\nparticipant C\nA ->> B, C: x",
			line:   5, column: 8,
		},
		{
			name:   "flow arrow is not a sequence request",
			source: "sequenceDiagram\nparticipant A\nparticipant B\nA --> B: x",
			line:   4, column: 3,
		},
		{
			name:   "missing message colon",
			source: "sequenceDiagram\nparticipant A\nA ->> A",
			line:   3, column: 8,
		},
		{
			name:   "empty message label",
			source: "sequenceDiagram\nparticipant A\nA ->> A:   ",
			line:   3, column: 8,
		},
		{
			name:   "empty alias",
			source: "sequenceDiagram\nparticipant A as   \nA ->> A: x",
			line:   2, column: 15,
		},
		{
			name:   "no message",
			source: "sequenceDiagram\nparticipant A",
			line:   1, column: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertParseError(t, tt.source, tt.line, tt.column)
		})
	}

	t.Run("message label preserves arrow after first colon", func(t *testing.T) {
		source := "sequenceDiagram\nparticipant A\nparticipant B\nA ->> B: first ->> C: second"
		diagram, err := sequence.Parse(source, sequence.DefaultLimits())
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if got, want := diagram.Messages[0].Label, "first ->> C: second"; got != want {
			t.Fatalf("label=%q, want %q", got, want)
		}
	})
}

func TestParseSequenceValidatesTextAndTransport(t *testing.T) {
	for _, tt := range []struct {
		name   string
		source string
	}{
		{"invalid UTF-8", "sequenceDiagram\nparticipant A\nA ->> A: \xff"},
		{"lone CR", "sequenceDiagram\rparticipant A\nA ->> A: x"},
		{"ESC alias", "sequenceDiagram\nparticipant A as bad\x1b\nA ->> A: x"},
		{"bidi message", "sequenceDiagram\nparticipant A\nA ->> A: bad\u202e"},
		{"combining prefix", "sequenceDiagram\nparticipant A as \u0301bad\nA ->> A: x"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if diagram, err := sequence.Parse(tt.source, sequence.DefaultLimits()); err == nil || diagram != nil {
				t.Fatalf("Parse() diagram=%+v err=%v, want nil graph and error", diagram, err)
			}
		})
	}
}

func TestParseSequenceLimits(t *testing.T) {
	limits := sequence.DefaultLimits()

	t.Run("participants exact boundary", func(t *testing.T) {
		var source strings.Builder
		source.WriteString("sequenceDiagram\n")
		for index := 0; index < limits.MaxParticipants; index++ {
			source.WriteString("participant ")
			source.WriteString(sequenceID(index))
			source.WriteByte('\n')
		}
		source.WriteString(sequenceID(0) + " ->> " + sequenceID(0) + ": x")
		if _, err := sequence.Parse(source.String(), limits); err != nil {
			t.Fatalf("%d participants rejected: %v", limits.MaxParticipants, err)
		}
		source.WriteString("\nparticipant Overflow")
		if diagram, err := sequence.Parse(source.String(), limits); err == nil || diagram != nil {
			t.Fatalf("participant limit bypassed: diagram=%+v err=%v", diagram, err)
		}
	})

	t.Run("messages exact boundary", func(t *testing.T) {
		var source strings.Builder
		source.WriteString("sequenceDiagram\nparticipant A\n")
		for index := 0; index < limits.MaxMessages; index++ {
			source.WriteString("A ->> A: x\n")
		}
		if _, err := sequence.Parse(source.String(), limits); err != nil {
			t.Fatalf("%d messages rejected: %v", limits.MaxMessages, err)
		}
		source.WriteString("A ->> A: overflow")
		if diagram, err := sequence.Parse(source.String(), limits); err == nil || diagram != nil {
			t.Fatalf("message limit bypassed: diagram=%+v err=%v", diagram, err)
		}
	})

	t.Run("ID and labels exact boundaries", func(t *testing.T) {
		id64 := "A" + strings.Repeat("x", limits.MaxIDBytes-1)
		label96 := strings.Repeat("한", limits.MaxLabelCells/2)
		accepted := "sequenceDiagram\nparticipant " + id64 + " as " + label96 + "\n" + id64 + " ->> " + id64 + ": " + label96
		if _, err := sequence.Parse(accepted, limits); err != nil {
			t.Fatalf("exact ID/label boundary rejected: %v", err)
		}
		tooLongID := "A" + strings.Repeat("x", limits.MaxIDBytes)
		if _, err := sequence.Parse("sequenceDiagram\nparticipant "+tooLongID+"\nA ->> A: x", limits); err == nil {
			t.Fatal("65-byte ID accepted")
		}
		tooWide := strings.Repeat("한", limits.MaxLabelCells/2+1)
		if _, err := sequence.Parse("sequenceDiagram\nparticipant A\nA ->> A: "+tooWide, limits); err == nil {
			t.Fatal("97+ cell message accepted")
		}
	})

	t.Run("source byte boundary", func(t *testing.T) {
		prefix := "sequenceDiagram\nparticipant A\nA ->> A: x\n%%"
		within := prefix + strings.Repeat("x", maxSequenceSourceBytes-len(prefix))
		if len(within) != maxSequenceSourceBytes {
			t.Fatalf("within size=%d", len(within))
		}
		if _, err := sequence.Parse(within, limits); err != nil {
			t.Fatalf("256KiB source rejected: %v", err)
		}
		if diagram, err := sequence.Parse(within+"x", limits); err == nil || diagram != nil {
			t.Fatalf("source byte limit bypassed: diagram=%+v err=%v", diagram, err)
		}
	})
}

func TestParseSequenceRejectsInvalidLimitsWithoutPanic(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*sequence.Limits)
	}{
		{"zero participants", func(limits *sequence.Limits) { limits.MaxParticipants = 0 }},
		{"negative participants", func(limits *sequence.Limits) { limits.MaxParticipants = -1 }},
		{"zero messages", func(limits *sequence.Limits) { limits.MaxMessages = 0 }},
		{"negative messages", func(limits *sequence.Limits) { limits.MaxMessages = -1 }},
		{"zero ID bytes", func(limits *sequence.Limits) { limits.MaxIDBytes = 0 }},
		{"negative ID bytes", func(limits *sequence.Limits) { limits.MaxIDBytes = -1 }},
		{"zero label cells", func(limits *sequence.Limits) { limits.MaxLabelCells = 0 }},
		{"negative label cells", func(limits *sequence.Limits) { limits.MaxLabelCells = -1 }},
		{"zero lines", func(limits *sequence.Limits) { limits.MaxLines = 0 }},
		{"negative lines", func(limits *sequence.Limits) { limits.MaxLines = -1 }},
		{"zero source bytes", func(limits *sequence.Limits) { limits.MaxSourceBytes = 0 }},
		{"negative source bytes", func(limits *sequence.Limits) { limits.MaxSourceBytes = -1 }},
		{"zero activations", func(limits *sequence.Limits) { limits.MaxActivations = 0 }},
		{"negative activations", func(limits *sequence.Limits) { limits.MaxActivations = -1 }},
		{"zero activation depth", func(limits *sequence.Limits) { limits.MaxActivationDepth = 0 }},
		{"negative activation depth", func(limits *sequence.Limits) { limits.MaxActivationDepth = -1 }},
		{"MaxInt", func(limits *sequence.Limits) {
			limits.MaxLines = math.MaxInt
			limits.MaxParticipants = math.MaxInt
			limits.MaxMessages = math.MaxInt
			limits.MaxIDBytes = math.MaxInt
			limits.MaxLabelCells = math.MaxInt
			limits.MaxSourceBytes = math.MaxInt
			limits.MaxActivations = math.MaxInt
			limits.MaxActivationDepth = math.MaxInt
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limits := sequence.DefaultLimits()
			tt.mutate(&limits)
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("Parse() panicked: %v", recovered)
				}
			}()
			diagram, err := sequence.Parse("sequenceDiagram\nparticipant A\nA ->> A: x", limits)
			if tt.name != "MaxInt" && (err == nil || diagram != nil) {
				t.Fatalf("invalid limit accepted: diagram=%+v err=%v", diagram, err)
			}
		})
	}
}

func assertParseError(t *testing.T, source string, wantLine, wantColumn int) {
	t.Helper()
	diagram, err := sequence.Parse(source, sequence.DefaultLimits())
	if err == nil || diagram != nil {
		t.Fatalf("Parse() diagram=%+v err=%v, want nil graph and error", diagram, err)
	}
	var parseErr *sequence.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error type=%T, want *sequence.ParseError", err)
	}
	if parseErr.Line != wantLine || parseErr.Column != wantColumn {
		t.Fatalf("error location=%d:%d, want %d:%d (%v)", parseErr.Line, parseErr.Column, wantLine, wantColumn, err)
	}
}

func sequenceID(index int) string {
	return "P" + string(rune('A'+index))
}
