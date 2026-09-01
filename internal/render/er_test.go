package render

import (
	"errors"
	"fmt"
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

func TestERRendersCanonicalAttributeConstraints(t *testing.T) {
	diagram := &er.Diagram{Entities: []er.Entity{{
		ID: "A", Label: "A",
		Attributes: []er.Attribute{{Type: "string", Name: "email", Key: er.ForeignKey | er.PrimaryKey, Constraint: er.NotNull | er.Unique}},
	}}}
	unicodeOutput, err := ER(diagram, Options{MaxWidth: 128, MaxHeight: 32})
	if err != nil {
		t.Fatal(err)
	}
	asciiOutput, err := ER(diagram, Options{ASCII: true, MaxWidth: 128, MaxHeight: 32})
	if err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{unicodeOutput, asciiOutput} {
		if !strings.Contains(output, "PK FK UNIQUE NOT NULL string email") {
			t.Fatalf("canonical attribute missing:\n%s", output)
		}
	}
}

func TestERAttributeConstraintLimitsAndHostileBits(t *testing.T) {
	validAttribute := er.Attribute{Type: strings.Repeat("a", 64), Name: strings.Repeat("b", 9), Key: er.PrimaryKey | er.ForeignKey, Constraint: er.Unique | er.NotNull}
	if got, err := ER(&er.Diagram{Entities: []er.Entity{{ID: "A", Label: "A", Attributes: []er.Attribute{validAttribute}}}}, Options{MaxWidth: 128, MaxHeight: 16}); err != nil || !strings.Contains(got, er.FormatAttribute(validAttribute)) {
		t.Fatalf("96-cell attribute output=%q err=%v", got, err)
	}
	overwide := validAttribute
	overwide.Name += "b"
	for _, attribute := range []er.Attribute{
		overwide,
		{Type: "string", Name: "id", Constraint: er.Constraint(4)},
		{Type: "string", Name: "id", Constraint: er.Unique | er.Constraint(4)},
		{Type: "string", Name: "id", Key: er.Key(4)},
		{Type: "string", Name: "id", Key: er.PrimaryKey | er.Key(4)},
	} {
		output, err := ER(&er.Diagram{Entities: []er.Entity{{ID: "A", Label: "A", Attributes: []er.Attribute{attribute}}}}, Options{MaxWidth: 512, MaxHeight: 512})
		if output != "" || !errors.Is(err, ErrInvalidER) {
			t.Fatalf("attribute=%+v output=%q err=%v", attribute, output, err)
		}
	}
	attributes := make([]er.Attribute, 32)
	for index := range attributes {
		attributes[index] = er.Attribute{Type: "string", Name: fmt.Sprintf("a%d", index)}
	}
	if _, err := ER(&er.Diagram{Entities: []er.Entity{{ID: "A", Label: "A", Attributes: attributes}}}, Options{MaxWidth: 128, MaxHeight: 64}); err != nil {
		t.Fatalf("32 attributes rejected: %v", err)
	}
	if _, err := ER(&er.Diagram{Entities: []er.Entity{{ID: "A", Label: "A", Attributes: []er.Attribute{validAttribute}}}}, Options{MaxWidth: 99, MaxHeight: 16}); !errors.Is(err, ErrOutputBounds) {
		t.Fatalf("tight width error=%v", err)
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
