package render

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
)

func TestSVGConvertsTerminalGeometryToVectorPaths(t *testing.T) {
	graph, err := flow.Parse(`flowchart TD
Input[입력] --> Decision{valid?}
Decision -->|yes| Store[(저장)]
Decision -.->|no| Reject[거부]`, flow.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := Flow(graph, Options{MaxWidth: 120, MaxHeight: 80})
	if err != nil {
		t.Fatal(err)
	}

	svg, err := SVG(terminal)
	if err != nil {
		t.Fatal(err)
	}
	document := struct{ XMLName xml.Name }{}
	if err := xml.Unmarshal([]byte(svg), &document); err != nil {
		t.Fatalf("invalid SVG XML: %v\n%s", err, svg)
	}
	for _, want := range []string{"<svg", "<path", "<polygon", "입력", "valid?", "저장", "거부"} {
		if !strings.Contains(svg, want) {
			t.Fatalf("SVG token %q 누락:\n%s", want, svg)
		}
	}
	if strings.ContainsAny(svg, "┌┐└┘├┤┬┴┼─│▶▼╔╗╚╝═║") {
		t.Fatalf("box-drawing glyph가 vector path로 변환되지 않음:\n%s", svg)
	}
}

func TestSVGIsDeterministic(t *testing.T) {
	const terminal = "┌─────┐\n│ API │\n└──┬──┘\n   ▼"
	want, err := SVG(terminal)
	if err != nil {
		t.Fatal(err)
	}
	for iteration := 0; iteration < 100; iteration++ {
		got, renderErr := SVG(terminal)
		if renderErr != nil {
			t.Fatal(renderErr)
		}
		if got != want {
			t.Fatalf("SVG changed at iteration %d", iteration)
		}
	}
}

func TestSVGEscapesTextContent(t *testing.T) {
	svg, err := SVG("┌────────┐\n│ A<&> B │\n└────────┘")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(svg, "A&lt;&amp;&gt; B") || strings.Contains(svg, ">A<&> B<") {
		t.Fatalf("SVG text escaping failed: %s", svg)
	}
}

func TestSVGRejectsOversizedCellBudget(t *testing.T) {
	rows := make([]string, 500)
	for index := range rows {
		rows[index] = strings.Repeat("x", 121)
	}
	if _, err := SVG(strings.Join(rows, "\n")); err == nil {
		t.Fatal("oversized SVG cell budget must fail closed")
	}
}
