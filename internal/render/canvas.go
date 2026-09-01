package render

import (
	"fmt"
	"strings"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

type cell struct {
	text         string
	continuation bool
	connections  lineConnections
	lineDashed   bool
}

type lineConnections uint8

const (
	connectNorth lineConnections = 1 << iota
	connectEast
	connectSouth
	connectWest
)

type canvas struct {
	cells         []cell
	width, height int
	usedX, usedY  int
	ascii         bool
}

func newCanvas(width, height int, ascii bool) (*canvas, error) {
	if width <= 0 || height <= 0 || width > 512 || height > 512 {
		return nil, fmt.Errorf("%w: canvas %dx%d", ErrOutputBounds, width, height)
	}
	cells := make([]cell, width*height)
	return &canvas{cells: cells, width: width, height: height, usedX: -1, usedY: -1, ascii: ascii}, nil
}

func (c *canvas) put(x, y int, value string) error {
	if x < 0 || y < 0 || x >= c.width || y >= c.height {
		return fmt.Errorf("%w: point (%d,%d), limit=%dx%d", ErrOutputBounds, x, y, c.width, c.height)
	}
	c.cells[c.index(x, y)] = cell{text: value}
	c.mark(x, y)
	return nil
}

func (c *canvas) putText(x, y int, text string) error {
	lastBaseX := -1
	for _, r := range text {
		width, err := textcell.RuneWidth(r)
		if err != nil {
			return err
		}
		if width == 0 {
			if lastBaseX < 0 {
				return fmt.Errorf("결합 문자는 label 시작에 올 수 없음")
			}
			c.cells[c.index(lastBaseX, y)].text += string(r)
			continue
		}
		if err := c.put(x, y, string(r)); err != nil {
			return err
		}
		lastBaseX = x
		for offset := 1; offset < width; offset++ {
			if x+offset >= c.width {
				return fmt.Errorf("%w: wide rune continuation", ErrOutputBounds)
			}
			c.cells[c.index(x+offset, y)] = cell{continuation: true}
			c.mark(x+offset, y)
		}
		x += width
	}
	return nil
}

func (c *canvas) horizontal(x1, x2, y int, dashed bool) error {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	for x := x1; x <= x2; x++ {
		connections := lineConnections(0)
		if x > x1 {
			connections |= connectWest
		}
		if x < x2 {
			connections |= connectEast
		}
		if connections == 0 {
			connections = connectEast | connectWest
		}
		if err := c.connectLine(x, y, connections, dashed); err != nil {
			return err
		}
	}
	return nil
}

func (c *canvas) vertical(x, y1, y2 int, dashed bool) error {
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	for y := y1; y <= y2; y++ {
		connections := lineConnections(0)
		if y > y1 {
			connections |= connectNorth
		}
		if y < y2 {
			connections |= connectSouth
		}
		if connections == 0 {
			connections = connectNorth | connectSouth
		}
		if err := c.connectLine(x, y, connections, dashed); err != nil {
			return err
		}
	}
	return nil
}

func (c *canvas) connectLine(x, y int, connections lineConnections, dashed bool) error {
	if x < 0 || y < 0 || x >= c.width || y >= c.height {
		return fmt.Errorf("%w: line point (%d,%d)", ErrOutputBounds, x, y)
	}
	cellIndex := c.index(x, y)
	existing := c.cells[cellIndex]
	if existing.connections == 0 {
		parsedConnections, parsedDashed := glyphConnections(existing.text)
		existing.connections = parsedConnections
		existing.lineDashed = parsedDashed
	}
	if existing.text == "" || existing.text == " " || existing.connections != 0 {
		lineDashed := dashed
		if existing.connections != 0 {
			lineDashed = existing.lineDashed && dashed
		}
		connections |= existing.connections
		c.cells[cellIndex] = cell{
			text:        c.connectionGlyph(connections, lineDashed),
			connections: connections,
			lineDashed:  lineDashed,
		}
	}
	c.mark(x, y)
	return nil
}

func (c *canvas) connectionGlyph(connections lineConnections, dashed bool) string {
	if c.ascii {
		if connections&(connectNorth|connectSouth) != 0 && connections&(connectEast|connectWest) != 0 {
			return "+"
		}
		if connections&(connectEast|connectWest) != 0 {
			if dashed {
				return "."
			}
			return "-"
		}
		if dashed {
			return ":"
		}
		return "|"
	}
	if connections == connectEast || connections == connectWest || connections == connectEast|connectWest {
		if dashed {
			return "┄"
		}
		return "─"
	}
	if connections == connectNorth || connections == connectSouth || connections == connectNorth|connectSouth {
		if dashed {
			return "┊"
		}
		return "│"
	}
	switch connections {
	case connectEast | connectSouth:
		return "┌"
	case connectSouth | connectWest:
		return "┐"
	case connectNorth | connectEast:
		return "└"
	case connectNorth | connectWest:
		return "┘"
	case connectEast | connectSouth | connectWest:
		return "┬"
	case connectNorth | connectSouth | connectWest:
		return "┤"
	case connectNorth | connectEast | connectWest:
		return "┴"
	case connectNorth | connectEast | connectSouth:
		return "├"
	case connectNorth | connectEast | connectSouth | connectWest:
		return "┼"
	default:
		return ""
	}
}

func glyphConnections(value string) (lineConnections, bool) {
	switch value {
	case "-", "─":
		return connectEast | connectWest, false
	case ".", "┄":
		return connectEast | connectWest, true
	case "|", "│":
		return connectNorth | connectSouth, false
	case ":", "┊":
		return connectNorth | connectSouth, true
	case "┌":
		return connectEast | connectSouth, false
	case "┐":
		return connectSouth | connectWest, false
	case "└":
		return connectNorth | connectEast, false
	case "┘":
		return connectNorth | connectWest, false
	case "┬":
		return connectEast | connectSouth | connectWest, false
	case "┤":
		return connectNorth | connectSouth | connectWest, false
	case "┴":
		return connectNorth | connectEast | connectWest, false
	case "├":
		return connectNorth | connectEast | connectSouth, false
	case "+", "┼":
		return connectNorth | connectEast | connectSouth | connectWest, false
	default:
		return 0, false
	}
}

func (c *canvas) arrow(x, y int, direction flowDirection) error {
	value := map[flowDirection]string{right: "▶", down: "▼", left: "◀", up: "▲"}[direction]
	if c.ascii {
		value = map[flowDirection]string{right: ">", down: "v", left: "<", up: "^"}[direction]
	}
	return c.put(x, y, value)
}

func (c *canvas) mark(x, y int) {
	if x > c.usedX {
		c.usedX = x
	}
	if y > c.usedY {
		c.usedY = y
	}
}

func (c *canvas) String() string {
	if c.usedX < 0 || c.usedY < 0 {
		return ""
	}
	var output strings.Builder
	output.Grow((c.usedX + 1) * (c.usedY + 1))
	for y := 0; y <= c.usedY; y++ {
		lastX := c.usedX
		for lastX >= 0 {
			current := c.at(lastX, y)
			if current.continuation || current.text == "" {
				lastX--
				continue
			}
			break
		}
		for x := 0; x <= lastX; x++ {
			current := c.at(x, y)
			if current.continuation {
				continue
			}
			if current.text == "" {
				output.WriteByte(' ')
			} else {
				output.WriteString(current.text)
			}
		}
		if y < c.usedY {
			output.WriteByte('\n')
		}
	}
	return strings.TrimRight(output.String(), "\n")
}

func (c *canvas) index(x, y int) int {
	return y*c.width + x
}

func (c *canvas) at(x, y int) cell {
	return c.cells[c.index(x, y)]
}

func isLine(value string) bool {
	connections, _ := glyphConnections(value)
	return connections != 0
}

type flowDirection uint8

const (
	right flowDirection = iota
	down
	left
	up
)
