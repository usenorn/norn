package telegrambot

import (
	"html"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/usenorn/norn/internal/entity"
)

const (
	heldTag     = "\ue000"
	bullet      = "•"
	listIndent  = "   "
	cellDivider = " · "
)

func markup(markdown string, budget int) string {
	document := entity.DocumentFromMarkdown(literalTags(markdown))

	var (
		built strings.Builder
		spent int
	)

	for index, block := range document.Content {
		rendered := renderBlock(block, "")
		if rendered == "" {
			continue
		}

		length := utf8.RuneCountInString(plainOf(rendered))
		if spent > 0 && spent+length > budget {
			built.WriteString(ellipsis)

			break
		}

		if index > 0 && built.Len() > 0 {
			built.WriteString("\n\n")
		}

		if length > budget {
			built.WriteString(escaped(textOf(block), budget))

			break
		}

		built.WriteString(rendered)
		spent += length
	}

	return strings.ReplaceAll(strings.TrimSpace(built.String()), heldTag, "&lt;")
}

func literalTags(markdown string) string {
	var built strings.Builder

	fenced := false

	for index, line := range strings.Split(markdown, "\n") {
		if index > 0 {
			built.WriteString("\n")
		}

		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
		}

		if fenced || strings.HasPrefix(strings.TrimSpace(line), "```") {
			built.WriteString(line)

			continue
		}

		built.WriteString(outsideCode(line))
	}

	return built.String()
}

func outsideCode(line string) string {
	parts := strings.Split(line, "`")

	for index := 0; index < len(parts); index += 2 {
		parts[index] = strings.ReplaceAll(parts[index], "<", heldTag)
	}

	return strings.Join(parts, "`")
}

func renderBlock(node entity.Node, indent string) string {
	switch node.Type {
	case entity.NodeParagraph:
		return renderInline(node.Content)
	case entity.NodeHeading:
		return "<b>" + renderInline(node.Content) + "</b>"
	case entity.NodeBulletList, entity.NodeTaskList:
		return renderList(node, indent, func(int) string { return bullet })
	case entity.NodeOrderedList:
		return renderList(node, indent, func(position int) string { return strconv.Itoa(position) + "." })
	case entity.NodeBlockquote:
		return "<blockquote>" + renderBlocks(node.Content, indent) + "</blockquote>"
	case entity.NodeCodeBlock:
		return "<pre>" + html.EscapeString(strings.TrimRight(textOf(node), "\n")) + "</pre>"
	case entity.NodeTable:
		return renderTable(node)
	case entity.NodeHorizontalRule, entity.NodeImage, entity.NodeAttachment:
		return ""
	default:
		return html.EscapeString(textOf(node))
	}
}

func renderBlocks(nodes []entity.Node, indent string) string {
	rendered := make([]string, 0, len(nodes))

	for _, node := range nodes {
		if block := renderBlock(node, indent); block != "" {
			rendered = append(rendered, block)
		}
	}

	return strings.Join(rendered, "\n")
}

func renderList(list entity.Node, indent string, marker func(int) string) string {
	lines := make([]string, 0, len(list.Content))

	for index, item := range list.Content {
		lines = append(lines, indent+marker(index+1)+" "+renderBlocks(item.Content, indent+listIndent))
	}

	return strings.Join(lines, "\n")
}

func renderTable(table entity.Node) string {
	rows := make([]string, 0, len(table.Content))

	for _, row := range table.Content {
		cells := make([]string, 0, len(row.Content))

		for _, cell := range row.Content {
			cells = append(cells, renderBlocks(cell.Content, ""))
		}

		rows = append(rows, strings.Join(cells, cellDivider))
	}

	return strings.Join(rows, "\n")
}

func renderInline(nodes []entity.Node) string {
	var built strings.Builder

	for _, node := range nodes {
		switch node.Type {
		case entity.NodeText:
			built.WriteString(marked(html.EscapeString(node.Text), node.Marks))
		case entity.NodeHardBreak:
			built.WriteString("\n")
		default:
			built.WriteString(html.EscapeString(textOf(node)))
		}
	}

	return built.String()
}

func marked(text string, marks []entity.Mark) string {
	for _, mark := range marks {
		switch mark.Type {
		case entity.MarkBold:
			text = "<b>" + text + "</b>"
		case entity.MarkItalic:
			text = "<i>" + text + "</i>"
		case entity.MarkStrike:
			text = "<s>" + text + "</s>"
		case entity.MarkCode:
			text = "<code>" + text + "</code>"
		case entity.MarkLink:
			if href, ok := linkOf(mark); ok {
				text = "<a href=\"" + html.EscapeString(href) + "\">" + text + "</a>"
			}
		}
	}

	return text
}

func linkOf(mark entity.Mark) (string, bool) {
	href, _ := mark.Attrs["href"].(string)

	parsed, err := url.Parse(href)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return "", false
	}

	return href, true
}

func textOf(node entity.Node) string {
	if node.Text != "" || len(node.Content) == 0 {
		return node.Text
	}

	parts := make([]string, 0, len(node.Content))

	for _, child := range node.Content {
		parts = append(parts, textOf(child))
	}

	return strings.Join(parts, " ")
}

func plainOf(rendered string) string {
	var built strings.Builder

	inTag := false

	for _, r := range rendered {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			built.WriteRune(r)
		}
	}

	return html.UnescapeString(built.String())
}
