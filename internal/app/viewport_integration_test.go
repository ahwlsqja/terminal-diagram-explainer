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

const operationalStatusFlowSource = `flowchart TD
RAW[(raw_events)] -->|insert-block IMV| WITNESS[(159 merged: typed witness)]
LEGACY[156/157 legacy Producer] -.-> PAUSE[158 merged: 1-year pause]
WITNESS --> WINDOW[160 WIP: physical window 20k]
WINDOW --> CANDIDATE[semantic candidate 12k]
CANDIDATE --> DECISION{revision decision}
DECISION -->|unique tuple| CURRENT[current/hour 기록]
DECISION -->|equal-version A/B| HOLD[HOLD quarantine]
CURRENT --> EVIDENCE[publication evidence]
HOLD --> EVIDENCE
EVIDENCE --> RECEIPT[(terminal receipt)]
RECEIPT --> CURSOR[(contiguous cursor)]
`

const complexDependencyFlowSource = `flowchart TD
Base[완료 2233 · 2234] --> V3[진행 2235: v3 Producer]
V3 --> Cap[대기 2236: fault · capacity]
Cap --> Cad[대기 2237: cadence 활성]
Session[진행 2179: Session-Hour] --> Verify[미등록: Verification]
Dict[진행 2147: Dictionary] --> Verify
Need{대기 2180: Runner 필요?}
Need -->|true| Runner[조건부 2175 · 2149]
Need -->|false| Historical[미등록: Historical build]
Runner --> Historical
Cad --> Verify
Historical --> Verify
Verify --> Activation[미등록: Activation]
Activation --> Cutover[미등록: API · SPA · Production]
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

func TestRunKeepsOperationalStatusFlowCompact(t *testing.T) {
	result := invoke([]string{"-width", "120", "-fit"}, strings.NewReader(operationalStatusFlowSource))
	if result.code != 0 || result.stderr != "" {
		t.Fatalf("status flow 렌더 실패: %+v", result)
	}
	width, height := outputDimensions(t, result.stdout)
	if width > 120 || height > 50 {
		t.Fatalf("status flow=%dx%d, want width<=120 height<=50:\n%s", width, height, result.stdout)
	}
}

func TestRunRendersComplexDependencyFlowWithoutManualFallback(t *testing.T) {
	result := invoke([]string{"-width", "120", "-fit"}, strings.NewReader(complexDependencyFlowSource))
	if result.code != 0 || result.stderr != "" {
		t.Fatalf("dependency flow가 수동 fallback을 요구함: %+v", result)
	}
	width, _ := outputDimensions(t, result.stdout)
	if width > 120 {
		t.Fatalf("dependency flow width=%d, limit=120:\n%s", width, result.stdout)
	}
	for _, label := range []string{"완료 2233 · 2234", "진행 2235: v3 Producer", "미등록: Verification", "미등록: API · SPA · Production"} {
		if !strings.Contains(result.stdout, label) {
			t.Fatalf("dependency flow label %q 누락:\n%s", label, result.stdout)
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

func outputDimensions(t *testing.T, output string) (int, int) {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	maximumWidth := 0
	for index, line := range lines {
		width, err := textcell.Width(line)
		if err != nil {
			t.Fatalf("출력 %d행 폭 계산 실패: %v", index+1, err)
		}
		if width > maximumWidth {
			maximumWidth = width
		}
	}
	return maximumWidth, len(lines)
}
