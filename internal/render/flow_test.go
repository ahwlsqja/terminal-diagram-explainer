package render

import (
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
)

func TestFlowBranchRendersDeterministically(t *testing.T) {
	input := `flowchart LR
Receive[이벤트 수신] --> Validate{유효한가?}
Validate -->|yes| Store[(ClickHouse VIEW)]
Validate -.->|no| Drop[거부 + 관측]`
	graph, err := flow.Parse(input, flow.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	first, err := Flow(graph, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		got, renderErr := Flow(graph, DefaultOptions())
		if renderErr != nil {
			t.Fatal(renderErr)
		}
		if got != first {
			t.Fatalf("render changed at iteration %d", i)
		}
	}
	for _, label := range []string{"이벤트 수신", "유효한가?", "ClickHouse VIEW", "거부 + 관측", "yes", "no"} {
		if !strings.Contains(first, label) {
			t.Fatalf("missing label %q in:\n%s", label, first)
		}
	}
}

func TestFlowRejectsCycle(t *testing.T) {
	graph, err := flow.Parse("flowchart LR\nA --> B\nB --> A", flow.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Flow(graph, DefaultOptions()); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestFlowOutputStaysWithinBounds(t *testing.T) {
	var source strings.Builder
	source.WriteString("flowchart LR\n")
	for i := 0; i < 20; i++ {
		source.WriteString(nodeID(i) + " --> " + nodeID(i+1) + "\n")
	}
	graph, err := flow.Parse(source.String(), flow.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	options := DefaultOptions()
	options.MaxWidth = 120
	if _, err := Flow(graph, options); err == nil {
		t.Fatal("expected width limit error")
	}
}

func nodeID(i int) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	return string(alphabet[i%len(alphabet)]) + string(alphabet[(i/len(alphabet))%len(alphabet)])
}
