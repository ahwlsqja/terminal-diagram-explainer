package render

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/flow"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

const maxGraphWorkSteps = 32_768

const (
	maxRenderNodes      = 48
	maxRenderEdges      = 96
	maxRenderIDBytes    = 64
	maxRenderLabelCells = 96
)

var (
	ErrInvalidGraph = errors.New("유효하지 않은 graph")
	ErrWorkBudget   = errors.New("graph 작업 budget 초과")
	ErrOutputBounds = errors.New("출력 경계 초과")
	ErrLayout       = errors.New("layout 충돌")
)

type rankPlan struct {
	ranks       []int
	maxRank     int
	feedback    []bool
	componentOf []int
}

type workBudget struct {
	used  int
	limit int
}

func (b *workBudget) take() error {
	if b.limit <= 0 || b.used >= b.limit {
		return ErrWorkBudget
	}
	b.used++
	return nil
}

func analyzeRanksWithBudget(graph *flow.Graph, limit int) (rankPlan, error) {
	if err := validateGraph(graph); err != nil {
		return rankPlan{}, err
	}
	budget := &workBudget{limit: limit}
	nodeCount := len(graph.Nodes)
	outgoing := make([][]int, nodeCount)
	for edgeIndex, edge := range graph.Edges {
		if err := budget.take(); err != nil {
			return rankPlan{}, err
		}
		outgoing[edge.From] = append(outgoing[edge.From], edgeIndex)
	}

	componentOf, err := tarjanComponents(graph, outgoing, budget)
	if err != nil {
		return rankPlan{}, err
	}
	for range graph.Nodes {
		if err := budget.take(); err != nil {
			return rankPlan{}, err
		}
	}
	componentOf = normalizeComponents(componentOf)

	feedback := make([]bool, len(graph.Edges))
	keptInternal := make([][]int, nodeCount)
	for edgeIndex, edge := range graph.Edges {
		if err := budget.take(); err != nil {
			return rankPlan{}, err
		}
		if edge.From == edge.To {
			feedback[edgeIndex] = true
			continue
		}
		if componentOf[edge.From] != componentOf[edge.To] {
			continue
		}
		reachable, reachErr := reachableWithinComponent(
			graph,
			keptInternal,
			componentOf,
			edge.To,
			edge.From,
			budget,
		)
		if reachErr != nil {
			return rankPlan{}, reachErr
		}
		if reachable {
			feedback[edgeIndex] = true
			continue
		}
		keptInternal[edge.From] = append(keptInternal[edge.From], edgeIndex)
	}

	ranks, maxRank, err := rankForwardGraph(graph, feedback, budget)
	if err != nil {
		return rankPlan{}, err
	}
	return rankPlan{
		ranks:       ranks,
		maxRank:     maxRank,
		feedback:    feedback,
		componentOf: componentOf,
	}, nil
}

func validateGraph(graph *flow.Graph) error {
	if graph == nil {
		return fmt.Errorf("%w: nil", ErrInvalidGraph)
	}
	if len(graph.Nodes) == 0 || len(graph.Nodes) > maxRenderNodes {
		return fmt.Errorf("%w: node count %d", ErrInvalidGraph, len(graph.Nodes))
	}
	if len(graph.Edges) > maxRenderEdges {
		return fmt.Errorf("%w: edge count %d", ErrInvalidGraph, len(graph.Edges))
	}
	if graph.Direction != flow.LeftToRight && graph.Direction != flow.TopToBottom {
		return fmt.Errorf("%w: direction %d", ErrInvalidGraph, graph.Direction)
	}
	seenIDs := make(map[string]struct{}, len(graph.Nodes))
	for nodeIndex, node := range graph.Nodes {
		if !validNodeID(node.ID, maxRenderIDBytes) {
			return fmt.Errorf("%w: node %d ID", ErrInvalidGraph, nodeIndex)
		}
		if _, exists := seenIDs[node.ID]; exists {
			return fmt.Errorf("%w: duplicate node ID %q", ErrInvalidGraph, node.ID)
		}
		seenIDs[node.ID] = struct{}{}
		if node.Shape != flow.Process && node.Shape != flow.Decision && node.Shape != flow.DataStore {
			return fmt.Errorf("%w: node %d shape", ErrInvalidGraph, nodeIndex)
		}
		labelWidth, err := textcell.Width(node.Label)
		if err != nil || labelWidth == 0 || labelWidth > maxRenderLabelCells {
			return fmt.Errorf("%w: node %d label", ErrInvalidGraph, nodeIndex)
		}
	}
	for edgeIndex, edge := range graph.Edges {
		if edge.From < 0 || edge.From >= len(graph.Nodes) || edge.To < 0 || edge.To >= len(graph.Nodes) {
			return fmt.Errorf("%w: edge %d endpoint", ErrInvalidGraph, edgeIndex)
		}
		if edge.Label != "" {
			labelWidth, err := textcell.Width(edge.Label)
			if err != nil || labelWidth > maxRenderLabelCells {
				return fmt.Errorf("%w: edge %d label", ErrInvalidGraph, edgeIndex)
			}
		}
	}
	return nil
}

func validNodeID(id string, maxBytes int) bool {
	if id == "" || len(id) > maxBytes || !utf8.ValidString(id) {
		return false
	}
	for index, r := range id {
		if index == 0 {
			if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
				return false
			}
			continue
		}
		if r != '_' && r != '-' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func tarjanComponents(graph *flow.Graph, outgoing [][]int, budget *workBudget) ([]int, error) {
	nodeCount := len(graph.Nodes)
	indices := make([]int, nodeCount)
	lowlink := make([]int, nodeCount)
	componentOf := make([]int, nodeCount)
	onStack := make([]bool, nodeCount)
	for index := range indices {
		indices[index] = -1
		componentOf[index] = -1
	}
	stack := make([]int, 0, nodeCount)
	nextIndex := 0
	componentCount := 0

	var visit func(int) error
	visit = func(node int) error {
		if err := budget.take(); err != nil {
			return err
		}
		indices[node] = nextIndex
		lowlink[node] = nextIndex
		nextIndex++
		stack = append(stack, node)
		onStack[node] = true

		for _, edgeIndex := range outgoing[node] {
			if err := budget.take(); err != nil {
				return err
			}
			child := graph.Edges[edgeIndex].To
			if indices[child] == -1 {
				if err := visit(child); err != nil {
					return err
				}
				if lowlink[child] < lowlink[node] {
					lowlink[node] = lowlink[child]
				}
			} else if onStack[child] && indices[child] < lowlink[node] {
				lowlink[node] = indices[child]
			}
		}

		if lowlink[node] != indices[node] {
			return nil
		}
		for {
			if err := budget.take(); err != nil {
				return err
			}
			last := len(stack) - 1
			member := stack[last]
			stack = stack[:last]
			onStack[member] = false
			componentOf[member] = componentCount
			if member == node {
				break
			}
		}
		componentCount++
		return nil
	}

	for node := range graph.Nodes {
		if err := budget.take(); err != nil {
			return nil, err
		}
		if indices[node] == -1 {
			if err := visit(node); err != nil {
				return nil, err
			}
		}
	}
	return componentOf, nil
}

func normalizeComponents(componentOf []int) []int {
	normalized := make([]int, len(componentOf))
	remap := make(map[int]int)
	next := 0
	for node, component := range componentOf {
		mapped, exists := remap[component]
		if !exists {
			mapped = next
			remap[component] = mapped
			next++
		}
		normalized[node] = mapped
	}
	return normalized
}

func reachableWithinComponent(
	graph *flow.Graph,
	keptOutgoing [][]int,
	componentOf []int,
	start int,
	target int,
	budget *workBudget,
) (bool, error) {
	queue := []int{start}
	seen := make([]bool, len(graph.Nodes))
	seen[start] = true
	component := componentOf[start]
	for head := 0; head < len(queue); head++ {
		if err := budget.take(); err != nil {
			return false, err
		}
		current := queue[head]
		if current == target {
			return true, nil
		}
		for _, edgeIndex := range keptOutgoing[current] {
			if err := budget.take(); err != nil {
				return false, err
			}
			next := graph.Edges[edgeIndex].To
			if componentOf[next] != component || seen[next] {
				continue
			}
			seen[next] = true
			queue = append(queue, next)
		}
	}
	return false, nil
}

func rankForwardGraph(graph *flow.Graph, feedback []bool, budget *workBudget) ([]int, int, error) {
	indegree := make([]int, len(graph.Nodes))
	children := make([][]int, len(graph.Nodes))
	for edgeIndex, edge := range graph.Edges {
		if err := budget.take(); err != nil {
			return nil, 0, err
		}
		if feedback[edgeIndex] {
			continue
		}
		indegree[edge.To]++
		children[edge.From] = append(children[edge.From], edge.To)
	}
	queue := make([]int, 0, len(graph.Nodes))
	for node, degree := range indegree {
		if err := budget.take(); err != nil {
			return nil, 0, err
		}
		if degree == 0 {
			queue = append(queue, node)
		}
	}
	ranks := make([]int, len(graph.Nodes))
	processed := 0
	maxRank := 0
	for head := 0; head < len(queue); head++ {
		if err := budget.take(); err != nil {
			return nil, 0, err
		}
		current := queue[head]
		processed++
		for _, child := range children[current] {
			if err := budget.take(); err != nil {
				return nil, 0, err
			}
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
		return nil, 0, fmt.Errorf("%w: feedback partition retained a cycle", ErrInvalidGraph)
	}
	return ranks, maxRank, nil
}
