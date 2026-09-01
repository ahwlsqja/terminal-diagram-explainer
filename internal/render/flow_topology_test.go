package render

import (
	"errors"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
)

func TestFlowSeparatesCrossedEdgesInsteadOfCollapsingAdjacency(t *testing.T) {
	crossed, err := flow.Parse(`flowchart TD
A[A]
B[B]
C[C]
D[D]
A --> D
B --> C`, flow.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	crossedOutput, err := Flow(crossed, DefaultOptions())
	if err != nil {
		t.Fatalf("교차 최소화로 풀 수 있는 graph가 실패함: %v", err)
	}

	dense, err := flow.Parse(`flowchart TD
A[A]
B[B]
C[C]
D[D]
A --> C
A --> D
B --> C
B --> D`, flow.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if crossedOutput == "" {
		t.Fatal("교차 최소화 결과가 비어 있음")
	}
	denseOutput, denseErr := Flow(dense, DefaultOptions())
	if denseErr != nil {
		t.Fatalf("다대다 graph를 outer route manifest로 복구하지 못함: %v", denseErr)
	}
	for _, want := range []string{"R01 A --> C", "R02 A --> D", "R03 B --> C", "R04 B --> D"} {
		if !strings.Contains(denseOutput, want) {
			t.Fatalf("다대다 graph의 endpoint manifest가 %q를 잃음:\n%s", want, denseOutput)
		}
	}
}

func TestFlowPromotesMixedFanOutFanInEdgeToOuterRoute(t *testing.T) {
	graph := mustParseGraph(t, `flowchart TD
A --> D
B --> C
B --> D`)
	output, err := Flow(graph, DefaultOptions())
	if err != nil {
		t.Fatalf("유효한 혼합 fan-out/fan-in DAG가 실패함: %v", err)
	}
	if !strings.Contains(output, "R01 B --> D") {
		t.Fatalf("혼합 junction edge의 endpoint manifest가 없음:\n%s", output)
	}
}

func TestFlowRejectsParallelEdgesInsteadOfCollapsingThem(t *testing.T) {
	graph, err := flow.Parse(`flowchart LR
A[Alpha] -->|first| B[Beta]
A -->|second| B`, flow.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Flow(graph, DefaultOptions()); !errors.Is(err, ErrInvalidGraph) {
		t.Fatalf("parallel edge가 하나의 route로 축약됨: %v", err)
	}
}

func TestTDUnlabeledOuterRoutesPreserveEndpointIdentityInEveryOutput(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		outerEdges []string
	}{
		{
			name: "A-to-D-and-B-to-D",
			source: `flowchart TD
A --> B
A --> D
B --> C
B --> D
C --> D`,
			outerEdges: []string{"A --> D", "B --> D"},
		},
		{
			name: "A-to-C-and-B-to-D",
			source: `flowchart TD
A --> B
A --> C
B --> C
B --> D
C --> D`,
			outerEdges: []string{"A --> C", "B --> D"},
		},
	}

	outputs := make([]string, len(tests))
	svgs := make([]string, len(tests))
	htmls := make([]string, len(tests))
	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			terminal := mustRender(t, mustParseGraph(t, tt.source), DefaultOptions())
			outputs[index] = terminal
			for _, want := range tt.outerEdges {
				if !strings.Contains(terminal, want) {
					t.Fatalf("unlabeled outer route lost endpoint identity %q:\n%s", want, terminal)
				}
			}

			svg, err := SVG(terminal)
			if err != nil {
				t.Fatalf("SVG() error = %v", err)
			}
			html, err := HTML(terminal)
			if err != nil {
				t.Fatalf("HTML() error = %v", err)
			}
			svgs[index] = svg
			htmls[index] = html
		})
	}
	if outputs[0] == outputs[1] {
		t.Fatalf("distinct successful TD graphs collapsed to the same output:\n%s", outputs[0])
	}
	if svgs[0] == svgs[1] || htmls[0] == htmls[1] {
		t.Fatal("distinct outer-route endpoints collapsed in an SVG or HTML derivative")
	}
}

func TestCanvasUsesDirectionalCornerAtAnElbow(t *testing.T) {
	canvas, err := newCanvas(7, 7, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := canvas.vertical(2, 0, 2, false); err != nil {
		t.Fatal(err)
	}
	if err := canvas.horizontal(2, 5, 2, false); err != nil {
		t.Fatal(err)
	}

	if got := canvas.at(2, 2).text; got != "└" {
		t.Fatalf("elbow glyph=%q, want └:\n%s", got, canvas.String())
	}
}

func TestTDSingleSuccessorAlignsWithoutOneCellDogleg(t *testing.T) {
	graph, err := flow.Parse(`flowchart TD
QUEUE[time-first candidate queue] --> PRODUCER[bounded Producer]
PRODUCER --> TERMINAL[terminal receipt and cursor]`, flow.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	output, err := Flow(graph, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output, "┌┘") || strings.Contains(output, "└┐") || strings.Contains(output, "┼┼") {
		t.Fatalf("단일 successor chain에 불필요한 one-cell dogleg가 남음:\n%s", output)
	}
}
