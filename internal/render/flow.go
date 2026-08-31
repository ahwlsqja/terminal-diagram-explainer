package render

import (
	"fmt"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

type Options struct {
	ASCII     bool
	MaxWidth  int
	MaxHeight int
}

func DefaultOptions() Options {
	return Options{MaxWidth: 240, MaxHeight: 200}
}

type placement struct {
	x, y, width, height int
}

func Flow(graph *flow.Graph, options Options) (string, error) {
	if graph == nil || len(graph.Nodes) == 0 {
		return "", fmt.Errorf("빈 graph")
	}
	if options.MaxWidth <= 0 || options.MaxHeight <= 0 {
		return "", fmt.Errorf("출력 제한은 양수여야 함")
	}
	ranks, maxRank, err := topologicalRanks(graph)
	if err != nil {
		return "", err
	}

	placements, err := place(graph, ranks, maxRank, options)
	if err != nil {
		return "", err
	}
	canvas, err := newCanvas(options.MaxWidth, options.MaxHeight, options.ASCII)
	if err != nil {
		return "", err
	}
	if err := drawEdges(canvas, graph, placements); err != nil {
		return "", err
	}
	for index, node := range graph.Nodes {
		if err := drawNode(canvas, node, placements[index]); err != nil {
			return "", err
		}
	}
	return canvas.String(), nil
}

func topologicalRanks(graph *flow.Graph) ([]int, int, error) {
	indegree := make([]int, len(graph.Nodes))
	children := make([][]int, len(graph.Nodes))
	for _, edge := range graph.Edges {
		indegree[edge.To]++
		children[edge.From] = append(children[edge.From], edge.To)
	}
	queue := make([]int, 0, len(graph.Nodes))
	for index, degree := range indegree {
		if degree == 0 {
			queue = append(queue, index)
		}
	}
	ranks := make([]int, len(graph.Nodes))
	processed := 0
	maxRank := 0
	for head := 0; head < len(queue); head++ {
		current := queue[head]
		processed++
		for _, child := range children[current] {
			if ranks[child] < ranks[current]+1 {
				ranks[child] = ranks[current] + 1
				if ranks[child] > maxRank {
					maxRank = ranks[child]
				}
			}
			indegree[child]--
			if indegree[child] == 0 {
				queue = append(queue, child)
			}
		}
	}
	if processed != len(graph.Nodes) {
		return nil, 0, fmt.Errorf("순환 graph는 v0.1에서 지원하지 않음")
	}
	return ranks, maxRank, nil
}

func place(graph *flow.Graph, ranks []int, maxRank int, options Options) ([]placement, error) {
	groups := make([][]int, maxRank+1)
	widths := make([]int, len(graph.Nodes))
	for index, node := range graph.Nodes {
		label := displayLabel(node)
		width, err := textcell.Width(label)
		if err != nil {
			return nil, err
		}
		widths[index] = max(7, width+4)
		groups[ranks[index]] = append(groups[ranks[index]], index)
	}
	placements := make([]placement, len(graph.Nodes))
	if graph.Direction == flow.LeftToRight {
		return placeLR(graph, groups, widths, placements, options)
	}
	return placeTD(graph, groups, widths, placements, options)
}

func placeLR(graph *flow.Graph, groups [][]int, widths []int, placements []placement, options Options) ([]placement, error) {
	columnWidths := make([]int, len(groups))
	columnGaps := make([]int, len(groups))
	maxCount := 0
	for rank, group := range groups {
		if len(group) > maxCount {
			maxCount = len(group)
		}
		for _, node := range group {
			columnWidths[rank] = max(columnWidths[rank], widths[node])
		}
		columnGaps[rank] = 10
	}
	for _, edge := range graph.Edges {
		labelWidth, _ := textcell.Width(edge.Label)
		columnGaps[rankOf(groups, edge.From)] = max(columnGaps[rankOf(groups, edge.From)], labelWidth+7)
	}
	x := 0
	for rank, group := range groups {
		offsetY := (maxCount - len(group)) * 3
		for row, node := range group {
			placements[node] = placement{x: x, y: offsetY + row*6, width: widths[node], height: 3}
		}
		x += columnWidths[rank] + columnGaps[rank]
	}
	if x-columnGaps[len(columnGaps)-1] > options.MaxWidth {
		return nil, fmt.Errorf("출력 폭 제한 초과: 필요 %d, 제한 %d", x-columnGaps[len(columnGaps)-1], options.MaxWidth)
	}
	neededHeight := maxCount*6 - 3
	if neededHeight > options.MaxHeight {
		return nil, fmt.Errorf("출력 높이 제한 초과: 필요 %d, 제한 %d", neededHeight, options.MaxHeight)
	}
	return placements, nil
}

func placeTD(graph *flow.Graph, groups [][]int, widths []int, placements []placement, options Options) ([]placement, error) {
	rowWidths := make([]int, len(groups))
	maxRowWidth := 0
	for rank, group := range groups {
		for index, node := range group {
			if index > 0 {
				rowWidths[rank] += 6
			}
			rowWidths[rank] += widths[node]
		}
		maxRowWidth = max(maxRowWidth, rowWidths[rank])
	}
	if maxRowWidth > options.MaxWidth {
		return nil, fmt.Errorf("출력 폭 제한 초과: 필요 %d, 제한 %d", maxRowWidth, options.MaxWidth)
	}
	for rank, group := range groups {
		x := (maxRowWidth - rowWidths[rank]) / 2
		for _, node := range group {
			placements[node] = placement{x: x, y: rank * 8, width: widths[node], height: 3}
			x += widths[node] + 6
		}
	}
	neededHeight := len(groups)*8 - 5
	if neededHeight > options.MaxHeight {
		return nil, fmt.Errorf("출력 높이 제한 초과: 필요 %d, 제한 %d", neededHeight, options.MaxHeight)
	}
	return placements, nil
}

func rankOf(groups [][]int, node int) int {
	for rank, group := range groups {
		for _, current := range group {
			if current == node {
				return rank
			}
		}
	}
	return 0
}

func drawEdges(canvas *canvas, graph *flow.Graph, placements []placement) error {
	type edgeLabel struct {
		x, y int
		text string
	}
	labels := make([]edgeLabel, 0, len(graph.Edges))
	for _, edge := range graph.Edges {
		from := placements[edge.From]
		to := placements[edge.To]
		if graph.Direction == flow.LeftToRight {
			startX, startY := from.x+from.width, from.y+1
			endX, endY := to.x-1, to.y+1
			if endX <= startX+2 {
				return fmt.Errorf("edge routing 공간 부족")
			}
			if startY == endY {
				if err := canvas.horizontal(startX, endX, startY, edge.Dashed); err != nil {
					return err
				}
				if err := canvas.arrow(endX, endY, right); err != nil {
					return err
				}
				if edge.Label != "" {
					labels = append(labels, edgeLabel{x: startX + 2, y: max(0, endY-1), text: edge.Label})
				}
				continue
			}
			laneX := startX + 2
			if err := canvas.horizontal(startX, laneX, startY, edge.Dashed); err != nil {
				return err
			}
			if err := canvas.vertical(laneX, startY, endY, edge.Dashed); err != nil {
				return err
			}
			if err := canvas.horizontal(laneX, endX, endY, edge.Dashed); err != nil {
				return err
			}
			if err := canvas.arrow(endX, endY, right); err != nil {
				return err
			}
			if edge.Label != "" {
				labels = append(labels, edgeLabel{x: laneX + 2, y: max(0, endY-1), text: edge.Label})
			}
		} else {
			startX, startY := from.x+from.width/2, from.y+from.height
			endX, endY := to.x+to.width/2, to.y-1
			if endY <= startY+2 {
				return fmt.Errorf("edge routing 공간 부족")
			}
			if startX == endX {
				if err := canvas.vertical(startX, startY, endY, edge.Dashed); err != nil {
					return err
				}
				if err := canvas.arrow(endX, endY, down); err != nil {
					return err
				}
				if edge.Label != "" {
					labels = append(labels, edgeLabel{x: endX + 2, y: startY + 1, text: edge.Label})
				}
				continue
			}
			laneY := startY + 2
			if err := canvas.vertical(startX, startY, laneY, edge.Dashed); err != nil {
				return err
			}
			if err := canvas.horizontal(startX, endX, laneY, edge.Dashed); err != nil {
				return err
			}
			if err := canvas.vertical(endX, laneY, endY, edge.Dashed); err != nil {
				return err
			}
			if err := canvas.arrow(endX, endY, down); err != nil {
				return err
			}
			if edge.Label != "" {
				labels = append(labels, edgeLabel{x: endX + 1, y: max(0, laneY-1), text: edge.Label})
			}
		}
	}
	for _, label := range labels {
		if err := canvas.putText(label.x, label.y, label.text); err != nil {
			return err
		}
	}
	return nil
}

func drawNode(canvas *canvas, node flow.Node, place placement) error {
	label := displayLabel(node)
	if canvas.ascii {
		return drawASCIIBox(canvas, label, place)
	}
	topLeft, topRight, bottomLeft, bottomRight, horizontal, vertical := "┌", "┐", "└", "┘", "─", "│"
	if node.Shape == flow.Decision {
		topLeft, topRight, bottomLeft, bottomRight = "╭", "╮", "╰", "╯"
	}
	if node.Shape == flow.DataStore {
		topLeft, topRight, bottomLeft, bottomRight, horizontal, vertical = "╔", "╗", "╚", "╝", "═", "║"
	}
	if err := canvas.put(place.x, place.y, topLeft); err != nil {
		return err
	}
	if err := canvas.put(place.x+place.width-1, place.y, topRight); err != nil {
		return err
	}
	if err := canvas.put(place.x, place.y+2, bottomLeft); err != nil {
		return err
	}
	if err := canvas.put(place.x+place.width-1, place.y+2, bottomRight); err != nil {
		return err
	}
	for x := place.x + 1; x < place.x+place.width-1; x++ {
		if err := canvas.put(x, place.y, horizontal); err != nil {
			return err
		}
		if err := canvas.put(x, place.y+2, horizontal); err != nil {
			return err
		}
	}
	if err := canvas.put(place.x, place.y+1, vertical); err != nil {
		return err
	}
	if err := canvas.put(place.x+place.width-1, place.y+1, vertical); err != nil {
		return err
	}
	labelWidth, _ := textcell.Width(label)
	return canvas.putText(place.x+(place.width-labelWidth)/2, place.y+1, label)
}

func drawASCIIBox(canvas *canvas, label string, place placement) error {
	for _, point := range [][2]int{{place.x, place.y}, {place.x + place.width - 1, place.y}, {place.x, place.y + 2}, {place.x + place.width - 1, place.y + 2}} {
		if err := canvas.put(point[0], point[1], "+"); err != nil {
			return err
		}
	}
	for x := place.x + 1; x < place.x+place.width-1; x++ {
		if err := canvas.put(x, place.y, "-"); err != nil {
			return err
		}
		if err := canvas.put(x, place.y+2, "-"); err != nil {
			return err
		}
	}
	if err := canvas.put(place.x, place.y+1, "|"); err != nil {
		return err
	}
	if err := canvas.put(place.x+place.width-1, place.y+1, "|"); err != nil {
		return err
	}
	labelWidth, _ := textcell.Width(label)
	return canvas.putText(place.x+(place.width-labelWidth)/2, place.y+1, label)
}

func displayLabel(node flow.Node) string {
	if node.Shape == flow.Decision {
		return "? " + node.Label
	}
	return node.Label
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
