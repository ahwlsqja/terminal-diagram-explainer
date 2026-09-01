package render

import (
	"fmt"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

const (
	scopeBandGap      = 2
	scopeTitleTop     = 1
	scopeContentTopLR = 4
	scopeFramePadLR   = 4
)

type scopeRect struct {
	left, top, right, bottom int
}

type scopeTree struct {
	children [][]flow.ScopeRef
	direct   [][]int
	depths   []int
	maxDepth int
}

type scopedLayout struct {
	placements []placement
	routeX     []int
	routeY     []int
	frames     []scopeRect
}

func flowScoped(graph *flow.Graph, plan rankPlan, outer []bool, options Options) (string, error) {
	if options.MaxWidth > 512 || options.MaxHeight > 512 {
		return "", fmt.Errorf("%w: canvas %dx%d", ErrOutputBounds, options.MaxWidth, options.MaxHeight)
	}
	layout, err := placeScoped(graph, plan, outer)
	if err != nil {
		return "", err
	}
	if err := validateScopedFrames(layout.frames, options); err != nil {
		return "", err
	}
	routes, err := planScopedOuterRoutes(graph, plan, outer, layout, options)
	if err != nil {
		return "", err
	}
	canvas, err := newCanvas(options.MaxWidth, options.MaxHeight, options.ASCII)
	if err != nil {
		return "", err
	}
	if err := drawScopeFrames(canvas, graph, layout.frames); err != nil {
		return "", err
	}
	if err := drawForwardEdges(canvas, graph, layout.placements, plan.ranks, outer); err != nil {
		return "", err
	}
	if err := drawOuterRoutes(canvas, routes); err != nil {
		return "", err
	}
	for index, node := range graph.Nodes {
		if err := drawNode(canvas, node, layout.placements[index]); err != nil {
			return "", err
		}
	}
	return appendOuterLegends(canvas.String(), graph, plan.feedback, outer, options)
}

func placeScoped(graph *flow.Graph, plan rankPlan, outer []bool) (scopedLayout, error) {
	tree := buildScopeTree(graph)
	widths := make([]int, len(graph.Nodes))
	for index, node := range graph.Nodes {
		width, err := textcell.Width(displayLabel(node))
		if err != nil {
			return scopedLayout{}, err
		}
		widths[index] = max(7, width+4)
	}
	if graph.Direction == flow.LeftToRight {
		return placeScopedLR(graph, plan, outer, tree, widths)
	}
	return placeScopedTD(graph, plan, outer, tree, widths)
}

func buildScopeTree(graph *flow.Graph) scopeTree {
	scopeCount := len(graph.Subgraphs) + 1
	tree := scopeTree{
		children: make([][]flow.ScopeRef, scopeCount),
		direct:   make([][]int, scopeCount),
		depths:   make([]int, scopeCount),
	}
	for index, subgraph := range graph.Subgraphs {
		ref := flow.ScopeRef(index + 1)
		tree.children[subgraph.Parent] = append(tree.children[subgraph.Parent], ref)
		tree.depths[ref] = tree.depths[subgraph.Parent] + 1
		tree.maxDepth = max(tree.maxDepth, tree.depths[ref])
	}
	for index, node := range graph.Nodes {
		tree.direct[node.Scope] = append(tree.direct[node.Scope], index)
	}
	return tree
}

func placeScopedLR(graph *flow.Graph, plan rankPlan, outer []bool, tree scopeTree, widths []int) (scopedLayout, error) {
	layout := scopedLayout{
		placements: make([]placement, len(graph.Nodes)),
		routeX:     make([]int, len(graph.Nodes)),
		routeY:     make([]int, len(graph.Nodes)),
		frames:     make([]scopeRect, len(graph.Subgraphs)),
	}
	columnWidths := make([]int, plan.maxRank+1)
	columnGaps := make([]int, plan.maxRank+1)
	forwardCounts := forwardEdgeCountsByRank(graph, plan.ranks, outer)
	for nodeIndex, rank := range plan.ranks {
		columnWidths[rank] = max(columnWidths[rank], widths[nodeIndex])
		columnGaps[rank] = max(10, forwardCounts[rank]*2+4)
	}
	for edgeIndex, edge := range graph.Edges {
		if outer[edgeIndex] {
			continue
		}
		labelWidth, _ := textcell.Width(edge.Label)
		columnGaps[plan.ranks[edge.From]] = max(columnGaps[plan.ranks[edge.From]], labelWidth+7)
	}
	x := tree.maxDepth * scopeFramePadLR
	for rank := 0; rank <= plan.maxRank; rank++ {
		for nodeIndex, nodeRank := range plan.ranks {
			if nodeRank == rank {
				layout.placements[nodeIndex] = placement{x: x, width: widths[nodeIndex], height: 3}
			}
		}
		x += columnWidths[rank] + columnGaps[rank]
	}

	directHeights := make([]int, len(tree.direct))
	for ref, nodes := range tree.direct {
		counts := make([]int, plan.maxRank+1)
		for _, nodeIndex := range nodes {
			counts[plan.ranks[nodeIndex]]++
		}
		maxCount := 0
		for _, count := range counts {
			maxCount = max(maxCount, count)
		}
		if maxCount > 0 {
			directHeights[ref] = maxCount*6 - 1
		}
	}
	heights := make([]int, len(tree.direct))
	for ref := len(tree.direct) - 1; ref >= 0; ref-- {
		contentHeight := stackedHeight(directHeights[ref], tree.children[ref], heights)
		if ref == int(flow.RootScope) {
			heights[ref] = contentHeight
		} else {
			heights[ref] = scopeContentTopLR + contentHeight + 2
		}
	}

	var placeScope func(flow.ScopeRef, int)
	placeScope = func(ref flow.ScopeRef, top int) {
		cursor := top
		if ref != flow.RootScope {
			cursor += scopeContentTopLR
			layout.frames[int(ref)-1].top = top
			layout.frames[int(ref)-1].bottom = top + heights[ref]
		}
		placedItem := false
		if directHeights[ref] > 0 {
			used := make([]int, plan.maxRank+1)
			for _, nodeIndex := range tree.direct[ref] {
				rank := plan.ranks[nodeIndex]
				y := cursor + used[rank]*6
				layout.placements[nodeIndex].y = y
				layout.routeY[nodeIndex] = y + 4
				used[rank]++
			}
			cursor += directHeights[ref]
			placedItem = true
		}
		for _, child := range tree.children[ref] {
			if placedItem {
				cursor += scopeBandGap
			}
			placeScope(child, cursor)
			cursor += heights[child]
			placedItem = true
		}
	}
	placeScope(flow.RootScope, 0)

	for ref := len(tree.direct) - 1; ref > 0; ref-- {
		left, right, found := scopeHorizontalContent(flow.ScopeRef(ref), tree, layout.placements, layout.frames)
		if !found {
			return scopedLayout{}, fmt.Errorf("%w: empty scope %d", ErrInvalidGraph, ref)
		}
		left -= scopeFramePadLR
		right += scopeFramePadLR
		titleWidth, _ := textcell.Width(graph.Subgraphs[ref-1].Label)
		if right-left < titleWidth+4 {
			right = left + titleWidth + 4
		}
		layout.frames[ref-1].left = left
		layout.frames[ref-1].right = right
	}

	shiftScopedLayoutY(&layout, -scopedContentBounds(layout).top)
	shiftScopedLayoutX(&layout, scopedLeftPadding(outer)-scopedContentBounds(layout).left)
	return layout, nil
}

func placeScopedTD(graph *flow.Graph, plan rankPlan, outer []bool, tree scopeTree, widths []int) (scopedLayout, error) {
	layout := scopedLayout{
		placements: make([]placement, len(graph.Nodes)),
		routeX:     make([]int, len(graph.Nodes)),
		routeY:     make([]int, len(graph.Nodes)),
		frames:     make([]scopeRect, len(graph.Subgraphs)),
	}
	yBase := tree.maxDepth * scopeContentTopLR
	forwardCounts := forwardEdgeCountsByRank(graph, plan.ranks, outer)
	rankY := make([]int, plan.maxRank+1)
	for rank := 1; rank <= plan.maxRank; rank++ {
		rowGap := max(5, forwardCounts[rank-1]+3)
		rankY[rank] = rankY[rank-1] + 3 + rowGap
	}
	for nodeIndex, rank := range plan.ranks {
		layout.placements[nodeIndex] = placement{y: yBase + rankY[rank], width: widths[nodeIndex], height: 3}
	}

	slotWidths := make([][]int, len(tree.direct))
	directWidths := make([]int, len(tree.direct))
	for ref, nodes := range tree.direct {
		used := make([]int, plan.maxRank+1)
		for _, nodeIndex := range nodes {
			rank := plan.ranks[nodeIndex]
			slot := used[rank]
			for len(slotWidths[ref]) <= slot {
				slotWidths[ref] = append(slotWidths[ref], 0)
			}
			slotWidths[ref][slot] = max(slotWidths[ref][slot], widths[nodeIndex]+2)
			used[rank]++
		}
		for _, width := range slotWidths[ref] {
			directWidths[ref] += width
		}
	}
	directBandWidths := append([]int(nil), directWidths...)
	for ref, width := range directBandWidths {
		if width > 0 {
			directBandWidths[ref]++
		}
	}
	for edgeIndex, edge := range graph.Edges {
		if outer[edgeIndex] || edge.Label == "" {
			continue
		}
		labelWidth, _ := textcell.Width(edge.Label)
		ref := graph.Nodes[edge.From].Scope
		directBandWidths[ref] = max(directBandWidths[ref], directWidths[ref]+labelWidth+3)
	}

	scopeWidths := make([]int, len(tree.direct))
	for ref := len(tree.direct) - 1; ref >= 0; ref-- {
		contentWidth := stackedWidth(directBandWidths[ref], tree.children[ref], scopeWidths)
		if ref == int(flow.RootScope) {
			scopeWidths[ref] = contentWidth
			continue
		}
		titleWidth, _ := textcell.Width(graph.Subgraphs[ref-1].Label)
		scopeWidths[ref] = max(titleWidth+4, contentWidth+4)
	}

	var placeScope func(flow.ScopeRef, int)
	placeScope = func(ref flow.ScopeRef, left int) {
		cursor := left
		if ref != flow.RootScope {
			layout.frames[int(ref)-1].left = left
			layout.frames[int(ref)-1].right = left + scopeWidths[ref]
			cursor += 2
		}
		placedItem := false
		if directBandWidths[ref] > 0 {
			slotLefts := make([]int, len(slotWidths[ref]))
			next := cursor
			for slot, width := range slotWidths[ref] {
				slotLefts[slot] = next
				next += width
			}
			used := make([]int, plan.maxRank+1)
			for _, nodeIndex := range tree.direct[ref] {
				rank := plan.ranks[nodeIndex]
				slot := used[rank]
				layout.placements[nodeIndex].x = slotLefts[slot]
				layout.routeX[nodeIndex] = cursor + directBandWidths[ref] - 1
				used[rank]++
			}
			cursor += directBandWidths[ref]
			placedItem = true
		}
		for _, child := range tree.children[ref] {
			if placedItem {
				cursor += scopeBandGap
			}
			placeScope(child, cursor)
			cursor += scopeWidths[child]
			placedItem = true
		}
	}
	placeScope(flow.RootScope, 0)

	for ref := len(tree.direct) - 1; ref > 0; ref-- {
		top, bottom, found := scopeVerticalContent(flow.ScopeRef(ref), tree, layout.placements, layout.frames)
		if !found {
			return scopedLayout{}, fmt.Errorf("%w: empty scope %d", ErrInvalidGraph, ref)
		}
		layout.frames[ref-1].top = top - 4
		layout.frames[ref-1].bottom = bottom + 2
	}

	shiftScopedLayoutY(&layout, -scopedMinimumY(graph, outer, layout))
	shiftScopedLayoutX(&layout, -scopedContentBounds(layout).left)
	return layout, nil
}

func stackedHeight(directHeight int, children []flow.ScopeRef, heights []int) int {
	total := 0
	items := 0
	if directHeight > 0 {
		total = directHeight
		items++
	}
	for _, child := range children {
		if items > 0 {
			total += scopeBandGap
		}
		total += heights[child]
		items++
	}
	return total
}

func stackedWidth(directWidth int, children []flow.ScopeRef, widths []int) int {
	total := 0
	items := 0
	if directWidth > 0 {
		total = directWidth
		items++
	}
	for _, child := range children {
		if items > 0 {
			total += scopeBandGap
		}
		total += widths[child]
		items++
	}
	return total
}

func scopeHorizontalContent(ref flow.ScopeRef, tree scopeTree, placements []placement, frames []scopeRect) (int, int, bool) {
	left, right := 0, 0
	found := false
	for _, nodeIndex := range tree.direct[ref] {
		current := placements[nodeIndex]
		if !found || current.x < left {
			left = current.x
		}
		if !found || current.x+current.width > right {
			right = current.x + current.width
		}
		found = true
	}
	for _, child := range tree.children[ref] {
		current := frames[int(child)-1]
		if !found || current.left < left {
			left = current.left
		}
		if !found || current.right > right {
			right = current.right
		}
		found = true
	}
	return left, right, found
}

func scopeVerticalContent(ref flow.ScopeRef, tree scopeTree, placements []placement, frames []scopeRect) (int, int, bool) {
	top, bottom := 0, 0
	found := false
	for _, nodeIndex := range tree.direct[ref] {
		current := placements[nodeIndex]
		if !found || current.y < top {
			top = current.y
		}
		if !found || current.y+current.height > bottom {
			bottom = current.y + current.height
		}
		found = true
	}
	for _, child := range tree.children[ref] {
		current := frames[int(child)-1]
		if !found || current.top < top {
			top = current.top
		}
		if !found || current.bottom > bottom {
			bottom = current.bottom
		}
		found = true
	}
	return top, bottom, found
}

func scopedLeftPadding(outer []bool) int {
	count := 0
	for _, value := range outer {
		if value {
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return count * 2
}

func scopedContentBounds(layout scopedLayout) scopeRect {
	bounds := scopeRect{}
	found := false
	for _, current := range layout.placements {
		bounds, found = includeScopeRect(bounds, found, scopeRect{
			left: current.x, top: current.y, right: current.x + current.width, bottom: current.y + current.height,
		})
	}
	for _, current := range layout.frames {
		bounds, found = includeScopeRect(bounds, found, current)
	}
	return bounds
}

func scopedMinimumY(graph *flow.Graph, outer []bool, layout scopedLayout) int {
	top := scopedContentBounds(layout).top
	if graph.Direction != flow.TopToBottom {
		return top
	}
	for edgeIndex, edge := range graph.Edges {
		if outer[edgeIndex] {
			top = min(top, layout.placements[edge.To].y-2)
		}
	}
	return top
}

func includeScopeRect(bounds scopeRect, found bool, current scopeRect) (scopeRect, bool) {
	if !found {
		return current, true
	}
	bounds.left = min(bounds.left, current.left)
	bounds.top = min(bounds.top, current.top)
	bounds.right = max(bounds.right, current.right)
	bounds.bottom = max(bounds.bottom, current.bottom)
	return bounds, true
}

func shiftScopedLayoutX(layout *scopedLayout, dx int) {
	if dx == 0 {
		return
	}
	for index := range layout.placements {
		layout.placements[index].x += dx
		layout.routeX[index] += dx
	}
	for index := range layout.frames {
		layout.frames[index].left += dx
		layout.frames[index].right += dx
	}
}

func shiftScopedLayoutY(layout *scopedLayout, dy int) {
	if dy == 0 {
		return
	}
	for index := range layout.placements {
		layout.placements[index].y += dy
		layout.routeY[index] += dy
	}
	for index := range layout.frames {
		layout.frames[index].top += dy
		layout.frames[index].bottom += dy
	}
}

func planScopedOuterRoutes(graph *flow.Graph, plan rankPlan, outer []bool, layout scopedLayout, options Options) ([]feedbackRoute, error) {
	outerCount := 0
	for _, value := range outer {
		if value {
			outerCount++
		}
	}
	if outerCount == 0 {
		return nil, nil
	}
	bounds := scopedContentBounds(layout)
	routes := make([]feedbackRoute, 0, outerCount)
	lane := 0
	for edgeIndex, edge := range graph.Edges {
		if !outer[edgeIndex] {
			continue
		}
		from := layout.placements[edge.From]
		to := layout.placements[edge.To]
		route := feedbackRoute{edgeIndex: edgeIndex, feedback: plan.feedback[edgeIndex]}
		if graph.Direction == flow.LeftToRight {
			sourceX := from.x + from.width
			sourceY := from.y + from.height/2
			sourceTurnX := sourceX + 2
			sourceRouteY := layout.routeY[edge.From]
			targetArrowX := to.x - 1
			targetY := to.y + to.height/2
			targetRouteY := layout.routeY[edge.To]
			globalRight := bounds.right + 1 + lane*2
			globalLeft := bounds.left - 2 - lane*2
			laneY := bounds.bottom + 1 + lane*2
			route.segments = []routeSegment{
				{x1: sourceX, y1: sourceY, x2: sourceTurnX, y2: sourceY, dashed: edge.Dashed},
				{x1: sourceTurnX, y1: sourceY, x2: sourceTurnX, y2: sourceRouteY, dashed: edge.Dashed},
				{x1: sourceTurnX, y1: sourceRouteY, x2: globalRight, y2: sourceRouteY, dashed: edge.Dashed},
				{x1: globalRight, y1: sourceRouteY, x2: globalRight, y2: laneY, dashed: edge.Dashed},
				{x1: globalRight, y1: laneY, x2: globalLeft, y2: laneY, dashed: edge.Dashed},
				{x1: globalLeft, y1: laneY, x2: globalLeft, y2: targetRouteY, dashed: edge.Dashed},
				{x1: globalLeft, y1: targetRouteY, x2: targetArrowX, y2: targetRouteY, dashed: edge.Dashed},
				{x1: targetArrowX, y1: targetRouteY, x2: targetArrowX, y2: targetY, dashed: edge.Dashed},
			}
			route.arrowX, route.arrowY, route.direction = targetArrowX, targetY, right
		} else {
			sourceX := from.x + from.width/2
			sourceY := from.y + from.height
			sourceRailY := sourceY + 2
			sourceRouteX := layout.routeX[edge.From]
			targetX := to.x + to.width/2
			targetRailY := to.y - 2
			targetArrowY := to.y - 1
			targetRouteX := layout.routeX[edge.To]
			laneY := bounds.bottom + 1 + lane*2
			route.segments = []routeSegment{
				{x1: sourceX, y1: sourceY, x2: sourceX, y2: sourceRailY, dashed: edge.Dashed},
				{x1: sourceX, y1: sourceRailY, x2: sourceRouteX, y2: sourceRailY, dashed: edge.Dashed},
				{x1: sourceRouteX, y1: sourceRailY, x2: sourceRouteX, y2: laneY, dashed: edge.Dashed},
				{x1: sourceRouteX, y1: laneY, x2: targetRouteX, y2: laneY, dashed: edge.Dashed},
				{x1: targetRouteX, y1: laneY, x2: targetRouteX, y2: targetRailY, dashed: edge.Dashed},
				{x1: targetRouteX, y1: targetRailY, x2: targetX, y2: targetRailY, dashed: edge.Dashed},
				{x1: targetX, y1: targetRailY, x2: targetX, y2: targetArrowY, dashed: edge.Dashed},
			}
			route.arrowX, route.arrowY, route.direction = targetX, targetArrowY, down
		}
		if err := validateFeedbackRoute(route, layout.placements, options); err != nil {
			return nil, err
		}
		if err := validateScopedRouteFrames(graph, edge, route, layout.frames); err != nil {
			return nil, err
		}
		routes = append(routes, route)
		lane++
	}
	return routes, nil
}

func validateScopedFrames(frames []scopeRect, options Options) error {
	for index, frame := range frames {
		if frame.left < 0 || frame.top < 0 || frame.right > options.MaxWidth || frame.bottom > options.MaxHeight {
			return fmt.Errorf("%w: subgraph frame %d", ErrOutputBounds, index)
		}
		if frame.right-frame.left < 4 || frame.bottom-frame.top < 4 {
			return fmt.Errorf("%w: subgraph frame %d", ErrLayout, index)
		}
	}
	return nil
}

func validateScopedRouteFrames(graph *flow.Graph, edge flow.Edge, route feedbackRoute, frames []scopeRect) error {
	for frameIndex, frame := range frames {
		ref := flow.ScopeRef(frameIndex + 1)
		containsSource := scopeContainsNode(graph, ref, graph.Nodes[edge.From].Scope)
		containsTarget := scopeContainsNode(graph, ref, graph.Nodes[edge.To].Scope)
		for _, segment := range route.segments {
			if !segmentIntersectsScopeRect(segment, frame) {
				continue
			}
			if !containsSource && !containsTarget {
				return fmt.Errorf("%w: outer route crosses unrelated subgraph", ErrLayout)
			}
			titleWidth, _ := textcell.Width(graph.Subgraphs[frameIndex].Label)
			title := scopeRect{
				left: frame.left + 2, top: frame.top + scopeTitleTop,
				right: frame.left + 2 + titleWidth, bottom: frame.top + scopeTitleTop + 1,
			}
			if segmentIntersectsScopeRect(segment, title) {
				return fmt.Errorf("%w: outer route overlaps subgraph title", ErrLayout)
			}
			portals, collinear := scopeBorderPortals(segment, frame)
			if collinear {
				return fmt.Errorf("%w: outer route overlaps subgraph border", ErrLayout)
			}
			for _, side := range portals {
				allowed := false
				if graph.Direction == flow.LeftToRight {
					allowed = side == portalRight && containsSource || side == portalLeft && containsTarget
				} else {
					allowed = side == portalBottom && (containsSource || containsTarget)
				}
				if !allowed {
					return fmt.Errorf("%w: outer route uses invalid subgraph portal", ErrLayout)
				}
			}
		}
	}
	return nil
}

type portalSide uint8

const (
	portalLeft portalSide = iota
	portalTop
	portalRight
	portalBottom
)

func scopeBorderPortals(segment routeSegment, frame scopeRect) ([]portalSide, bool) {
	left, right := frame.left, frame.right-1
	top, bottom := frame.top, frame.bottom-1
	if segment.y1 == segment.y2 {
		start, end := segment.x1, segment.x2
		if start > end {
			start, end = end, start
		}
		if (segment.y1 == top || segment.y1 == bottom) && start <= right && end >= left {
			return nil, true
		}
		if segment.y1 <= top || segment.y1 >= bottom {
			return nil, false
		}
		portals := make([]portalSide, 0, 2)
		if start < left && end >= left {
			portals = append(portals, portalLeft)
		}
		if start <= right && end > right {
			portals = append(portals, portalRight)
		}
		return portals, false
	}

	start, end := segment.y1, segment.y2
	if start > end {
		start, end = end, start
	}
	if (segment.x1 == left || segment.x1 == right) && start <= bottom && end >= top {
		return nil, true
	}
	if segment.x1 <= left || segment.x1 >= right {
		return nil, false
	}
	portals := make([]portalSide, 0, 2)
	if start < top && end >= top {
		portals = append(portals, portalTop)
	}
	if start <= bottom && end > bottom {
		portals = append(portals, portalBottom)
	}
	return portals, false
}

func scopeContainsNode(graph *flow.Graph, ancestor, scope flow.ScopeRef) bool {
	for scope != flow.RootScope {
		if scope == ancestor {
			return true
		}
		scope = graph.Subgraphs[int(scope)-1].Parent
	}
	return false
}

func segmentIntersectsScopeRect(segment routeSegment, current scopeRect) bool {
	left, right := current.left, current.right-1
	top, bottom := current.top, current.bottom-1
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

func drawScopeFrames(canvas *canvas, graph *flow.Graph, frames []scopeRect) error {
	for index, frame := range frames {
		if err := drawScopeFrame(canvas, graph.Subgraphs[index].Label, frame); err != nil {
			return err
		}
	}
	return nil
}

func drawScopeFrame(canvas *canvas, title string, frame scopeRect) error {
	left, right := frame.left, frame.right-1
	top, bottom := frame.top, frame.bottom-1
	topLeft, topRight, bottomLeft, bottomRight, horizontal, vertical := "┌", "┐", "└", "┘", "─", "│"
	if canvas.ascii {
		topLeft, topRight, bottomLeft, bottomRight, horizontal, vertical = "+", "+", "+", "+", "-", "|"
	}
	for _, point := range []struct {
		x, y int
		text string
	}{
		{x: left, y: top, text: topLeft},
		{x: right, y: top, text: topRight},
		{x: left, y: bottom, text: bottomLeft},
		{x: right, y: bottom, text: bottomRight},
	} {
		if err := canvas.put(point.x, point.y, point.text); err != nil {
			return err
		}
	}
	for x := left + 1; x < right; x++ {
		if err := canvas.put(x, top, horizontal); err != nil {
			return err
		}
		if err := canvas.put(x, bottom, horizontal); err != nil {
			return err
		}
	}
	for y := top + 1; y < bottom; y++ {
		if err := canvas.put(left, y, vertical); err != nil {
			return err
		}
		if err := canvas.put(right, y, vertical); err != nil {
			return err
		}
	}
	return canvas.putText(left+2, top+scopeTitleTop, title)
}
