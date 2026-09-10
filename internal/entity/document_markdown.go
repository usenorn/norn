package entity

import (
	"strconv"
	"strings"
)

// Markdown renders the document as the text every reader outside the editor sees: the API, the
// MCP tools, an export, an email. It is a projection and is never parsed back into the stored
// document, so it is free to spell a toggle or an attachment in the way that reads best.
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
		writeBlocks(builder, node.Content, prefix)

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
			writeLine(builder, prefix, line)
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

		writeBlocks(&written, item.Content, "")

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

// writeDetails spells a toggle as the HTML a markdown reader renders, because markdown has no
// collapsible block of its own and dropping the summary would lose what the block is for.
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
	var builder strings.Builder

	for _, node := range nodes {
		switch node.Type {
		case NodeText:
			builder.WriteString(marked(node))
		case NodeMention:
			builder.WriteString("@" + attrString(node, "label"))
		case NodeIssueRef:
			builder.WriteString(attrString(node, "reference"))
		case NodeHardBreak:
			builder.WriteString("\n")
		case NodeImage:
			builder.WriteString("![" + attrString(node, "alt") + "](" + attrString(node, "src") + ")")
		default:
			builder.WriteString(inlineOf(node.Content))
		}
	}

	return builder.String()
}

// marked wraps text in its marks. Code comes first and swallows the rest: a fenced span is
// literal, so emphasis inside it would be written as the characters themselves anyway.
func marked(node Node) string {
	text := node.Text

	if hasMark(node, MarkCode) {
		return "`" + text + "`"
	}

	if hasMark(node, MarkBold) {
		text = "**" + text + "**"
	}

	if hasMark(node, MarkItalic) {
		text = "_" + text + "_"
	}

	if hasMark(node, MarkStrike) {
		text = "~~" + text + "~~"
	}

	for _, mark := range node.Marks {
		if mark.Type != MarkLink {
			continue
		}

		href, _ := mark.Attrs["href"].(string)
		if href == "" {
			continue
		}

		if auto, _ := mark.Attrs["auto"].(bool); auto {
			text = "<" + text + ">"

			continue
		}

		text = "[" + text + "](" + href + ")"
	}

	return text
}

func hasMark(node Node, kind string) bool {
	for _, mark := range node.Marks {
		if mark.Type == kind {
			return true
		}
	}

	return false
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
