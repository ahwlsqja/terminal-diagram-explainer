package render

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ahwlsqja/terminal-diagram-explainer/internal/er"
	"github.com/ahwlsqja/terminal-diagram-explainer/internal/textcell"
)

const (
	maxEREntities                  = 32
	maxERRelationships             = 64
	maxERAttributes                = 192
	maxERAttributesPerEntity       = 32
	maxERTableConstraints          = 64
	maxERTableConstraintsPerEntity = 8
	maxERTableConstraintColumns    = 8
	maxERTableConstraintCells      = 236
)

var ErrInvalidER = errors.New("유효하지 않은 ER diagram")

type erEntityLayout struct {
	box       placement
	portCount int
}

type erRelationshipLayout struct {
	fromY, toY int
	railX      int
}

type erLayout struct {
	entities      []erEntityLayout
	relationships []erRelationshipLayout
	width         int
	diagramHeight int
	totalHeight   int
	legend        []string
}

func ER(diagram *er.Diagram, options Options) (string, error) {
	if options.MaxWidth <= 0 || options.MaxHeight <= 0 || options.MaxWidth > 512 || options.MaxHeight > 512 {
		return "", fmt.Errorf("%w: canvas %dx%d", ErrOutputBounds, options.MaxWidth, options.MaxHeight)
	}
	if err := validateER(diagram); err != nil {
		return "", err
	}
	layout, err := planER(diagram)
	if err != nil {
		return "", err
	}
	if layout.width > options.MaxWidth || layout.totalHeight > options.MaxHeight {
		return "", fmt.Errorf("%w: ER needs %dx%d, limit=%dx%d", ErrOutputBounds, layout.width, layout.totalHeight, options.MaxWidth, options.MaxHeight)
	}
	canvas, err := newCanvas(options.MaxWidth, options.MaxHeight, options.ASCII)
	if err != nil {
		return "", err
	}
	for index, entity := range diagram.Entities {
		if err := drawEREntity(canvas, entity, diagram.Entities, layout.entities[index].box); err != nil {
			return "", err
		}
	}
	for _, current := range layout.entities {
		if current.portCount == 0 {
			continue
		}
		x := current.box.x + current.box.width - 1
		startY := current.box.y + current.box.height - 1
		endY := current.box.y + current.box.height + current.portCount - 1
		if err := canvas.vertical(x, startY, endY, false); err != nil {
			return "", err
		}
	}
	for index, relationship := range diagram.Relationships {
		current := layout.relationships[index]
		from := layout.entities[relationship.From].box
		to := layout.entities[relationship.To].box
		fromX := from.x + from.width - 1
		toX := to.x + to.width - 1
		if err := canvas.horizontal(fromX, current.railX, current.fromY, false); err != nil {
			return "", err
		}
		if err := canvas.vertical(current.railX, current.fromY, current.toY, false); err != nil {
			return "", err
		}
		if err := canvas.horizontal(toX, current.railX, current.toY, false); err != nil {
			return "", err
		}
	}
	for index, relationship := range diagram.Relationships {
		current := layout.relationships[index]
		from := layout.entities[relationship.From].box
		to := layout.entities[relationship.To].box
		if err := canvas.put(from.x+from.width+1, current.fromY, erMarkerGlyph(relationship.FromMarker)); err != nil {
			return "", err
		}
		if err := canvas.put(to.x+to.width+1, current.toY, erMarkerGlyph(relationship.ToMarker)); err != nil {
			return "", err
		}
	}
	output := canvas.String()
	if len(layout.legend) > 0 {
		output += "\n\nrelationships:\n" + strings.Join(layout.legend, "\n")
	}
	return output, nil
}

func validateER(diagram *er.Diagram) error {
	if diagram == nil || len(diagram.Entities) == 0 || len(diagram.Entities) > maxEREntities || len(diagram.Relationships) > maxERRelationships {
		return fmt.Errorf("%w: counts", ErrInvalidER)
	}
	ids := make(map[string]struct{}, len(diagram.Entities))
	labels := make(map[string]struct{}, len(diagram.Entities))
	totalAttributes := 0
	totalConstraints := 0
	for entityIndex, entity := range diagram.Entities {
		if !validNodeID(entity.ID, maxRenderIDBytes) {
			return fmt.Errorf("%w: entity %d ID", ErrInvalidER, entityIndex)
		}
		if _, exists := ids[entity.ID]; exists {
			return fmt.Errorf("%w: duplicate entity ID", ErrInvalidER)
		}
		ids[entity.ID] = struct{}{}
		if _, exists := labels[entity.Label]; exists {
			return fmt.Errorf("%w: duplicate entity label", ErrInvalidER)
		}
		labels[entity.Label] = struct{}{}
		if width, err := textcell.Width(entity.Label); err != nil || width == 0 || width > maxRenderLabelCells {
			return fmt.Errorf("%w: entity %d label", ErrInvalidER, entityIndex)
		}
		if len(entity.Attributes) > maxERAttributesPerEntity {
			return fmt.Errorf("%w: entity %d attributes", ErrInvalidER, entityIndex)
		}
		if len(entity.TableConstraints) > maxERTableConstraintsPerEntity {
			return fmt.Errorf("%w: entity %d table constraints", ErrInvalidER, entityIndex)
		}
		totalAttributes += len(entity.Attributes)
		attributeNames := make(map[string]struct{}, len(entity.Attributes))
		for attributeIndex, attribute := range entity.Attributes {
			if !validNodeID(attribute.Type, maxRenderIDBytes) || !validNodeID(attribute.Name, maxRenderIDBytes) || attribute.Key&^(er.PrimaryKey|er.ForeignKey) != 0 || attribute.Constraint&^(er.Unique|er.NotNull) != 0 {
				return fmt.Errorf("%w: entity %d attribute %d", ErrInvalidER, entityIndex, attributeIndex)
			}
			if _, exists := attributeNames[attribute.Name]; exists {
				return fmt.Errorf("%w: duplicate attribute", ErrInvalidER)
			}
			attributeNames[attribute.Name] = struct{}{}
			if width, err := textcell.Width(er.FormatAttribute(attribute)); err != nil || width == 0 || width > maxRenderLabelCells {
				return fmt.Errorf("%w: attribute text", ErrInvalidER)
			}
		}
		hasAttributePrimaryKey := false
		for _, attribute := range entity.Attributes {
			hasAttributePrimaryKey = hasAttributePrimaryKey || attribute.Key&er.PrimaryKey != 0
		}
		hasTablePrimaryKey := false
		for constraintIndex, constraint := range entity.TableConstraints {
			totalConstraints++
			if constraint.Kind < er.CompositePrimaryKey || constraint.Kind > er.CompositeForeignKey {
				return fmt.Errorf("%w: entity %d table constraint %d kind", ErrInvalidER, entityIndex, constraintIndex)
			}
			if len(constraint.Columns) < 2 || len(constraint.Columns) > maxERTableConstraintColumns {
				return fmt.Errorf("%w: table constraint columns", ErrInvalidER)
			}
			seen := make(map[int]struct{}, len(constraint.Columns))
			for _, column := range constraint.Columns {
				if column < 0 || column >= len(entity.Attributes) {
					return fmt.Errorf("%w: table constraint local column", ErrInvalidER)
				}
				if _, exists := seen[column]; exists {
					return fmt.Errorf("%w: duplicate table constraint column", ErrInvalidER)
				}
				seen[column] = struct{}{}
			}
			if constraint.Kind == er.CompositeForeignKey {
				if constraint.Reference == nil || constraint.Reference.Entity < 0 || constraint.Reference.Entity >= len(diagram.Entities) || len(constraint.Reference.Columns) != len(constraint.Columns) {
					return fmt.Errorf("%w: foreign table constraint reference", ErrInvalidER)
				}
				referenceEntity := diagram.Entities[constraint.Reference.Entity]
				referenceSeen := make(map[int]struct{}, len(constraint.Reference.Columns))
				for _, column := range constraint.Reference.Columns {
					if column < 0 || column >= len(referenceEntity.Attributes) {
						return fmt.Errorf("%w: foreign table constraint reference column", ErrInvalidER)
					}
					if _, exists := referenceSeen[column]; exists {
						return fmt.Errorf("%w: duplicate foreign reference column", ErrInvalidER)
					}
					referenceSeen[column] = struct{}{}
				}
			} else if constraint.Reference != nil {
				return fmt.Errorf("%w: non-foreign table constraint reference", ErrInvalidER)
			}
			if constraint.Kind == er.CompositePrimaryKey {
				if hasTablePrimaryKey || hasAttributePrimaryKey {
					return fmt.Errorf("%w: mixed or duplicate primary key", ErrInvalidER)
				}
				hasTablePrimaryKey = true
			}
			if width, err := textcell.Width(er.FormatEntityTableConstraint(entity, constraint, diagram.Entities)); err != nil || width == 0 || width > maxERTableConstraintCells {
				return fmt.Errorf("%w: table constraint text", ErrInvalidER)
			}
		}
	}
	if totalAttributes > maxERAttributes {
		return fmt.Errorf("%w: total attributes", ErrInvalidER)
	}
	if totalConstraints > maxERTableConstraints {
		return fmt.Errorf("%w: total table constraints", ErrInvalidER)
	}
	for relationshipIndex, relationship := range diagram.Relationships {
		if relationship.From < 0 || relationship.From >= len(diagram.Entities) || relationship.To < 0 || relationship.To >= len(diagram.Entities) {
			return fmt.Errorf("%w: relationship %d endpoint", ErrInvalidER, relationshipIndex)
		}
		if relationship.FromMarker > er.OneOrMany || relationship.ToMarker > er.OneOrMany {
			return fmt.Errorf("%w: relationship %d cardinality", ErrInvalidER, relationshipIndex)
		}
		if width, err := textcell.Width(relationship.Label); err != nil || width == 0 || width > maxRenderLabelCells {
			return fmt.Errorf("%w: relationship %d label", ErrInvalidER, relationshipIndex)
		}
	}
	return nil
}

func planER(diagram *er.Diagram) (erLayout, error) {
	entityCount := len(diagram.Entities)
	parent := make([]int, entityCount)
	for index := range parent {
		parent[index] = index
	}
	var find func(int) int
	find = func(value int) int {
		if parent[value] != value {
			parent[value] = find(parent[value])
		}
		return parent[value]
	}
	union := func(left, right int) {
		leftRoot, rightRoot := find(left), find(right)
		if leftRoot == rightRoot {
			return
		}
		if leftRoot < rightRoot {
			parent[rightRoot] = leftRoot
		} else {
			parent[leftRoot] = rightRoot
		}
	}
	degrees := make([]int, entityCount)
	for _, relationship := range diagram.Relationships {
		union(relationship.From, relationship.To)
		degrees[relationship.From]++
		degrees[relationship.To]++
	}
	components := make([][]int, 0)
	componentIndex := make(map[int]int)
	for entityIndex := range diagram.Entities {
		root := find(entityIndex)
		index, exists := componentIndex[root]
		if !exists {
			index = len(components)
			componentIndex[root] = index
			components = append(components, nil)
		}
		components[index] = append(components[index], entityIndex)
	}

	layout := erLayout{entities: make([]erEntityLayout, entityCount), relationships: make([]erRelationshipLayout, len(diagram.Relationships))}
	componentWidths := make([]int, len(components))
	for component, entities := range components {
		for _, entityIndex := range entities {
			width, err := erEntityWidth(diagram.Entities[entityIndex], diagram.Entities)
			if err != nil {
				return erLayout{}, err
			}
			componentWidths[component] = max(componentWidths[component], width)
		}
	}
	y := 0
	for component, entities := range components {
		for entityOffset, entityIndex := range entities {
			entity := diagram.Entities[entityIndex]
			width, _ := erEntityWidth(entity, diagram.Entities)
			height := erEntityHeight(entity)
			layout.entities[entityIndex] = erEntityLayout{box: placement{x: 0, y: y, width: width, height: height}, portCount: degrees[entityIndex]}
			y += height + degrees[entityIndex]
			if entityOffset+1 < len(entities) {
				y += 2
			}
		}
		if component+1 < len(components) {
			y += 3
		}
	}
	layout.diagramHeight = y

	portCursor := make([]int, entityCount)
	relationCursor := make([]int, len(components))
	for relationshipIndex, relationship := range diagram.Relationships {
		component := componentIndex[find(relationship.From)]
		fromLayout := layout.entities[relationship.From]
		toLayout := layout.entities[relationship.To]
		fromY := fromLayout.box.y + fromLayout.box.height + portCursor[relationship.From]
		portCursor[relationship.From]++
		toY := toLayout.box.y + toLayout.box.height + portCursor[relationship.To]
		portCursor[relationship.To]++
		railX := componentWidths[component] + 4 + relationCursor[component]*2
		relationCursor[component]++
		layout.relationships[relationshipIndex] = erRelationshipLayout{fromY: fromY, toY: toY, railX: railX}
		layout.width = max(layout.width, railX+1)
	}
	for component, width := range componentWidths {
		layout.width = max(layout.width, width)
		if relationCursor[component] > 0 {
			layout.width = max(layout.width, width+2)
		}
	}
	for index, relationship := range diagram.Relationships {
		line := fmt.Sprintf("R%02d %s %s -- %s %s |%s|", index+1, diagram.Entities[relationship.From].ID, erCardinalityText(relationship.FromMarker), erCardinalityText(relationship.ToMarker), diagram.Entities[relationship.To].ID, relationship.Label)
		layout.legend = append(layout.legend, line)
		width, _ := textcell.Width(line)
		layout.width = max(layout.width, width)
	}
	layout.totalHeight = layout.diagramHeight
	if len(layout.legend) > 0 {
		layout.totalHeight += 2 + len(layout.legend)
	}
	return layout, nil
}

func erEntityWidth(entity er.Entity, entities []er.Entity) (int, error) {
	labelWidth, err := textcell.Width(entity.Label)
	if err != nil {
		return 0, err
	}
	width := max(7, labelWidth+4)
	for _, attribute := range entity.Attributes {
		attributeWidth, attrErr := textcell.Width(er.FormatAttribute(attribute))
		if attrErr != nil {
			return 0, attrErr
		}
		width = max(width, attributeWidth+4)
	}
	for _, constraint := range entity.TableConstraints {
		constraintWidth, constraintErr := textcell.Width(er.FormatEntityTableConstraint(entity, constraint, entities))
		if constraintErr != nil {
			return 0, constraintErr
		}
		width = max(width, constraintWidth+4)
	}
	return width, nil
}

func erEntityHeight(entity er.Entity) int {
	height := 4 + len(entity.Attributes) + len(entity.TableConstraints)
	if len(entity.Attributes) > 0 && len(entity.TableConstraints) > 0 {
		height++
	}
	return height
}

func drawEREntity(canvas *canvas, entity er.Entity, entities []er.Entity, current placement) error {
	left, right := current.x, current.x+current.width-1
	top, bottom := current.y, current.y+current.height-1
	topLeft, topRight, bottomLeft, bottomRight, horizontal, vertical, teeLeft, teeRight := "┌", "┐", "└", "┘", "─", "│", "├", "┤"
	if canvas.ascii {
		topLeft, topRight, bottomLeft, bottomRight, horizontal, vertical, teeLeft, teeRight = "+", "+", "+", "+", "-", "|", "+", "+"
	}
	for _, point := range []struct {
		x, y int
		text string
	}{{left, top, topLeft}, {right, top, topRight}, {left, bottom, bottomLeft}, {right, bottom, bottomRight}} {
		if err := canvas.put(point.x, point.y, point.text); err != nil {
			return err
		}
	}
	for x := left + 1; x < right; x++ {
		if err := canvas.put(x, top, horizontal); err != nil {
			return err
		}
		if err := canvas.put(x, top+2, horizontal); err != nil {
			return err
		}
		if len(entity.Attributes) > 0 && len(entity.TableConstraints) > 0 {
			if err := canvas.put(x, top+3+len(entity.Attributes), horizontal); err != nil {
				return err
			}
		}
		if err := canvas.put(x, bottom, horizontal); err != nil {
			return err
		}
	}
	if err := canvas.put(left, top+2, teeLeft); err != nil {
		return err
	}
	if err := canvas.put(right, top+2, teeRight); err != nil {
		return err
	}
	if len(entity.Attributes) > 0 && len(entity.TableConstraints) > 0 {
		divider := top + 3 + len(entity.Attributes)
		if err := canvas.put(left, divider, teeLeft); err != nil {
			return err
		}
		if err := canvas.put(right, divider, teeRight); err != nil {
			return err
		}
	}
	for y := top + 1; y < bottom; y++ {
		if y == top+2 {
			continue
		}
		if err := canvas.put(left, y, vertical); err != nil {
			return err
		}
		if err := canvas.put(right, y, vertical); err != nil {
			return err
		}
	}
	labelWidth, _ := textcell.Width(entity.Label)
	if err := canvas.putText(left+(current.width-labelWidth)/2, top+1, entity.Label); err != nil {
		return err
	}
	for index, attribute := range entity.Attributes {
		if err := canvas.putText(left+2, top+3+index, er.FormatAttribute(attribute)); err != nil {
			return err
		}
	}
	constraintY := top + 3 + len(entity.Attributes)
	if len(entity.Attributes) > 0 && len(entity.TableConstraints) > 0 {
		constraintY++
	}
	for index, constraint := range entity.TableConstraints {
		if err := canvas.putText(left+2, constraintY+index, er.FormatEntityTableConstraint(entity, constraint, entities)); err != nil {
			return err
		}
	}
	return nil
}

func erMarkerGlyph(cardinality er.Cardinality) string {
	return map[er.Cardinality]string{er.ZeroOrOne: "?", er.ExactlyOne: "1", er.ZeroOrMany: "*", er.OneOrMany: "+"}[cardinality]
}

func erCardinalityText(cardinality er.Cardinality) string {
	return map[er.Cardinality]string{er.ZeroOrOne: "0..1", er.ExactlyOne: "1", er.ZeroOrMany: "0..N", er.OneOrMany: "1..N"}[cardinality]
}
