package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/eval"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("eval-pack", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "evals/가 있는 repository root")
	result := flags.String("f", "", "검증할 evaluation result JSON 파일")
	batch := flags.String("batch", "", "batch submission JSON 파일")
	review := flags.String("review", "", "batch review JSON 파일")
	inspect := flags.String("inspect-batch", "", "batch submission binding JSON 파일")
	digest := flags.Bool("corpus-digest", false, "current corpus digest 출력")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || (*digest && (*result != "" || *batch != "" || *review != "" || *inspect != "")) || (*result == "" && *batch == "" && *review == "" && *inspect == "" && !*digest) || (*inspect != "" && (*result != "" || *batch != "" || *review != "")) || (*result != "" && (*batch != "" || *review != "")) || ((*batch == "") != (*review == "")) {
		fmt.Fprintln(stderr, "사용법: eval-pack [-root repository-root] -corpus-digest | -inspect-batch submission.json | -f result.json | -batch submission.json -review review.json")
		return 2
	}
	if *digest {
		corpus, err := eval.LoadCorpus(*root)
		if err != nil {
			fmt.Fprintln(stderr, "corpus 입력 실패:", err)
			return 2
		}
		fmt.Fprintln(stdout, eval.CorpusDigest(corpus))
		return 0
	}
	if *inspect != "" {
		binding, err := eval.InspectBatchFile(*root, *inspect)
		if err != nil {
			fmt.Fprintln(stderr, "batch 입력 실패:", err)
			return 2
		}
		data, _ := json.Marshal(binding)
		fmt.Fprintln(stdout, string(data))
		return 0
	}
	if *batch == "" {
		if err := eval.ValidateFile(*root, *result); err != nil {
			fmt.Fprintln(stderr, "평가 결과 검증 실패:", err)
			return 1
		}
		fmt.Fprintln(stdout, "평가 결과 검증 통과")
		return 0
	}
	report, err := eval.EvaluateBatchFiles(*root, *batch, *review)
	if err != nil {
		var input *eval.InputError
		if errors.As(err, &input) {
			fmt.Fprintln(stderr, "batch 입력 실패:", input)
			return 2
		}
		failure := eval.SafeFailure{Code: "invalid_submission"}
		var validation *eval.BatchValidationError
		if errors.As(err, &validation) {
			failure = eval.SafeFailure{Code: validation.Code}
		}
		data, _ := json.Marshal(eval.BatchReport{Schema: eval.ReportSchema, Verdict: "invalid", Failures: []eval.SafeFailure{failure}})
		fmt.Fprintln(stdout, string(data))
		return 1
	}
	data, err := json.Marshal(report)
	if err != nil {
		fmt.Fprintln(stderr, "report 직렬화 실패")
		return 1
	}
	fmt.Fprintln(stdout, string(data))
	if report.Verdict != "pass" {
		return 1
	}
	return 0
}
