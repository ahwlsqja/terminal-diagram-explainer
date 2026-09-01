package er_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/er"
)

func TestERDefaultCountBoundaries(t *testing.T) {
	limits := er.DefaultLimits()
	var entities strings.Builder
	entities.WriteString("erDiagram\n")
	for index := 0; index < limits.MaxEntities; index++ {
		fmt.Fprintf(&entities, "E%d{}\n", index)
	}
	if _, err := er.Parse(entities.String(), limits); err != nil {
		t.Fatalf("%d entities rejected: %v", limits.MaxEntities, err)
	}
	entities.WriteString("Overflow{}\n")
	if diagram, err := er.Parse(entities.String(), limits); err == nil || diagram != nil {
		t.Fatalf("entity limit bypassed: %#v %v", diagram, err)
	}

	var relationships strings.Builder
	relationships.WriteString("erDiagram\n")
	for index := 0; index < limits.MaxRelationships; index++ {
		fmt.Fprintf(&relationships, "A ||--o{ B : R%d\n", index)
	}
	relationships.WriteString("A{}\nB{}\n")
	if _, err := er.Parse(relationships.String(), limits); err != nil {
		t.Fatalf("%d relationships rejected: %v", limits.MaxRelationships, err)
	}
	relationships.WriteString("A ||--o{ B : overflow\n")
	if diagram, err := er.Parse(relationships.String(), limits); err == nil || diagram != nil {
		t.Fatalf("relationship limit bypassed: %#v %v", diagram, err)
	}

	var attributes strings.Builder
	attributes.WriteString("erDiagram\n")
	for entity := 0; entity < limits.MaxEntities; entity++ {
		fmt.Fprintf(&attributes, "E%d{\n", entity)
		for attribute := 0; attribute < limits.MaxAttributes/limits.MaxEntities; attribute++ {
			fmt.Fprintf(&attributes, "string a%d\n", attribute)
		}
		attributes.WriteString("}\n")
	}
	if _, err := er.Parse(attributes.String(), limits); err != nil {
		t.Fatalf("%d attributes rejected: %v", limits.MaxAttributes, err)
	}
}

func TestERInvalidLimitsNeverPanic(t *testing.T) {
	tests := []func(*er.Limits){
		func(l *er.Limits) { l.MaxEntities = 0 },
		func(l *er.Limits) { l.MaxRelationships = -1 },
		func(l *er.Limits) { l.MaxAttributes = 0 },
		func(l *er.Limits) { l.MaxAttributesPerEntity = -1 },
		func(l *er.Limits) { l.MaxLines = 0 },
		func(l *er.Limits) { l.MaxSourceBytes = -1 },
		func(l *er.Limits) { l.MaxIDBytes = 0 },
		func(l *er.Limits) { l.MaxLabelCells = -1 },
		func(l *er.Limits) {
			l.MaxEntities, l.MaxRelationships, l.MaxAttributes = math.MaxInt, math.MaxInt, math.MaxInt
			l.MaxAttributesPerEntity, l.MaxLines, l.MaxSourceBytes = math.MaxInt, math.MaxInt, math.MaxInt
			l.MaxIDBytes, l.MaxLabelCells = math.MaxInt, math.MaxInt
		},
	}
	for index, mutate := range tests {
		t.Run(fmt.Sprintf("case-%d", index), func(t *testing.T) {
			limits := er.DefaultLimits()
			mutate(&limits)
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("Parse panicked: %v", recovered)
				}
			}()
			diagram, err := er.Parse("erDiagram\nA{}", limits)
			if index < len(tests)-1 && (err == nil || diagram != nil) {
				t.Fatalf("invalid limits accepted: %#v %v", diagram, err)
			}
		})
	}
}
