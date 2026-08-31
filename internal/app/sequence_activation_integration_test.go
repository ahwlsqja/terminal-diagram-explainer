package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sequenceActivationSource = `sequenceDiagram
participant Client as 브라우저
participant API as API 서버
activate API
Client ->> API: 요청
API -->> Client: 완료
deactivate API
`

func TestRunSequenceActivationStdinFileAndASCII(t *testing.T) {
	fromStdin := invoke(nil, strings.NewReader(sequenceActivationSource))
	path := filepath.Join(t.TempDir(), "activation.mmd")
	if err := os.WriteFile(path, []byte(sequenceActivationSource), 0o600); err != nil {
		t.Fatal(err)
	}
	fromFile := invoke([]string{"-f", path}, strings.NewReader("ignored"))
	if fromStdin != fromFile || fromStdin.code != 0 || fromStdin.stderr != "" {
		t.Fatalf("stdin=%+v file=%+v", fromStdin, fromFile)
	}
	if !strings.Contains(fromStdin.stdout, "│") || !strings.Contains(fromStdin.stdout, "┊") {
		t.Fatalf("activation/lifeline distinction missing:\n%s", fromStdin.stdout)
	}
	ascii := invoke([]string{"-ascii"}, strings.NewReader(sequenceActivationSource))
	if ascii.code != 0 || ascii.stderr != "" || strings.ContainsAny(ascii.stdout, "┌┐└┘├┤─│┄┊┼▶◀") {
		t.Fatalf("ASCII result=%+v", ascii)
	}
}

func TestRunSequenceActivationFailureDoesNotCommitOutput(t *testing.T) {
	for _, source := range []string{
		"sequenceDiagram\nparticipant A\ndeactivate A\nA ->> A: x",
		"sequenceDiagram\nparticipant A\nactivate A\ndeactivate A\nA ->> A: x",
		"sequenceDiagram\nparticipant A\nactivate A\nloop x\nA ->> A: x\nend\ndeactivate A",
	} {
		got := invoke(nil, strings.NewReader(source))
		if got.code != 2 || got.stdout != "" || got.stderr == "" {
			t.Fatalf("source=%q result=%+v", source, got)
		}
	}
}
