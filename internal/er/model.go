package er

import "strings"

type Cardinality uint8

const (
	ZeroOrOne Cardinality = iota
	ExactlyOne
	ZeroOrMany
	OneOrMany
)

type Key uint8

const (
	PrimaryKey Key = 1 << iota
	ForeignKey
)

// Constraint는 attribute에 명시된 schema 제약을 나타낸다.
type Constraint uint8

const (
	Unique Constraint = 1 << iota
	NotNull
)

type Attribute struct {
	Type       string
	Name       string
	Key        Key
	Constraint Constraint
}

// FormatAttribute는 parser와 renderer가 공유하는 attribute의 정규 표기다.
// 기존 PK/FK 표기는 호환성을 위해 그대로 유지한다.
func FormatAttribute(attribute Attribute) string {
	parts := make([]string, 0, 6)
	if attribute.Key&PrimaryKey != 0 {
		parts = append(parts, "PK")
	}
	if attribute.Key&ForeignKey != 0 {
		parts = append(parts, "FK")
	}
	if attribute.Constraint&Unique != 0 {
		parts = append(parts, "UNIQUE")
	}
	if attribute.Constraint&NotNull != 0 {
		parts = append(parts, "NOT NULL")
	}
	parts = append(parts, attribute.Type, attribute.Name)
	return strings.Join(parts, " ")
}

type Entity struct {
	ID         string
	Label      string
	Attributes []Attribute
}

type Relationship struct {
	From       int
	To         int
	FromMarker Cardinality
	ToMarker   Cardinality
	Label      string
}

type Diagram struct {
	Entities      []Entity
	Relationships []Relationship
}

type Limits struct {
	MaxSourceBytes         int
	MaxLines               int
	MaxEntities            int
	MaxRelationships       int
	MaxAttributes          int
	MaxAttributesPerEntity int
	MaxIDBytes             int
	MaxLabelCells          int
}

func DefaultLimits() Limits {
	return Limits{
		MaxSourceBytes:         256 * 1024,
		MaxLines:               2048,
		MaxEntities:            32,
		MaxRelationships:       64,
		MaxAttributes:          192,
		MaxAttributesPerEntity: 32,
		MaxIDBytes:             64,
		MaxLabelCells:          96,
	}
}
