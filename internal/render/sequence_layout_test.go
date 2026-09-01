package render

import (
	"errors"
	"strings"
	"testing"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/sequence"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

func TestSequenceTwoParticipantGolden(t *testing.T) {
	diagram := &sequence.Diagram{
		Participants: []sequence.Participant{{ID: "A", Label: "A"}, {ID: "B", Label: "B"}},
		Messages:     []sequence.Message{{From: 0, To: 1, Label: "call", Kind: sequence.Request}},
	}

	got := mustRenderSequence(t, diagram, Options{MaxWidth: 40, MaxHeight: 20})
	want := "┌─────┐   ┌─────┐\n" +
		"│  A  │   │  B  │\n" +
		"└─────┘   └─────┘\n" +
		"   ┊  call   ┊\n" +
		"   └─────────▶"
	if got != want {
		t.Fatalf("two-party request output drifted:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestSequenceReverseReturnGolden(t *testing.T) {
	diagram := &sequence.Diagram{
		Participants: []sequence.Participant{{ID: "A", Label: "A"}, {ID: "B", Label: "B"}},
		Messages:     []sequence.Message{{From: 1, To: 0, Label: "done", Kind: sequence.Return}},
	}

	got := mustRenderSequence(t, diagram, Options{MaxWidth: 40, MaxHeight: 20})
	want := "┌─────┐   ┌─────┐\n" +
		"│  A  │   │  B  │\n" +
		"└─────┘   └─────┘\n" +
		"   ┊  done   ┊\n" +
		"   ◀┄┄┄┄┄┄┄┄┄┘"
	if got != want {
		t.Fatalf("two-party return output drifted:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestSequenceASCIITwoParticipantGolden(t *testing.T) {
	diagram := &sequence.Diagram{
		Participants: []sequence.Participant{{ID: "A", Label: "A"}, {ID: "B", Label: "B"}},
		Messages: []sequence.Message{
			{From: 0, To: 1, Label: "call", Kind: sequence.Request},
			{From: 1, To: 0, Label: "done", Kind: sequence.Return},
		},
	}

	got := mustRenderSequence(t, diagram, Options{ASCII: true, MaxWidth: 40, MaxHeight: 20})
	want := "+-----+   +-----+\n" +
		"|  A  |   |  B  |\n" +
		"+-----+   +-----+\n" +
		"   :  call   :\n" +
		"   +--------->\n" +
		"   :  done   :\n" +
		"   <.........+"
	if got != want {
		t.Fatalf("ASCII output drifted:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if strings.ContainsAny(got, "┌┐└┘─│┄┊┼▶◀") {
		t.Fatalf("ASCII output contains Unicode drawing glyph:\n%s", got)
	}
}

func TestSequenceFanoutLongHopJunctionAndLabels(t *testing.T) {
	diagram := &sequence.Diagram{
		Participants: []sequence.Participant{
			{ID: "API", Label: "API"},
			{ID: "Auth", Label: "Auth"},
			{ID: "Store", Label: "Store"},
			{ID: "Queue", Label: "Queue"},
		},
		Messages: []sequence.Message{
			{From: 0, To: 1, Label: "validate", Kind: sequence.Request},
			{From: 0, To: 2, Label: "저장 e\u0301", Kind: sequence.Request},
			{From: 0, To: 3, Label: "fan-out-to-queue", Kind: sequence.Request},
		},
	}

	output := mustRenderSequence(t, diagram, Options{MaxWidth: 160, MaxHeight: 40})
	assertSequenceOutputClean(t, output, 160, 40)
	for _, label := range []string{"API", "Auth", "Store", "Queue", "validate", "저장 e\u0301", "fan-out-to-queue"} {
		assertSequenceTextOnce(t, output, label)
	}
	if strings.Count(output, "▶") != 3 {
		t.Fatalf("request arrow count=%d, want 3:\n%s", strings.Count(output, "▶"), output)
	}

	grid := newSequenceGrid(t, output)
	for _, message := range diagram.Messages {
		row := sequenceArrowRow(t, grid, message.Label)
		if row < 0 {
			t.Fatalf("message label %q has no arrow row:\n%s", message.Label, output)
		}
	}
	longHopArrowY := sequenceArrowRow(t, grid, "fan-out-to-queue")
	longHopRow := strings.Join(grid.rows[longHopArrowY], "")
	if strings.Count(longHopRow, "┼")+strings.Count(longHopRow, "┴") < 2 {
		t.Fatalf("long-hop route must retain intermediate lifeline junctions:\n%s", output)
	}
}

func TestSequenceSelfMessageUsesPrivateRightRail(t *testing.T) {
	diagram := &sequence.Diagram{
		Participants: []sequence.Participant{
			{ID: "A", Label: "A"},
			{ID: "B", Label: "B"},
			{ID: "C", Label: "C"},
		},
		Messages: []sequence.Message{
			{From: 0, To: 0, Label: "self-label-is-deliberately-long", Kind: sequence.Request},
			{From: 0, To: 2, Label: "then-fanout", Kind: sequence.Request},
		},
	}

	output := mustRenderSequence(t, diagram, Options{MaxWidth: 160, MaxHeight: 30})
	assertSequenceOutputClean(t, output, 160, 30)
	grid := newSequenceGrid(t, output)
	a := sequenceHeaderBox(t, grid, "A")
	b := sequenceHeaderBox(t, grid, "B")
	labelY := sequenceTextY(t, grid, "self-label-is-deliberately-long")
	railX := sequenceSelfRailX(t, grid, a.center(), labelY)
	if b.left <= railX+1 {
		t.Fatalf("next header intersects or touches self private rail: next=%+v rail=%d:\n%s", b, railX, output)
	}
	if grid.at(railX, labelY+2) != "│" {
		t.Fatalf("self rail at (%d,%d)=%q, want vertical solid rail:\n%s", railX, labelY+2, grid.at(railX, labelY+2), output)
	}
	if grid.at(a.center(), labelY+3) != "◀" {
		t.Fatalf("self return arrow at (%d,%d)=%q, want left arrow:\n%s", a.center(), labelY+3, grid.at(a.center(), labelY+3), output)
	}
}

func TestSequenceSelfReturnPreservesDashedLoopAndNextLifeline(t *testing.T) {
	diagram := &sequence.Diagram{
		Participants: []sequence.Participant{{ID: "A", Label: "A"}, {ID: "B", Label: "B"}},
		Messages:     []sequence.Message{{From: 0, To: 0, Label: "retry-return", Kind: sequence.Return}},
	}

	for _, test := range []struct {
		name       string
		ascii      bool
		horizontal string
		vertical   string
		arrow      string
	}{
		{name: "Unicode", horizontal: "┄", vertical: "┊", arrow: "◀"},
		{name: "ASCII", ascii: true, horizontal: ".", vertical: ":", arrow: "<"},
	} {
		t.Run(test.name, func(t *testing.T) {
			output := mustRenderSequence(t, diagram, Options{ASCII: test.ascii, MaxWidth: 100, MaxHeight: 20})
			assertSequenceOutputClean(t, output, 100, 20)
			assertSequenceTextOnce(t, output, "retry-return")

			grid := newSequenceGrid(t, output)
			a := sequenceHeaderBox(t, grid, "A")
			b := sequenceHeaderBox(t, grid, "B")
			labelY := sequenceTextY(t, grid, "retry-return")
			railX := sequenceSelfRailX(t, grid, a.center(), labelY)

			for _, y := range []int{labelY, labelY + 1, labelY + 2, labelY + 3} {
				if got := grid.at(b.center(), y); got != test.vertical {
					t.Fatalf("next lifeline at (%d,%d)=%q, want %q:\n%s", b.center(), y, got, test.vertical, output)
				}
			}
			if got := grid.at(railX, labelY+2); got != test.vertical {
				t.Fatalf("self dashed rail at (%d,%d)=%q, want %q:\n%s", railX, labelY+2, got, test.vertical, output)
			}
			if got := grid.at(a.center(), labelY+3); got != test.arrow {
				t.Fatalf("self return arrow at (%d,%d)=%q, want %q:\n%s", a.center(), labelY+3, got, test.arrow, output)
			}
			for _, y := range []int{labelY + 1, labelY + 3} {
				if !sequenceRowContainsBetween(grid, y, a.center()+1, railX, test.horizontal) {
					t.Fatalf("self return row %d has no dashed horizontal glyph %q:\n%s", y, test.horizontal, output)
				}
			}
		})
	}
}

func TestSequenceLastParticipantSelfRailIsIncludedInTightBounds(t *testing.T) {
	diagram := &sequence.Diagram{
		Participants: []sequence.Participant{{ID: "A", Label: "A"}, {ID: "B", Label: "B"}},
		Messages:     []sequence.Message{{From: 1, To: 1, Label: "last-self-rail", Kind: sequence.Request}},
	}
	output := mustRenderSequence(t, diagram, Options{MaxWidth: 100, MaxHeight: 20})
	width := sequenceOutputWidth(t, output)
	height := len(strings.Split(output, "\n"))
	if _, err := Sequence(diagram, Options{MaxWidth: width, MaxHeight: height}); err != nil {
		t.Fatalf("exact sequence bounds rejected: %v", err)
	}
	if _, err := Sequence(diagram, Options{MaxWidth: width - 1, MaxHeight: height}); !errors.Is(err, ErrOutputBounds) {
		t.Fatalf("one-cell-short sequence width error=%v, want ErrOutputBounds", err)
	}
}

func TestSequenceMaximumNormalFixtureFitsDefaultGeometry(t *testing.T) {
	participants := make([]sequence.Participant, 16)
	for index := range participants {
		participants[index] = sequence.Participant{ID: sequenceID(index), Label: sequenceID(index)}
	}
	messages := make([]sequence.Message, 96)
	for index := range messages {
		messages[index] = sequence.Message{From: index % 16, To: (index + 7) % 16, Label: "m", Kind: sequence.Request}
	}
	diagram := &sequence.Diagram{Participants: participants, Messages: messages}

	output := mustRenderSequence(t, diagram, DefaultOptions())
	assertSequenceOutputClean(t, output, 240, 200)
	if got := len(strings.Split(output, "\n")); got != 195 {
		t.Fatalf("96 normal messages output height=%d, want exactly 195:\n%s", got, output)
	}
	if count := strings.Count(output, "▶") + strings.Count(output, "◀"); count != 96 {
		t.Fatalf("normal message arrow count=%d, want 96", count)
	}
}

func TestSequenceMaximumParticipantLabelsFailClosedAtDefaultWidth(t *testing.T) {
	participants := make([]sequence.Participant, 16)
	for index := range participants {
		participants[index] = sequence.Participant{ID: sequenceID(index), Label: strings.Repeat(sequenceID(index), 48)}
	}
	diagram := &sequence.Diagram{
		Participants: participants,
		Messages:     []sequence.Message{{From: 0, To: 1, Label: "m", Kind: sequence.Request}},
	}
	if _, err := Sequence(diagram, DefaultOptions()); !errors.Is(err, ErrOutputBounds) {
		t.Fatalf("wide participant fixture error=%v, want ErrOutputBounds", err)
	}
}

func TestSequenceLongLabelUsesExactWidthPreflight(t *testing.T) {
	const label = "LLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLL"
	diagram := &sequence.Diagram{
		Participants: []sequence.Participant{{ID: "A", Label: "A"}, {ID: "B", Label: "B"}},
		Messages:     []sequence.Message{{From: 0, To: 1, Label: label, Kind: sequence.Request}},
	}
	output := mustRenderSequence(t, diagram, Options{MaxWidth: 160, MaxHeight: 20})
	width := sequenceOutputWidth(t, output)
	if _, err := Sequence(diagram, Options{MaxWidth: width, MaxHeight: 20}); err != nil {
		t.Fatalf("tight long label width rejected: %v", err)
	}
	if _, err := Sequence(diagram, Options{MaxWidth: width - 1, MaxHeight: 20}); !errors.Is(err, ErrOutputBounds) {
		t.Fatalf("one-cell-short long label error=%v, want ErrOutputBounds", err)
	}
}

func TestSequenceMixedSelfMessagesFailClosedAtHeight(t *testing.T) {
	diagram := &sequence.Diagram{Participants: []sequence.Participant{{ID: "A", Label: "A"}}}
	for index := 0; index < 50; index++ {
		diagram.Messages = append(diagram.Messages, sequence.Message{From: 0, To: 0, Label: "self", Kind: sequence.Request})
	}
	if _, err := Sequence(diagram, DefaultOptions()); !errors.Is(err, ErrOutputBounds) {
		t.Fatalf("self-message height error=%v, want ErrOutputBounds", err)
	}
}

func TestSequenceDeterministicAcrossRepeatedRenders(t *testing.T) {
	diagram := &sequence.Diagram{
		Participants: []sequence.Participant{{ID: "A", Label: "수신"}, {ID: "B", Label: "처리"}, {ID: "C", Label: "저장"}},
		Messages: []sequence.Message{
			{From: 0, To: 1, Label: "검증", Kind: sequence.Request},
			{From: 1, To: 1, Label: "retry", Kind: sequence.Request},
			{From: 1, To: 0, Label: "ok", Kind: sequence.Return},
			{From: 0, To: 2, Label: "기록", Kind: sequence.Request},
		},
	}
	want := mustRenderSequence(t, diagram, Options{MaxWidth: 100, MaxHeight: 30})
	for run := 0; run < 256; run++ {
		got, err := Sequence(diagram, Options{MaxWidth: 100, MaxHeight: 30})
		if err != nil {
			t.Fatalf("run %d render error: %v", run, err)
		}
		if got != want {
			t.Fatalf("run %d rendered non-deterministically", run)
		}
	}
}

func mustRenderSequence(t *testing.T, diagram *sequence.Diagram, options Options) string {
	t.Helper()
	output, err := Sequence(diagram, options)
	if err != nil {
		t.Fatal(err)
	}
	return output
}

func assertSequenceTextOnce(t *testing.T, output, text string) {
	t.Helper()
	if count := strings.Count(output, text); count != 1 {
		t.Fatalf("text %q count=%d, want 1:\n%s", text, count, output)
	}
}

func assertSequenceOutputClean(t *testing.T, output string, maxWidth, maxHeight int) {
	t.Helper()
	if strings.HasPrefix(output, "\n") {
		t.Fatalf("output starts with blank line:\n%q", output)
	}
	rows := strings.Split(output, "\n")
	if len(rows) > maxHeight {
		t.Fatalf("height=%d, limit=%d", len(rows), maxHeight)
	}
	for y, row := range rows {
		if strings.HasSuffix(row, " ") || strings.HasSuffix(row, "\t") {
			t.Fatalf("row %d has trailing whitespace: %q", y, row)
		}
		width, err := textcell.Width(row)
		if err != nil {
			t.Fatalf("row %d width: %v", y, err)
		}
		if width > maxWidth {
			t.Fatalf("row %d width=%d, limit=%d: %q", y, width, maxWidth, row)
		}
	}
}

func sequenceOutputWidth(t *testing.T, output string) int {
	t.Helper()
	width := 0
	for _, row := range strings.Split(output, "\n") {
		current, err := textcell.Width(row)
		if err != nil {
			t.Fatal(err)
		}
		if current > width {
			width = current
		}
	}
	return width
}

func sequenceID(index int) string {
	return string(rune('A'+index/26)) + string(rune('A'+index%26))
}

type sequenceGrid struct {
	rows [][]string
	raw  []string
}

func newSequenceGrid(t *testing.T, output string) sequenceGrid {
	t.Helper()
	grid := sequenceGrid{}
	for _, line := range strings.Split(output, "\n") {
		row := make([]string, 0)
		for _, r := range line {
			width, err := textcell.RuneWidth(r)
			if err != nil {
				t.Fatal(err)
			}
			if width == 0 {
				continue
			}
			row = append(row, string(r))
			for offset := 1; offset < width; offset++ {
				row = append(row, "")
			}
		}
		grid.rows = append(grid.rows, row)
		grid.raw = append(grid.raw, line)
	}
	return grid
}

func (g sequenceGrid) at(x, y int) string {
	if y < 0 || y >= len(g.rows) || x < 0 || x >= len(g.rows[y]) {
		return ""
	}
	return g.rows[y][x]
}

func sequenceTextY(t *testing.T, grid sequenceGrid, text string) int {
	t.Helper()
	for y, row := range grid.raw {
		if strings.Contains(row, text) {
			return y
		}
	}
	t.Fatalf("text %q not found", text)
	return -1
}

func sequenceTextX(t *testing.T, grid sequenceGrid, text string) int {
	t.Helper()
	for _, row := range grid.raw {
		byteIndex := strings.Index(row, text)
		if byteIndex < 0 {
			continue
		}
		width, err := textcell.Width(row[:byteIndex])
		if err != nil {
			t.Fatal(err)
		}
		return width
	}
	t.Fatalf("text %q not found", text)
	return -1
}

func sequenceArrowRow(t *testing.T, grid sequenceGrid, label string) int {
	t.Helper()
	labelY := sequenceTextY(t, grid, label)
	for y := labelY + 1; y < len(grid.rows); y++ {
		row := strings.Join(grid.rows[y], "")
		if strings.ContainsAny(row, "▶◀><") {
			return y
		}
		if y > labelY+2 {
			break
		}
	}
	return -1
}

type sequenceBox struct {
	left, right int
}

func (b sequenceBox) center() int { return (b.left + b.right) / 2 }

func sequenceHeaderBox(t *testing.T, grid sequenceGrid, label string) sequenceBox {
	t.Helper()
	y := sequenceTextY(t, grid, label)
	labelX := sequenceTextX(t, grid, label)
	left := labelX - 1
	for left >= 0 && grid.at(left, y) != "│" && grid.at(left, y) != "|" {
		left--
	}
	right := labelX
	for right < len(grid.rows[y]) && grid.at(right, y) != "│" && grid.at(right, y) != "|" {
		right++
	}
	if left >= 0 && right < len(grid.rows[y]) {
		return sequenceBox{left: left, right: right}
	}
	t.Fatalf("header box for %q not found", label)
	return sequenceBox{}
}

func sequenceSelfRailX(t *testing.T, grid sequenceGrid, sourceX, labelY int) int {
	t.Helper()
	for x := sourceX + 1; x < len(grid.rows[labelY+2]); x++ {
		if value := grid.at(x, labelY+2); value == "│" || value == "|" || value == "┊" || value == ":" {
			return x
		}
	}
	t.Fatalf("self rail not found after source x=%d", sourceX)
	return -1
}

func sequenceRowContainsBetween(grid sequenceGrid, y, left, right int, want string) bool {
	for x := left; x < right; x++ {
		if grid.at(x, y) == want {
			return true
		}
	}
	return false
}
