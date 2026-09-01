package render

import (
	"fmt"
	"html"
	"strings"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

const (
	svgCellWidth  = 10
	svgCellHeight = 20
	svgPadding    = 12
)

type svgToken struct {
	text  string
	x     int
	width int
}

type svgLineSpec struct {
	connections lineConnections
	dashed      bool
	emphasized  bool
	rounded     bool
}

// SVG converts canonical terminal output into a fixed-geometry vector image.
// Line glyphs and arrows become SVG primitives, so host font metrics and soft
// wrapping cannot disconnect the diagram.
func SVG(terminal string) (string, error) {
	terminal = strings.TrimRight(terminal, "\n")
	if terminal == "" {
		return "", fmt.Errorf("%w: empty SVG source", ErrInvalidGraph)
	}
	lines := strings.Split(terminal, "\n")
	maximumWidth := 0
	for _, line := range lines {
		width, err := textcell.Width(line)
		if err != nil {
			return "", err
		}
		maximumWidth = max(maximumWidth, width)
	}
	if maximumWidth <= 0 || len(lines) > 512 || maximumWidth > 512 {
		return "", fmt.Errorf("%w: SVG source %dx%d", ErrOutputBounds, maximumWidth, len(lines))
	}
	if maximumWidth*len(lines) > 60_000 {
		return "", fmt.Errorf("%w: SVG cell budget %d", ErrOutputBounds, maximumWidth*len(lines))
	}

	width := maximumWidth*svgCellWidth + svgPadding*2
	height := len(lines)*svgCellHeight + svgPadding*2
	var output strings.Builder
	fmt.Fprintf(&output, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img">`, width, height, width, height)
	output.WriteString(`<title>Software flow diagram</title>`)
	fmt.Fprintf(&output, `<rect width="%d" height="%d" rx="8" fill="#1e1e1e"/>`, width, height)
	output.WriteString(`<g fill="none" stroke="#eeeeee" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">`)
	for row, line := range lines {
		for _, token := range tokenizeSVGLine(line) {
			if spec, ok := svgSpec(token.text); ok {
				writeSVGLine(&output, token.x, row, spec)
				continue
			}
			if writeSVGArrow(&output, token.text, token.x, row) {
				continue
			}
		}
	}
	output.WriteString(`</g>`)
	output.WriteString(`<g fill="#f3f3f3" stroke="none" font-family="SFMono-Regular,Consolas,Liberation Mono,Noto Sans Mono CJK KR,monospace" font-size="15">`)
	for row, line := range lines {
		for _, token := range svgTextRuns(line) {
			x := svgPadding + token.x*svgCellWidth
			y := svgPadding + row*svgCellHeight + 15
			textWidth := token.width * svgCellWidth
			fmt.Fprintf(&output, `<text x="%d" y="%d" textLength="%d" lengthAdjust="spacingAndGlyphs">%s</text>`, x, y, textWidth, html.EscapeString(token.text))
		}
	}
	output.WriteString(`</g></svg>`)
	return output.String(), nil
}

func svgTextRuns(line string) []svgToken {
	tokens := tokenizeSVGLine(line)
	runs := make([]svgToken, 0, len(tokens))
	current := svgToken{}
	hasCurrent := false
	pendingSpaces := 0
	flush := func() {
		if !hasCurrent {
			return
		}
		runs = append(runs, current)
		current = svgToken{}
		hasCurrent = false
		pendingSpaces = 0
	}
	for _, token := range tokens {
		if _, ok := svgSpec(token.text); ok || isSVGArrow(token.text) {
			flush()
			continue
		}
		if token.text == " " {
			if hasCurrent {
				pendingSpaces++
			}
			continue
		}
		if !hasCurrent {
			current = svgToken{text: token.text, x: token.x, width: token.width}
			hasCurrent = true
			continue
		}
		if pendingSpaces > 1 {
			flush()
			current = svgToken{text: token.text, x: token.x, width: token.width}
			hasCurrent = true
			continue
		}
		if pendingSpaces == 1 {
			current.text += " "
			current.width++
		}
		current.text += token.text
		current.width += token.width
		pendingSpaces = 0
	}
	flush()
	return runs
}

func tokenizeSVGLine(line string) []svgToken {
	tokens := make([]svgToken, 0, len(line))
	x := 0
	for _, current := range line {
		width, err := textcell.RuneWidth(current)
		if err != nil {
			continue
		}
		if width == 0 {
			if len(tokens) > 0 {
				tokens[len(tokens)-1].text += string(current)
			}
			continue
		}
		tokens = append(tokens, svgToken{text: string(current), x: x, width: width})
		x += width
	}
	return tokens
}

func svgSpec(value string) (svgLineSpec, bool) {
	if strings.Contains("─│┄┊┌┐└┘┬┤┴├┼", value) {
		if connections, dashed := glyphConnections(value); connections != 0 {
			return svgLineSpec{connections: connections, dashed: dashed}, true
		}
	}
	spec := svgLineSpec{}
	switch value {
	case "╭":
		spec.connections, spec.rounded = connectEast|connectSouth, true
	case "╮":
		spec.connections, spec.rounded = connectSouth|connectWest, true
	case "╰":
		spec.connections, spec.rounded = connectNorth|connectEast, true
	case "╯":
		spec.connections, spec.rounded = connectNorth|connectWest, true
	case "═":
		spec.connections, spec.emphasized = connectEast|connectWest, true
	case "║":
		spec.connections, spec.emphasized = connectNorth|connectSouth, true
	case "╔":
		spec.connections, spec.emphasized = connectEast|connectSouth, true
	case "╗":
		spec.connections, spec.emphasized = connectSouth|connectWest, true
	case "╚":
		spec.connections, spec.emphasized = connectNorth|connectEast, true
	case "╝":
		spec.connections, spec.emphasized = connectNorth|connectWest, true
	case "━":
		spec.connections, spec.emphasized = connectEast|connectWest, true
	case "┃":
		spec.connections, spec.emphasized = connectNorth|connectSouth, true
	case "┏":
		spec.connections, spec.emphasized = connectEast|connectSouth, true
	case "┓":
		spec.connections, spec.emphasized = connectSouth|connectWest, true
	case "┗":
		spec.connections, spec.emphasized = connectNorth|connectEast, true
	case "┛":
		spec.connections, spec.emphasized = connectNorth|connectWest, true
	default:
		return svgLineSpec{}, false
	}
	return spec, true
}

func writeSVGLine(output *strings.Builder, cellX, row int, spec svgLineSpec) {
	centerX := svgPadding + cellX*svgCellWidth + svgCellWidth/2
	centerY := svgPadding + row*svgCellHeight + svgCellHeight/2
	style := ""
	if spec.dashed {
		style += ` stroke-dasharray="3 3"`
	}
	if spec.emphasized {
		style += ` stroke-width="3"`
	}
	if spec.rounded {
		style += ` stroke-linecap="round"`
	}
	var path strings.Builder
	path.WriteString(fmt.Sprintf("M %d %d", centerX, centerY))
	if spec.connections&connectNorth != 0 {
		path.WriteString(fmt.Sprintf(" L %d %d M %d %d", centerX, centerY-svgCellHeight/2, centerX, centerY))
	}
	if spec.connections&connectEast != 0 {
		path.WriteString(fmt.Sprintf(" L %d %d M %d %d", centerX+svgCellWidth/2, centerY, centerX, centerY))
	}
	if spec.connections&connectSouth != 0 {
		path.WriteString(fmt.Sprintf(" L %d %d M %d %d", centerX, centerY+svgCellHeight/2, centerX, centerY))
	}
	if spec.connections&connectWest != 0 {
		path.WriteString(fmt.Sprintf(" L %d %d", centerX-svgCellWidth/2, centerY))
	}
	fmt.Fprintf(output, `<path d="%s"%s/>`, path.String(), style)
}

func writeSVGArrow(output *strings.Builder, value string, cellX, row int) bool {
	if !isSVGArrow(value) {
		return false
	}
	left := svgPadding + cellX*svgCellWidth
	top := svgPadding + row*svgCellHeight
	right := left + svgCellWidth
	bottom := top + svgCellHeight
	centerX := left + svgCellWidth/2
	centerY := top + svgCellHeight/2
	points := ""
	switch value {
	case "▶":
		points = fmt.Sprintf("%d,%d %d,%d %d,%d", left+1, top+3, right-1, centerY, left+1, bottom-3)
	case "◀":
		points = fmt.Sprintf("%d,%d %d,%d %d,%d", right-1, top+3, left+1, centerY, right-1, bottom-3)
	case "▼":
		points = fmt.Sprintf("%d,%d %d,%d %d,%d", left+1, top+4, right-1, top+4, centerX, bottom-2)
	case "▲":
		points = fmt.Sprintf("%d,%d %d,%d %d,%d", left+1, bottom-4, right-1, bottom-4, centerX, top+2)
	}
	fmt.Fprintf(output, `<polygon points="%s" fill="#eeeeee" stroke="none"/>`, points)
	return true
}

func isSVGArrow(value string) bool {
	return value == "▶" || value == "◀" || value == "▼" || value == "▲"
}
