package render

import (
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/state"
	"testing"
)

func FuzzStateRenderer(f *testing.F) {
	f.Add("A", "A")
	f.Fuzz(func(t *testing.T, id, label string) {
		d := &state.Diagram{
			States: []state.State{{ID: id, Label: label}},
			Transitions: []state.Transition{
				{From: state.Endpoint{Kind: state.Initial, Index: -1}, To: state.Endpoint{Kind: state.StateRef, Index: 0}},
				{From: state.Endpoint{Kind: state.StateRef, Index: 0}, To: state.Endpoint{Kind: state.StateRef, Index: 0}, Event: "retry"},
			},
			Policies: []state.TransitionPolicy{{TransitionIndex: 1, Kind: state.RetryPolicy, Detail: label}},
		}
		_, _ = State(d, DefaultOptions())
	})
}

func FuzzStateChoiceRenderer(f *testing.F) {
	f.Add("Choice")
	f.Fuzz(func(t *testing.T, label string) {
		diagram := &state.Diagram{
			States: []state.State{
				{ID: "A", Label: "A"},
				{ID: "X", Label: label, Kind: state.ChoiceState},
				{ID: "B", Label: "B"},
				{ID: "C", Label: "C"},
			},
			Transitions: []state.Transition{
				{From: state.Endpoint{Kind: state.Initial, Index: -1}, To: state.Endpoint{Kind: state.StateRef, Index: 0}},
				{From: state.Endpoint{Kind: state.StateRef, Index: 0}, To: state.Endpoint{Kind: state.StateRef, Index: 1}, Event: "evaluated"},
				{From: state.Endpoint{Kind: state.StateRef, Index: 1}, To: state.Endpoint{Kind: state.StateRef, Index: 2}, Guard: "b"},
				{From: state.Endpoint{Kind: state.StateRef, Index: 1}, To: state.Endpoint{Kind: state.StateRef, Index: 3}, Guard: "c"},
			},
		}
		_, _ = State(diagram, DefaultOptions())
	})
}
