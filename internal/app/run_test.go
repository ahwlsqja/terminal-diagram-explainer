package app

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRunRendersStdin(t *testing.T) {
	in := strings.NewReader("flowchart LR\nA[요청] --> B[응답]\n")
	var out, errOut bytes.Buffer
	code := Run(nil, in, &out, &errOut)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "요청") || !strings.Contains(out.String(), "응답") {
		t.Fatalf("stdout=%s", out.String())
	}
}

func TestRunDoesNotEmitPartialOutputOnInvalidInput(t *testing.T) {
	in := strings.NewReader("flowchart LR\nclassDef foo invalid\nA --> B\n")
	var out, errOut bytes.Buffer
	code := Run(nil, in, &out, &errOut)
	if code == 0 {
		t.Fatal("expected failure")
	}
	if out.Len() != 0 {
		t.Fatalf("partial stdout=%q", out.String())
	}
	if !strings.Contains(errOut.String(), "2행") {
		t.Fatalf("stderr=%q", errOut.String())
	}
}

func TestRunRejectsOversizedInput(t *testing.T) {
	in := strings.NewReader("flowchart LR\nA[" + strings.Repeat("x", MaxInputBytes) + "]")
	var out, errOut bytes.Buffer
	code := Run(nil, in, &out, &errOut)
	if code == 0 || out.Len() != 0 || !strings.Contains(errOut.String(), "입력 크기") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
}

func TestRunVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"-version"}, strings.NewReader(""), &out, &errOut)
	if code != 0 || strings.TrimSpace(out.String()) != Version {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
}

func TestRunRejectsDirectoryInput(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"-f", t.TempDir()}, strings.NewReader(""), &out, &errOut)
	if code == 0 || out.Len() != 0 || !strings.Contains(errOut.String(), "regular file") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
}

func TestRunReadsRegularFile(t *testing.T) {
	path := t.TempDir() + "/diagram.mmd"
	if err := os.WriteFile(path, []byte("flowchart TD\nA[시작] --> B[끝]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := Run([]string{"-f", path}, strings.NewReader(""), &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "시작") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
}
