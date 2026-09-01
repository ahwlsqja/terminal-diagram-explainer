// Package eval은 공개 prompt와 평가자용 oracle로 설명 산출물을 검증한다.
// 도식은 CLI로 다시 렌더링해 수동 편집이 아닌 현재 renderer의 출력을 증거로 사용한다.
package eval

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/app"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/er"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/sequence"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/state"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

type Fact struct {
	ID        string `json:"fact_id"`
	SourceID  string `json:"source_id"`
	Anchor    string `json:"anchor"`
	Statement string `json:"statement"`
}

type Prompt struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Request string `json:"request"`
	Facts   []Fact `json:"facts"`
}

type Oracle struct {
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

// EvaluationResult는 eval-pack이 받는 이식 가능한 제출 형식이다.
// Renderer stdout은 마지막 개행을 포함한 정확한 CLI stdout이어야 한다.
type EvaluationResult struct {
	CaseID        string         `json:"case_id"`
	DiagramSource string         `json:"diagram_source"`
	Renderer      RendererResult `json:"renderer"`
	Claims        []Claim        `json:"claims"`
	FinalAnswer   string         `json:"final_answer"`
}

type RendererResult struct {
	ExitCode int    `json:"exit_code"`
	Stderr   string `json:"stderr"`
	Stdout   string `json:"stdout"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

type Claim struct {
	Text    string   `json:"text"`
	FactIDs []string `json:"fact_ids"`
}

type Corpus struct {
	prompts     map[string]Prompt
	oracles     map[string]Oracle
	promptOrder []Prompt
}

func LoadCorpus(root string) (*Corpus, error) {
	prompts, err := loadJSON[[]Prompt](filepath.Join(root, "evals", "prompts.json"))
	if err != nil {
		return nil, fmt.Errorf("prompts.json 로드 실패: %w", err)
	}
	oracles, err := loadJSON[[]Oracle](filepath.Join(root, "evals", "oracles.json"))
	if err != nil {
		return nil, fmt.Errorf("oracles.json 로드 실패: %w", err)
	}
	corpus := &Corpus{prompts: make(map[string]Prompt, len(prompts)), oracles: make(map[string]Oracle, len(oracles))}
	for _, prompt := range prompts {
		if prompt.ID == "" {
			return nil, fmt.Errorf("빈 prompt id")
		}
		if _, exists := corpus.prompts[prompt.ID]; exists {
			return nil, fmt.Errorf("중복 prompt id: %s", prompt.ID)
		}
		corpus.prompts[prompt.ID] = prompt
		corpus.promptOrder = append(corpus.promptOrder, prompt)
	}
	for _, oracle := range oracles {
		if oracle.ID == "" {
			return nil, fmt.Errorf("빈 oracle id")
		}
		if _, exists := corpus.oracles[oracle.ID]; exists {
			return nil, fmt.Errorf("중복 oracle id: %s", oracle.ID)
		}
		corpus.oracles[oracle.ID] = oracle
	}
	if len(corpus.prompts) != len(corpus.oracles) {
		return nil, fmt.Errorf("prompt/oracle ID 집합 크기가 다름")
	}
	for id := range corpus.prompts {
		if _, exists := corpus.oracles[id]; !exists {
			return nil, fmt.Errorf("prompt %q의 oracle이 없음", id)
		}
	}
	if len(corpus.promptOrder) != 18 {
		return nil, fmt.Errorf("batch v1 corpus는 18 cases여야 함")
	}
	return corpus, nil
}

func ValidateFile(root, resultPath string) error {
	corpus, err := LoadCorpus(root)
	if err != nil {
		return err
	}
	result, err := loadStrictResult(resultPath)
	if err != nil {
		return fmt.Errorf("결과 JSON 로드 실패: %w", err)
	}
	return corpus.Validate(result)
}

func (c *Corpus) Validate(result EvaluationResult) error {
	if err := validateResultResources(result); err != nil {
		return err
	}
	prompt, exists := c.prompts[result.CaseID]
	if !exists {
		return fmt.Errorf("알 수 없는 case_id: %q", result.CaseID)
	}
	oracle, exists := c.oracles[result.CaseID]
	if !exists {
		return fmt.Errorf("case_id %q의 oracle이 없음", result.CaseID)
	}
	if strings.TrimSpace(result.FinalAnswer) == "" {
		return fmt.Errorf("final_answer가 비어 있음")
	}

	analysis, err := analyze(result.DiagramSource)
	if err != nil {
		return fmt.Errorf("diagram_source 분석 실패: %w", err)
	}
	if !contains(oracle.ExpectedKinds, analysis.kind) {
		return fmt.Errorf("허용되지 않은 다이어그램 종류: %s", analysis.kind)
	}
	if analysis.kind == "none" {
		if result.Renderer != (RendererResult{}) {
			return fmt.Errorf("text-only 결과의 renderer는 비어 있어야 함")
		}
	} else {
		replayed, err := replay(result.DiagramSource)
		if err != nil {
			return err
		}
		if result.Renderer.ExitCode != replayed.ExitCode || result.Renderer.Stderr != replayed.Stderr || result.Renderer.Stdout != replayed.Stdout {
			return fmt.Errorf("renderer stdout/stderr/exit_code가 CLI 재현 결과와 다름")
		}
		if result.Renderer.Width != replayed.Width || result.Renderer.Height != replayed.Height {
			return fmt.Errorf("renderer dimensions가 CLI 재현 결과와 다름: got=%dx%d want=%dx%d", result.Renderer.Width, result.Renderer.Height, replayed.Width, replayed.Height)
		}
	}
	if oracle.MaxElements > 0 && analysis.elements > oracle.MaxElements {
		return fmt.Errorf("요소 수 초과: %d > %d", analysis.elements, oracle.MaxElements)
	}
	for _, notation := range oracle.RequiredNotation {
		if !analysis.hasNotation(notation) {
			return fmt.Errorf("필수 표기 누락: %q", notation)
		}
	}
	for _, notation := range oracle.ProhibitedNotation {
		if analysis.hasNotation(notation) {
			return fmt.Errorf("금지 표기 사용: %q", notation)
		}
	}

	availableFacts := make(map[string]struct{}, len(prompt.Facts))
	for _, fact := range prompt.Facts {
		availableFacts[fact.ID] = struct{}{}
	}
	covered := make(map[string]struct{})
	var claimText strings.Builder
	claimText.Grow(len(result.FinalAnswer) + len(result.Renderer.Stdout) + 1)
	claimText.WriteString(result.FinalAnswer)
	if analysis.kind != "none" {
		claimText.WriteByte('\n')
		claimText.WriteString(result.Renderer.Stdout)
	}
	for _, claim := range result.Claims {
		if strings.TrimSpace(claim.Text) == "" || len(claim.FactIDs) == 0 {
			return fmt.Errorf("claim은 text와 fact_ids를 모두 가져야 함")
		}
		claimText.WriteByte('\n')
		claimText.WriteString(claim.Text)
		for _, factID := range claim.FactIDs {
			if _, exists := availableFacts[factID]; !exists {
				return fmt.Errorf("존재하지 않는 fact_id: %q", factID)
			}
			covered[factID] = struct{}{}
		}
	}
	for _, required := range oracle.RequiredFactIDs {
		if _, exists := covered[required]; !exists {
			return fmt.Errorf("필수 fact_id가 claim에 매핑되지 않음: %q", required)
		}
	}
	allClaims := claimText.String()
	for _, forbidden := range oracle.ForbiddenClaims {
		if containsFold(allClaims, forbidden) {
			return fmt.Errorf("금지 주장 포함: %q", forbidden)
		}
	}
	if analysis.kind != "none" && strings.Count(result.FinalAnswer, result.Renderer.Stdout) != 1 {
		return fmt.Errorf("final_answer는 renderer stdout을 정확히 한 번 포함해야 함")
	}
	return nil
}

// Replay는 비어 있지 않은 source의 CLI 동등 renderer 증거를 반환한다.
func Replay(source string) (kind string, renderer RendererResult, elements int, err error) {
	analysis, err := analyze(source)
	if err != nil {
		return "", RendererResult{}, 0, err
	}
	if analysis.kind == "none" {
		return analysis.kind, RendererResult{}, 0, nil
	}
	renderer, err = replay(source)
	return analysis.kind, renderer, analysis.elements, err
}

func replay(source string) (RendererResult, error) {
	var stdout, stderr bytes.Buffer
	exitCode := app.Run([]string{"-f", "-"}, strings.NewReader(source), &stdout, &stderr)
	if exitCode != 0 {
		return RendererResult{}, fmt.Errorf("renderer 재현 실패: exit=%d stderr=%q", exitCode, stderr.String())
	}
	width, height, err := dimensions(stdout.String())
	if err != nil {
		return RendererResult{}, fmt.Errorf("renderer 출력 크기 계산 실패: %w", err)
	}
	return RendererResult{ExitCode: exitCode, Stderr: stderr.String(), Stdout: stdout.String(), Width: width, Height: height}, nil
}

type sourceAnalysis struct {
	kind     string
	elements int
	features map[string]bool
}

func analyze(source string) (sourceAnalysis, error) {
	if strings.TrimSpace(source) == "" {
		return sourceAnalysis{kind: "none", features: map[string]bool{}}, nil
	}
	header := firstContentLine(source)
	if header == "sequenceDiagram" {
		diagram, err := sequence.Parse(source, sequence.DefaultLimits())
		if err != nil {
			return sourceAnalysis{}, err
		}
		features := map[string]bool{}
		for _, step := range diagram.Steps {
			switch step.Kind {
			case sequence.ActivateStep:
				features["activate "] = true
			case sequence.DeactivateStep:
				features["deactivate "] = true
			case sequence.FragmentStartStep:
				if step.Fragment == sequence.AltFragment {
					features["alt"] = true
				}
				if step.Fragment == sequence.ParFragment {
					features["par "] = true
				}
			case sequence.FragmentBranchStep:
				if step.Branch == sequence.AndBranch {
					features["and "] = true
				}
			}
		}
		return sourceAnalysis{kind: "sequence", elements: len(diagram.Participants), features: features}, nil
	}
	if header == "stateDiagram-v2" {
		diagram, err := state.Parse(source, state.DefaultLimits())
		if err != nil {
			return sourceAnalysis{}, err
		}
		features := map[string]bool{"stateDiagram-v2": true}
		for _, current := range diagram.States {
			features[current.ID] = true
			features[strings.ToLower(current.Label)] = true
		}
		for _, transition := range diagram.Transitions {
			if transition.From.Kind == state.Initial || transition.To.Kind == state.Final {
				features["[*]"] = true
			}
			if transition.From.Kind != state.StateRef || transition.To.Kind != state.StateRef {
				continue
			}
			text := diagram.States[transition.From.Index].ID + " --> " + diagram.States[transition.To.Index].ID
			if transition.Label() != "" {
				text += " : " + transition.Label()
				features[transition.Label()] = true
				features[transition.Event] = true
				if transition.Guard != "" {
					features["["+transition.Guard+"]"] = true
				}
			}
			features[text] = true
		}
		return sourceAnalysis{kind: "state", elements: len(diagram.States), features: features}, nil
	}
	if header == "erDiagram" {
		diagram, err := er.Parse(source, er.DefaultLimits())
		if err != nil {
			return sourceAnalysis{}, err
		}
		features := map[string]bool{}
		for _, entity := range diagram.Entities {
			for _, attr := range entity.Attributes {
				if attr.Key&er.PrimaryKey != 0 {
					features["PK"] = true
				}
				if attr.Key&er.ForeignKey != 0 {
					features["FK"] = true
				}
				if attr.Constraint&er.Unique != 0 {
					features["UNIQUE"] = true
				}
				if attr.Constraint&er.NotNull != 0 {
					features["NOT NULL"] = true
				}
			}
			for _, constraint := range entity.TableConstraints {
				formatted := er.FormatEntityTableConstraint(entity, constraint, diagram.Entities)
				features[formatted] = true
				switch constraint.Kind {
				case er.CompositePrimaryKey:
					features["PRIMARY KEY ("] = true
				case er.CompositeUnique:
					features["UNIQUE ("] = true
				case er.CompositeForeignKey:
					features["FOREIGN KEY ("] = true
				}
			}
		}
		for _, relation := range diagram.Relationships {
			if relation.FromMarker == er.ExactlyOne && relation.ToMarker == er.ZeroOrMany {
				features["||--o{"] = true
			}
			features[strings.ToLower(relation.Label)] = true
		}
		return sourceAnalysis{kind: "er", elements: len(diagram.Entities), features: features}, nil
	}
	graph, err := flow.Parse(source, flow.DefaultLimits())
	if err != nil {
		return sourceAnalysis{}, err
	}
	features := map[string]bool{}
	for _, node := range graph.Nodes {
		if node.Shape == flow.Decision {
			features["{"] = true
		}
		features[strings.ToLower(node.ID)] = true
		features[strings.ToLower(node.Label)] = true
	}
	for _, edge := range graph.Edges {
		features[strings.ToLower(edge.Label)] = true
	}
	for _, subgraph := range graph.Subgraphs {
		features["subgraph"] = true
		features["subgraph "+strings.ToLower(subgraph.ID)] = true
		features["subgraph "+strings.ToLower(subgraph.Label)] = true
	}
	if graphHasCycle(graph) {
		features["feedback"] = true
		features["loop"] = true
	}
	return sourceAnalysis{kind: "flow", elements: len(graph.Nodes), features: features}, nil
}

func (analysis sourceAnalysis) hasNotation(notation string) bool {
	if analysis.features[notation] || analysis.features[strings.ToLower(notation)] {
		return true
	}
	normalized := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(notation)), "[")
	if normalized != "" && analysis.features[normalized] {
		return true
	}
	return false
}

func firstContentLine(source string) string {
	for _, raw := range strings.Split(source, "\n") {
		line := strings.TrimSpace(raw)
		if line != "" && !strings.HasPrefix(line, "%%") {
			return line
		}
	}
	return ""
}

func graphHasCycle(graph *flow.Graph) bool {
	state := make([]uint8, len(graph.Nodes))
	adjacent := make([][]int, len(graph.Nodes))
	for _, edge := range graph.Edges {
		adjacent[edge.From] = append(adjacent[edge.From], edge.To)
	}
	var visit func(int) bool
	visit = func(node int) bool {
		state[node] = 1
		for _, next := range adjacent[node] {
			if state[next] == 1 || (state[next] == 0 && visit(next)) {
				return true
			}
		}
		state[node] = 2
		return false
	}
	for node := range graph.Nodes {
		if state[node] == 0 && visit(node) {
			return true
		}
	}
	return false
}

func dimensions(stdout string) (int, int, error) {
	if !strings.HasSuffix(stdout, "\n") {
		return 0, 0, fmt.Errorf("CLI stdout의 trailing newline이 없음")
	}
	rows := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	width := 0
	for _, row := range rows {
		rowWidth, err := textcell.Width(row)
		if err != nil {
			return 0, 0, err
		}
		if rowWidth > width {
			width = rowWidth
		}
	}
	return width, len(rows), nil
}

func loadJSON[T any](path string) (T, error) {
	var value T
	data, err := os.ReadFile(path)
	if err != nil {
		return value, err
	}
	if err := decodeStrict(data, &value); err != nil {
		return value, err
	}
	return value, nil
}

func loadStrictResult(path string) (EvaluationResult, error) {
	var result EvaluationResult
	data, err := readBoundedFile(path, maxResultBytes)
	if err != nil {
		return EvaluationResult{}, err
	}
	if err := decodeStrict(data, &result); err != nil {
		return EvaluationResult{}, err
	}
	return result, nil
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func validateResultResources(result EvaluationResult) error {
	if len(result.DiagramSource) > 256*1024 || len(result.Renderer.Stdout) > 200*1024 || len(result.Renderer.Stderr) > 16*1024 || len(result.FinalAnswer) > 256*1024 {
		return fmt.Errorf("결과 리소스 제한 초과")
	}
	if len(result.Claims) > 256 {
		return fmt.Errorf("claim 수 제한 초과")
	}
	total := 0
	for _, claim := range result.Claims {
		total += len(claim.Text)
		if len(claim.FactIDs) > 64 {
			return fmt.Errorf("claim fact_id 제한 초과")
		}
	}
	if total > 64*1024 {
		return fmt.Errorf("claim 텍스트 제한 초과")
	}
	return nil
}

func containsFold(text, want string) bool {
	return strings.Contains(foldWhitespace(text), foldWhitespace(want))
}

// foldWhitespace는 시각적으로 같은 Unicode 공백 우회를 막되,
// 나머지 문자는 보존해 exact literal 비교를 결정적으로 유지한다.
func foldWhitespace(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(text)), " ")
}
