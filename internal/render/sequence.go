package render

import (
	"errors"
	"fmt"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/sequence"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

const (
	maxSequenceParticipants  = 16
	maxSequenceMessages      = 96
	maxSequenceSteps         = 192
	maxSequenceFragments     = 32
	maxSequenceFragmentDepth = 8
	sequenceHeaderGap        = 4
)

var ErrInvalidSequence = errors.New("유효하지 않은 sequence diagram")

type sequenceParticipantLayout struct {
	center int
	box    placement
}

type sequenceMessageLayout struct {
	labelX, labelY int
	arrowY         int
	railX          int
	topY           int
	self           bool
}

type sequenceLayout struct {
	participants []sequenceParticipantLayout
	messages     []sequenceMessageLayout
	width        int
	height       int
}

func Sequence(diagram *sequence.Diagram, options Options) (string, error) {
	if diagram != nil && diagram.Steps == nil && diagram.Messages != nil {
		return sequenceLegacy(diagram, options)
	}
	if diagram != nil && diagram.Messages == nil && diagram.Steps != nil {
		return sequenceExtended(diagram, options)
	}
	return "", fmt.Errorf("%w: timeline mode", ErrInvalidSequence)
}

func sequenceLegacy(diagram *sequence.Diagram, options Options) (string, error) {
	if options.MaxWidth <= 0 || options.MaxHeight <= 0 || options.MaxWidth > 512 || options.MaxHeight > 512 {
		return "", fmt.Errorf("%w: canvas %dx%d", ErrOutputBounds, options.MaxWidth, options.MaxHeight)
	}
	if err := validateSequence(diagram); err != nil {
		return "", err
	}
	layout, err := planSequence(diagram)
	if err != nil {
		return "", err
	}
	if layout.width > options.MaxWidth || layout.height > options.MaxHeight {
		return "", fmt.Errorf("%w: sequence needs %dx%d, limit=%dx%d", ErrOutputBounds, layout.width, layout.height, options.MaxWidth, options.MaxHeight)
	}
	canvas, err := newCanvas(options.MaxWidth, options.MaxHeight, options.ASCII)
	if err != nil {
		return "", err
	}
	for index, participant := range diagram.Participants {
		if err := drawSequenceParticipant(canvas, participant.Label, layout.participants[index].box); err != nil {
			return "", err
		}
	}
	for _, participant := range layout.participants {
		if err := canvas.vertical(participant.center, 3, layout.height-1, true); err != nil {
			return "", err
		}
	}
	for index, message := range diagram.Messages {
		current := layout.messages[index]
		dashed := message.Kind == sequence.Return
		fromX := layout.participants[message.From].center
		toX := layout.participants[message.To].center
		if current.self {
			if err := canvas.horizontal(fromX, current.railX, current.topY, dashed); err != nil {
				return "", err
			}
			if err := canvas.vertical(current.railX, current.topY, current.arrowY, dashed); err != nil {
				return "", err
			}
			if err := canvas.horizontal(fromX, current.railX, current.arrowY, dashed); err != nil {
				return "", err
			}
			if err := canvas.arrow(fromX, current.arrowY, left); err != nil {
				return "", err
			}
			continue
		}
		if err := canvas.horizontal(fromX, toX, current.arrowY, dashed); err != nil {
			return "", err
		}
		direction := right
		if toX < fromX {
			direction = left
		}
		if err := canvas.arrow(toX, current.arrowY, direction); err != nil {
			return "", err
		}
	}
	for index, message := range diagram.Messages {
		current := layout.messages[index]
		if err := canvas.putText(current.labelX, current.labelY, message.Label); err != nil {
			return "", err
		}
	}
	return canvas.String(), nil
}

type sequenceBranchTrace struct {
	label string
	y     int
}

type sequenceFrameTrace struct {
	kind       sequence.FragmentKind
	label      string
	depth      int
	topY       int
	bottomY    int
	separators []sequenceBranchTrace
}

type sequenceTrace struct {
	messages []sequence.Message
	frames   []sequenceFrameTrace
	depth    int
	height   int
}

type sequenceReplayFrame struct {
	traceIndex     int
	kind           sequence.FragmentKind
	branchMessages int
	sawElse        bool
}

func sequenceExtended(diagram *sequence.Diagram, options Options) (string, error) {
	if options.MaxWidth <= 0 || options.MaxHeight <= 0 || options.MaxWidth > 512 || options.MaxHeight > 512 {
		return "", fmt.Errorf("%w: canvas %dx%d", ErrOutputBounds, options.MaxWidth, options.MaxHeight)
	}
	trace, err := validateSequenceSteps(diagram)
	if err != nil {
		return "", err
	}
	layout, err := planSequenceExtended(diagram, trace)
	if err != nil {
		return "", err
	}
	if layout.width > options.MaxWidth || layout.height > options.MaxHeight {
		return "", fmt.Errorf("%w: sequence needs %dx%d, limit=%dx%d", ErrOutputBounds, layout.width, layout.height, options.MaxWidth, options.MaxHeight)
	}
	canvas, err := newCanvas(options.MaxWidth, options.MaxHeight, options.ASCII)
	if err != nil {
		return "", err
	}
	for index, participant := range diagram.Participants {
		if err := drawSequenceParticipant(canvas, participant.Label, layout.participants[index].box); err != nil {
			return "", err
		}
	}
	for _, participant := range layout.participants {
		if err := canvas.vertical(participant.center, 3, layout.height-1, true); err != nil {
			return "", err
		}
	}
	for _, frame := range trace.frames {
		if err := drawSequenceFrame(canvas, frame, layout.width); err != nil {
			return "", err
		}
	}
	if err := drawSequenceRoutes(canvas, trace.messages, layout); err != nil {
		return "", err
	}
	for _, frame := range trace.frames {
		if err := drawSequenceFrameLabels(canvas, frame); err != nil {
			return "", err
		}
	}
	for index, message := range trace.messages {
		current := layout.messages[index]
		if err := canvas.putText(current.labelX, current.labelY, message.Label); err != nil {
			return "", err
		}
	}
	return canvas.String(), nil
}

func validateSequence(diagram *sequence.Diagram) error {
	if diagram == nil {
		return fmt.Errorf("%w: nil", ErrInvalidSequence)
	}
	if len(diagram.Participants) == 0 || len(diagram.Participants) > maxSequenceParticipants {
		return fmt.Errorf("%w: participant count %d", ErrInvalidSequence, len(diagram.Participants))
	}
	if len(diagram.Messages) == 0 || len(diagram.Messages) > maxSequenceMessages {
		return fmt.Errorf("%w: message count %d", ErrInvalidSequence, len(diagram.Messages))
	}
	ids := make(map[string]struct{}, len(diagram.Participants))
	labels := make(map[string]struct{}, len(diagram.Participants))
	for index, participant := range diagram.Participants {
		if participant.ID == "participant" || !validNodeID(participant.ID, maxRenderIDBytes) {
			return fmt.Errorf("%w: participant %d ID", ErrInvalidSequence, index)
		}
		if _, exists := ids[participant.ID]; exists {
			return fmt.Errorf("%w: duplicate participant ID %q", ErrInvalidSequence, participant.ID)
		}
		ids[participant.ID] = struct{}{}
		if _, exists := labels[participant.Label]; exists {
			return fmt.Errorf("%w: duplicate participant label %q", ErrInvalidSequence, participant.Label)
		}
		labels[participant.Label] = struct{}{}
		width, err := textcell.Width(participant.Label)
		if err != nil || width == 0 || width > maxRenderLabelCells {
			return fmt.Errorf("%w: participant %d label", ErrInvalidSequence, index)
		}
	}
	for index, message := range diagram.Messages {
		if message.From < 0 || message.From >= len(diagram.Participants) || message.To < 0 || message.To >= len(diagram.Participants) {
			return fmt.Errorf("%w: message %d endpoint", ErrInvalidSequence, index)
		}
		if message.Kind != sequence.Request && message.Kind != sequence.Return {
			return fmt.Errorf("%w: message %d kind", ErrInvalidSequence, index)
		}
		width, err := textcell.Width(message.Label)
		if err != nil || width == 0 || width > maxRenderLabelCells {
			return fmt.Errorf("%w: message %d label", ErrInvalidSequence, index)
		}
	}
	return nil
}

func validateSequenceSteps(diagram *sequence.Diagram) (sequenceTrace, error) {
	if len(diagram.Steps) == 0 || len(diagram.Steps) > maxSequenceSteps {
		return sequenceTrace{}, fmt.Errorf("%w: step count %d", ErrInvalidSequence, len(diagram.Steps))
	}
	trace := sequenceTrace{}
	stack := make([]sequenceReplayFrame, 0)
	rowCursor := 0
	fragmentCount := 0
	for stepIndex, step := range diagram.Steps {
		switch step.Kind {
		case sequence.MessageStep:
			if step.Fragment != sequence.LoopFragment || step.Label != "" {
				return sequenceTrace{}, fmt.Errorf("%w: step %d message shape", ErrInvalidSequence, stepIndex)
			}
			trace.messages = append(trace.messages, step.Message)
			for frameIndex := range stack {
				stack[frameIndex].branchMessages++
			}
			if step.Message.From == step.Message.To {
				rowCursor += 4
			} else {
				rowCursor += 2
			}
		case sequence.FragmentStartStep:
			if !isZeroSequenceMessage(step.Message) {
				return sequenceTrace{}, fmt.Errorf("%w: step %d fragment shape", ErrInvalidSequence, stepIndex)
			}
			if step.Fragment != sequence.LoopFragment && step.Fragment != sequence.AltFragment && step.Fragment != sequence.OptFragment {
				return sequenceTrace{}, fmt.Errorf("%w: step %d fragment kind", ErrInvalidSequence, stepIndex)
			}
			if err := validateSequenceControlLabel(step.Label); err != nil {
				return sequenceTrace{}, fmt.Errorf("%w: step %d fragment label", ErrInvalidSequence, stepIndex)
			}
			fragmentCount++
			if fragmentCount > maxSequenceFragments || len(stack) >= maxSequenceFragmentDepth {
				return sequenceTrace{}, fmt.Errorf("%w: fragment limit", ErrInvalidSequence)
			}
			frame := sequenceFrameTrace{
				kind: step.Fragment, label: step.Label,
				depth: len(stack) + 1, topY: 3 + rowCursor,
			}
			trace.frames = append(trace.frames, frame)
			stack = append(stack, sequenceReplayFrame{
				traceIndex: len(trace.frames) - 1,
				kind:       step.Fragment,
			})
			trace.depth = max(trace.depth, len(stack))
			rowCursor++
		case sequence.FragmentBranchStep:
			if !isZeroSequenceMessage(step.Message) || step.Fragment != sequence.LoopFragment {
				return sequenceTrace{}, fmt.Errorf("%w: step %d branch shape", ErrInvalidSequence, stepIndex)
			}
			if len(stack) == 0 {
				return sequenceTrace{}, fmt.Errorf("%w: step %d root branch", ErrInvalidSequence, stepIndex)
			}
			frame := &stack[len(stack)-1]
			if frame.kind != sequence.AltFragment || frame.sawElse || frame.branchMessages == 0 {
				return sequenceTrace{}, fmt.Errorf("%w: step %d invalid branch", ErrInvalidSequence, stepIndex)
			}
			if err := validateSequenceControlLabel(step.Label); err != nil {
				return sequenceTrace{}, fmt.Errorf("%w: step %d branch label", ErrInvalidSequence, stepIndex)
			}
			traceFrame := &trace.frames[frame.traceIndex]
			traceFrame.separators = append(traceFrame.separators, sequenceBranchTrace{label: step.Label, y: 3 + rowCursor})
			frame.branchMessages = 0
			frame.sawElse = true
			rowCursor++
		case sequence.FragmentEndStep:
			if !isZeroSequenceMessage(step.Message) || step.Fragment != sequence.LoopFragment || step.Label != "" {
				return sequenceTrace{}, fmt.Errorf("%w: step %d end shape", ErrInvalidSequence, stepIndex)
			}
			if len(stack) == 0 {
				return sequenceTrace{}, fmt.Errorf("%w: step %d root end", ErrInvalidSequence, stepIndex)
			}
			frame := stack[len(stack)-1]
			if frame.branchMessages == 0 || frame.kind == sequence.AltFragment && !frame.sawElse {
				return sequenceTrace{}, fmt.Errorf("%w: step %d incomplete fragment", ErrInvalidSequence, stepIndex)
			}
			trace.frames[frame.traceIndex].bottomY = 3 + rowCursor
			stack = stack[:len(stack)-1]
			rowCursor++
		default:
			return sequenceTrace{}, fmt.Errorf("%w: step %d kind", ErrInvalidSequence, stepIndex)
		}
	}
	if len(stack) != 0 {
		return sequenceTrace{}, fmt.Errorf("%w: unclosed fragment", ErrInvalidSequence)
	}
	legacyView := &sequence.Diagram{Participants: diagram.Participants, Messages: trace.messages}
	if err := validateSequence(legacyView); err != nil {
		return sequenceTrace{}, err
	}
	trace.height = 3 + rowCursor
	return trace, nil
}

func isZeroSequenceMessage(message sequence.Message) bool {
	return message.From == 0 && message.To == 0 && message.Label == "" && message.Kind == sequence.Request
}

func validateSequenceControlLabel(label string) error {
	width, err := textcell.Width(label)
	if err != nil || width == 0 || width > maxRenderLabelCells {
		return ErrInvalidSequence
	}
	return nil
}

func planSequenceExtended(diagram *sequence.Diagram, trace sequenceTrace) (sequenceLayout, error) {
	layout, err := planSequence(&sequence.Diagram{Participants: diagram.Participants, Messages: trace.messages})
	if err != nil {
		return sequenceLayout{}, err
	}
	messageIndex := 0
	rowCursor := 0
	for _, step := range diagram.Steps {
		switch step.Kind {
		case sequence.MessageStep:
			current := &layout.messages[messageIndex]
			current.labelY = 3 + rowCursor
			if current.self {
				current.topY = current.labelY + 1
				current.arrowY = current.labelY + 3
				rowCursor += 4
			} else {
				current.arrowY = current.labelY + 1
				rowCursor += 2
			}
			messageIndex++
		case sequence.FragmentStartStep, sequence.FragmentBranchStep, sequence.FragmentEndStep:
			rowCursor++
		}
	}
	layout.height = trace.height
	padding := trace.depth*2 + 2
	shiftSequenceLayoutX(&layout, padding)
	layout.width += padding * 2
	for _, frame := range trace.frames {
		depthInset := 2 * (frame.depth - 1)
		titleWidth, _ := textcell.Width(sequenceFrameTitle(frame.kind, frame.label))
		layout.width = max(layout.width, depthInset*2+titleWidth+4)
		for _, separator := range frame.separators {
			labelWidth, _ := textcell.Width("else: " + separator.label)
			layout.width = max(layout.width, depthInset*2+labelWidth+4)
		}
	}
	return layout, nil
}

func shiftSequenceLayoutX(layout *sequenceLayout, dx int) {
	for index := range layout.participants {
		layout.participants[index].center += dx
		layout.participants[index].box.x += dx
	}
	for index := range layout.messages {
		layout.messages[index].labelX += dx
		layout.messages[index].railX += dx
	}
}

func drawSequenceRoutes(canvas *canvas, messages []sequence.Message, layout sequenceLayout) error {
	for index, message := range messages {
		current := layout.messages[index]
		dashed := message.Kind == sequence.Return
		fromX := layout.participants[message.From].center
		toX := layout.participants[message.To].center
		if current.self {
			if err := canvas.horizontal(fromX, current.railX, current.topY, dashed); err != nil {
				return err
			}
			if err := canvas.vertical(current.railX, current.topY, current.arrowY, dashed); err != nil {
				return err
			}
			if err := canvas.horizontal(fromX, current.railX, current.arrowY, dashed); err != nil {
				return err
			}
			if err := canvas.arrow(fromX, current.arrowY, left); err != nil {
				return err
			}
			continue
		}
		if err := canvas.horizontal(fromX, toX, current.arrowY, dashed); err != nil {
			return err
		}
		direction := right
		if toX < fromX {
			direction = left
		}
		if err := canvas.arrow(toX, current.arrowY, direction); err != nil {
			return err
		}
	}
	return nil
}

func drawSequenceFrame(canvas *canvas, frame sequenceFrameTrace, width int) error {
	leftX := 2 * (frame.depth - 1)
	rightX := width - 1 - leftX
	if err := canvas.horizontal(leftX, rightX, frame.topY, false); err != nil {
		return err
	}
	if err := canvas.horizontal(leftX, rightX, frame.bottomY, false); err != nil {
		return err
	}
	if err := canvas.vertical(leftX, frame.topY, frame.bottomY, false); err != nil {
		return err
	}
	if err := canvas.vertical(rightX, frame.topY, frame.bottomY, false); err != nil {
		return err
	}
	if canvas.ascii {
		for _, point := range [][2]int{{leftX, frame.topY}, {rightX, frame.topY}, {leftX, frame.bottomY}, {rightX, frame.bottomY}} {
			if err := canvas.put(point[0], point[1], "+"); err != nil {
				return err
			}
		}
	} else {
		for _, point := range []struct {
			x, y int
			text string
		}{
			{x: leftX, y: frame.topY, text: "┌"},
			{x: rightX, y: frame.topY, text: "┐"},
			{x: leftX, y: frame.bottomY, text: "└"},
			{x: rightX, y: frame.bottomY, text: "┘"},
		} {
			if err := canvas.put(point.x, point.y, point.text); err != nil {
				return err
			}
		}
	}
	for _, separator := range frame.separators {
		if err := canvas.horizontal(leftX, rightX, separator.y, false); err != nil {
			return err
		}
		if canvas.ascii {
			if err := canvas.put(leftX, separator.y, "+"); err != nil {
				return err
			}
			if err := canvas.put(rightX, separator.y, "+"); err != nil {
				return err
			}
		} else {
			if err := canvas.put(leftX, separator.y, "├"); err != nil {
				return err
			}
			if err := canvas.put(rightX, separator.y, "┤"); err != nil {
				return err
			}
		}
	}
	return nil
}

func drawSequenceFrameLabels(canvas *canvas, frame sequenceFrameTrace) error {
	leftX := 2 * (frame.depth - 1)
	if err := canvas.putText(leftX+2, frame.topY, sequenceFrameTitle(frame.kind, frame.label)); err != nil {
		return err
	}
	for _, separator := range frame.separators {
		if err := canvas.putText(leftX+2, separator.y, "else: "+separator.label); err != nil {
			return err
		}
	}
	return nil
}

func sequenceFrameTitle(kind sequence.FragmentKind, label string) string {
	prefix := map[sequence.FragmentKind]string{
		sequence.LoopFragment: "loop: ",
		sequence.AltFragment:  "alt: ",
		sequence.OptFragment:  "opt: ",
	}[kind]
	return prefix + label
}

func planSequence(diagram *sequence.Diagram) (sequenceLayout, error) {
	participantCount := len(diagram.Participants)
	headerWidths := make([]int, participantCount)
	headerHalves := make([]int, participantCount)
	for index, participant := range diagram.Participants {
		width, err := textcell.Width(participant.Label)
		if err != nil {
			return sequenceLayout{}, err
		}
		headerWidths[index] = max(7, width+4)
		if headerWidths[index]%2 == 0 {
			headerWidths[index]++
		}
		headerHalves[index] = headerWidths[index] / 2
	}

	pairLabelWidths := make([][]int, participantCount)
	selfRailOffsets := make([]int, participantCount)
	for index := range pairLabelWidths {
		pairLabelWidths[index] = make([]int, participantCount)
	}
	for _, message := range diagram.Messages {
		labelWidth, _ := textcell.Width(message.Label)
		if message.From == message.To {
			selfRailOffsets[message.From] = max(selfRailOffsets[message.From], max(4, labelWidth+3))
			continue
		}
		leftIndex, rightIndex := message.From, message.To
		if leftIndex > rightIndex {
			leftIndex, rightIndex = rightIndex, leftIndex
		}
		pairLabelWidths[leftIndex][rightIndex] = max(pairLabelWidths[leftIndex][rightIndex], labelWidth)
	}

	centers := make([]int, participantCount)
	centers[0] = headerHalves[0]
	for rightIndex := 1; rightIndex < participantCount; rightIndex++ {
		lowerBound := centers[rightIndex-1] + headerHalves[rightIndex-1] + headerHalves[rightIndex] + sequenceHeaderGap
		if selfRailOffsets[rightIndex-1] > 0 {
			lowerBound = max(lowerBound, centers[rightIndex-1]+selfRailOffsets[rightIndex-1]+headerHalves[rightIndex]+2)
		}
		for leftIndex := 0; leftIndex < rightIndex; leftIndex++ {
			if pairLabelWidths[leftIndex][rightIndex] == 0 {
				continue
			}
			lowerBound = max(lowerBound, centers[leftIndex]+pairLabelWidths[leftIndex][rightIndex]+4)
		}
		centers[rightIndex] = lowerBound
	}

	layout := sequenceLayout{
		participants: make([]sequenceParticipantLayout, participantCount),
		messages:     make([]sequenceMessageLayout, len(diagram.Messages)),
		height:       3,
	}
	for index := range diagram.Participants {
		layout.participants[index] = sequenceParticipantLayout{
			center: centers[index],
			box: placement{
				x: centers[index] - headerHalves[index],
				y: 0, width: headerWidths[index], height: 3,
			},
		}
		layout.width = max(layout.width, centers[index]+headerHalves[index]+1)
	}

	rowCursor := 0
	for index, message := range diagram.Messages {
		labelWidth, _ := textcell.Width(message.Label)
		current := sequenceMessageLayout{labelY: 3 + rowCursor}
		fromX := centers[message.From]
		toX := centers[message.To]
		if message.From == message.To {
			current.self = true
			current.labelX = fromX + 2
			current.railX = fromX + max(4, labelWidth+3)
			current.topY = current.labelY + 1
			current.arrowY = current.labelY + 3
			rowCursor += 4
			layout.width = max(layout.width, max(current.railX+1, current.labelX+labelWidth))
		} else {
			leftX, rightX := fromX, toX
			if leftX > rightX {
				leftX, rightX = rightX, leftX
			}
			current.labelX = leftX + (rightX-leftX-labelWidth)/2
			current.arrowY = current.labelY + 1
			rowCursor += 2
			layout.width = max(layout.width, current.labelX+labelWidth)
		}
		layout.messages[index] = current
	}
	layout.height = 3 + rowCursor
	return layout, nil
}

func drawSequenceParticipant(canvas *canvas, label string, current placement) error {
	if canvas.ascii {
		return drawASCIIBox(canvas, label, current)
	}
	left, right := current.x, current.x+current.width-1
	if err := canvas.put(left, 0, "┌"); err != nil {
		return err
	}
	if err := canvas.put(right, 0, "┐"); err != nil {
		return err
	}
	if err := canvas.put(left, 2, "└"); err != nil {
		return err
	}
	if err := canvas.put(right, 2, "┘"); err != nil {
		return err
	}
	for x := left + 1; x < right; x++ {
		if err := canvas.put(x, 0, "─"); err != nil {
			return err
		}
		if err := canvas.put(x, 2, "─"); err != nil {
			return err
		}
	}
	if err := canvas.put(left, 1, "│"); err != nil {
		return err
	}
	if err := canvas.put(right, 1, "│"); err != nil {
		return err
	}
	labelWidth, _ := textcell.Width(label)
	return canvas.putText(left+(current.width-labelWidth)/2, 1, label)
}
