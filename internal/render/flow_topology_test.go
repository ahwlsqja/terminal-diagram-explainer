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
	if _, denseErr := Flow(dense, DefaultOptions()); !errors.Is(denseErr, ErrLayout) {
		t.Fatalf("현재 router가 안전하게 분리하지 못하는 다대다 graph를 성공 처리함: %v", denseErr)
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
