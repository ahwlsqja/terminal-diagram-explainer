package eval

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/app"
)

const (
	maxSubmissionBytes = 64 << 20
	maxReviewBytes     = 1 << 20
	maxResultBytes     = 1 << 20
)

type InputError struct{ Err error }

func (e *InputError) Error() string { return e.Err.Error() }
func (e *InputError) Unwrap() error { return e.Err }

const (
	BatchSchema  = "eval-pack.batch.v1"
	ReviewSchema = "eval-pack.review.v1"
	ReportSchema = "eval-pack.report.v1"
)

type BatchSubmission struct {
	Schema       string     `json:"schema"`
	SubjectID    string     `json:"subject_id"`
	CorpusDigest string     `json:"corpus_digest"`
	Runs         []BatchRun `json:"runs"`
}
type BatchRun struct {
	RunID   string             `json:"run_id"`
	Results []EvaluationResult `json:"results"`
}
type BatchReview struct {
	Schema           string       `json:"schema"`
	EvaluatorID      string       `json:"evaluator_id"`
	SubjectID        string       `json:"subject_id"`
	CorpusDigest     string       `json:"corpus_digest"`
	SubmissionDigest string       `json:"submission_digest"`
	Reviews          []CaseReview `json:"reviews"`
}
type CaseReview struct {
	RunID                   string            `json:"run_id"`
	CaseID                  string            `json:"case_id"`
	FactSSOTFidelity        int               `json:"fact_ssot_fidelity"`
	RuntimeOwnershipFailure int               `json:"runtime_ownership_failure"`
	DiagramNotation         int               `json:"diagram_notation"`
	ReadabilityContract     int               `json:"readability_contract"`
	SecurityRedaction       int               `json:"security_redaction"`
	SemanticFailFast        []SemanticFailure `json:"semantic_fail_fast"`
}
type SemanticFailure struct {
	Code         string   `json:"code"`
	EvidenceRefs []string `json:"evidence_refs"`
}
type BatchReport struct {
	Schema           string          `json:"schema"`
	Verdict          string          `json:"verdict"`
	SubjectID        string          `json:"subject_id"`
	EvaluatorID      string          `json:"evaluator_id"`
	Corpus           CorpusReport    `json:"corpus"`
	RendererVersion  string          `json:"renderer_version"`
	SubmissionDigest string          `json:"submission_digest"`
	Runs             []RunReport     `json:"runs"`
	Aggregate        AggregateReport `json:"aggregate"`
	CaseVariance     []CaseVariance  `json:"case_variance"`
	Failures         []SafeFailure   `json:"failures"`
}
type CorpusReport struct {
	CaseCount int    `json:"case_count"`
	Digest    string `json:"digest"`
}
type RunReport struct {
	RunID    string        `json:"run_id"`
	Pass     bool          `json:"pass"`
	Total    Numerator     `json:"total"`
	Fact     Numerator     `json:"fact"`
	Failures []SafeFailure `json:"failures"`
}
type AggregateReport struct {
	PassedRuns   int       `json:"passed_runs"`
	RunCount     int       `json:"run_count"`
	TotalAverage Numerator `json:"total_average"`
	FactAverage  Numerator `json:"fact_average"`
}
type CaseVariance struct {
	CaseID              string   `json:"case_id"`
	Observed            bool     `json:"observed"`
	Attempts            int      `json:"attempts"`
	Sum                 int      `json:"sum"`
	Min                 int      `json:"min"`
	Max                 int      `json:"max"`
	TotalVariance       Rational `json:"total_variance"`
	DistinctResultCount int      `json:"distinct_result_count"`
	StaticPassCount     int      `json:"static_pass_count"`
}
type Numerator struct {
	Numerator   int `json:"numerator"`
	Denominator int `json:"denominator"`
}
type Rational struct {
	Numerator   int `json:"numerator"`
	Denominator int `json:"denominator"`
}
type SafeFailure struct {
	Code   string `json:"code"`
	RunID  string `json:"run_id,omitempty"`
	CaseID string `json:"case_id,omitempty"`
}
type BatchValidationError struct {
	Code string
}

type BatchBinding struct {
	Schema           string          `json:"schema"`
	SubjectID        string          `json:"subject_id"`
	CorpusDigest     string          `json:"corpus_digest"`
	SubmissionDigest string          `json:"submission_digest"`
	RendererVersion  string          `json:"renderer_version"`
	Results          []BindingResult `json:"results"`
}

type BindingResult struct {
	RunID        string `json:"run_id"`
	CaseID       string `json:"case_id"`
	ResultDigest string `json:"result_digest"`
}

func (e *BatchValidationError) Error() string { return e.Code }

var semanticCodes = map[string]bool{"unsupported_claim": true, "par_without_concurrency": true, "activation_without_lifetime": true, "inferred_schema_relationship": true, "sensitive_data_exposure": true, "renderer_evidence_mismatch": true, "required_fact_omission": true}

func EvaluateBatch(c *Corpus, submission BatchSubmission, review BatchReview) (BatchReport, error) {
	if err := validateSubmission(c, submission); err != nil {
		return BatchReport{}, err
	}
	if err := validateReview(c, submission, review); err != nil {
		return BatchReport{}, err
	}
	reviews := map[string]CaseReview{}
	for _, r := range review.Reviews {
		reviews[r.RunID+"\x00"+r.CaseID] = r
	}
	report := BatchReport{Schema: ReportSchema, Verdict: "fail", SubjectID: submission.SubjectID, EvaluatorID: review.EvaluatorID, Corpus: CorpusReport{CaseCount: len(c.promptOrder), Digest: corpusDigest(c)}, RendererVersion: app.Version, SubmissionDigest: SubmissionDigest(submission)}
	caseScores := map[string][]int{}
	caseArtifacts := map[string]map[string]bool{}
	caseStatic := map[string]int{}
	aggregateTotal, aggregateFact := 0, 0
	for _, run := range submission.Runs {
		rr := RunReport{RunID: run.RunID}
		total, fact := 0, 0
		for _, result := range run.Results {
			r := reviews[run.RunID+"\x00"+result.CaseID]
			staticErr := c.Validate(result)
			score := 0
			factScore := 0
			if staticErr == nil {
				score = r.FactSSOTFidelity + r.RuntimeOwnershipFailure + r.DiagramNotation + r.ReadabilityContract + r.SecurityRedaction + 5
				factScore = r.FactSSOTFidelity
			}
			total += score
			fact += factScore
			caseScores[result.CaseID] = append(caseScores[result.CaseID], score)
			data, _ := json.Marshal(result)
			hash := sha256.Sum256(data)
			if caseArtifacts[result.CaseID] == nil {
				caseArtifacts[result.CaseID] = map[string]bool{}
			}
			caseArtifacts[result.CaseID][hex.EncodeToString(hash[:])] = true
			if staticErr == nil {
				caseStatic[result.CaseID]++
			}
			if staticErr != nil {
				rr.Failures = append(rr.Failures, SafeFailure{Code: "static_validation_failed", RunID: run.RunID, CaseID: result.CaseID})
			}
			for _, violation := range r.SemanticFailFast {
				rr.Failures = append(rr.Failures, SafeFailure{Code: violation.Code, RunID: run.RunID, CaseID: result.CaseID})
			}
			if score < 75 {
				rr.Failures = append(rr.Failures, SafeFailure{Code: "case_below_75", RunID: run.RunID, CaseID: result.CaseID})
			}
		}
		rr.Total = Numerator{total, len(run.Results)}
		rr.Fact = Numerator{fact, len(run.Results)}
		if total < 88*len(c.promptOrder) {
			rr.Failures = append(rr.Failures, SafeFailure{Code: "run_average_below_88", RunID: run.RunID})
		}
		if fact < 27*len(c.promptOrder) {
			rr.Failures = append(rr.Failures, SafeFailure{Code: "fact_average_below_27", RunID: run.RunID})
		}
		rr.Pass = len(rr.Failures) == 0
		sortFailures(rr.Failures)
		if rr.Pass {
			report.Aggregate.PassedRuns++
		} else {
			report.Failures = append(report.Failures, rr.Failures...)
		}
		report.Runs = append(report.Runs, rr)
		aggregateTotal += total
		aggregateFact += fact
	}
	report.Aggregate.RunCount = len(report.Runs)
	report.Aggregate.TotalAverage = Numerator{aggregateTotal, len(report.Runs) * len(c.promptOrder)}
	report.Aggregate.FactAverage = Numerator{aggregateFact, len(report.Runs) * len(c.promptOrder)}
	sort.Slice(report.Runs, func(i, j int) bool { return report.Runs[i].RunID < report.Runs[j].RunID })
	sortFailures(report.Failures)
	for _, p := range c.promptOrder {
		scores := caseScores[p.ID]
		v := CaseVariance{CaseID: p.ID, Observed: len(scores) > 1, Attempts: len(scores), TotalVariance: Rational{0, 1}, DistinctResultCount: len(caseArtifacts[p.ID]), StaticPassCount: caseStatic[p.ID]}
		if len(scores) > 0 {
			v.Min, v.Max = scores[0], scores[0]
			for _, x := range scores {
				v.Sum += x
				if x < v.Min {
					v.Min = x
				}
				if x > v.Max {
					v.Max = x
				}
			}
		}
		if len(scores) > 1 {
			sum, sq := 0, 0
			for _, x := range scores {
				sum += x
				sq += x * x
			}
			n := len(scores)
			v.TotalVariance = Rational{n*sq - sum*sum, n * n}
		}
		report.CaseVariance = append(report.CaseVariance, v)
	}
	report.Verdict = "pass"
	for _, rr := range report.Runs {
		if !rr.Pass {
			report.Verdict = "fail"
			break
		}
	}
	return report, nil
}
func sortFailures(values []SafeFailure) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].RunID != values[j].RunID {
			return values[i].RunID < values[j].RunID
		}
		if values[i].CaseID != values[j].CaseID {
			return values[i].CaseID < values[j].CaseID
		}
		return values[i].Code < values[j].Code
	})
}

func EvaluateBatchFiles(root, submissionPath, reviewPath string) (BatchReport, error) {
	c, err := LoadCorpus(root)
	if err != nil {
		return BatchReport{}, &InputError{err}
	}
	var submission BatchSubmission
	if err := decodeFileStrict(submissionPath, maxSubmissionBytes, &submission); err != nil {
		return BatchReport{}, &InputError{err}
	}
	var review BatchReview
	if err := decodeFileStrict(reviewPath, maxReviewBytes, &review); err != nil {
		return BatchReport{}, &InputError{err}
	}
	if err := requireReviewFields(reviewPath, maxReviewBytes); err != nil {
		return BatchReport{}, &InputError{err}
	}
	if submission.Schema != BatchSchema || review.Schema != ReviewSchema {
		return BatchReport{}, &InputError{fmt.Errorf("지원하지 않는 schema version")}
	}
	if err := validateFileResourceEnvelope(submission); err != nil {
		return BatchReport{}, &InputError{err}
	}
	return EvaluateBatch(c, submission, review)
}

func InspectBatchFile(root, submissionPath string) (BatchBinding, error) {
	c, err := LoadCorpus(root)
	if err != nil {
		return BatchBinding{}, &InputError{err}
	}
	var submission BatchSubmission
	if err := decodeFileStrict(submissionPath, maxSubmissionBytes, &submission); err != nil {
		return BatchBinding{}, &InputError{err}
	}
	if submission.Schema != BatchSchema {
		return BatchBinding{}, &InputError{fmt.Errorf("지원하지 않는 schema version")}
	}
	if err := validateFileResourceEnvelope(submission); err != nil {
		return BatchBinding{}, &InputError{err}
	}
	if err := validateSubmission(c, submission); err != nil {
		return BatchBinding{}, err
	}
	binding := BatchBinding{Schema: "eval-pack.binding.v1", SubjectID: submission.SubjectID, CorpusDigest: submission.CorpusDigest, SubmissionDigest: SubmissionDigest(submission), RendererVersion: app.Version}
	for _, run := range submission.Runs {
		for _, result := range run.Results {
			if err := c.Validate(result); err != nil {
				return BatchBinding{}, &BatchValidationError{Code: "static_validation_failed"}
			}
			data, _ := json.Marshal(result)
			sum := sha256.Sum256(data)
			binding.Results = append(binding.Results, BindingResult{RunID: run.RunID, CaseID: result.CaseID, ResultDigest: hex.EncodeToString(sum[:])})
		}
	}
	sort.Slice(binding.Results, func(i, j int) bool {
		if binding.Results[i].RunID == binding.Results[j].RunID {
			return binding.Results[i].CaseID < binding.Results[j].CaseID
		}
		return binding.Results[i].RunID < binding.Results[j].RunID
	})
	return binding, nil
}

func CorpusDigest(c *Corpus) string { return corpusDigest(c) }

func SubmissionDigest(s BatchSubmission) string {
	canonical := s
	canonical.Runs = append([]BatchRun(nil), s.Runs...)
	for i := range canonical.Runs {
		canonical.Runs[i].Results = append([]EvaluationResult(nil), canonical.Runs[i].Results...)
		sort.Slice(canonical.Runs[i].Results, func(a, b int) bool { return canonical.Runs[i].Results[a].CaseID < canonical.Runs[i].Results[b].CaseID })
	}
	sort.Slice(canonical.Runs, func(i, j int) bool { return canonical.Runs[i].RunID < canonical.Runs[j].RunID })
	data, _ := json.Marshal(canonical)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validateFileResourceEnvelope(submission BatchSubmission) error {
	if len(submission.Runs) < 1 || len(submission.Runs) > 3 {
		return fmt.Errorf("run 수 제한 초과")
	}
	for _, run := range submission.Runs {
		for _, result := range run.Results {
			encoded, err := json.Marshal(result)
			if err != nil {
				return fmt.Errorf("결과 직렬화 실패")
			}
			if len(encoded) > maxResultBytes {
				return fmt.Errorf("결과 크기 제한 초과")
			}
		}
	}
	return nil
}

func requireReviewFields(path string, limit int64) error {
	data, err := readBoundedFile(path, limit)
	if err != nil {
		return err
	}
	var raw struct {
		Reviews []map[string]json.RawMessage `json:"reviews"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	required := []string{"run_id", "case_id", "fact_ssot_fidelity", "runtime_ownership_failure", "diagram_notation", "readability_contract", "security_redaction", "semantic_fail_fast"}
	for _, review := range raw.Reviews {
		for _, key := range required {
			if _, ok := review[key]; !ok {
				return fmt.Errorf("review 필수 필드 누락: %s", key)
			}
			if bytes.Equal(bytes.TrimSpace(review[key]), []byte("null")) {
				return fmt.Errorf("review null 필드: %s", key)
			}
		}
	}
	return nil
}

func decodeFileStrict(path string, limit int64, out any) error {
	data, err := readBoundedFile(path, limit)
	if err != nil {
		return err
	}
	return decodeStrict(data, out)
}
func readBoundedFile(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("입력 크기 제한 초과")
	}
	return data, nil
}
func decodeStrict(data []byte, out any) error {
	if err := rejectDuplicateKeys(data); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("JSON은 하나의 객체여야 함")
		}
		return err
	}
	return nil
}
func rejectDuplicateKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := walkJSONValue(dec, 0); err != nil {
		return err
	}
	_, err := dec.Token()
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("JSON은 하나의 값이어야 함")
	}
	return err
}
func walkJSONValue(dec *json.Decoder, depth int) error {
	if depth > 64 {
		return fmt.Errorf("JSON depth 제한 초과")
	}
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok {
				return fmt.Errorf("object key가 아님")
			}
			if seen[name] {
				return fmt.Errorf("중복 JSON key: %s", name)
			}
			seen[name] = true
			if err := walkJSONValue(dec, depth+1); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	case '[':
		for dec.More() {
			if err := walkJSONValue(dec, depth+1); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	default:
		return fmt.Errorf("잘못된 JSON delimiter")
	}
}

func validateSubmission(c *Corpus, s BatchSubmission) error {
	if s.CorpusDigest != corpusDigest(c) {
		return &BatchValidationError{Code: "corpus_digest_mismatch"}
	}
	if s.Schema != BatchSchema || !validID(s.SubjectID, 64) {
		return &BatchValidationError{Code: "invalid_submission"}
	}
	if len(s.Runs) < 1 || len(s.Runs) > 3 {
		return &BatchValidationError{Code: "run_count_out_of_range"}
	}
	runs := map[string]bool{}
	for _, run := range s.Runs {
		if !validID(run.RunID, 64) {
			return &BatchValidationError{Code: "invalid_run_id"}
		}
		if runs[run.RunID] {
			return &BatchValidationError{Code: "duplicate_run"}
		}
		runs[run.RunID] = true
		seen := map[string]bool{}
		if len(run.Results) != len(c.promptOrder) {
			return &BatchValidationError{Code: "missing_case"}
		}
		for _, result := range run.Results {
			encoded, err := json.Marshal(result)
			if err != nil {
				return &BatchValidationError{Code: "invalid_result"}
			}
			if len(encoded) > maxResultBytes {
				return &BatchValidationError{Code: "result_too_large"}
			}
			if _, ok := c.prompts[result.CaseID]; !ok {
				return &BatchValidationError{Code: "unknown_case"}
			}
			if seen[result.CaseID] {
				return &BatchValidationError{Code: "duplicate_case"}
			}
			seen[result.CaseID] = true
		}
	}
	return nil
}
func validateReview(c *Corpus, s BatchSubmission, r BatchReview) error {
	if r.SubjectID != s.SubjectID || r.CorpusDigest != s.CorpusDigest || r.SubmissionDigest != SubmissionDigest(s) {
		return &BatchValidationError{Code: "invalid_review_binding"}
	}
	if r.Schema != ReviewSchema || !validID(r.EvaluatorID, 64) {
		return &BatchValidationError{Code: "invalid_review"}
	}
	seen := map[string]bool{}
	if len(r.Reviews) != len(s.Runs)*len(c.promptOrder) {
		return &BatchValidationError{Code: "missing_review"}
	}
	runs := map[string]bool{}
	for _, run := range s.Runs {
		runs[run.RunID] = true
	}
	for _, x := range r.Reviews {
		k := x.RunID + "\x00" + x.CaseID
		_, knownRun := runs[x.RunID]
		_, knownCase := c.prompts[x.CaseID]
		if !knownRun || !knownCase {
			return &BatchValidationError{Code: "unknown_review"}
		}
		if seen[k] {
			return &BatchValidationError{Code: "duplicate_review"}
		}
		seen[k] = true
		if x.FactSSOTFidelity < 0 || x.FactSSOTFidelity > 30 || x.RuntimeOwnershipFailure < 0 || x.RuntimeOwnershipFailure > 20 || x.DiagramNotation < 0 || x.DiagramNotation > 20 || x.ReadabilityContract < 0 || x.ReadabilityContract > 15 || x.SecurityRedaction < 0 || x.SecurityRedaction > 10 {
			return &BatchValidationError{Code: "score_out_of_range"}
		}
		if x.SemanticFailFast == nil {
			return &BatchValidationError{Code: "invalid_fail_fast"}
		}
		for _, f := range x.SemanticFailFast {
			if !semanticCodes[f.Code] || len(f.EvidenceRefs) == 0 || len(f.EvidenceRefs) > 16 {
				return &BatchValidationError{Code: "invalid_fail_fast"}
			}
			for _, ref := range f.EvidenceRefs {
				if !validEvidenceRef(ref) {
					return &BatchValidationError{Code: "invalid_fail_fast"}
				}
			}
		}
	}
	return nil
}
func validEvidenceRef(ref string) bool {
	if strings.HasPrefix(ref, "fact:") {
		return validID(strings.TrimPrefix(ref, "fact:"), 64)
	}
	if strings.HasPrefix(ref, "claim[") && strings.HasSuffix(ref, "]") {
		n := strings.TrimSuffix(strings.TrimPrefix(ref, "claim["), "]")
		return n != "" && strings.Trim(n, "0123456789") == ""
	}
	return false
}
func validID(v string, max int) bool {
	if len(v) == 0 || len(v) > max {
		return false
	}
	for _, r := range v {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}
func corpusDigest(c *Corpus) string {
	data, _ := json.Marshal(struct {
		P []Prompt
		O []Oracle
	}{c.promptOrder, orderedOracles(c)})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func orderedOracles(c *Corpus) []Oracle {
	out := make([]Oracle, 0, len(c.promptOrder))
	for _, p := range c.promptOrder {
		out = append(out, c.oracles[p.ID])
	}
	return out
}
