package flow_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
)

type subgraphWantNode struct {
	id    string
	label string
	scope flow.ScopeRef
}

type subgraphWantScope struct {
	id     string
	label  string
	parent flow.ScopeRef
}

type subgraphWantEdge struct {
	from string
	to   string
}

func TestSubgraphParseSuccessCases(t *testing.T) {
	tests := []struct {
		name   string
		source string
		nodes  []subgraphWantNode
		scopes []subgraphWantScope
		edges  []subgraphWantEdge
	}{
		{
			name: "기본 scope와 root node",
			source: `flowchart LR
subgraph Ingest[수집]
A --> B
end
C`,
			nodes: []subgraphWantNode{
				{id: "A", label: "A", scope: 1},
				{id: "B", label: "B", scope: 1},
				{id: "C", label: "C", scope: flow.RootScope},
			},
			scopes: []subgraphWantScope{
				{id: "Ingest", label: "수집", parent: flow.RootScope},
			},
			edges: []subgraphWantEdge{{from: "A", to: "B"}},
		},
		{
			name: "nested preorder와 parent ref",
			source: `flowchart TD
subgraph Outer[외부]
A
subgraph Inner[内部]
B
end
end`,
			nodes: []subgraphWantNode{
				{id: "A", label: "A", scope: 1},
				{id: "B", label: "B", scope: 2},
			},
			scopes: []subgraphWantScope{
				{id: "Outer", label: "외부", parent: flow.RootScope},
				{id: "Inner", label: "内部", parent: 1},
			},
		},
		{
			name: "node 없는 parent와 nonempty child",
			source: `flowchart TD
subgraph Parent[부모]
subgraph Child[자식]
A
end
end`,
			nodes: []subgraphWantNode{{id: "A", label: "A", scope: 2}},
			scopes: []subgraphWantScope{
				{id: "Parent", label: "부모", parent: flow.RootScope},
				{id: "Child", label: "자식", parent: 1},
			},
		},
		{
			name: "comments와 semicolon",
			source: `%% subgraph Fake
flowchart LR;
subgraph Real[실제] ;
%% end
A;
end ;`,
			nodes:  []subgraphWantNode{{id: "A", label: "A", scope: 1}},
			scopes: []subgraphWantScope{{id: "Real", label: "실제", parent: flow.RootScope}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph, err := flow.Parse(tt.source, flow.DefaultLimits())
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			assertSubgraphGraph(t, graph, tt.nodes, tt.scopes, tt.edges)
		})
	}
}

func TestSubgraphCrossScopeReferencesAndMembership(t *testing.T) {
	t.Run("bare edge reference does not reassign ownership", func(t *testing.T) {
		source := `flowchart LR
subgraph Left
X
end
subgraph Right
Y
X --> Y
end`
		graph, err := flow.Parse(source, flow.DefaultLimits())
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		assertSubgraphGraph(t, graph,
			[]subgraphWantNode{
				{id: "X", label: "X", scope: 1},
				{id: "Y", label: "Y", scope: 2},
			},
			[]subgraphWantScope{
				{id: "Left", label: "Left", parent: flow.RootScope},
				{id: "Right", label: "Right", parent: flow.RootScope},
			},
			[]subgraphWantEdge{{from: "X", to: "Y"}},
		)
	})

	tests := []struct {
		name   string
		source string
		line   int
		column int
	}{
		{
			name: "standalone node cannot join sibling scope",
			source: `flowchart LR
subgraph Left
X
end
subgraph Right
X
end`,
			line: 6, column: 1,
		},
		{
			name: "standalone explicit node cannot join sibling scope",
			source: `flowchart LR
subgraph Left
X
end
subgraph Right
X[X]
end`,
			line: 6, column: 1,
		},
		{
			name: "explicit edge occurrence asserts membership",
			source: `flowchart LR
subgraph Left
X
end
subgraph Right
Y --> X[X]
end`,
			line: 6, column: 7,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertSubgraphParseError(t, tt.source, tt.line, tt.column)
		})
	}
}

func TestSubgraphNodeAndScopeShareOneNamespace(t *testing.T) {
	tests := []struct {
		name   string
		source string
		line   int
		column int
	}{
		{
			name: "node then scope",
			source: `flowchart LR
Shared
subgraph Shared
A
end`,
			line: 3, column: 10,
		},
		{
			name: "scope then node",
			source: `flowchart LR
subgraph Shared
A
end
Shared`,
			line: 5, column: 1,
		},
		{
			name: "scope then implicit edge node",
			source: `flowchart LR
subgraph Shared
A
end
X --> Shared`,
			line: 5, column: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertSubgraphParseError(t, tt.source, tt.line, tt.column)
		})
	}
}

func TestSubgraphDocumentWideEndAndLegacyNodeCompatibility(t *testing.T) {
	errorTests := []struct {
		name   string
		source string
		line   int
		column int
	}{
		{
			name: "scope opener after root end makes earlier end invalid",
			source: `flowchart LR
end
subgraph S
A
end`,
			line: 2, column: 1,
		},
		{
			name: "root end after closed scope is extra",
			source: `flowchart LR
subgraph S
A
end
end`,
			line: 5, column: 1,
		},
		{
			name: "indented root extra end keeps physical column",
			source: `flowchart LR
subgraph S
A
end
  end`,
			line: 5, column: 3,
		},
	}
	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			assertSubgraphParseError(t, tt.source, tt.line, tt.column)
		})
	}

	successTests := []struct {
		name      string
		source    string
		nodeIDs   []string
		firstText string
		edges     int
	}{
		{name: "flat standalone end remains a node", source: "flowchart LR\nend", nodeIDs: []string{"end"}},
		{name: "commented opener keeps document flat", source: "flowchart LR\n%% subgraph S\nend", nodeIDs: []string{"end"}},
		{name: "flat end semicolon remains a node", source: "flowchart LR\nend;", nodeIDs: []string{"end"}},
		{name: "end edge source remains node syntax", source: "flowchart LR\nend --> A", nodeIDs: []string{"end", "A"}, edges: 1},
		{name: "explicit end remains node syntax", source: "flowchart LR\nend[End]", nodeIDs: []string{"end"}, firstText: "End"},
		{name: "subgraph edge source remains node syntax", source: "flowchart LR\nsubgraph --> A", nodeIDs: []string{"subgraph", "A"}, edges: 1},
	}
	for _, tt := range successTests {
		t.Run(tt.name, func(t *testing.T) {
			graph, err := flow.Parse(tt.source, flow.DefaultLimits())
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if len(graph.Subgraphs) != 0 {
				t.Fatalf("Subgraphs=%v, want none", graph.Subgraphs)
			}
			if len(graph.Nodes) != len(tt.nodeIDs) {
				t.Fatalf("nodes=%d, want %d: %#v", len(graph.Nodes), len(tt.nodeIDs), graph.Nodes)
			}
			for index, id := range tt.nodeIDs {
				if graph.Nodes[index].ID != id || graph.Nodes[index].Scope != flow.RootScope {
					t.Fatalf("node %d=%+v, want ID=%q root scope", index, graph.Nodes[index], id)
				}
			}
			if tt.firstText != "" && graph.Nodes[0].Label != tt.firstText {
				t.Fatalf("first label=%q, want %q", graph.Nodes[0].Label, tt.firstText)
			}
			if len(graph.Edges) != tt.edges {
				t.Fatalf("edges=%d, want %d", len(graph.Edges), tt.edges)
			}
		})
	}
}

func TestSubgraphMalformedStructureAndPhysicalColumns(t *testing.T) {
	tests := []struct {
		name   string
		source string
		line   int
		column int
	}{
		{
			name: "unclosed scope anchors opening keyword",
			source: `flowchart LR
subgraph S
A`,
			line: 2, column: 1,
		},
		{
			name: "comment-only leaf is empty",
			source: `flowchart LR
subgraph S
%% node
end
A`,
			line: 4, column: 1,
		},
		{
			name: "empty child is rejected even with parent node",
			source: `flowchart LR
subgraph Outer
A
subgraph Inner
end
end`,
			line: 5, column: 1,
		},
		{
			name: "double semicolon is not end control",
			source: `flowchart LR
subgraph S
A
end;;`,
			line: 4, column: 4,
		},
		{
			name:   "leading spaces contribute to byte column",
			source: "flowchart LR\n  A ==> B",
			line:   2, column: 5,
		},
		{
			name:   "leading tab contributes one byte column",
			source: "flowchart LR\n\tA ==> B",
			line:   2, column: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertSubgraphParseError(t, tt.source, tt.line, tt.column)
		})
	}
}

func TestSubgraphLimitBoundaries(t *testing.T) {
	limits := flow.DefaultLimits()
	if limits.MaxSubgraphs != 32 || limits.MaxSubgraphDepth != 8 {
		t.Fatalf("subgraph limits=(%d,%d), want (32,8)", limits.MaxSubgraphs, limits.MaxSubgraphDepth)
	}

	tests := []struct {
		name       string
		source     string
		wantScopes int
		wantLine   int
		wantColumn int
	}{
		{name: "depth 8 accepted", source: nestedSubgraphSource(8), wantScopes: 8},
		{name: "depth 9 rejected", source: nestedSubgraphSource(9), wantLine: 10, wantColumn: 1},
		{name: "32 scopes accepted", source: siblingSubgraphSource(32), wantScopes: 32},
		{name: "33 scopes rejected", source: siblingSubgraphSource(33), wantLine: 98, wantColumn: 1},
		{
			name:       "64-byte scope ID accepted",
			source:     "flowchart LR\nsubgraph " + strings.Repeat("S", 64) + "\nA\nend",
			wantScopes: 1,
		},
		{
			name:       "65-byte scope ID rejected",
			source:     "flowchart LR\nsubgraph " + strings.Repeat("S", 65) + "\nA\nend",
			wantLine:   2,
			wantColumn: 10,
		},
		{
			name:       "96-cell scope label accepted",
			source:     "flowchart LR\nsubgraph S[" + strings.Repeat("한", 48) + "]\nA\nend",
			wantScopes: 1,
		},
		{
			name:       "98-cell scope label rejected",
			source:     "flowchart LR\nsubgraph S[" + strings.Repeat("한", 49) + "]\nA\nend",
			wantLine:   2,
			wantColumn: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph, err := flow.Parse(tt.source, limits)
			if tt.wantLine > 0 {
				assertSubgraphErrorResult(t, graph, err, tt.wantLine, tt.wantColumn)
				return
			}
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if len(graph.Subgraphs) != tt.wantScopes {
				t.Fatalf("subgraphs=%d, want %d", len(graph.Subgraphs), tt.wantScopes)
			}
		})
	}
}

func TestSubgraphInvalidLimitsNeverPanic(t *testing.T) {
	hugeInt := int(^uint(0) >> 1)
	tests := []struct {
		name              string
		mutate            func(*flow.Limits)
		requireScopedFail bool
	}{
		{
			name:              "negative depth",
			requireScopedFail: true,
			mutate: func(limits *flow.Limits) {
				limits.MaxSubgraphDepth = -1
			},
		},
		{
			name: "huge depth",
			mutate: func(limits *flow.Limits) {
				limits.MaxSubgraphDepth = hugeInt
			},
		},
		{
			name:              "negative subgraph count",
			requireScopedFail: true,
			mutate: func(limits *flow.Limits) {
				limits.MaxSubgraphs = -1
			},
		},
	}
	inputs := []struct {
		name   string
		source string
		scoped bool
	}{
		{name: "flat input", source: "flowchart LR\nA"},
		{name: "scoped input", source: "flowchart LR\nsubgraph S\nA\nend", scoped: true},
	}

	for _, tt := range tests {
		for _, input := range inputs {
			t.Run(tt.name+"/"+input.name, func(t *testing.T) {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Errorf("Parse() panicked for caller-provided Limits: %v", recovered)
					}
				}()
				limits := flow.DefaultLimits()
				tt.mutate(&limits)
				graph, err := flow.Parse(input.source, limits)
				if input.scoped && tt.requireScopedFail {
					assertSubgraphErrorResultType(t, graph, err)
					return
				}
				if err == nil {
					if graph == nil {
						t.Fatal("Parse() succeeded with nil graph")
					}
					return
				}
				assertSubgraphErrorResultType(t, graph, err)
			})
		}
	}
}

func TestSubgraphScopeRefRepresentabilityBoundary(t *testing.T) {
	limits := flow.DefaultLimits()
	limits.MaxNodes = 256
	limits.MaxSubgraphs = 256
	limits.MaxSubgraphDepth = 1

	tests := []struct {
		name       string
		count      int
		wantLine   int
		wantColumn int
	}{
		{name: "255 non-root refs are representable", count: 255},
		{name: "256th opener is rejected before ref wrap", count: 256, wantLine: 767, wantColumn: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph, err := flow.Parse(siblingSubgraphSource(tt.count), limits)
			if tt.wantLine > 0 {
				assertSubgraphErrorResult(t, graph, err, tt.wantLine, tt.wantColumn)
				return
			}
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if len(graph.Subgraphs) != 255 || len(graph.Nodes) != 255 {
				t.Fatalf("counts subgraphs/nodes=%d/%d, want 255/255", len(graph.Subgraphs), len(graph.Nodes))
			}
			if graph.Nodes[254].Scope != flow.ScopeRef(255) {
				t.Fatalf("last representable Scope=%d, want 255", graph.Nodes[254].Scope)
			}
		})
	}
}

func TestSubgraphStatementErrorsReturnNoPartialGraph(t *testing.T) {
	tests := []struct {
		name   string
		source string
		line   int
		column int
	}{
		{
			name: "malformed chain does not expose prefix node or edge",
			source: `flowchart LR
subgraph S
A --> B -->
end`,
			line: 3, column: 12,
		},
		{
			name:   "same-line definition conflict exposes no graph",
			source: "flowchart LR\nA[one] --> A[two]",
			line:   2, column: 12,
		},
		{
			name: "scope symbol in middle of chain exposes no prefix mutation",
			source: `flowchart LR
subgraph S
A
end
X --> S --> Y`,
			line: 5, column: 7,
		},
		{
			name: "explicit membership conflict exposes no new left endpoint",
			source: `flowchart LR
subgraph Left
X
end
subgraph Right
Y --> X[X]
end`,
			line: 6, column: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph, err := flow.Parse(tt.source, flow.DefaultLimits())
			assertSubgraphErrorResult(t, graph, err, tt.line, tt.column)
		})
	}
}

func assertSubgraphGraph(
	t *testing.T,
	graph *flow.Graph,
	wantNodes []subgraphWantNode,
	wantScopes []subgraphWantScope,
	wantEdges []subgraphWantEdge,
) {
	t.Helper()
	if graph == nil {
		t.Fatal("graph=nil")
	}
	if len(graph.Subgraphs) != len(wantScopes) {
		t.Fatalf("subgraphs=%d, want %d: %#v", len(graph.Subgraphs), len(wantScopes), graph.Subgraphs)
	}
	for index, want := range wantScopes {
		got := graph.Subgraphs[index]
		if got.ID != want.id || got.Label != want.label || got.Parent != want.parent {
			t.Fatalf("subgraph %d=%+v, want ID=%q Label=%q Parent=%d", index, got, want.id, want.label, want.parent)
		}
	}
	if len(graph.Nodes) != len(wantNodes) {
		t.Fatalf("nodes=%d, want %d: %#v", len(graph.Nodes), len(wantNodes), graph.Nodes)
	}
	for index, want := range wantNodes {
		got := graph.Nodes[index]
		if got.ID != want.id || got.Label != want.label || got.Scope != want.scope {
			t.Fatalf("node %d=%+v, want ID=%q Label=%q Scope=%d", index, got, want.id, want.label, want.scope)
		}
	}
	if len(graph.Edges) != len(wantEdges) {
		t.Fatalf("edges=%d, want %d: %#v", len(graph.Edges), len(wantEdges), graph.Edges)
	}
	for index, want := range wantEdges {
		got := graph.Edges[index]
		fromID := graph.Nodes[got.From].ID
		toID := graph.Nodes[got.To].ID
		if fromID != want.from || toID != want.to {
			t.Fatalf("edge %d=%s->%s, want %s->%s", index, fromID, toID, want.from, want.to)
		}
	}
}

func assertSubgraphParseError(t *testing.T, source string, line, column int) {
	t.Helper()
	graph, err := flow.Parse(source, flow.DefaultLimits())
	assertSubgraphErrorResult(t, graph, err, line, column)
}

func assertSubgraphErrorResult(t *testing.T, graph *flow.Graph, err error, line, column int) {
	t.Helper()
	if err == nil {
		t.Fatalf("Parse() succeeded, want %d:%d error; graph=%#v", line, column, graph)
	}
	if graph != nil {
		t.Fatalf("Parse() returned partial graph on error: %#v", graph)
	}
	var parseErr *flow.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error type=%T, want *flow.ParseError: %v", err, err)
	}
	if parseErr.Line != line || parseErr.Column != column {
		t.Fatalf("error location=%d:%d, want %d:%d: %v", parseErr.Line, parseErr.Column, line, column, err)
	}
}

func assertSubgraphErrorResultType(t *testing.T, graph *flow.Graph, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("Parse() succeeded, want explicit error; graph=%#v", graph)
	}
	if graph != nil {
		t.Fatalf("Parse() returned partial graph on error: %#v", graph)
	}
	var parseErr *flow.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error type=%T, want *flow.ParseError: %v", err, err)
	}
}

func nestedSubgraphSource(depth int) string {
	var source strings.Builder
	source.WriteString("flowchart TD\n")
	for level := 0; level < depth; level++ {
		fmt.Fprintf(&source, "subgraph S%02d\n", level)
	}
	source.WriteString("A\n")
	for level := 0; level < depth; level++ {
		source.WriteString("end\n")
	}
	return strings.TrimSuffix(source.String(), "\n")
}

func siblingSubgraphSource(count int) string {
	var source strings.Builder
	source.WriteString("flowchart LR\n")
	for index := 0; index < count; index++ {
		fmt.Fprintf(&source, "subgraph S%02d\nN%02d\nend\n", index, index)
	}
	return strings.TrimSuffix(source.String(), "\n")
}
