package render

import (
	"errors"
	"fmt"
	"strings"
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
	for transitionIndex := 1; transitionIndex < len(diagram.Transitions); transitionIndex++ {
		diagram.Policies = append(diagram.Policies, state.TransitionPolicy{
			TransitionIndex: transitionIndex,
			Kind:            state.RetryPolicy,
			Detail:          fmt.Sprintf("policy%d", transitionIndex),
		})
	}
	diagram.Policies = append(diagram.Policies, state.TransitionPolicy{TransitionIndex: 1, Kind: state.TimeoutPolicy, Detail: "request deadline"})
	largeOutput, largeErr := State(diagram, Options{MaxWidth: 512, MaxHeight: 512})
	if largeErr != nil {
		t.Fatalf("maximum policy fixture failed: %v", largeErr)
	}
	if strings.Count(largeOutput, "  policy ") != 64 {
		t.Fatalf("policy lines=%d", strings.Count(largeOutput, "  policy "))
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

func TestStateMaximumMixedChoiceFixtureIsBounded(t *testing.T) {
	diagram := &state.Diagram{States: make([]state.State, 32)}
	for index := 0; index < 24; index++ {
		diagram.States[index] = state.State{ID: fmt.Sprintf("S%d", index), Label: fmt.Sprintf("S%d", index)}
	}
	for index := 0; index < 8; index++ {
		diagram.States[24+index] = state.State{ID: fmt.Sprintf("C%d", index), Label: fmt.Sprintf("C%d", index), Kind: state.ChoiceState}
	}
	diagram.Transitions = append(diagram.Transitions, state.Transition{From: state.Endpoint{Kind: state.Initial, Index: -1}, To: state.Endpoint{Kind: state.StateRef, Index: 0}})
	for index := 0; index < 8; index++ {
		choice := 24 + index
		diagram.Transitions = append(diagram.Transitions,
			state.Transition{From: state.Endpoint{Kind: state.StateRef, Index: index}, To: state.Endpoint{Kind: state.StateRef, Index: choice}, Event: fmt.Sprintf("evaluate%d", index)},
			state.Transition{From: state.Endpoint{Kind: state.StateRef, Index: choice}, To: state.Endpoint{Kind: state.StateRef, Index: 8 + index}, Guard: fmt.Sprintf("yes%d", index)},
			state.Transition{From: state.Endpoint{Kind: state.StateRef, Index: choice}, To: state.Endpoint{Kind: state.StateRef, Index: 16 + index}, Guard: fmt.Sprintf("no%d", index)},
		)
	}
	for _, direction := range []state.Direction{state.TopDown, state.LeftRight} {
		diagram.Direction = direction
		output, err := State(diagram, Options{MaxWidth: 512, MaxHeight: 512})
		if err != nil {
			t.Fatalf("direction=%v: %v", direction, err)
		}
		if strings.Count(output, "╱") != 16 {
			t.Fatalf("direction=%v choice diagonals=%d", direction, strings.Count(output, "╱"))
		}
	}
	allocations := testing.AllocsPerRun(10, func() {
		_, _ = State(diagram, Options{MaxWidth: 512, MaxHeight: 512})
	})
	if allocations > 2_500 {
		t.Fatalf("choice allocations/run=%.0f", allocations)
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
		switch mode % 13 {
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
		case 7:
			diagram.Policies = append(diagram.Policies, state.TransitionPolicy{TransitionIndex: 0, Kind: state.RetryPolicy, Detail: "bad target"})
		case 8:
			diagram.Policies = append(diagram.Policies, state.TransitionPolicy{TransitionIndex: 9, Kind: state.RetryPolicy, Detail: "bad index"})
		case 9:
			diagram.Transitions = append(diagram.Transitions, state.Transition{From: state.Endpoint{Kind: state.StateRef, Index: 0}, To: state.Endpoint{Kind: state.StateRef, Index: 1}, Event: "go"})
			diagram.Policies = append(diagram.Policies, state.TransitionPolicy{TransitionIndex: 1, Kind: state.PolicyKind(mode), Detail: "policy"})
		case 10:
			diagram.States[0].Kind = state.StateKind(99)
		case 11:
			diagram = directChoiceFixture()
		case 12:
			diagram = directChoiceFixture()
			diagram.Transitions = diagram.Transitions[:3]
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

func directChoiceFixture() *state.Diagram {
	return &state.Diagram{
		States: []state.State{
			{ID: "A", Label: "A"},
			{ID: "X", Label: "X", Kind: state.ChoiceState},
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
}
