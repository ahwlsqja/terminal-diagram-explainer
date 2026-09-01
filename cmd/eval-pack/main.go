package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/eval"
)

func main() {
	flags := flag.NewFlagSet("eval-pack", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	root := flags.String("root", ".", "evals/가 있는 repository root")
	result := flags.String("f", "", "검증할 evaluation result JSON 파일")
	if err := flags.Parse(os.Args[1:]); err != nil || flags.NArg() != 0 || *result == "" {
		fmt.Fprintln(os.Stderr, "사용법: eval-pack [-root repository-root] -f result.json")
		os.Exit(2)
	}
	if err := eval.ValidateFile(*root, *result); err != nil {
		fmt.Fprintln(os.Stderr, "평가 결과 검증 실패:", err)
		os.Exit(1)
	}
	fmt.Println("평가 결과 검증 통과")
}
