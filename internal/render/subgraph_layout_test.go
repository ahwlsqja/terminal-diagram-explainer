package render

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

func TestSubgraphSiblingFramesUseDistinctCrossAxisBands(t *testing.T) {
	for _, tt := range []struct {
		name      string
		direction string
	}{
		{name: "LR uses y bands", direction: "LR"},
		{name: "TD uses x bands", direction: "TD"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			source := "flowchart " + tt.direction + `
subgraph Source[Source Scope]
A[Alpha]
end
subgraph Obstacle[Obstacle Scope]
O[Idle]
end
subgraph Target[Target Scope]
B[Beta]
end
A -->|cross| B`
			output := renderSubgraphFixture(t, source, Options{MaxWidth: 180, MaxHeight: 100})
			grid := newSubgraphCellGrid(t, output)

			sourceFrame := grid.boxContaining(t, "Source Scope")
			obstacleFrame := grid.boxContaining(t, "Obstacle Scope")
			targetFrame := grid.boxContaining(t, "Target Scope")
			alphaNode := grid.boxContaining(t, "Alpha")
			idleNode := grid.boxContaining(t, "Idle")
			betaNode := grid.boxContaining(t, "Beta")

			assertStrictlyContainsBox(t, sourceFrame, alphaNode, "source frame")
			assertStrictlyContainsBox(t, obstacleFrame, idleNode, "obstacle frame")
			assertStrictlyContainsBox(t, targetFrame, betaNode, "target frame")
			if tt.direction == "LR" {
				assertBoxesSeparatedVertically(t, sourceFrame, obstacleFrame)
				assertBoxesSeparatedVertically(t, obstacleFrame, targetFrame)
			} else {
				assertBoxesSeparatedHorizontally(t, sourceFrame, obstacleFrame)
				assertBoxesSeparatedHorizontally(t, obstacleFrame, targetFrame)
			}
			grid.assertNoUnexpectedDrawingInside(t, obstacleFrame, []subgraphTestBox{idleNode})
			assertTextCount(t, output, "Source Scope", 1)
			assertTextCount(t, output, "Obstacle Scope", 1)
			assertTextCount(t, output, "Target Scope", 1)
			assertTextCount(t, output, "Alpha", 1)
			assertTextCount(t, output, "Idle", 1)
			assertTextCount(t, output, "Beta", 1)
		})
	}
}

func TestSubgraphNestedChildOnlyParentContainsChildAndNode(t *testing.T) {
	for _, direction := range []string{"LR", "TD"} {
		t.Run(direction, func(t *testing.T) {
			source := "flowchart " + direction + `
subgraph Parent[부모 범위]
subgraph Child[子 scope]
A[Leaf]
end
end`
			output := renderSubgraphFixture(t, source, Options{MaxWidth: 100, MaxHeight: 60})
			grid := newSubgraphCellGrid(t, output)
			parentFrame := grid.boxContaining(t, "부모 범위")
			childFrame := grid.boxContaining(t, "子 scope")
			node := grid.boxContaining(t, "Leaf")

			assertStrictlyContainsBox(t, parentFrame, childFrame, "parent frame")
			assertStrictlyContainsBox(t, childFrame, node, "child frame")
			assertTextCount(t, output, "부모 범위", 1)
			assertTextCount(t, output, "子 scope", 1)
			assertTextCount(t, output, "Leaf", 1)
		})
	}
}

func TestSubgraphTDNestedOutputHasNoLeadingBlankLines(t *testing.T) {
	source := `flowchart TD
subgraph Parent[Parent]
X[Parent Node]
subgraph Child[Child]
A[Child Node]
end
X --> A
end`
	output := renderSubgraphFixture(t, source, Options{MaxWidth: 100, MaxHeight: 60})
	if strings.HasPrefix(output, "\n") {
		leading := len(output) - len(strings.TrimLeft(output, "\n"))
		t.Fatalf("nested TD output starts with %d blank lines:\n%q", leading, output)
	}
}

func TestSubgraphTDDepthTwoFeedbackToRootKeepsTopIngressVisible(t *testing.T) {
	source := `flowchart TD
Root[Root]
subgraph Parent[Parent]
subgraph Child[Child]
A[Scoped]
end
end
Root --> A
A -->|return| Root`
	graph := parseSubgraphFixture(t, source)
	output, err := Flow(graph, Options{MaxWidth: 80, MaxHeight: 40})
	if errors.Is(err, ErrOutputBounds) {
		t.Fatalf("valid depth-2 feedback fixture was rejected as out of bounds: %v", err)
	}
	if err != nil {
		t.Fatalf("depth-2 feedback render failed: %v", err)
	}
	if strings.HasPrefix(output, "\n") {
		t.Fatalf("depth-2 feedback output starts with a blank line:\n%q", output)
	}
	assertSubgraphOutputWithinLimits(t, output, 80, 40)
	if !strings.Contains(output, "feedback:") || !strings.Contains(output, "F01 A --> Root |return|") {
		t.Fatalf("root-target feedback legend is missing:\n%s", output)
	}

	grid := newSubgraphCellGrid(t, output)
	rootNode := grid.boxContaining(t, "Root")
	parentFrame := grid.boxContaining(t, "Parent")
	childFrame := grid.boxContaining(t, "Child")
	scopedNode := grid.boxContaining(t, "Scoped")
	assertStrictlyContainsBox(t, parentFrame, childFrame, "parent frame")
	assertStrictlyContainsBox(t, childFrame, scopedNode, "child frame")

	connectorX := rootNode.left + (rootNode.right-rootNode.left+1)/2
	connectorY := rootNode.top - 2
	arrowY := rootNode.top - 1
	if connectorY < 0 {
		t.Fatalf("root ingress connector is outside output: node=%+v", rootNode)
	}
	if current := grid.cells[connectorY][connectorX]; current != "│" && current != "┊" && current != "┼" {
		t.Fatalf("root ingress connector missing at (%d,%d), cell=%q:\n%s", connectorX, connectorY, current, output)
	}
	if current := grid.cells[arrowY][connectorX]; current != "▼" {
		t.Fatalf("root feedback arrow missing at (%d,%d), cell=%q:\n%s", connectorX, arrowY, current, output)
	}
}

func TestSubgraphSameScopeAdjacentEdgeStaysInline(t *testing.T) {
	for _, direction := range []string{"LR", "TD"} {
		t.Run(direction, func(t *testing.T) {
			source := "flowchart " + direction + `
subgraph S[Local Scope]
A[Start] -->|inside| B[Done]
end`
			output := renderSubgraphFixture(t, source, Options{MaxWidth: 100, MaxHeight: 50})
			diagram := strings.Split(output, "\n\nrouted:\n")[0]
			if !strings.Contains(diagram, "inside") {
				t.Fatalf("same-scope edge label was not rendered inline:\n%s", output)
			}
			if strings.Contains(output, "routed:") {
				t.Fatalf("adjacent same-scope edge unexpectedly used routed legend:\n%s", output)
			}
			frame := newSubgraphCellGrid(t, output).boxContaining(t, "Local Scope")
			labelX, labelY := locateSubgraphText(t, output, "inside")
			if !frame.containsCell(labelX, labelY) {
				t.Fatalf("inline label at (%d,%d) is outside frame %+v:\n%s", labelX, labelY, frame, output)
			}
		})
	}
}

func TestSubgraphTDLongInlineLabelStaysInsideOwnFrame(t *testing.T) {
	const longLabel = "LLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLL"
	source := `flowchart TD
subgraph S[Scope]
A[A] -->|` + longLabel + `| B[B]
end`
	output := renderSubgraphFixture(t, source, Options{MaxWidth: 100, MaxHeight: 50})
	grid := newSubgraphCellGrid(t, output)
	frame := grid.topBorderFrameContaining(t, "Scope")
	labelX, labelY := locateSubgraphText(t, output, longLabel)
	labelWidth, err := textcell.Width(longLabel)
	if err != nil {
		t.Fatal(err)
	}

	if labelX <= frame.left || labelX+labelWidth > frame.right {
		t.Errorf("inline label cells [%d,%d) escape own frame interior (%d,%d):\n%s", labelX, labelX+labelWidth, frame.left, frame.right, output)
	}
	grid.assertFrameSideCell(t, "own frame right border", frame, frame.right, labelY)
}

func TestSubgraphTDSiblingFramesAndCrossRouteSurviveLongInlineLabel(t *testing.T) {
	const longLabel = "LLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLL"
	source := `flowchart TD
subgraph S1[Left]
A[A] -->|` + longLabel + `| B[B]
end
subgraph S2[Right]
C[C]
end
C -->|cross| B`
	output := renderSubgraphFixture(t, source, Options{MaxWidth: 100, MaxHeight: 60})
	grid := newSubgraphCellGrid(t, output)
	leftFrame := grid.topBorderFrameContaining(t, "Left")
	rightFrame := grid.topBorderFrameContaining(t, "Right")
	labelX, labelY := locateSubgraphText(t, output, longLabel)
	labelWidth, err := textcell.Width(longLabel)
	if err != nil {
		t.Fatal(err)
	}
	labelRight := labelX + labelWidth

	if labelX <= leftFrame.left || labelRight > leftFrame.right {
		t.Errorf("inline label cells [%d,%d) escape left frame interior (%d,%d):\n%s", labelX, labelRight, leftFrame.left, leftFrame.right, output)
	}
	if labelY > rightFrame.top && labelY < rightFrame.bottom && labelRight > rightFrame.left+1 && labelX < rightFrame.right {
		t.Errorf("inline label cells [%d,%d) enter sibling frame %+v:\n%s", labelX, labelRight, rightFrame, output)
	}
	for _, check := range []struct {
		name  string
		frame subgraphTestBox
		x     int
	}{
		{name: "left frame right border", frame: leftFrame, x: leftFrame.right},
		{name: "right frame left border", frame: rightFrame, x: rightFrame.left},
		{name: "right frame right border", frame: rightFrame, x: rightFrame.right},
	} {
		grid.assertFrameSideCell(t, check.name, check.frame, check.x, labelY)
	}

	sourceNode := grid.boxContaining(t, "C")
	connectorX := (sourceNode.left + sourceNode.right) / 2
	connectorY := sourceNode.bottom + 2
	if current := grid.cells[connectorY][connectorX]; current != "│" && current != "┊" && current != "┼" {
		t.Errorf("cross-scope outer connector is not continuous at (%d,%d), cell=%q:\n%s", connectorX, connectorY, current, output)
	}
}

func TestSubgraphAdjacentCrossScopeEdgeUsesOuterRouteAndLegend(t *testing.T) {
	for _, direction := range []string{"LR", "TD"} {
		t.Run(direction, func(t *testing.T) {
			source := "flowchart " + direction + `
subgraph Left[Left Scope]
A[Producer]
end
subgraph Right[Right Scope]
B[Consumer]
end
A -->|cross scope| B`
			output := renderSubgraphFixture(t, source, Options{MaxWidth: 120, MaxHeight: 70})
			parts := strings.Split(output, "\n\nrouted:\n")
			if len(parts) != 2 {
				t.Fatalf("cross-scope edge is missing routed legend:\n%s", output)
			}
			if strings.Contains(parts[0], "cross scope") {
				t.Fatalf("cross-scope label was rendered inline instead of in the legend:\n%s", output)
			}
			if !strings.Contains(parts[1], "R01 A --> B |cross scope|") {
				t.Fatalf("cross-scope routed legend is incomplete:\n%s", output)
			}
			assertTextCount(t, output, "cross scope", 1)
		})
	}
}

func TestSubgraphNestedCycleSkipAndCJKLabelsRemainVisible(t *testing.T) {
	for _, direction := range []string{"LR", "TD"} {
		t.Run(direction, func(t *testing.T) {
			source := "flowchart " + direction + `
subgraph Parent[부모 영역]
subgraph Child[事件 처리]
A[한글 시작] --> B[中間]
B --> C[終了]
C -.->|재시도 é| A
A -->|건너뛰기| C
end
end`
			output := renderSubgraphFixture(t, source, Options{MaxWidth: 180, MaxHeight: 120})
			for _, label := range []string{"부모 영역", "事件 처리", "한글 시작", "中間", "終了"} {
				assertTextCount(t, output, label, 1)
			}
			if !strings.Contains(output, "feedback:") || !strings.Contains(output, "F01 C -.-> A |재시도 é|") {
				t.Fatalf("nested feedback legend is missing:\n%s", output)
			}
			if !strings.Contains(output, "routed:") || !strings.Contains(output, "R01 A --> C |건너뛰기|") {
				t.Fatalf("nested skip-rank legend is missing:\n%s", output)
			}
			assertTextCount(t, output, "재시도 é", 1)
			assertTextCount(t, output, "건너뛰기", 1)

			grid := newSubgraphCellGrid(t, output)
			parentFrame := grid.boxContaining(t, "부모 영역")
			childFrame := grid.boxContaining(t, "事件 처리")
			assertStrictlyContainsBox(t, parentFrame, childFrame, "nested CJK parent")
			for _, nodeLabel := range []string{"한글 시작", "中間", "終了"} {
				assertStrictlyContainsBox(t, childFrame, grid.boxContaining(t, nodeLabel), nodeLabel)
			}
		})
	}
}

func TestSubgraphASCIIUsesASCIIFramesAndPreservesTitles(t *testing.T) {
	source := `flowchart LR
subgraph Outer[부모]
subgraph Inner[事件]
A[한글] --> B[Done]
end
end`
	output := renderSubgraphFixture(t, source, Options{ASCII: true, MaxWidth: 100, MaxHeight: 60})
	if strings.ContainsAny(output, "┌┐└┘╭╮╰╯╔╗╚╝─│┄┊┼▶▼═║") {
		t.Fatalf("ASCII subgraph output contains Unicode drawing glyphs:\n%s", output)
	}
	for _, token := range []string{"+", "-", "|", ">", "부모", "事件", "한글", "Done"} {
		if !strings.Contains(output, token) {
			t.Fatalf("ASCII subgraph output is missing %q:\n%s", token, output)
		}
	}
}

func TestSubgraphOutputIsDeterministic(t *testing.T) {
	source := `flowchart TD
subgraph Parent[Parent]
subgraph One[One]
A --> B
B --> A
end
subgraph Two[Two]
C -->|cross| A
end
end`
	graph, err := flow.Parse(source, flow.DefaultLimits())
	if err != nil {
		t.Fatalf("subgraph parse failed: %v", err)
	}
	options := Options{MaxWidth: 140, MaxHeight: 100}
	want, err := Flow(graph, options)
	if err != nil {
		t.Fatalf("subgraph render failed: %v", err)
	}
	for iteration := 0; iteration < 256; iteration++ {
		got, renderErr := Flow(graph, options)
		if renderErr != nil {
			t.Fatalf("iteration %d: %v", iteration, renderErr)
		}
		if got != want {
			t.Fatalf("subgraph output changed at iteration %d", iteration)
		}
	}
}

func TestSubgraphBoundsFailClosedBeforeClipping(t *testing.T) {
	t.Run("small options", func(t *testing.T) {
		source := `flowchart TD
subgraph Parent[아주 긴 부모 범위 제목]
subgraph Child[중첩 범위]
A[Node]
end
end`
		graph := parseSubgraphFixture(t, source)
		output, err := Flow(graph, Options{MaxWidth: 12, MaxHeight: 8})
		if err == nil || output != "" {
			t.Fatalf("small subgraph canvas did not fail closed: output=%q err=%v", output, err)
		}
		if !errors.Is(err, ErrOutputBounds) {
			t.Fatalf("small subgraph canvas error=%v, want ErrOutputBounds", err)
		}
	})

	t.Run("hard cap", func(t *testing.T) {
		graph := parseSubgraphFixture(t, "flowchart LR\nsubgraph S[Scope]\nA\nend")
		output, err := Flow(graph, Options{MaxWidth: 513, MaxHeight: 40})
		if err == nil || output != "" {
			t.Fatalf("canvas hard cap did not fail closed: output=%q err=%v", output, err)
		}
		if !errors.Is(err, ErrOutputBounds) {
			t.Fatalf("hard-cap error=%v, want ErrOutputBounds", err)
		}
	})

	t.Run("scope bands exceed 512", func(t *testing.T) {
		var source strings.Builder
		source.WriteString("flowchart TD\n")
		for index := 0; index < 8; index++ {
			fmt.Fprintf(&source, "subgraph S%d[%s]\nN%d\nend\n", index, strings.Repeat("界", 48), index)
		}
		graph := parseSubgraphFixture(t, source.String())
		output, err := Flow(graph, Options{MaxWidth: 512, MaxHeight: 512})
		if err == nil || output != "" {
			t.Fatalf("oversized sibling scopes did not fail closed: output=%q err=%v", output, err)
		}
		if !errors.Is(err, ErrOutputBounds) {
			t.Fatalf("oversized sibling error=%v, want ErrOutputBounds", err)
		}
	})
}

func parseSubgraphFixture(t *testing.T, source string) *flow.Graph {
	t.Helper()
	graph, err := flow.Parse(source, flow.DefaultLimits())
	if err != nil {
		t.Fatalf("subgraph parse failed: %v\nsource:\n%s", err, source)
	}
	return graph
}

func renderSubgraphFixture(t *testing.T, source string, options Options) string {
	t.Helper()
	graph := parseSubgraphFixture(t, source)
	output, err := Flow(graph, options)
	if err != nil {
		t.Fatalf("subgraph render failed: %v\nsource:\n%s", err, source)
	}
	assertSubgraphOutputWithinLimits(t, output, options.MaxWidth, options.MaxHeight)
	return output
}

func assertSubgraphOutputWithinLimits(t *testing.T, output string, maxWidth, maxHeight int) {
	t.Helper()
	lines := strings.Split(output, "\n")
	if len(lines) > maxHeight {
		t.Fatalf("output height=%d, limit=%d", len(lines), maxHeight)
	}
	for lineIndex, line := range lines {
		width, err := textcell.Width(line)
		if err != nil {
			t.Fatalf("line %d width error: %v", lineIndex+1, err)
		}
		if width > maxWidth {
			t.Fatalf("line %d width=%d, limit=%d: %q", lineIndex+1, width, maxWidth, line)
		}
	}
}

type subgraphTestBox struct {
	left, top, right, bottom int // inclusive cell coordinates
}

func (b subgraphTestBox) area() int {
	return (b.right - b.left + 1) * (b.bottom - b.top + 1)
}

func (b subgraphTestBox) containsCell(x, y int) bool {
	return x > b.left && x < b.right && y > b.top && y < b.bottom
}

type subgraphCellGrid struct {
	cells  [][]string
	output string
}

func newSubgraphCellGrid(t *testing.T, output string) subgraphCellGrid {
	t.Helper()
	lines := strings.Split(output, "\n")
	maxWidth := 0
	for _, line := range lines {
		width, err := textcell.Width(line)
		if err != nil {
			t.Fatalf("output width error: %v", err)
		}
		if width > maxWidth {
			maxWidth = width
		}
	}
	cells := make([][]string, len(lines))
	for y, line := range lines {
		cells[y] = make([]string, maxWidth)
		x := 0
		for _, r := range line {
			width, err := textcell.RuneWidth(r)
			if err != nil {
				t.Fatalf("rune width error: %v", err)
			}
			if width == 0 {
				if x == 0 {
					t.Fatalf("combining rune without base in output row %d", y)
				}
				cells[y][x-1] += string(r)
				continue
			}
			cells[y][x] = string(r)
			for offset := 1; offset < width; offset++ {
				cells[y][x+offset] = "<continuation>"
			}
			x += width
		}
	}
	return subgraphCellGrid{cells: cells, output: output}
}

func (g subgraphCellGrid) boxContaining(t *testing.T, text string) subgraphTestBox {
	t.Helper()
	textX, textY := locateSubgraphText(t, g.output, text)
	best := subgraphTestBox{}
	found := false
	for top := 0; top < len(g.cells); top++ {
		for left := 0; left < len(g.cells[top]); left++ {
			if g.cells[top][left] != "┌" {
				continue
			}
			for right := left + 2; right < len(g.cells[top]); right++ {
				if g.cells[top][right] != "┐" || !g.horizontalBorder(top, left, right) {
					continue
				}
				for bottom := top + 2; bottom < len(g.cells); bottom++ {
					if g.cells[bottom][left] != "└" || g.cells[bottom][right] != "┘" {
						continue
					}
					candidate := subgraphTestBox{left: left, top: top, right: right, bottom: bottom}
					if !candidate.containsCell(textX, textY) || !g.horizontalBorder(bottom, left, right) || !g.verticalBorders(candidate) {
						continue
					}
					if !found || candidate.area() < best.area() {
						best = candidate
						found = true
					}
				}
			}
		}
	}
	if !found {
		t.Fatalf("no Unicode box contains %q at (%d,%d):\n%s", text, textX, textY, g.output)
	}
	return best
}

func (g subgraphCellGrid) topBorderFrameContaining(t *testing.T, text string) subgraphTestBox {
	t.Helper()
	textX, textY := locateSubgraphText(t, g.output, text)
	best := subgraphTestBox{}
	found := false
	for top := 0; top < textY; top++ {
		for left := 0; left < len(g.cells[top]); left++ {
			if g.cells[top][left] != "┌" {
				continue
			}
			for right := left + 2; right < len(g.cells[top]); right++ {
				if g.cells[top][right] != "┐" || !g.horizontalBorder(top, left, right) {
					continue
				}
				if textX <= left || textX >= right {
					continue
				}
				bottom := -1
				for y := textY + 1; y < len(g.cells); y++ {
					if g.cells[y][left] == "└" && g.cells[y][right] == "┘" && g.horizontalBorder(y, left, right) {
						bottom = y
						break
					}
				}
				if bottom < 0 {
					continue
				}
				candidate := subgraphTestBox{left: left, top: top, right: right, bottom: bottom}
				if !found || top > best.top || top == best.top && right-left < best.right-best.left {
					best = candidate
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("no Unicode frame top border contains %q at (%d,%d):\n%s", text, textX, textY, g.output)
	}
	return best
}

func (g subgraphCellGrid) assertFrameSideCell(t *testing.T, name string, frame subgraphTestBox, x, y int) {
	t.Helper()
	if y < frame.top || y > frame.bottom {
		return
	}
	current := g.cells[y][x]
	valid := false
	switch {
	case y == frame.top && x == frame.left:
		valid = current == "┌" || current == "┼"
	case y == frame.top && x == frame.right:
		valid = current == "┐" || current == "┼"
	case y == frame.bottom && x == frame.left:
		valid = current == "└" || current == "┼"
	case y == frame.bottom && x == frame.right:
		valid = current == "┘" || current == "┼"
	default:
		valid = current == "│" || current == "┼"
	}
	if !valid {
		t.Errorf("%s was overwritten at (%d,%d), cell=%q:\n%s", name, x, y, current, g.output)
	}
	if (y == frame.top || y == frame.bottom) && !g.horizontalBorder(y, frame.left, frame.right) {
		t.Errorf("%s horizontal border is not continuous on row %d:\n%s", name, y, g.output)
	}
}

func (g subgraphCellGrid) horizontalBorder(y, left, right int) bool {
	for x := left + 1; x < right; x++ {
		switch g.cells[y][x] {
		case "─", "┼":
		default:
			return false
		}
	}
	return true
}

func (g subgraphCellGrid) verticalBorders(box subgraphTestBox) bool {
	for y := box.top + 1; y < box.bottom; y++ {
		for _, x := range []int{box.left, box.right} {
			switch g.cells[y][x] {
			case "│", "┼":
			default:
				return false
			}
		}
	}
	return true
}

func (g subgraphCellGrid) assertNoUnexpectedDrawingInside(t *testing.T, frame subgraphTestBox, allowed []subgraphTestBox) {
	t.Helper()
	for y := frame.top + 1; y < frame.bottom; y++ {
		for x := frame.left + 1; x < frame.right; x++ {
			insideAllowed := false
			for _, box := range allowed {
				if x >= box.left && x <= box.right && y >= box.top && y <= box.bottom {
					insideAllowed = true
					break
				}
			}
			if insideAllowed {
				continue
			}
			switch g.cells[y][x] {
			case "─", "│", "┄", "┊", "┼", "▶", "▼":
				t.Fatalf("unexpected drawing glyph %q inside unrelated frame %+v at (%d,%d):\n%s", g.cells[y][x], frame, x, y, g.output)
			}
		}
	}
}

func locateSubgraphText(t *testing.T, output, text string) (int, int) {
	t.Helper()
	for y, line := range strings.Split(output, "\n") {
		byteIndex := strings.Index(line, text)
		if byteIndex < 0 {
			continue
		}
		x, err := textcell.Width(line[:byteIndex])
		if err != nil {
			t.Fatalf("text location width error: %v", err)
		}
		return x, y
	}
	t.Fatalf("text %q not found:\n%s", text, output)
	return 0, 0
}

func assertStrictlyContainsBox(t *testing.T, outer, inner subgraphTestBox, name string) {
	t.Helper()
	if outer.left >= inner.left || outer.top >= inner.top || outer.right <= inner.right || outer.bottom <= inner.bottom {
		t.Fatalf("%s %+v does not strictly contain %+v", name, outer, inner)
	}
}

func assertBoxesSeparatedVertically(t *testing.T, first, second subgraphTestBox) {
	t.Helper()
	if first.bottom >= second.top && second.bottom >= first.top {
		t.Fatalf("sibling frames overlap vertically: first=%+v second=%+v", first, second)
	}
}

func assertBoxesSeparatedHorizontally(t *testing.T, first, second subgraphTestBox) {
	t.Helper()
	if first.right >= second.left && second.right >= first.left {
		t.Fatalf("sibling frames overlap horizontally: first=%+v second=%+v", first, second)
	}
}

func assertTextCount(t *testing.T, output, text string, want int) {
	t.Helper()
	if got := strings.Count(output, text); got != want {
		t.Fatalf("text %q count=%d, want=%d:\n%s", text, got, want, output)
	}
}
