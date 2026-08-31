package render

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/sequence"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

func TestSequenceValidationRejectsMalformedDirectDiagrams(t *testing.T) {
	tests := []struct {
		name    string
		diagram *sequence.Diagram
	}{
		{name: "nil"},
		{name: "no participants", diagram: &sequence.Diagram{Messages: []sequence.Message{{From: 0, To: 0, Label: "self", Kind: sequence.Request}}}},
		{name: "seventeen participants", diagram: sequenceDiagram(17, 1)},
		{name: "no messages", diagram: sequenceDiagram(1, 0)},
		{name: "ninety seven messages", diagram: sequenceDiagram(1, 97)},
		{
			name: "duplicate participant ID",
			diagram: &sequence.Diagram{
				Participants: []sequence.Participant{{ID: "A", Label: "A"}, {ID: "A", Label: "B"}},
				Messages:     []sequence.Message{{From: 0, To: 1, Label: "call", Kind: sequence.Request}},
			},
		},
		{
			name: "invalid participant ID",
			diagram: &sequence.Diagram{
				Participants: []sequence.Participant{{ID: "1A", Label: "A"}},
				Messages:     []sequence.Message{{From: 0, To: 0, Label: "self", Kind: sequence.Request}},
			},
		},
		{
			name: "duplicate participant label",
			diagram: &sequence.Diagram{
				Participants: []sequence.Participant{{ID: "A", Label: "same"}, {ID: "B", Label: "same"}},
				Messages:     []sequence.Message{{From: 0, To: 1, Label: "call", Kind: sequence.Request}},
			},
		},
		{
			name: "empty participant label",
			diagram: &sequence.Diagram{
				Participants: []sequence.Participant{{ID: "A", Label: ""}},
				Messages:     []sequence.Message{{From: 0, To: 0, Label: "self", Kind: sequence.Request}},
			},
		},
		{
			name: "terminal escape in participant label",
			diagram: &sequence.Diagram{
				Participants: []sequence.Participant{{ID: "A", Label: "safe\x1b[31munsafe"}},
				Messages:     []sequence.Message{{From: 0, To: 0, Label: "self", Kind: sequence.Request}},
			},
		},
		{
			name: "participant label wider than ninety six cells",
			diagram: &sequence.Diagram{
				Participants: []sequence.Participant{{ID: "A", Label: strings.Repeat("x", 97)}},
				Messages:     []sequence.Message{{From: 0, To: 0, Label: "self", Kind: sequence.Request}},
			},
		},
		{
			name: "empty message label",
			diagram: &sequence.Diagram{
				Participants: []sequence.Participant{{ID: "A", Label: "A"}},
				Messages:     []sequence.Message{{From: 0, To: 0, Label: "", Kind: sequence.Request}},
			},
		},
		{
			name: "terminal escape in message label",
			diagram: &sequence.Diagram{
				Participants: []sequence.Participant{{ID: "A", Label: "A"}},
				Messages:     []sequence.Message{{From: 0, To: 0, Label: "safe\x1b[31munsafe", Kind: sequence.Request}},
			},
		},
		{
			name: "message label wider than ninety six cells",
			diagram: &sequence.Diagram{
				Participants: []sequence.Participant{{ID: "A", Label: "A"}},
				Messages:     []sequence.Message{{From: 0, To: 0, Label: strings.Repeat("x", 97), Kind: sequence.Request}},
			},
		},
		{
			name: "invalid message kind",
			diagram: &sequence.Diagram{
				Participants: []sequence.Participant{{ID: "A", Label: "A"}},
				Messages:     []sequence.Message{{From: 0, To: 0, Label: "self", Kind: sequence.Kind(99)}},
			},
		},
		{
			name: "negative endpoint",
			diagram: &sequence.Diagram{
				Participants: []sequence.Participant{{ID: "A", Label: "A"}},
				Messages:     []sequence.Message{{From: -1, To: 0, Label: "bad", Kind: sequence.Request}},
			},
		},
		{
			name: "endpoint beyond participants",
			diagram: &sequence.Diagram{
				Participants: []sequence.Participant{{ID: "A", Label: "A"}},
				Messages:     []sequence.Message{{From: 0, To: 1, Label: "bad", Kind: sequence.Request}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertSequenceErrorNoPanic(t, test.diagram, Options{MaxWidth: 512, MaxHeight: 512}, ErrInvalidSequence)
		})
	}
}

func TestSequenceValidTightGeometryIsOutputBoundsError(t *testing.T) {
	assertSequenceErrorNoPanic(t, sequenceDiagram(1, 1), Options{MaxWidth: 1, MaxHeight: 1}, ErrOutputBounds)
}

func TestSequenceOptionsHardCanvasCapIsOutputBoundsError(t *testing.T) {
	assertSequenceErrorNoPanic(t, sequenceDiagram(1, 1), Options{MaxWidth: 513, MaxHeight: 512}, ErrOutputBounds)
}

func TestSequenceRepresentativeDirectModelIsStableAndBounded(t *testing.T) {
	diagram := sequenceDiagram(16, 96)
	options := Options{MaxWidth: 512, MaxHeight: 512}
	want, err := Sequence(diagram, options)
	if err != nil {
		t.Fatalf("Sequence() error = %v", err)
	}
	if want == "" {
		t.Fatal("Sequence() returned an empty successful rendering")
	}
	for iteration := 0; iteration < 256; iteration++ {
		got, renderErr := Sequence(diagram, options)
		if renderErr != nil {
			t.Fatalf("iteration=%d: %v", iteration, renderErr)
		}
		if got != want {
			t.Fatalf("iteration=%d output is nondeterministic", iteration)
		}
	}
	allocations := testing.AllocsPerRun(10, func() {
		if _, renderErr := Sequence(diagram, options); renderErr != nil {
			panic(renderErr)
		}
	})
	if allocations > 2_500 {
		t.Fatalf("allocations/run=%.0f, limit=2500", allocations)
	}
	t.Logf("sequence allocations/run=%.0f", allocations)
}

func sequenceDiagram(participantCount, messageCount int) *sequence.Diagram {
	diagram := &sequence.Diagram{
		Participants: make([]sequence.Participant, participantCount),
		Messages:     make([]sequence.Message, messageCount),
	}
	for index := range diagram.Participants {
		id := fmt.Sprintf("P%02d", index)
		diagram.Participants[index] = sequence.Participant{ID: id, Label: id}
	}
	for index := range diagram.Messages {
		from := index % participantCount
		to := from
		if participantCount > 1 {
			to = (from + 1) % participantCount
		}
		kind := sequence.Request
		if index%2 == 1 {
			kind = sequence.Return
		}
		diagram.Messages[index] = sequence.Message{
			From:  from,
			To:    to,
			Label: fmt.Sprintf("M%02d", index),
			Kind:  kind,
		}
	}
	return diagram
}

func assertSequenceErrorNoPanic(t *testing.T, diagram *sequence.Diagram, options Options, target error) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Sequence() panicked: %v", recovered)
		}
	}()
	output, err := Sequence(diagram, options)
	if !errors.Is(err, target) {
		t.Fatalf("Sequence() error=%v, want errors.Is(_, %v)", err, target)
	}
	if output != "" {
		t.Fatalf("Sequence() returned partial output: %q", output)
	}
	if target != ErrInvalidSequence && errors.Is(err, ErrInvalidSequence) {
		t.Fatalf("valid geometry was misclassified as invalid diagram: %v", err)
	}
}

func FuzzSequenceNoPanic(f *testing.F) {
	for _, seed := range [][6]uint8{
		{1, 1, 0, 0, 0, 0},
		{16, 96, 0, 0, 0, 0},
		{0, 1, 0, 0, 0, 0},
		{17, 1, 0, 0, 0, 0},
		{1, 0, 0, 0, 0, 0},
		{2, 1, 1, 1, 1, 1},
		{2, 1, 2, 2, 2, 2},
	} {
		f.Add(seed[0], seed[1], seed[2], seed[3], seed[4], seed[5])
	}

	f.Fuzz(func(t *testing.T, participantSeed, messageSeed, idMode, labelMode, endpointMode, kindMode uint8) {
		diagram := fuzzSequenceDiagram(participantSeed, messageSeed, idMode, labelMode, endpointMode, kindMode)
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("Sequence() panicked: %v", recovered)
			}
		}()

		output, err := Sequence(diagram, Options{MaxWidth: 512, MaxHeight: 512})
		if err != nil {
			if output != "" {
				t.Fatalf("error returned partial output: %q", output)
			}
			return
		}
		if output == "" {
			t.Fatal("successful render returned empty output")
		}
		assertSequenceFuzzOutputWithinBounds(t, output, 512, 512)
	})
}

func fuzzSequenceDiagram(participantSeed, messageSeed, idMode, labelMode, endpointMode, kindMode uint8) *sequence.Diagram {
	participantCount := int(participantSeed) % 18
	messageCount := int(messageSeed) % 98
	diagram := &sequence.Diagram{
		Participants: make([]sequence.Participant, participantCount),
		Messages:     make([]sequence.Message, messageCount),
	}
	for index := range diagram.Participants {
		id := fmt.Sprintf("P%02d", index)
		label := fmt.Sprintf("L%02d", index)
		switch idMode % 4 {
		case 1:
			id = "Duplicate"
		case 2:
			if index == 0 {
				id = "1Bad"
			}
		case 3:
			if index == 0 {
				id = "participant"
			}
		}
		switch labelMode % 5 {
		case 1:
			label = "Duplicate"
		case 2:
			if index == 0 {
				label = ""
			}
		case 3:
			if index == 0 {
				label = "bad\x1b[31m"
			}
		case 4:
			if index == 0 {
				label = strings.Repeat("x", 97)
			}
		}
		diagram.Participants[index] = sequence.Participant{ID: id, Label: label}
	}
	for index := range diagram.Messages {
		from, to := 0, 0
		if participantCount > 0 {
			from = index % participantCount
			to = (index + 1) % participantCount
		}
		label := fmt.Sprintf("M%02d", index)
		kind := sequence.Request
		if index%2 == 1 {
			kind = sequence.Return
		}
		if index == 0 {
			switch endpointMode % 3 {
			case 1:
				from = -1
			case 2:
				to = participantCount
			}
			switch kindMode % 4 {
			case 1:
				kind = sequence.Kind(99)
			case 2:
				label = ""
			case 3:
				label = "bad\x1b[31m"
			}
		}
		diagram.Messages[index] = sequence.Message{From: from, To: to, Label: label, Kind: kind}
	}
	return diagram
}

func assertSequenceFuzzOutputWithinBounds(t *testing.T, output string, maxWidth, maxHeight int) {
	t.Helper()
	rows := strings.Split(output, "\n")
	if len(rows) > maxHeight {
		t.Fatalf("height=%d, limit=%d", len(rows), maxHeight)
	}
	for rowIndex, row := range rows {
		width, err := textcell.Width(row)
		if err != nil {
			t.Fatalf("row %d width error: %v", rowIndex, err)
		}
		if width > maxWidth {
			t.Fatalf("row %d width=%d, limit=%d", rowIndex, width, maxWidth)
		}
	}
}
