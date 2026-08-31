package sequence

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

type ParseError struct {
	Line    int
	Column  int
	Message string
}

func (e *ParseError) Error() string {
	if e.Column > 0 {
		return fmt.Sprintf("%d행 %d열: %s", e.Line, e.Column, e.Message)
	}
	return fmt.Sprintf("%d행: %s", e.Line, e.Message)
}

type sourceLine struct {
	text       string
	lineNo     int
	baseColumn int
}

type fragmentFrame struct {
	kind           FragmentKind
	openLine       int
	openColumn     int
	branchMessages int
	sawElse        bool
	branchCount    int
}

type activationOpen struct {
	line         int
	column       int
	messageCount int
}

func Parse(source string, limits Limits) (*Diagram, error) {
	if err := validateLimits(limits); err != nil {
		return nil, err
	}
	if len(source) > limits.MaxSourceBytes {
		return nil, &ParseError{Line: 1, Column: 1, Message: "입력 크기 제한 초과"}
	}
	if !utf8.ValidString(source) {
		return nil, &ParseError{Line: 1, Column: 1, Message: "유효하지 않은 UTF-8"}
	}
	source = strings.ReplaceAll(source, "\r\n", "\n")
	if strings.ContainsRune(source, '\r') {
		return nil, &ParseError{Line: 1, Column: 1, Message: "단독 CR 문자는 지원하지 않음"}
	}

	rawLines := strings.Split(source, "\n")
	if strings.HasSuffix(source, "\n") && len(rawLines) > 0 && rawLines[len(rawLines)-1] == "" {
		rawLines = rawLines[:len(rawLines)-1]
	}
	if len(rawLines) > limits.MaxLines {
		return nil, &ParseError{Line: limits.MaxLines + 1, Message: "행 수 제한 초과"}
	}

	diagram := &Diagram{}
	participants := make(map[string]int)
	labels := make(map[string]struct{})
	fragments := make([]fragmentFrame, 0)
	activations := make([][]activationOpen, 0)
	headerFound := false
	timelineStarted := false
	messageCount := 0
	fragmentCount := 0
	activationCount := 0
	for index, raw := range rawLines {
		line := normalizeSourceLine(raw, index+1)
		if line.text == "" || strings.HasPrefix(line.text, "%%") {
			continue
		}
		if !headerFound {
			if line.text != "sequenceDiagram" {
				return nil, sequenceError(line, 1, "`sequenceDiagram` 헤더가 필요함")
			}
			headerFound = true
			continue
		}

		if isParticipantDeclaration(line.text) {
			if timelineStarted {
				return nil, sequenceError(line, 1, "participant 선언은 message보다 앞에 있어야 함")
			}
			participant, idColumn, labelColumn, err := parseParticipant(line, limits)
			if err != nil {
				return nil, err
			}
			if _, exists := participants[participant.ID]; exists {
				return nil, &ParseError{Line: line.lineNo, Column: idColumn, Message: fmt.Sprintf("participant %s가 중복됨", participant.ID)}
			}
			if _, exists := labels[participant.Label]; exists {
				return nil, &ParseError{Line: line.lineNo, Column: labelColumn, Message: fmt.Sprintf("participant label %q가 중복됨", participant.Label)}
			}
			if len(diagram.Participants) >= limits.MaxParticipants {
				return nil, &ParseError{Line: line.lineNo, Column: idColumn, Message: "participant 수 제한 초과"}
			}
			participants[participant.ID] = len(diagram.Participants)
			labels[participant.Label] = struct{}{}
			diagram.Participants = append(diagram.Participants, participant)
			activations = append(activations, nil)
			continue
		}

		if isMessageCandidate(line.text) {
			message, arrowColumn, senderColumn, receiverColumn, err := parseMessage(line, limits)
			if err != nil {
				return nil, err
			}
			from, exists := participants[messageSenderID(line.text)]
			if !exists {
				return nil, &ParseError{Line: line.lineNo, Column: senderColumn, Message: "선언되지 않은 sender participant"}
			}
			to, exists := participants[messageReceiverID(line.text)]
			if !exists {
				return nil, &ParseError{Line: line.lineNo, Column: receiverColumn, Message: "선언되지 않은 receiver participant"}
			}
			if messageCount >= limits.MaxMessages {
				return nil, &ParseError{Line: line.lineNo, Column: arrowColumn, Message: "message 수 제한 초과"}
			}
			message.From = from
			message.To = to
			if diagram.Steps == nil {
				if messageCount >= limits.MaxSteps {
					return nil, &ParseError{Line: line.lineNo, Column: arrowColumn, Message: "step 수 제한 초과"}
				}
				diagram.Messages = append(diagram.Messages, message)
			} else {
				if len(diagram.Steps) >= limits.MaxSteps {
					return nil, &ParseError{Line: line.lineNo, Column: arrowColumn, Message: "step 수 제한 초과"}
				}
				diagram.Steps = append(diagram.Steps, Step{Kind: MessageStep, Message: message})
			}
			messageCount++
			for frameIndex := range fragments {
				fragments[frameIndex].branchMessages++
			}
			timelineStarted = true
			continue
		}

		step, recognized, err := parseFragmentControl(line, limits)
		if err != nil {
			return nil, err
		}
		if !recognized {
			_, _, _, _, messageErr := parseMessage(line, limits)
			return nil, messageErr
		}
		if step.Kind == ActivateStep || step.Kind == DeactivateStep {
			participantID := step.Label
			participantIndex, exists := participants[participantID]
			if !exists {
				return nil, sequenceError(line, activationIDColumn(line.text, step.Kind), "선언되지 않은 activation participant")
			}
			step.Participant = participantIndex
			step.Label = ""
		}
		if len(diagram.Steps) == 0 && diagram.Steps == nil {
			if len(diagram.Messages)+1 > limits.MaxSteps {
				return nil, sequenceError(line, 1, "step 수 제한 초과")
			}
			promoted := make([]Step, len(diagram.Messages), len(diagram.Messages)+1)
			for messageIndex, message := range diagram.Messages {
				promoted[messageIndex] = Step{Kind: MessageStep, Message: message}
			}
			diagram.Messages = nil
			diagram.Steps = promoted
		}
		if len(diagram.Steps) >= limits.MaxSteps {
			return nil, sequenceError(line, 1, "step 수 제한 초과")
		}
		switch step.Kind {
		case FragmentStartStep:
			if !activationStacksEmpty(activations) {
				return nil, sequenceError(line, 1, "activation은 fragment 경계를 넘을 수 없음")
			}
			if fragmentCount >= limits.MaxFragments {
				return nil, sequenceError(line, 1, "fragment 수 제한 초과")
			}
			if len(fragments) >= limits.MaxFragmentDepth {
				return nil, sequenceError(line, 1, "fragment 중첩 깊이 제한 초과")
			}
			fragments = append(fragments, fragmentFrame{
				kind:       step.Fragment,
				openLine:   line.lineNo,
				openColumn: line.baseColumn,
			})
			fragmentCount++
		case FragmentBranchStep:
			if !activationStacksEmpty(activations) {
				return nil, sequenceError(line, 1, "activation은 fragment branch를 넘을 수 없음")
			}
			if len(fragments) == 0 {
				return nil, sequenceError(line, 1, "열린 alt fragment가 없는 `else`")
			}
			frame := &fragments[len(fragments)-1]
			switch {
			case frame.kind == AltFragment && step.Branch == ElseBranch:
				if frame.sawElse {
					return nil, sequenceError(line, 1, "alt fragment의 `else`가 중복됨")
				}
				frame.sawElse = true
			case frame.kind == ParFragment && step.Branch == AndBranch:
			case frame.kind == AltFragment:
				return nil, sequenceError(line, 1, "alt fragment에서는 `else`만 허용함")
			case frame.kind == ParFragment:
				return nil, sequenceError(line, 1, "par fragment에서는 `and`만 허용함")
			default:
				return nil, sequenceError(line, 1, "fragment branch가 허용되지 않음")
			}
			if frame.branchMessages == 0 {
				return nil, sequenceError(line, 1, "빈 fragment branch는 허용하지 않음")
			}
			frame.branchMessages = 0
			frame.branchCount++
		case FragmentEndStep:
			if !activationStacksEmpty(activations) {
				return nil, sequenceError(line, 1, "activation은 fragment 경계를 넘을 수 없음")
			}
			if len(fragments) == 0 {
				return nil, sequenceError(line, 1, "열린 fragment가 없는 `end`")
			}
			frame := fragments[len(fragments)-1]
			if frame.branchMessages == 0 {
				return nil, sequenceError(line, 1, "빈 fragment branch는 허용하지 않음")
			}
			if frame.kind == AltFragment && !frame.sawElse {
				return nil, sequenceError(line, 1, "alt fragment에는 `else`가 필요함")
			}
			if frame.kind == ParFragment && frame.branchCount == 0 {
				return nil, sequenceError(line, 1, "par fragment에는 `and`가 필요함")
			}
			fragments = fragments[:len(fragments)-1]
		case ActivateStep:
			if activationCount >= limits.MaxActivations {
				return nil, sequenceError(line, activationIDColumn(line.text, step.Kind), "activation 수 제한 초과")
			}
			stack := activations[step.Participant]
			if len(stack) >= limits.MaxActivationDepth {
				return nil, sequenceError(line, activationIDColumn(line.text, step.Kind), "activation 중첩 깊이 제한 초과")
			}
			activations[step.Participant] = append(stack, activationOpen{
				line: line.lineNo, column: line.baseColumn,
				messageCount: messageCount,
			})
			activationCount++
		case DeactivateStep:
			stack := activations[step.Participant]
			if len(stack) == 0 {
				return nil, sequenceError(line, 1, "대응하는 activate가 없는 deactivate")
			}
			open := stack[len(stack)-1]
			if messageCount == open.messageCount {
				return nil, sequenceError(line, 1, "message가 없는 activation은 허용하지 않음")
			}
			activations[step.Participant] = stack[:len(stack)-1]
		}
		diagram.Steps = append(diagram.Steps, step)
		timelineStarted = true
	}

	if !headerFound {
		return nil, &ParseError{Line: 1, Column: 1, Message: "`sequenceDiagram` 헤더가 필요함"}
	}
	if len(diagram.Participants) == 0 {
		return nil, &ParseError{Line: 1, Column: 1, Message: "participant가 없음"}
	}
	if len(fragments) > 0 {
		frame := fragments[len(fragments)-1]
		return nil, &ParseError{Line: frame.openLine, Column: frame.openColumn, Message: "fragment를 닫는 `end`가 없음"}
	}
	if line, column, active := firstOpenActivation(activations); active {
		return nil, &ParseError{Line: line, Column: column, Message: "대응하는 deactivate가 없음"}
	}
	if messageCount == 0 {
		return nil, &ParseError{Line: 1, Column: 1, Message: "message가 없음"}
	}
	return diagram, nil
}

func validateLimits(limits Limits) error {
	if limits.MaxSourceBytes <= 0 || limits.MaxLines <= 0 || limits.MaxParticipants <= 0 ||
		limits.MaxMessages <= 0 || limits.MaxSteps <= 0 || limits.MaxFragments <= 0 ||
		limits.MaxFragmentDepth <= 0 || limits.MaxActivations <= 0 || limits.MaxActivationDepth <= 0 ||
		limits.MaxIDBytes <= 0 || limits.MaxLabelCells <= 0 {
		return &ParseError{Line: 1, Column: 1, Message: "모든 parser 제한은 양수여야 함"}
	}
	return nil
}

func normalizeSourceLine(raw string, lineNo int) sourceLine {
	leftTrimmed := strings.TrimLeftFunc(raw, unicode.IsSpace)
	return sourceLine{
		text:       strings.TrimSpace(raw),
		lineNo:     lineNo,
		baseColumn: len(raw) - len(leftTrimmed) + 1,
	}
}

func isParticipantDeclaration(line string) bool {
	const keyword = "participant"
	return line == keyword || strings.HasPrefix(line, keyword+" ") || strings.HasPrefix(line, keyword+"\t")
}

func isMessageCandidate(line string) bool {
	_, pos := scanID(line, 0, true)
	pos = skipHSpace(line, pos)
	return pos < len(line) && (strings.HasPrefix(line[pos:], "->>") || strings.HasPrefix(line[pos:], "-->>"))
}

func parseFragmentControl(line sourceLine, limits Limits) (Step, bool, error) {
	if line.text == "end" {
		return Step{Kind: FragmentEndStep}, true, nil
	}
	for _, activation := range []struct {
		keyword string
		kind    StepKind
	}{
		{keyword: "activate", kind: ActivateStep},
		{keyword: "deactivate", kind: DeactivateStep},
	} {
		if !hasKeyword(line.text, activation.keyword) {
			continue
		}
		pos := len(activation.keyword)
		if pos >= len(line.text) || !isHSpace(line.text[pos]) {
			return Step{}, true, sequenceError(line, pos+1, fmt.Sprintf("`%s` participant가 필요함", activation.keyword))
		}
		pos = skipHSpace(line.text, pos)
		idStart := pos
		id, next := scanID(line.text, pos, false)
		if id == "" {
			return Step{}, true, sequenceError(line, idStart+1, fmt.Sprintf("`%s` participant가 필요함", activation.keyword))
		}
		if err := validateID(id, limits.MaxIDBytes); err != nil {
			return Step{}, true, sequenceError(line, idStart+1, err.Error())
		}
		if next != len(line.text) {
			return Step{}, true, sequenceError(line, next+1, fmt.Sprintf("지원하지 않는 `%s` 문법", activation.keyword))
		}
		return Step{Kind: activation.kind, Label: id}, true, nil
	}
	for _, candidate := range []struct {
		keyword  string
		kind     StepKind
		fragment FragmentKind
		branch   BranchKind
	}{
		{keyword: "loop", kind: FragmentStartStep, fragment: LoopFragment},
		{keyword: "alt", kind: FragmentStartStep, fragment: AltFragment},
		{keyword: "opt", kind: FragmentStartStep, fragment: OptFragment},
		{keyword: "par", kind: FragmentStartStep, fragment: ParFragment},
		{keyword: "else", kind: FragmentBranchStep, branch: ElseBranch},
		{keyword: "and", kind: FragmentBranchStep, branch: AndBranch},
	} {
		if !hasKeyword(line.text, candidate.keyword) {
			continue
		}
		pos := len(candidate.keyword)
		if pos >= len(line.text) || !isHSpace(line.text[pos]) {
			return Step{}, true, sequenceError(line, pos+1, fmt.Sprintf("`%s` label이 필요함", candidate.keyword))
		}
		pos = skipHSpace(line.text, pos)
		label := strings.TrimSpace(line.text[pos:])
		if label == "" {
			return Step{}, true, sequenceError(line, pos+1, fmt.Sprintf("빈 `%s` label은 허용하지 않음", candidate.keyword))
		}
		if err := validateLabel(label, limits.MaxLabelCells); err != nil {
			return Step{}, true, sequenceError(line, pos+1, err.Error())
		}
		step := Step{Kind: candidate.kind, Fragment: candidate.fragment, Label: label}
		if candidate.kind == FragmentBranchStep {
			step.Branch = candidate.branch
			step.Fragment = LoopFragment
		}
		return step, true, nil
	}
	return Step{}, false, nil
}

func activationIDColumn(line string, kind StepKind) int {
	keyword := "activate"
	if kind == DeactivateStep {
		keyword = "deactivate"
	}
	return skipHSpace(line, len(keyword)) + 1
}

func activationStacksEmpty(stacks [][]activationOpen) bool {
	for _, stack := range stacks {
		if len(stack) != 0 {
			return false
		}
	}
	return true
}

func firstOpenActivation(stacks [][]activationOpen) (int, int, bool) {
	line, column := 0, 0
	for _, stack := range stacks {
		for _, open := range stack {
			if line == 0 || open.line < line || open.line == line && open.column < column {
				line, column = open.line, open.column
			}
		}
	}
	return line, column, line != 0
}

func hasKeyword(line, keyword string) bool {
	return line == keyword || strings.HasPrefix(line, keyword+" ") || strings.HasPrefix(line, keyword+"\t")
}

func parseParticipant(line sourceLine, limits Limits) (Participant, int, int, error) {
	pos := len("participant")
	if pos == len(line.text) {
		return Participant{}, 0, 0, sequenceError(line, pos+1, "participant ID가 필요함")
	}
	if !isHSpace(line.text[pos]) {
		return Participant{}, 0, 0, sequenceError(line, pos+1, "participant 뒤에 공백이 필요함")
	}
	pos = skipHSpace(line.text, pos)
	idStart := pos
	id, next := scanID(line.text, pos, false)
	if id == "" {
		return Participant{}, 0, 0, sequenceError(line, idStart+1, "participant ID가 필요함")
	}
	if err := validateID(id, limits.MaxIDBytes); err != nil {
		return Participant{}, 0, 0, sequenceError(line, idStart+1, err.Error())
	}
	if id == "participant" {
		return Participant{}, 0, 0, sequenceError(line, idStart+1, "`participant`는 ID로 사용할 수 없음")
	}
	participant := Participant{ID: id, Label: id}
	idColumn := line.baseColumn + idStart
	labelColumn := idColumn
	pos = next
	if pos == len(line.text) {
		return participant, idColumn, labelColumn, nil
	}
	if !isHSpace(line.text[pos]) {
		return Participant{}, 0, 0, sequenceError(line, pos+1, "지원하지 않는 participant 문법")
	}
	pos = skipHSpace(line.text, pos)
	if !strings.HasPrefix(line.text[pos:], "as") || pos+2 >= len(line.text) || !isHSpace(line.text[pos+2]) {
		return Participant{}, 0, 0, sequenceError(line, pos+1, "participant alias는 `as` 문법만 지원함")
	}
	pos = skipHSpace(line.text, pos+2)
	if pos >= len(line.text) {
		return Participant{}, 0, 0, sequenceError(line, pos+1, "빈 participant label은 허용하지 않음")
	}
	label := strings.TrimSpace(line.text[pos:])
	if label == "" {
		return Participant{}, 0, 0, sequenceError(line, pos+1, "빈 participant label은 허용하지 않음")
	}
	if err := validateLabel(label, limits.MaxLabelCells); err != nil {
		return Participant{}, 0, 0, sequenceError(line, pos+1, err.Error())
	}
	participant.Label = label
	labelColumn = line.baseColumn + pos
	return participant, idColumn, labelColumn, nil
}

func parseMessage(line sourceLine, limits Limits) (Message, int, int, int, error) {
	pos := 0
	senderStart := pos
	sender, next := scanID(line.text, pos, true)
	if sender == "" {
		return Message{}, 0, 0, 0, sequenceError(line, senderStart+1, "sender participant ID가 필요함")
	}
	if err := validateID(sender, limits.MaxIDBytes); err != nil {
		return Message{}, 0, 0, 0, sequenceError(line, senderStart+1, err.Error())
	}
	if sender == "participant" {
		return Message{}, 0, 0, 0, sequenceError(line, senderStart+1, "`participant`는 ID로 사용할 수 없음")
	}
	pos = skipHSpace(line.text, next)
	arrowStart := pos
	kind := Request
	switch {
	case strings.HasPrefix(line.text[pos:], "-->>"):
		kind = Return
		pos += 4
	case strings.HasPrefix(line.text[pos:], "->>"):
		pos += 3
	default:
		return Message{}, 0, 0, 0, sequenceError(line, pos+1, "지원 message 화살표는 `->>`와 `-->>`뿐임")
	}
	pos = skipHSpace(line.text, pos)
	receiverStart := pos
	receiver, next := scanID(line.text, pos, false)
	if receiver == "" {
		return Message{}, 0, 0, 0, sequenceError(line, receiverStart+1, "receiver participant ID가 필요함")
	}
	if err := validateID(receiver, limits.MaxIDBytes); err != nil {
		return Message{}, 0, 0, 0, sequenceError(line, receiverStart+1, err.Error())
	}
	if receiver == "participant" {
		return Message{}, 0, 0, 0, sequenceError(line, receiverStart+1, "`participant`는 ID로 사용할 수 없음")
	}
	pos = skipHSpace(line.text, next)
	if pos >= len(line.text) || line.text[pos] != ':' {
		return Message{}, 0, 0, 0, sequenceError(line, pos+1, "message label 앞에 `:`가 필요함")
	}
	colon := pos
	pos++
	labelStart := skipHSpace(line.text, pos)
	label := strings.TrimSpace(line.text[labelStart:])
	if label == "" {
		return Message{}, 0, 0, 0, sequenceError(line, colon+1, "빈 message label은 허용하지 않음")
	}
	if err := validateLabel(label, limits.MaxLabelCells); err != nil {
		return Message{}, 0, 0, 0, sequenceError(line, labelStart+1, err.Error())
	}
	return Message{Label: label, Kind: kind},
		line.baseColumn + arrowStart,
		line.baseColumn + senderStart,
		line.baseColumn + receiverStart,
		nil
}

func scanID(line string, start int, stopAtArrow bool) (string, int) {
	if start >= len(line) || !isIDStart(line[start]) {
		return "", start
	}
	pos := start + 1
	for pos < len(line) {
		if stopAtArrow && (strings.HasPrefix(line[pos:], "->>") || strings.HasPrefix(line[pos:], "-->>")) {
			break
		}
		if !isIDPart(line[pos]) {
			break
		}
		pos++
	}
	return line[start:pos], pos
}

func messageSenderID(line string) string {
	id, _ := scanID(line, 0, true)
	return id
}

func messageReceiverID(line string) string {
	_, pos := scanID(line, 0, true)
	pos = skipHSpace(line, pos)
	if strings.HasPrefix(line[pos:], "-->>") {
		pos += 4
	} else if strings.HasPrefix(line[pos:], "->>") {
		pos += 3
	}
	pos = skipHSpace(line, pos)
	id, _ := scanID(line, pos, false)
	return id
}

func validateID(id string, maxBytes int) error {
	if id == "" || len(id) > maxBytes {
		return fmt.Errorf("participant ID 길이 제한 초과")
	}
	if !isIDStart(id[0]) {
		return fmt.Errorf("participant ID가 유효하지 않음")
	}
	for index := 1; index < len(id); index++ {
		if !isIDPart(id[index]) {
			return fmt.Errorf("participant ID가 유효하지 않음")
		}
	}
	return nil
}

func validateLabel(label string, maxCells int) error {
	width, err := textcell.Width(label)
	if err != nil {
		return fmt.Errorf("제어 문자 또는 지원하지 않는 label: %w", err)
	}
	if width == 0 {
		return fmt.Errorf("빈 label은 허용하지 않음")
	}
	if width > maxCells {
		return fmt.Errorf("label 폭 제한 초과: %d > %d", width, maxCells)
	}
	return nil
}

func skipHSpace(line string, pos int) int {
	for pos < len(line) && isHSpace(line[pos]) {
		pos++
	}
	return pos
}

func isHSpace(value byte) bool {
	return value == ' ' || value == '\t'
}

func isIDStart(value byte) bool {
	return value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func isIDPart(value byte) bool {
	return isIDStart(value) || value >= '0' && value <= '9' || value == '-'
}

func sequenceError(line sourceLine, localColumn int, message string) error {
	return &ParseError{Line: line.lineNo, Column: line.baseColumn + localColumn - 1, Message: message}
}
