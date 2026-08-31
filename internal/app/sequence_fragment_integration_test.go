package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sequenceFragmentSource = `sequenceDiagram
participant Client as 클라이언트
participant API as API 서버
loop 최대 3회
Client ->> API: 요청
alt 성공
API -->> Client: 완료
else 실패
API -->> Client: 재시도
end
end
`

func TestRunSequenceFragmentStdinFileAndASCII(t *testing.T) {
	fromStdin := invoke(nil, strings.NewReader(sequenceFragmentSource))
	path := filepath.Join(t.TempDir(), "fragment.mmd")
	if err := os.WriteFile(path, []byte(sequenceFragmentSource), 0o600); err != nil {
		t.Fatal(err)
	}
	fromFile := invoke([]string{"-f", path}, strings.NewReader("ignored"))
	if fromStdin != fromFile || fromStdin.code != 0 || fromStdin.stderr != "" {
		t.Fatalf("stdin=%+v file=%+v", fromStdin, fromFile)
	}
	for _, text := range []string{"loop: 최대 3회", "alt: 성공", "else: 실패", "요청", "완료", "재시도"} {
		if !strings.Contains(fromStdin.stdout, text) {
			t.Fatalf("missing %q:\n%s", text, fromStdin.stdout)
		}
	}
	ascii := invoke([]string{"-ascii"}, strings.NewReader(sequenceFragmentSource))
	if ascii.code != 0 || ascii.stderr != "" || strings.ContainsAny(ascii.stdout, "┌┐└┘├┤─│┄┊┼▶◀") {
		t.Fatalf("ASCII result=%+v", ascii)
	}
}

func TestRunSequenceFragmentFailureDoesNotCommitOutput(t *testing.T) {
	for _, source := range []string{
		"sequenceDiagram\nparticipant A\nalt x\nA ->> A: x\nend",
		"sequenceDiagram\nparticipant A\nloop x\nend",
		"sequenceDiagram\nparticipant A\npar x\nA ->> A: x\nend",
	} {
		got := invoke(nil, strings.NewReader(source))
		if got.code != 2 || got.stdout != "" || got.stderr == "" {
			t.Fatalf("source=%q result=%+v", source, got)
		}
	}
}
