package app

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const integrationSource = "flowchart LR\nA[request] --> B[response]\n"

type invocation struct {
	code   int
	stdout string
	stderr string
}

func invoke(args []string, stdin io.Reader) invocation {
	var stdout, stderr bytes.Buffer
	code := Run(args, stdin, &stdout, &stderr)
	return invocation{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func TestRunStdinAndFileAreEquivalent(t *testing.T) {
	fromStdin := invoke(nil, strings.NewReader(integrationSource))

	path := filepath.Join(t.TempDir(), "diagram.mmd")
	if err := os.WriteFile(path, []byte(integrationSource), 0o600); err != nil {
		t.Fatal(err)
	}
	fromFile := invoke([]string{"-f", path}, strings.NewReader("ignored invalid stdin"))

	if fromStdin != fromFile {
		t.Fatalf("stdin=%+v file=%+v", fromStdin, fromFile)
	}
	if fromStdin.code != 0 || fromStdin.stdout == "" || fromStdin.stderr != "" {
		t.Fatalf("unexpected success result: %+v", fromStdin)
	}
}

func TestRunCycleStdinAndFileAreEquivalent(t *testing.T) {
	cycleSource := "flowchart TD\nA[Request] --> B[Worker]\nB -.->|retry| A\n"
	fromStdin := invoke(nil, strings.NewReader(cycleSource))

	path := filepath.Join(t.TempDir(), "cycle.mmd")
	if err := os.WriteFile(path, []byte(cycleSource), 0o600); err != nil {
		t.Fatal(err)
	}
	fromFile := invoke([]string{"-f", path}, strings.NewReader("ignored"))
	if fromStdin != fromFile {
		t.Fatalf("stdin=%+v file=%+v", fromStdin, fromFile)
	}
	if fromStdin.code != 0 || fromStdin.stderr != "" || !strings.Contains(fromStdin.stdout, "F01 B -.-> A |retry|") {
		t.Fatalf("cycle result=%+v", fromStdin)
	}
}

func TestRunInputSizeBoundaryAndEmptyInput(t *testing.T) {
	prefix := "flowchart LR\nA[start]\n%%"
	exactlyAtLimit := prefix + strings.Repeat("x", MaxInputBytes-len(prefix))
	if got := len(exactlyAtLimit); got != MaxInputBytes {
		t.Fatalf("fixture size=%d", got)
	}

	tests := []struct {
		name       string
		input      string
		wantCode   int
		wantStderr string
	}{
		{name: "empty", input: "", wantCode: 2, wantStderr: "flowchart 헤더가 없음"},
		{name: "exactly at limit", input: exactlyAtLimit, wantCode: 0},
		{name: "one byte over limit", input: exactlyAtLimit + "x", wantCode: 2, wantStderr: "입력 크기 제한 초과"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := invoke(nil, strings.NewReader(test.input))
			if got.code != test.wantCode {
				t.Fatalf("code=%d want=%d stdout=%q stderr=%q", got.code, test.wantCode, got.stdout, got.stderr)
			}
			if test.wantCode == 0 {
				if got.stdout == "" || got.stderr != "" {
					t.Fatalf("stdout=%q stderr=%q", got.stdout, got.stderr)
				}
				return
			}
			if got.stdout != "" || !strings.Contains(got.stderr, test.wantStderr) {
				t.Fatalf("stdout=%q stderr=%q", got.stdout, got.stderr)
			}
		})
	}
}

func TestRunRejectsSpecialFileInputs(t *testing.T) {
	paths := []string{t.TempDir()}
	if info, err := os.Stat("/dev/null"); err == nil && !info.Mode().IsRegular() {
		paths = append(paths, "/dev/null")
	}

	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			got := invoke([]string{"-f", path}, strings.NewReader(integrationSource))
			if got.code != 1 || got.stdout != "" || !strings.Contains(got.stderr, "regular file만 입력으로 허용함") {
				t.Fatalf("code=%d stdout=%q stderr=%q", got.code, got.stdout, got.stderr)
			}
		})
	}
}

func TestRunQuotesHostileFilePathOnStderr(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing\nforged\t\x1b[31m\"file")
	got := invoke([]string{"-f", path}, strings.NewReader(integrationSource))

	if got.code != 1 || got.stdout != "" {
		t.Fatalf("code=%d stdout=%q stderr=%q", got.code, got.stdout, got.stderr)
	}
	if !strings.Contains(got.stderr, strconv.Quote(path)) {
		t.Fatalf("path was not safely quoted: stderr=%q want quoted path=%s", got.stderr, strconv.Quote(path))
	}
	if strings.Count(got.stderr, "\n") != 1 || strings.ContainsRune(got.stderr, '\x1b') || strings.ContainsRune(got.stderr, '\t') {
		t.Fatalf("stderr contains an injectable control character: %q", got.stderr)
	}
}

func TestRunDoesNotCommitOutputWhenRenderingFails(t *testing.T) {
	wideLabel := strings.Repeat("x", 96)
	source := "flowchart LR\nA[" + wideLabel + "] --> B[" + wideLabel + "] --> C[" + wideLabel + "]\n"
	got := invoke(nil, strings.NewReader(source))
	if got.code != 2 || got.stdout != "" || !strings.Contains(got.stderr, "출력 폭 제한") {
		t.Fatalf("code=%d stdout=%q stderr=%q", got.code, got.stdout, got.stderr)
	}
}

func TestRunASCIIFlagUsesOnlyASCIIForASCIIInput(t *testing.T) {
	unicodeResult := invoke(nil, strings.NewReader(integrationSource))
	asciiResult := invoke([]string{"-ascii"}, strings.NewReader(integrationSource))
	if unicodeResult.code != 0 || asciiResult.code != 0 {
		t.Fatalf("unicode=%+v ascii=%+v", unicodeResult, asciiResult)
	}
	if unicodeResult.stdout == asciiResult.stdout || !strings.ContainsAny(unicodeResult.stdout, "┌┐└┘─│▶") {
		t.Fatalf("default output is not Unicode-rendered: %q", unicodeResult.stdout)
	}
	for _, character := range asciiResult.stdout {
		if character > 127 {
			t.Fatalf("-ascii emitted non-ASCII rune %q in %q", character, asciiResult.stdout)
		}
	}
}

func TestRunASCIIFlagPreservesNonASCIILabels(t *testing.T) {
	result := invoke([]string{"-ascii"}, strings.NewReader("flowchart LR\nA[요청] --> B[응답]\n"))
	if result.code != 0 || result.stderr != "" {
		t.Fatalf("result=%+v", result)
	}
	if !strings.Contains(result.stdout, "요청") || !strings.Contains(result.stdout, "응답") {
		t.Fatalf("localized labels were not preserved: %q", result.stdout)
	}
	if strings.ContainsAny(result.stdout, "┌┐└┘─│▶") {
		t.Fatalf("-ascii emitted Unicode drawing characters: %q", result.stdout)
	}
}

func TestRunRejectsPositionalAndUnknownFlags(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{name: "positional", args: []string{"diagram.mmd"}, wantStderr: "위치 인자는 허용하지 않음"},
		{name: "unknown flag", args: []string{"-definitely-unknown"}, wantStderr: "잘못된 CLI 옵션"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := invoke(test.args, strings.NewReader(integrationSource))
			if got.code != 2 || got.stdout != "" || !strings.Contains(got.stderr, test.wantStderr) {
				t.Fatalf("code=%d stdout=%q stderr=%q", got.code, got.stdout, got.stderr)
			}
		})
	}
}

func TestRunDoesNotEchoHostileUnknownFlag(t *testing.T) {
	got := invoke([]string{"-unknown\nforged\t\x1b[31m"}, strings.NewReader(integrationSource))
	if got.code != 2 || got.stdout != "" || got.stderr != "잘못된 CLI 옵션\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", got.code, got.stdout, got.stderr)
	}
}

func TestRunHelpDoesNotClaimLabelsAreASCIIOnly(t *testing.T) {
	got := invoke([]string{"-h"}, strings.NewReader(integrationSource))
	if got.code != 2 || got.stdout != "" || got.stderr != "사용법: term-diagram [-ascii] [-fit] [-format text|svg] [-width cells] [-height cells] [-f path|-] [-version]\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", got.code, got.stdout, got.stderr)
	}
	if strings.Contains(got.stderr, "ASCII 문자만 사용") {
		t.Fatalf("help retained the inaccurate ASCII-only claim: %q", got.stderr)
	}
}

type errorReader struct {
	err error
}

func (reader errorReader) Read([]byte) (int, error) {
	return 0, reader.err
}

type prefixErrorWriter struct {
	limit int
	data  bytes.Buffer
	err   error
}

func (writer *prefixErrorWriter) Write(input []byte) (int, error) {
	remaining := writer.limit - writer.data.Len()
	if remaining <= 0 {
		return 0, writer.err
	}
	if remaining > len(input) {
		remaining = len(input)
	}
	_, _ = writer.data.Write(input[:remaining])
	return remaining, writer.err
}

func TestRunClassifiesReadAndWriteFailures(t *testing.T) {
	readFailure := errors.New("synthetic read failure")
	readResult := invoke(nil, errorReader{err: readFailure})
	if readResult.code != 2 || readResult.stdout != "" || !strings.Contains(readResult.stderr, "입력 읽기 실패") {
		t.Fatalf("read result=%+v", readResult)
	}

	writeFailure := errors.New("synthetic write failure")
	writer := &prefixErrorWriter{limit: 7, err: writeFailure}
	var stderr bytes.Buffer
	code := Run(nil, strings.NewReader(integrationSource), writer, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), writeFailure.Error()) {
		t.Fatalf("code=%d partial=%q stderr=%q", code, writer.data.String(), stderr.String())
	}
	if writer.data.Len() != writer.limit {
		t.Fatalf("expected the adversarial writer to accept a prefix: len=%d data=%q", writer.data.Len(), writer.data.String())
	}
}

func TestRunVersionReportsWriteFailure(t *testing.T) {
	writeFailure := errors.New("synthetic version write failure")
	writer := &prefixErrorWriter{limit: 0, err: writeFailure}
	var stderr bytes.Buffer
	code := Run([]string{"-version"}, strings.NewReader(""), writer, &stderr)
	if code != 1 || writer.data.Len() != 0 || stderr.String() != "버전 출력 실패\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, writer.data.String(), stderr.String())
	}
}

func TestRunOutputIsDeterministicAndCWDIndependent(t *testing.T) {
	baseline := invoke(nil, strings.NewReader(integrationSource))
	if baseline.code != 0 {
		t.Fatalf("baseline=%+v", baseline)
	}

	t.Setenv("NO_COLOR", "1")
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("COLUMNS", "7")
	t.Setenv("LINES", "3")
	t.Setenv("TERM_DIAGRAM_ASCII", "1")
	t.Chdir(t.TempDir())

	for iteration := 0; iteration < 32; iteration++ {
		got := invoke(nil, strings.NewReader(integrationSource))
		if got != baseline {
			t.Fatalf("iteration=%d baseline=%+v got=%+v", iteration, baseline, got)
		}
	}
}

func TestCLIExitCodesAndProcessEnvironmentIndependence(t *testing.T) {
	binary := buildCLI(t)
	expected := invoke(nil, strings.NewReader(integrationSource))
	workingDirectory := t.TempDir()
	inputPath := filepath.Join(workingDirectory, "diagram.mmd")
	if err := os.WriteFile(inputPath, []byte(integrationSource), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "success", stdin: integrationSource, wantCode: 0, wantStdout: expected.stdout},
		{name: "file matches stdin", args: []string{"-f", inputPath}, stdin: "ignored", wantCode: 0, wantStdout: expected.stdout},
		{name: "version", args: []string{"-version"}, wantCode: 0, wantStdout: Version + "\n"},
		{name: "invalid input", stdin: "", wantCode: 2, wantStderr: "flowchart 헤더가 없음"},
		{name: "unknown flag", args: []string{"-unknown"}, wantCode: 2, wantStderr: "잘못된 CLI 옵션"},
		{name: "missing file", args: []string{"-f", filepath.Join(workingDirectory, "missing")}, wantCode: 1, wantStderr: "입력 파일 열기 실패"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := exec.Command(binary, test.args...)
			command.Dir = workingDirectory
			command.Env = []string{
				"NO_COLOR=1",
				"CLICOLOR_FORCE=1",
				"COLUMNS=1",
				"LINES=1",
				"TERM_DIAGRAM_ASCII=1",
			}
			command.Stdin = strings.NewReader(test.stdin)
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr
			err := command.Run()
			if code := processExitCode(err); code != test.wantCode {
				t.Fatalf("code=%d want=%d err=%v stdout=%q stderr=%q", code, test.wantCode, err, stdout.String(), stderr.String())
			}
			if test.wantStdout != "" && stdout.String() != test.wantStdout {
				t.Fatalf("stdout=%q want=%q", stdout.String(), test.wantStdout)
			}
			if test.wantCode != 0 && stdout.Len() != 0 {
				t.Fatalf("failure emitted stdout=%q", stdout.String())
			}
			if test.wantStderr == "" {
				if stderr.Len() != 0 {
					t.Fatalf("stderr=%q", stderr.String())
				}
			} else if !strings.Contains(stderr.String(), test.wantStderr) {
				t.Fatalf("stderr=%q want substring=%q", stderr.String(), test.wantStderr)
			}
		})
	}
}

func buildCLI(t *testing.T) string {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate integration test source")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	buildDirectory := t.TempDir()
	binary := filepath.Join(buildDirectory, "term-diagram")
	command := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", binary, "./cmd/term-diagram")
	command.Dir = repositoryRoot
	command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOPROXY=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	return binary
}

func processExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode()
	}
	return -1
}

func TestExitCodeContractIsComplete(t *testing.T) {
	tests := []struct {
		name string
		got  invocation
		want int
	}{
		{name: "success", got: invoke(nil, strings.NewReader(integrationSource)), want: 0},
		{name: "input file I/O", got: invoke([]string{"-f", filepath.Join(t.TempDir(), "missing")}, strings.NewReader("")), want: 1},
		{name: "invalid syntax", got: invoke(nil, strings.NewReader("not a flowchart")), want: 2},
		{name: "invalid CLI use", got: invoke([]string{"positional"}, strings.NewReader(integrationSource)), want: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got.code != test.want {
				t.Fatalf("code=%d want=%d stdout=%q stderr=%q", test.got.code, test.want, test.got.stdout, test.got.stderr)
			}
		})
	}
}
