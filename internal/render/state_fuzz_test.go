package render

import (
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/state"
	"testing"
)

func FuzzStateRenderer(f *testing.F) {
	f.Add("A", "A")
	f.Fuzz(func(t *testing.T, id, label string) {
		d := &state.Diagram{States: []state.State{{ID: id, Label: label}}, Transitions: []state.Transition{{From: state.Endpoint{Kind: state.Initial, Index: -1}, To: state.Endpoint{Kind: state.StateRef, Index: 0}}}}
		_, _ = State(d, DefaultOptions())
	})
}
