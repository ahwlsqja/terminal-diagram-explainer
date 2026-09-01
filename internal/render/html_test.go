package render

import (
	"strings"
	"testing"
)

func TestHTMLWrapsSVGInInteractiveViewer(t *testing.T) {
	const terminal = "┌────────┐\n│ Merge │\n└───┬────┘\n    ▼"
	html, err := HTML(terminal)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<!doctype html>",
		"<svg",
		"Merge",
		`data-action="zoom-in"`,
		`data-action="zoom-out"`,
		`data-action="fit"`,
		`aria-label="다이어그램 확대"`,
		"ResizeObserver",
		"pointerdown",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("interactive HTML token %q 누락", want)
		}
	}
	if strings.Contains(html, "<script src=") || strings.Contains(html, "fetch(") || strings.Contains(html, "WebSocket") {
		t.Fatal("interactive HTML must not load external runtime data")
	}
}

func TestHTMLIsDeterministic(t *testing.T) {
	const terminal = "┌─────┐\n│ API │\n└─────┘"
	want, err := HTML(terminal)
	if err != nil {
		t.Fatal(err)
	}
	for iteration := 0; iteration < 50; iteration++ {
		got, renderErr := HTML(terminal)
		if renderErr != nil {
			t.Fatal(renderErr)
		}
		if got != want {
			t.Fatalf("HTML changed at iteration %d", iteration)
		}
	}
}
