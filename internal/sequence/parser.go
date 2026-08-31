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
	headerFound := false
	messagesStarted := false
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
			if messagesStarted {
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
			continue
		}

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
		if len(diagram.Messages) >= limits.MaxMessages {
			return nil, &ParseError{Line: line.lineNo, Column: arrowColumn, Message: "message 수 제한 초과"}
		}
		message.From = from
		message.To = to
		diagram.Messages = append(diagram.Messages, message)
		messagesStarted = true
	}

	if !headerFound {
		return nil, &ParseError{Line: 1, Column: 1, Message: "`sequenceDiagram` 헤더가 필요함"}
	}
	if len(diagram.Participants) == 0 {
		return nil, &ParseError{Line: 1, Column: 1, Message: "participant가 없음"}
	}
	if len(diagram.Messages) == 0 {
		return nil, &ParseError{Line: 1, Column: 1, Message: "message가 없음"}
	}
	return diagram, nil
}

func validateLimits(limits Limits) error {
	if limits.MaxSourceBytes <= 0 || limits.MaxLines <= 0 || limits.MaxParticipants <= 0 ||
		limits.MaxMessages <= 0 || limits.MaxIDBytes <= 0 || limits.MaxLabelCells <= 0 {
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
