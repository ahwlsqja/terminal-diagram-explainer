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

type ScopeRef uint8

const RootScope ScopeRef = 0

type Node struct {
	ID       string
	Label    string
	Shape    Shape
	Scope    ScopeRef
	explicit bool
}

type Subgraph struct {
	ID     string
	Label  string
	Parent ScopeRef
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
	Subgraphs []Subgraph
}

type Limits struct {
	MaxLines         int
	MaxNodes         int
	MaxEdges         int
	MaxSubgraphs     int
	MaxSubgraphDepth int
	MaxIDBytes       int
	MaxLabelCells    int
}

func DefaultLimits() Limits {
	return Limits{
		MaxLines:         2048,
		MaxNodes:         48,
		MaxEdges:         96,
		MaxSubgraphs:     32,
		MaxSubgraphDepth: 8,
		MaxIDBytes:       64,
		MaxLabelCells:    96,
	}
}
