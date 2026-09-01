package flow

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

type symbolKind uint8

const (
	nodeSymbol symbolKind = iota
	subgraphSymbol
)

type symbol struct {
	kind  symbolKind
	index int
}

type scopeFrame struct {
	ref           ScopeRef
	openLine      int
	openColumn    int
	hasDirectNode bool
	hasChild      bool
}

func Parse(source string, limits Limits) (*Graph, error) {
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

	lines := make([]sourceLine, len(rawLines))
	for index, raw := range rawLines {
		lines[index] = normalizeSourceLine(raw, index+1)
	}
	scopeDocument := containsCompleteSubgraphHeader(lines, limits)

	graph := &Graph{}
	symbols := make(map[string]symbol)
	stack := make([]scopeFrame, 0)
	headerFound := false
	for _, current := range lines {
		line := current.text
		if line == "" || strings.HasPrefix(line, "%%") {
			continue
		}
		if !headerFound {
			direction, err := parseHeader(line, current.lineNo, current.baseColumn)
			if err != nil {
				return nil, err
			}
			graph.Direction = direction
			headerFound = true
			continue
		}

		if line == "end" {
			if len(stack) == 0 {
				if scopeDocument {
					return nil, parseErrorAt(current.lineNo, current.baseColumn, 1, "대응하는 subgraph가 없는 `end`")
				}
			} else {
				frame := stack[len(stack)-1]
				if !frame.hasDirectNode && !frame.hasChild {
					return nil, parseErrorAt(current.lineNo, current.baseColumn, 1, "빈 subgraph는 허용하지 않음")
				}
				stack = stack[:len(stack)-1]
				if len(stack) > 0 {
					stack[len(stack)-1].hasChild = true
				}
				continue
			}
		}

		subgraph, candidate, err := parseSubgraphHeader(line, current.lineNo, current.baseColumn, limits)
		if candidate {
			if err != nil {
				return nil, err
			}
			if len(graph.Subgraphs) >= limits.MaxSubgraphs {
				return nil, parseErrorAt(current.lineNo, current.baseColumn, 1, "subgraph 수 제한 초과")
			}
			if len(graph.Subgraphs) >= int(^ScopeRef(0)) {
				return nil, parseErrorAt(current.lineNo, current.baseColumn, 1, "subgraph 참조 범위 초과")
			}
			if len(stack) >= limits.MaxSubgraphDepth {
				return nil, parseErrorAt(current.lineNo, current.baseColumn, 1, "subgraph 중첩 깊이 제한 초과")
			}
			if _, exists := symbols[subgraph.ID]; exists {
				return nil, parseErrorAt(current.lineNo, current.baseColumn, subgraphIDColumn(line), fmt.Sprintf("ID %s가 이미 사용됨", subgraph.ID))
			}
			parent := RootScope
			if len(stack) > 0 {
				parent = stack[len(stack)-1].ref
			}
			subgraph.Parent = parent
			ref := ScopeRef(len(graph.Subgraphs) + 1)
			graph.Subgraphs = append(graph.Subgraphs, subgraph)
			symbols[subgraph.ID] = symbol{kind: subgraphSymbol, index: int(ref)}
			stack = append(stack, scopeFrame{
				ref:        ref,
				openLine:   current.lineNo,
				openColumn: current.baseColumn,
			})
			continue
		}

		scope := RootScope
		if len(stack) > 0 {
			scope = stack[len(stack)-1].ref
		}
		parser := statementParser{
			line:       line,
			lineNo:     current.lineNo,
			baseColumn: current.baseColumn,
			limits:     limits,
		}
		statement, err := parser.parse()
		if err != nil {
			return nil, err
		}
		updatedSymbols, ownsDirectNode, err := commitStatement(graph, symbols, statement, scope, limits)
		if err != nil {
			return nil, err
		}
		symbols = updatedSymbols
		if ownsDirectNode && len(stack) > 0 {
			stack[len(stack)-1].hasDirectNode = true
		}
	}
	if !headerFound {
		return nil, &ParseError{Line: 1, Column: 1, Message: "flowchart 헤더가 없음"}
	}
	if len(stack) > 0 {
		frame := stack[len(stack)-1]
		return nil, &ParseError{Line: frame.openLine, Column: frame.openColumn, Message: "subgraph를 닫는 `end`가 없음"}
	}
	if len(graph.Nodes) == 0 {
		return nil, &ParseError{Line: 1, Message: "노드가 없음"}
	}
	return graph, nil
}

func normalizeSourceLine(raw string, lineNo int) sourceLine {
	leftTrimmed := strings.TrimLeftFunc(raw, unicode.IsSpace)
	baseColumn := len(raw) - len(leftTrimmed) + 1
	line := strings.TrimSpace(raw)
	if strings.HasSuffix(line, ";") {
		line = strings.TrimSpace(strings.TrimSuffix(line, ";"))
	}
	return sourceLine{text: line, lineNo: lineNo, baseColumn: baseColumn}
}

func containsCompleteSubgraphHeader(lines []sourceLine, limits Limits) bool {
	for _, line := range lines {
		if line.text == "" || strings.HasPrefix(line.text, "%%") {
			continue
		}
		_, candidate, err := parseSubgraphHeader(line.text, line.lineNo, line.baseColumn, limits)
		if candidate && err == nil {
			return true
		}
	}
	return false
}

func parseHeader(line string, lineNo, baseColumn int) (Direction, error) {
	fields := strings.Fields(line)
	if len(fields) != 2 || (fields[0] != "flowchart" && fields[0] != "graph") {
		return 0, parseErrorAt(lineNo, baseColumn, 1, "`flowchart LR`, `flowchart TD` 또는 `flowchart TB` 헤더가 필요함")
	}
	switch fields[1] {
	case "LR":
		return LeftToRight, nil
	case "TD", "TB":
		return TopToBottom, nil
	default:
		return 0, parseErrorAt(lineNo, baseColumn, 1, "지원 방향은 LR, TD, TB뿐임")
	}
}

func parseSubgraphHeader(line string, lineNo, baseColumn int, limits Limits) (Subgraph, bool, error) {
	const keyword = "subgraph"
	if !strings.HasPrefix(line, keyword) || len(line) == len(keyword) {
		return Subgraph{}, false, nil
	}
	if line[len(keyword)] != ' ' && line[len(keyword)] != '\t' {
		return Subgraph{}, false, nil
	}
	pos := len(keyword)
	for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
		pos++
	}
	if pos >= len(line) || !isIDStart(rune(line[pos])) {
		return Subgraph{}, false, nil
	}
	candidate := true
	idStart := pos
	pos++
	for pos < len(line) && isIDPart(rune(line[pos])) {
		if strings.HasPrefix(line[pos:], "-->") || strings.HasPrefix(line[pos:], "-.->") {
			break
		}
		pos++
		if pos-idStart > limits.MaxIDBytes {
			return Subgraph{}, candidate, parseErrorAt(lineNo, baseColumn, idStart+1, "subgraph ID 길이 제한 초과")
		}
	}
	id := line[idStart:pos]
	result := Subgraph{ID: id, Label: id}
	for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
		pos++
	}
	if pos < len(line) && line[pos] == '[' {
		label, next, err := parseLabel(line, pos+1, "]", lineNo, baseColumn, limits.MaxLabelCells)
		if err != nil {
			return Subgraph{}, candidate, err
		}
		result.Label = label
		pos = next
		for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
			pos++
		}
	}
	if pos != len(line) {
		return Subgraph{}, candidate, parseErrorAt(lineNo, baseColumn, pos+1, "지원하지 않는 subgraph 문법")
	}
	return result, candidate, nil
}

func subgraphIDColumn(line string) int {
	pos := len("subgraph")
	for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
		pos++
	}
	return pos + 1
}

type parsedNode struct {
	node   Node
	line   int
	column int
}

type parsedArrow struct {
	dashed bool
	label  string
	column int
}

type parsedStatement struct {
	nodes  []parsedNode
	arrows []parsedArrow
}

type statementParser struct {
	line       string
	lineNo     int
	baseColumn int
	pos        int
	limits     Limits
}

func (p *statementParser) parse() (parsedStatement, error) {
	left, err := p.node()
	if err != nil {
		return parsedStatement{}, err
	}
	result := parsedStatement{nodes: []parsedNode{left}}
	for {
		p.spaces()
		if p.pos == len(p.line) {
			return result, nil
		}
		arrow, err := p.arrow()
		if err != nil {
			return parsedStatement{}, err
		}
		right, err := p.node()
		if err != nil {
			return parsedStatement{}, err
		}
		result.arrows = append(result.arrows, arrow)
		result.nodes = append(result.nodes, right)
	}
}

func (p *statementParser) node() (parsedNode, error) {
	p.spaces()
	start := p.pos
	if p.pos >= len(p.line) || !isIDStart(rune(p.line[p.pos])) {
		return parsedNode{}, p.errorAt(p.pos+1, "노드 ID가 필요함")
	}
	p.pos++
	for p.pos < len(p.line) && isIDPart(rune(p.line[p.pos])) {
		if strings.HasPrefix(p.line[p.pos:], "-->") || strings.HasPrefix(p.line[p.pos:], "-.->") {
			break
		}
		p.pos++
		if p.pos-start > p.limits.MaxIDBytes {
			return parsedNode{}, p.errorAt(start+1, "노드 ID 길이 제한 초과")
		}
	}
	id := p.line[start:p.pos]
	node := Node{ID: id, Label: id, Shape: Process}

	if p.pos < len(p.line) {
		switch p.line[p.pos] {
		case '[':
			node.explicit = true
			if strings.HasPrefix(p.line[p.pos:], "[(") {
				node.Shape = DataStore
				label, next, err := parseLabel(p.line, p.pos+2, ")]", p.lineNo, p.baseColumn, p.limits.MaxLabelCells)
				if err != nil {
					return parsedNode{}, err
				}
				node.Label, p.pos = label, next
			} else {
				label, next, err := parseLabel(p.line, p.pos+1, "]", p.lineNo, p.baseColumn, p.limits.MaxLabelCells)
				if err != nil {
					return parsedNode{}, err
				}
				node.Label, p.pos = label, next
			}
		case '{':
			node.explicit = true
			node.Shape = Decision
			label, next, err := parseLabel(p.line, p.pos+1, "}", p.lineNo, p.baseColumn, p.limits.MaxLabelCells)
			if err != nil {
				return parsedNode{}, err
			}
			node.Label, p.pos = label, next
		}
	}
	return parsedNode{node: node, line: p.lineNo, column: p.physicalColumn(start + 1)}, nil
}

func parseLabel(line string, start int, closing string, lineNo, baseColumn, maxCells int) (string, int, error) {
	endOffset := strings.Index(line[start:], closing)
	if endOffset < 0 {
		return "", 0, parseErrorAt(lineNo, baseColumn, start+1, "닫는 구분자가 없음")
	}
	end := start + endOffset
	label := strings.TrimSpace(line[start:end])
	startsQuoted := strings.HasPrefix(label, "\"")
	endsQuoted := strings.HasSuffix(label, "\"")
	if startsQuoted != endsQuoted {
		return "", 0, parseErrorAt(lineNo, baseColumn, start+1, "label의 따옴표가 닫히지 않음")
	}
	if startsQuoted && endsQuoted {
		if len(label) < 2 {
			return "", 0, parseErrorAt(lineNo, baseColumn, start+1, "빈 따옴표 label은 허용하지 않음")
		}
		label = label[1 : len(label)-1]
		if strings.Contains(label, "\"") {
			return "", 0, parseErrorAt(lineNo, baseColumn, start+1, "label 내부 따옴표 escape는 현재 지원하지 않음")
		}
	} else if strings.Contains(label, "\"") {
		return "", 0, parseErrorAt(lineNo, baseColumn, start+1, "label 따옴표는 양끝에 함께 있어야 함")
	}
	if label == "" {
		return "", 0, parseErrorAt(lineNo, baseColumn, start+1, "빈 label은 허용하지 않음")
	}
	if err := validateLabel(label, maxCells); err != nil {
		return "", 0, parseErrorAt(lineNo, baseColumn, start+1, err.Error())
	}
	return label, end + len(closing), nil
}

func (p *statementParser) arrow() (parsedArrow, error) {
	start := p.pos
	result := parsedArrow{column: p.physicalColumn(start + 1)}
	switch {
	case strings.HasPrefix(p.line[p.pos:], "-.->"):
		result.dashed = true
		p.pos += 4
	case strings.HasPrefix(p.line[p.pos:], "-->"):
		p.pos += 3
	default:
		return parsedArrow{}, p.errorAt(p.pos+1, "지원하지 않는 문법; `-->` 또는 `-.->`가 필요함")
	}
	p.spaces()
	if p.pos < len(p.line) && p.line[p.pos] == '|' {
		labelStart := p.pos + 1
		end := strings.IndexByte(p.line[labelStart:], '|')
		if end < 0 {
			return parsedArrow{}, p.errorAt(p.pos+1, "edge label의 닫는 `|`가 없음")
		}
		result.label = strings.TrimSpace(p.line[labelStart : labelStart+end])
		if result.label == "" {
			return parsedArrow{}, p.errorAt(labelStart+1, "빈 edge label은 허용하지 않음")
		}
		if err := validateLabel(result.label, p.limits.MaxLabelCells); err != nil {
			return parsedArrow{}, p.errorAt(labelStart+1, err.Error())
		}
		p.pos = labelStart + end + 1
	}
	return result, nil
}

func commitStatement(graph *Graph, symbols map[string]symbol, statement parsedStatement, scope ScopeRef, limits Limits) (map[string]symbol, bool, error) {
	nodes := append([]Node(nil), graph.Nodes...)
	edges := append([]Edge(nil), graph.Edges...)
	shadow := make(map[string]symbol, len(symbols)+len(statement.nodes))
	for id, current := range symbols {
		shadow[id] = current
	}
	indices := make([]int, len(statement.nodes))
	ownsDirectNode := false
	standalone := len(statement.arrows) == 0

	for occurrenceIndex, occurrence := range statement.nodes {
		currentSymbol, exists := shadow[occurrence.node.ID]
		if exists && currentSymbol.kind == subgraphSymbol {
			return nil, false, &ParseError{Line: occurrence.line, Column: occurrence.column, Message: fmt.Sprintf("ID %s는 subgraph로 이미 사용됨", occurrence.node.ID)}
		}
		if exists {
			current := nodes[currentSymbol.index]
			membershipAssertion := standalone || occurrence.node.explicit
			if membershipAssertion && current.Scope != scope {
				return nil, false, &ParseError{Line: occurrence.line, Column: occurrence.column, Message: fmt.Sprintf("노드 %s의 subgraph 소속이 충돌함", occurrence.node.ID)}
			}
			if occurrence.node.explicit {
				if current.explicit && (current.Label != occurrence.node.Label || current.Shape != occurrence.node.Shape) {
					return nil, false, &ParseError{Line: occurrence.line, Column: occurrence.column, Message: fmt.Sprintf("노드 %s의 정의가 충돌함", occurrence.node.ID)}
				}
				if !current.explicit {
					upgraded := occurrence.node
					upgraded.Scope = current.Scope
					nodes[currentSymbol.index] = upgraded
				}
			}
			indices[occurrenceIndex] = currentSymbol.index
			if current.Scope == scope && scope != RootScope {
				ownsDirectNode = true
			}
		} else {
			if len(nodes) >= limits.MaxNodes {
				return nil, false, &ParseError{Line: occurrence.line, Column: occurrence.column, Message: "노드 수 제한 초과"}
			}
			node := occurrence.node
			node.Scope = scope
			index := len(nodes)
			nodes = append(nodes, node)
			shadow[node.ID] = symbol{kind: nodeSymbol, index: index}
			indices[occurrenceIndex] = index
			if scope != RootScope {
				ownsDirectNode = true
			}
		}

		if occurrenceIndex == 0 {
			continue
		}
		arrow := statement.arrows[occurrenceIndex-1]
		if len(edges) >= limits.MaxEdges {
			return nil, false, &ParseError{Line: occurrence.line, Column: arrow.column, Message: "edge 수 제한 초과"}
		}
		edges = append(edges, Edge{
			From:   indices[occurrenceIndex-1],
			To:     indices[occurrenceIndex],
			Label:  arrow.label,
			Dashed: arrow.dashed,
		})
	}

	graph.Nodes = nodes
	graph.Edges = edges
	return shadow, ownsDirectNode, nil
}

func (p *statementParser) spaces() {
	for p.pos < len(p.line) && p.line[p.pos] == ' ' {
		p.pos++
	}
}

func (p *statementParser) physicalColumn(localColumn int) int {
	return p.baseColumn + localColumn - 1
}

func (p *statementParser) errorAt(localColumn int, message string) error {
	return parseErrorAt(p.lineNo, p.baseColumn, localColumn, message)
}

func parseErrorAt(lineNo, baseColumn, localColumn int, message string) error {
	return &ParseError{Line: lineNo, Column: baseColumn + localColumn - 1, Message: message}
}

func validateLabel(label string, maxCells int) error {
	if strings.Contains(strings.ToLower(label), "<br") {
		return fmt.Errorf("HTML line break는 지원하지 않는 label")
	}
	width, err := textcell.Width(label)
	if err != nil {
		return fmt.Errorf("제어 문자 또는 지원하지 않는 label: %w", err)
	}
	if width > maxCells {
		return fmt.Errorf("label 폭 제한 초과: %d > %d", width, maxCells)
	}
	return nil
}

func isIDStart(r rune) bool {
	return r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z'
}

func isIDPart(r rune) bool {
	return isIDStart(r) || r >= '0' && r <= '9' || r == '-'
}
