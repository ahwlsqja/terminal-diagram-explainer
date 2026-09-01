package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const erIntegrationSource = `erDiagram
Customer ||--o{ Order : 주문 생성
Customer[고객] { uuid id PK }
Order[주문] { uuid customer_id FK }
`

func TestRunERStdinFileAndASCII(t *testing.T) {
	stdin := invoke(nil, strings.NewReader(erIntegrationSource))
	path := filepath.Join(t.TempDir(), "schema.er")
	if err := os.WriteFile(path, []byte(erIntegrationSource), 0o600); err != nil {
		t.Fatal(err)
	}
	file := invoke([]string{"-f", path}, strings.NewReader("ignored"))
	if stdin != file || stdin.code != 0 || stdin.stderr != "" || !strings.Contains(stdin.stdout, "relationships:") {
		t.Fatalf("stdin=%+v file=%+v", stdin, file)
	}
	ascii := invoke([]string{"-ascii"}, strings.NewReader(erIntegrationSource))
	if ascii.code != 0 || ascii.stderr != "" || strings.ContainsAny(ascii.stdout, "┌┐└┘├┤─│┄┊┼▶◀") {
		t.Fatalf("ASCII=%+v", ascii)
	}
}

func TestRunERExactDispatcherAndNoPartialOutput(t *testing.T) {
	for _, source := range []string{"erDiagram;\nA{}", "erDiagramX\nA{}"} {
		result := invoke(nil, strings.NewReader(source))
		if result.code != 2 || result.stdout != "" || !strings.Contains(result.stderr, "flowchart LR") {
			t.Fatalf("source=%q result=%+v", source, result)
		}
	}
	for _, source := range []string{
		"erDiagram\nA ||--|| Missing : x\nA{}",
		"erDiagram\nA{\nstring email NOT PK\n}",
	} {
		invalid := invoke(nil, strings.NewReader(source))
		if invalid.code != 2 || invalid.stdout != "" || invalid.stderr == "" {
			t.Fatalf("source=%q invalid=%+v", source, invalid)
		}
	}
}

func TestRunERConstraintCanonicalOrder(t *testing.T) {
	source := "erDiagram\nA{\nstring email NOT NULL FK UNIQUE PK\n}"
	path := filepath.Join(t.TempDir(), "constraints.er")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{nil, {"-ascii"}, {"-f", path}} {
		result := invoke(arguments, strings.NewReader(source))
		if result.code != 0 || result.stderr != "" || !strings.Contains(result.stdout, "PK FK UNIQUE NOT NULL string email") {
			t.Fatalf("arguments=%v result=%+v", arguments, result)
		}
	}
}
