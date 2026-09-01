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
	ID               string
	Label            string
	Attributes       []Attribute
	TableConstraints []TableConstraint
}

// TableConstraintKind는 entity 본문에 독립 행으로 선언되는 복합 제약의 종류다.
// 0은 유효한 종류가 아니므로, 직접 AST를 만들 때도 누락을 발견할 수 있다.
type TableConstraintKind uint8

const (
	CompositePrimaryKey TableConstraintKind = iota + 1
	CompositeUnique
	CompositeForeignKey
)

type ForeignReference struct {
	Entity  int
	Columns []int
}

type TableConstraint struct {
	Kind      TableConstraintKind
	Columns   []int
	Reference *ForeignReference
}

func FormatEntityTableConstraint(entity Entity, constraint TableConstraint, entities []Entity) string {
	columns := make([]string, len(constraint.Columns))
	for index, column := range constraint.Columns {
		if column < 0 || column >= len(entity.Attributes) {
			return ""
		}
		columns[index] = entity.Attributes[column].Name
	}
	switch constraint.Kind {
	case CompositePrimaryKey:
		return "PRIMARY KEY (" + strings.Join(columns, ", ") + ")"
	case CompositeUnique:
		return "UNIQUE (" + strings.Join(columns, ", ") + ")"
	case CompositeForeignKey:
		if constraint.Reference == nil || constraint.Reference.Entity < 0 || constraint.Reference.Entity >= len(entities) {
			return ""
		}
		referenceEntity := entities[constraint.Reference.Entity]
		referenceColumns := make([]string, len(constraint.Reference.Columns))
		for index, column := range constraint.Reference.Columns {
			if column < 0 || column >= len(referenceEntity.Attributes) {
				return ""
			}
			referenceColumns[index] = referenceEntity.Attributes[column].Name
		}
		return "FOREIGN KEY (" + strings.Join(columns, ", ") + ") REFERENCES " + referenceEntity.ID + "(" + strings.Join(referenceColumns, ", ") + ")"
	default:
		return ""
	}
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
	MaxSourceBytes               int
	MaxLines                     int
	MaxEntities                  int
	MaxRelationships             int
	MaxAttributes                int
	MaxAttributesPerEntity       int
	MaxTableConstraints          int
	MaxTableConstraintsPerEntity int
	MaxTableConstraintColumns    int
	MaxTableConstraintCells      int
	MaxIDBytes                   int
	MaxLabelCells                int
}

func DefaultLimits() Limits {
	return Limits{
		MaxSourceBytes:               256 * 1024,
		MaxLines:                     2048,
		MaxEntities:                  32,
		MaxRelationships:             64,
		MaxAttributes:                192,
		MaxAttributesPerEntity:       32,
		MaxTableConstraints:          64,
		MaxTableConstraintsPerEntity: 8,
		MaxTableConstraintColumns:    8,
		MaxTableConstraintCells:      236,
		MaxIDBytes:                   64,
		MaxLabelCells:                96,
	}
}
