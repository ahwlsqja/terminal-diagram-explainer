package render

import (
	"fmt"
	"strings"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

type routeSegment struct {
	x1, y1 int
	x2, y2 int
	dashed bool
}

type feedbackRoute struct {
	edgeIndex int
	feedback  bool
	segments  []routeSegment
	arrowX    int
	arrowY    int
	direction flowDirection
}

func hasOuterRoutes(outer []bool) bool {
	for _, value := range outer {
		if value {
			return true
		}
	}
	return false
}

func outerEdgeMask(graph *flow.Graph, plan rankPlan) []bool {
	outer := make([]bool, len(graph.Edges))
	mixedJunction := mixedForwardJunctionMask(graph, plan)
	for edgeIndex, edge := range graph.Edges {
		outer[edgeIndex] = plan.feedback[edgeIndex] ||
			plan.ranks[edge.To] > plan.ranks[edge.From]+1 ||
			graph.Nodes[edge.From].Scope != graph.Nodes[edge.To].Scope ||
			mixedJunction[edgeIndex]
	}
	return outer
}

func mixedForwardJunctionMask(graph *flow.Graph, plan rankPlan) []bool {
	indegree := make([]int, len(graph.Nodes))
	outdegree := make([]int, len(graph.Nodes))
	pairCount := make(map[[2]int]int, len(graph.Edges))
	for edgeIndex, edge := range graph.Edges {
		if plan.feedback[edgeIndex] {
			continue
		}
		outdegree[edge.From]++
		indegree[edge.To]++
		pairCount[[2]int{edge.From, edge.To}]++
	}
	mixed := make([]bool, len(graph.Edges))
	for edgeIndex, edge := range graph.Edges {
		mixed[edgeIndex] = !plan.feedback[edgeIndex] &&
			pairCount[[2]int{edge.From, edge.To}] == 1 &&
			outdegree[edge.From] > 1 &&
			indegree[edge.To] > 1
	}
	return mixed
}

func shiftPlacements(placements []placement, dx, dy int) {
	for index := range placements {
		placements[index].x += dx
		placements[index].y += dy
	}
}

func planOuterRoutes(graph *flow.Graph, plan rankPlan, outer []bool, placements []placement, options Options) ([]feedbackRoute, error) {
	outerCount := 0
	for _, value := range outer {
		if value {
			outerCount++
		}
	}
	if outerCount == 0 {
		return nil, nil
	}

	rankCount := plan.maxRank + 1
	rankLeft := make([]int, rankCount)
	rankRight := make([]int, rankCount)
	rankTop := make([]int, rankCount)
	rankBottom := make([]int, rankCount)
	for rank := 0; rank < rankCount; rank++ {
		rankLeft[rank] = options.MaxWidth
		rankTop[rank] = options.MaxHeight
		rankRight[rank] = -1
		rankBottom[rank] = -1
	}
	maxRight := -1
	maxBottom := -1
	for nodeIndex, current := range placements {
		rank := plan.ranks[nodeIndex]
		rankLeft[rank] = min(rankLeft[rank], current.x)
		rankRight[rank] = max(rankRight[rank], current.x+current.width)
		rankTop[rank] = min(rankTop[rank], current.y)
		rankBottom[rank] = max(rankBottom[rank], current.y+current.height)
		maxRight = max(maxRight, current.x+current.width-1)
		maxBottom = max(maxBottom, current.y+current.height-1)
	}
	if graph.Direction == flow.TopToBottom {
		maxRight = max(maxRight, maxForwardLabelRight(graph, outer, placements))
	}

	routes := make([]feedbackRoute, 0, outerCount)
	mixedJunction := mixedForwardJunctionMask(graph, plan)
	type targetLaneKey struct {
		target int
		dashed bool
	}
	var forwardTargetLanes map[targetLaneKey]int
	nextLane := 0
	for edgeIndex, edge := range graph.Edges {
		if !outer[edgeIndex] {
			continue
		}
		from := placements[edge.From]
		to := placements[edge.To]
		route := feedbackRoute{edgeIndex: edgeIndex, feedback: plan.feedback[edgeIndex]}
		if graph.Direction == flow.TopToBottom && !plan.feedback[edgeIndex] && !mixedJunction[edgeIndex] {
			if compact, ok := planCompactTDForwardRoute(edgeIndex, edge, from, to, placements, options); ok {
				routes = append(routes, compact)
				continue
			}
		}
		laneIndex := nextLane
		if !plan.feedback[edgeIndex] {
			if forwardTargetLanes == nil {
				forwardTargetLanes = make(map[targetLaneKey]int)
			}
			key := targetLaneKey{target: edge.To, dashed: edge.Dashed}
			if existing, exists := forwardTargetLanes[key]; exists {
				laneIndex = existing
			} else {
				forwardTargetLanes[key] = laneIndex
				nextLane++
			}
		} else {
			nextLane++
		}
		if graph.Direction == flow.LeftToRight {
			sourceX := from.x + from.width
			sourceY := from.y + from.height/2
			sourceRank := plan.ranks[edge.From]
			sourceRail := rankRight[sourceRank] + 2
			if sourceRank+1 < rankCount {
				sourceRail = rankLeft[sourceRank+1] - 2
			}
			targetVerticalRail := rankLeft[plan.ranks[edge.To]] - 2
			targetArrowX := to.x - 1
			targetY := to.y + to.height/2
			laneY := maxBottom + 2 + laneIndex*2
			route.segments = []routeSegment{
				{x1: sourceX, y1: sourceY, x2: sourceRail, y2: sourceY, dashed: edge.Dashed},
				{x1: sourceRail, y1: sourceY, x2: sourceRail, y2: laneY, dashed: edge.Dashed},
				{x1: targetVerticalRail, y1: laneY, x2: sourceRail, y2: laneY, dashed: edge.Dashed},
				{x1: targetVerticalRail, y1: laneY, x2: targetVerticalRail, y2: targetY, dashed: edge.Dashed},
				{x1: targetVerticalRail, y1: targetY, x2: targetArrowX, y2: targetY, dashed: edge.Dashed},
			}
			route.arrowX = targetArrowX
			route.arrowY = targetY
			route.direction = right
		} else {
			sourceX := from.x + from.width/2
			sourceY := from.y + from.height
			sourceRail := rankBottom[plan.ranks[edge.From]]
			targetX := to.x + to.width/2
			targetHorizontalRail := rankTop[plan.ranks[edge.To]] - 2
			targetArrowY := to.y - 1
			laneX := maxRight + 2 + laneIndex*2
			route.segments = []routeSegment{
				{x1: sourceX, y1: sourceY, x2: sourceX, y2: sourceRail, dashed: edge.Dashed},
				{x1: sourceX, y1: sourceRail, x2: laneX, y2: sourceRail, dashed: edge.Dashed},
				{x1: laneX, y1: sourceRail, x2: laneX, y2: targetHorizontalRail, dashed: edge.Dashed},
				{x1: laneX, y1: targetHorizontalRail, x2: targetX, y2: targetHorizontalRail, dashed: edge.Dashed},
				{x1: targetX, y1: targetHorizontalRail, x2: targetX, y2: targetArrowY, dashed: edge.Dashed},
			}
			route.arrowX = targetX
			route.arrowY = targetArrowY
			route.direction = down
		}
		if err := validateFeedbackRoute(route, placements, options); err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}
	return routes, nil
}

func planCompactTDForwardRoute(edgeIndex int, edge flow.Edge, from, to placement, placements []placement, options Options) (feedbackRoute, bool) {
	sourceX := from.x + from.width/2
	sourceY := from.y + from.height
	targetX := to.x + to.width/2
	targetRailY := to.y - 2
	targetArrowY := to.y - 1
	if targetRailY <= sourceY {
		return feedbackRoute{}, false
	}

	segments := make([]routeSegment, 0, 4)
	tryLane := func(laneX int) (feedbackRoute, bool) {
		segments = segments[:0]
		if sourceX != laneX {
			segments = append(segments, routeSegment{x1: sourceX, y1: sourceY, x2: laneX, y2: sourceY, dashed: edge.Dashed})
		}
		segments = append(segments, routeSegment{x1: laneX, y1: sourceY, x2: laneX, y2: targetRailY, dashed: edge.Dashed})
		if laneX != targetX {
			segments = append(segments, routeSegment{x1: laneX, y1: targetRailY, x2: targetX, y2: targetRailY, dashed: edge.Dashed})
		}
		segments = append(segments, routeSegment{x1: targetX, y1: targetRailY, x2: targetX, y2: targetArrowY, dashed: edge.Dashed})
		route := feedbackRoute{
			edgeIndex: edgeIndex,
			segments:  segments,
			arrowX:    targetX,
			arrowY:    targetArrowY,
			direction: down,
		}
		if err := validateFeedbackRoute(route, placements, options); err == nil {
			return route, true
		}
		return feedbackRoute{}, false
	}
	if route, ok := tryLane(sourceX); ok {
		return route, true
	}
	if targetX != sourceX {
		if route, ok := tryLane(targetX); ok {
			return route, true
		}
	}
	for distance := 1; ; distance++ {
		left := sourceX - distance
		if left >= 0 && left != targetX {
			if route, ok := tryLane(left); ok {
				return route, true
			}
		}
		right := sourceX + distance
		if right < options.MaxWidth && right != targetX {
			if route, ok := tryLane(right); ok {
				return route, true
			}
		}
		if left < 0 && right >= options.MaxWidth {
			break
		}
	}
	return feedbackRoute{}, false
}

func maxForwardLabelRight(graph *flow.Graph, outer []bool, placements []placement) int {
	maxRight := -1
	for edgeIndex, edge := range graph.Edges {
		if outer[edgeIndex] || edge.Label == "" {
			continue
		}
		from := placements[edge.From]
		to := placements[edge.To]
		startX := from.x + from.width/2
		endX := to.x + to.width/2
		labelX := endX + 1
		if startX == endX {
			labelX = endX + 2
		}
		labelWidth, _ := textcell.Width(edge.Label)
		maxRight = max(maxRight, labelX+labelWidth-1)
	}
	return maxRight
}

func validateFeedbackRoute(route feedbackRoute, placements []placement, options Options) error {
	for _, current := range placements {
		if current.x < 0 || current.y < 0 || current.x+current.width > options.MaxWidth || current.y+current.height > options.MaxHeight {
			return fmt.Errorf("%w: feedback node", ErrOutputBounds)
		}
	}
	for _, segment := range route.segments {
		if segment.x1 != segment.x2 && segment.y1 != segment.y2 {
			return fmt.Errorf("%w: feedback segment is not orthogonal", ErrLayout)
		}
		for _, point := range [][2]int{{segment.x1, segment.y1}, {segment.x2, segment.y2}} {
			if point[0] < 0 || point[1] < 0 || point[0] >= options.MaxWidth || point[1] >= options.MaxHeight {
				return fmt.Errorf("%w: feedback route", ErrOutputBounds)
			}
		}
		for _, current := range placements {
			if segmentIntersectsPlacement(segment, current) {
				return fmt.Errorf("%w: feedback route overlaps a node", ErrLayout)
			}
		}
	}
	if route.arrowX < 0 || route.arrowY < 0 || route.arrowX >= options.MaxWidth || route.arrowY >= options.MaxHeight {
		return fmt.Errorf("%w: feedback arrow", ErrOutputBounds)
	}
	for _, current := range placements {
		if pointInsidePlacement(route.arrowX, route.arrowY, current) {
			return fmt.Errorf("%w: feedback arrow overlaps a node", ErrLayout)
		}
	}
	return nil
}

func segmentIntersectsPlacement(segment routeSegment, current placement) bool {
	left := current.x
	right := current.x + current.width - 1
	top := current.y
	bottom := current.y + current.height - 1
	if segment.y1 == segment.y2 {
		start, end := segment.x1, segment.x2
		if start > end {
			start, end = end, start
		}
		return segment.y1 >= top && segment.y1 <= bottom && start <= right && end >= left
	}
	start, end := segment.y1, segment.y2
	if start > end {
		start, end = end, start
	}
	return segment.x1 >= left && segment.x1 <= right && start <= bottom && end >= top
}

func pointInsidePlacement(x, y int, current placement) bool {
	return x >= current.x && x < current.x+current.width && y >= current.y && y < current.y+current.height
}

func drawOuterRoutes(canvas *canvas, routes []feedbackRoute) error {
	for _, route := range routes {
		for _, segment := range route.segments {
			if segment.y1 == segment.y2 {
				if err := canvas.horizontal(segment.x1, segment.x2, segment.y1, segment.dashed); err != nil {
					return err
				}
			} else {
				if err := canvas.vertical(segment.x1, segment.y1, segment.y2, segment.dashed); err != nil {
					return err
				}
			}
		}
		if err := canvas.arrow(route.arrowX, route.arrowY, route.direction); err != nil {
			return err
		}
	}
	return nil
}

func appendOuterLegends(output string, graph *flow.Graph, feedback, outer []bool, options Options) (string, error) {
	feedbackLegend := make([]string, 0)
	routedLegend := make([]string, 0)
	feedbackSequence := 1
	routedSequence := 1
	for edgeIndex, edge := range graph.Edges {
		if !outer[edgeIndex] {
			continue
		}
		arrow := "-->"
		if edge.Dashed {
			arrow = "-.->"
		}
		prefix := "R"
		sequence := routedSequence
		if feedback[edgeIndex] {
			prefix = "F"
			sequence = feedbackSequence
		}
		line := fmt.Sprintf("%s%02d %s %s %s", prefix, sequence, graph.Nodes[edge.From].ID, arrow, graph.Nodes[edge.To].ID)
		if edge.Label != "" {
			line += " |" + edge.Label + "|"
		}
		width, err := textcell.Width(line)
		if err != nil {
			return "", err
		}
		if width > options.MaxWidth {
			return "", fmt.Errorf("%w: feedback legend width %d > %d", ErrOutputBounds, width, options.MaxWidth)
		}
		if feedback[edgeIndex] {
			feedbackLegend = append(feedbackLegend, line)
			feedbackSequence++
		} else {
			routedLegend = append(routedLegend, line)
			routedSequence++
		}
	}
	if len(feedbackLegend) == 0 && len(routedLegend) == 0 {
		return output, nil
	}
	baseHeight := 0
	if output != "" {
		baseHeight = len(strings.Split(output, "\n"))
	}
	neededHeight := baseHeight
	if len(feedbackLegend) > 0 {
		neededHeight += 2 + len(feedbackLegend)
	}
	if len(routedLegend) > 0 {
		neededHeight += 2 + len(routedLegend)
	}
	if neededHeight > options.MaxHeight {
		return "", fmt.Errorf("%w: feedback legend height %d > %d", ErrOutputBounds, neededHeight, options.MaxHeight)
	}
	var suffix strings.Builder
	if len(feedbackLegend) > 0 {
		suffix.WriteString("\n\nfeedback:\n")
		suffix.WriteString(strings.Join(feedbackLegend, "\n"))
	}
	if len(routedLegend) > 0 {
		suffix.WriteString("\n\nrouted:\n")
		suffix.WriteString(strings.Join(routedLegend, "\n"))
	}
	return output + suffix.String(), nil
}
