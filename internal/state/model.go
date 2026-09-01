// Package state는 의도적으로 제한한 stateDiagram-v2 부분집합을 처리한다.
package state

import (
	"fmt"
	"unicode"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

const MaxTextBytes = 4 * 1024

type Direction uint8

const (
	TopDown Direction = iota
	LeftRight
)

type EndpointKind uint8

const (
	InvalidEndpoint EndpointKind = iota
	StateRef
	Initial
	Final
)

// Endpoint는 pseudo state에 -1, 일반 state에 실제 state index를 사용한다.
type Endpoint struct {
	Kind  EndpointKind
	Index int
}

type State struct {
	ID    string
	Label string
}

type Transition struct {
	From  Endpoint
	To    Endpoint
	Event string
	Guard string
}

func (t Transition) Label() string {
	if t.Event == "" {
		return ""
	}
	if t.Guard == "" {
		return t.Event
	}
	return t.Event + " [" + t.Guard + "]"
}

func TextCells(text string) (int, error) {
	if len(text) == 0 || len(text) > MaxTextBytes {
		return 0, fmt.Errorf("state text byte 제한 초과")
	}
	for _, r := range text {
		if r != ' ' && unicode.IsSpace(r) {
			return 0, fmt.Errorf("state text의 Unicode whitespace는 허용하지 않음")
		}
		if r >= 0xFE00 && r <= 0xFE0F || r >= 0xE0100 && r <= 0xE01EF {
			return 0, fmt.Errorf("state text의 variation selector는 허용하지 않음")
		}
	}
	return textcell.Width(text)
}

func TransitionLabelCells(event, guard string) (int, error) {
	if event == "" {
		if guard == "" {
			return 0, nil
		}
		return 0, fmt.Errorf("guard 앞 event가 필요함")
	}
	eventWidth, err := TextCells(event)
	if err != nil {
		return 0, err
	}
	if guard == "" {
		return eventWidth, nil
	}
	guardWidth, err := TextCells(guard)
	if err != nil {
		return 0, err
	}
	return eventWidth + guardWidth + 3, nil
}

type Diagram struct {
	Direction   Direction
	States      []State
	Transitions []Transition
}

type Limits struct {
	MaxBytes       int
	MaxLines       int
	MaxStates      int
	MaxTransitions int
	MaxIDBytes     int
	MaxLabelCells  int
}

func DefaultLimits() Limits {
	return Limits{MaxBytes: 256 * 1024, MaxLines: 2048, MaxStates: 32, MaxTransitions: 64, MaxIDBytes: 64, MaxLabelCells: 96}
}
