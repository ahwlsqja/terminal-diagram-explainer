package render

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
)

func TestSubgraphValidationRejectsMalformedDirectGraphs(t *testing.T) {
	tests := []struct {
		name  string
		graph func() *flow.Graph
	}{
		{
			name: "node scope is out of range",
			graph: func() *flow.Graph {
				graph := scopedValidationGraph()
				graph.Nodes[0].Scope = flow.ScopeRef(len(graph.Subgraphs) + 1)
				return graph
			},
		},
		{
			name: "parent points forward",
			graph: func() *flow.Graph {
				graph := scopedValidationGraph()
				graph.Subgraphs[0].Parent = flow.ScopeRef(2)
				return graph
			},
		},
		{
			name: "parent points to itself",
			graph: func() *flow.Graph {
				graph := scopedValidationGraph()
				graph.Subgraphs[1].Parent = flow.ScopeRef(2)
				return graph
			},
		},
		{
			name:  "depth exceeds eight",
			graph: nestedScopedGraph(9),
		},
		{
			name: "duplicate subgraph ID",
			graph: func() *flow.Graph {
				graph := scopedValidationGraph()
				graph.Subgraphs[1].ID = graph.Subgraphs[0].ID
				return graph
			},
		},
		{
			name: "duplicate node ID",
			graph: func() *flow.Graph {
				graph := scopedValidationGraph()
				graph.Nodes[1].ID = graph.Nodes[0].ID
				return graph
			},
		},
		{
			name: "node and subgraph IDs collide",
			graph: func() *flow.Graph {
				graph := scopedValidationGraph()
				graph.Subgraphs[0].ID = graph.Nodes[0].ID
				return graph
			},
		},
		{
			name: "empty subtree",
			graph: func() *flow.Graph {
				graph := scopedValidationGraph()
				graph.Subgraphs = append(graph.Subgraphs, flow.Subgraph{
					ID:     "Empty",
					Label:  "Empty",
					Parent: flow.RootScope,
				})
				return graph
			},
		},
		{
			name:  "subgraph count exceeds thirty two",
			graph: scopedCountGraph(33),
		},
		{
			name: "subgraph ID exceeds sixty four bytes",
			graph: func() *flow.Graph {
				graph := scopedValidationGraph()
				graph.Subgraphs[0].ID = strings.Repeat("S", 65)
				return graph
			},
		},
		{
			name: "wide subgraph title exceeds ninety six cells",
			graph: func() *flow.Graph {
				graph := scopedValidationGraph()
				graph.Subgraphs[0].Label = strings.Repeat("한", 49)
				return graph
			},
		},
		{
			name: "hostile subgraph title contains terminal escape",
			graph: func() *flow.Graph {
				graph := scopedValidationGraph()
				graph.Subgraphs[0].Label = "safe\x1b[31munsafe"
				return graph
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertScopedFlowErrorNoPanic(t, test.graph(), Options{MaxWidth: 512, MaxHeight: 512}, ErrInvalidGraph)
		})
	}
}

func TestSubgraphParentWithOwnedDescendantIsNotEmpty(t *testing.T) {
	graph := &flow.Graph{
		Direction: flow.TopToBottom,
		Nodes: []flow.Node{
			{ID: "DB", Label: "Store", Scope: flow.ScopeRef(2)},
		},
		Subgraphs: []flow.Subgraph{
			{ID: "Backend", Label: "Backend", Parent: flow.RootScope},
			{ID: "Storage", Label: "Storage", Parent: flow.ScopeRef(1)},
		},
	}

	output, err := Flow(graph, Options{MaxWidth: 120, MaxHeight: 80})
	if err != nil {
		t.Fatalf("parent with an owned descendant rejected: %v", err)
	}
	for _, label := range []string{"Backend", "Storage", "Store"} {
		if !strings.Contains(output, label) {
			t.Fatalf("missing %q in scoped output:\n%s", label, output)
		}
	}
}

func TestSubgraphGeometryFailuresAreTypedAndDoNotPanic(t *testing.T) {
	graph := representativeScopedGraph()

	t.Run("small canvas", func(t *testing.T) {
		assertScopedFlowErrorNoPanic(t, graph, Options{MaxWidth: 8, MaxHeight: 8}, ErrOutputBounds)
	})

	t.Run("canvas hard cap preflight", func(t *testing.T) {
		assertScopedFlowErrorNoPanic(t, graph, Options{MaxWidth: 513, MaxHeight: 512}, ErrOutputBounds)
	})
}

func TestSubgraphRepresentativeAllocationIsBounded(t *testing.T) {
	graph := representativeScopedGraph()
	options := Options{MaxWidth: 512, MaxHeight: 512}

	output, err := Flow(graph, options)
	if err != nil {
		t.Fatalf("representative scoped graph failed: %v", err)
	}
	for _, label := range []string{"Service boundary", "Data boundary", "Async boundary"} {
		assertScopedFrameLabel(t, output, label)
	}

	allocations := testing.AllocsPerRun(20, func() {
		if _, renderErr := Flow(graph, options); renderErr != nil {
			panic(renderErr)
		}
	})
	if allocations > 2_500 {
		t.Fatalf("scoped allocations/run = %.0f, limit = 2500", allocations)
	}
	t.Logf("scoped allocations/run: %.0f", allocations)
}

func scopedValidationGraph() *flow.Graph {
	return &flow.Graph{
		Direction: flow.TopToBottom,
		Nodes: []flow.Node{
			{ID: "API", Label: "API", Scope: flow.ScopeRef(1)},
			{ID: "DB", Label: "DB", Scope: flow.ScopeRef(2)},
		},
		Edges: []flow.Edge{{From: 0, To: 1}},
		Subgraphs: []flow.Subgraph{
			{ID: "Backend", Label: "Backend", Parent: flow.RootScope},
			{ID: "Storage", Label: "Storage", Parent: flow.ScopeRef(1)},
		},
	}
}

func nestedScopedGraph(depth int) func() *flow.Graph {
	return func() *flow.Graph {
		subgraphs := make([]flow.Subgraph, depth)
		for index := range subgraphs {
			parent := flow.RootScope
			if index > 0 {
				parent = flow.ScopeRef(index)
			}
			subgraphs[index] = flow.Subgraph{
				ID:     fmt.Sprintf("S%02d", index+1),
				Label:  fmt.Sprintf("Scope %02d", index+1),
				Parent: parent,
			}
		}
		return &flow.Graph{
			Direction: flow.TopToBottom,
			Nodes: []flow.Node{
				{ID: "Leaf", Label: "Leaf", Scope: flow.ScopeRef(depth)},
			},
			Subgraphs: subgraphs,
		}
	}
}

func scopedCountGraph(count int) func() *flow.Graph {
	return func() *flow.Graph {
		graph := &flow.Graph{Direction: flow.TopToBottom}
		for index := 0; index < count; index++ {
			graph.Subgraphs = append(graph.Subgraphs, flow.Subgraph{
				ID:     fmt.Sprintf("S%02d", index+1),
				Label:  fmt.Sprintf("Scope %02d", index+1),
				Parent: flow.RootScope,
			})
			graph.Nodes = append(graph.Nodes, flow.Node{
				ID:    fmt.Sprintf("N%02d", index+1),
				Label: fmt.Sprintf("Node %02d", index+1),
				Scope: flow.ScopeRef(index + 1),
			})
		}
		return graph
	}
}

func representativeScopedGraph() *flow.Graph {
	return &flow.Graph{
		Direction: flow.LeftToRight,
		Nodes: []flow.Node{
			{ID: "API", Label: "API", Scope: flow.ScopeRef(1)},
			{ID: "Worker", Label: "Worker", Scope: flow.ScopeRef(1)},
			{ID: "DB", Label: "DB", Scope: flow.ScopeRef(2)},
			{ID: "Queue", Label: "Queue", Scope: flow.ScopeRef(3)},
			{ID: "Done", Label: "Done", Scope: flow.RootScope},
		},
		Edges: []flow.Edge{
			{From: 0, To: 1},
			{From: 1, To: 2, Label: "store"},
			{From: 1, To: 3, Dashed: true},
			{From: 3, To: 1, Label: "retry", Dashed: true},
			{From: 2, To: 4},
		},
		Subgraphs: []flow.Subgraph{
			{ID: "Service", Label: "Service boundary", Parent: flow.RootScope},
			{ID: "Data", Label: "Data boundary", Parent: flow.ScopeRef(1)},
			{ID: "Async", Label: "Async boundary", Parent: flow.RootScope},
		},
	}
}

func assertScopedFlowErrorNoPanic(t *testing.T, graph *flow.Graph, options Options, target error) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Flow() panicked: %v", recovered)
		}
	}()

	output, err := Flow(graph, options)
	if !errors.Is(err, target) {
		t.Fatalf("Flow() error = %v, want errors.Is(_, %v)", err, target)
	}
	if output != "" {
		t.Fatalf("Flow() returned partial output on error: %q", output)
	}
	if target != ErrInvalidGraph && errors.Is(err, ErrInvalidGraph) {
		t.Fatalf("Flow() misclassified %v as ErrInvalidGraph: %v", target, err)
	}
}

func assertScopedFrameLabel(t *testing.T, output, label string) {
	t.Helper()
	if strings.Count(output, label) != 1 {
		t.Fatalf("subgraph label %q count != 1:\n%s", label, output)
	}
	for _, line := range strings.Split(output, "\n") {
		labelIndex := strings.Index(line, label)
		if labelIndex < 0 {
			continue
		}
		before := line[:labelIndex]
		after := line[labelIndex+len(label):]
		if strings.ContainsAny(before, "|│┃║") && strings.ContainsAny(after, "|│┃║") {
			return
		}
	}
	t.Fatalf("subgraph label %q is not inside vertical frame borders:\n%s", label, output)
}
