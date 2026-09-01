package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvaluateBatchPassesExactBoundaries(t *testing.T) {
	root := repositoryRoot(t)
	corpus, err := LoadCorpus(root)
	if err != nil {
		t.Fatal(err)
	}
	submission, review := boundaryFixtures(t, corpus)
	report, err := EvaluateBatch(corpus, submission, review)
	if err != nil {
		t.Fatalf("EvaluateBatch() error = %v", err)
	}
	if report.Verdict != "pass" || len(report.Runs) != 1 || !report.Runs[0].Pass {
		t.Fatalf("report=%+v", report)
	}
}

func TestEvaluateBatchRejectsCoverageAndScoreSchemaFailures(t *testing.T) {
	corpus, err := LoadCorpus(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	submission, review := boundaryFixtures(t, corpus)
	tests := []struct {
		name   string
		mutate func(*BatchSubmission, *BatchReview)
		want   string
	}{
		{"missing case", func(s *BatchSubmission, _ *BatchReview) { s.Runs[0].Results = s.Runs[0].Results[1:] }, "missing_case"},
		{"duplicate case", func(s *BatchSubmission, _ *BatchReview) { s.Runs[0].Results[1] = s.Runs[0].Results[0] }, "duplicate_case"},
		{"unknown case", func(s *BatchSubmission, _ *BatchReview) { s.Runs[0].Results[0].CaseID = "unknown" }, "unknown_case"},
		{"negative score", func(_ *BatchSubmission, r *BatchReview) { r.Reviews[0].FactSSOTFidelity = -1 }, "score_out_of_range"},
		{"score too high", func(_ *BatchSubmission, r *BatchReview) { r.Reviews[0].SecurityRedaction = 11 }, "score_out_of_range"},
		{"invalid failfast", func(_ *BatchSubmission, r *BatchReview) {
			r.Reviews[0].SemanticFailFast = []SemanticFailure{{Code: "bad"}}
		}, "invalid_fail_fast"},
		{"empty failfast evidence", func(_ *BatchSubmission, r *BatchReview) {
			r.Reviews[0].SemanticFailFast = []SemanticFailure{{Code: "unsupported_claim", EvidenceRefs: []string{}}}
		}, "invalid_fail_fast"},
		{"review binding", func(_ *BatchSubmission, r *BatchReview) {
			r.SubmissionDigest = strings.Repeat("0", 64)
		}, "invalid_review_binding"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, r := cloneBatch(t, submission, review)
			tt.mutate(&s, &r)
			_, err := EvaluateBatch(corpus, s, r)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err=%v want=%s", err, tt.want)
			}
		})
	}
}

func TestEvaluateBatchFailsExactGatesWithoutRawStaticError(t *testing.T) {
	corpus, err := LoadCorpus(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	submission, review := boundaryFixtures(t, corpus)
	review.Reviews[0].ReadabilityContract = 1 // 74점
	review.Reviews[1].FactSSOTFidelity = 26
	review.Reviews[2].SemanticFailFast = []SemanticFailure{{Code: "unsupported_claim", EvidenceRefs: []string{"fact:F01"}}}
	// 정적 검증 실패는 원문 없이 안전한 code만 report에 남긴다.
	submission.Runs[0].Results[3].FinalAnswer = ""
	review.SubmissionDigest = SubmissionDigest(submission)
	report, err := EvaluateBatch(corpus, submission, review)
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "fail" {
		t.Fatal("gate failure should fail")
	}
	encoded, _ := json.Marshal(report)
	if strings.Contains(string(encoded), "final_answer가 비어") {
		t.Fatal("raw static error leaked")
	}
	for _, code := range []string{"case_below_75", "run_average_below_88", "fact_average_below_27", "unsupported_claim", "static_validation_failed"} {
		if !strings.Contains(string(encoded), code) {
			t.Fatalf("missing %s", code)
		}
	}
}

func TestBatchReportDoesNotLeakSensitiveStaticFailure(t *testing.T) {
	corpus, err := LoadCorpus(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	submission, review := boundaryFixtures(t, corpus)
	for index := range submission.Runs[0].Results {
		result := &submission.Runs[0].Results[index]
		if result.CaseID == "sensitive-log-redaction" {
			result.FinalAnswer = "sk_live_eval_4f9a2c7d\n" + result.Renderer.Stdout
		}
	}
	review.SubmissionDigest = SubmissionDigest(submission)
	report, err := EvaluateBatch(corpus, submission, review)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"sk_live_eval_4f9a2c7d", "eva.redaction@example.test"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("report에 민감 literal이 포함됨: %s", forbidden)
		}
	}
	if !strings.Contains(string(data), "static_validation_failed") {
		t.Fatal("safe static failure code가 없음")
	}
}

func TestStrictJSONRejectsDuplicateKeysAtEveryDepth(t *testing.T) {
	for _, data := range [][]byte{[]byte(`{"schema":"a","schema":"b"}`), []byte(`{"reviews":[{"fact_ssot_fidelity":0,"fact_ssot_fidelity":1}]}`)} {
		var v any
		if err := decodeStrict(data, &v); err == nil || !strings.Contains(err.Error(), "중복 JSON key") {
			t.Fatalf("err=%v", err)
		}
	}
}

func TestReadBoundedFileRejectsOversizeBeforeDecode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversize.json")
	if err := os.WriteFile(path, []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readBoundedFile(path, 4); err == nil {
		t.Fatal("oversize input을 거부해야 함")
	}
}

func TestStrictJSONRejectsDepthOver64(t *testing.T) {
	data := []byte("0")
	for i := 0; i < 66; i++ {
		data = append([]byte("["), append(data, ']')...)
	}
	var value any
	if err := decodeStrict(data, &value); err == nil || !strings.Contains(err.Error(), "depth") {
		t.Fatalf("err=%v", err)
	}
}

func TestEvaluateBatchCanonicalizesRunOrderAndReportsExactVariance(t *testing.T) {
	corpus, err := LoadCorpus(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	submission, review := boundaryFixtures(t, corpus)
	baseReviews := append([]CaseReview(nil), review.Reviews...)
	for _, id := range []string{"run-2", "run-3"} {
		clone := submission.Runs[0]
		clone.RunID = id
		submission.Runs = append(submission.Runs, clone)
		for _, old := range baseReviews {
			copy := old
			copy.RunID = id
			review.Reviews = append(review.Reviews, copy)
		}
	}
	review.SubmissionDigest = SubmissionDigest(submission)
	for i := range review.Reviews {
		if review.Reviews[i].RunID == "run-2" && review.Reviews[i].CaseID == corpus.promptOrder[0].ID {
			review.Reviews[i].SecurityRedaction = 2
		}
		if review.Reviews[i].RunID == "run-3" && review.Reviews[i].CaseID == corpus.promptOrder[0].ID {
			review.Reviews[i].SecurityRedaction = 0
		}
	}
	report, err := EvaluateBatch(corpus, submission, review)
	if err != nil {
		t.Fatal(err)
	}
	v := report.CaseVariance[0]
	if v.Attempts != 3 || v.TotalVariance != (Rational{6, 9}) || v.DistinctResultCount != 1 || v.StaticPassCount != 3 {
		t.Fatalf("variance=%+v", v)
	}
	submission.Runs[0], submission.Runs[2] = submission.Runs[2], submission.Runs[0]
	review.SubmissionDigest = SubmissionDigest(submission)
	review.Reviews[0], review.Reviews[len(review.Reviews)-1] = review.Reviews[len(review.Reviews)-1], review.Reviews[0]
	shuffled, err := EvaluateBatch(corpus, submission, review)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(report)
	b, _ := json.Marshal(shuffled)
	if string(a) != string(b) {
		t.Fatal("report must be byte-identical after input order shuffle")
	}
}

func TestEvaluateBatchCanonicalizesFailedResultOrder(t *testing.T) {
	corpus, err := LoadCorpus(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	submission, review := boundaryFixtures(t, corpus)
	submission.Runs[0].Results[0].FinalAnswer = ""
	submission.Runs[0].Results[1].FinalAnswer = ""
	review.SubmissionDigest = SubmissionDigest(submission)
	first, err := EvaluateBatch(corpus, submission, review)
	if err != nil {
		t.Fatal(err)
	}
	submission.Runs[0].Results[0], submission.Runs[0].Results[1] = submission.Runs[0].Results[1], submission.Runs[0].Results[0]
	review.SubmissionDigest = SubmissionDigest(submission)
	second, err := EvaluateBatch(corpus, submission, review)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatal("실패 report도 result 입력 순서와 무관해야 함")
	}
}

func TestInspectBatchBindingIsCanonicalAndContainsNoArtifacts(t *testing.T) {
	corpus, err := LoadCorpus(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	submission, _ := boundaryFixtures(t, corpus)
	path := filepath.Join(t.TempDir(), "submission.json")
	writeBatchJSON(t, path, submission)
	first, err := InspectBatchFile(repositoryRoot(t), path)
	if err != nil {
		t.Fatal(err)
	}
	submission.Runs[0].Results[0], submission.Runs[0].Results[len(submission.Runs[0].Results)-1] = submission.Runs[0].Results[len(submission.Runs[0].Results)-1], submission.Runs[0].Results[0]
	writeBatchJSON(t, path, submission)
	second, err := InspectBatchFile(repositoryRoot(t), path)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatal("binding은 result 입력 순서와 무관해야 함")
	}
	encoded := string(a)
	for _, forbidden := range []string{"sk_live_eval_4f9a2c7d", "eva.redaction@example.test", "final_answer", "diagram_source"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("binding에 artifact 원문이 포함됨: %s", forbidden)
		}
	}
}

func TestResultResourceLimitsRejectAmplification(t *testing.T) {
	tests := []EvaluationResult{
		{DiagramSource: strings.Repeat("x", 256*1024+1)},
		{Renderer: RendererResult{Stdout: strings.Repeat("x", 200*1024+1)}},
		{Renderer: RendererResult{Stderr: strings.Repeat("x", 16*1024+1)}},
		{FinalAnswer: strings.Repeat("x", 256*1024+1)},
		{Claims: make([]Claim, 257)},
		{Claims: []Claim{{Text: strings.Repeat("x", 64*1024+1)}}},
		{Claims: []Claim{{FactIDs: make([]string, 65)}}},
	}
	for index, result := range tests {
		if err := validateResultResources(result); err == nil {
			t.Fatalf("resource case %d가 통과함", index)
		}
	}
}

func writeBatchJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func cloneBatch(t *testing.T, submission BatchSubmission, review BatchReview) (BatchSubmission, BatchReview) {
	t.Helper()
	data, err := json.Marshal(struct {
		S BatchSubmission
		R BatchReview
	}{submission, review})
	if err != nil {
		t.Fatal(err)
	}
	var copy struct {
		S BatchSubmission
		R BatchReview
	}
	if err := json.Unmarshal(data, &copy); err != nil {
		t.Fatal(err)
	}
	return copy.S, copy.R
}

func boundaryFixtures(t *testing.T, corpus *Corpus) (BatchSubmission, BatchReview) {
	t.Helper()
	run := BatchRun{RunID: "run-1"}
	for _, prompt := range corpus.promptOrder {
		oracle := corpus.oracles[prompt.ID]
		result := EvaluationResult{CaseID: prompt.ID, FinalAnswer: "근거 기반 설명"}
		if oracle.ReferenceSource != "" {
			_, renderer, _, err := Replay(oracle.ReferenceSource)
			if err != nil {
				t.Fatal(err)
			}
			result.DiagramSource, result.Renderer = oracle.ReferenceSource, renderer
			result.FinalAnswer += "\n" + renderer.Stdout
		}
		for _, fact := range prompt.Facts {
			result.Claims = append(result.Claims, Claim{Text: "근거 " + fact.ID, FactIDs: []string{fact.ID}})
		}
		run.Results = append(run.Results, result)
	}
	submission := BatchSubmission{Schema: "eval-pack.batch.v1", SubjectID: "subject-1", CorpusDigest: CorpusDigest(corpus), Runs: []BatchRun{run}}
	review := BatchReview{Schema: "eval-pack.review.v1", EvaluatorID: "evaluator-1", SubjectID: submission.SubjectID, CorpusDigest: submission.CorpusDigest}
	for _, result := range run.Results {
		review.Reviews = append(review.Reviews, CaseReview{RunID: run.RunID, CaseID: result.CaseID, FactSSOTFidelity: 27, RuntimeOwnershipFailure: 20, DiagramNotation: 20, ReadabilityContract: 15, SecurityRedaction: 1, SemanticFailFast: []SemanticFailure{}})
	}
	review.SubmissionDigest = SubmissionDigest(submission)
	return submission, review
}
