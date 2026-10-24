package entity

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const DocumentType = "doc"

type DocumentLimits struct {
	MaxDepth     int
	MaxNodes     int
	MaxTextRunes int
}

var IssueDocumentLimits = DocumentLimits{MaxDepth: 8, MaxNodes: 5000, MaxTextRunes: IssueDescriptionMaxLen}

var (
	ErrDocumentMalformed = errors.New("document is not in the shape this instance understands")
	ErrDocumentTooDeep   = errors.New("document nests deeper than this instance holds")
	ErrDocumentTooLarge  = errors.New("document is larger than this instance holds")
)

const (
	NodeParagraph      = "paragraph"
	NodeHeading        = "heading"
	NodeBulletList     = "bulletList"
	NodeOrderedList    = "orderedList"
	NodeListItem       = "listItem"
	NodeTaskList       = "taskList"
	NodeTaskItem       = "taskItem"
	NodeBlockquote     = "blockquote"
	NodeHorizontalRule = "horizontalRule"
	NodeCodeBlock      = "codeBlock"
	NodeTable          = "table"
	NodeTableRow       = "tableRow"
	NodeTableHeader    = "tableHeader"
	NodeTableCell      = "tableCell"
	NodeDetails        = "details"
	NodeDetailsSummary = "detailsSummary"
	NodeDetailsContent = "detailsContent"
	NodeImage          = "image"
	NodeAttachment     = "attachment"
	NodeMention        = "mention"
	NodeIssueRef       = "issueRef"
	NodeHardBreak      = "hardBreak"
	NodeText           = "text"
)

const (
	MarkBold   = "bold"
	MarkItalic = "italic"
	MarkStrike = "strike"
	MarkCode   = "code"
	MarkLink   = "link"
)

func DocumentNodes() []string {
	return []string{
		NodeParagraph, NodeHeading, NodeBulletList, NodeOrderedList, NodeListItem,
		NodeTaskList, NodeTaskItem, NodeBlockquote, NodeHorizontalRule, NodeCodeBlock,
		NodeTable, NodeTableRow, NodeTableHeader, NodeTableCell,
		NodeDetails, NodeDetailsSummary, NodeDetailsContent,
		NodeImage, NodeAttachment, NodeMention, NodeIssueRef, NodeHardBreak, NodeText,
	}
}

func DocumentMarks() []string {
	return []string{MarkBold, MarkItalic, MarkStrike, MarkCode, MarkLink}
}

type Document struct {
	Type    string `json:"type"`
	Content []Node `json:"content,omitempty"`
}

type Node struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []Node         `json:"content,omitempty"`
	Marks   []Mark         `json:"marks,omitempty"`
	Text    string         `json:"text,omitempty"`
}

type Mark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

func Described(markdown string, document *Document) (string, Document, error) {
	return DescribedWithin(IssueDocumentLimits, markdown, document)
}

func DescribedWithin(limits DocumentLimits, markdown string, document *Document) (string, Document, error) {
	if document == nil {
		parsed := DocumentFromMarkdown(markdown)

		return parsed.Markdown(), parsed, nil
	}

	if err := ValidateDocument(*document, limits); err != nil {
		return "", Document{}, err
	}

	return document.Markdown(), *document, nil
}

func NewDocument(content ...Node) Document {
	return Document{Type: DocumentType, Content: content}
}

func (d Document) Empty() bool {
	return strings.TrimSpace(d.Markdown()) == ""
}

func (d Document) Encode() ([]byte, error) {
	if d.Type == "" {
		d.Type = DocumentType
	}

	return json.Marshal(d)
}

func DecodeDocument(raw []byte) (Document, error) {
	if len(raw) == 0 {
		return Document{}, nil
	}

	var decoded Document

	if err := json.Unmarshal(raw, &decoded); err != nil {
		return Document{}, ErrDocumentMalformed
	}

	return decoded, nil
}

func ValidateDocument(document Document, limits DocumentLimits) error {
	if document.Type != DocumentType {
		return ErrDocumentMalformed
	}

	size, err := walkDocument(document.Content, 1, limits)
	if err != nil {
		return err
	}

	if size.nodes > limits.MaxNodes || size.textRunes > limits.MaxTextRunes {
		return ErrDocumentTooLarge
	}

	return nil
}

type documentSize struct {
	nodes     int
	textRunes int
}

func walkDocument(nodes []Node, depth int, limits DocumentLimits) (documentSize, error) {
	if depth > limits.MaxDepth {
		return documentSize{}, ErrDocumentTooDeep
	}

	size := documentSize{nodes: len(nodes)}

	for _, node := range nodes {
		if !slices.Contains(DocumentNodes(), node.Type) {
			return documentSize{}, ErrDocumentMalformed
		}

		for _, mark := range node.Marks {
			if !slices.Contains(DocumentMarks(), mark.Type) {
				return documentSize{}, ErrDocumentMalformed
			}
		}

		nested, err := walkDocument(node.Content, depth+1, limits)
		if err != nil {
			return documentSize{}, err
		}

		size.nodes += nested.nodes
		size.textRunes += utf8.RuneCountInString(node.Text) + nested.textRunes
	}

	return size, nil
}

func DocumentMentions(document Document) []CommentMention {
	var (
		found []CommentMention
		seen  = map[string]bool{}
	)

	eachNode(document.Content, func(node Node) {
		if node.Type != NodeMention {
			return
		}

		kind, _ := node.Attrs["kind"].(string)
		id, _ := node.Attrs["id"].(string)
		label, _ := node.Attrs["label"].(string)

		parsed, err := uuid.Parse(id)
		if err != nil || seen[kind+id] {
			return
		}

		seen[kind+id] = true

		mention := CommentMention{Name: label}

		switch kind {
		case string(MentionKindTeam):
			mention.Kind = MentionKindTeam
			mention.TeamID = parsed
		default:
			mention.Kind = MentionKindAccount
			mention.AccountID = parsed
		}

		found = append(found, mention)
	})

	return found
}

func DocumentReferences(document Document) []uuid.UUID {
	var (
		found []uuid.UUID
		seen  = map[uuid.UUID]bool{}
	)

	eachNode(document.Content, func(node Node) {
		if node.Type != NodeIssueRef {
			return
		}

		id, _ := node.Attrs["issueId"].(string)

		parsed, err := uuid.Parse(id)
		if err != nil || seen[parsed] {
			return
		}

		seen[parsed] = true
		found = append(found, parsed)
	})

	return found
}

func DocumentAttachments(document Document) []uuid.UUID {
	var (
		found []uuid.UUID
		seen  = map[uuid.UUID]bool{}
	)

	eachNode(document.Content, func(node Node) {
		if node.Type != NodeAttachment && node.Type != NodeImage {
			return
		}

		id, _ := node.Attrs["attachmentId"].(string)

		parsed, err := uuid.Parse(id)
		if err != nil || seen[parsed] {
			return
		}

		seen[parsed] = true
		found = append(found, parsed)
	})

	return found
}

func DocumentText(document Document) string {
	var builder strings.Builder

	eachNode(document.Content, func(node Node) {
		switch node.Type {
		case NodeText:
			builder.WriteString(node.Text)
		case NodeMention:
			label, _ := node.Attrs["label"].(string)
			builder.WriteString("@" + label)
		case NodeIssueRef:
			reference, _ := node.Attrs["reference"].(string)
			builder.WriteString(reference)
		}
	})

	return builder.String()
}

func eachNode(nodes []Node, visit func(Node)) {
	for _, node := range nodes {
		visit(node)
		eachNode(node.Content, visit)
	}
}

func attrString(node Node, name string) string {
	value, _ := node.Attrs[name].(string)

	return value
}

func attrInt(node Node, name string) int {
	switch value := node.Attrs[name].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	}

	return 0
}

func attrBool(node Node, name string) bool {
	value, _ := node.Attrs[name].(bool)

	return value
}
