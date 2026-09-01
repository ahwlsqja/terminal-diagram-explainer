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

func TestRunERCompositeConstraintsStdinFileAndASCII(t *testing.T) {
	source := `erDiagram
Order {
  string tenant_id
  string id
  string account_id
  PRIMARY KEY (tenant_id, id)
  FOREIGN KEY (tenant_id, account_id) REFERENCES Account(tenant_id, id)
}
Account {
  string tenant_id
  string id
}`
	path := filepath.Join(t.TempDir(), "composite.er")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{nil, {"-ascii"}, {"-f", path}} {
		result := invoke(arguments, strings.NewReader(source))
		if result.code != 0 || result.stderr != "" || !strings.Contains(result.stdout, "FOREIGN KEY (tenant_id, account_id) REFERENCES Account(tenant_id, id)") || strings.Contains(result.stdout, "relationships:") {
			t.Fatalf("arguments=%v result=%+v", arguments, result)
		}
	}
	invalid := invoke(nil, strings.NewReader("erDiagram\nA{\nstring a\nstring b\nUNIQUE (a)\n}"))
	if invalid.code != 2 || invalid.stdout != "" || invalid.stderr == "" {
		t.Fatalf("invalid=%+v", invalid)
	}
}

func TestRunERCompositeConstraintDefaultWidthBoundary(t *testing.T) {
	first := strings.Repeat("a", 64)
	second := strings.Repeat("b", 64)
	third := strings.Repeat("c", 64)
	for _, test := range []struct {
		name   string
		last   string
		code   int
		stdout bool
	}{
		{name: "236 cells", last: strings.Repeat("d", 29), code: 0, stdout: true},
		{name: "237 cells", last: strings.Repeat("d", 30), code: 2, stdout: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := "erDiagram\nA{\nstring " + first + "\nstring " + second + "\nstring " + third + "\nstring " + test.last + "\nUNIQUE (" + first + ", " + second + ", " + third + ", " + test.last + ")\n}"
			result := invoke(nil, strings.NewReader(source))
			if result.code != test.code || (result.stdout != "") != test.stdout {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}
