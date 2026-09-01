package render

import (
	"errors"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/er"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

const erFixture = `erDiagram
Customer ||--o{ Order : 주문 생성
Customer[고객] {
uuid id PK
string email
}
Order[주문] {
uuid id PK
uuid customer_id FK
}
Audit[감사] { uuid id PK }
`

func TestERRendersTablesCardinalityLegendAndDisconnectedEntity(t *testing.T) {
	diagram, err := er.Parse(erFixture, er.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	output, err := ER(diagram, Options{MaxWidth: 160, MaxHeight: 80})
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"고객", "주문", "감사", "PK uuid id", "FK uuid customer_id", "relationships:", "R01 Customer 1 -- 0..N Order |주문 생성|"} {
		if !strings.Contains(output, text) {
			t.Fatalf("text %q missing:\n%s", text, output)
		}
	}
	assertEROutputBounds(t, output, 160, 80)
}

func TestERRendersSelfAndDuplicateRelationshipPorts(t *testing.T) {
	diagram := &er.Diagram{
		Entities: []er.Entity{{ID: "Node", Label: "Node"}},
		Relationships: []er.Relationship{
			{From: 0, To: 0, FromMarker: er.ExactlyOne, ToMarker: er.ZeroOrMany, Label: "parent"},
			{From: 0, To: 0, FromMarker: er.ZeroOrOne, ToMarker: er.OneOrMany, Label: "alternate"},
		},
	}
	output, err := ER(diagram, Options{MaxWidth: 100, MaxHeight: 40})
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"R01 Node 1 -- 0..N Node |parent|", "R02 Node 0..1 -- 1..N Node |alternate|"} {
		if !strings.Contains(output, text) {
			t.Fatalf("missing %q:\n%s", text, output)
		}
	}
}

func TestERASCIIAndDeterminism(t *testing.T) {
	diagram, err := er.Parse(erFixture, er.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	options := Options{ASCII: true, MaxWidth: 160, MaxHeight: 80}
	want, err := ER(diagram, options)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(want, "┌┐└┘├┤─│┄┊┼▶◀") {
		t.Fatalf("ASCII output contains Unicode drawing glyphs:\n%s", want)
	}
	for run := 0; run < 256; run++ {
		got, renderErr := ER(diagram, options)
		if renderErr != nil || got != want {
			t.Fatalf("run=%d err=%v changed=%v", run, renderErr, got != want)
		}
	}
}

func TestERDirectValidationAndBounds(t *testing.T) {
	valid := &er.Diagram{Entities: []er.Entity{{ID: "A", Label: "A"}}}
	invalids := []*er.Diagram{
		nil,
		{},
		{Entities: []er.Entity{{ID: "A", Label: "A"}, {ID: "A", Label: "B"}}},
		{Entities: []er.Entity{{ID: "A", Label: "A", Attributes: []er.Attribute{{Type: "string", Name: "id", Key: er.Key(99)}}}}},
		{Entities: []er.Entity{{ID: "A", Label: "A"}}, Relationships: []er.Relationship{{From: 0, To: 1, Label: "bad"}}},
	}
	for _, diagram := range invalids {
		output, err := ER(diagram, Options{MaxWidth: 512, MaxHeight: 512})
		if !errors.Is(err, ErrInvalidER) || output != "" {
			t.Fatalf("diagram=%#v output=%q err=%v", diagram, output, err)
		}
	}
	if output, err := ER(valid, Options{MaxWidth: 2, MaxHeight: 2}); !errors.Is(err, ErrOutputBounds) || output != "" {
		t.Fatalf("bounds output=%q err=%v", output, err)
	}
}

func assertEROutputBounds(t *testing.T, output string, maxWidth, maxHeight int) {
	t.Helper()
	rows := strings.Split(output, "\n")
	if len(rows) > maxHeight {
		t.Fatalf("height=%d", len(rows))
	}
	for index, row := range rows {
		width, err := textcell.Width(row)
		if err != nil || width > maxWidth || strings.HasSuffix(row, " ") {
			t.Fatalf("row=%d width=%d err=%v text=%q", index, width, err, row)
		}
	}
}
