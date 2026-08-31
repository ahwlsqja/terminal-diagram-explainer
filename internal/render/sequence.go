package render

import (
	"errors"
	"fmt"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/sequence"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

const (
	maxSequenceParticipants = 16
	maxSequenceMessages     = 96
	sequenceHeaderGap       = 4
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
