package flow_test

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/render"
)

func TestGrammarAcceptsUnambiguousSupportedForms(t *testing.T) {
	source := "%% leading comment\n" +
		" graph\tTB ;\n" +
		"Root_1[\"start\"]-->|ok|Decision{continue?};\n" +
		"Decision -.-> Store[(archive)]\n" +
		"node-with-hyphen"

	graph, err := flow.Parse(source, flow.DefaultLimits())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if graph.Direction != flow.TopToBottom {
		t.Fatalf("direction = %v, want TopToBottom", graph.Direction)
	}
	if len(graph.Nodes) != 4 || len(graph.Edges) != 2 {
		t.Fatalf("nodes=%d edges=%d, want 4 and 2", len(graph.Nodes), len(graph.Edges))
	}
	if graph.Nodes[0].Label != "start" || graph.Nodes[1].Shape != flow.Decision || graph.Nodes[2].Shape != flow.DataStore {
		t.Fatalf("unexpected nodes: %#v", graph.Nodes)
	}
	if graph.Edges[0].Label != "ok" || graph.Edges[0].Dashed || !graph.Edges[1].Dashed {
		t.Fatalf("unexpected edges: %#v", graph.Edges)
	}
}

func TestGrammarRejectsAmbiguousOrUnsupportedForms(t *testing.T) {
	tests := map[string]string{
		"extra header token":       "flowchart LR extra\nA",
		"unsupported direction":    "flowchart RL\nA",
		"second header":            "flowchart LR\ngraph TD",
		"inline comment":           "flowchart LR\nA --> B %% hidden",
		"unsupported round node":   "flowchart LR\nA(label)",
		"unsupported thick arrow":  "flowchart LR\nA ==> B",
		"unsupported dotted arrow": "flowchart LR\nA -.->> B",
		"unsupported HTML break":   "flowchart TD\nA[\"line<br/>break\"]",
		"tab around arrow":         "flowchart LR\nA\t-->\tB",
		"non ASCII identifier":     "flowchart LR\n가 --> B",
		"digit-leading identifier": "flowchart LR\n1A --> B",
		"empty statement":          "flowchart LR\n;",
	}

	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			assertParseError(t, source, "")
		})
	}
}

func TestGrammarDisambiguatesCompactArrowsFromHyphenatedIDs(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "solid", source: "flowchart LR\nA-->B"},
		{name: "dashed", source: "flowchart LR\nA-.->B"},
		{name: "hyphenated source", source: "flowchart LR\nnode-1-->node-2"},
		{name: "explicit source", source: "flowchart LR\nA[start]-->B[end]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph, err := flow.Parse(tt.source, flow.DefaultLimits())
			if err != nil {
				t.Fatalf("compact supported arrow rejected: %v", err)
			}
			if len(graph.Edges) != 1 {
				t.Fatalf("edges=%d, want 1", len(graph.Edges))
			}
		})
	}
}

func TestExactParserLimits(t *testing.T) {
	t.Run("lines", func(t *testing.T) {
		limits := flow.DefaultLimits()
		lines := []string{"flowchart LR", "A"}
		for len(lines) < limits.MaxLines {
			lines = append(lines, "%% padding")
		}
		if _, err := flow.Parse(strings.Join(lines, "\n"), limits); err != nil {
			t.Fatalf("exactly %d lines rejected: %v", limits.MaxLines, err)
		}
		assertParseError(t, strings.Join(append(lines, "%% over"), "\n"), "행 수 제한 초과")
	})

	t.Run("nodes", func(t *testing.T) {
		limits := flow.DefaultLimits()
		lines := []string{"flowchart LR"}
		for i := 0; i < limits.MaxNodes; i++ {
			lines = append(lines, propertyNodeID(i))
		}
		graph, err := flow.Parse(strings.Join(lines, "\n"), limits)
		if err != nil {
			t.Fatalf("exactly %d nodes rejected: %v", limits.MaxNodes, err)
		}
		if len(graph.Nodes) != limits.MaxNodes {
			t.Fatalf("nodes=%d, want %d", len(graph.Nodes), limits.MaxNodes)
		}
		assertParseError(t, strings.Join(append(lines, propertyNodeID(limits.MaxNodes)), "\n"), "노드 수 제한 초과")
	})

	t.Run("edges", func(t *testing.T) {
		limits := flow.DefaultLimits()
		lines := []string{"flowchart LR"}
		for i := 0; i < limits.MaxEdges; i++ {
			lines = append(lines, "A --> B")
		}
		graph, err := flow.Parse(strings.Join(lines, "\n"), limits)
		if err != nil {
			t.Fatalf("exactly %d edges rejected: %v", limits.MaxEdges, err)
		}
		if len(graph.Edges) != limits.MaxEdges {
			t.Fatalf("edges=%d, want %d", len(graph.Edges), limits.MaxEdges)
		}
		assertParseError(t, strings.Join(append(lines, "A --> B"), "\n"), "edge 수 제한 초과")
	})

	t.Run("identifier bytes", func(t *testing.T) {
		limits := flow.DefaultLimits()
		if _, err := flow.Parse("flowchart LR\n"+strings.Repeat("A", limits.MaxIDBytes), limits); err != nil {
			t.Fatalf("exactly %d ID bytes rejected: %v", limits.MaxIDBytes, err)
		}
		assertParseError(t, "flowchart LR\n"+strings.Repeat("A", limits.MaxIDBytes+1), "노드 ID 길이 제한 초과")
	})

	t.Run("node label cells", func(t *testing.T) {
		limits := flow.DefaultLimits()
		if _, err := flow.Parse("flowchart LR\nA["+strings.Repeat("x", limits.MaxLabelCells)+"]", limits); err != nil {
			t.Fatalf("exactly %d node-label cells rejected: %v", limits.MaxLabelCells, err)
		}
		assertParseError(t, "flowchart LR\nA["+strings.Repeat("x", limits.MaxLabelCells+1)+"]", "label 폭 제한 초과")
	})

	t.Run("edge label cells", func(t *testing.T) {
		limits := flow.DefaultLimits()
		if _, err := flow.Parse("flowchart LR\nA -->|"+strings.Repeat("x", limits.MaxLabelCells)+"| B", limits); err != nil {
			t.Fatalf("exactly %d edge-label cells rejected: %v", limits.MaxLabelCells, err)
		}
		assertParseError(t, "flowchart LR\nA -->|"+strings.Repeat("x", limits.MaxLabelCells+1)+"| B", "label 폭 제한 초과")
	})
}

func TestTrailingNewlineDoesNotCreatePhantomLine(t *testing.T) {
	limits := flow.DefaultLimits()
	lines := []string{"flowchart LR", "A"}
	for len(lines) < limits.MaxLines {
		lines = append(lines, "%% padding")
	}
	for name, separator := range map[string]string{"LF": "\n", "CRLF": "\r\n"} {
		t.Run(name, func(t *testing.T) {
			source := strings.Join(lines, separator) + separator
			if _, err := flow.Parse(source, limits); err != nil {
				t.Fatalf("newline-terminated %d-line input rejected: %v", limits.MaxLines, err)
			}
		})
	}
}

func TestDuplicateDefinitionsAreCanonicalAndBounded(t *testing.T) {
	t.Run("implicit upgraded by explicit", func(t *testing.T) {
		graph, err := flow.Parse("flowchart LR\nA --> B\nA[canonical] --> C\nA --> D", flow.DefaultLimits())
		if err != nil {
			t.Fatal(err)
		}
		if len(graph.Nodes) != 4 || graph.Nodes[0].ID != "A" || graph.Nodes[0].Label != "canonical" {
			t.Fatalf("unexpected canonical nodes: %#v", graph.Nodes)
		}
	})

	t.Run("identical explicit definitions", func(t *testing.T) {
		graph, err := flow.Parse("flowchart LR\nA[one] --> B\nA[\"one\"] --> C\nA[one]", flow.DefaultLimits())
		if err != nil {
			t.Fatal(err)
		}
		if len(graph.Nodes) != 3 {
			t.Fatalf("nodes=%d, want 3", len(graph.Nodes))
		}
	})

	for _, tt := range []struct {
		name   string
		source string
	}{
		{name: "different label", source: "flowchart LR\nA[one]\nA[two]"},
		{name: "different shape", source: "flowchart LR\nA[one]\nA{one}"},
		{name: "same chain conflict", source: "flowchart LR\nA[one] --> A[two]"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assertParseError(t, tt.source, "정의가 충돌함")
		})
	}

	t.Run("duplicates do not consume node limit", func(t *testing.T) {
		limits := flow.DefaultLimits()
		limits.MaxNodes = 2
		graph, err := flow.Parse("flowchart LR\nA[x] --> B[y]\nA[x] --> B[y]\nA --> B", limits)
		if err != nil {
			t.Fatal(err)
		}
		if len(graph.Nodes) != 2 || len(graph.Edges) != 3 {
			t.Fatalf("nodes=%d edges=%d, want 2 and 3", len(graph.Nodes), len(graph.Edges))
		}
	})
}

func TestMalformedDelimitersAndQuotesAreRejected(t *testing.T) {
	tests := map[string]string{
		"missing process closer":      "flowchart LR\nA[unterminated",
		"missing decision closer":     "flowchart LR\nA{unterminated",
		"missing data-store closer":   "flowchart LR\nA[(unterminated]",
		"missing edge-label closer":   "flowchart LR\nA -->|unterminated B",
		"empty process label":         "flowchart LR\nA[]",
		"empty quoted process label":  "flowchart LR\nA[\"\"]",
		"single quote decision label": "flowchart LR\nA{\"}",
		"empty decision label":        "flowchart LR\nA{}",
		"empty data-store label":      "flowchart LR\nA[()]",
		"empty edge label":            "flowchart LR\nA -->|| B",
		"leading unmatched quote":     "flowchart LR\nA[\"unterminated]",
		"trailing unmatched quote":    "flowchart LR\nA[unterminated\"]",
		"mismatched quote":            "flowchart LR\nA[\"unterminated']",
		"data-store unmatched quote":  "flowchart LR\nA[(\"unterminated)]",
		"decision unmatched quote":    "flowchart LR\nA{\"unterminated}",
		"trailing process delimiter":  "flowchart LR\nA[label]]",
		"trailing decision delimiter": "flowchart LR\nA{label}}",
	}

	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			assertParseError(t, source, "")
		})
	}
}

func TestCRLFNormalizationAndLoneCRRejection(t *testing.T) {
	lf := "%% comment\nflowchart TD\nA[start] --> B[end]\n"
	crlf := strings.ReplaceAll(lf, "\n", "\r\n")

	want, err := flow.Parse(lf, flow.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	got, err := flow.Parse(crlf, flow.DefaultLimits())
	if err != nil {
		t.Fatalf("CRLF input rejected: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CRLF graph differs from LF graph:\n got: %#v\nwant: %#v", got, want)
	}

	for _, source := range []string{
		"flowchart LR\rA --> B",
		"flowchart LR\nA[bad\rlabel] --> B",
		"flowchart LR\r\nA --> B\rC",
		"flowchart LR\n%% hidden\rcontrol\nA",
	} {
		assertParseError(t, source, "단독 CR")
	}
}

func TestInvalidUTF8IsRejectedEverywhere(t *testing.T) {
	for name, source := range map[string]string{
		"header":  "\xffflowchart LR\nA",
		"node":    "flowchart LR\nA[\xff]",
		"edge":    "flowchart LR\nA -->|\xc0| B",
		"comment": "flowchart LR\n%% \xfe\nA",
		"suffix":  "flowchart LR\nA\xf5",
	} {
		t.Run(name, func(t *testing.T) {
			err := assertParseError(t, source, "유효하지 않은 UTF-8")
			if err.Line != 1 || err.Column != 1 {
				t.Fatalf("invalid UTF-8 location = %d:%d, want 1:1", err.Line, err.Column)
			}
		})
	}
}

func TestTerminalControlsBidiAndFormatsAreRejectedInLabels(t *testing.T) {
	for name, forbidden := range map[string]string{
		"NUL":                "\x00",
		"ESC":                "\x1b",
		"DEL":                "\x7f",
		"C1":                 "\u0085",
		"left-to-right mark": "\u200e",
		"bidi override":      "\u202e",
		"bidi isolate":       "\u2066",
		"zero-width joiner":  "\u200d",
		"variation selector": "\ufe0f",
	} {
		t.Run(name, func(t *testing.T) {
			assertParseError(t, "flowchart LR\nA[safe"+forbidden+"unsafe]", "지원하지 않는 label")
			assertParseError(t, "flowchart LR\nA -->|safe"+forbidden+"unsafe| B", "지원하지 않는 label")
		})
	}
}

func TestCombiningMarkBoundaries(t *testing.T) {
	eightMarks := "a" + strings.Repeat("\u0301", 8)
	nineMarks := "a" + strings.Repeat("\u0301", 9)

	for _, source := range []string{
		"flowchart LR\nA[" + eightMarks + "]",
		"flowchart LR\nA -->|" + eightMarks + "| B",
		"flowchart LR\nA[" + eightMarks + "b" + strings.Repeat("\u0301", 8) + "]",
	} {
		if _, err := flow.Parse(source, flow.DefaultLimits()); err != nil {
			t.Fatalf("valid combining sequence rejected: %v", err)
		}
	}

	for _, source := range []string{
		"flowchart LR\nA[\u0301leading]",
		"flowchart LR\nA -->|\u0301leading| B",
		"flowchart LR\nA[" + nineMarks + "]",
		"flowchart LR\nA -->|" + nineMarks + "| B",
	} {
		assertParseError(t, source, "지원하지 않는 label")
	}
}

func TestParseIsDeterministicForSuccessAndFailure(t *testing.T) {
	valid := `flowchart LR
A[first] -->|one| B{branch}
A -->|two| C[(store)]
B -.->|three| C`
	wantGraph, err := flow.Parse(valid, flow.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	invalid := "flowchart LR\nA -->|missing B"
	wantErr := assertParseError(t, invalid, "")
	for i := 0; i < 100; i++ {
		gotGraph, parseErr := flow.Parse(valid, flow.DefaultLimits())
		if parseErr != nil {
			t.Fatalf("iteration %d valid parse error = %v", i, parseErr)
		}
		if !reflect.DeepEqual(gotGraph, wantGraph) {
			t.Fatalf("iteration %d graph changed:\n got: %#v\nwant: %#v", i, gotGraph, wantGraph)
		}

		gotErr := assertParseError(t, invalid, "")
		if *gotErr != *wantErr {
			t.Fatalf("iteration %d error changed: got %#v, want %#v", i, gotErr, wantErr)
		}
	}
}

func TestCycleCrossesParseBoundaryAndRendersFeedback(t *testing.T) {
	for name, source := range map[string]string{
		"two-node cycle": "flowchart LR\nA --> B\nB --> A",
		"self cycle":     "flowchart TD\nA --> A",
	} {
		t.Run(name, func(t *testing.T) {
			graph, err := flow.Parse(source, flow.DefaultLimits())
			if err != nil {
				t.Fatalf("parser should preserve graph structure for renderer validation: %v", err)
			}
			output, err := render.Flow(graph, render.DefaultOptions())
			if err != nil {
				t.Fatalf("cycle render error = %v", err)
			}
			if !strings.Contains(output, "feedback:") {
				t.Fatalf("feedback legend missing:\n%s", output)
			}
		})
	}
}

func TestDenseRepeatedEdgesPreserveSourceOrderAndLimits(t *testing.T) {
	limits := flow.DefaultLimits()
	lines := []string{"flowchart LR"}
	for i := 0; i < limits.MaxEdges; i++ {
		arrow := "-->"
		if i%2 == 1 {
			arrow = "-.->"
		}
		lines = append(lines, fmt.Sprintf("A %s|edge-%02d| B", arrow, i))
	}

	graph, err := flow.Parse(strings.Join(lines, "\n"), limits)
	if err != nil {
		t.Fatalf("dense repeated-edge graph rejected: %v", err)
	}
	if len(graph.Nodes) != 2 || len(graph.Edges) != limits.MaxEdges {
		t.Fatalf("nodes=%d edges=%d, want 2 and %d", len(graph.Nodes), len(graph.Edges), limits.MaxEdges)
	}
	for i, edge := range graph.Edges {
		if edge.From != 0 || edge.To != 1 || edge.Label != fmt.Sprintf("edge-%02d", i) || edge.Dashed != (i%2 == 1) {
			t.Fatalf("edge %d changed: %#v", i, edge)
		}
	}

	assertParseError(t, strings.Join(append(lines, "A --> B"), "\n"), "edge 수 제한 초과")
}

func TestRandomizedValidDAGsRoundTripParserProperties(t *testing.T) {
	const (
		seed       = int64(0x5eedc0de)
		iterations = 200
	)
	rng := rand.New(rand.NewSource(seed))

	for iteration := 0; iteration < iterations; iteration++ {
		nodeCount := 2 + rng.Intn(15)
		directionName := []string{"LR", "TD", "TB"}[rng.Intn(3)]
		lines := []string{"flowchart " + directionName}
		wantLabels := make([]string, nodeCount)
		wantShapes := make([]flow.Shape, nodeCount)
		for i := 0; i < nodeCount; i++ {
			id := propertyNodeID(i)
			label := fmt.Sprintf("node-%02d", i)
			wantLabels[i] = label
			switch rng.Intn(3) {
			case 0:
				wantShapes[i] = flow.Process
				lines = append(lines, fmt.Sprintf("%s[%s]", id, label))
			case 1:
				wantShapes[i] = flow.Decision
				lines = append(lines, fmt.Sprintf("%s{%s}", id, label))
			case 2:
				wantShapes[i] = flow.DataStore
				lines = append(lines, fmt.Sprintf("%s[(%s)]", id, label))
			}
		}

		var wantEdges []flow.Edge
		for from := 0; from < nodeCount; from++ {
			for to := from + 1; to < nodeCount && len(wantEdges) < flow.DefaultLimits().MaxEdges; to++ {
				if rng.Intn(4) != 0 {
					continue
				}
				dashed := rng.Intn(2) == 0
				arrow := "-->"
				if dashed {
					arrow = "-.->"
				}
				label := ""
				if rng.Intn(2) == 0 {
					label = fmt.Sprintf("edge-%02d", len(wantEdges))
					arrow += "|" + label + "|"
				}
				lines = append(lines, fmt.Sprintf("%s %s %s", propertyNodeID(from), arrow, propertyNodeID(to)))
				wantEdges = append(wantEdges, flow.Edge{From: from, To: to, Label: label, Dashed: dashed})
			}
		}

		source := strings.Join(lines, "\n")
		graph, err := flow.Parse(source, flow.DefaultLimits())
		if err != nil {
			t.Fatalf("seed=%d iteration=%d parse error=%v\nsource:\n%s", seed, iteration, err, source)
		}
		if len(graph.Nodes) != nodeCount {
			t.Fatalf("seed=%d iteration=%d nodes=%d, want %d", seed, iteration, len(graph.Nodes), nodeCount)
		}
		wantDirection := flow.TopToBottom
		if directionName == "LR" {
			wantDirection = flow.LeftToRight
		}
		if graph.Direction != wantDirection {
			t.Fatalf("seed=%d iteration=%d direction=%v, want %v", seed, iteration, graph.Direction, wantDirection)
		}
		for i, node := range graph.Nodes {
			if node.ID != propertyNodeID(i) || node.Label != wantLabels[i] || node.Shape != wantShapes[i] {
				t.Fatalf("seed=%d iteration=%d node %d changed: %#v", seed, iteration, i, node)
			}
		}
		if !reflect.DeepEqual(graph.Edges, wantEdges) {
			t.Fatalf("seed=%d iteration=%d edges changed:\n got: %#v\nwant: %#v", seed, iteration, graph.Edges, wantEdges)
		}
		for _, edge := range graph.Edges {
			if edge.From >= edge.To {
				t.Fatalf("seed=%d iteration=%d non-DAG edge parsed: %#v", seed, iteration, edge)
			}
		}
	}
}

func assertParseError(t *testing.T, source, contains string) *flow.ParseError {
	t.Helper()
	_, err := flow.Parse(source, flow.DefaultLimits())
	if err == nil {
		t.Fatalf("Parse(%q) succeeded, want error", source)
	}
	var parseErr *flow.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error type = %T, want *flow.ParseError", err)
	}
	if parseErr.Line <= 0 || parseErr.Column < 0 {
		t.Fatalf("invalid error location: %#v", parseErr)
	}
	if contains != "" && !strings.Contains(parseErr.Message, contains) {
		t.Fatalf("error = %q, want substring %q", parseErr.Message, contains)
	}
	return parseErr
}

func propertyNodeID(i int) string {
	return fmt.Sprintf("N%03d", i)
}
