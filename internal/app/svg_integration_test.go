package app

import (
	"strings"
	"testing"
)

func TestRunRendersSVGArtifact(t *testing.T) {
	result := invoke([]string{"-format", "svg", "-width", "120", "-fit"}, strings.NewReader(complexDependencyFlowSource))
	if result.code != 0 || result.stderr != "" {
		t.Fatalf("SVG render failed: %+v", result)
	}
	for _, want := range []string{"<svg", "<path", "미등록: Verification", "미등록: API · SPA · Production"} {
		if !strings.Contains(result.stdout, want) {
			t.Fatalf("SVG token %q 누락", want)
		}
	}
	if strings.ContainsAny(result.stdout, "┌┐└┘├┤┬┴┼─│▶▼") {
		t.Fatalf("SVG output에 terminal drawing glyph가 남음")
	}
}

func TestRunRejectsUnknownOutputFormat(t *testing.T) {
	result := invoke([]string{"-format", "pdf"}, strings.NewReader("flowchart LR\nA --> B\n"))
	if result.code != 2 || result.stdout != "" || !strings.Contains(result.stderr, "출력 형식") {
		t.Fatalf("unknown format result=%+v", result)
	}
}
