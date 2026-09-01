package er_test

import (
	"errors"
	"fmt"
	"math"
	"slices"
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

func TestParseERAttributeConstraintsAllMarkerPermutations(t *testing.T) {
	markers := []string{"PK", "FK", "UNIQUE", "NOT NULL"}
	var permutations func([]string, []string)
	permutations = func(prefix, remaining []string) {
		if len(remaining) == 0 {
			source := "erDiagram\nA{\nstring value " + strings.Join(prefix, " ") + "\n}"
			diagram, err := er.Parse(source, er.DefaultLimits())
			if err != nil {
				t.Fatalf("markers=%q err=%v", prefix, err)
			}
			attribute := diagram.Entities[0].Attributes[0]
			if attribute.Key != er.PrimaryKey|er.ForeignKey || attribute.Constraint != er.Unique|er.NotNull {
				t.Fatalf("markers=%q attribute=%+v", prefix, attribute)
			}
			if got, want := er.FormatAttribute(attribute), "PK FK UNIQUE NOT NULL string value"; got != want {
				t.Fatalf("FormatAttribute()=%q want %q", got, want)
			}
			return
		}
		for index, marker := range remaining {
			next := append(append([]string{}, remaining[:index]...), remaining[index+1:]...)
			permutations(append(prefix, marker), next)
		}
	}
	permutations(nil, markers)
}

func TestParseERAttributeConstraintBitCombinationsWithoutInference(t *testing.T) {
	markers := []struct {
		text       string
		key        er.Key
		constraint er.Constraint
	}{
		{text: "PK", key: er.PrimaryKey},
		{text: "FK", key: er.ForeignKey},
		{text: "UNIQUE", constraint: er.Unique},
		{text: "NOT NULL", constraint: er.NotNull},
	}
	for mask := 0; mask < 1<<len(markers); mask++ {
		parts := []string{"string", "value"}
		var wantKey er.Key
		var wantConstraint er.Constraint
		for index := len(markers) - 1; index >= 0; index-- {
			if mask&(1<<index) == 0 {
				continue
			}
			parts = append(parts, markers[index].text)
			wantKey |= markers[index].key
			wantConstraint |= markers[index].constraint
		}
		diagram, err := er.Parse("erDiagram\nA{\n"+strings.Join(parts, " ")+"\n}", er.DefaultLimits())
		if err != nil {
			t.Fatalf("mask=%04b error=%v", mask, err)
		}
		attribute := diagram.Entities[0].Attributes[0]
		if attribute.Key != wantKey || attribute.Constraint != wantConstraint {
			t.Fatalf("mask=%04b attribute=%+v want key=%v constraint=%v", mask, attribute, wantKey, wantConstraint)
		}
	}
}

func TestParseERAttributeConstraintMalformedMarkerColumns(t *testing.T) {
	tests := []struct {
		name, attribute string
		column          int
	}{
		{"standalone NOT", "string value NOT", 14},
		{"standalone NULL", "string value NULL", 14},
		{"reversed", "string value NULL NOT", 14},
		{"NOT PK", "string value NOT PK", 18},
		{"duplicate PK", "string value PK PK", 17},
		{"duplicate UNIQUE", "string value UNIQUE UNIQUE", 21},
		{"duplicate NOT NULL", "string value NOT NULL NOT NULL", 23},
		{"lowercase marker", "string value unique", 14},
		{"mixed marker", "string value Not", 14},
		{"unknown DEFAULT", "string value DEFAULT", 14},
		{"unknown CHECK", "string value CHECK", 14},
		{"unknown NOTNULL", "string value NOTNULL", 14},
		{"extra", "string value PK FK UNIQUE NOT NULL PK", 36},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := er.Parse("erDiagram\nA{\n"+test.attribute+"\n}", er.DefaultLimits())
			var parseErr *er.ParseError
			if !errors.As(err, &parseErr) || parseErr.Line != 3 || parseErr.Column != test.column {
				t.Fatalf("error=%v parse=%+v want column=%d", err, parseErr, test.column)
			}
		})
	}
}

func TestParseERAttributeTypeAndNameMayUseLowercaseConstraintWords(t *testing.T) {
	diagram, err := er.Parse("erDiagram\nA{\nunique not\nnull value\n}", er.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if got := diagram.Entities[0].Attributes; len(got) != 2 || got[0].Type != "unique" || got[0].Name != "not" || got[1].Type != "null" {
		t.Fatalf("attributes=%#v", got)
	}
}

func TestParseERConstraintAttributeCountAndWidthLimits(t *testing.T) {
	var source strings.Builder
	source.WriteString("erDiagram\nA{\n")
	for index := 0; index < 32; index++ {
		fmt.Fprintf(&source, "string a%d UNIQUE NOT NULL\n", index)
	}
	source.WriteString("}")
	diagram, err := er.Parse(source.String(), er.DefaultLimits())
	if err != nil || len(diagram.Entities[0].Attributes) != 32 {
		t.Fatalf("32 constrained attributes diagram=%#v err=%v", diagram, err)
	}

	tooMany := strings.Replace(source.String(), "\n}", "\nstring overflow UNIQUE NOT NULL\n}", 1)
	if diagram, err := er.Parse(tooMany, er.DefaultLimits()); err == nil || diagram != nil {
		t.Fatalf("33 constrained attributes accepted: %#v %v", diagram, err)
	}

	exact := "erDiagram\nA{\n" + strings.Repeat("a", 64) + " " + strings.Repeat("b", 9) + " PK FK UNIQUE NOT NULL\n}"
	if diagram, err := er.Parse(exact, er.DefaultLimits()); err != nil || diagram == nil {
		t.Fatalf("96-cell attribute rejected: %#v %v", diagram, err)
	}
	overwide := strings.Replace(exact, strings.Repeat("b", 9), strings.Repeat("b", 10), 1)
	if diagram, err := er.Parse(overwide, er.DefaultLimits()); err == nil || diagram != nil {
		t.Fatalf("97-cell attribute accepted: %#v %v", diagram, err)
	}
}

func TestParseERCompositeTableConstraintsResolveForwardAndSelf(t *testing.T) {
	source := `erDiagram
Order {
  PRIMARY KEY (tenant_id, id)
  UNIQUE (tenant_id, email)
  FOREIGN KEY (tenant_id, account_id) REFERENCES Account(tenant_id, id)
  uuid tenant_id
  uuid id
  uuid account_id
  string email
}
Account {
  uuid tenant_id
  uuid id
  FOREIGN KEY (tenant_id, id) REFERENCES Account(tenant_id, id)
}`
	diagram, err := er.Parse(source, er.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if len(diagram.Relationships) != 0 || len(diagram.Entities[0].TableConstraints) != 3 || len(diagram.Entities[1].TableConstraints) != 1 {
		t.Fatalf("diagram=%#v", diagram)
	}
	for index, want := range []er.TableConstraintKind{er.CompositePrimaryKey, er.CompositeUnique, er.CompositeForeignKey} {
		if got := diagram.Entities[0].TableConstraints[index].Kind; got != want {
			t.Fatalf("constraint %d kind=%v", index, got)
		}
	}
	foreign := diagram.Entities[0].TableConstraints[2]
	if got, want := foreign.Columns, []int{0, 2}; !slices.Equal(got, want) || foreign.Reference == nil || foreign.Reference.Entity != 1 || !slices.Equal(foreign.Reference.Columns, []int{0, 1}) {
		t.Fatalf("foreign=%+v", foreign)
	}
}

func TestParseERCompositeTableConstraintRejectsBadColumns(t *testing.T) {
	for _, source := range []string{
		"erDiagram\nA {\nstring a\nstring b\nPRIMARY KEY (a)\n}",
		"erDiagram\nA {\nstring a\nstring b\nPRIMARY KEY (a, a)\n}",
		"erDiagram\nA {\nstring a\nstring b\nPRIMARY KEY (a, missing)\n}",
		"erDiagram\nA {\nstring a PK\nstring b\nPRIMARY KEY (a, b)\n}",
		"erDiagram\nA {\nstring a\nstring b\nPRIMARY KEY (a, b)\nPRIMARY KEY (a, b)\n}",
		"erDiagram\nA {\nstring a\nstring b\nFOREIGN KEY (a, b) REFERENCES A(a)\n}",
	} {
		diagram, err := er.Parse(source, er.DefaultLimits())
		if err == nil || diagram != nil {
			t.Fatalf("source=%q diagram=%#v err=%v", source, diagram, err)
		}
	}
}

func TestParseERCompositeKeywordsDoNotCaptureAttributePrefixes(t *testing.T) {
	diagram, err := er.Parse("erDiagram\nA{\nUNIQUEtype one\nFOREIGNtype two\nPRIMARYtype three\n}\n", er.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if got := diagram.Entities[0].Attributes; len(got) != 3 || got[0].Type != "UNIQUEtype" || got[1].Type != "FOREIGNtype" || got[2].Type != "PRIMARYtype" {
		t.Fatalf("attributes=%#v", got)
	}
}

func TestParseERCompositeHugeCustomColumnLimitDoesNotPreallocate(t *testing.T) {
	limits := er.DefaultLimits()
	limits.MaxTableConstraintColumns = math.MaxInt
	diagram, err := er.Parse("erDiagram\nA{\nstring a\nstring b\nUNIQUE (a, b)\n}\n", limits)
	if err != nil || diagram == nil {
		t.Fatalf("diagram=%#v err=%v", diagram, err)
	}
}

func TestParseERCompositeTableConstraintLimits(t *testing.T) {
	limits := er.DefaultLimits()
	if limits.MaxTableConstraints != 64 || limits.MaxTableConstraintsPerEntity != 8 || limits.MaxTableConstraintColumns != 8 || limits.MaxTableConstraintCells != 236 {
		t.Fatalf("limits=%+v", limits)
	}
	columns := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	attributes := "string a\nstring b\nstring c\nstring d\nstring e\nstring f\nstring g\nstring h\n"
	if _, err := er.Parse("erDiagram\nA{\n"+attributes+"UNIQUE ("+strings.Join(columns, ", ")+")\n}", limits); err != nil {
		t.Fatal(err)
	}
	if diagram, err := er.Parse("erDiagram\nA{\n"+attributes+"UNIQUE ("+strings.Join(append(columns, "i"), ", ")+")\n}", limits); err == nil || diagram != nil {
		t.Fatalf("9 columns=%#v %v", diagram, err)
	}
}

func TestFormatEntityTableConstraintInvalidIndicesDoNotPanic(t *testing.T) {
	entity := er.Entity{ID: "A", Attributes: []er.Attribute{{Type: "string", Name: "a"}, {Type: "string", Name: "b"}}}
	for _, constraint := range []er.TableConstraint{
		{Kind: er.CompositeUnique, Columns: []int{-1, 1}},
		{Kind: er.CompositeUnique, Columns: []int{0, 2}},
		{Kind: er.CompositeForeignKey, Columns: []int{0, 1}},
		{Kind: er.CompositeForeignKey, Columns: []int{0, 1}, Reference: &er.ForeignReference{Entity: 3, Columns: []int{0, 1}}},
	} {
		if got := er.FormatEntityTableConstraint(entity, constraint, []er.Entity{entity}); got != "" {
			t.Fatalf("invalid constraint formatted: %+v -> %q", constraint, got)
		}
	}
}

func TestParseERCompositeTableConstraintCellBoundary(t *testing.T) {
	first := strings.Repeat("a", 64)
	second := strings.Repeat("b", 64)
	third := strings.Repeat("c", 64)
	within := strings.Repeat("d", 29) // UNIQUE 행은 정확히 236 cells다.
	over := strings.Repeat("d", 30)
	attributes := "string " + first + "\nstring " + second + "\nstring " + third + "\n"
	for _, test := range []struct {
		name, last string
		valid      bool
	}{{"236", within, true}, {"237", over, false}} {
		source := "erDiagram\nA{\n" + attributes + "string " + test.last + "\nUNIQUE (" + first + ", " + second + ", " + third + ", " + test.last + ")\n}"
		diagram, err := er.Parse(source, er.DefaultLimits())
		if test.valid && err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		if !test.valid && (err == nil || diagram != nil) {
			t.Fatalf("%s: diagram=%#v err=%v", test.name, diagram, err)
		}
	}
}

func TestParseERRejectsForbiddenSourceCharacters(t *testing.T) {
	for _, source := range []string{
		"erDiagram\nA{\nstring id\v\n}",
		"erDiagram\nA{\nstring id\x1b\n}",
		"erDiagram\nA{\nstring id\x00\n}",
		"erDiagram\nA{\nstring id\u0085\n}",
		"erDiagram\nA{\nstring\u00a0id\n}",
		"erDiagram\nA{\nstring\u2028id\n}",
		"erDiagram\nA{\nstring\u2029id\n}",
		"erDiagram\n%% comment\u202e\nA{}",
		"erDiagram\nA[\u200dlabel]{}",
		"erDiagram\nA\ufe0f{}",
	} {
		diagram, err := er.Parse(source, er.DefaultLimits())
		if diagram != nil || err == nil {
			t.Fatalf("source=%q diagram=%#v err=%v", source, diagram, err)
		}
	}
}

func FuzzParseNoPanic(f *testing.F) {
	for _, source := range []string{"erDiagram\nA{}", "erDiagram\nA ||--o{ B : x\nA{}\nB{}", "erDiagram\nA{\nstring id UNIQUE NOT NULL\n}", "erDiagram\nA{\nstring a\nstring b\nUNIQUE (a, b)\n}", "erDiagram\nA{\nstring a\nstring b\nFOREIGN KEY (a, b) REFERENCES A(a, b)\n}", "erDiagram\n%% \u202e\nA{}", "\xff"} {
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
