package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const subgraphIntegrationSource = `flowchart TD
subgraph Service[서비스 계층]
  API[API] --> Worker[Worker]
  subgraph Data[데이터 계층]
    DB[(Store)]
  end
  Worker --> DB
end
`

func TestRunSubgraphStdinAndFileAreEquivalent(t *testing.T) {
	fromStdin := invoke(nil, strings.NewReader(subgraphIntegrationSource))

	path := filepath.Join(t.TempDir(), "subgraph.mmd")
	if err := os.WriteFile(path, []byte(subgraphIntegrationSource), 0o600); err != nil {
		t.Fatal(err)
	}
	fromFile := invoke([]string{"-f", path}, strings.NewReader("ignored invalid stdin"))

	if fromStdin != fromFile {
		t.Fatalf("stdin=%+v file=%+v", fromStdin, fromFile)
	}
	if fromStdin.code != 0 || fromStdin.stderr != "" {
		t.Fatalf("subgraph result=%+v", fromStdin)
	}
	if !strings.HasSuffix(fromStdin.stdout, "\n") || strings.HasSuffix(fromStdin.stdout, "\n\n") {
		t.Fatalf("stdout must have exactly one trailing newline: %q", fromStdin.stdout)
	}
	for _, label := range []string{"서비스 계층", "데이터 계층"} {
		assertCLIFrameLabel(t, fromStdin.stdout, label)
	}
	for _, label := range []string{"API", "Worker", "Store"} {
		if !strings.Contains(fromStdin.stdout, label) {
			t.Fatalf("node label %q missing from stdout:\n%s", label, fromStdin.stdout)
		}
	}
}

func TestRunSubgraphASCIIStdinAndFileAreEquivalent(t *testing.T) {
	fromStdin := invoke([]string{"-ascii"}, strings.NewReader(subgraphIntegrationSource))

	path := filepath.Join(t.TempDir(), "subgraph-ascii.mmd")
	if err := os.WriteFile(path, []byte(subgraphIntegrationSource), 0o600); err != nil {
		t.Fatal(err)
	}
	fromFile := invoke([]string{"-ascii", "-f", path}, strings.NewReader("ignored"))

	if fromStdin != fromFile {
		t.Fatalf("stdin=%+v file=%+v", fromStdin, fromFile)
	}
	if fromStdin.code != 0 || fromStdin.stderr != "" {
		t.Fatalf("ASCII subgraph result=%+v", fromStdin)
	}
	if strings.ContainsAny(fromStdin.stdout, "┌┐└┘┏┓┗┛╔╗╚╝─━═│┃║▶▼┼") {
		t.Fatalf("ASCII subgraph output contains Unicode drawing glyphs:\n%s", fromStdin.stdout)
	}
	for _, label := range []string{"서비스 계층", "데이터 계층"} {
		assertCLIFrameLabel(t, fromStdin.stdout, label)
	}
}

func assertCLIFrameLabel(t *testing.T, output, label string) {
	t.Helper()
	if strings.Count(output, label) != 1 {
		t.Fatalf("subgraph label %q count != 1:\n%s", label, output)
	}
	for _, line := range strings.Split(output, "\n") {
		labelIndex := strings.Index(line, label)
		if labelIndex < 0 {
			continue
		}
		before := line[:labelIndex]
		after := line[labelIndex+len(label):]
		if strings.ContainsAny(before, "|│┃║") && strings.ContainsAny(after, "|│┃║") {
			return
		}
	}
	t.Fatalf("subgraph label %q is not inside vertical frame borders:\n%s", label, output)
}
