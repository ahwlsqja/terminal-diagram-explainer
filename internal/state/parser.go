package state

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type ParseError struct {
	Line, Column int
	Message      string
}

type pendingTransition struct {
	transition Transition
	fromID     string
	toID       string
	line       int
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%d행 %d열: %s", e.Line, e.Column, e.Message)
}

func Parse(source string, limits Limits) (*Diagram, error) {
	rawBytes := len(source)
	if err := validateLimits(limits); err != nil {
		return nil, err
	}
	effective := normalizeLimits(limits)
	if rawBytes > effective.MaxBytes {
		return nil, perr(1, 1, "입력 크기 제한 초과")
	}
	if !utf8.ValidString(source) {
		return nil, perr(1, 1, "유효하지 않은 UTF-8")
	}
	if strings.Contains(source, "\r") {
		if strings.Contains(strings.ReplaceAll(source, "\r\n", ""), "\r") {
			return nil, perr(1, 1, "단독 CR 문자는 지원하지 않음")
		}
		source = strings.ReplaceAll(source, "\r\n", "\n")
	}
	if err := safeSource(source); err != nil {
		return nil, err
	}
	lines := strings.Split(source, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > effective.MaxLines {
		return nil, perr(effective.MaxLines+1, 1, "행 수 제한 초과")
	}
	d := &Diagram{Direction: TopDown}
	ids := make(map[string]int)
	labels := make(map[string]struct{})
	header := false
	directionSeen := false
	seenDeclarationOrTransition := false
	pending := make([]pendingTransition, 0, min(effective.MaxTransitions, 64))
	for n, raw := range lines {
		lineNo := n + 1
		line := strings.Trim(raw, " \t")
		if line == "" || strings.HasPrefix(line, "%%") {
			continue
		}
		if !header {
			if line != "stateDiagram-v2" {
				return nil, perr(lineNo, 1, "`stateDiagram-v2` 헤더가 필요함")
			}
			header = true
			continue
		}
		if strings.HasPrefix(line, "direction") {
			if directionSeen || seenDeclarationOrTransition {
				return nil, perr(lineNo, 1, "direction은 헤더 뒤 한 번만 허용됨")
			}
			if line == "direction TD" {
				d.Direction = TopDown
			} else if line == "direction LR" {
				d.Direction = LeftRight
			} else {
				return nil, perr(lineNo, 1, "지원 방향은 TD, LR뿐임")
			}
			directionSeen = true
			continue
		}
		if strings.HasPrefix(line, "state ") {
			seenDeclarationOrTransition = true
			st, err := parseState(line, lineNo, effective)
			if err != nil {
				return nil, err
			}
			if _, ok := ids[st.ID]; ok {
				return nil, perr(lineNo, 1, "중복 state ID")
			}
			if _, ok := labels[st.Label]; ok {
				return nil, perr(lineNo, 1, "중복 state label")
			}
			if len(d.States) >= effective.MaxStates {
				return nil, perr(lineNo, 1, "state 수 제한 초과")
			}
			ids[st.ID] = len(d.States)
			labels[st.Label] = struct{}{}
			d.States = append(d.States, st)
			continue
		}
		if strings.Contains(line, "-->") {
			seenDeclarationOrTransition = true
			tr, fromID, toID, err := parseTransition(line, lineNo, effective)
			if err != nil {
				return nil, err
			}
			if (tr.From.Kind != StateRef || tr.To.Kind != StateRef) && (tr.Event != "" || tr.Guard != "") {
				return nil, perr(lineNo, 1, "pseudo state transition에는 label을 붙일 수 없음")
			}
			pending = append(pending, pendingTransition{transition: tr, fromID: fromID, toID: toID, line: lineNo})
			if len(pending) > effective.MaxTransitions {
				return nil, perr(lineNo, 1, "transition 수 제한 초과")
			}
			continue
		}
		return nil, perr(lineNo, 1, "지원하지 않는 state 문법")
	}
	if !header {
		return nil, perr(1, 1, "stateDiagram-v2 헤더가 없음")
	}
	transitionLines := make([]int, 0, len(pending))
	for _, current := range pending {
		tr := current.transition
		if tr.From.Kind == StateRef {
			i, ok := ids[current.fromID]
			if !ok {
				return nil, perr(current.line, 1, "선언되지 않은 state ID")
			}
			tr.From.Index = i
		}
		if tr.To.Kind == StateRef {
			i, ok := ids[current.toID]
			if !ok {
				return nil, perr(current.line, 1, "선언되지 않은 state ID")
			}
			tr.To.Index = i
		}
		d.Transitions = append(d.Transitions, tr)
		transitionLines = append(transitionLines, current.line)
	}
	if len(d.States) == 0 {
		return nil, perr(1, 1, "state가 없음")
	}
	if len(d.Transitions) == 0 {
		return nil, perr(1, 1, "transition이 없음")
	}
	if err := validateSemantics(d, transitionLines); err != nil {
		return nil, err
	}
	return d, nil
}

func normalizeLimits(l Limits) Limits {
	d := DefaultLimits()
	if l.MaxBytes > 0 && l.MaxBytes < d.MaxBytes {
		d.MaxBytes = l.MaxBytes
	}
	if l.MaxLines > 0 && l.MaxLines < d.MaxLines {
		d.MaxLines = l.MaxLines
	}
	if l.MaxStates > 0 && l.MaxStates < d.MaxStates {
		d.MaxStates = l.MaxStates
	}
	if l.MaxTransitions > 0 && l.MaxTransitions < d.MaxTransitions {
		d.MaxTransitions = l.MaxTransitions
	}
	if l.MaxIDBytes > 0 && l.MaxIDBytes < d.MaxIDBytes {
		d.MaxIDBytes = l.MaxIDBytes
	}
	if l.MaxLabelCells > 0 && l.MaxLabelCells < d.MaxLabelCells {
		d.MaxLabelCells = l.MaxLabelCells
	}
	return d
}

func validateLimits(l Limits) error {
	if l.MaxBytes <= 0 || l.MaxLines <= 0 || l.MaxStates <= 0 || l.MaxTransitions <= 0 || l.MaxIDBytes <= 0 || l.MaxLabelCells <= 0 {
		return perr(1, 1, "모든 제한값은 양수여야 함")
	}
	return nil
}
func perr(line, col int, msg string) error { return &ParseError{line, col, msg} }

func safeSource(s string) error {
	line, column := 1, 1
	for _, r := range s {
		if r == '\n' {
			line++
			column = 1
			continue
		}
		if r == '\t' || r == ' ' {
			column++
			continue
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.IsSpace(r) || (r >= 0x200B && r <= 0x200F) || (r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069) || (r >= 0xFE00 && r <= 0xFE0F) || (r >= 0xE0100 && r <= 0xE01EF) {
			return perr(line, column, "지원하지 않는 제어 또는 공백 문자")
		}
		column++
	}
	return nil
}
func validID(s string, max int) bool {
	if len(s) == 0 || len(s) > max {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (i > 0 && c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}
func validLabel(s string, max int) bool {
	if s == "" || strings.ContainsRune(s, '"') {
		return false
	}
	w, e := TextCells(s)
	return e == nil && w > 0 && w <= max
}
func parseState(line string, n int, l Limits) (State, error) {
	body := strings.TrimPrefix(line, "state ")
	if strings.HasPrefix(body, "\"") {
		end := strings.Index(body[1:], "\"")
		if end < 0 {
			return State{}, perr(n, 7, "닫는 따옴표가 없음")
		}
		end++
		label := body[1:end]
		tail := strings.Trim(body[end+1:], " \t")
		if !strings.HasPrefix(tail, "as ") {
			return State{}, perr(n, 1, "state label에는 `as ID`가 필요함")
		}
		id := strings.TrimPrefix(tail, "as ")
		if !validID(id, l.MaxIDBytes) || !validLabel(label, l.MaxLabelCells) {
			return State{}, perr(n, 1, "유효하지 않은 state 선언")
		}
		return State{ID: id, Label: label}, nil
	}
	if !validID(body, l.MaxIDBytes) {
		return State{}, perr(n, 1, "유효하지 않은 state ID")
	}
	return State{ID: body, Label: body}, nil
}
func parseTransition(line string, n int, l Limits) (Transition, string, string, error) {
	parts := strings.Split(line, "-->")
	if len(parts) != 2 {
		return Transition{}, "", "", perr(n, 1, "transition은 하나의 `-->`만 허용됨")
	}
	if len(parts[0]) == 0 || len(parts[1]) == 0 || (parts[0][len(parts[0])-1] != ' ' && parts[0][len(parts[0])-1] != '\t') || (parts[1][0] != ' ' && parts[1][0] != '\t') {
		return Transition{}, "", "", perr(n, 1, "transition은 `A --> B` 형식이어야 함")
	}
	left := strings.Trim(parts[0], " \t")
	rest := strings.Trim(parts[1], " \t")
	endpoint := func(v string) (Endpoint, string, bool) {
		if v == "[*]" {
			return Endpoint{Kind: Initial, Index: -1}, "", true
		}
		if validID(v, l.MaxIDBytes) {
			return Endpoint{Kind: StateRef, Index: -2}, v, true
		}
		return Endpoint{}, "", false
	}
	right := rest
	label := ""
	if p := strings.Index(rest, " : "); p >= 0 {
		right = strings.Trim(rest[:p], " \t")
		label = rest[p+3:]
		if label == "" {
			return Transition{}, "", "", perr(n, p+4, "event가 비어 있음")
		}
	} else if strings.Contains(rest, ":") {
		return Transition{}, "", "", perr(n, 1, "label은 ` : event` 형식이어야 함")
	}
	from, fromID, ok := endpoint(left)
	if !ok {
		return Transition{}, "", "", perr(n, 1, "유효하지 않은 transition 시작점")
	}
	to, toID, ok := endpoint(right)
	if !ok {
		return Transition{}, "", "", perr(n, 1, "유효하지 않은 transition 끝점")
	}
	if from.Kind == Initial {
		from.Kind = Initial
	}
	if to.Kind == Initial {
		to.Kind = Final
	}
	tr := Transition{From: from, To: to}
	if label != "" {
		if strings.ContainsAny(label, "[]") {
			if !(strings.HasSuffix(label, "]") && strings.Count(label, "[") == 1 && strings.Count(label, "]") == 1) {
				return tr, "", "", perr(n, 1, "유효하지 않은 guard")
			}
			cut := strings.LastIndex(label, " [")
			if cut <= 0 {
				return tr, "", "", perr(n, 1, "guard 앞 event가 필요함")
			}
			tr.Event = label[:cut]
			tr.Guard = label[cut+2 : len(label)-1]
			if tr.Guard == "" {
				return tr, "", "", perr(n, 1, "guard가 비어 있음")
			}
		} else {
			tr.Event = label
		}
		width, widthErr := TransitionLabelCells(tr.Event, tr.Guard)
		if widthErr != nil || width == 0 || width > l.MaxLabelCells {
			return tr, "", "", perr(n, 1, "유효하지 않은 transition label")
		}
	}
	return tr, fromID, toID, nil
}
func validateSemantics(d *Diagram, lines []int) error {
	initial := 0
	seen := map[string]struct{}{}
	for index, t := range d.Transitions {
		line := lines[index]
		if t.From.Kind == Initial {
			initial++
			if t.To.Kind != StateRef {
				return perr(line, 1, "initial transition 방향이 잘못됨")
			}
			if initial > 1 {
				return perr(line, 1, "initial transition은 정확히 하나여야 함")
			}
		}
		if t.To.Kind == Final && t.From.Kind != StateRef {
			return perr(line, 1, "final transition 방향이 잘못됨")
		}
		if t.From.Kind == Final || t.To.Kind == Initial {
			return perr(line, 1, "pseudo state 방향이 잘못됨")
		}
		key := fmt.Sprintf("%d/%d/%d/%d/%s/%s", t.From.Kind, t.From.Index, t.To.Kind, t.To.Index, t.Event, t.Guard)
		if _, ok := seen[key]; ok {
			return perr(line, 1, "중복 transition")
		}
		seen[key] = struct{}{}
	}
	if initial != 1 {
		return perr(1, 1, "initial transition은 정확히 하나여야 함")
	}
	return nil
}
