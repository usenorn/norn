package entity

import (
	"strconv"
	"strings"
)

func (d Document) Markdown() string {
	var builder strings.Builder

	writeBlocks(&builder, d.Content, "")

	return strings.TrimRight(collapseBlankLines(builder.String()), "\n")
}

func writeBlocks(builder *strings.Builder, nodes []Node, prefix string) {
	for index, node := range nodes {
		if index > 0 {
			builder.WriteString(prefix)
			builder.WriteString("\n")
		}

		writeBlock(builder, node, prefix)
	}
}

func writeItemBlocks(builder *strings.Builder, nodes []Node, prefix string) {
	for index, node := range nodes {
		if index > 0 && !listed(node) {
			builder.WriteString(prefix)
			builder.WriteString("\n")
		}

		writeBlock(builder, node, prefix)
	}
}

func listed(node Node) bool {
	switch node.Type {
	case NodeBulletList, NodeOrderedList, NodeTaskList:
		return true
	}

	return false
}

func writeBlock(builder *strings.Builder, node Node, prefix string) {
	switch node.Type {
	case NodeParagraph:
		writeLine(builder, prefix, inlineOf(node.Content))

	case NodeHeading:
		level := attrInt(node, "level")
		if level < 1 || level > 6 {
			level = 1
		}

		writeLine(builder, prefix, strings.Repeat("#", level)+" "+inlineOf(node.Content))

	case NodeBulletList:
		writeList(builder, node, prefix, func(int) string { return "- " })

	case NodeOrderedList:
		start := attrInt(node, "start")
		if start == 0 {
			start = 1
		}

		writeList(builder, node, prefix, func(index int) string {
			return strconv.Itoa(start+index) + ". "
		})

	case NodeTaskList:
		writeList(builder, node, prefix, func(int) string { return "- " })

	case NodeListItem, NodeTaskItem:
		writeItemBlocks(builder, node.Content, prefix)

	case NodeBlockquote:
		var quoted strings.Builder

		writeBlocks(&quoted, node.Content, "")

		for _, line := range strings.Split(strings.TrimRight(quoted.String(), "\n"), "\n") {
			writeLine(builder, prefix, strings.TrimRight("> "+line, " "))
		}

	case NodeHorizontalRule:
		writeLine(builder, prefix, "---")

	case NodeCodeBlock:
		language := attrString(node, "language")

		writeLine(builder, prefix, "```"+language)

		for _, line := range strings.Split(textOf(node.Content), "\n") {
			writeVerbatim(builder, prefix, line)
		}

		writeLine(builder, prefix, "```")

	case NodeTable:
		writeTable(builder, node, prefix)

	case NodeDetails:
		writeDetails(builder, node, prefix)

	case NodeImage:
		writeLine(builder, prefix, "!["+attrString(node, "alt")+"]("+attrString(node, "src")+")")

	case NodeAttachment:
		writeLine(builder, prefix, "["+attachmentLabel(node)+"]("+attrString(node, "href")+")")

	default:
		if line := inlineOf([]Node{node}); line != "" {
			writeLine(builder, prefix, line)
		}
	}
}

func writeList(builder *strings.Builder, node Node, prefix string, marker func(int) string) {
	for index, item := range node.Content {
		var written strings.Builder

		writeItemBlocks(&written, item.Content, "")

		lines := strings.Split(strings.TrimRight(written.String(), "\n"), "\n")
		lead := marker(index)

		if item.Type == NodeTaskItem {
			lead += box(attrBool(item, "checked")) + " "
		}

		for at, line := range lines {
			if at == 0 {
				writeLine(builder, prefix, strings.TrimRight(lead+line, " "))

				continue
			}

			writeLine(builder, prefix, strings.TrimRight(strings.Repeat(" ", len(lead))+line, " "))
		}
	}
}

func box(checked bool) string {
	if checked {
		return "[x]"
	}

	return "[ ]"
}

func writeTable(builder *strings.Builder, node Node, prefix string) {
	rows := node.Content
	if len(rows) == 0 {
		return
	}

	for index, row := range rows {
		cells := make([]string, 0, len(row.Content))

		for _, cell := range row.Content {
			cells = append(cells, strings.TrimSpace(inlineOf(flatten(cell.Content))))
		}

		writeLine(builder, prefix, "| "+strings.Join(cells, " | ")+" |")

		if index > 0 || len(rows) == 1 {
			continue
		}

		divider := make([]string, len(cells))
		for at := range divider {
			divider[at] = "---"
		}

		writeLine(builder, prefix, "| "+strings.Join(divider, " | ")+" |")
	}
}

func writeDetails(builder *strings.Builder, node Node, prefix string) {
	summary, body := "", []Node{}

	for _, part := range node.Content {
		switch part.Type {
		case NodeDetailsSummary:
			summary = inlineOf(flatten(part.Content))
		case NodeDetailsContent:
			body = append(body, part.Content...)
		}
	}

	writeLine(builder, prefix, "<details>")
	writeLine(builder, prefix, "<summary>"+summary+"</summary>")
	writeLine(builder, prefix, "")
	writeBlocks(builder, body, prefix)
	writeLine(builder, prefix, "")
	writeLine(builder, prefix, "</details>")
}

func flatten(nodes []Node) []Node {
	var inline []Node

	for _, node := range nodes {
		switch node.Type {
		case NodeText, NodeMention, NodeIssueRef, NodeHardBreak:
			inline = append(inline, node)
		default:
			inline = append(inline, flatten(node.Content)...)
		}
	}

	return inline
}

func inlineOf(nodes []Node) string {
	return markedRun(nodes, nil)
}

func markedRun(nodes []Node, applied []Mark) string {
	var builder strings.Builder

	for at := 0; at < len(nodes); {
		mark, found := nextMark(nodes[at], applied)
		if !found {
			builder.WriteString(leafOf(nodes[at]))

			at++

			continue
		}

		end := at + 1
		for end < len(nodes) && carries(nodes[end], mark) {
			end++
		}

		opened, closed := delimiters(mark)

		builder.WriteString(opened)
		builder.WriteString(markedRun(nodes[at:end], append(slicedMarks(applied), mark)))
		builder.WriteString(closed)

		at = end
	}

	return builder.String()
}

func markOrder() []string {
	return []string{MarkLink, MarkBold, MarkItalic, MarkStrike, MarkCode}
}

func nextMark(node Node, applied []Mark) (Mark, bool) {
	for _, kind := range markOrder() {
		for _, mark := range node.Marks {
			if mark.Type != kind || appliedAlready(applied, mark) {
				continue
			}

			if mark.Type == MarkLink && attr(mark, "href") == "" {
				continue
			}

			return mark, true
		}
	}

	return Mark{}, false
}

func appliedAlready(applied []Mark, mark Mark) bool {
	for _, seen := range applied {
		if sameMark(seen, mark) {
			return true
		}
	}

	return false
}

func carries(node Node, mark Mark) bool {
	for _, held := range node.Marks {
		if sameMark(held, mark) {
			return true
		}
	}

	return false
}

func sameMark(a, b Mark) bool {
	if a.Type != b.Type {
		return false
	}

	if a.Type == MarkLink {
		return attr(a, "href") == attr(b, "href")
	}

	return true
}

func delimiters(mark Mark) (string, string) {
	switch mark.Type {
	case MarkCode:
		return "`", "`"
	case MarkBold:
		return "**", "**"
	case MarkItalic:
		return "_", "_"
	case MarkStrike:
		return "~~", "~~"
	case MarkLink:
		if auto, _ := mark.Attrs["auto"].(bool); auto {
			return "", ""
		}

		return "[", "](" + attr(mark, "href") + ")"
	}

	return "", ""
}

func attr(mark Mark, name string) string {
	value, _ := mark.Attrs[name].(string)

	return value
}

func leafOf(node Node) string {
	switch node.Type {
	case NodeText:
		return node.Text
	case NodeMention:
		return "@" + attrString(node, "label")
	case NodeIssueRef:
		return attrString(node, "reference")
	case NodeHardBreak:
		return "\n"
	case NodeImage:
		return "![" + attrString(node, "alt") + "](" + attrString(node, "src") + ")"
	}

	return inlineOf(node.Content)
}

func textOf(nodes []Node) string {
	var builder strings.Builder

	for _, node := range nodes {
		builder.WriteString(node.Text)
		builder.WriteString(textOf(node.Content))
	}

	return builder.String()
}

func attachmentLabel(node Node) string {
	if name := attrString(node, "fileName"); name != "" {
		return name
	}

	return "attachment"
}

func writeLine(builder *strings.Builder, prefix, line string) {
	for index, part := range strings.Split(line, "\n") {
		if index > 0 {
			builder.WriteString("\n")
		}

		builder.WriteString(prefix)
		builder.WriteString(strings.TrimRight(part, " \t"))
	}

	builder.WriteString("\n")
}

func writeVerbatim(builder *strings.Builder, prefix, line string) {
	builder.WriteString(prefix)
	builder.WriteString(line)
	builder.WriteString("\n")
}

func collapseBlankLines(value string) string {
	for strings.Contains(value, "\n\n\n") {
		value = strings.ReplaceAll(value, "\n\n\n", "\n\n")
	}

	return value
}
