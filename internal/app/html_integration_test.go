package app

import (
	"strings"
	"testing"
)

func TestRunRendersInteractiveHTMLArtifact(t *testing.T) {
	result := invoke([]string{"-format", "html", "-width", "120", "-fit"}, strings.NewReader(complexDependencyFlowSource))
	if result.code != 0 || result.stderr != "" {
		t.Fatalf("HTML render failed: %+v", result)
	}
	for _, want := range []string{"<!doctype html>", "<svg", "미등록: Verification", `data-action="fit"`} {
		if !strings.Contains(result.stdout, want) {
			t.Fatalf("HTML token %q 누락", want)
		}
	}
}
