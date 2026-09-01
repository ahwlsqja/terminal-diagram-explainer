package render

import (
	"sort"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
)

type orderScore struct {
	sum   int
	count int
}

// minimizeForwardCrossings performs bounded median-style sweeps while keeping
// source order as the stable tie-break. It only considers adjacent-rank forward
// edges; feedback and skip-rank routes are handled by the outer router.
func minimizeForwardCrossings(graph *flow.Graph, groups [][]int, ranks []int, outer []bool) {
	if len(groups) < 2 {
		return
	}
	positions := make([]int, len(graph.Nodes))
	for iteration := 0; iteration < 2; iteration++ {
		for rank := 1; rank < len(groups); rank++ {
			setGroupPositions(positions, groups[rank-1])
			scores := make(map[int]orderScore, len(groups[rank]))
			for edgeIndex, edge := range graph.Edges {
				if outer[edgeIndex] || ranks[edge.From] != rank-1 || ranks[edge.To] != rank {
					continue
				}
				score := scores[edge.To]
				score.sum += positions[edge.From]
				score.count++
				scores[edge.To] = score
			}
			stableSortGroup(groups[rank], scores)
		}
		for rank := len(groups) - 2; rank >= 0; rank-- {
			setGroupPositions(positions, groups[rank+1])
			scores := make(map[int]orderScore, len(groups[rank]))
			for edgeIndex, edge := range graph.Edges {
				if outer[edgeIndex] || ranks[edge.From] != rank || ranks[edge.To] != rank+1 {
					continue
				}
				score := scores[edge.From]
				score.sum += positions[edge.To]
				score.count++
				scores[edge.From] = score
			}
			stableSortGroup(groups[rank], scores)
		}
	}
}

func setGroupPositions(positions []int, group []int) {
	for position, node := range group {
		positions[node] = position
	}
}

func stableSortGroup(group []int, scores map[int]orderScore) {
	sort.SliceStable(group, func(left, right int) bool {
		leftScore, leftExists := scores[group[left]]
		rightScore, rightExists := scores[group[right]]
		if leftExists != rightExists {
			return leftExists
		}
		if !leftExists {
			return false
		}
		return leftScore.sum*rightScore.count < rightScore.sum*leftScore.count
	})
}
