package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/eval"
)

func TestRunLegacyAndFlagContracts(t *testing.T) {
	root := repoRoot(t)
	result := filepath.Join(t.TempDir(), "result.json")
	data := `{"case_id":"simple-normalization","diagram_source":"","renderer":{"exit_code":0,"stderr":"","stdout":"","width":0,"height":0},"claims":[{"text":"trim","fact_ids":["F01"]},{"text":"lower","fact_ids":["F02"]}],"final_answer":"normalization"}`
	if err := os.WriteFile(result, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"-root", root, "-f", result}, &out, &errOut); code != 0 || out.String() != "평가 결과 검증 통과\n" {
		t.Fatalf("legacy code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	for _, args := range [][]string{{}, {"-f", result, "-batch", "x", "-review", "y"}, {"-batch", "x"}, {"-corpus-digest", "-f", result}, {"-inspect-batch", result, "-f", result}} {
		if code := run(args, &out, &errOut); code != 2 {
			t.Fatalf("args=%v code=%d", args, code)
		}
	}
}

func TestRunCorpusDigestAndInspectBinding(t *testing.T) {
	root, dir := repoRoot(t), t.TempDir()
	var out, errOut bytes.Buffer
	if code := run([]string{"-root", root, "-corpus-digest"}, &out, &errOut); code != 0 || !regexp.MustCompile(`^[0-9a-f]{64}\n$`).MatchString(out.String()) {
		t.Fatalf("digest code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	submission, _ := cliBatchFixture(t, root)
	path := filepath.Join(dir, "submission.json")
	writeJSON(t, path, submission)
	out.Reset()
	errOut.Reset()
	if code := run([]string{"-root", root, "-inspect-batch", path}, &out, &errOut); code != 0 {
		t.Fatalf("inspect code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	for _, forbidden := range []string{"sk_live_eval_4f9a2c7d", "eva.redaction@example.test", "final_answer", "diagram_source"} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("inspect output leak: %s", forbidden)
		}
	}
}

func TestRunBatchInputFailuresAreExit2(t *testing.T) {
	root, dir := repoRoot(t), t.TempDir()
	bad, review := filepath.Join(dir, "bad.json"), filepath.Join(dir, "review.json")
	if err := os.WriteFile(bad, []byte(`{`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(review, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"-root", root, "-batch", bad, "-review", review}, &out, &errOut); code != 2 || out.Len() != 0 {
		t.Fatalf("malformed code=%d out=%q", code, out.String())
	}
	if err := os.WriteFile(bad, []byte(`{"schema":"bad","subject_id":"s","runs":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(review, []byte(`{"schema":"bad","evaluator_id":"e","reviews":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errOut.Reset()
	if code := run([]string{"-root", root, "-batch", bad, "-review", review}, &out, &errOut); code != 2 {
		t.Fatalf("version code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
}

func TestRunBatchPassAndFailReports(t *testing.T) {
	root, dir := repoRoot(t), t.TempDir()
	submission, review := cliBatchFixture(t, root)
	submissionPath, reviewPath := filepath.Join(dir, "submission.json"), filepath.Join(dir, "review.json")
	writeJSON(t, submissionPath, submission)
	writeJSON(t, reviewPath, review)
	var out, errOut bytes.Buffer
	if code := run([]string{"-root", root, "-batch", submissionPath, "-review", reviewPath}, &out, &errOut); code != 0 {
		t.Fatalf("pass code=%d out=%s err=%s", code, out.String(), errOut.String())
	}
	var report eval.BatchReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.Verdict != "pass" {
		t.Fatalf("report=%s err=%v", out.String(), err)
	}
	review.Reviews[0].ReadabilityContract = 1
	writeJSON(t, reviewPath, review)
	out.Reset()
	errOut.Reset()
	if code := run([]string{"-root", root, "-batch", submissionPath, "-review", reviewPath}, &out, &errOut); code != 1 {
		t.Fatalf("fail code=%d", code)
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.Verdict != "fail" {
		t.Fatalf("fail report=%s err=%v", out.String(), err)
	}
}

func TestRunBatchReportsConcreteStructuralCodes(t *testing.T) {
	root, dir := repoRoot(t), t.TempDir()
	submission, review := cliBatchFixture(t, root)
	submission.Runs[0].Results = submission.Runs[0].Results[1:]
	submissionPath, reviewPath := filepath.Join(dir, "submission.json"), filepath.Join(dir, "review.json")
	writeJSON(t, submissionPath, submission)
	writeJSON(t, reviewPath, review)
	var out, errOut bytes.Buffer
	if code := run([]string{"-root", root, "-batch", submissionPath, "-review", reviewPath}, &out, &errOut); code != 1 || !strings.Contains(out.String(), `"code":"missing_case"`) {
		t.Fatalf("missing case code=%d out=%q err=%q", code, out.String(), errOut.String())
	}

	submission, review = cliBatchFixture(t, root)
	review.SubmissionDigest = strings.Repeat("0", 64)
	writeJSON(t, submissionPath, submission)
	writeJSON(t, reviewPath, review)
	out.Reset()
	errOut.Reset()
	if code := run([]string{"-root", root, "-batch", submissionPath, "-review", reviewPath}, &out, &errOut); code != 1 || !strings.Contains(out.String(), `"code":"invalid_review_binding"`) {
		t.Fatalf("binding code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
}

func TestInvalidReportDoesNotReflectUntrustedIdentifiers(t *testing.T) {
	root, dir := repoRoot(t), t.TempDir()
	submission, review := cliBatchFixture(t, root)
	submissionPath, reviewPath := filepath.Join(dir, "submission.json"), filepath.Join(dir, "review.json")
	secretCase := "sk_live_eval_4f9a2c7d"
	submission.Runs[0].Results[0].CaseID = secretCase
	writeJSON(t, submissionPath, submission)
	writeJSON(t, reviewPath, review)
	var out, errOut bytes.Buffer
	if code := run([]string{"-root", root, "-batch", submissionPath, "-review", reviewPath}, &out, &errOut); code != 1 || !strings.Contains(out.String(), `"code":"unknown_case"`) || strings.Contains(out.String(), secretCase) {
		t.Fatalf("unknown case code=%d out=%q err=%q", code, out.String(), errOut.String())
	}

	submission, review = cliBatchFixture(t, root)
	privateRun := "sk_live_eval_4f9a2c7d"
	submission.Runs[0].RunID = privateRun
	submission.Runs[0].Results = submission.Runs[0].Results[1:]
	writeJSON(t, submissionPath, submission)
	writeJSON(t, reviewPath, review)
	out.Reset()
	errOut.Reset()
	if code := run([]string{"-root", root, "-batch", submissionPath, "-review", reviewPath}, &out, &errOut); code != 1 || !strings.Contains(out.String(), `"code":"missing_case"`) || strings.Contains(out.String(), privateRun) {
		t.Fatalf("invalid run code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
}

func TestRunBatchRejectsNonIntegerOrMissingReviewAxes(t *testing.T) {
	root, dir := repoRoot(t), t.TempDir()
	submission, review := cliBatchFixture(t, root)
	submissionPath, reviewPath := filepath.Join(dir, "submission.json"), filepath.Join(dir, "review.json")
	writeJSON(t, submissionPath, submission)
	data, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	token := []byte(`"security_redaction":1`)
	mutations := [][]byte{
		bytes.Replace(data, token, []byte(`"security_redaction":null`), 1),
		bytes.Replace(data, token, []byte(`"security_redaction":1.5`), 1),
		bytes.Replace(data, append([]byte(","), token...), nil, 1),
		bytes.Replace(data, token, []byte(`"security_redaction":1,"extra_axis":0`), 1),
	}
	for index, candidate := range mutations {
		if err := os.WriteFile(reviewPath, candidate, 0o600); err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		if code := run([]string{"-root", root, "-batch", submissionPath, "-review", reviewPath}, &out, &errOut); code != 2 || out.Len() != 0 {
			t.Fatalf("mutation %d code=%d out=%q err=%q", index, code, out.String(), errOut.String())
		}
	}
}

type cliPrompt struct {
	ID    string `json:"id"`
	Facts []struct {
		ID string `json:"fact_id"`
	} `json:"facts"`
}
type cliOracle struct {
	ID     string `json:"id"`
	Source string `json:"reference_source"`
}

func cliBatchFixture(t *testing.T, root string) (eval.BatchSubmission, eval.BatchReview) {
	t.Helper()
	var prompts []cliPrompt
	var oracles []cliOracle
	readJSON(t, filepath.Join(root, "evals", "prompts.json"), &prompts)
	readJSON(t, filepath.Join(root, "evals", "oracles.json"), &oracles)
	byID := map[string]string{}
	for _, o := range oracles {
		byID[o.ID] = o.Source
	}
	run := eval.BatchRun{RunID: "run-1"}
	review := eval.BatchReview{Schema: eval.ReviewSchema, EvaluatorID: "evaluator-1"}
	for _, p := range prompts {
		r := eval.EvaluationResult{CaseID: p.ID, FinalAnswer: "근거 설명"}
		if source := byID[p.ID]; source != "" {
			_, renderer, _, err := eval.Replay(source)
			if err != nil {
				t.Fatal(err)
			}
			r.DiagramSource, r.Renderer = source, renderer
			r.FinalAnswer += "\n" + renderer.Stdout
		}
		for _, f := range p.Facts {
			r.Claims = append(r.Claims, eval.Claim{Text: "근거", FactIDs: []string{f.ID}})
		}
		run.Results = append(run.Results, r)
		review.Reviews = append(review.Reviews, eval.CaseReview{RunID: "run-1", CaseID: p.ID, FactSSOTFidelity: 27, RuntimeOwnershipFailure: 20, DiagramNotation: 20, ReadabilityContract: 15, SecurityRedaction: 1, SemanticFailFast: []eval.SemanticFailure{}})
	}
	corpus, err := eval.LoadCorpus(root)
	if err != nil {
		t.Fatal(err)
	}
	submission := eval.BatchSubmission{Schema: eval.BatchSchema, SubjectID: "subject-1", CorpusDigest: eval.CorpusDigest(corpus), Runs: []eval.BatchRun{run}}
	review.SubjectID, review.CorpusDigest, review.SubmissionDigest = submission.SubjectID, submission.CorpusDigest, eval.SubmissionDigest(submission)
	return submission, review
}
func readJSON(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatal(err)
	}
}
func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
