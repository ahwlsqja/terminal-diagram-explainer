package render

import (
	"fmt"
	"strings"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

type cell struct {
	text         string
	continuation bool
}

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
		if err := c.line(x, y, true, dashed); err != nil {
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
		if err := c.line(x, y, false, dashed); err != nil {
			return err
		}
	}
	return nil
}

func (c *canvas) line(x, y int, horizontal, dashed bool) error {
	if x < 0 || y < 0 || x >= c.width || y >= c.height {
		return fmt.Errorf("%w: line point (%d,%d)", ErrOutputBounds, x, y)
	}
	want := c.glyph(horizontal, dashed)
	cellIndex := c.index(x, y)
	existing := c.cells[cellIndex].text
	if existing == "" || existing == " " {
		c.cells[cellIndex] = cell{text: want}
	} else if existing != want && isLine(existing) {
		existingKind := lineKind(existing)
		wantKind := lineKind(want)
		switch {
		case existingKind == wantKind && existingKind == 1:
			c.cells[cellIndex] = cell{text: c.glyph(true, false)}
		case existingKind == wantKind && existingKind == 2:
			c.cells[cellIndex] = cell{text: c.glyph(false, false)}
		case existingKind == 3:
			// Keep an existing junction.
		case c.ascii:
			c.cells[cellIndex] = cell{text: "+"}
		default:
			c.cells[cellIndex] = cell{text: "┼"}
		}
	}
	c.mark(x, y)
	return nil
}

func (c *canvas) glyph(horizontal, dashed bool) string {
	if c.ascii {
		if horizontal {
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
	if horizontal {
		if dashed {
			return "┄"
		}
		return "─"
	}
	if dashed {
		return "┊"
	}
	return "│"
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
	return strings.Contains("-|.:─│┄┊┼+", value)
}

// lineKind returns 1 for horizontal, 2 for vertical and 3 for a junction.
func lineKind(value string) int {
	switch value {
	case "-", ".", "─", "┄":
		return 1
	case "|", ":", "│", "┊":
		return 2
	case "+", "┼":
		return 3
	default:
		return 0
	}
}

type flowDirection uint8

const (
	right flowDirection = iota
	down
	left
	up
)
