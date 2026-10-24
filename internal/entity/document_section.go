package entity

import (
	"slices"
	"strings"
)

func (d Document) ReplaceSection(heading string, replacement Document) (Document, error) {
	wanted := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(heading), "#"))

	start := -1

	for index, node := range d.Content {
		if node.Type != NodeHeading || headingText(node) != wanted {
			continue
		}

		if start >= 0 {
			return Document{}, ErrDocSectionAmbiguous
		}

		start = index
	}

	if start < 0 {
		return Document{}, ErrDocSectionNotFound
	}

	end := sectionEnd(d.Content, start)

	content := slices.Concat(d.Content[:start], replacement.Content, d.Content[end:])

	return NewDocument(content...), nil
}

func sectionEnd(nodes []Node, start int) int {
	level := attrInt(nodes[start], "level")

	for index := start + 1; index < len(nodes); index++ {
		if nodes[index].Type == NodeHeading && attrInt(nodes[index], "level") <= level {
			return index
		}
	}

	return len(nodes)
}

func headingText(node Node) string {
	return strings.TrimSpace(DocumentText(NewDocument(node)))
}
