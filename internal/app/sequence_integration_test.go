package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sequenceIntegrationSource = "\r\n%% leading comment\r\nsequenceDiagram\r\nparticipant Client as 클라이언트\r\nparticipant API as API 서버\r\nClient->>API: 요청 e\u0301\r\nAPI-->>Client: 201 정상\r\n"

func TestRunSequenceFirstEffectiveHeaderAndFileAreEquivalent(t *testing.T) {
	fromStdin := invoke(nil, strings.NewReader(sequenceIntegrationSource))
	path := filepath.Join(t.TempDir(), "sequence.mmd")
	if err := os.WriteFile(path, []byte(sequenceIntegrationSource), 0o600); err != nil {
		t.Fatal(err)
	}
	fromFile := invoke([]string{"-f", path}, strings.NewReader("ignored invalid stdin"))
	if fromStdin != fromFile {
		t.Fatalf("stdin=%+v file=%+v", fromStdin, fromFile)
	}
	if fromStdin.code != 0 || fromStdin.stderr != "" {
		t.Fatalf("sequence result=%+v", fromStdin)
	}
	if !strings.HasSuffix(fromStdin.stdout, "\n") || strings.HasSuffix(fromStdin.stdout, "\n\n") {
		t.Fatalf("stdout must have exactly one trailing newline: %q", fromStdin.stdout)
	}
	for _, label := range []string{"클라이언트", "API 서버", "요청 e\u0301", "201 정상"} {
		if !strings.Contains(fromStdin.stdout, label) {
			t.Fatalf("missing %q in:\n%s", label, fromStdin.stdout)
		}
	}
}

func TestRunSequenceASCIIStdinAndFileAreEquivalent(t *testing.T) {
	fromStdin := invoke([]string{"-ascii"}, strings.NewReader(sequenceIntegrationSource))
	path := filepath.Join(t.TempDir(), "sequence-ascii.mmd")
	if err := os.WriteFile(path, []byte(sequenceIntegrationSource), 0o600); err != nil {
		t.Fatal(err)
	}
	fromFile := invoke([]string{"-ascii", "-f", path}, strings.NewReader("ignored"))
	if fromStdin != fromFile {
		t.Fatalf("stdin=%+v file=%+v", fromStdin, fromFile)
	}
	if fromStdin.code != 0 || fromStdin.stderr != "" {
		t.Fatalf("ASCII sequence result=%+v", fromStdin)
	}
	if strings.ContainsAny(fromStdin.stdout, "┌┐└┘─│▶◀▼▲┄┊┼") {
		t.Fatalf("ASCII output contains Unicode drawing glyphs:\n%s", fromStdin.stdout)
	}
	for _, label := range []string{"클라이언트", "API 서버", "요청 e\u0301", "201 정상"} {
		if !strings.Contains(fromStdin.stdout, label) {
			t.Fatalf("ASCII output lost label %q:\n%s", label, fromStdin.stdout)
		}
	}
}

func TestRunSequenceDoesNotCommitOutputOnParseOrRenderFailure(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		wantStderr string
	}{
		{
			name: "unknown endpoint",
			source: "sequenceDiagram\n" +
				"participant Client\n" +
				"Client->>Missing: request\n",
			wantStderr: "participant",
		},
		{
			name: "render bounds",
			source: "sequenceDiagram\n" +
				"participant A as A" + strings.Repeat("x", 95) + "\n" +
				"participant B as B" + strings.Repeat("x", 95) + "\n" +
				"participant C as C" + strings.Repeat("x", 95) + "\n" +
				"A->>B: first\n" +
				"B->>C: second\n",
			wantStderr: "출력 경계",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := invoke(nil, strings.NewReader(test.source))
			if got.code != 2 || got.stdout != "" || !strings.Contains(got.stderr, test.wantStderr) {
				t.Fatalf("result=%+v", got)
			}
		})
	}
}

func TestRunSequenceLikeHeadersRemainOnFlowErrorPathUnlessExact(t *testing.T) {
	for _, source := range []string{
		"sequenceDiagram;\nparticipant A\nA->>A: self\n",
		"sequenceDiagramX\nparticipant A\nA->>A: self\n",
	} {
		got := invoke(nil, strings.NewReader(source))
		if got.code != 2 || got.stdout != "" || !strings.Contains(got.stderr, "flowchart LR") {
			t.Fatalf("source=%q result=%+v", source, got)
		}
	}
}

func TestRunFlowGoldenAndInvalidDiagnosticRemainUnchangedAfterSequenceDispatch(t *testing.T) {
	golden := invoke(nil, strings.NewReader("flowchart LR\nA[Start] --> B[Done]"))
	const want = "┌───────┐          ┌──────┐\n" +
		"│ Start │─────────▶│ Done │\n" +
		"└───────┘          └──────┘\n"
	if golden.code != 0 || golden.stderr != "" || golden.stdout != want {
		t.Fatalf("Flow golden drifted: %+v", golden)
	}

	invalid := invoke(nil, strings.NewReader("flowchart LR\nclassDef foo invalid\n"))
	const wantDiagnostic = "2행 10열: 지원하지 않는 문법; `-->` 또는 `-.->`가 필요함\n"
	if invalid.code != 2 || invalid.stdout != "" || invalid.stderr != wantDiagnostic {
		t.Fatalf("Flow invalid diagnostic drifted: %+v", invalid)
	}
}
