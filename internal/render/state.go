package render

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/state"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

var ErrInvalidState = errors.New("유효하지 않은 state diagram")

func State(d *state.Diagram, o Options) (string, error) {
	if o.MaxWidth <= 0 || o.MaxHeight <= 0 || o.MaxWidth > 512 || o.MaxHeight > 512 {
		return "", fmt.Errorf("%w: canvas", ErrOutputBounds)
	}
	if err := validateState(d); err != nil {
		return "", err
	}
	feedback := classifyStateFeedback(d)
	canvasText, err := renderStateCanvas(d, o)
	if err != nil {
		return "", err
	}
	start, finals := statePseudoLines(d, o.ASCII)
	normal, back := stateLegends(d, feedback)
	var b strings.Builder
	if start != "" {
		b.WriteString(start)
		b.WriteByte('\n')
	}
	b.WriteString(canvasText)
	if len(finals) > 0 {
		b.WriteByte('\n')
		b.WriteString(strings.Join(finals, "\n"))
	}
	if len(normal) > 0 {
		b.WriteString("\n\ntransitions:\n")
		b.WriteString(strings.Join(normal, "\n"))
	}
	if len(back) > 0 {
		b.WriteString("\n\nfeedback:\n")
		b.WriteString(strings.Join(back, "\n"))
	}
	result := b.String()
	w, h := measureStateOutput(result)
	if w > o.MaxWidth || h > o.MaxHeight {
		return "", fmt.Errorf("%w: state needs %dx%d", ErrOutputBounds, w, h)
	}
	return result, nil
}

func renderStateCanvas(d *state.Diagram, o Options) (string, error) {
	if d.Direction == state.LeftRight {
		return renderStateLR(d, o)
	}
	return renderStateTD(d, o)
}
func renderStateTD(d *state.Diagram, o Options) (string, error) {
	width, err := stateBoxWidth(d)
	if err != nil {
		return "", err
	}
	n := stateConnectorLaneCount(d)
	needW := width + 3 + 2*n
	needH := len(d.States)*5 - 1
	if needW > o.MaxWidth || needH > o.MaxHeight {
		return "", fmt.Errorf("%w: TD state needs %dx%d", ErrOutputBounds, needW, needH)
	}
	c, err := newCanvas(o.MaxWidth, o.MaxHeight, o.ASCII)
	if err != nil {
		return "", err
	}
	boxes := make([]placement, len(d.States))
	for i, s := range d.States {
		boxes[i] = placement{x: 0, y: i * 5, width: width, height: 3}
		if err := drawStateBox(c, s, boxes[i]); err != nil {
			return "", err
		}
	}
	lane := 0
	choiceRendered := make([]bool, len(d.States))
	for _, t := range d.Transitions {
		if t.From.Kind != state.StateRef || t.To.Kind != state.StateRef {
			continue
		}
		if d.States[t.From.Index].Kind == state.ChoiceState {
			if choiceRendered[t.From.Index] {
				continue
			}
			choiceRendered[t.From.Index] = true
			x := width + 3 + lane*2
			lane++
			if err := renderStateTDChoiceFanout(c, d, boxes, t.From.Index, x); err != nil {
				return "", err
			}
			continue
		}
		from, to := boxes[t.From.Index], boxes[t.To.Index]
		x := width + 3 + lane*2
		lane++
		sy, ty := from.y+1, to.y+1
		if err := c.horizontal(from.x+from.width, x, sy, false); err != nil {
			return "", err
		}
		if d.States[t.To.Index].Kind == state.ChoiceState {
			if err := renderStateTDChoiceInbound(c, from, to, x); err != nil {
				return "", err
			}
			continue
		}
		if t.From.Index == t.To.Index {
			err = c.vertical(x, sy-1, sy+2, false)
		} else {
			err = c.vertical(x, sy, ty, false)
		}
		if err != nil {
			return "", err
		}
		if err := c.horizontal(to.x+to.width, x, ty, false); err != nil {
			return "", err
		}
		if err := c.arrow(to.x+to.width, ty, left); err != nil {
			return "", err
		}
	}
	return c.String(), nil
}

func renderStateTDChoiceInbound(c *canvas, from, to placement, laneX int) error {
	sourceY := from.y + 1
	targetX := to.x + to.width/2
	portY, approachY, direction := to.y, to.y-1, down
	if sourceY > to.y {
		portY, approachY, direction = to.y+to.height-1, to.y+to.height, up
	}
	if err := c.vertical(laneX, sourceY, approachY, false); err != nil {
		return err
	}
	if err := c.horizontal(targetX, laneX, approachY, false); err != nil {
		return err
	}
	if err := c.vertical(targetX, approachY, portY, false); err != nil {
		return err
	}
	return c.arrow(targetX, portY, direction)
}

func renderStateTDChoiceFanout(c *canvas, d *state.Diagram, boxes []placement, choiceIndex, laneX int) error {
	from := boxes[choiceIndex]
	sourceY := from.y + 1
	minY, maxY := sourceY, sourceY
	for _, transition := range d.Transitions {
		if transition.From.Kind != state.StateRef || transition.From.Index != choiceIndex || transition.To.Kind != state.StateRef {
			continue
		}
		targetY := boxes[transition.To.Index].y + 1
		minY = min(minY, targetY)
		maxY = max(maxY, targetY)
	}
	if err := c.horizontal(from.x+from.width, laneX, sourceY, false); err != nil {
		return err
	}
	if err := c.vertical(laneX, minY, maxY, false); err != nil {
		return err
	}
	for _, transition := range d.Transitions {
		if transition.From.Kind != state.StateRef || transition.From.Index != choiceIndex || transition.To.Kind != state.StateRef {
			continue
		}
		to := boxes[transition.To.Index]
		targetY := to.y + 1
		if err := c.horizontal(to.x+to.width, laneX, targetY, false); err != nil {
			return err
		}
		if err := c.arrow(to.x+to.width, targetY, left); err != nil {
			return err
		}
	}
	return nil
}

func renderStateLR(d *state.Diagram, o Options) (string, error) {
	ws := make([]int, len(d.States))
	total := 0
	for i, s := range d.States {
		w, e := stateSingleBoxWidth(s)
		if e != nil {
			return "", e
		}
		ws[i] = w
		total += w
		if i > 0 {
			total += 5
		}
	}
	n := stateConnectorLaneCount(d)
	needW := total
	for _, transition := range d.Transitions {
		if transition.From.Kind == state.StateRef && transition.To.Kind == state.StateRef && transition.From.Index == transition.To.Index {
			boxRight := 0
			for index := 0; index <= transition.From.Index; index++ {
				boxRight += ws[index]
				if index > 0 {
					boxRight += 5
				}
			}
			needW = max(needW, boxRight+3)
		}
	}
	needH := 3 + 3 + 2*n
	if needW > o.MaxWidth || needH > o.MaxHeight {
		return "", fmt.Errorf("%w: LR state needs %dx%d", ErrOutputBounds, needW, needH)
	}
	c, err := newCanvas(o.MaxWidth, o.MaxHeight, o.ASCII)
	if err != nil {
		return "", err
	}
	boxes := make([]placement, len(d.States))
	x := 0
	for i, s := range d.States {
		boxes[i] = placement{x: x, y: 0, width: ws[i], height: 3}
		if err := drawStateBox(c, s, boxes[i]); err != nil {
			return "", err
		}
		x += ws[i] + 5
	}
	lane := 0
	choiceRendered := make([]bool, len(d.States))
	for _, t := range d.Transitions {
		if t.From.Kind != state.StateRef || t.To.Kind != state.StateRef {
			continue
		}
		if d.States[t.From.Index].Kind == state.ChoiceState {
			if choiceRendered[t.From.Index] {
				continue
			}
			choiceRendered[t.From.Index] = true
			y := 5 + lane*2
			lane++
			if err := renderStateLRChoiceFanout(c, d, boxes, t.From.Index, y); err != nil {
				return "", err
			}
			continue
		}
		from, to := boxes[t.From.Index], boxes[t.To.Index]
		y := 5 + lane*2
		lane++
		sx, tx := from.x+from.width/2, to.x+to.width/2
		if d.States[t.To.Index].Kind == state.ChoiceState {
			if err := renderStateLRChoiceInbound(c, from, to, y); err != nil {
				return "", err
			}
			continue
		}
		if t.From.Index == t.To.Index {
			loopX := from.x + from.width + 2
			if err := c.vertical(sx, from.y+from.height, y, false); err != nil {
				return "", err
			}
			if err := c.horizontal(sx, loopX, y, false); err != nil {
				return "", err
			}
			if err := c.vertical(loopX, from.y+1, y, false); err != nil {
				return "", err
			}
			if err := c.horizontal(from.x+from.width, loopX, from.y+1, false); err != nil {
				return "", err
			}
			if err := c.arrow(from.x+from.width, from.y+1, left); err != nil {
				return "", err
			}
			continue
		}
		if err := c.vertical(sx, from.y+from.height, y, false); err != nil {
			return "", err
		}
		if err := c.horizontal(sx, tx, y, false); err != nil {
			return "", err
		}
		if err := c.vertical(tx, to.y+to.height, y, false); err != nil {
			return "", err
		}
		mark := "▲"
		if o.ASCII {
			mark = "^"
		}
		if err := c.put(tx, to.y+to.height, mark); err != nil {
			return "", err
		}
	}
	return c.String(), nil
}

func renderStateLRChoiceInbound(c *canvas, from, to placement, laneY int) error {
	sourceX := from.x + from.width/2
	portY := to.y + 1
	portX, approachX, direction := to.x-1, to.x-2, right
	if from.x > to.x {
		portX, approachX, direction = to.x+to.width, to.x+to.width+1, left
	}
	if err := c.vertical(sourceX, from.y+from.height, laneY, false); err != nil {
		return err
	}
	if err := c.horizontal(sourceX, approachX, laneY, false); err != nil {
		return err
	}
	if err := c.vertical(approachX, portY, laneY, false); err != nil {
		return err
	}
	if err := c.horizontal(approachX, portX, portY, false); err != nil {
		return err
	}
	return c.arrow(portX, portY, direction)
}

func renderStateLRChoiceFanout(c *canvas, d *state.Diagram, boxes []placement, choiceIndex, laneY int) error {
	from := boxes[choiceIndex]
	sourceX := from.x + from.width/2
	minX, maxX := sourceX, sourceX
	for _, transition := range d.Transitions {
		if transition.From.Kind != state.StateRef || transition.From.Index != choiceIndex || transition.To.Kind != state.StateRef {
			continue
		}
		targetX := boxes[transition.To.Index].x + boxes[transition.To.Index].width/2
		minX = min(minX, targetX)
		maxX = max(maxX, targetX)
	}
	if err := c.vertical(sourceX, from.y+from.height, laneY, false); err != nil {
		return err
	}
	if err := c.horizontal(minX, maxX, laneY, false); err != nil {
		return err
	}
	for _, transition := range d.Transitions {
		if transition.From.Kind != state.StateRef || transition.From.Index != choiceIndex || transition.To.Kind != state.StateRef {
			continue
		}
		to := boxes[transition.To.Index]
		targetX := to.x + to.width/2
		if err := c.vertical(targetX, to.y+to.height, laneY, false); err != nil {
			return err
		}
		if err := c.arrow(targetX, to.y+to.height, up); err != nil {
			return err
		}
	}
	return nil
}

func drawStateBox(c *canvas, s state.State, p placement) error {
	if s.Kind == state.ChoiceState {
		return drawChoiceState(c, s, p)
	}
	tl, tr, bl, br, h, v := "╭", "╮", "╰", "╯", "─", "│"
	if c.ascii {
		tl, tr, bl, br, h, v = "+", "+", "+", "+", "-", "|"
	}
	if err := c.put(p.x, p.y, tl); err != nil {
		return err
	}
	if err := c.put(p.x+p.width-1, p.y, tr); err != nil {
		return err
	}
	if err := c.put(p.x, p.y+2, bl); err != nil {
		return err
	}
	if err := c.put(p.x+p.width-1, p.y+2, br); err != nil {
		return err
	}
	for x := p.x + 1; x < p.x+p.width-1; x++ {
		if err := c.put(x, p.y, h); err != nil {
			return err
		}
		if err := c.put(x, p.y+2, h); err != nil {
			return err
		}
	}
	if err := c.put(p.x, p.y+1, v); err != nil {
		return err
	}
	if err := c.put(p.x+p.width-1, p.y+1, v); err != nil {
		return err
	}
	return c.putText(p.x+2, p.y+1, s.Label)
}

func drawChoiceState(c *canvas, s state.State, p placement) error {
	topLeft, topRight, bottomLeft, bottomRight, horizontal, left, right := "╱", "╲", "╲", "╱", "─", "◁", "▷"
	if c.ascii {
		topLeft, topRight, bottomLeft, bottomRight, horizontal, left, right = "/", "\\", "\\", "/", "-", "<", ">"
	}
	for _, point := range []struct {
		x, y int
		text string
	}{
		{x: p.x + 1, y: p.y, text: topLeft},
		{x: p.x + p.width - 2, y: p.y, text: topRight},
		{x: p.x, y: p.y + 1, text: left},
		{x: p.x + p.width - 1, y: p.y + 1, text: right},
		{x: p.x + 1, y: p.y + 2, text: bottomLeft},
		{x: p.x + p.width - 2, y: p.y + 2, text: bottomRight},
	} {
		if err := c.put(point.x, point.y, point.text); err != nil {
			return err
		}
	}
	for x := p.x + 2; x < p.x+p.width-2; x++ {
		if err := c.put(x, p.y, horizontal); err != nil {
			return err
		}
		if err := c.put(x, p.y+2, horizontal); err != nil {
			return err
		}
	}
	return c.putText(p.x+2, p.y+1, s.Label)
}
func stateBoxWidth(d *state.Diagram) (int, error) {
	r := 0
	for _, s := range d.States {
		w, e := stateSingleBoxWidth(s)
		if e != nil {
			return 0, e
		}
		if w > r {
			r = w
		}
	}
	return r, nil
}
func stateSingleBoxWidth(s state.State) (int, error) {
	w, e := textcell.Width(s.Label)
	return w + 4, e
}
func stateConcreteCount(d *state.Diagram) int {
	n := 0
	for _, t := range d.Transitions {
		if t.From.Kind == state.StateRef && t.To.Kind == state.StateRef {
			n++
		}
	}
	return n
}

func stateConnectorLaneCount(d *state.Diagram) int {
	count := 0
	seenChoice := make([]bool, len(d.States))
	for _, transition := range d.Transitions {
		if transition.From.Kind != state.StateRef || transition.To.Kind != state.StateRef {
			continue
		}
		if d.States[transition.From.Index].Kind == state.ChoiceState {
			if !seenChoice[transition.From.Index] {
				seenChoice[transition.From.Index] = true
				count++
			}
			continue
		}
		count++
	}
	return count
}

func classifyStateFeedback(d *state.Diagram) []bool {
	feedback := make([]bool, len(d.Transitions))
	adj := make([][]int, len(d.States))
	budget := 0
	for i, t := range d.Transitions {
		if t.From.Kind != state.StateRef || t.To.Kind != state.StateRef {
			continue
		}
		if t.From.Index == t.To.Index || stateReaches(adj, t.To.Index, t.From.Index, &budget) {
			feedback[i] = true
			continue
		}
		adj[t.From.Index] = append(adj[t.From.Index], t.To.Index)
	}
	return feedback
}
func stateReaches(adj [][]int, from, target int, budget *int) bool {
	seen := make([]bool, len(adj))
	stack := []int{from}
	seen[from] = true
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if x == target {
			return true
		}
		for _, next := range adj[x] {
			*budget++
			if *budget > 32768 {
				return false
			}
			if !seen[next] {
				seen[next] = true
				stack = append(stack, next)
			}
		}
	}
	return false
}
func stateLegends(d *state.Diagram, feedback []bool) ([]string, []string) {
	normal, back := []string{}, []string{}
	policies := make([][]state.TransitionPolicy, len(d.Transitions))
	for _, policy := range d.Policies {
		policies[policy.TransitionIndex] = append(policies[policy.TransitionIndex], policy)
	}
	for i, t := range d.Transitions {
		if t.From.Kind != state.StateRef || t.To.Kind != state.StateRef {
			continue
		}
		s := fmt.Sprintf("%s --> %s", d.States[t.From.Index].ID, d.States[t.To.Index].ID)
		if t.Label() != "" {
			s += " : " + t.Label()
		}
		lines := []string{s}
		for _, policy := range policies[i] {
			lines = append(lines, "  policy "+policy.Kind.String()+": "+policy.Detail)
		}
		if feedback[i] {
			back = append(back, lines...)
		} else {
			normal = append(normal, lines...)
		}
	}
	return normal, back
}
func statePseudoLines(d *state.Diagram, ascii bool) (string, []string) {
	start, end := "●", "◎"
	if ascii {
		start = "(*)"
		end = "(( ))"
	}
	var first string
	finals := []string{}
	for _, t := range d.Transitions {
		if t.From.Kind == state.Initial {
			first = start + " --> " + d.States[t.To.Index].ID
		}
		if t.To.Kind == state.Final {
			finals = append(finals, d.States[t.From.Index].ID+" --> "+end)
		}
	}
	return first, finals
}

func validateState(d *state.Diagram) error {
	if d == nil || len(d.States) < 1 || len(d.States) > 32 || len(d.Transitions) < 1 || len(d.Transitions) > 64 || len(d.Policies) > 64 || (d.Direction != state.TopDown && d.Direction != state.LeftRight) {
		return fmt.Errorf("%w: diagram", ErrInvalidState)
	}
	ids := map[string]struct{}{}
	labels := map[string]struct{}{}
	for i, s := range d.States {
		if s.Kind != state.OrdinaryState && s.Kind != state.ChoiceState {
			return fmt.Errorf("%w: state %d kind", ErrInvalidState, i)
		}
		if !validStateID(s.ID) {
			return fmt.Errorf("%w: state %d ID", ErrInvalidState, i)
		}
		if _, ok := ids[s.ID]; ok {
			return fmt.Errorf("%w: duplicate ID", ErrInvalidState)
		}
		ids[s.ID] = struct{}{}
		if _, ok := labels[s.Label]; ok {
			return fmt.Errorf("%w: duplicate label", ErrInvalidState)
		}
		labels[s.Label] = struct{}{}
		if strings.ContainsRune(s.Label, '"') {
			return fmt.Errorf("%w: state label", ErrInvalidState)
		}
		if w, e := state.TextCells(s.Label); e != nil || w == 0 || w > 96 {
			return fmt.Errorf("%w: state label", ErrInvalidState)
		}
	}
	initial := 0
	seen := map[string]struct{}{}
	for i, t := range d.Transitions {
		if t.From.Kind < state.StateRef || t.From.Kind > state.Final || t.To.Kind < state.StateRef || t.To.Kind > state.Final {
			return fmt.Errorf("%w: transition %d kind", ErrInvalidState, i)
		}
		for _, e := range []state.Endpoint{t.From, t.To} {
			if e.Kind == state.StateRef && (e.Index < 0 || e.Index >= len(d.States)) {
				return fmt.Errorf("%w: transition endpoint", ErrInvalidState)
			}
			if e.Kind != state.StateRef && e.Index != -1 {
				return fmt.Errorf("%w: pseudo index", ErrInvalidState)
			}
		}
		if t.From.Kind == state.Initial {
			initial++
			if t.To.Kind != state.StateRef {
				return fmt.Errorf("%w: initial orientation", ErrInvalidState)
			}
		}
		if t.To.Kind == state.Final && t.From.Kind != state.StateRef {
			return fmt.Errorf("%w: final orientation", ErrInvalidState)
		}
		if t.From.Kind == state.Final || t.To.Kind == state.Initial {
			return fmt.Errorf("%w: pseudo orientation", ErrInvalidState)
		}
		if (t.From.Kind != state.StateRef || t.To.Kind != state.StateRef) && (t.Event != "" || t.Guard != "") {
			return fmt.Errorf("%w: pseudo label", ErrInvalidState)
		}
		if t.Event != "" || t.Guard != "" {
			if strings.ContainsAny(t.Event, "[]") || strings.ContainsAny(t.Guard, "[]") {
				return fmt.Errorf("%w: transition label", ErrInvalidState)
			}
			w, e := state.TransitionLabelCells(t.Event, t.Guard)
			if t.Event == "" && t.Guard != "" {
				w, e = state.ChoiceGuardCells(t.Guard)
			}
			if e != nil || w == 0 || w > 96 {
				return fmt.Errorf("%w: transition label", ErrInvalidState)
			}
		}
		key := fmt.Sprintf("%d/%d/%d/%d/%s/%s", t.From.Kind, t.From.Index, t.To.Kind, t.To.Index, t.Event, t.Guard)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("%w: duplicate transition", ErrInvalidState)
		}
		seen[key] = struct{}{}
	}
	if initial != 1 {
		return fmt.Errorf("%w: initial count", ErrInvalidState)
	}
	if err := validateChoiceTopology(d); err != nil {
		return err
	}
	seenPolicies := make(map[string]struct{}, len(d.Policies))
	for index, policy := range d.Policies {
		if policy.TransitionIndex < 0 || policy.TransitionIndex >= len(d.Transitions) {
			return fmt.Errorf("%w: policy %d transition", ErrInvalidState, index)
		}
		if policy.Kind < state.RetryPolicy || policy.Kind > state.CompensationPolicy {
			return fmt.Errorf("%w: policy %d kind", ErrInvalidState, index)
		}
		transition := d.Transitions[policy.TransitionIndex]
		if transition.From.Kind != state.StateRef || transition.To.Kind != state.StateRef || transition.Event == "" {
			return fmt.Errorf("%w: policy %d target", ErrInvalidState, index)
		}
		if strings.ContainsRune(transition.Event, '"') || strings.ContainsRune(transition.Guard, '"') {
			return fmt.Errorf("%w: policy %d target label", ErrInvalidState, index)
		}
		if d.States[transition.From.Index].Kind == state.ChoiceState || d.States[transition.To.Index].Kind == state.ChoiceState {
			return fmt.Errorf("%w: policy %d choice target", ErrInvalidState, index)
		}
		if width, err := state.PolicyDetailCells(policy.Detail); err != nil || width == 0 || width > 96 {
			return fmt.Errorf("%w: policy %d detail", ErrInvalidState, index)
		}
		key := fmt.Sprintf("%d/%d", policy.TransitionIndex, policy.Kind)
		if _, exists := seenPolicies[key]; exists {
			return fmt.Errorf("%w: duplicate policy", ErrInvalidState)
		}
		seenPolicies[key] = struct{}{}
	}
	return nil
}

func validateChoiceTopology(d *state.Diagram) error {
	inbound := make([]int, len(d.States))
	outbound := make([]int, len(d.States))
	guards := make([]map[string]struct{}, len(d.States))
	targets := make([]map[int]struct{}, len(d.States))
	for index, transition := range d.Transitions {
		fromChoice := transition.From.Kind == state.StateRef && d.States[transition.From.Index].Kind == state.ChoiceState
		toChoice := transition.To.Kind == state.StateRef && d.States[transition.To.Index].Kind == state.ChoiceState
		switch {
		case fromChoice && toChoice:
			return fmt.Errorf("%w: choice-to-choice transition", ErrInvalidState)
		case fromChoice:
			if transition.To.Kind != state.StateRef || d.States[transition.To.Index].Kind != state.OrdinaryState || transition.Event != "" || transition.Guard == "" || transition.Guard != strings.Trim(transition.Guard, " \t") {
				return fmt.Errorf("%w: choice outbound %d", ErrInvalidState, index)
			}
			if guards[transition.From.Index] == nil {
				guards[transition.From.Index] = make(map[string]struct{})
				targets[transition.From.Index] = make(map[int]struct{})
			}
			if _, exists := guards[transition.From.Index][transition.Guard]; exists {
				return fmt.Errorf("%w: duplicate choice guard", ErrInvalidState)
			}
			if _, exists := targets[transition.From.Index][transition.To.Index]; exists {
				return fmt.Errorf("%w: duplicate choice target", ErrInvalidState)
			}
			guards[transition.From.Index][transition.Guard] = struct{}{}
			targets[transition.From.Index][transition.To.Index] = struct{}{}
			outbound[transition.From.Index]++
			if outbound[transition.From.Index] > 8 {
				return fmt.Errorf("%w: choice outbound limit", ErrInvalidState)
			}
		case toChoice:
			if transition.From.Kind != state.StateRef || d.States[transition.From.Index].Kind != state.OrdinaryState || transition.Guard != "" {
				return fmt.Errorf("%w: choice inbound %d", ErrInvalidState, index)
			}
			inbound[transition.To.Index]++
		case transition.Event == "" && transition.Guard != "":
			return fmt.Errorf("%w: guard-only ordinary transition", ErrInvalidState)
		}
	}
	for index, current := range d.States {
		if current.Kind != state.ChoiceState {
			continue
		}
		if inbound[index] != 1 || outbound[index] < 2 {
			return fmt.Errorf("%w: choice topology %d", ErrInvalidState, index)
		}
	}
	return nil
}

func validStateID(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (i > 0 && c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}
func measureStateOutput(s string) (int, int) {
	lines := strings.Split(s, "\n")
	w := 0
	for _, line := range lines {
		x, _ := textcell.Width(line)
		if x > w {
			w = x
		}
	}
	return w, len(lines)
}
