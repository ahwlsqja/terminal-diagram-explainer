package render

import (
	"errors"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/sequence"
)

const fragmentFixture = `sequenceDiagram
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
opt 감사
Client ->> Client: 기록
end`

func TestSequenceFragmentFramesAndBranchAreVisible(t *testing.T) {
	diagram, err := sequence.Parse(fragmentFixture, sequence.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	output := mustRenderSequence(t, diagram, Options{MaxWidth: 160, MaxHeight: 80})
	assertSequenceOutputClean(t, output, 160, 80)
	for _, text := range []string{"loop: 최대 3회", "alt: 성공", "else: 실패", "opt: 감사", "요청", "완료", "재시도", "기록"} {
		assertSequenceTextOnce(t, output, text)
	}
	if strings.Count(output, "┌") < 5 || !strings.Contains(output, "├") || !strings.Contains(output, "┤") {
		t.Fatalf("fragment borders or separator missing:\n%s", output)
	}
}

func TestSequenceNestedFragmentUsesDistinctInsets(t *testing.T) {
	source := `sequenceDiagram
participant A
participant B
loop outer
alt middle
opt inner
A ->> B: call
end
else fallback
B -->> A: done
end
end`
	diagram, err := sequence.Parse(source, sequence.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	output := mustRenderSequence(t, diagram, Options{MaxWidth: 100, MaxHeight: 60})
	lines := strings.Split(output, "\n")
	lefts := make(map[int]struct{})
	for _, line := range lines {
		if strings.Contains(line, "loop: ") || strings.Contains(line, "alt: ") || strings.Contains(line, "opt: ") {
			lefts[strings.Index(line, "┌")] = struct{}{}
		}
	}
	if len(lefts) != 3 {
		t.Fatalf("nested fragment insets=%v:\n%s", lefts, output)
	}
}

func TestSequenceFragmentASCIIAndBounds(t *testing.T) {
	diagram, err := sequence.Parse(fragmentFixture, sequence.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	output := mustRenderSequence(t, diagram, Options{ASCII: true, MaxWidth: 160, MaxHeight: 80})
	if strings.ContainsAny(output, "┌┐└┘├┤─│┄┊┼▶◀") {
		t.Fatalf("ASCII fragment output contains Unicode drawing glyphs:\n%s", output)
	}
	for _, text := range []string{"loop: 최대 3회", "alt: 성공", "else: 실패", "opt: 감사"} {
		assertSequenceTextOnce(t, output, text)
	}
	width := sequenceOutputWidth(t, output)
	height := len(strings.Split(output, "\n"))
	if _, err := Sequence(diagram, Options{ASCII: true, MaxWidth: width, MaxHeight: height}); err != nil {
		t.Fatalf("exact bounds rejected: %v", err)
	}
	if _, err := Sequence(diagram, Options{ASCII: true, MaxWidth: width - 1, MaxHeight: height}); !errors.Is(err, ErrOutputBounds) {
		t.Fatalf("short width error=%v", err)
	}
}

func TestSequenceFragmentRenderIsDeterministic(t *testing.T) {
	diagram, err := sequence.Parse(fragmentFixture, sequence.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	want := mustRenderSequence(t, diagram, Options{MaxWidth: 160, MaxHeight: 80})
	for run := 0; run < 256; run++ {
		got, renderErr := Sequence(diagram, Options{MaxWidth: 160, MaxHeight: 80})
		if renderErr != nil || got != want {
			t.Fatalf("run=%d err=%v changed=%v", run, renderErr, got != want)
		}
	}
}
