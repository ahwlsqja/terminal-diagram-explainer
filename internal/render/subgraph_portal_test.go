package render

import (
	"errors"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
)

func TestScopedPlannerKeepsCrossScopeRouteInsideSharedAncestor(t *testing.T) {
	for _, tt := range []struct {
		name   string
		source string
	}{
		{
			name: "TD nested child to direct sibling",
			source: `flowchart TD
subgraph Outer
subgraph Inner
A[Leaf]
end
B[Sibling]
end
A --> B`,
		},
		{
			name: "LR nested child to direct sibling",
			source: `flowchart LR
subgraph Platform
subgraph Compute
A[Worker]
end
B[(DB)]
end
A --> B`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			graph := parseSubgraphFixture(t, tt.source)
			plan, err := analyzeRanksWithBudget(graph, maxGraphWorkSteps)
			if err != nil {
				t.Fatal(err)
			}
			outer := outerEdgeMask(graph, plan)
			layout, err := placeScoped(graph, plan, outer)
			if err != nil {
				t.Fatal(err)
			}
			routes, err := planScopedOuterRoutes(graph, plan, outer, layout, Options{MaxWidth: 160, MaxHeight: 120})
			if err != nil {
				t.Fatal(err)
			}
			if len(routes) != 1 {
				t.Fatalf("routes=%d, want 1", len(routes))
			}

			sharedAncestor := layout.frames[0]
			for _, segment := range routes[0].segments {
				portals, collinear := scopeBorderPortals(segment, sharedAncestor)
				if collinear || len(portals) != 0 {
					t.Fatalf("route escaped shared ancestor through portals=%v collinear=%t: segment=%+v frame=%+v", portals, collinear, segment, sharedAncestor)
				}
			}
		})
	}
}

func TestScopedPlannerReservesSharedAncestorCorridorsForSiblingFrames(t *testing.T) {
	for _, direction := range []string{"TD", "LR"} {
		t.Run(direction, func(t *testing.T) {
			source := `flowchart ` + direction + `
subgraph Outer
subgraph Left
A1 --> A2
end
subgraph Right
B1 --> B2
end
end
A1 --> B1
A2 --> B2`
			output := renderSubgraphFixture(t, source, Options{MaxWidth: 160, MaxHeight: 120})
			for _, endpoint := range []string{"A1 --> B1", "A2 --> B2"} {
				if !strings.Contains(output, endpoint) {
					t.Fatalf("cross-sibling manifest lost %q:\n%s", endpoint, output)
				}
			}
		})
	}
}

func TestScopedPlannerRendersDenseMixedJunctions(t *testing.T) {
	for _, direction := range []string{"TD", "LR"} {
		t.Run(direction, func(t *testing.T) {
			source := `flowchart ` + direction + `
subgraph Outer
A --> C
A --> D
B --> C
B --> D
end`
			output := renderSubgraphFixture(t, source, Options{MaxWidth: 160, MaxHeight: 120})
			for _, endpoint := range []string{"A --> C", "A --> D", "B --> C", "B --> D"} {
				if !strings.Contains(output, endpoint) {
					t.Fatalf("dense scoped manifest lost %q:\n%s", endpoint, output)
				}
			}
		})
	}
}

func TestScopedPortalValidatorRejectsInvalidLRPortals(t *testing.T) {
	frame := scopeRect{left: 10, top: 10, right: 30, bottom: 30}
	tests := []struct {
		name    string
		graph   *flow.Graph
		segment routeSegment
	}{
		{name: "source exits top", graph: portalSourceGraph(flow.LeftToRight), segment: routeSegment{x1: 20, y1: 20, x2: 20, y2: 9}},
		{name: "source exits left", graph: portalSourceGraph(flow.LeftToRight), segment: routeSegment{x1: 20, y1: 20, x2: 9, y2: 20}},
		{name: "source crosses corner", graph: portalSourceGraph(flow.LeftToRight), segment: routeSegment{x1: 9, y1: 10, x2: 11, y2: 10}},
		{name: "source follows collinear border", graph: portalSourceGraph(flow.LeftToRight), segment: routeSegment{x1: 29, y1: 15, x2: 29, y2: 25}},
		{name: "target enters right", graph: portalTargetGraph(flow.LeftToRight), segment: routeSegment{x1: 31, y1: 20, x2: 20, y2: 20}},
		{name: "target enters corner", graph: portalTargetGraph(flow.LeftToRight), segment: routeSegment{x1: 9, y1: 10, x2: 11, y2: 10}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edge := tt.graph.Edges[0]
			route := feedbackRoute{edgeIndex: 0, segments: []routeSegment{tt.segment}}
			err := validateScopedRouteFrames(tt.graph, edge, route, []scopeRect{frame})
			if !errors.Is(err, ErrLayout) {
				t.Fatalf("portal validation error=%v, want ErrLayout for segment=%+v", err, tt.segment)
			}
		})
	}
}

func TestScopedPortalValidatorRejectsInvalidTDPortals(t *testing.T) {
	frame := scopeRect{left: 10, top: 10, right: 30, bottom: 30}
	tests := []struct {
		name    string
		graph   *flow.Graph
		segment routeSegment
	}{
		{name: "source exits top", graph: portalSourceGraph(flow.TopToBottom), segment: routeSegment{x1: 20, y1: 20, x2: 20, y2: 9}},
		{name: "source exits side", graph: portalSourceGraph(flow.TopToBottom), segment: routeSegment{x1: 20, y1: 20, x2: 9, y2: 20}},
		{name: "source crosses corner", graph: portalSourceGraph(flow.TopToBottom), segment: routeSegment{x1: 10, y1: 28, x2: 10, y2: 31}},
		{name: "source follows collinear border", graph: portalSourceGraph(flow.TopToBottom), segment: routeSegment{x1: 15, y1: 29, x2: 25, y2: 29}},
		{name: "target enters top", graph: portalTargetGraph(flow.TopToBottom), segment: routeSegment{x1: 20, y1: 9, x2: 20, y2: 20}},
		{name: "target enters side", graph: portalTargetGraph(flow.TopToBottom), segment: routeSegment{x1: 9, y1: 20, x2: 20, y2: 20}},
		{name: "target crosses corner", graph: portalTargetGraph(flow.TopToBottom), segment: routeSegment{x1: 10, y1: 31, x2: 10, y2: 28}},
		{name: "target follows collinear border", graph: portalTargetGraph(flow.TopToBottom), segment: routeSegment{x1: 15, y1: 29, x2: 25, y2: 29}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edge := tt.graph.Edges[0]
			route := feedbackRoute{edgeIndex: 0, segments: []routeSegment{tt.segment}}
			err := validateScopedRouteFrames(tt.graph, edge, route, []scopeRect{frame})
			if !errors.Is(err, ErrLayout) {
				t.Fatalf("portal validation error=%v, want ErrLayout for segment=%+v", err, tt.segment)
			}
		})
	}
}

func TestScopedPortalValidatorRejectsUnrelatedFrame(t *testing.T) {
	graph := &flow.Graph{
		Direction: flow.LeftToRight,
		Nodes: []flow.Node{
			{ID: "A", Label: "A", Scope: flow.ScopeRef(1)},
			{ID: "Root", Label: "Root", Scope: flow.RootScope},
		},
		Edges: []flow.Edge{{From: 0, To: 1}},
		Subgraphs: []flow.Subgraph{
			{ID: "Owned", Label: "Owned", Parent: flow.RootScope},
			{ID: "Unrelated", Label: "Unrelated", Parent: flow.RootScope},
		},
	}
	frames := []scopeRect{
		{left: 10, top: 10, right: 30, bottom: 30},
		{left: 40, top: 10, right: 60, bottom: 30},
	}
	route := feedbackRoute{
		edgeIndex: 0,
		segments:  []routeSegment{{x1: 20, y1: 20, x2: 61, y2: 20}},
	}

	err := validateScopedRouteFrames(graph, graph.Edges[0], route, frames)
	if !errors.Is(err, ErrLayout) {
		t.Fatalf("unrelated-frame validation error=%v, want ErrLayout", err)
	}
}

func TestScopedPlannerRoutesUseValidPortals(t *testing.T) {
	for _, direction := range []flow.Direction{flow.LeftToRight, flow.TopToBottom} {
		t.Run(portalDirectionName(direction), func(t *testing.T) {
			graph := portalPlannerGraph(direction)
			plan, err := analyzeRanksWithBudget(graph, maxGraphWorkSteps)
			if err != nil {
				t.Fatal(err)
			}
			outer := outerEdgeMask(graph, plan)
			layout, err := placeScoped(graph, plan, outer)
			if err != nil {
				t.Fatal(err)
			}
			routes, err := planScopedOuterRoutes(graph, plan, outer, layout, Options{MaxWidth: 160, MaxHeight: 120})
			if err != nil {
				t.Fatalf("planner-generated route failed portal validation: %v", err)
			}
			if len(routes) != 1 {
				t.Fatalf("planner routes=%d, want 1", len(routes))
			}
		})
	}
}

func portalSourceGraph(direction flow.Direction) *flow.Graph {
	return &flow.Graph{
		Direction: direction,
		Nodes: []flow.Node{
			{ID: "Inside", Label: "Inside", Scope: flow.ScopeRef(1)},
			{ID: "Root", Label: "Root", Scope: flow.RootScope},
		},
		Edges:     []flow.Edge{{From: 0, To: 1}},
		Subgraphs: []flow.Subgraph{{ID: "S", Label: "S", Parent: flow.RootScope}},
	}
}

func portalTargetGraph(direction flow.Direction) *flow.Graph {
	return &flow.Graph{
		Direction: direction,
		Nodes: []flow.Node{
			{ID: "Root", Label: "Root", Scope: flow.RootScope},
			{ID: "Inside", Label: "Inside", Scope: flow.ScopeRef(1)},
		},
		Edges:     []flow.Edge{{From: 0, To: 1}},
		Subgraphs: []flow.Subgraph{{ID: "S", Label: "S", Parent: flow.RootScope}},
	}
}

func portalPlannerGraph(direction flow.Direction) *flow.Graph {
	return &flow.Graph{
		Direction: direction,
		Nodes: []flow.Node{
			{ID: "A", Label: "A", Scope: flow.ScopeRef(1)},
			{ID: "B", Label: "B", Scope: flow.ScopeRef(2)},
		},
		Edges: []flow.Edge{{From: 0, To: 1, Label: "cross"}},
		Subgraphs: []flow.Subgraph{
			{ID: "Left", Label: "Left", Parent: flow.RootScope},
			{ID: "Right", Label: "Right", Parent: flow.RootScope},
		},
	}
}

func portalDirectionName(direction flow.Direction) string {
	if direction == flow.LeftToRight {
		return "LR right exit and left entry"
	}
	return "TD bottom exit and entry"
}
