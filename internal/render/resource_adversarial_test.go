package render

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

func TestAdversarialBranchingLRAndTD(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		arrow     string
		junction  string
		maxWidth  int
		maxHeight int
	}{
		{name: "LR", header: "flowchart LR", arrow: "▶", junction: "┴", maxWidth: 100, maxHeight: 20},
		{name: "TD", header: "flowchart TD", arrow: "▼", junction: "┤", maxWidth: 60, maxHeight: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := tt.header + `
Root[수신] -->|허용| Accept[저장]
Root -->|거부| Reject[격리]`
			graph := mustParseGraph(t, source)
			got := mustRender(t, graph, Options{MaxWidth: tt.maxWidth, MaxHeight: tt.maxHeight})

			for _, label := range []string{"수신", "저장", "격리", "허용", "거부"} {
				if !strings.Contains(got, label) {
					t.Errorf("분기 출력에서 label %q 누락:\n%s", label, got)
				}
			}
			if count := strings.Count(got, tt.arrow); count != 2 {
				t.Errorf("화살표 수 = %d, want 2:\n%s", count, got)
			}
			if !strings.Contains(got, tt.junction) {
				t.Errorf("공유 경로 junction %q 누락:\n%s", tt.junction, got)
			}
		})
	}
}

func TestAdversarialCanvasLineJunctions(t *testing.T) {
	tests := []struct {
		name  string
		ascii bool
		want  string
	}{
		{name: "unicode", want: "┼"},
		{name: "ascii", ascii: true, want: "+"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canvas, err := newCanvas(9, 7, tt.ascii)
			if err != nil {
				t.Fatal(err)
			}
			if err := canvas.horizontal(1, 7, 3, false); err != nil {
				t.Fatal(err)
			}
			if err := canvas.vertical(4, 1, 5, false); err != nil {
				t.Fatal(err)
			}

			rows := strings.Split(canvas.String(), "\n")
			if got := string([]rune(rows[3])[4]); got != tt.want {
				t.Fatalf("교차점 = %q, want %q:\n%s", got, tt.want, canvas.String())
			}
		})
	}
}

func TestAdversarialEdgeLabelsPreserved(t *testing.T) {
	source := `flowchart LR
Start[시작] -->|승인 é| Good[正常]
Start -.->|거절 이유| Bad[失敗]`
	graph := mustParseGraph(t, source)
	got := mustRender(t, graph, Options{MaxWidth: 100, MaxHeight: 20})

	for _, label := range []string{"승인 é", "거절 이유"} {
		if count := strings.Count(got, label); count != 1 {
			t.Errorf("edge label %q count = %d, want 1:\n%s", label, count, got)
		}
	}
	if !strings.Contains(got, "┄") {
		t.Errorf("dashed edge glyph 누락:\n%s", got)
	}
}

func TestAdversarialWideAndCombiningLabelsStayAligned(t *testing.T) {
	source := `flowchart TD
Korean[한글 수신] --> CJK[事件処理]
CJK --> Combining[école]
Combining --> WideCombining[한́글]`
	graph := mustParseGraph(t, source)
	got := mustRender(t, graph, Options{MaxWidth: 40, MaxHeight: 40})

	for _, label := range []string{"한글 수신", "事件処理", "école", "한́글"} {
		if !strings.Contains(got, label) {
			t.Errorf("Unicode label %q가 보존되지 않음:\n%s", label, got)
		}
	}
	assertOutputWithinLimits(t, got, 40, 40)

	lines := strings.Split(got, "\n")
	boxCount := 0
	for row := 0; row+2 < len(lines); row++ {
		trimmed := strings.TrimLeft(lines[row], " ")
		if !strings.HasPrefix(trimmed, "┌") || !strings.HasSuffix(trimmed, "┐") {
			continue
		}
		boxCount++
		topWidth, topErr := textcell.Width(lines[row])
		middleWidth, middleErr := textcell.Width(lines[row+1])
		bottomWidth, bottomErr := textcell.Width(lines[row+2])
		if topErr != nil || middleErr != nil || bottomErr != nil {
			t.Fatalf("box 폭 측정 실패: top=%v middle=%v bottom=%v", topErr, middleErr, bottomErr)
		}
		if topWidth != middleWidth || middleWidth != bottomWidth {
			t.Errorf("row %d box 정렬 폭 = (%d, %d, %d):\n%s", row, topWidth, middleWidth, bottomWidth, got)
		}
	}
	if boxCount != 4 {
		t.Fatalf("aligned box count=%d, want 4:\n%s", boxCount, got)
	}
}

func TestAdversarialASCIIModeUsesOnlyASCIIDrawingGlyphs(t *testing.T) {
	graph := mustParseGraph(t, `flowchart LR
A[한글] -->|ok| B{事件}
A -.->|no| C[(저장)]`)
	got := mustRender(t, graph, Options{ASCII: true, MaxWidth: 80, MaxHeight: 20})

	if strings.ContainsAny(got, "┌┐└┘╭╮╰╯╔╗╚╝─│┄┊┼▶▼═║") {
		t.Fatalf("ASCII mode에 Unicode drawing glyph가 포함됨:\n%s", got)
	}
	for _, token := range []string{"+", "-", "|", ">", ".", "한글", "事件", "저장", "ok", "no"} {
		if !strings.Contains(got, token) {
			t.Errorf("ASCII mode token %q 누락:\n%s", token, got)
		}
	}
}

func TestAdversarialDeterministicOutputAndRepeatedRuns(t *testing.T) {
	graphs := []*flow.Graph{
		mustParseGraph(t, `flowchart LR
A[입력] -->|yes| B[처리]
A -.->|no| C[격리]
B --> D[완료]
C --> D`),
		mustParseGraph(t, `flowchart TD
A[입력] -->|yes| B[처리]
A -.->|no| C[격리]
B --> D[완료]
C --> D`),
	}
	options := []Options{
		{MaxWidth: 100, MaxHeight: 40},
		{ASCII: true, MaxWidth: 100, MaxHeight: 40},
	}

	for graphIndex, graph := range graphs {
		for optionIndex, option := range options {
			want := mustRender(t, graph, option)
			for run := 0; run < 256; run++ {
				got, err := Flow(graph, option)
				if err != nil {
					t.Fatalf("graph=%d option=%d run=%d: %v", graphIndex, optionIndex, run, err)
				}
				if got != want {
					t.Fatalf("graph=%d option=%d run=%d에서 비결정적 출력", graphIndex, optionIndex, run)
				}
			}
		}
	}
}

func TestAdversarialOutputNeverClipsSilently(t *testing.T) {
	t.Run("successful output stays inside limits", func(t *testing.T) {
		graph := mustParseGraph(t, `flowchart LR
A[수신事件] -->|검증완료| B[저장完了]`)
		const maxWidth = 39
		const maxHeight = 3
		got := mustRender(t, graph, Options{MaxWidth: maxWidth, MaxHeight: maxHeight})
		assertOutputWithinLimits(t, got, maxWidth, maxHeight)
		for _, suffix := range []string{"수신事件", "검증완료", "저장完了"} {
			if !strings.Contains(got, suffix) {
				t.Errorf("출력 경계에서 label %q가 잘림:\n%s", suffix, got)
			}
		}
	})

	t.Run("oversized TD edge label returns an error", func(t *testing.T) {
		graph := mustParseGraph(t, `flowchart TD
A -->|edge-label-too-wide| B`)
		_, err := Flow(graph, Options{MaxWidth: 10, MaxHeight: 20})
		if err == nil {
			t.Fatal("edge label clipping 대신 오류를 기대했으나 성공함")
		}
		if !strings.Contains(err.Error(), "경계") && !strings.Contains(err.Error(), "폭 제한") {
			t.Fatalf("clipping 방지 오류 = %q", err)
		}
	})
}

func TestAdversarialWidthAndHeightFailures(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		options Options
		want    string
	}{
		{
			name:    "LR width",
			source:  "flowchart LR\nA[긴 라벨 시작] --> B[긴 라벨 끝]",
			options: Options{MaxWidth: 15, MaxHeight: 20},
			want:    "출력 폭 제한 초과",
		},
		{
			name:    "TD height",
			source:  "flowchart TD\nA --> B\nB --> C",
			options: Options{MaxWidth: 40, MaxHeight: 10},
			want:    "출력 높이 제한 초과",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph := mustParseGraph(t, tt.source)
			_, err := Flow(graph, tt.options)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Flow() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestAdversarialStress48Nodes96Edges(t *testing.T) {
	graph := adversarialDAG(48, 96)
	options := Options{MaxWidth: 512, MaxHeight: 512}
	got := mustRender(t, graph, options)

	if len(graph.Nodes) != 48 || len(graph.Edges) != 96 {
		t.Fatalf("stress fixture = %d nodes/%d edges", len(graph.Nodes), len(graph.Edges))
	}
	for _, label := range []string{"N00", "N24", "N47"} {
		if !strings.Contains(got, label) {
			t.Errorf("stress output에서 %q 누락", label)
		}
	}
	if count := strings.Count(got, "┌─────┐"); count != 48 {
		t.Errorf("rendered node boxes = %d, want 48", count)
	}
	assertOutputWithinLimits(t, got, options.MaxWidth, options.MaxHeight)
}

func TestAdversarialCycleRenders(t *testing.T) {
	graph := &flow.Graph{
		Direction: flow.TopToBottom,
		Nodes: []flow.Node{
			{ID: "A", Label: "A"},
			{ID: "B", Label: "B"},
			{ID: "C", Label: "C"},
		},
		Edges: []flow.Edge{
			{From: 0, To: 1},
			{From: 1, To: 2},
			{From: 2, To: 0},
		},
	}

	output, err := Flow(graph, Options{MaxWidth: 80, MaxHeight: 60})
	if err != nil {
		t.Fatalf("cycle render error = %v", err)
	}
	if !strings.Contains(output, "feedback:") {
		t.Fatalf("feedback legend missing:\n%s", output)
	}
}

func TestAdversarialAllocationGrowthIsBounded(t *testing.T) {
	options := Options{MaxWidth: 512, MaxHeight: 512}
	small := adversarialDAG(12, 24)
	large := adversarialDAG(48, 96)
	mustRender(t, small, options)
	mustRender(t, large, options)

	smallAllocs := testing.AllocsPerRun(20, func() {
		if _, err := Flow(small, options); err != nil {
			panic(err)
		}
	})
	largeAllocs := testing.AllocsPerRun(20, func() {
		if _, err := Flow(large, options); err != nil {
			panic(err)
		}
	})

	// 입력은 4배 커진다. 8배 + 고정 여유를 넘는 증가는 비선형 자원 회귀로 본다.
	if limit := smallAllocs*8 + 128; largeAllocs > limit {
		t.Fatalf("allocation growth too high: small=%.0f large=%.0f limit=%.0f", smallAllocs, largeAllocs, limit)
	}
	t.Logf("allocations/run: 12 nodes/24 edges=%.0f, 48 nodes/96 edges=%.0f", smallAllocs, largeAllocs)
}

func TestAdversarialCycleStressAllocationAndBounds(t *testing.T) {
	graph := cycleStressGraph(48, 96)
	options := Options{MaxWidth: 512, MaxHeight: 512}
	output := mustRender(t, graph, options)
	assertOutputWithinLimits(t, output, options.MaxWidth, options.MaxHeight)

	allocations := testing.AllocsPerRun(10, func() {
		if _, err := Flow(graph, options); err != nil {
			panic(err)
		}
	})
	if allocations > 2_500 {
		t.Fatalf("cycle allocations/run = %.0f, limit = 2500", allocations)
	}
	t.Logf("cycle allocations/run: %.0f", allocations)
}

func adversarialDAG(nodeCount, edgeCount int) *flow.Graph {
	graph := &flow.Graph{Direction: flow.TopToBottom}
	for index := 0; index < nodeCount; index++ {
		id := fmt.Sprintf("N%02d", index)
		graph.Nodes = append(graph.Nodes, flow.Node{ID: id, Label: id})
	}
	for index := 0; index+1 < nodeCount && len(graph.Edges) < edgeCount; index++ {
		graph.Edges = append(graph.Edges, flow.Edge{From: index, To: index + 1})
	}
	for span := 2; len(graph.Edges) < edgeCount; span++ {
		for from := 0; from+span < nodeCount && len(graph.Edges) < edgeCount; from++ {
			graph.Edges = append(graph.Edges, flow.Edge{From: from, To: from + span})
		}
	}
	return graph
}

func mustParseGraph(t *testing.T, source string) *flow.Graph {
	t.Helper()
	graph, err := flow.Parse(source, flow.DefaultLimits())
	if err != nil {
		t.Fatalf("flow.Parse() error = %v", err)
	}
	return graph
}

func mustRender(t *testing.T, graph *flow.Graph, options Options) string {
	t.Helper()
	got, err := Flow(graph, options)
	if err != nil {
		t.Fatalf("Flow() error = %v", err)
	}
	return got
}

func assertOutputWithinLimits(t *testing.T, output string, maxWidth, maxHeight int) {
	t.Helper()
	lines := strings.Split(output, "\n")
	if len(lines) > maxHeight {
		t.Errorf("output height = %d, limit = %d", len(lines), maxHeight)
	}
	for index, line := range lines {
		width, err := textcell.Width(line)
		if err != nil {
			t.Fatalf("line %d width error: %v", index+1, err)
		}
		if width > maxWidth {
			t.Errorf("line %d width = %d, limit = %d: %q", index+1, width, maxWidth, line)
		}
	}
}
