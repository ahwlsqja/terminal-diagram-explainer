package flow

import (
	"fmt"
	"strings"
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

func Parse(source string, limits Limits) (*Graph, error) {
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

	graph := &Graph{}
	indices := make(map[string]int)
	headerFound := false
	for index, raw := range lines {
		lineNo := index + 1
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "%%") {
			continue
		}
		if !headerFound {
			direction, err := parseHeader(line, lineNo)
			if err != nil {
				return nil, err
			}
			graph.Direction = direction
			headerFound = true
			continue
		}

		line = strings.TrimSpace(strings.TrimSuffix(line, ";"))
		parser := statementParser{
			line:    line,
			lineNo:  lineNo,
			graph:   graph,
			indices: indices,
			limits:  limits,
		}
		if err := parser.parse(); err != nil {
			return nil, err
		}
	}
	if !headerFound {
		return nil, &ParseError{Line: 1, Column: 1, Message: "flowchart 헤더가 없음"}
	}
	if len(graph.Nodes) == 0 {
		return nil, &ParseError{Line: 1, Message: "노드가 없음"}
	}
	return graph, nil
}

func parseHeader(line string, lineNo int) (Direction, error) {
	fields := strings.Fields(strings.TrimSuffix(line, ";"))
	if len(fields) != 2 || (fields[0] != "flowchart" && fields[0] != "graph") {
		return 0, &ParseError{Line: lineNo, Column: 1, Message: "`flowchart LR`, `flowchart TD` 또는 `flowchart TB` 헤더가 필요함"}
	}
	switch fields[1] {
	case "LR":
		return LeftToRight, nil
	case "TD", "TB":
		return TopToBottom, nil
	default:
		return 0, &ParseError{Line: lineNo, Column: 1, Message: "지원 방향은 LR, TD, TB뿐임"}
	}
}

type statementParser struct {
	line    string
	lineNo  int
	pos     int
	graph   *Graph
	indices map[string]int
	limits  Limits
}

func (p *statementParser) parse() error {
	left, err := p.node()
	if err != nil {
		return err
	}
	leftIndex, err := p.remember(left)
	if err != nil {
		return err
	}

	for {
		p.spaces()
		if p.pos == len(p.line) {
			return nil
		}
		dashed, label, err := p.arrow()
		if err != nil {
			return err
		}
		right, err := p.node()
		if err != nil {
			return err
		}
		rightIndex, err := p.remember(right)
		if err != nil {
			return err
		}
		if len(p.graph.Edges) >= p.limits.MaxEdges {
			return p.errorAt(p.pos+1, "edge 수 제한 초과")
		}
		p.graph.Edges = append(p.graph.Edges, Edge{From: leftIndex, To: rightIndex, Label: label, Dashed: dashed})
		leftIndex = rightIndex
	}
}

func (p *statementParser) node() (Node, error) {
	p.spaces()
	start := p.pos
	if p.pos >= len(p.line) || !isIDStart(rune(p.line[p.pos])) {
		return Node{}, p.errorAt(p.pos+1, "노드 ID가 필요함")
	}
	p.pos++
	for p.pos < len(p.line) && isIDPart(rune(p.line[p.pos])) {
		if strings.HasPrefix(p.line[p.pos:], "-->") || strings.HasPrefix(p.line[p.pos:], "-.->") {
			break
		}
		p.pos++
	}
	id := p.line[start:p.pos]
	if len(id) > p.limits.MaxIDBytes {
		return Node{}, p.errorAt(start+1, "노드 ID 길이 제한 초과")
	}
	node := Node{ID: id, Label: id, Shape: Process}

	if p.pos >= len(p.line) {
		return node, nil
	}
	switch p.line[p.pos] {
	case '[':
		node.explicit = true
		if strings.HasPrefix(p.line[p.pos:], "[(") {
			node.Shape = DataStore
			label, next, err := p.label(p.pos+2, ")]")
			if err != nil {
				return Node{}, err
			}
			node.Label, p.pos = label, next
		} else {
			label, next, err := p.label(p.pos+1, "]")
			if err != nil {
				return Node{}, err
			}
			node.Label, p.pos = label, next
		}
	case '{':
		node.explicit = true
		node.Shape = Decision
		label, next, err := p.label(p.pos+1, "}")
		if err != nil {
			return Node{}, err
		}
		node.Label, p.pos = label, next
	}
	return node, nil
}

func (p *statementParser) label(start int, closing string) (string, int, error) {
	endOffset := strings.Index(p.line[start:], closing)
	if endOffset < 0 {
		return "", 0, p.errorAt(start+1, "닫는 구분자가 없음")
	}
	end := start + endOffset
	label := strings.TrimSpace(p.line[start:end])
	startsQuoted := strings.HasPrefix(label, "\"")
	endsQuoted := strings.HasSuffix(label, "\"")
	if startsQuoted != endsQuoted {
		return "", 0, p.errorAt(start+1, "label의 따옴표가 닫히지 않음")
	}
	if startsQuoted && endsQuoted {
		if len(label) < 2 {
			return "", 0, p.errorAt(start+1, "빈 따옴표 label은 허용하지 않음")
		}
		label = label[1 : len(label)-1]
		if strings.Contains(label, "\"") {
			return "", 0, p.errorAt(start+1, "label 내부 따옴표 escape는 v0.1에서 지원하지 않음")
		}
	} else if strings.Contains(label, "\"") {
		return "", 0, p.errorAt(start+1, "label 따옴표는 양끝에 함께 있어야 함")
	}
	if label == "" {
		return "", 0, p.errorAt(start+1, "빈 label은 허용하지 않음")
	}
	if err := validateLabel(label, p.limits.MaxLabelCells); err != nil {
		return "", 0, p.errorAt(start+1, err.Error())
	}
	return label, end + len(closing), nil
}

func (p *statementParser) arrow() (bool, string, error) {
	dashed := false
	switch {
	case strings.HasPrefix(p.line[p.pos:], "-.->"):
		dashed = true
		p.pos += 4
	case strings.HasPrefix(p.line[p.pos:], "-->"):
		p.pos += 3
	default:
		return false, "", p.errorAt(p.pos+1, "지원하지 않는 문법; `-->` 또는 `-.->`가 필요함")
	}
	p.spaces()
	label := ""
	if p.pos < len(p.line) && p.line[p.pos] == '|' {
		start := p.pos + 1
		end := strings.IndexByte(p.line[start:], '|')
		if end < 0 {
			return false, "", p.errorAt(p.pos+1, "edge label의 닫는 `|`가 없음")
		}
		label = strings.TrimSpace(p.line[start : start+end])
		if label == "" {
			return false, "", p.errorAt(start+1, "빈 edge label은 허용하지 않음")
		}
		if err := validateLabel(label, p.limits.MaxLabelCells); err != nil {
			return false, "", p.errorAt(start+1, err.Error())
		}
		p.pos = start + end + 1
	}
	return dashed, label, nil
}

func (p *statementParser) remember(node Node) (int, error) {
	if existing, ok := p.indices[node.ID]; ok {
		current := p.graph.Nodes[existing]
		if node.explicit {
			if current.explicit && (current.Label != node.Label || current.Shape != node.Shape) {
				return 0, p.errorAt(p.pos+1, fmt.Sprintf("노드 %s의 정의가 충돌함", node.ID))
			}
			if !current.explicit {
				p.graph.Nodes[existing] = node
			}
		}
		return existing, nil
	}
	if len(p.graph.Nodes) >= p.limits.MaxNodes {
		return 0, p.errorAt(p.pos+1, "노드 수 제한 초과")
	}
	index := len(p.graph.Nodes)
	p.indices[node.ID] = index
	p.graph.Nodes = append(p.graph.Nodes, node)
	return index, nil
}

func (p *statementParser) spaces() {
	for p.pos < len(p.line) && p.line[p.pos] == ' ' {
		p.pos++
	}
}

func (p *statementParser) errorAt(column int, message string) error {
	return &ParseError{Line: p.lineNo, Column: column, Message: message}
}

func validateLabel(label string, maxCells int) error {
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
