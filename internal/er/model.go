package er

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

type Attribute struct {
	Type string
	Name string
	Key  Key
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
