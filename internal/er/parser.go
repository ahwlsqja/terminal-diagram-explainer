package er

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

type pendingRelationship struct {
	fromID, toID string
	fromColumn   int
	toColumn     int
	line         int
	fromMarker   Cardinality
	toMarker     Cardinality
	label        string
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
	lines := strings.Split(source, "\n")
	if strings.HasSuffix(source, "\n") && len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > limits.MaxLines {
		return nil, &ParseError{Line: limits.MaxLines + 1, Message: "행 수 제한 초과"}
	}

	diagram := &Diagram{}
	entityIndices := make(map[string]int)
	entityLabels := make(map[string]struct{})
	pending := make([]pendingRelationship, 0)
	headerFound := false
	currentEntity := -1
	currentOpenLine, currentOpenColumn := 0, 0
	totalAttributes := 0
	for index, raw := range lines {
		line := normalizeLine(raw, index+1)
		if line.text == "" || strings.HasPrefix(line.text, "%%") {
			continue
		}
		if !headerFound {
			if line.text != "erDiagram" {
				return nil, parseError(line, 1, "`erDiagram` 헤더가 필요함")
			}
			headerFound = true
			continue
		}

		if currentEntity >= 0 {
			if line.text == "}" {
				currentEntity = -1
				continue
			}
			attribute, err := parseAttribute(line, limits)
			if err != nil {
				return nil, err
			}
			if err := appendAttribute(diagram, currentEntity, attribute, limits, &totalAttributes, line); err != nil {
				return nil, err
			}
			continue
		}

		if line.text == "}" {
			return nil, parseError(line, 1, "열린 entity block이 없는 `}`")
		}
		if isRelationshipCandidate(line.text) || hasRelationshipIntent(line.text) {
			if len(pending) >= limits.MaxRelationships {
				return nil, parseError(line, 1, "relationship 수 제한 초과")
			}
			relation, err := parseRelationship(line, limits)
			if err != nil {
				return nil, err
			}
			pending = append(pending, relation)
			continue
		}
		if entityOpenIndex(line.text) >= 0 {
			entity, inline, closed, err := parseEntity(line, limits)
			if err != nil {
				return nil, err
			}
			if len(diagram.Entities) >= limits.MaxEntities {
				return nil, parseError(line, 1, "entity 수 제한 초과")
			}
			if _, exists := entityIndices[entity.ID]; exists {
				return nil, parseError(line, 1, fmt.Sprintf("entity %s가 중복됨", entity.ID))
			}
			if _, exists := entityLabels[entity.Label]; exists {
				return nil, parseError(line, 1, fmt.Sprintf("entity label %q가 중복됨", entity.Label))
			}
			entityIndex := len(diagram.Entities)
			entityIndices[entity.ID] = entityIndex
			entityLabels[entity.Label] = struct{}{}
			diagram.Entities = append(diagram.Entities, entity)
			if inline != nil {
				if err := appendAttribute(diagram, entityIndex, *inline, limits, &totalAttributes, line); err != nil {
					return nil, err
				}
			}
			if !closed {
				currentEntity = entityIndex
				currentOpenLine, currentOpenColumn = line.lineNo, line.baseColumn
			}
			continue
		}

		return nil, parseError(line, 1, "top-level에는 entity block 또는 relationship이 필요함")
	}
	if !headerFound {
		return nil, &ParseError{Line: 1, Column: 1, Message: "`erDiagram` 헤더가 필요함"}
	}
	if currentEntity >= 0 {
		return nil, &ParseError{Line: currentOpenLine, Column: currentOpenColumn, Message: "entity block을 닫는 `}`가 없음"}
	}
	if len(diagram.Entities) == 0 {
		return nil, &ParseError{Line: 1, Column: 1, Message: "entity가 없음"}
	}
	for _, relation := range pending {
		from, exists := entityIndices[relation.fromID]
		if !exists {
			return nil, &ParseError{Line: relation.line, Column: relation.fromColumn, Message: fmt.Sprintf("선언되지 않은 entity %s", relation.fromID)}
		}
		to, exists := entityIndices[relation.toID]
		if !exists {
			return nil, &ParseError{Line: relation.line, Column: relation.toColumn, Message: fmt.Sprintf("선언되지 않은 entity %s", relation.toID)}
		}
		diagram.Relationships = append(diagram.Relationships, Relationship{
			From: from, To: to,
			FromMarker: relation.fromMarker, ToMarker: relation.toMarker,
			Label: relation.label,
		})
	}
	return diagram, nil
}

func validateLimits(limits Limits) error {
	if limits.MaxSourceBytes <= 0 || limits.MaxLines <= 0 || limits.MaxEntities <= 0 || limits.MaxRelationships <= 0 || limits.MaxAttributes <= 0 || limits.MaxAttributesPerEntity <= 0 || limits.MaxIDBytes <= 0 || limits.MaxLabelCells <= 0 {
		return &ParseError{Line: 1, Column: 1, Message: "모든 parser 제한은 양수여야 함"}
	}
	return nil
}

func normalizeLine(raw string, lineNo int) sourceLine {
	trimmedLeft := strings.TrimLeftFunc(raw, unicode.IsSpace)
	return sourceLine{text: strings.TrimSpace(raw), lineNo: lineNo, baseColumn: len(raw) - len(trimmedLeft) + 1}
}

func parseEntity(line sourceLine, limits Limits) (Entity, *Attribute, bool, error) {
	open := entityOpenIndex(line.text)
	if open < 0 {
		return Entity{}, nil, false, parseError(line, 1, "entity block opener가 필요함")
	}
	header := strings.TrimSpace(line.text[:open])
	id, label, err := parseEntityHeader(header, limits)
	if err != nil {
		return Entity{}, nil, false, parseError(line, 1, err.Error())
	}
	rest := strings.TrimSpace(line.text[open+1:])
	entity := Entity{ID: id, Label: label}
	if rest == "" {
		return entity, nil, false, nil
	}
	if !strings.HasSuffix(rest, "}") {
		return Entity{}, nil, false, parseError(line, open+2, "inline entity block을 닫는 `}`가 없음")
	}
	content := strings.TrimSpace(strings.TrimSuffix(rest, "}"))
	if content == "" {
		return entity, nil, true, nil
	}
	attribute, attrErr := parseAttribute(sourceLine{text: content, lineNo: line.lineNo, baseColumn: line.baseColumn + open + 1}, limits)
	if attrErr != nil {
		return Entity{}, nil, false, attrErr
	}
	return entity, &attribute, true, nil
}

func entityOpenIndex(line string) int {
	inLabel := false
	for index := 0; index < len(line); index++ {
		switch line[index] {
		case '[':
			inLabel = true
		case ']':
			inLabel = false
		case '{':
			if !inLabel {
				return index
			}
		}
	}
	return -1
}

func parseEntityHeader(header string, limits Limits) (string, string, error) {
	bracket := strings.IndexByte(header, '[')
	idText := strings.TrimSpace(header)
	label := ""
	if bracket >= 0 {
		if !strings.HasSuffix(header, "]") {
			return "", "", fmt.Errorf("entity label의 `]`가 없음")
		}
		idText = header[:bracket]
		label = strings.TrimSpace(header[bracket+1 : len(header)-1])
		if label == "" {
			return "", "", fmt.Errorf("빈 entity label은 허용하지 않음")
		}
		if strings.ContainsAny(label, "[]") {
			return "", "", fmt.Errorf("entity label 안의 bracket은 지원하지 않음")
		}
	}
	id := strings.TrimSpace(idText)
	if err := validateID(id, limits.MaxIDBytes); err != nil {
		return "", "", err
	}
	if label == "" {
		label = id
	}
	if err := validateText(label, limits.MaxLabelCells); err != nil {
		return "", "", err
	}
	return id, label, nil
}

func isRelationshipCandidate(line string) bool {
	colon := strings.IndexByte(line, ':')
	if colon < 0 {
		return false
	}
	head := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, line[:colon])
	for _, left := range []string{"o|--", "||--", "}o--", "}|--"} {
		marker := strings.Index(head, left)
		if marker <= 0 {
			continue
		}
		rest := head[marker+len(left):]
		for _, right := range []string{"|o", "||", "o{", "|{"} {
			if strings.HasPrefix(rest, right) {
				return true
			}
		}
	}
	return false
}

func hasRelationshipIntent(line string) bool {
	inLabel := false
	seenDash := false
	for index := 0; index < len(line); index++ {
		switch line[index] {
		case '[':
			inLabel = true
		case ']':
			inLabel = false
		case '-':
			if !inLabel && index+1 < len(line) && line[index+1] == '-' {
				seenDash = true
			}
		case ':':
			if !inLabel && seenDash {
				return true
			}
		}
	}
	return false
}

func parseAttribute(line sourceLine, limits Limits) (Attribute, error) {
	fields := strings.Fields(line.text)
	if len(fields) < 2 || len(fields) > 4 {
		return Attribute{}, parseError(line, 1, "attribute는 `type name [PK] [FK]` 형식이어야 함")
	}
	if err := validateID(fields[0], limits.MaxIDBytes); err != nil {
		return Attribute{}, parseError(line, 1, "attribute type이 유효하지 않음")
	}
	if err := validateID(fields[1], limits.MaxIDBytes); err != nil {
		return Attribute{}, parseError(line, strings.Index(line.text, fields[1])+1, "attribute name이 유효하지 않음")
	}
	attribute := Attribute{Type: fields[0], Name: fields[1]}
	for _, marker := range fields[2:] {
		var key Key
		switch marker {
		case "PK":
			key = PrimaryKey
		case "FK":
			key = ForeignKey
		default:
			return Attribute{}, parseError(line, strings.Index(line.text, marker)+1, "지원 key marker는 PK와 FK뿐임")
		}
		if attribute.Key&key != 0 {
			return Attribute{}, parseError(line, strings.Index(line.text, marker)+1, "key marker가 중복됨")
		}
		attribute.Key |= key
	}
	if err := validateText(renderedAttribute(attribute), limits.MaxLabelCells); err != nil {
		return Attribute{}, parseError(line, 1, err.Error())
	}
	return attribute, nil
}

func appendAttribute(diagram *Diagram, entityIndex int, attribute Attribute, limits Limits, total *int, line sourceLine) error {
	entity := &diagram.Entities[entityIndex]
	if len(entity.Attributes) >= limits.MaxAttributesPerEntity || *total >= limits.MaxAttributes {
		return parseError(line, 1, "attribute 수 제한 초과")
	}
	for _, existing := range entity.Attributes {
		if existing.Name == attribute.Name {
			return parseError(line, 1, fmt.Sprintf("attribute %s가 중복됨", attribute.Name))
		}
	}
	entity.Attributes = append(entity.Attributes, attribute)
	*total++
	return nil
}

func parseRelationship(line sourceLine, limits Limits) (pendingRelationship, error) {
	colon := strings.IndexByte(line.text, ':')
	if colon < 0 {
		return pendingRelationship{}, parseError(line, 1, "relationship label 앞에 `:`가 필요함")
	}
	head := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, line.text[:colon])
	label := strings.TrimSpace(line.text[colon+1:])
	if label == "" {
		return pendingRelationship{}, parseError(line, colon+1, "빈 relationship label은 허용하지 않음")
	}
	if err := validateText(label, limits.MaxLabelCells); err != nil {
		return pendingRelationship{}, parseError(line, colon+2, err.Error())
	}
	leftTokens := []struct {
		token string
		card  Cardinality
	}{{"o|", ZeroOrOne}, {"||", ExactlyOne}, {"}o", ZeroOrMany}, {"}|", OneOrMany}}
	rightTokens := []struct {
		token string
		card  Cardinality
	}{{"|o", ZeroOrOne}, {"||", ExactlyOne}, {"o{", ZeroOrMany}, {"|{", OneOrMany}}
	for _, left := range leftTokens {
		needle := left.token + "--"
		marker := strings.Index(head, needle)
		if marker <= 0 {
			continue
		}
		fromID := head[:marker]
		if err := validateID(fromID, limits.MaxIDBytes); err != nil {
			continue
		}
		rest := head[marker+len(needle):]
		for _, right := range rightTokens {
			if !strings.HasPrefix(rest, right.token) {
				continue
			}
			toID := rest[len(right.token):]
			if err := validateID(toID, limits.MaxIDBytes); err != nil {
				return pendingRelationship{}, parseError(line, 1, "relationship target ID가 유효하지 않음")
			}
			fromOffset := strings.Index(line.text, fromID)
			toOffset := strings.LastIndex(line.text[:colon], toID)
			return pendingRelationship{
				fromID: fromID, toID: toID,
				fromColumn: line.baseColumn + fromOffset,
				toColumn:   line.baseColumn + toOffset,
				line:       line.lineNo, fromMarker: left.card, toMarker: right.card, label: label,
			}, nil
		}
	}
	return pendingRelationship{}, parseError(line, 1, "지원하지 않는 relationship cardinality 문법")
}

func validateID(id string, maxBytes int) error {
	if id == "" || len(id) > maxBytes || !isIDStart(id[0]) {
		return fmt.Errorf("ID가 유효하지 않음")
	}
	for index := 1; index < len(id); index++ {
		if !isIDPart(id[index]) {
			return fmt.Errorf("ID가 유효하지 않음")
		}
	}
	return nil
}

func validateText(text string, maxCells int) error {
	width, err := textcell.Width(text)
	if err != nil {
		return fmt.Errorf("제어 문자 또는 지원하지 않는 text: %w", err)
	}
	if width == 0 || width > maxCells {
		return fmt.Errorf("text 폭 제한 초과")
	}
	return nil
}

func renderedAttribute(attribute Attribute) string {
	prefix := ""
	if attribute.Key&PrimaryKey != 0 {
		prefix += "PK "
	}
	if attribute.Key&ForeignKey != 0 {
		prefix += "FK "
	}
	return prefix + attribute.Type + " " + attribute.Name
}

func isIDStart(value byte) bool {
	return value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func isIDPart(value byte) bool {
	return isIDStart(value) || value >= '0' && value <= '9' || value == '-'
}

func parseError(line sourceLine, localColumn int, message string) error {
	return &ParseError{Line: line.lineNo, Column: line.baseColumn + localColumn - 1, Message: message}
}
