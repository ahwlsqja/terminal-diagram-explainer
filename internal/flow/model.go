package flow

type Direction uint8

const (
	LeftToRight Direction = iota
	TopToBottom
)

type Shape uint8

const (
	Process Shape = iota
	Decision
	DataStore
)

type Node struct {
	ID       string
	Label    string
	Shape    Shape
	explicit bool
}

type Edge struct {
	From   int
	To     int
	Label  string
	Dashed bool
}

type Graph struct {
	Direction Direction
	Nodes     []Node
	Edges     []Edge
}

type Limits struct {
	MaxLines      int
	MaxNodes      int
	MaxEdges      int
	MaxIDBytes    int
	MaxLabelCells int
}

func DefaultLimits() Limits {
	return Limits{
		MaxLines:      2048,
		MaxNodes:      48,
		MaxEdges:      96,
		MaxIDBytes:    64,
		MaxLabelCells: 96,
	}
}
