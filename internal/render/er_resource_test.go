package render

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/er"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

func TestERFullParserMaximumFailsBoundsSafelyAndAllocationsStayBounded(t *testing.T) {
	diagram := &er.Diagram{Entities: make([]er.Entity, 32)}
	for entity := range diagram.Entities {
		diagram.Entities[entity] = er.Entity{ID: fmt.Sprintf("E%d", entity), Label: fmt.Sprintf("E%d", entity)}
		for attribute := 0; attribute < 6; attribute++ {
			diagram.Entities[entity].Attributes = append(diagram.Entities[entity].Attributes, er.Attribute{Type: "string", Name: fmt.Sprintf("a%d", attribute)})
		}
	}
	for relationship := 0; relationship < 64; relationship++ {
		diagram.Relationships = append(diagram.Relationships, er.Relationship{From: relationship % 32, To: (relationship + 1) % 32, FromMarker: er.ExactlyOne, ToMarker: er.ZeroOrMany, Label: fmt.Sprintf("R%d", relationship)})
	}
	output, err := ER(diagram, Options{MaxWidth: 512, MaxHeight: 512})
	if !errors.Is(err, ErrOutputBounds) || output != "" {
		t.Fatalf("full maximum must fail closed when geometry exceeds canvas: output=%q err=%v", output, err)
	}
	allocations := testing.AllocsPerRun(10, func() {
		_, _ = ER(diagram, Options{MaxWidth: 512, MaxHeight: 512})
	})
	if allocations > 2_500 {
		t.Fatalf("ER allocations/run=%.0f", allocations)
	}
	t.Logf("ER full-max failure allocations/run=%.0f", allocations)
}

func TestERTightBounds(t *testing.T) {
	diagram := &er.Diagram{Entities: []er.Entity{{ID: "A", Label: "A", Attributes: []er.Attribute{{Type: "uuid", Name: "id", Key: er.PrimaryKey}}}}}
	output, err := ER(diagram, Options{MaxWidth: 100, MaxHeight: 30})
	if err != nil {
		t.Fatal(err)
	}
	width := 0
	rows := strings.Split(output, "\n")
	for _, row := range rows {
		rowWidth, widthErr := textcell.Width(row)
		if widthErr != nil {
			t.Fatal(widthErr)
		}
		width = max(width, rowWidth)
	}
	if _, err := ER(diagram, Options{MaxWidth: width, MaxHeight: len(rows)}); err != nil {
		t.Fatalf("tight bounds rejected: %v", err)
	}
}

func TestERRelationshipLegendTightBounds(t *testing.T) {
	diagram := &er.Diagram{
		Entities:      []er.Entity{{ID: "A", Label: "A"}, {ID: "B", Label: "B"}},
		Relationships: []er.Relationship{{From: 0, To: 1, FromMarker: er.ExactlyOne, ToMarker: er.ZeroOrMany, Label: "links"}},
	}
	output, err := ER(diagram, Options{MaxWidth: 100, MaxHeight: 40})
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(output, "\n")
	width := 0
	for _, row := range rows {
		rowWidth, _ := textcell.Width(row)
		width = max(width, rowWidth)
	}
	if _, err := ER(diagram, Options{MaxWidth: width, MaxHeight: len(rows)}); err != nil {
		t.Fatalf("relationship tight bounds rejected: %v", err)
	}
	if _, err := ER(diagram, Options{MaxWidth: width, MaxHeight: len(rows) - 1}); !errors.Is(err, ErrOutputBounds) {
		t.Fatalf("short height error=%v", err)
	}
}

func FuzzERNoPanic(f *testing.F) {
	for _, seed := range []uint8{0, 1, 2, 3, 4, 255} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, mode uint8) {
		diagram := &er.Diagram{Entities: []er.Entity{{ID: "A", Label: "A"}}}
		switch mode % 5 {
		case 1:
			diagram.Entities[0].Label = "bad\x1b"
		case 2:
			diagram.Entities[0].Attributes = []er.Attribute{{Type: "string", Name: "id", Key: er.Key(99)}}
		case 3:
			diagram.Relationships = []er.Relationship{{From: -1, To: 0, Label: "bad"}}
		case 4:
			diagram.Relationships = []er.Relationship{{From: 0, To: 0, FromMarker: er.ExactlyOne, ToMarker: er.ZeroOrMany, Label: "self"}}
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("ER panicked: %v", recovered)
			}
		}()
		output, err := ER(diagram, Options{MaxWidth: 512, MaxHeight: 512})
		if err != nil && output != "" {
			t.Fatalf("partial output=%q", output)
		}
	})
}
