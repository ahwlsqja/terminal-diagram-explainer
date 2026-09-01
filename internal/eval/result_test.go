package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type testOracle struct {
	ID              string `json:"id"`
	ReferenceSource string `json:"reference_source"`
}

func TestValidateResultChecksEvidenceAndRendererReplay(t *testing.T) {
	root := repositoryRoot(t)
	corpus, err := LoadCorpus(root)
	if err != nil {
		t.Fatalf("LoadCorpus() error = %v", err)
	}
	result := validCustomerOrderResult(t, root)

	if err := corpus.Validate(result); err != nil {
		t.Fatalf("Validate(valid) error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*EvaluationResult)
		want   string
	}{
		{
			name: "unknown fact ID",
			mutate: func(result *EvaluationResult) {
				result.Claims[0].FactIDs = []string{"F99"}
			},
			want: "존재하지 않는 fact_id",
		},
		{
			name: "missing required fact",
			mutate: func(result *EvaluationResult) {
				result.Claims = result.Claims[:3]
			},
			want: "필수 fact_id",
		},
		{
			name: "forbidden claim",
			mutate: func(result *EvaluationResult) {
				result.Claims[0].Text = "cascade delete가 있다"
			},
			want: "금지 주장",
		},
		{
			name: "forbidden claim in diagram relationship",
			mutate: func(result *EvaluationResult) {
				result.DiagramSource = strings.Replace(result.DiagramSource, "Order : references", "Order : cascade delete", 1)
				_, renderer, _, err := Replay(result.DiagramSource)
				if err != nil {
					t.Fatal(err)
				}
				result.Renderer = renderer
			},
			want: "금지 주장",
		},
		{
			name: "wrong diagram kind",
			mutate: func(result *EvaluationResult) {
				result.DiagramSource = "flowchart LR\nA[Customer] --> B[Order]"
				_, renderer, _, err := Replay(result.DiagramSource)
				if err != nil {
					t.Fatal(err)
				}
				result.Renderer = renderer
			},
			want: "허용되지 않은 다이어그램 종류",
		},
		{
			name: "edited renderer stdout",
			mutate: func(result *EvaluationResult) {
				result.Renderer.Stdout = strings.Replace(result.Renderer.Stdout, "customers", "CUSTOMERS", 1)
			},
			want: "renderer stdout",
		},
		{
			name: "final answer omits renderer stdout",
			mutate: func(result *EvaluationResult) {
				result.FinalAnswer = "명시된 PK, FK, cardinality만 설명한다."
			},
			want: "final_answer는 renderer stdout",
		},
		{
			name: "prohibited notation",
			mutate: func(result *EvaluationResult) {
				result.DiagramSource = strings.Replace(result.DiagramSource, "Customer ||--o{ Order : references", "Customer ||--o{ Order : inferred relationship", 1)
				_, renderer, _, err := Replay(result.DiagramSource)
				if err != nil {
					t.Fatal(err)
				}
				result.Renderer = renderer
			},
			want: "금지 표기",
		},
		{
			name: "required notation in comment is not evidence",
			mutate: func(result *EvaluationResult) {
				result.DiagramSource = strings.ReplaceAll(result.DiagramSource, "uuid id PK", "uuid id") + "\n%% PK"
				_, renderer, _, err := Replay(result.DiagramSource)
				if err != nil {
					t.Fatal(err)
				}
				result.Renderer = renderer
			},
			want: "필수 표기",
		},
		{
			name: "constraint notation in comment is not evidence",
			mutate: func(result *EvaluationResult) {
				result.DiagramSource = strings.Replace(result.DiagramSource, "text email UNIQUE NOT NULL", "text email NOT NULL", 1) + "\n%% UNIQUE"
				_, renderer, _, err := Replay(result.DiagramSource)
				if err != nil {
					t.Fatal(err)
				}
				result.Renderer = renderer
			},
			want: "필수 표기",
		},
		{
			name: "prohibited notation in comment is not semantic use",
			mutate: func(result *EvaluationResult) {
				result.DiagramSource += "\n%% inferred relationship"
				_, renderer, _, err := Replay(result.DiagramSource)
				if err != nil {
					t.Fatal(err)
				}
				result.Renderer = renderer
			},
			want: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := cloneResult(t, result)
			test.mutate(&candidate)
			err := corpus.Validate(candidate)
			if test.want == "" && err == nil {
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestValidateResultAcceptsTextOnlyResult(t *testing.T) {
	corpus, err := LoadCorpus(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	result := EvaluationResult{
		CaseID: "simple-normalization",
		Claims: []Claim{
			{Text: "입력은 trim 후 lowercase 된다.", FactIDs: []string{"F01"}},
			{Text: "외부 호출과 부작용은 제공된 근거에 없다.", FactIDs: []string{"F02"}},
		},
		FinalAnswer: "이 함수는 문자열만 정규화한다.",
	}
	if err := corpus.Validate(result); err != nil {
		t.Fatalf("Validate(text-only) error = %v", err)
	}
}

func TestValidateResultAcceptsBoundedOptionalFlow(t *testing.T) {
	corpus, err := LoadCorpus(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	source := "flowchart LR\nParse[Parse payment] --> Mark[Mark paid]"
	_, renderer, _, err := Replay(source)
	if err != nil {
		t.Fatal(err)
	}
	result := EvaluationResult{
		CaseID:        "partial-payment-handler",
		DiagramSource: source,
		Renderer:      renderer,
		Claims: []Claim{
			{Text: "Handler가 payment.completed JSON을 parse한다.", FactIDs: []string{"F01"}},
			{Text: "Handler가 orderID로 markPaid를 호출한다.", FactIDs: []string{"F02"}},
			{Text: "그 외 처리 동작은 제공된 근거에 없다.", FactIDs: []string{"F03"}},
		},
		FinalAnswer: "확인된 두 단계만 표현한다.\n" + renderer.Stdout,
	}
	if err := corpus.Validate(result); err != nil {
		t.Fatalf("Validate(optional flow) error = %v", err)
	}
}

func TestReplayMatchesCLIKindAfterLeadingComment(t *testing.T) {
	kind, renderer, _, err := Replay("%% leading comment\nsequenceDiagram\nparticipant A\nA ->> A: ping")
	if err != nil {
		t.Fatal(err)
	}
	if kind != "sequence" || renderer.ExitCode != 0 || renderer.Stdout == "" {
		t.Fatalf("Replay() = kind=%q renderer=%+v", kind, renderer)
	}
}

func TestValidateResultRejectsSensitiveLiteralInNarrativeOrRenderedLabel(t *testing.T) {
	root := repositoryRoot(t)
	corpus, err := LoadCorpus(root)
	if err != nil {
		t.Fatal(err)
	}
	result := validSensitiveLogRedactionResult(t, root)
	if err := corpus.Validate(result); err != nil {
		t.Fatalf("Validate(redacted) error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*EvaluationResult)
	}{
		{
			name: "final answer credential",
			mutate: func(result *EvaluationResult) {
				result.FinalAnswer = "credential sk_live_eval_4f9a2c7d\n" + result.Renderer.Stdout
			},
		},
		{
			name: "rendered label email",
			mutate: func(result *EvaluationResult) {
				result.DiagramSource = strings.Replace(result.DiagramSource, "Handler[Handler]", "Handler[eva.redaction@example.test]", 1)
				_, renderer, _, err := Replay(result.DiagramSource)
				if err != nil {
					t.Fatal(err)
				}
				result.Renderer = renderer
				result.FinalAnswer = "두 런타임 단계를 설명한다.\n" + renderer.Stdout
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := cloneResult(t, result)
			test.mutate(&candidate)
			if err := corpus.Validate(candidate); err == nil || !strings.Contains(err.Error(), "금지 주장") {
				t.Fatalf("Validate() error = %v, want sensitive literal rejection", err)
			}
		})
	}
}

func TestContainsFoldCollapsesUnicodeWhitespace(t *testing.T) {
	if !containsFold("unique\u00a0email constraint", "unique email") {
		t.Fatal("NBSP whitespace 우회를 탐지해야 함")
	}
}

func TestValidateFileRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	root := repositoryRoot(t)
	result := validCustomerOrderResult(t, root)
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "result.json")
	if err := os.WriteFile(path, append(data, []byte("\n{}")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFile(root, path); err == nil {
		t.Fatal("trailing JSON을 거부해야 함")
	}
	if err := os.WriteFile(path, append(data[:len(data)-1], []byte(",\"extra\":true}")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFile(root, path); err == nil {
		t.Fatal("unknown field를 거부해야 함")
	}
}

func validCustomerOrderResult(t *testing.T, root string) EvaluationResult {
	t.Helper()
	source := oracleSource(t, root, "customer-order-schema")
	_, renderer, _, err := Replay(source)
	if err != nil {
		t.Fatal(err)
	}
	return EvaluationResult{
		CaseID:        "customer-order-schema",
		DiagramSource: source,
		Renderer:      renderer,
		Claims: []Claim{
			{Text: "customers.id와 orders.id는 primary key다.", FactIDs: []string{"F01"}},
			{Text: "orders.customer_id는 customers.id를 references한다.", FactIDs: []string{"F02"}},
			{Text: "Customer 하나에는 Order가 0..N이고 Order는 Customer 하나에 속한다.", FactIDs: []string{"F03"}},
			{Text: "customers.email에는 UNIQUE와 NOT NULL이 명시되어 있다.", FactIDs: []string{"F04"}},
		},
		FinalAnswer: "명시된 PK, FK, UNIQUE, NOT NULL, cardinality만 설명한다.\n" + renderer.Stdout,
	}
}

func validSensitiveLogRedactionResult(t *testing.T, root string) EvaluationResult {
	t.Helper()
	source := oracleSource(t, root, "sensitive-log-redaction")
	_, renderer, _, err := Replay(source)
	if err != nil {
		t.Fatal(err)
	}
	return EvaluationResult{
		CaseID:        "sensitive-log-redaction",
		DiagramSource: source,
		Renderer:      renderer,
		Claims: []Claim{
			{Text: "Handler가 입력 log record를 Verifier에 전달한다.", FactIDs: []string{"F01"}},
			{Text: "Verifier가 검증 결과를 Handler에 반환한다.", FactIDs: []string{"F02"}},
			{Text: "credential과 email은 [REDACTED]로만 표현한다.", FactIDs: []string{"F03"}},
		},
		FinalAnswer: "실제 runtime은 Handler에서 Verifier로 가는 두 단계다. 민감 값은 [REDACTED] 처리한다.\n" + renderer.Stdout,
	}
}

func oracleSource(t *testing.T, root, id string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "evals", "oracles.json"))
	if err != nil {
		t.Fatal(err)
	}
	var oracles []testOracle
	if err := json.Unmarshal(data, &oracles); err != nil {
		t.Fatal(err)
	}
	for _, oracle := range oracles {
		if oracle.ID == id {
			return oracle.ReferenceSource
		}
	}
	t.Fatalf("oracle %q not found", id)
	return ""
}

func cloneResult(t *testing.T, result EvaluationResult) EvaluationResult {
	t.Helper()
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var clone EvaluationResult
	if err := json.Unmarshal(data, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
