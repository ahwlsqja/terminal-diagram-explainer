package app

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/er"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/render"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/sequence"
)

const (
	Version       = "0.10.0"
	MaxInputBytes = 256 * 1024
)

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("term-diagram", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	ascii := flags.Bool("ascii", false, "ASCII 테두리와 연결선 사용; UTF-8 label은 보존")
	filePath := flags.String("f", "-", "입력 파일 경로 또는 stdin용 -")
	version := flags.Bool("version", false, "버전 출력")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(stderr, "사용법: term-diagram [-ascii] [-f path|-] [-version]")
		} else {
			fmt.Fprintln(stderr, "잘못된 CLI 옵션")
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "위치 인자는 허용하지 않음")
		return 2
	}
	if *version {
		if _, err := fmt.Fprintln(stdout, Version); err != nil {
			fmt.Fprintln(stderr, "버전 출력 실패")
			return 1
		}
		return 0
	}

	reader := stdin
	var file *os.File
	if *filePath != "-" {
		info, err := os.Lstat(*filePath)
		if err != nil {
			fmt.Fprintf(stderr, "입력 파일 열기 실패: %q\n", *filePath)
			return 1
		}
		if !info.Mode().IsRegular() {
			fmt.Fprintf(stderr, "regular file만 입력으로 허용함: %q\n", *filePath)
			return 1
		}
		opened, err := os.Open(*filePath)
		if err != nil {
			fmt.Fprintf(stderr, "입력 파일 열기 실패: %q\n", *filePath)
			return 1
		}
		file = opened
		defer file.Close()
		info, err = file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			fmt.Fprintf(stderr, "regular file만 입력으로 허용함: %q\n", *filePath)
			return 1
		}
		reader = file
	}

	source, err := readBounded(reader)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	options := render.DefaultOptions()
	options.ASCII = *ascii
	var output string
	if isSequenceSource(string(source)) {
		diagram, parseErr := sequence.Parse(string(source), sequence.DefaultLimits())
		if parseErr != nil {
			fmt.Fprintln(stderr, parseErr)
			return 2
		}
		output, err = render.Sequence(diagram, options)
	} else if isERSource(string(source)) {
		diagram, parseErr := er.Parse(string(source), er.DefaultLimits())
		if parseErr != nil {
			fmt.Fprintln(stderr, parseErr)
			return 2
		}
		output, err = render.ER(diagram, options)
	} else {
		graph, parseErr := flow.Parse(string(source), flow.DefaultLimits())
		if parseErr != nil {
			fmt.Fprintln(stderr, parseErr)
			return 2
		}
		output, err = render.Flow(graph, options)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	var committed bytes.Buffer
	committed.WriteString(output)
	committed.WriteByte('\n')
	if _, err := io.Copy(stdout, &committed); err != nil {
		fmt.Fprintf(stderr, "출력 실패: %v\n", err)
		return 1
	}
	return 0
}

func isERSource(source string) bool {
	for _, raw := range strings.Split(source, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "%%") {
			continue
		}
		return line == "erDiagram"
	}
	return false
}

func isSequenceSource(source string) bool {
	for _, raw := range strings.Split(source, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "%%") {
			continue
		}
		return line == "sequenceDiagram"
	}
	return false
}

func readBounded(reader io.Reader) ([]byte, error) {
	limited := io.LimitReader(reader, MaxInputBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("입력 읽기 실패")
	}
	if len(data) > MaxInputBytes {
		return nil, fmt.Errorf("입력 크기 제한 초과: %d bytes", MaxInputBytes)
	}
	return data, nil
}
