package render

import (
	"fmt"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
)

type routePoint struct {
	x int
	y int
}

type forwardRoute struct {
	edgeIndex int
	segments  []routeSegment
	arrowX    int
	arrowY    int
	direction flowDirection
	labelX    int
	labelY    int
}

type routeDisjointSet struct {
	parent []int
}

func newRouteDisjointSet(size int) *routeDisjointSet {
	parent := make([]int, size)
	for index := range parent {
		parent[index] = index
	}
	return &routeDisjointSet{parent: parent}
}

func (set *routeDisjointSet) find(value int) int {
	for set.parent[value] != value {
		set.parent[value] = set.parent[set.parent[value]]
		value = set.parent[value]
	}
	return value
}

func (set *routeDisjointSet) union(left, right int) {
	leftRoot := set.find(left)
	rightRoot := set.find(right)
	if leftRoot == rightRoot {
		return
	}
	if leftRoot < rightRoot {
		set.parent[rightRoot] = leftRoot
		return
	}
	set.parent[leftRoot] = rightRoot
}

// validateForwardRouteTopology rejects a successful drawing when shared cells
// would connect more than one source to more than one target. A shared trunk is
// unambiguous only for a pure fan-out or a pure fan-in.
func planForwardRoutes(graph *flow.Graph, placements []placement, ranks []int, outer []bool) ([]forwardRoute, error) {
	routes := make([]forwardRoute, 0, len(graph.Edges))
	rankRight := make([]int, 0)
	if graph.Direction == flow.LeftToRight {
		maxRank := 0
		for _, rank := range ranks {
			maxRank = max(maxRank, rank)
		}
		rankRight = make([]int, maxRank+1)
		for nodeIndex, current := range placements {
			rankRight[ranks[nodeIndex]] = max(rankRight[ranks[nodeIndex]], current.x+current.width)
		}
	}
	for edgeIndex, edge := range graph.Edges {
		if outer[edgeIndex] {
			continue
		}
		from := placements[edge.From]
		to := placements[edge.To]
		route := forwardRoute{edgeIndex: edgeIndex}
		if graph.Direction == flow.LeftToRight {
			startX, startY := from.x+from.width, from.y+1
			endX, endY := to.x-1, to.y+1
			if endX <= startX+2 {
				return nil, fmt.Errorf("edge routing 공간 부족")
			}
			route.arrowX, route.arrowY, route.direction = endX, endY, right
			if startY == endY {
				route.segments = []routeSegment{{x1: startX, y1: startY, x2: endX, y2: endY, dashed: edge.Dashed}}
				route.labelX, route.labelY = startX+2, max(0, endY-1)
				routes = append(routes, route)
				continue
			}
			laneX := rankRight[ranks[edge.From]] + 2 + forwardEdgeLaneIndex(graph, ranks, outer, edgeIndex)*2
			if laneX >= endX {
				return nil, fmt.Errorf("%w: LR forward rail has no gap", ErrLayout)
			}
			route.segments = []routeSegment{
				{x1: startX, y1: startY, x2: laneX, y2: startY, dashed: edge.Dashed},
				{x1: laneX, y1: startY, x2: laneX, y2: endY, dashed: edge.Dashed},
				{x1: laneX, y1: endY, x2: endX, y2: endY, dashed: edge.Dashed},
			}
			route.labelX, route.labelY = laneX+2, max(0, endY-1)
			routes = append(routes, route)
			continue
		}

		startX, startY := from.x+from.width/2, from.y+from.height
		endX, endY := to.x+to.width/2, to.y-1
		if endY <= startY+2 {
			return nil, fmt.Errorf("edge routing 공간 부족")
		}
		route.arrowX, route.arrowY, route.direction = endX, endY, down
		if startX == endX {
			route.segments = []routeSegment{{x1: startX, y1: startY, x2: endX, y2: endY, dashed: edge.Dashed}}
			route.labelX, route.labelY = endX+2, startY+1
			routes = append(routes, route)
			continue
		}
		laneY := startY + 2 + forwardEdgeLaneIndex(graph, ranks, outer, edgeIndex)
		route.segments = []routeSegment{
			{x1: startX, y1: startY, x2: startX, y2: laneY, dashed: edge.Dashed},
			{x1: startX, y1: laneY, x2: endX, y2: laneY, dashed: edge.Dashed},
			{x1: endX, y1: laneY, x2: endX, y2: endY, dashed: edge.Dashed},
		}
		route.labelX, route.labelY = endX+1, max(0, laneY-1)
		routes = append(routes, route)
	}
	return routes, nil
}

func validateForwardRouteTopology(graph *flow.Graph, routes []forwardRoute) error {
	seenEndpointPairs := make(map[[2]int]struct{}, len(graph.Edges))
	set := newRouteDisjointSet(len(routes))
	owners := make(map[routePoint]int)
	for routeIndex, route := range routes {
		edge := graph.Edges[route.edgeIndex]
		endpointPair := [2]int{edge.From, edge.To}
		if _, exists := seenEndpointPairs[endpointPair]; exists {
			return fmt.Errorf("%w: parallel forward edge %d", ErrInvalidGraph, route.edgeIndex)
		}
		seenEndpointPairs[endpointPair] = struct{}{}
		for _, segment := range route.segments {
			forEachRoutePoint(segment, func(point routePoint) {
				if owner, exists := owners[point]; exists {
					if owner != routeIndex {
						set.union(routeIndex, owner)
					}
					return
				}
				owners[point] = routeIndex
			})
		}
	}
	if len(routes) < 2 {
		return nil
	}

	type componentEndpoints struct {
		source         int
		target         int
		multipleSource bool
		multipleTarget bool
		initialized    bool
	}
	components := make([]componentEndpoints, len(routes))
	for routeIndex, route := range routes {
		root := set.find(routeIndex)
		component := &components[root]
		edge := graph.Edges[route.edgeIndex]
		if !component.initialized {
			component.source = edge.From
			component.target = edge.To
			component.initialized = true
			continue
		}
		component.multipleSource = component.multipleSource || component.source != edge.From
		component.multipleTarget = component.multipleTarget || component.target != edge.To
	}
	for _, component := range components {
		if component.multipleSource && component.multipleTarget {
			return fmt.Errorf("%w: 독립 forward edge가 같은 route cell을 공유함", ErrLayout)
		}
	}
	return nil
}

func forwardEdgeCountsByRank(graph *flow.Graph, ranks []int, outer []bool) []int {
	maxRank := 0
	for _, rank := range ranks {
		maxRank = max(maxRank, rank)
	}
	counts := make([]int, maxRank+1)
	for edgeIndex, edge := range graph.Edges {
		if outer[edgeIndex] {
			continue
		}
		counts[ranks[edge.From]]++
	}
	return counts
}

func forwardEdgeLaneIndex(graph *flow.Graph, ranks []int, outer []bool, edgeIndex int) int {
	rank := ranks[graph.Edges[edgeIndex].From]
	lane := 0
	for previous := 0; previous < edgeIndex; previous++ {
		if !outer[previous] && ranks[graph.Edges[previous].From] == rank {
			lane++
		}
	}
	return lane
}

func forEachRoutePoint(segment routeSegment, visit func(routePoint)) {
	if segment.y1 == segment.y2 {
		start, end := segment.x1, segment.x2
		if start > end {
			start, end = end, start
		}
		for x := start; x <= end; x++ {
			visit(routePoint{x: x, y: segment.y1})
		}
		return
	}
	start, end := segment.y1, segment.y2
	if start > end {
		start, end = end, start
	}
	for y := start; y <= end; y++ {
		visit(routePoint{x: segment.x1, y: y})
	}
}
