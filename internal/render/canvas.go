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
	cells         [][]cell
	width, height int
	usedX, usedY  int
	ascii         bool
}

func newCanvas(width, height int, ascii bool) (*canvas, error) {
	if width <= 0 || height <= 0 || width > 512 || height > 512 {
		return nil, fmt.Errorf("canvas 제한 오류: %dx%d", width, height)
	}
	cells := make([][]cell, height)
	for y := range cells {
		cells[y] = make([]cell, width)
	}
	return &canvas{cells: cells, width: width, height: height, usedX: -1, usedY: -1, ascii: ascii}, nil
}

func (c *canvas) put(x, y int, value string) error {
	if x < 0 || y < 0 || x >= c.width || y >= c.height {
		return fmt.Errorf("출력 경계 초과: (%d,%d), limit=%dx%d", x, y, c.width, c.height)
	}
	c.cells[y][x] = cell{text: value}
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
			c.cells[y][lastBaseX].text += string(r)
			continue
		}
		if err := c.put(x, y, string(r)); err != nil {
			return err
		}
		lastBaseX = x
		for offset := 1; offset < width; offset++ {
			if x+offset >= c.width {
				return fmt.Errorf("출력 폭 제한 초과")
			}
			c.cells[y][x+offset] = cell{continuation: true}
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
		return fmt.Errorf("출력 경계 초과: (%d,%d)", x, y)
	}
	want := c.glyph(horizontal, dashed)
	existing := c.cells[y][x].text
	if existing == "" || existing == " " {
		c.cells[y][x] = cell{text: want}
	} else if existing != want && isLine(existing) {
		existingKind := lineKind(existing)
		wantKind := lineKind(want)
		switch {
		case existingKind == wantKind && existingKind == 1:
			c.cells[y][x] = cell{text: c.glyph(true, false)}
		case existingKind == wantKind && existingKind == 2:
			c.cells[y][x] = cell{text: c.glyph(false, false)}
		case existingKind == 3:
			// Keep an existing junction.
		case c.ascii:
			c.cells[y][x] = cell{text: "+"}
		default:
			c.cells[y][x] = cell{text: "┼"}
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
	value := map[flowDirection]string{right: "▶", down: "▼"}[direction]
	if c.ascii {
		value = map[flowDirection]string{right: ">", down: "v"}[direction]
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
	for y := 0; y <= c.usedY; y++ {
		var row strings.Builder
		for x := 0; x <= c.usedX; x++ {
			current := c.cells[y][x]
			if current.continuation {
				continue
			}
			if current.text == "" {
				row.WriteByte(' ')
			} else {
				row.WriteString(current.text)
			}
		}
		output.WriteString(strings.TrimRight(row.String(), " "))
		if y < c.usedY {
			output.WriteByte('\n')
		}
	}
	return strings.TrimRight(output.String(), "\n")
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
)
