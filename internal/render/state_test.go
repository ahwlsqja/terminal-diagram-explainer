package render

import (
	"errors"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/state"
	"strings"
	"testing"
)

func TestStateRendererMarkersAndFeedback(t *testing.T) {
	d, err := state.Parse("stateDiagram-v2\n[*] --> A\nA --> B : next\nB --> A : back\nB --> [*]\nC --> [*]\nstate A\nstate B\nstate C\n", state.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	got, err := State(d, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"● --> A", "B --> ◎", "feedback:\nB --> A : back"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	ascii, err := State(d, Options{ASCII: true, MaxWidth: 240, MaxHeight: 200})
	if err != nil || !strings.Contains(ascii, "(*) --> A") || !strings.Contains(ascii, "B --> (( ))") {
		t.Fatalf("ascii=%q err=%v", ascii, err)
	}
}

func TestStateRendererRejectsHostileEndpoint(t *testing.T) {
	_, err := State(&state.Diagram{States: []state.State{{ID: "A", Label: "A"}}, Transitions: []state.Transition{{From: state.Endpoint{Kind: state.Initial, Index: -1}, To: state.Endpoint{Kind: state.StateRef, Index: 2}}}}, DefaultOptions())
	if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("err=%v", err)
	}
}

func TestStateFeedbackUsesReachabilityNotDeclarationIndex(t *testing.T) {
	d, err := state.Parse("stateDiagram-v2\n[*] --> B\nB --> A : earlier\nA --> B : back\nA --> A : self\nstate A\nstate B\n", state.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	got, err := State(d, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "transitions:\nB --> A : earlier") {
		t.Fatalf("acyclic later-to-earlier edge classified as feedback:\n%s", got)
	}
	if !strings.Contains(got, "feedback:\nA --> B : back\nA --> A : self") {
		t.Fatalf("cycle/self feedback missing:\n%s", got)
	}
	if !strings.Contains(got, "◀") {
		t.Fatalf("TD connector arrow missing:\n%s", got)
	}
	for _, direction := range []state.Direction{state.TopDown, state.LeftRight} {
		d.Direction = direction
		out, renderErr := State(d, DefaultOptions())
		if renderErr != nil {
			t.Fatal(renderErr)
		}
		if direction == state.LeftRight && !strings.Contains(out, "▲") {
			t.Fatalf("LR connector arrow missing:\n%s", out)
		}
	}
}

func TestStateLRSelfTransitionUsesVisibleLateralLoop(t *testing.T) {
	diagram, err := state.Parse("stateDiagram-v2\ndirection LR\n[*] --> A\nA --> A : retry\nstate A\n", state.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		options Options
		arrow   string
	}{
		{options: DefaultOptions(), arrow: "◀"},
		{options: Options{ASCII: true, MaxWidth: 240, MaxHeight: 200}, arrow: "<"},
	} {
		output, renderErr := State(diagram, test.options)
		if renderErr != nil {
			t.Fatal(renderErr)
		}
		if !strings.Contains(output, test.arrow) || !strings.Contains(output, "feedback:\nA --> A : retry") {
			t.Fatalf("self loop missing for options=%+v:\n%s", test.options, output)
		}
		lines := strings.Split(output, "\n")
		connectorRows := 0
		for _, line := range lines {
			if strings.ContainsAny(line, "│─┼|+-") {
				connectorRows++
			}
		}
		if connectorRows < 4 {
			t.Fatalf("self loop collapsed to a dangling line:\n%s", output)
		}
	}
}

func TestStateRendererBoundsAndNoMutation(t *testing.T) {
	d, err := state.Parse("stateDiagram-v2\n[*] --> A\nA --> B\nB --> [*]\nstate \"한글"+" A\" as A\nstate B\n", state.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	before := *d
	got, err := State(d, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	w, h := measureStateOutput(got)
	canvasW, _ := stateBoxWidth(d)
	canvasW += 3 + 2*stateConcreteCount(d)
	canvasH := len(d.States)*5 - 1
	if canvasW > w {
		w = canvasW
	}
	if canvasH > h {
		h = canvasH
	}
	if _, err = State(d, Options{MaxWidth: w, MaxHeight: h}); err != nil {
		t.Fatalf("tight bounds: %v", err)
	}
	if _, err = State(d, Options{MaxWidth: w - 1, MaxHeight: h}); !errors.Is(err, ErrOutputBounds) {
		t.Fatalf("width bound: %v", err)
	}
	if _, err = State(d, Options{MaxWidth: w, MaxHeight: h - 1}); !errors.Is(err, ErrOutputBounds) {
		t.Fatalf("height bound: %v", err)
	}
	if d.Direction != before.Direction || len(d.States) != len(before.States) || len(d.Transitions) != len(before.Transitions) {
		t.Fatal("renderer mutated diagram")
	}
	for i := 0; i < 256; i++ {
		out, e := State(d, DefaultOptions())
		if e != nil || out != got {
			t.Fatalf("non-deterministic iteration %d", i)
		}
	}
}

func TestStateRendererDirectHostileMatrix(t *testing.T) {
	valid := &state.Diagram{States: []state.State{{ID: "A", Label: "A"}}, Transitions: []state.Transition{{From: state.Endpoint{Kind: state.Initial, Index: -1}, To: state.Endpoint{Kind: state.StateRef, Index: 0}}}}
	bad := []*state.Diagram{
		nil,
		{States: nil},
		{States: valid.States, Transitions: nil},
		{Direction: state.Direction(99), States: valid.States, Transitions: valid.Transitions},
		{States: []state.State{{ID: "A", Label: "x"}, {ID: "A", Label: "y"}}, Transitions: valid.Transitions},
		{States: []state.State{{ID: "A", Label: "x"}, {ID: "B", Label: "x"}}, Transitions: valid.Transitions},
		{States: valid.States, Transitions: []state.Transition{{From: state.Endpoint{Kind: state.EndpointKind(99), Index: -1}, To: state.Endpoint{Kind: state.StateRef, Index: 0}}}},
		{States: valid.States, Transitions: []state.Transition{{From: state.Endpoint{Kind: state.Final, Index: -1}, To: state.Endpoint{Kind: state.StateRef, Index: 0}}}},
		{States: valid.States, Transitions: []state.Transition{{From: state.Endpoint{Kind: state.Initial, Index: 0}, To: state.Endpoint{Kind: state.StateRef, Index: 0}}}},
		{States: valid.States, Transitions: []state.Transition{{From: state.Endpoint{Kind: state.Initial, Index: -1}, To: state.Endpoint{Kind: state.StateRef, Index: 0}, Event: "bad"}}},
		{States: valid.States, Transitions: []state.Transition{{From: state.Endpoint{Kind: state.Initial, Index: -1}, To: state.Endpoint{Kind: state.StateRef, Index: 0}}, {From: state.Endpoint{Kind: state.Initial, Index: -1}, To: state.Endpoint{Kind: state.StateRef, Index: 0}}}},
		{States: valid.States, Transitions: []state.Transition{{From: state.Endpoint{Kind: state.Initial, Index: -1}, To: state.Endpoint{Kind: state.StateRef, Index: 0}}, {From: state.Endpoint{Kind: state.StateRef, Index: 0}, To: state.Endpoint{Kind: state.StateRef, Index: 0}, Guard: "bad"}}},
		{States: []state.State{{ID: "A", Label: "A\u00a0B"}}, Transitions: valid.Transitions},
		{States: []state.State{{ID: "A", Label: "A\ufe00B"}}, Transitions: valid.Transitions},
		{States: valid.States, Transitions: []state.Transition{{From: state.Endpoint{Kind: state.Initial, Index: -1}, To: state.Endpoint{Kind: state.StateRef, Index: 0}}, {From: state.Endpoint{Kind: state.StateRef, Index: 0}, To: state.Endpoint{Kind: state.StateRef, Index: 0}, Event: strings.Repeat("x", state.MaxTextBytes+1)}}},
		{States: valid.States, Transitions: []state.Transition{{From: state.Endpoint{Kind: state.Initial, Index: -1}, To: state.Endpoint{Kind: state.StateRef, Index: 0}}, {From: state.Endpoint{Kind: state.StateRef, Index: 0}, To: state.Endpoint{Kind: state.StateRef, Index: 0}, Event: "go", Guard: strings.Repeat("x", state.MaxTextBytes+1)}}},
	}
	for _, d := range bad {
		if _, err := State(d, DefaultOptions()); !errors.Is(err, ErrInvalidState) {
			t.Fatalf("err=%v", err)
		}
	}
}

func TestStateRendererAttachesPoliciesWithoutChangingFeedback(t *testing.T) {
	diagram, err := state.Parse(`stateDiagram-v2
[*] --> A
A --> B : dispatch
B --> A : retry
state A
state B
policy A --> B : dispatch :: timeout "30s from enqueue"
policy A --> B : dispatch :: compensation "release reservation"
policy B --> A : retry :: retry "attempt below 3"
`, state.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	output, err := State(diagram, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	want := "transitions:\nA --> B : dispatch\n  policy timeout: 30s from enqueue\n  policy compensation: release reservation\n\nfeedback:\nB --> A : retry\n  policy retry: attempt below 3"
	if !strings.Contains(output, want) {
		t.Fatalf("policy legend mismatch:\n%s", output)
	}
}

func TestStateRendererDoesNotInferPolicyFromEvent(t *testing.T) {
	diagram, err := state.Parse("stateDiagram-v2\n[*] --> A\nA --> A : retry\nstate A\n", state.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	output, err := State(diagram, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output, "policy ") {
		t.Fatalf("event inferred as policy:\n%s", output)
	}
}

func TestStateRendererRejectsHostileDirectPolicies(t *testing.T) {
	valid := &state.Diagram{
		States: []state.State{{ID: "A", Label: "A"}, {ID: "B", Label: "B"}},
		Transitions: []state.Transition{
			{From: state.Endpoint{Kind: state.Initial, Index: -1}, To: state.Endpoint{Kind: state.StateRef, Index: 0}},
			{From: state.Endpoint{Kind: state.StateRef, Index: 0}, To: state.Endpoint{Kind: state.StateRef, Index: 1}, Event: "go"},
		},
	}
	bad := []state.TransitionPolicy{
		{TransitionIndex: -1, Kind: state.RetryPolicy, Detail: "x"},
		{TransitionIndex: 9, Kind: state.RetryPolicy, Detail: "x"},
		{TransitionIndex: 0, Kind: state.RetryPolicy, Detail: "x"},
		{TransitionIndex: 1, Kind: state.InvalidPolicy, Detail: "x"},
		{TransitionIndex: 1, Kind: state.PolicyKind(99), Detail: "x"},
		{TransitionIndex: 1, Kind: state.RetryPolicy, Detail: ""},
		{TransitionIndex: 1, Kind: state.RetryPolicy, Detail: "x\u00a0y"},
		{TransitionIndex: 1, Kind: state.RetryPolicy, Detail: "x\ufe0fy"},
		{TransitionIndex: 1, Kind: state.RetryPolicy, Detail: strings.Repeat("x", 97)},
	}
	for _, policy := range bad {
		diagram := *valid
		diagram.Policies = []state.TransitionPolicy{policy}
		if _, err := State(&diagram, DefaultOptions()); !errors.Is(err, ErrInvalidState) {
			t.Fatalf("policy=%#v err=%v", policy, err)
		}
	}
	diagram := *valid
	diagram.Policies = []state.TransitionPolicy{
		{TransitionIndex: 1, Kind: state.RetryPolicy, Detail: "first"},
		{TransitionIndex: 1, Kind: state.RetryPolicy, Detail: "second"},
	}
	if _, err := State(&diagram, DefaultOptions()); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("duplicate direct policy err=%v", err)
	}
	diagram = *valid
	diagram.Policies = make([]state.TransitionPolicy, 65)
	if _, err := State(&diagram, DefaultOptions()); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("65 direct policies err=%v", err)
	}
	for _, transition := range []state.Transition{
		{From: state.Endpoint{Kind: state.StateRef, Index: 0}, To: state.Endpoint{Kind: state.StateRef, Index: 1}, Event: "go \"quoted\""},
		{From: state.Endpoint{Kind: state.StateRef, Index: 0}, To: state.Endpoint{Kind: state.StateRef, Index: 1}, Event: "go", Guard: "\"ready\""},
	} {
		diagram = *valid
		diagram.Transitions[1] = transition
		diagram.Policies = []state.TransitionPolicy{{TransitionIndex: 1, Kind: state.RetryPolicy, Detail: "detail"}}
		if _, err := State(&diagram, DefaultOptions()); !errors.Is(err, ErrInvalidState) {
			t.Fatalf("quoted direct policy target=%#v err=%v", transition, err)
		}
	}
}

func TestStateRendererDrawsExplicitChoiceAndFeedback(t *testing.T) {
	diagram, err := state.Parse(`stateDiagram-v2
[*] --> Pending
Pending --> Decide : evaluated
Decide --> Accepted : [accepted]
Decide --> Pending : [retry]
state Pending
state "승인 결과" as Decide <<choice>>
state Accepted
`, state.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		options Options
		marker  string
	}{
		{options: DefaultOptions(), marker: "◁ 승인 결과 ▷"},
		{options: Options{ASCII: true, MaxWidth: 240, MaxHeight: 200}, marker: "< 승인 결과 >"},
	} {
		output, renderErr := State(diagram, test.options)
		if renderErr != nil {
			t.Fatal(renderErr)
		}
		if !strings.Contains(output, test.marker) || !strings.Contains(output, "Decide --> Accepted : [accepted]") {
			t.Fatalf("choice output missing marker=%q:\n%s", test.marker, output)
		}
		if !strings.Contains(output, "feedback:\nDecide --> Pending : [retry]") {
			t.Fatalf("choice feedback missing:\n%s", output)
		}
	}
}

func TestStateChoiceUsesDistinctInboundAndFanoutPorts(t *testing.T) {
	diagram, err := state.Parse(`stateDiagram-v2
[*] --> Pending
Pending --> Decide : evaluated
Decide --> Approved : [approved]
Decide --> Rejected : [rejected]
state Pending
state "승인 결과" as Decide <<choice>>
state Approved
state Rejected
`, state.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name      string
		direction state.Direction
		options   Options
		forbidden string
		required  string
	}{
		{name: "TD Unicode", direction: state.TopDown, options: DefaultOptions(), forbidden: "▷◀", required: "▷─"},
		{name: "TD ASCII", direction: state.TopDown, options: Options{ASCII: true, MaxWidth: 240, MaxHeight: 200}, forbidden: "><", required: ">-"},
		{name: "LR Unicode", direction: state.LeftRight, options: DefaultOptions(), required: "▶◁ 승인 결과 ▷"},
		{name: "LR ASCII", direction: state.LeftRight, options: Options{ASCII: true, MaxWidth: 240, MaxHeight: 200}, required: ">< 승인 결과 >"},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagram.Direction = test.direction
			output, renderErr := State(diagram, test.options)
			if renderErr != nil {
				t.Fatal(renderErr)
			}
			if test.forbidden != "" && strings.Contains(output, test.forbidden) {
				t.Fatalf("shared port overlap %q:\n%s", test.forbidden, output)
			}
			if !strings.Contains(output, test.required) {
				t.Fatalf("distinct port marker %q missing:\n%s", test.required, output)
			}
			width, height := measureStateOutput(output)
			canvasWidth, canvasHeight := choiceCanvasBounds(t, diagram)
			width = max(width, canvasWidth)
			height = max(height, canvasHeight)
			tight := test.options
			tight.MaxWidth, tight.MaxHeight = width, height
			if _, err := State(diagram, tight); err != nil {
				t.Fatalf("tight bounds %dx%d: %v", width, height, err)
			}
			tight.MaxWidth--
			if _, err := State(diagram, tight); !errors.Is(err, ErrOutputBounds) {
				t.Fatalf("width-1 bound err=%v", err)
			}
			tight.MaxWidth, tight.MaxHeight = width, height-1
			if _, err := State(diagram, tight); !errors.Is(err, ErrOutputBounds) {
				t.Fatalf("height-1 bound err=%v", err)
			}
		})
	}
}

func TestStateChoiceInboundPreservesDiamondInReverseOrder(t *testing.T) {
	diagram, err := state.Parse(`stateDiagram-v2
[*] --> Pending
Pending --> Decide : evaluated
Decide --> Approved : [approved]
Decide --> Rejected : [rejected]
state "승인 결과" as Decide <<choice>>
state Pending
state Approved
state Rejected
`, state.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		direction state.Direction
		options   Options
		marker    string
		arrow     string
	}{
		{direction: state.TopDown, options: DefaultOptions(), marker: "◁ 승인 결과 ▷", arrow: "▲"},
		{direction: state.TopDown, options: Options{ASCII: true, MaxWidth: 240, MaxHeight: 200}, marker: "< 승인 결과 >", arrow: "^"},
		{direction: state.LeftRight, options: DefaultOptions(), marker: "◁ 승인 결과 ▷◀"},
		{direction: state.LeftRight, options: Options{ASCII: true, MaxWidth: 240, MaxHeight: 200}, marker: "< 승인 결과 ><"},
	} {
		diagram.Direction = test.direction
		output, renderErr := State(diagram, test.options)
		if renderErr != nil {
			t.Fatal(renderErr)
		}
		if !strings.Contains(output, test.marker) {
			t.Fatalf("reverse-order diamond marker %q missing:\n%s", test.marker, output)
		}
		if test.arrow != "" && !strings.Contains(output, test.arrow) {
			t.Fatalf("reverse-order inbound arrow %q missing:\n%s", test.arrow, output)
		}
	}
}

func choiceCanvasBounds(t *testing.T, diagram *state.Diagram) (int, int) {
	t.Helper()
	lanes := stateConnectorLaneCount(diagram)
	if diagram.Direction == state.TopDown {
		width, err := stateBoxWidth(diagram)
		if err != nil {
			t.Fatal(err)
		}
		return width + 3 + 2*lanes, len(diagram.States)*5 - 1
	}
	width := 0
	boxWidths := make([]int, len(diagram.States))
	for index, current := range diagram.States {
		boxWidth, err := stateSingleBoxWidth(current)
		if err != nil {
			t.Fatal(err)
		}
		boxWidths[index] = boxWidth
		width += boxWidth
		if index > 0 {
			width += 5
		}
	}
	for _, transition := range diagram.Transitions {
		if transition.From.Kind != state.StateRef || transition.To.Kind != state.StateRef || transition.From.Index != transition.To.Index {
			continue
		}
		boxRight := 0
		for index := 0; index <= transition.From.Index; index++ {
			boxRight += boxWidths[index]
			if index > 0 {
				boxRight += 5
			}
		}
		width = max(width, boxRight+3)
	}
	return width, 6 + 2*lanes
}

func TestStateRendererRejectsHostileDirectChoices(t *testing.T) {
	base := &state.Diagram{
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
	mutations := []func(*state.Diagram){
		func(d *state.Diagram) { d.States[1].Kind = state.StateKind(99) },
		func(d *state.Diagram) { d.Transitions = d.Transitions[:3] },
		func(d *state.Diagram) { d.Transitions = append(d.Transitions, d.Transitions[1]) },
		func(d *state.Diagram) { d.Transitions[2].Event = "branch" },
		func(d *state.Diagram) { d.Transitions[2].Guard = "" },
		func(d *state.Diagram) { d.Transitions[3].Guard = "b" },
		func(d *state.Diagram) { d.Transitions[3].To.Index = 2 },
		func(d *state.Diagram) { d.Transitions[0].To.Index = 1 },
		func(d *state.Diagram) {
			d.Policies = []state.TransitionPolicy{{TransitionIndex: 1, Kind: state.TimeoutPolicy, Detail: "deadline"}}
		},
	}
	for _, mutate := range mutations {
		diagram := cloneStateDiagram(base)
		mutate(diagram)
		if _, err := State(diagram, DefaultOptions()); !errors.Is(err, ErrInvalidState) {
			t.Fatalf("hostile choice accepted: %#v err=%v", diagram, err)
		}
	}
}

func cloneStateDiagram(diagram *state.Diagram) *state.Diagram {
	clone := *diagram
	clone.States = append([]state.State(nil), diagram.States...)
	clone.Transitions = append([]state.Transition(nil), diagram.Transitions...)
	clone.Policies = append([]state.TransitionPolicy(nil), diagram.Policies...)
	return &clone
}
