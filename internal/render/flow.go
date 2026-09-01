package render

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

type Options struct {
	ASCII     bool
	MaxWidth  int
	MaxHeight int
	AutoFit   bool
}

func DefaultOptions() Options {
	return Options{MaxWidth: 240, MaxHeight: 200}
}

type placement struct {
	x, y, width, height int
}

const tdNodeGap = 3

func Flow(graph *flow.Graph, options Options) (string, error) {
	output, err := flowInRequestedDirection(graph, options)
	if err == nil || !options.AutoFit || !errors.Is(err, ErrOutputBounds) || graph == nil {
		return output, err
	}
	alternate := *graph
	if alternate.Direction == flow.LeftToRight {
		alternate.Direction = flow.TopToBottom
	} else {
		alternate.Direction = flow.LeftToRight
	}
	options.AutoFit = false
	alternateOutput, alternateErr := flowInRequestedDirection(&alternate, options)
	if alternateErr == nil {
		return alternateOutput, nil
	}
	return "", fmt.Errorf("%w: 요청 방향=%v; 대체 방향=%v", ErrOutputBounds, err, alternateErr)
}

func flowInRequestedDirection(graph *flow.Graph, options Options) (string, error) {
	if options.MaxWidth <= 0 || options.MaxHeight <= 0 {
		return "", fmt.Errorf("%w: limits must be positive", ErrOutputBounds)
	}
	plan, err := analyzeRanksWithBudget(graph, maxGraphWorkSteps)
	if err != nil {
		return "", err
	}

	outer := outerEdgeMask(graph, plan)
	if len(graph.Subgraphs) > 0 {
		return flowScoped(graph, plan, outer, options)
	}
	placements, err := place(graph, plan.ranks, plan.maxRank, outer, options)
	if err != nil {
		return "", err
	}
	if hasOuterRoutes(outer) {
		if graph.Direction == flow.LeftToRight {
			shiftPlacements(placements, 2, 0)
		} else {
			shiftPlacements(placements, 0, 2)
		}
	}
	outerRoutes, err := planOuterRoutes(graph, plan, outer, placements, options)
	if err != nil {
		return "", err
	}
	canvas, err := newCanvas(options.MaxWidth, options.MaxHeight, options.ASCII)
	if err != nil {
		return "", err
	}
	if err := drawForwardEdges(canvas, graph, placements, plan.ranks, outer); err != nil {
		return "", err
	}
	if err := drawOuterRoutes(canvas, outerRoutes); err != nil {
		return "", err
	}
	for index, node := range graph.Nodes {
		if err := drawNode(canvas, node, placements[index]); err != nil {
			return "", err
		}
	}
	return appendOuterLegends(strings.TrimLeft(canvas.String(), "\n"), graph, plan.feedback, outer, options)
}

func place(graph *flow.Graph, ranks []int, maxRank int, feedback []bool, options Options) ([]placement, error) {
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
	minimizeForwardCrossings(graph, groups, ranks, feedback)
	placements := make([]placement, len(graph.Nodes))
	if graph.Direction == flow.LeftToRight {
		return placeLR(graph, groups, widths, placements, ranks, feedback, options)
	}
	return placeTD(graph, groups, widths, placements, ranks, feedback, options)
}

func placeLR(graph *flow.Graph, groups [][]int, widths []int, placements []placement, ranks []int, feedback []bool, options Options) ([]placement, error) {
	columnWidths := make([]int, len(groups))
	columnGaps := make([]int, len(groups))
	forwardCounts := forwardEdgeCountsByRank(graph, ranks, feedback)
	maxCount := 0
	for rank, group := range groups {
		if len(group) > maxCount {
			maxCount = len(group)
		}
		for _, node := range group {
			columnWidths[rank] = max(columnWidths[rank], widths[node])
		}
		columnGaps[rank] = max(10, forwardCounts[rank]*2+4)
	}
	for edgeIndex, edge := range graph.Edges {
		if feedback[edgeIndex] {
			continue
		}
		labelWidth, _ := textcell.Width(edge.Label)
		columnGaps[ranks[edge.From]] = max(columnGaps[ranks[edge.From]], labelWidth+7)
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
		return nil, fmt.Errorf("%w: 출력 폭 제한 초과: 필요 %d, 제한 %d", ErrOutputBounds, x-columnGaps[len(columnGaps)-1], options.MaxWidth)
	}
	neededHeight := maxCount*6 - 3
	if neededHeight > options.MaxHeight {
		return nil, fmt.Errorf("%w: 출력 높이 제한 초과: 필요 %d, 제한 %d", ErrOutputBounds, neededHeight, options.MaxHeight)
	}
	return placements, nil
}

func placeTD(graph *flow.Graph, groups [][]int, widths []int, placements []placement, ranks []int, feedback []bool, options Options) ([]placement, error) {
	rowWidths := make([]int, len(groups))
	maxRowWidth := 0
	for rank, group := range groups {
		for index, node := range group {
			if index > 0 {
				rowWidths[rank] += tdNodeGap
			}
			rowWidths[rank] += widths[node]
		}
		maxRowWidth = max(maxRowWidth, rowWidths[rank])
	}
	if maxRowWidth > options.MaxWidth {
		return nil, fmt.Errorf("%w: 출력 폭 제한 초과: 필요 %d, 제한 %d", ErrOutputBounds, maxRowWidth, options.MaxWidth)
	}
	for rank, group := range groups {
		x := centeredTDRowLeft(graph, placements, ranks, feedback, rank, rowWidths[rank], maxRowWidth)
		for _, node := range group {
			placements[node] = placement{x: x, width: widths[node], height: 3}
			x += widths[node] + tdNodeGap
		}
	}
	doglegCounts := forwardDoglegCountsByRank(graph, placements, ranks, feedback)
	outerCounts := outerForwardEdgeCountsByRank(graph, ranks, feedback)
	for rank := range doglegCounts {
		doglegCounts[rank] += outerCounts[rank]
	}
	y := 0
	for rank, group := range groups {
		for _, node := range group {
			placements[node].y = y
		}
		if rank+1 < len(groups) {
			y += 3 + max(2, doglegCounts[rank]+2)
		}
	}
	neededHeight := y + 3
	if neededHeight > options.MaxHeight {
		return nil, fmt.Errorf("%w: 출력 높이 제한 초과: 필요 %d, 제한 %d", ErrOutputBounds, neededHeight, options.MaxHeight)
	}
	return placements, nil
}

func centeredTDRowLeft(graph *flow.Graph, placements []placement, ranks []int, outer []bool, rank, rowWidth, maxRowWidth int) int {
	centered := (maxRowWidth - rowWidth) / 2
	if rank == 0 {
		return centered
	}
	centerSum := 0
	centerCount := 0
	for edgeIndex, edge := range graph.Edges {
		if outer[edgeIndex] || ranks[edge.From] != rank-1 || ranks[edge.To] != rank {
			continue
		}
		from := placements[edge.From]
		centerSum += from.x + from.width/2
		centerCount++
	}
	if centerCount == 0 {
		return centered
	}
	desired := centerSum/centerCount - rowWidth/2
	if desired < 0 {
		return 0
	}
	if maximum := maxRowWidth - rowWidth; desired > maximum {
		return maximum
	}
	return desired
}

func drawForwardEdges(canvas *canvas, graph *flow.Graph, placements []placement, ranks []int, feedback []bool) error {
	routes, err := planForwardRoutes(graph, placements, ranks, feedback)
	if err != nil {
		return err
	}
	if err := validateForwardRouteTopology(graph, routes); err != nil {
		return err
	}
	for _, route := range routes {
		for _, segment := range route.segments {
			if segment.y1 == segment.y2 {
				if err := canvas.horizontal(segment.x1, segment.x2, segment.y1, segment.dashed); err != nil {
					return err
				}
			} else if err := canvas.vertical(segment.x1, segment.y1, segment.y2, segment.dashed); err != nil {
				return err
			}
		}
		if err := canvas.arrow(route.arrowX, route.arrowY, route.direction); err != nil {
			return err
		}
	}
	for _, route := range routes {
		label := graph.Edges[route.edgeIndex].Label
		if label == "" {
			continue
		}
		if err := canvas.putText(route.labelX, route.labelY, label); err != nil {
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
