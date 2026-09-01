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

type pendingTableConstraint struct {
	entity            int
	kind              TableConstraintKind
	columns           []string
	columnSpans       []int
	referenceID       string
	referenceCols     []string
	referenceSpans    []int
	referenceIDColumn int
	line              sourceLine
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
	if err := validateSourceCharacters(source); err != nil {
		return nil, err
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
	pendingConstraints := make([]pendingTableConstraint, 0)
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
			if isTableConstraintCandidate(line.text) {
				if len(pendingConstraints) >= limits.MaxTableConstraints || len(diagram.Entities[currentEntity].TableConstraints)+pendingConstraintCount(pendingConstraints, currentEntity) >= limits.MaxTableConstraintsPerEntity {
					return nil, parseError(line, 1, "table constraint 수 제한 초과")
				}
				constraint, err := parseTableConstraint(line, currentEntity, limits)
				if err != nil {
					return nil, err
				}
				pendingConstraints = append(pendingConstraints, constraint)
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
	for _, pendingConstraint := range pendingConstraints {
		entity := &diagram.Entities[pendingConstraint.entity]
		indices := make([]int, len(pendingConstraint.columns))
		for index, name := range pendingConstraint.columns {
			attributeIndex := findAttribute(entity.Attributes, name)
			if attributeIndex < 0 {
				return nil, parseError(pendingConstraint.line, pendingConstraint.columnSpans[index], fmt.Sprintf("선언되지 않은 attribute %s", name))
			}
			indices[index] = attributeIndex
		}
		constraint := TableConstraint{Kind: pendingConstraint.kind, Columns: indices}
		if pendingConstraint.kind == CompositePrimaryKey {
			if hasTablePrimaryKey(entity.TableConstraints) {
				return nil, parseError(pendingConstraint.line, 1, "table PRIMARY KEY가 중복됨")
			}
			for _, attribute := range entity.Attributes {
				if attribute.Key&PrimaryKey != 0 {
					return nil, parseError(pendingConstraint.line, 1, "attribute PK와 table PRIMARY KEY를 함께 사용할 수 없음")
				}
			}
		}
		if pendingConstraint.kind == CompositeForeignKey {
			target := entityIndices[pendingConstraint.referenceID]
			if _, exists := entityIndices[pendingConstraint.referenceID]; !exists {
				return nil, parseError(pendingConstraint.line, pendingConstraint.referenceIDColumn, fmt.Sprintf("선언되지 않은 entity %s", pendingConstraint.referenceID))
			}
			targetEntity := diagram.Entities[target]
			referenceIndices := make([]int, len(pendingConstraint.referenceCols))
			for index, name := range pendingConstraint.referenceCols {
				attributeIndex := findAttribute(targetEntity.Attributes, name)
				if attributeIndex < 0 {
					return nil, parseError(pendingConstraint.line, pendingConstraint.referenceSpans[index], fmt.Sprintf("선언되지 않은 attribute %s", name))
				}
				referenceIndices[index] = attributeIndex
			}
			if len(indices) != len(referenceIndices) {
				return nil, parseError(pendingConstraint.line, pendingConstraint.referenceIDColumn, "FOREIGN KEY column 수가 REFERENCES와 다름")
			}
			constraint.Reference = &ForeignReference{Entity: target, Columns: referenceIndices}
		}
		entity.TableConstraints = append(entity.TableConstraints, constraint)
	}
	return diagram, nil
}

func validateLimits(limits Limits) error {
	if limits.MaxSourceBytes <= 0 || limits.MaxLines <= 0 || limits.MaxEntities <= 0 || limits.MaxRelationships <= 0 || limits.MaxAttributes <= 0 || limits.MaxAttributesPerEntity <= 0 || limits.MaxTableConstraints <= 0 || limits.MaxTableConstraintsPerEntity <= 0 || limits.MaxTableConstraintColumns <= 0 || limits.MaxTableConstraintCells <= 0 || limits.MaxIDBytes <= 0 || limits.MaxLabelCells <= 0 {
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
	tokens := attributeTokens(line.text)
	if len(tokens) < 2 {
		return Attribute{}, parseError(line, 1, "attribute는 `type name [PK] [FK] [UNIQUE] [NOT NULL]` 형식이어야 함")
	}
	if len(tokens) > 7 {
		return Attribute{}, parseError(line, tokens[7].column, "attribute marker 수 제한 초과")
	}
	if err := validateID(tokens[0].text, limits.MaxIDBytes); err != nil {
		return Attribute{}, parseError(line, 1, "attribute type이 유효하지 않음")
	}
	if err := validateID(tokens[1].text, limits.MaxIDBytes); err != nil {
		return Attribute{}, parseError(line, tokens[1].column, "attribute name이 유효하지 않음")
	}
	attribute := Attribute{Type: tokens[0].text, Name: tokens[1].text}
	for index := 2; index < len(tokens); {
		token := tokens[index]
		switch token.text {
		case "PK":
			if attribute.Key&PrimaryKey != 0 {
				return Attribute{}, parseError(line, token.column, "PK marker가 중복됨")
			}
			attribute.Key |= PrimaryKey
		case "FK":
			if attribute.Key&ForeignKey != 0 {
				return Attribute{}, parseError(line, token.column, "FK marker가 중복됨")
			}
			attribute.Key |= ForeignKey
		case "UNIQUE":
			if attribute.Constraint&Unique != 0 {
				return Attribute{}, parseError(line, token.column, "UNIQUE marker가 중복됨")
			}
			attribute.Constraint |= Unique
		case "NOT":
			if index+1 >= len(tokens) || tokens[index+1].text != "NULL" {
				column := token.column
				if index+1 < len(tokens) {
					column = tokens[index+1].column
				}
				return Attribute{}, parseError(line, column, "NOT 뒤에는 인접한 NULL marker가 필요함")
			}
			if attribute.Constraint&NotNull != 0 {
				return Attribute{}, parseError(line, token.column, "NOT NULL marker가 중복됨")
			}
			attribute.Constraint |= NotNull
			index += 2
			continue
		default:
			return Attribute{}, parseError(line, token.column, "지원하지 않는 attribute marker")
		}
		index++
	}
	if err := validateText(FormatAttribute(attribute), limits.MaxLabelCells); err != nil {
		return Attribute{}, parseError(line, 1, err.Error())
	}
	return attribute, nil
}

type attributeToken struct {
	text   string
	column int
}

func attributeTokens(text string) []attributeToken {
	tokens := make([]attributeToken, 0, 7)
	for offset := 0; offset < len(text); {
		for offset < len(text) {
			r, size := utf8.DecodeRuneInString(text[offset:])
			if !unicode.IsSpace(r) {
				break
			}
			offset += size
		}
		start := offset
		for offset < len(text) {
			r, size := utf8.DecodeRuneInString(text[offset:])
			if unicode.IsSpace(r) {
				break
			}
			offset += size
		}
		if start < offset {
			tokens = append(tokens, attributeToken{text: text[start:offset], column: start + 1})
		}
	}
	return tokens
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

func pendingConstraintCount(constraints []pendingTableConstraint, entity int) int {
	count := 0
	for _, constraint := range constraints {
		if constraint.entity == entity {
			count++
		}
	}
	return count
}

func findAttribute(attributes []Attribute, name string) int {
	for index, attribute := range attributes {
		if attribute.Name == name {
			return index
		}
	}
	return -1
}

func hasTablePrimaryKey(constraints []TableConstraint) bool {
	for _, constraint := range constraints {
		if constraint.Kind == CompositePrimaryKey {
			return true
		}
	}
	return false
}

func isTableConstraintCandidate(text string) bool {
	return hasLeadingTableKeyword(text, "PRIMARY") || hasLeadingTableKeyword(text, "UNIQUE") || hasLeadingTableKeyword(text, "FOREIGN")
}

func hasLeadingTableKeyword(text, keyword string) bool {
	if !strings.HasPrefix(text, keyword) {
		return false
	}
	if len(text) == len(keyword) {
		return true
	}
	next := text[len(keyword)]
	return next == ' ' || next == '\t' || next == '('
}

// parseTableConstraint는 SQL의 작은, 의도적으로 제한된 복합 제약 부분집합만 받는다.
func parseTableConstraint(line sourceLine, entity int, limits Limits) (pendingTableConstraint, error) {
	p := tableConstraintParser{text: line.text}
	constraint := pendingTableConstraint{entity: entity, line: line}
	if p.keyword("PRIMARY") {
		if !p.requiredSpace() || !p.keyword("KEY") {
			return constraint, parseError(line, p.column(), "PRIMARY KEY 문법이 필요함")
		}
		constraint.kind = CompositePrimaryKey
		columns, spans, err := p.columns(limits)
		if err != nil {
			return constraint, parseError(line, p.column(), err.Error())
		}
		constraint.columns, constraint.columnSpans = columns, spans
	} else if p.keyword("UNIQUE") {
		constraint.kind = CompositeUnique
		columns, spans, err := p.columns(limits)
		if err != nil {
			return constraint, parseError(line, p.column(), err.Error())
		}
		constraint.columns, constraint.columnSpans = columns, spans
	} else if p.keyword("FOREIGN") {
		if !p.requiredSpace() || !p.keyword("KEY") {
			return constraint, parseError(line, p.column(), "FOREIGN KEY 문법이 필요함")
		}
		constraint.kind = CompositeForeignKey
		columns, spans, err := p.columns(limits)
		if err != nil {
			return constraint, parseError(line, p.column(), err.Error())
		}
		constraint.columns, constraint.columnSpans = columns, spans
		if !p.requiredSpace() || !p.keyword("REFERENCES") || !p.requiredSpace() {
			return constraint, parseError(line, p.column(), "REFERENCES 문법이 필요함")
		}
		id, idColumn := p.identifier(limits)
		if id == "" {
			return constraint, parseError(line, p.column(), "REFERENCES entity ID가 필요함")
		}
		constraint.referenceID, constraint.referenceIDColumn = id, idColumn
		references, referenceSpans, err := p.columns(limits)
		if err != nil {
			return constraint, parseError(line, p.column(), err.Error())
		}
		constraint.referenceCols, constraint.referenceSpans = references, referenceSpans
	} else {
		return constraint, parseError(line, 1, "지원하지 않는 table constraint")
	}
	p.space()
	if !p.done() {
		return constraint, parseError(line, p.column(), "table constraint 뒤의 text는 허용하지 않음")
	}
	if constraint.kind == CompositeForeignKey && len(constraint.columns) != len(constraint.referenceCols) {
		return constraint, parseError(line, constraint.referenceIDColumn, "FOREIGN KEY column 수가 REFERENCES와 다름")
	}
	formatted := formatPendingConstraint(constraint)
	if err := validateText(formatted, limits.MaxTableConstraintCells); err != nil {
		return constraint, parseError(line, 1, err.Error())
	}
	return constraint, nil
}

func formatPendingConstraint(constraint pendingTableConstraint) string {
	columns := strings.Join(constraint.columns, ", ")
	switch constraint.kind {
	case CompositePrimaryKey:
		return "PRIMARY KEY (" + columns + ")"
	case CompositeUnique:
		return "UNIQUE (" + columns + ")"
	default:
		return "FOREIGN KEY (" + columns + ") REFERENCES " + constraint.referenceID + "(" + strings.Join(constraint.referenceCols, ", ") + ")"
	}
}

type tableConstraintParser struct {
	text   string
	offset int
}

func (p *tableConstraintParser) done() bool {
	return p.offset == len(p.text)
}

func (p *tableConstraintParser) column() int {
	return p.offset + 1
}

func (p *tableConstraintParser) space() {
	for p.offset < len(p.text) && (p.text[p.offset] == ' ' || p.text[p.offset] == '\t') {
		p.offset++
	}
}

func (p *tableConstraintParser) requiredSpace() bool {
	start := p.offset
	p.space()
	return p.offset > start
}

func (p *tableConstraintParser) keyword(keyword string) bool {
	if !strings.HasPrefix(p.text[p.offset:], keyword) {
		return false
	}
	p.offset += len(keyword)
	return true
}

func (p *tableConstraintParser) identifier(limits Limits) (string, int) {
	p.space()
	start := p.offset
	if start >= len(p.text) || !isIDStart(p.text[start]) {
		return "", start + 1
	}
	p.offset++
	for p.offset < len(p.text) && isIDPart(p.text[p.offset]) {
		p.offset++
	}
	id := p.text[start:p.offset]
	if validateID(id, limits.MaxIDBytes) != nil {
		return "", start + 1
	}
	return id, start + 1
}

func (p *tableConstraintParser) columns(limits Limits) ([]string, []int, error) {
	p.space()
	if p.offset >= len(p.text) || p.text[p.offset] != '(' {
		return nil, nil, fmt.Errorf("column list가 필요함")
	}
	p.offset++
	columns := make([]string, 0, min(limits.MaxTableConstraintColumns, 8))
	spans := make([]int, 0, min(limits.MaxTableConstraintColumns, 8))
	for {
		id, span := p.identifier(limits)
		if id == "" {
			return nil, nil, fmt.Errorf("column ID가 필요함")
		}
		for _, existing := range columns {
			if existing == id {
				p.offset = span - 1
				return nil, nil, fmt.Errorf("column %s가 중복됨", id)
			}
		}
		columns = append(columns, id)
		spans = append(spans, span)
		if len(columns) > limits.MaxTableConstraintColumns {
			return nil, nil, fmt.Errorf("constraint column 수 제한 초과")
		}
		p.space()
		if p.offset >= len(p.text) {
			return nil, nil, fmt.Errorf("column list를 닫는 `)`가 필요함")
		}
		if p.text[p.offset] == ')' {
			p.offset++
			break
		}
		if p.text[p.offset] != ',' {
			return nil, nil, fmt.Errorf("column 사이에는 `,`가 필요함")
		}
		p.offset++
	}
	if len(columns) < 2 {
		return nil, nil, fmt.Errorf("table constraint는 두 column 이상이어야 함")
	}
	return columns, spans, nil
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

func validateSourceCharacters(source string) error {
	line, column := 1, 1
	for _, r := range source {
		if r == '\n' {
			line, column = line+1, 1
			continue
		}
		if r == ' ' || r == '\t' {
			column++
			continue
		}
		if unicode.IsSpace(r) {
			return &ParseError{Line: line, Column: column, Message: "구문 공백은 ASCII space, tab, newline만 허용함"}
		}
		if _, err := textcell.RuneWidth(r); err != nil {
			return &ParseError{Line: line, Column: column, Message: "입력 전체에서 제어 문자 또는 지원하지 않는 text는 사용할 수 없음"}
		}
		column++
	}
	return nil
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
