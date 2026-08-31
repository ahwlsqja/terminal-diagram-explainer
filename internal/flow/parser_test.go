package flow

import (
	"errors"
	"strings"
	"testing"
)

func TestParseFlowchart(t *testing.T) {
	input := `flowchart LR
Tracker[Browser SDK] --> Worker[Tracker Worker]
Worker -->|valid| Events[(ClickHouse)]
Worker -.->|invalid| Reject{Reject event}`

	graph, err := Parse(input, DefaultLimits())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if graph.Direction != LeftToRight {
		t.Fatalf("direction = %v", graph.Direction)
	}
	if len(graph.Nodes) != 4 || len(graph.Edges) != 3 {
		t.Fatalf("nodes=%d edges=%d", len(graph.Nodes), len(graph.Edges))
	}
	if graph.Nodes[2].Shape != DataStore || graph.Nodes[3].Shape != Decision {
		t.Fatalf("shapes = %v, %v", graph.Nodes[2].Shape, graph.Nodes[3].Shape)
	}
	if !graph.Edges[2].Dashed {
		t.Fatal("expected dashed edge")
	}
}

func TestParseRejectsUnsupportedSyntaxWithoutPanic(t *testing.T) {
	_, err := Parse("flowchart LR\nclassDef foo invalid\nA --> B", DefaultLimits())
	if err == nil {
		t.Fatal("expected error")
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error type = %T", err)
	}
	if parseErr.Line != 2 {
		t.Fatalf("line = %d, want 2", parseErr.Line)
	}
}

func TestParseRejectsTerminalEscape(t *testing.T) {
	_, err := Parse("flowchart LR\nA[safe\x1b[31mred] --> B", DefaultLimits())
	if err == nil || !strings.Contains(err.Error(), "제어 문자") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseEnforcesNodeAndEdgeLimits(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxNodes = 3
	limits.MaxEdges = 2

	if _, err := Parse("flowchart LR\nA --> B --> C --> D", limits); err == nil {
		t.Fatal("expected limit error")
	}
}

func TestParseRejectsConflictingNodeDefinitions(t *testing.T) {
	_, err := Parse("flowchart LR\nA[one] --> B\nA[two] --> C", DefaultLimits())
	if err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestParseRejectsLongID(t *testing.T) {
	_, err := Parse("flowchart LR\n"+strings.Repeat("A", 65)+" --> B", DefaultLimits())
	if err == nil {
		t.Fatal("expected ID limit error")
	}
}

func TestDefaultNodeLimit(t *testing.T) {
	limits := DefaultLimits()
	var accepted strings.Builder
	accepted.WriteString("flowchart TD\n")
	for i := 0; i < limits.MaxNodes-1; i++ {
		accepted.WriteString(limitNodeID(i) + " --> " + limitNodeID(i+1) + "\n")
	}
	if _, err := Parse(accepted.String(), limits); err != nil {
		t.Fatalf("48-node graph rejected: %v", err)
	}
	accepted.WriteString(limitNodeID(limits.MaxNodes-1) + " --> " + limitNodeID(limits.MaxNodes) + "\n")
	if _, err := Parse(accepted.String(), limits); err == nil {
		t.Fatal("49-node graph accepted")
	}
}

func TestDefaultEdgeLimit(t *testing.T) {
	limits := DefaultLimits()
	var source strings.Builder
	source.WriteString("flowchart TD\n")
	for i := 0; i < limits.MaxEdges; i++ {
		source.WriteString("A --> B\n")
	}
	if _, err := Parse(source.String(), limits); err != nil {
		t.Fatalf("96-edge graph rejected: %v", err)
	}
	source.WriteString("A --> B\n")
	if _, err := Parse(source.String(), limits); err == nil {
		t.Fatal("97-edge graph accepted")
	}
}

func TestDefaultLabelCellLimit(t *testing.T) {
	limits := DefaultLimits()
	if _, err := Parse("flowchart TD\nA["+strings.Repeat("한", 48)+"]", limits); err != nil {
		t.Fatalf("96-cell label rejected: %v", err)
	}
	if _, err := Parse("flowchart TD\nA["+strings.Repeat("한", 49)+"]", limits); err == nil {
		t.Fatal("98-cell label accepted")
	}
}

func TestParseRejectsBOM(t *testing.T) {
	if _, err := Parse("\ufeffflowchart TD\nA --> B", DefaultLimits()); err == nil {
		t.Fatal("BOM accepted")
	}
}

func limitNodeID(i int) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	return "N" + string(alphabet[i%len(alphabet)]) + string(alphabet[(i/len(alphabet))%len(alphabet)])
}

func FuzzParseNoPanic(f *testing.F) {
	for _, seed := range []string{
		"flowchart LR\nA --> B",
		"flowchart TD\nA{ok?} -->|yes| B[(store)]",
		"flowchart LR\nclassDef foo invalid",
		"\xff\xfe",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		graph, err := Parse(input, DefaultLimits())
		if err != nil {
			return
		}
		limits := DefaultLimits()
		if len(graph.Nodes) > limits.MaxNodes || len(graph.Edges) > limits.MaxEdges {
			t.Fatalf("limits bypassed: nodes=%d edges=%d", len(graph.Nodes), len(graph.Edges))
		}
	})
}
