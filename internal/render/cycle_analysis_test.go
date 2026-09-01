package render

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

func TestAnalyzeRanksSelfLoop(t *testing.T) {
	graph := &flow.Graph{
		Direction: flow.TopToBottom,
		Nodes:     []flow.Node{{ID: "A", Label: "A"}},
		Edges:     []flow.Edge{{From: 0, To: 0, Label: "again"}},
	}
	plan, err := analyzeRanksWithBudget(graph, maxGraphWorkSteps)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(plan.ranks) != "[0]" || fmt.Sprint(plan.feedback) != "[true]" {
		t.Fatalf("plan=%+v", plan)
	}
}

func TestAnalyzeRanksSourceOrderGreedy(t *testing.T) {
	graph := &flow.Graph{
		Direction: flow.TopToBottom,
		Nodes: []flow.Node{
			{ID: "A", Label: "A"},
			{ID: "B", Label: "B"},
		},
		Edges: []flow.Edge{
			{From: 1, To: 0},
			{From: 0, To: 1},
		},
	}
	plan, err := analyzeRanksWithBudget(graph, maxGraphWorkSteps)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(plan.feedback) != "[false true]" {
		t.Fatalf("feedback=%v, want [false true]", plan.feedback)
	}
	if fmt.Sprint(plan.ranks) != "[1 0]" {
		t.Fatalf("ranks=%v, want [1 0]", plan.ranks)
	}
}

func TestAnalyzeRanksCycleWithTail(t *testing.T) {
	graph := &flow.Graph{
		Direction: flow.TopToBottom,
		Nodes: []flow.Node{
			{ID: "A", Label: "A"},
			{ID: "B", Label: "B"},
			{ID: "C", Label: "C"},
			{ID: "D", Label: "D"},
		},
		Edges: []flow.Edge{
			{From: 0, To: 1},
			{From: 1, To: 2},
			{From: 2, To: 0, Label: "retry"},
			{From: 2, To: 3},
		},
	}
	plan, err := analyzeRanksWithBudget(graph, maxGraphWorkSteps)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(plan.feedback) != "[false false true false]" {
		t.Fatalf("feedback=%v", plan.feedback)
	}
	if fmt.Sprint(plan.ranks) != "[0 1 2 3]" {
		t.Fatalf("ranks=%v", plan.ranks)
	}
	assertForwardDAG(t, graph, plan)
}

func TestAnalyzeRanksParallelCycleEdges(t *testing.T) {
	graph := &flow.Graph{
		Direction: flow.LeftToRight,
		Nodes:     []flow.Node{{ID: "A", Label: "A"}, {ID: "B", Label: "B"}},
		Edges: []flow.Edge{
			{From: 0, To: 1},
			{From: 0, To: 1},
			{From: 1, To: 0},
		},
	}
	plan, err := analyzeRanksWithBudget(graph, maxGraphWorkSteps)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(plan.feedback) != "[false false true]" {
		t.Fatalf("feedback=%v", plan.feedback)
	}
}

func TestAnalyzeRanksBudgetBoundary(t *testing.T) {
	graph := &flow.Graph{
		Direction: flow.TopToBottom,
		Nodes:     []flow.Node{{ID: "A", Label: "A"}, {ID: "B", Label: "B"}},
		Edges:     []flow.Edge{{From: 1, To: 0}, {From: 0, To: 1}},
	}
	if _, err := analyzeRanksWithBudget(graph, 24); !errors.Is(err, ErrWorkBudget) {
		t.Fatalf("budget 24 error=%v, want ErrWorkBudget", err)
	}
	if _, err := analyzeRanksWithBudget(graph, 25); err != nil {
		t.Fatalf("budget 25 rejected: %v", err)
	}
}

func TestAnalyzeRanksRejectsMalformedEdges(t *testing.T) {
	tests := []flow.Edge{
		{From: -1, To: 0},
		{From: 0, To: -1},
		{From: 1, To: 0},
		{From: 0, To: 1},
	}
	for _, edge := range tests {
		graph := &flow.Graph{
			Direction: flow.TopToBottom,
			Nodes:     []flow.Node{{ID: "A", Label: "A"}},
			Edges:     []flow.Edge{edge},
		}
		if _, err := analyzeRanksWithBudget(graph, maxGraphWorkSteps); !errors.Is(err, ErrInvalidGraph) {
			t.Fatalf("edge=%+v error=%v, want ErrInvalidGraph", edge, err)
		}
	}
}

func TestCycleRenderingDirectionsAndLegend(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{name: "LR", header: "flowchart LR"},
		{name: "TD", header: "flowchart TD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := tt.header + "\nA[Request] --> B[Process]\nB -.->|retry| A\nB --> C[Done]"
			graph := mustParseGraph(t, source)
			output := mustRender(t, graph, Options{MaxWidth: 120, MaxHeight: 80})
			for _, want := range []string{"Request", "Process", "Done", "feedback:", "F01 B -.-> A |retry|"} {
				if !strings.Contains(output, want) {
					t.Fatalf("missing %q in:\n%s", want, output)
				}
			}
			if strings.Count(output, "retry") != 1 {
				t.Fatalf("feedback label count=%d:\n%s", strings.Count(output, "retry"), output)
			}
			assertOutputWithinLimits(t, output, 120, 80)
		})
	}
}

func TestTDCycleOutputHasNoLeadingBlankLines(t *testing.T) {
	source := `flowchart TD
Receive --> Validate
Validate --> Commit
Commit -.-> Backoff
Backoff --> Commit
Commit --> Ack`
	graph, err := flow.Parse(source, flow.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	output, err := Flow(graph, Options{MaxWidth: 120, MaxHeight: 80})
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(output, "\n") {
		t.Fatalf("TD cycle output starts with blank rows: %q", output)
	}
}

func TestSelfLoopRendersUnicodeAndASCII(t *testing.T) {
	graph := mustParseGraph(t, "flowchart TD\nA[Worker] -->|again| A")
	for _, options := range []Options{
		{MaxWidth: 80, MaxHeight: 40},
		{ASCII: true, MaxWidth: 80, MaxHeight: 40},
	} {
		output := mustRender(t, graph, options)
		if !strings.Contains(output, "F01 A --> A |again|") {
			t.Fatalf("self-loop legend missing:\n%s", output)
		}
		if options.ASCII && strings.ContainsAny(output, "┌┐└┘─│┼▶▼") {
			t.Fatalf("ASCII output contains Unicode drawing glyph:\n%s", output)
		}
	}
}

func TestCycleOutputIsDeterministic(t *testing.T) {
	graph := mustParseGraph(t, "flowchart TD\nA --> B\nB --> C\nC -.->|retry| A")
	options := Options{MaxWidth: 100, MaxHeight: 80}
	want := mustRender(t, graph, options)
	for iteration := 0; iteration < 256; iteration++ {
		got := mustRender(t, graph, options)
		if got != want {
			t.Fatalf("cycle output changed at iteration %d", iteration)
		}
	}
}

func TestAnalyzeRanksRandomOrderedGraphsAgainstOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(0xc1c1e))
	for iteration := 0; iteration < 1_000; iteration++ {
		nodeCount := 1 + rng.Intn(8)
		edgeCount := rng.Intn(1 + min(24, nodeCount*nodeCount))
		graph := &flow.Graph{Direction: flow.TopToBottom}
		for node := 0; node < nodeCount; node++ {
			id := fmt.Sprintf("N%d", node)
			graph.Nodes = append(graph.Nodes, flow.Node{ID: id, Label: id})
		}
		for edge := 0; edge < edgeCount; edge++ {
			graph.Edges = append(graph.Edges, flow.Edge{
				From: rng.Intn(nodeCount),
				To:   rng.Intn(nodeCount),
			})
		}

		plan, err := analyzeRanksWithBudget(graph, maxGraphWorkSteps)
		if err != nil {
			t.Fatalf("iteration %d analyze error: %v", iteration, err)
		}
		again, err := analyzeRanksWithBudget(graph, maxGraphWorkSteps)
		if err != nil || !reflect.DeepEqual(plan, again) {
			t.Fatalf("iteration %d non-deterministic plan: first=%+v second=%+v err=%v", iteration, plan, again, err)
		}
		assertForwardDAG(t, graph, plan)
		for edgeIndex, edge := range graph.Edges {
			if !plan.feedback[edgeIndex] {
				continue
			}
			if plan.componentOf[edge.From] != plan.componentOf[edge.To] {
				t.Fatalf("iteration %d feedback crosses SCC: edge=%+v components=%v", iteration, edge, plan.componentOf)
			}
			if edge.From != edge.To && !reachableInForward(graph, plan.feedback, edge.To, edge.From) {
				t.Fatalf("iteration %d feedback lacks reverse path: edge=%+v feedback=%v", iteration, edge, plan.feedback)
			}
		}
	}
}

func TestAnalyzeRanksExhaustiveThreeNodeGraphs(t *testing.T) {
	const nodeCount = 3
	for mask := 0; mask < 1<<(nodeCount*nodeCount); mask++ {
		graph := &flow.Graph{Direction: flow.TopToBottom}
		for node := 0; node < nodeCount; node++ {
			id := fmt.Sprintf("N%d", node)
			graph.Nodes = append(graph.Nodes, flow.Node{ID: id, Label: id})
		}
		bit := 0
		for from := 0; from < nodeCount; from++ {
			for to := 0; to < nodeCount; to++ {
				if mask&(1<<bit) != 0 {
					graph.Edges = append(graph.Edges, flow.Edge{From: from, To: to})
				}
				bit++
			}
		}
		plan, err := analyzeRanksWithBudget(graph, maxGraphWorkSteps)
		if err != nil {
			t.Fatalf("mask %09b analyze error: %v", mask, err)
		}
		assertForwardDAG(t, graph, plan)
		for edgeIndex, edge := range graph.Edges {
			if !plan.feedback[edgeIndex] || edge.From == edge.To {
				continue
			}
			if !reachableInForward(graph, plan.feedback, edge.To, edge.From) {
				t.Fatalf("mask %09b feedback lacks reverse path: edge=%+v", mask, edge)
			}
		}
	}
}

func TestAnalyzeRanksMaxSingleSCCWithinBudget(t *testing.T) {
	graph := cycleStressGraph(48, 96)
	plan, err := analyzeRanksWithBudget(graph, maxGraphWorkSteps)
	if err != nil {
		t.Fatal(err)
	}
	assertForwardDAG(t, graph, plan)
	feedbackCount := 0
	for _, value := range plan.feedback {
		if value {
			feedbackCount++
		}
	}
	if feedbackCount == 0 {
		t.Fatal("single SCC produced no feedback edge")
	}
}

func TestFeedbackRoutesStayOutsideNodeInteriors(t *testing.T) {
	for _, source := range []string{
		"flowchart LR\nA --> B\nB --> C\nC --> A\nB --> B",
		"flowchart TD\nA --> B\nB --> C\nC --> A\nB --> B",
	} {
		graph := mustParseGraph(t, source)
		options := Options{MaxWidth: 160, MaxHeight: 120}
		plan, err := analyzeRanksWithBudget(graph, maxGraphWorkSteps)
		if err != nil {
			t.Fatal(err)
		}
		outer := outerEdgeMask(graph, plan)
		placements, err := place(graph, plan.ranks, plan.maxRank, outer, options)
		if err != nil {
			t.Fatal(err)
		}
		if graph.Direction == flow.LeftToRight {
			shiftPlacements(placements, 2, 0)
		} else {
			shiftPlacements(placements, 0, 2)
		}
		routes, err := planOuterRoutes(graph, plan, outer, placements, options)
		if err != nil {
			t.Fatal(err)
		}
		for _, route := range routes {
			for _, segment := range route.segments {
				walkSegment(segment, func(x, y int) {
					for _, node := range placements {
						if pointInsidePlacement(x, y, node) {
							t.Fatalf("route cell (%d,%d) crosses node %+v for source:\n%s", x, y, node, source)
						}
					}
				})
			}
		}
	}
}

func TestFeedbackConnectorAvoidsForwardLabels(t *testing.T) {
	for _, source := range []string{
		"flowchart TD\nA[A]\nB[B]\nQ[Q]\nP[P]\nA --> B\nB -->|loop| A\nQ -->|LLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLL| P",
		"flowchart LR\nA[A]\nB[B]\nQ[Q]\nP[P]\nA --> B\nB -->|loop| A\nQ -->|LLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLL| P",
	} {
		graph := mustParseGraph(t, source)
		options := Options{MaxWidth: 160, MaxHeight: 100}
		plan, placements, routes, canvas := plannedCycleCanvas(t, graph, options)
		outer := outerEdgeMask(graph, plan)
		if err := drawForwardEdges(canvas, graph, placements, plan.ranks, outer); err != nil {
			t.Fatal(err)
		}
		if err := drawOuterRoutes(canvas, routes); err != nil {
			t.Fatal(err)
		}
		for _, route := range routes {
			for _, segment := range route.segments {
				walkSegment(segment, func(x, y int) {
					current := canvas.at(x, y).text
					if !isLine(current) && !(x == route.arrowX && y == route.arrowY) {
						t.Fatalf("feedback connector missing at (%d,%d), cell=%q source:\n%s", x, y, current, source)
					}
				})
			}
		}
	}
}

func TestFeedbackConnectorAvoidsForwardArrowCells(t *testing.T) {
	for _, source := range []string{
		"flowchart LR\nA[Request] --> B[Process]\nB -.->|retry| A\nB --> C[Done]",
		"flowchart LR\nX --> A\nA --> B\nB -->|loop| A\nY --> C",
		"flowchart TD\nX --> A\nA --> B\nB -->|loop| A\nY --> C",
	} {
		graph := mustParseGraph(t, source)
		options := Options{MaxWidth: 160, MaxHeight: 100}
		plan, placements, routes, canvas := plannedCycleCanvas(t, graph, options)
		outer := outerEdgeMask(graph, plan)
		if err := drawForwardEdges(canvas, graph, placements, plan.ranks, outer); err != nil {
			t.Fatal(err)
		}
		if err := drawOuterRoutes(canvas, routes); err != nil {
			t.Fatal(err)
		}
		assertRoutesVisible(t, source, routes, canvas)
	}
}

func TestSkipRankEdgeUsesOuterRouteWithoutCrossingIntermediateNode(t *testing.T) {
	source := "flowchart LR\nA[A]\nB[B]\nC[C]\nD[D]\nA --> B\nB --> A\nB --> D\nC -->|skip| D"
	graph := mustParseGraph(t, source)
	output := mustRender(t, graph, Options{MaxWidth: 160, MaxHeight: 100})
	if strings.Contains(output, "│──B──│") || strings.Contains(output, "|--B--|") {
		t.Fatalf("skip-rank edge crossed intermediate node:\n%s", output)
	}
	if !strings.Contains(output, "routed:") || !strings.Contains(output, "R01 C --> D |skip|") {
		t.Fatalf("skip-rank route legend missing:\n%s", output)
	}
}

func TestLRForwardDoglegAvoidsWideSiblingNode(t *testing.T) {
	wide := strings.Repeat("W", 32)
	source := "flowchart LR\nS[S]\nW[" + wide + "]\nT[T]\nU[U]\nX[X]\nY[Y]\nS --> U\nW --> T\nX --> Y\nY -->|loop| X"
	graph := mustParseGraph(t, source)
	output := mustRender(t, graph, Options{MaxWidth: 180, MaxHeight: 80})
	if strings.Contains(output, wide+"─") || strings.Contains(output, wide+"-") {
		t.Fatalf("forward dogleg leaked into wide sibling node:\n%s", output)
	}
}

func TestFeedbackLegendWorstCaseFitsDefaultWidth(t *testing.T) {
	idA := "A" + strings.Repeat("a", 63)
	idB := "B" + strings.Repeat("b", 63)
	label := strings.Repeat("x", 96)
	graph := &flow.Graph{
		Direction: flow.TopToBottom,
		Nodes: []flow.Node{
			{ID: idA, Label: "A"},
			{ID: idB, Label: "B"},
		},
		Edges: []flow.Edge{
			{From: 0, To: 1},
			{From: 1, To: 0, Label: label, Dashed: true},
		},
	}
	output := mustRender(t, graph, DefaultOptions())
	assertOutputWithinLimits(t, output, 240, 200)
	if !strings.Contains(output, idB+" -.-> "+idA+" |"+label+"|") {
		t.Fatalf("worst-case legend lost content:\n%s", output)
	}
}

func TestCycleSmallCanvasIsBoundsErrorNotInvalidGraph(t *testing.T) {
	graph := mustParseGraph(t, "flowchart TD\nA --> B\nB --> A")
	_, err := Flow(graph, Options{MaxWidth: 7, MaxHeight: 7})
	if !errors.Is(err, ErrOutputBounds) {
		t.Fatalf("error=%v, want ErrOutputBounds", err)
	}
	if errors.Is(err, ErrInvalidGraph) {
		t.Fatalf("valid cycle misclassified as ErrInvalidGraph: %v", err)
	}
}

func TestRendererHardLimitsAreIndependentOfParserCustomLimits(t *testing.T) {
	limits := flow.DefaultLimits()
	limits.MaxNodes = maxRenderNodes + 1
	var source strings.Builder
	source.WriteString("flowchart TD\n")
	for node := 0; node <= maxRenderNodes; node++ {
		source.WriteString(fmt.Sprintf("N%d\n", node))
	}
	graph, err := flow.Parse(source.String(), limits)
	if err != nil {
		t.Fatalf("custom parser limit rejected fixture: %v", err)
	}
	if _, err := Flow(graph, Options{MaxWidth: 512, MaxHeight: 512}); !errors.Is(err, ErrInvalidGraph) {
		t.Fatalf("error=%v, want renderer ErrInvalidGraph hard limit", err)
	}
}

func FuzzFlowCyclesNoPanic(f *testing.F) {
	for _, seed := range []string{
		"flowchart TD\nA --> A",
		"flowchart LR\nA --> B\nB -.->|retry| A",
		"flowchart TD\nA --> B --> C\nC --> A\nC --> D",
		"flowchart LR\nA[한글] --> B[事件]\nB --> A",
	} {
		f.Add(seed, false)
		f.Add(seed, true)
	}
	f.Fuzz(func(t *testing.T, source string, ascii bool) {
		graph, err := flow.Parse(source, flow.DefaultLimits())
		if err != nil {
			return
		}
		options := Options{ASCII: ascii, MaxWidth: 240, MaxHeight: 200}
		output, err := Flow(graph, options)
		if err != nil {
			return
		}
		if !utf8.ValidString(output) {
			t.Fatal("renderer returned invalid UTF-8")
		}
		lines := strings.Split(output, "\n")
		if len(lines) > options.MaxHeight {
			t.Fatalf("height=%d limit=%d", len(lines), options.MaxHeight)
		}
		for lineIndex, line := range lines {
			width, widthErr := textcell.Width(line)
			if widthErr != nil || width > options.MaxWidth {
				t.Fatalf("line=%d width=%d err=%v", lineIndex, width, widthErr)
			}
		}
	})
}

func reachableInForward(graph *flow.Graph, feedback []bool, start, target int) bool {
	queue := []int{start}
	seen := make([]bool, len(graph.Nodes))
	seen[start] = true
	for head := 0; head < len(queue); head++ {
		current := queue[head]
		if current == target {
			return true
		}
		for edgeIndex, edge := range graph.Edges {
			if feedback[edgeIndex] || edge.From != current || seen[edge.To] {
				continue
			}
			seen[edge.To] = true
			queue = append(queue, edge.To)
		}
	}
	return false
}

func cycleStressGraph(nodeCount, edgeCount int) *flow.Graph {
	graph := &flow.Graph{Direction: flow.TopToBottom}
	for node := 0; node < nodeCount; node++ {
		id := fmt.Sprintf("N%02d", node)
		graph.Nodes = append(graph.Nodes, flow.Node{ID: id, Label: id})
	}
	for node := 0; node < nodeCount; node++ {
		graph.Edges = append(graph.Edges, flow.Edge{From: node, To: (node + 1) % nodeCount})
	}
	for span := 2; len(graph.Edges) < edgeCount; span++ {
		for from := 0; from+span < nodeCount && len(graph.Edges) < edgeCount; from++ {
			graph.Edges = append(graph.Edges, flow.Edge{From: from, To: from + span})
		}
	}
	return graph
}

func walkSegment(segment routeSegment, visit func(int, int)) {
	if segment.y1 == segment.y2 {
		start, end := segment.x1, segment.x2
		if start > end {
			start, end = end, start
		}
		for x := start; x <= end; x++ {
			visit(x, segment.y1)
		}
		return
	}
	start, end := segment.y1, segment.y2
	if start > end {
		start, end = end, start
	}
	for y := start; y <= end; y++ {
		visit(segment.x1, y)
	}
}

func plannedCycleCanvas(t *testing.T, graph *flow.Graph, options Options) (rankPlan, []placement, []feedbackRoute, *canvas) {
	t.Helper()
	plan, err := analyzeRanksWithBudget(graph, maxGraphWorkSteps)
	if err != nil {
		t.Fatal(err)
	}
	outer := outerEdgeMask(graph, plan)
	placements, err := place(graph, plan.ranks, plan.maxRank, outer, options)
	if err != nil {
		t.Fatal(err)
	}
	if graph.Direction == flow.LeftToRight {
		shiftPlacements(placements, 2, 0)
	} else {
		shiftPlacements(placements, 0, 2)
	}
	routes, err := planOuterRoutes(graph, plan, outer, placements, options)
	if err != nil {
		t.Fatal(err)
	}
	canvas, err := newCanvas(options.MaxWidth, options.MaxHeight, options.ASCII)
	if err != nil {
		t.Fatal(err)
	}
	return plan, placements, routes, canvas
}

func assertRoutesVisible(t *testing.T, source string, routes []feedbackRoute, canvas *canvas) {
	t.Helper()
	for _, route := range routes {
		for _, segment := range route.segments {
			walkSegment(segment, func(x, y int) {
				current := canvas.at(x, y).text
				if !isLine(current) && !(x == route.arrowX && y == route.arrowY) {
					t.Fatalf("outer connector missing at (%d,%d), cell=%q source:\n%s", x, y, current, source)
				}
			})
		}
	}
}

func assertForwardDAG(t *testing.T, graph *flow.Graph, plan rankPlan) {
	t.Helper()
	for edgeIndex, edge := range graph.Edges {
		if plan.feedback[edgeIndex] {
			if edge.From != edge.To && plan.ranks[edge.From] <= plan.ranks[edge.To] {
				t.Fatalf("feedback edge %d is not backward: ranks=%v edge=%+v", edgeIndex, plan.ranks, edge)
			}
			continue
		}
		if plan.ranks[edge.From] >= plan.ranks[edge.To] {
			t.Fatalf("forward edge %d is not forward: ranks=%v edge=%+v", edgeIndex, plan.ranks, edge)
		}
	}
}
