package entity

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extensionast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

var documentReader = goldmark.New(goldmark.WithExtensions(extension.GFM))

// DocumentFromMarkdown reads text written before this instance stored documents, and text
// written by anything that still speaks markdown — the API, an agent, an import, an email. It
// is the one way in: nothing writes the stored document by hand.
func DocumentFromMarkdown(markdown string) Document {
	source := []byte(markdown)
	root := documentReader.Parser().Parse(text.NewReader(source))

	return NewDocument(blocksFrom(root, source)...)
}

func blocksFrom(parent ast.Node, source []byte) []Node {
	var nodes []Node

	for child := parent.FirstChild(); child != nil; child = child.NextSibling() {
		if converted, ok := blockFrom(child, source); ok {
			nodes = append(nodes, converted)
		}
	}

	return nodes
}

func blockFrom(node ast.Node, source []byte) (Node, bool) {
	switch typed := node.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		return Node{Type: NodeParagraph, Content: inlineFrom(node, source)}, true

	case *ast.Heading:
		return Node{
			Type:    NodeHeading,
			Attrs:   map[string]any{"level": typed.Level},
			Content: inlineFrom(node, source),
		}, true

	case *ast.ThematicBreak:
		return Node{Type: NodeHorizontalRule}, true

	case *ast.FencedCodeBlock:
		return Node{
			Type:    NodeCodeBlock,
			Attrs:   map[string]any{"language": string(typed.Language(source))},
			Content: []Node{{Type: NodeText, Text: linesOf(typed, source)}},
		}, true

	case *ast.CodeBlock:
		return Node{
			Type:    NodeCodeBlock,
			Content: []Node{{Type: NodeText, Text: linesOf(typed, source)}},
		}, true

	case *ast.Blockquote:
		return Node{Type: NodeBlockquote, Content: blocksFrom(node, source)}, true

	case *ast.List:
		return listFrom(typed, source), true

	case *extensionast.Table:
		return tableFrom(typed, source), true

	case *ast.HTMLBlock:
		return htmlBlockFrom(typed, source)
	}

	return Node{}, false
}

// listFrom keeps a task list apart from a plain one. A checklist is a different thing to a
// reader — it is clicked rather than read — and markdown spells both with a dash.
func listFrom(list *ast.List, source []byte) Node {
	var items []Node

	tasks := false

	for child := list.FirstChild(); child != nil; child = child.NextSibling() {
		item := Node{Type: NodeListItem, Content: blocksFrom(child, source)}

		if checked, ok := taskState(child); ok {
			tasks = true
			item.Type = NodeTaskItem
			item.Attrs = map[string]any{"checked": checked}
		}

		items = append(items, item)
	}

	switch {
	case tasks:
		return Node{Type: NodeTaskList, Content: items}
	case list.IsOrdered():
		return Node{
			Type:    NodeOrderedList,
			Attrs:   map[string]any{"start": list.Start},
			Content: items,
		}
	default:
		return Node{Type: NodeBulletList, Content: items}
	}
}

func taskState(item ast.Node) (bool, bool) {
	for block := item.FirstChild(); block != nil; block = block.NextSibling() {
		for inline := block.FirstChild(); inline != nil; inline = inline.NextSibling() {
			if checkbox, ok := inline.(*extensionast.TaskCheckBox); ok {
				return checkbox.IsChecked, true
			}
		}
	}

	return false, false
}

func tableFrom(table *extensionast.Table, source []byte) Node {
	var rows []Node

	for child := table.FirstChild(); child != nil; child = child.NextSibling() {
		header := false

		if _, ok := child.(*extensionast.TableHeader); ok {
			header = true
		}

		var cells []Node

		for cell := child.FirstChild(); cell != nil; cell = cell.NextSibling() {
			kind := NodeTableCell
			if header {
				kind = NodeTableHeader
			}

			cells = append(cells, Node{
				Type:    kind,
				Content: []Node{{Type: NodeParagraph, Content: inlineFrom(cell, source)}},
			})
		}

		rows = append(rows, Node{Type: NodeTableRow, Content: cells})
	}

	return Node{Type: NodeTable, Content: rows}
}

// htmlBlockFrom recognises the one piece of HTML this instance writes itself — the toggle —
// and keeps everything else as the text it is rather than letting markup into the document.
func htmlBlockFrom(block *ast.HTMLBlock, source []byte) (Node, bool) {
	raw := strings.TrimSpace(linesOf(block, source))

	if !strings.HasPrefix(strings.ToLower(raw), "<details") {
		if raw == "" {
			return Node{}, false
		}

		return Node{Type: NodeParagraph, Content: []Node{{Type: NodeText, Text: raw}}}, true
	}

	summary := ""

	if opened := strings.Index(strings.ToLower(raw), "<summary>"); opened >= 0 {
		rest := raw[opened+len("<summary>"):]
		if closed := strings.Index(strings.ToLower(rest), "</summary>"); closed >= 0 {
			summary = strings.TrimSpace(rest[:closed])
		}
	}

	return Node{
		Type: NodeDetails,
		Content: []Node{
			{
				Type:    NodeDetailsSummary,
				Content: []Node{{Type: NodeParagraph, Content: []Node{{Type: NodeText, Text: summary}}}},
			},
			{Type: NodeDetailsContent},
		},
	}, true
}

func inlineFrom(parent ast.Node, source []byte) []Node {
	var nodes []Node

	for child := parent.FirstChild(); child != nil; child = child.NextSibling() {
		nodes = append(nodes, inlineNodes(child, source, nil)...)
	}

	return nodes
}

func inlineNodes(node ast.Node, source []byte, marks []Mark) []Node {
	switch typed := node.(type) {
	case *ast.Text:
		var written []Node

		// A break that follows something other than plain text — a link, a code span — arrives
		// as an empty text node carrying the flag, so emptiness alone is not a reason to skip.
		if value := string(typed.Segment.Value(source)); value != "" {
			written = append(written, Node{Type: NodeText, Text: value, Marks: marks})
		}

		// Norn renders a single newline as a line break, so a wrapped line is something the
		// writer sees rather than source formatting: it survives as a break, not a space.
		if typed.HardLineBreak() || typed.SoftLineBreak() {
			written = append(written, Node{Type: NodeHardBreak})
		}

		return written

	case *ast.String:
		return []Node{{Type: NodeText, Text: string(typed.Value), Marks: marks}}

	case *ast.CodeSpan:
		return []Node{{
			Type:  NodeText,
			Text:  childText(typed, source),
			Marks: append(slicedMarks(marks), Mark{Type: MarkCode}),
		}}

	case *ast.Emphasis:
		kind := MarkItalic
		if typed.Level >= 2 {
			kind = MarkBold
		}

		return childrenWithMark(typed, source, marks, Mark{Type: kind})

	case *extensionast.Strikethrough:
		return childrenWithMark(typed, source, marks, Mark{Type: MarkStrike})

	case *ast.Link:
		return childrenWithMark(typed, source, marks, Mark{
			Type:  MarkLink,
			Attrs: map[string]any{"href": string(typed.Destination)},
		})

	case *extensionast.TaskCheckBox:
		return nil

	case *ast.AutoLink:
		// An address written between angle brackets is a link the writer did not decorate.
		// Keeping that apart from a written-out link is what lets the text come back as it
		// was rather than as a markdown link nobody typed.
		shown := string(typed.Label(source))
		href := string(typed.URL(source))

		return []Node{{
			Type: NodeText,
			Text: shown,
			Marks: append(slicedMarks(marks), Mark{
				Type:  MarkLink,
				Attrs: map[string]any{"href": href, "auto": true},
			}),
		}}

	case *ast.Image:
		return []Node{{
			Type: NodeImage,
			Attrs: map[string]any{
				"src": string(typed.Destination),
				"alt": childText(typed, source),
			},
		}}

	case *ast.RawHTML:
		return nil
	}

	var nodes []Node

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		nodes = append(nodes, inlineNodes(child, source, marks)...)
	}

	return nodes
}

func childrenWithMark(node ast.Node, source []byte, marks []Mark, added Mark) []Node {
	carried := append(slicedMarks(marks), added)

	var nodes []Node

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		nodes = append(nodes, inlineNodes(child, source, carried)...)
	}

	return nodes
}

func slicedMarks(marks []Mark) []Mark {
	return append([]Mark(nil), marks...)
}

func childText(node ast.Node, source []byte) string {
	var builder strings.Builder

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		switch typed := child.(type) {
		case *ast.Text:
			builder.Write(typed.Segment.Value(source))
		case *ast.String:
			builder.Write(typed.Value)
		default:
			builder.WriteString(childText(child, source))
		}
	}

	return builder.String()
}

func linesOf(node ast.Node, source []byte) string {
	var builder strings.Builder

	lines := node.Lines()

	for index := range lines.Len() {
		line := lines.At(index)
		builder.Write(line.Value(source))
	}

	return strings.TrimRight(builder.String(), "\n")
}
