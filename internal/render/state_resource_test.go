package render

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/state"
)

func TestStateMaximumFixtureIsBoundedAndAllocationLimited(t *testing.T) {
	diagram := &state.Diagram{Direction: state.TopDown, States: make([]state.State, 32)}
	for index := range diagram.States {
		diagram.States[index] = state.State{ID: fmt.Sprintf("S%d", index), Label: fmt.Sprintf("S%d", index)}
	}
	diagram.Transitions = append(diagram.Transitions, state.Transition{
		From: state.Endpoint{Kind: state.Initial, Index: -1},
		To:   state.Endpoint{Kind: state.StateRef, Index: 0},
	})
	for index := 0; index < 31; index++ {
		diagram.Transitions = append(diagram.Transitions, state.Transition{
			From:  state.Endpoint{Kind: state.StateRef, Index: index},
			To:    state.Endpoint{Kind: state.StateRef, Index: index + 1},
			Event: fmt.Sprintf("next%d", index),
		})
	}
	for index := 0; index < 32; index++ {
		diagram.Transitions = append(diagram.Transitions, state.Transition{
			From:  state.Endpoint{Kind: state.StateRef, Index: index},
			To:    state.Endpoint{Kind: state.StateRef, Index: (index + 2) % 32},
			Event: fmt.Sprintf("extra%d", index),
		})
	}
	output, err := State(diagram, DefaultOptions())
	if err != nil && !errors.Is(err, ErrOutputBounds) {
		t.Fatalf("maximum fixture output=%q err=%v", output, err)
	}
	if err != nil && output != "" {
		t.Fatalf("bounds failure returned partial output: %q", output)
	}
	allocations := testing.AllocsPerRun(10, func() {
		_, _ = State(diagram, DefaultOptions())
	})
	if allocations > 2_500 {
		t.Fatalf("state allocations/run=%.0f", allocations)
	}
}

func FuzzStateDirectASTNoPanic(f *testing.F) {
	for _, seed := range []uint8{0, 1, 2, 3, 4, 5, 255} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, mode uint8) {
		diagram := &state.Diagram{
			States: []state.State{{ID: "A", Label: "A"}, {ID: "B", Label: "B"}},
			Transitions: []state.Transition{{
				From: state.Endpoint{Kind: state.Initial, Index: -1},
				To:   state.Endpoint{Kind: state.StateRef, Index: 0},
			}},
		}
		switch mode % 7 {
		case 1:
			diagram.Direction = state.Direction(99)
		case 2:
			diagram.Transitions[0].To.Index = 9
		case 3:
			diagram.Transitions[0].From.Index = 0
		case 4:
			diagram.Transitions = append(diagram.Transitions, state.Transition{From: state.Endpoint{Kind: state.StateRef, Index: 0}, To: state.Endpoint{Kind: state.StateRef, Index: 1}, Event: "go"})
		case 5:
			diagram.Transitions = append(diagram.Transitions, state.Transition{From: state.Endpoint{Kind: state.StateRef, Index: 1}, To: state.Endpoint{Kind: state.StateRef, Index: 0}, Event: "back"})
		case 6:
			diagram.Transitions = append(diagram.Transitions, state.Transition{From: state.Endpoint{Kind: state.StateRef, Index: 0}, To: state.Endpoint{Kind: state.Final, Index: -1}})
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("State panicked: %v", recovered)
			}
		}()
		output, err := State(diagram, Options{MaxWidth: 512, MaxHeight: 512})
		if err != nil && output != "" {
			t.Fatalf("partial output=%q err=%v", output, err)
		}
	})
}
