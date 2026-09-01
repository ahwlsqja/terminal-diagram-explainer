package app

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRunDispatchesStateDiagramWithoutPartialOutput(t *testing.T) {
	input := "stateDiagram-v2\n[*] --> A\nA --> [*]\nstate A\n"
	var out, errOut bytes.Buffer
	if code := Run(nil, strings.NewReader(input), &out, &errOut); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "● --> A") {
		t.Fatalf("state output=%q", out.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Run(nil, strings.NewReader("stateDiagram-v2\nstate A\nA --> A\n"), &out, &errOut); code != 2 || out.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
}

func TestStateFileInputMatchesStdinAndASCII(t *testing.T) {
	input := "stateDiagram-v2\ndirection LR\n[*] --> A\nA --> B\nB --> [*]\nstate A\nstate B\n"
	path := t.TempDir() + "/state.mmd"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdinOut, fileOut, errOut bytes.Buffer
	if code := Run(nil, strings.NewReader(input), &stdinOut, &errOut); code != 0 {
		t.Fatalf("stdin %d: %s", code, errOut.String())
	}
	if code := Run([]string{"-f", path}, strings.NewReader("ignored"), &fileOut, &errOut); code != 0 {
		t.Fatalf("file %d: %s", code, errOut.String())
	}
	if stdinOut.String() != fileOut.String() {
		t.Fatalf("file output differs\nstdin=%q\nfile=%q", stdinOut.String(), fileOut.String())
	}
	var ascii bytes.Buffer
	if code := Run([]string{"-ascii"}, strings.NewReader(input), &ascii, &errOut); code != 0 || !strings.Contains(ascii.String(), "+") || !strings.Contains(ascii.String(), "^") {
		t.Fatalf("ascii=%q code=%d", ascii.String(), code)
	}
}

func TestRunRendersExplicitTransitionPolicy(t *testing.T) {
	input := "stateDiagram-v2\n[*] --> A\nA --> B : retry\nstate A\nstate B\npolicy A --> B : retry :: retry \"attempt below 3\"\n"
	var out, errOut bytes.Buffer
	if code := Run(nil, strings.NewReader(input), &out, &errOut); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "A --> B : retry\n  policy retry: attempt below 3") {
		t.Fatalf("state policy output=%q", out.String())
	}
}

func TestRunRendersExplicitChoiceWithoutPartialOutput(t *testing.T) {
	input := "stateDiagram-v2\n[*] --> Pending\nPending --> Decide : evaluated\nDecide --> A : [a]\nDecide --> B : [b]\nstate Pending\nstate Decide <<choice>>\nstate A\nstate B\n"
	var out, errOut bytes.Buffer
	if code := Run(nil, strings.NewReader(input), &out, &errOut); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "◁ Decide") || !strings.Contains(out.String(), "▷") || !strings.Contains(out.String(), "Decide --> A : [a]") {
		t.Fatalf("choice output=%q", out.String())
	}
	out.Reset()
	errOut.Reset()
	invalid := strings.Replace(input, "Decide --> B : [b]", "Decide --> B : b", 1)
	if code := Run(nil, strings.NewReader(invalid), &out, &errOut); code != 2 || out.Len() != 0 {
		t.Fatalf("invalid choice code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
}
