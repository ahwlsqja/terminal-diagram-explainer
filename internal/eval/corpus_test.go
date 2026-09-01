package eval_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/er"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/render"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/sequence"
)

type factDefinition struct {
	FactID    string `json:"fact_id"`
	SourceID  string `json:"source_id"`
	Anchor    string `json:"anchor"`
	Statement string `json:"statement"`
}

type promptDefinition struct {
	ID      string           `json:"id"`
	Title   string           `json:"title"`
	Request string           `json:"request"`
	Facts   []factDefinition `json:"facts"`
}

type oracleDefinition struct {
	ID                 string   `json:"id"`
	ExpectedKinds      []string `json:"expected_kinds"`
	ReferenceSource    string   `json:"reference_source"`
	RequiredFactIDs    []string `json:"required_fact_ids"`
	ForbiddenClaims    []string `json:"forbidden_claims"`
	RequiredNotation   []string `json:"required_notation"`
	ProhibitedNotation []string `json:"prohibited_notation"`
	MaxElements        int      `json:"max_elements"`
	Category           string   `json:"category"`
}

func TestEvaluationCorpusSeparatesPromptsFromHiddenOracles(t *testing.T) {
	prompts, promptRaw := loadPrompts(t)
	oracles := loadOracles(t)
	if len(prompts) != 18 || len(oracles) != 18 {
		t.Fatalf("prompts=%d oracles=%d, want 18", len(prompts), len(oracles))
	}
	for _, forbiddenKey := range []string{"reference_source", "required_fact_ids", "forbidden_claims", "required_notation", "prohibited_notation"} {
		if strings.Contains(string(promptRaw), `"`+forbiddenKey+`"`) {
			t.Fatalf("hidden oracle key %q leaked into prompts.json", forbiddenKey)
		}
	}

	oracleByID := make(map[string]oracleDefinition, len(oracles))
	categories := make(map[string]struct{})
	for _, oracle := range oracles {
		if oracle.ID == "" || oracle.Category == "" {
			t.Fatalf("incomplete oracle: %+v", oracle)
		}
		if _, exists := oracleByID[oracle.ID]; exists {
			t.Fatalf("duplicate oracle ID %q", oracle.ID)
		}
		oracleByID[oracle.ID] = oracle
		categories[oracle.Category] = struct{}{}
	}
	for _, category := range []string{"strong-par", "strong-activation", "strong-subgraph", "ssot", "ordering", "adversarial", "schema", "security", "security-redaction"} {
		if _, exists := categories[category]; !exists {
			t.Fatalf("coverage category %q missing", category)
		}
	}

	seenPrompts := make(map[string]struct{}, len(prompts))
	for _, prompt := range prompts {
		prompt := prompt
		t.Run(prompt.ID, func(t *testing.T) {
			if prompt.ID == "" || prompt.Title == "" || prompt.Request == "" || len(prompt.Facts) < 2 {
				t.Fatalf("incomplete prompt: %+v", prompt)
			}
			if _, exists := seenPrompts[prompt.ID]; exists {
				t.Fatalf("duplicate prompt ID %q", prompt.ID)
			}
			seenPrompts[prompt.ID] = struct{}{}
			factIDs := make(map[string]struct{}, len(prompt.Facts))
			for _, fact := range prompt.Facts {
				if fact.FactID == "" || fact.SourceID == "" || fact.Anchor == "" || fact.Statement == "" {
					t.Fatalf("incomplete fact: %+v", fact)
				}
				if _, exists := factIDs[fact.FactID]; exists {
					t.Fatalf("duplicate fact ID %q", fact.FactID)
				}
				factIDs[fact.FactID] = struct{}{}
			}
			oracle, exists := oracleByID[prompt.ID]
			if !exists {
				t.Fatalf("oracle missing")
			}
			for _, required := range oracle.RequiredFactIDs {
				if _, exists := factIDs[required]; !exists {
					t.Fatalf("required fact %q absent from public prompt", required)
				}
			}
			validateReference(t, oracle)
		})
	}
}

func validateReference(t *testing.T, oracle oracleDefinition) {
	t.Helper()
	kind := referenceKind(oracle.ReferenceSource)
	if !contains(oracle.ExpectedKinds, kind) {
		t.Fatalf("reference kind=%q expected=%v", kind, oracle.ExpectedKinds)
	}
	if kind == "none" {
		if oracle.MaxElements != 0 && len(oracle.ExpectedKinds) == 1 && oracle.ExpectedKinds[0] == "none" {
			t.Fatalf("text-only max_elements=%d", oracle.MaxElements)
		}
		return
	}
	output, elements, err := renderReferenceWithOptions(oracle.ReferenceSource, render.DefaultOptions())
	if err != nil || output == "" {
		t.Fatalf("reference output=%q err=%v", output, err)
	}
	if oracle.MaxElements > 0 && elements > oracle.MaxElements {
		t.Fatalf("elements=%d max=%d", elements, oracle.MaxElements)
	}
	for _, notation := range oracle.RequiredNotation {
		if !strings.Contains(oracle.ReferenceSource+"\n"+output, notation) {
			t.Fatalf("required notation %q missing", notation)
		}
	}
	for _, notation := range oracle.ProhibitedNotation {
		if strings.Contains(oracle.ReferenceSource, notation) {
			t.Fatalf("prohibited notation %q present", notation)
		}
	}
	asciiOutput, _, asciiErr := renderReferenceWithOptions(oracle.ReferenceSource, render.Options{ASCII: true, MaxWidth: 240, MaxHeight: 200})
	if asciiErr != nil || asciiOutput == "" || strings.ContainsAny(asciiOutput, "┌┐└┘╭╮╰╯╔╗╚╝─│┄┊┼▶◀▼═║") {
		t.Fatalf("ASCII output=%q err=%v", asciiOutput, asciiErr)
	}
}

func loadPrompts(t *testing.T) ([]promptDefinition, []byte) {
	t.Helper()
	data := readEvalFile(t, "prompts.json")
	var prompts []promptDefinition
	if err := json.Unmarshal(data, &prompts); err != nil {
		t.Fatal(err)
	}
	return prompts, data
}

func loadOracles(t *testing.T) []oracleDefinition {
	t.Helper()
	data := readEvalFile(t, "oracles.json")
	var oracles []oracleDefinition
	if err := json.Unmarshal(data, &oracles); err != nil {
		t.Fatal(err)
	}
	return oracles
}

func readEvalFile(t *testing.T, name string) []byte {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate eval files")
	}
	path := filepath.Join(filepath.Dir(sourceFile), "..", "..", "evals", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func referenceKind(source string) string {
	trimmed := strings.TrimSpace(source)
	switch {
	case trimmed == "":
		return "none"
	case strings.HasPrefix(trimmed, "flowchart "), strings.HasPrefix(trimmed, "graph "):
		return "flow"
	case strings.HasPrefix(trimmed, "sequenceDiagram"):
		return "sequence"
	case strings.HasPrefix(trimmed, "erDiagram"):
		return "er"
	default:
		return "unknown"
	}
}

func renderReferenceWithOptions(source string, options render.Options) (string, int, error) {
	switch referenceKind(source) {
	case "flow":
		graph, err := flow.Parse(source, flow.DefaultLimits())
		if err != nil {
			return "", 0, err
		}
		output, err := render.Flow(graph, options)
		return output, len(graph.Nodes), err
	case "sequence":
		diagram, err := sequence.Parse(source, sequence.DefaultLimits())
		if err != nil {
			return "", 0, err
		}
		output, err := render.Sequence(diagram, options)
		return output, len(diagram.Participants), err
	case "er":
		diagram, err := er.Parse(source, er.DefaultLimits())
		if err != nil {
			return "", 0, err
		}
		output, err := render.ER(diagram, options)
		return output, len(diagram.Entities), err
	default:
		return "", 0, nil
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
