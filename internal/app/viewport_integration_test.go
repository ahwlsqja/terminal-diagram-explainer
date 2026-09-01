package app

import (
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

const wideFlowSource = `flowchart LR
COLLECTOR[Collector]
QUEUE[Queue raw payload]
CONSUMER[Consumer]
RAW[CH raw INSERT block]
IMV[CH incremental MV]
WITNESS[(Append revision witness)]
DECISION[Exact duplicate collapse and A-B HOLD]
COLLECTOR --> QUEUE
QUEUE --> CONSUMER
CONSUMER --> RAW
RAW --> IMV
IMV --> WITNESS
WITNESS --> DECISION
`

func TestRunAutoFitsFlowWithinRequestedWidth(t *testing.T) {
	result := invoke([]string{"-width", "120", "-fit"}, strings.NewReader(wideFlowSource))
	if result.code != 0 || result.stderr != "" {
		t.Fatalf("auto-fit 실패: %+v", result)
	}
	lines := strings.Split(strings.TrimSuffix(result.stdout, "\n"), "\n")
	if len(lines) <= 3 {
		t.Fatalf("218-cell LR 입력이 좁은 viewport에서 그대로 유지됨:\n%s", result.stdout)
	}
	for index, line := range lines {
		width, err := textcell.Width(line)
		if err != nil {
			t.Fatalf("출력 %d행 폭 계산 실패: %v", index+1, err)
		}
		if width > 120 {
			t.Fatalf("출력 %d행 폭=%d, limit=120:\n%s", index+1, width, result.stdout)
		}
	}
}

func TestRunWidthWithoutFitFailsAtomically(t *testing.T) {
	result := invoke([]string{"-width", "120"}, strings.NewReader(wideFlowSource))
	if result.code != 2 || result.stdout != "" || !strings.Contains(result.stderr, "출력 폭 제한 초과") {
		t.Fatalf("좁은 viewport가 명시적 실패로 닫히지 않음: %+v", result)
	}
}

func TestRunRejectsInvalidViewportBounds(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "zero width", args: []string{"-width", "0"}, want: "viewport 폭"},
		{name: "oversized width", args: []string{"-width", "513"}, want: "viewport 폭"},
		{name: "zero height", args: []string{"-height", "0"}, want: "viewport 높이"},
		{name: "oversized height", args: []string{"-height", "513"}, want: "viewport 높이"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := invoke(test.args, strings.NewReader("flowchart LR\nA --> B\n"))
			if result.code != 2 || result.stdout != "" || !strings.Contains(result.stderr, test.want) {
				t.Fatalf("args=%v result=%+v", test.args, result)
			}
		})
	}
}
