package er_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/er"
)

func TestParseERForwardRelationsAttributesAndAliases(t *testing.T) {
	source := `erDiagram
Customer ||--o{ Order : 주문 생성
Customer[고객] {
  uuid id PK
  string email
}
Order[주문] {
  uuid id PK
  uuid customer_id PK FK
}
Audit[감사] {
}`
	diagram, err := er.Parse(source, er.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if len(diagram.Entities) != 3 || len(diagram.Relationships) != 1 {
		t.Fatalf("diagram=%#v", diagram)
	}
	if diagram.Entities[0].ID != "Customer" || diagram.Entities[0].Label != "고객" || diagram.Entities[2].Label != "감사" {
		t.Fatalf("entities=%#v", diagram.Entities)
	}
	attribute := diagram.Entities[1].Attributes[1]
	if attribute.Key != er.PrimaryKey|er.ForeignKey {
		t.Fatalf("combined key=%v", attribute.Key)
	}
	relation := diagram.Relationships[0]
	if relation.From != 0 || relation.To != 1 || relation.FromMarker != er.ExactlyOne || relation.ToMarker != er.ZeroOrMany || relation.Label != "주문 생성" {
		t.Fatalf("relation=%+v", relation)
	}
}

func TestParseERAllCardinalityCombinationsCompact(t *testing.T) {
	leftTokens := []string{"o|", "||", "}o", "}|"}
	rightTokens := []string{"|o", "||", "o{", "|{"}
	var source strings.Builder
	source.WriteString("erDiagram\n")
	for left := range leftTokens {
		for right := range rightTokens {
			fmt.Fprintf(&source, "A%d%d%s--%sB%d%d: R%d%d\n", left, right, leftTokens[left], rightTokens[right], left, right, left, right)
		}
	}
	for left := range leftTokens {
		for right := range rightTokens {
			fmt.Fprintf(&source, "A%d%d{}\nB%d%d{}\n", left, right, left, right)
		}
	}
	diagram, err := er.Parse(source.String(), er.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if len(diagram.Relationships) != 16 {
		t.Fatalf("relationships=%d", len(diagram.Relationships))
	}
}

func TestParseERSelfAndDuplicateRelationships(t *testing.T) {
	source := `erDiagram
Node ||--o{ Node : parent
Node ||--o{ Node : alternate
Node { uuid id PK }`
	diagram, err := er.Parse(source, er.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if len(diagram.Relationships) != 2 || diagram.Relationships[0].From != diagram.Relationships[0].To {
		t.Fatalf("diagram=%#v", diagram)
	}
}

func TestParseEREntityLabelMayContainRelationshipPunctuation(t *testing.T) {
	diagram, err := er.Parse("erDiagram\nA[foo -- { : bar]{}", er.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if diagram.Entities[0].Label != "foo -- { : bar" {
		t.Fatalf("label=%q", diagram.Entities[0].Label)
	}
	if diagram, err := er.Parse("erDiagram\nA[one][two]{}", er.DefaultLimits()); err == nil || diagram != nil {
		t.Fatalf("multiple bracket label accepted: %#v %v", diagram, err)
	}
}

func TestParseERMalformedRelationshipKeepsRelationshipDiagnostic(t *testing.T) {
	_, err := er.Parse("erDiagram\nA |--o{ B : x\nA{}\nB{}", er.DefaultLimits())
	if err == nil || !strings.Contains(err.Error(), "relationship cardinality") {
		t.Fatalf("error=%v", err)
	}
}

func TestParseERRejectsMalformedInput(t *testing.T) {
	tests := []struct {
		name   string
		source string
		line   int
	}{
		{name: "unknown endpoint", source: "erDiagram\nA ||--|| B : x\nA{}", line: 2},
		{name: "duplicate entity", source: "erDiagram\nA{}\nA{}", line: 3},
		{name: "duplicate label", source: "erDiagram\nA[Same]{}\nB[Same]{}", line: 3},
		{name: "attribute outside", source: "erDiagram\nstring id PK\nA{}", line: 2},
		{name: "relation inside", source: "erDiagram\nA{\nA ||--|| A : x\n}", line: 3},
		{name: "duplicate attr", source: "erDiagram\nA{\nstring id\nuuid id\n}", line: 4},
		{name: "duplicate key", source: "erDiagram\nA{\nstring id PK PK\n}", line: 3},
		{name: "unknown key", source: "erDiagram\nA{\nstring id UK\n}", line: 3},
		{name: "root close", source: "erDiagram\n}\nA{}", line: 2},
		{name: "unclosed", source: "erDiagram\nA{\nstring id", line: 2},
		{name: "bad cardinality", source: "erDiagram\nA |--o{ B : x\nA{}\nB{}", line: 2},
		{name: "empty relation label", source: "erDiagram\nA ||--|| B :\nA{}\nB{}", line: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagram, err := er.Parse(test.source, er.DefaultLimits())
			if err == nil || diagram != nil {
				t.Fatalf("Parse()=%#v %v", diagram, err)
			}
			var parseErr *er.ParseError
			if !errors.As(err, &parseErr) || parseErr.Line != test.line {
				t.Fatalf("error=%v want line=%d", err, test.line)
			}
		})
	}
}

func TestParseERLimits(t *testing.T) {
	limits := er.DefaultLimits()
	if limits.MaxEntities != 32 || limits.MaxRelationships != 64 || limits.MaxAttributes != 192 || limits.MaxAttributesPerEntity != 32 {
		t.Fatalf("limits=%+v", limits)
	}
	limited := limits
	limited.MaxAttributesPerEntity = 1
	if diagram, err := er.Parse("erDiagram\nA{\nstring one\nstring two\n}", limited); err == nil || diagram != nil {
		t.Fatalf("attribute limit bypassed: %#v %v", diagram, err)
	}
}

func FuzzParseNoPanic(f *testing.F) {
	for _, source := range []string{"erDiagram\nA{}", "erDiagram\nA ||--o{ B : x\nA{}\nB{}", "\xff"} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		diagram, err := er.Parse(source, er.DefaultLimits())
		if err != nil {
			if diagram != nil {
				t.Fatalf("partial diagram=%#v", diagram)
			}
			return
		}
		limits := er.DefaultLimits()
		if len(diagram.Entities) == 0 || len(diagram.Entities) > limits.MaxEntities || len(diagram.Relationships) > limits.MaxRelationships {
			t.Fatalf("limits bypassed: %#v", diagram)
		}
	})
}
