package entity

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	DocPassageMaxRunes = 1500

	docPassageHeadingSeparator = " › "
	docPassageJoint            = "\n\n"
)

type DocPassage struct {
	Position int
	Heading  string
	Body     string
}

type docHeading struct {
	level int
	text  string
}

func (d Doc) Passages() []DocPassage {
	var (
		passages []DocPassage
		trail    []docHeading
		section  []Node
		headed   bool
	)

	flush := func() {
		if len(section) == 0 && !headed {
			return
		}

		heading := d.passageHeading(trail)
		for _, body := range passageBodies(section) {
			passages = append(passages, DocPassage{Position: len(passages), Heading: heading, Body: body})
		}

		section, headed = nil, false
	}

	for _, node := range d.Content.Content {
		if node.Type != NodeHeading {
			section = append(section, node)

			continue
		}

		flush()

		level := attrInt(node, "level")
		for len(trail) > 0 && trail[len(trail)-1].level >= level {
			trail = trail[:len(trail)-1]
		}

		trail = append(trail, docHeading{level: level, text: headingText(node)})
		headed = true
	}

	flush()

	if len(passages) == 0 {
		passages = append(passages, DocPassage{Heading: d.Title})
	}

	return passages
}

func (d Doc) passageHeading(trail []docHeading) string {
	parts := []string{d.Title}

	for _, heading := range trail {
		if heading.text != "" {
			parts = append(parts, heading.text)
		}
	}

	return strings.Join(parts, docPassageHeadingSeparator)
}

func passageBodies(nodes []Node) []string {
	var (
		bodies  []string
		current []string
		runes   int
	)

	emit := func() {
		if len(current) > 0 {
			bodies = append(bodies, strings.Join(current, docPassageJoint))
		}

		current, runes = nil, 0
	}

	for _, node := range nodes {
		block := strings.TrimSpace(NewDocument(node).Markdown())
		if block == "" {
			continue
		}

		for _, piece := range splitRunes(block, DocPassageMaxRunes) {
			size := utf8.RuneCountInString(piece)
			if runes > 0 && runes+utf8.RuneCountInString(docPassageJoint)+size > DocPassageMaxRunes {
				emit()
			}

			current = append(current, piece)
			runes += size
		}
	}

	emit()

	if len(bodies) == 0 {
		return []string{""}
	}

	return bodies
}

func splitRunes(text string, limit int) []string {
	var pieces []string

	for utf8.RuneCountInString(text) > limit {
		runes := []rune(text)
		cut := limit

		for index := limit; index > limit/2; index-- {
			if unicode.IsSpace(runes[index]) {
				cut = index

				break
			}
		}

		pieces = append(pieces, strings.TrimSpace(string(runes[:cut])))
		text = strings.TrimSpace(string(runes[cut:]))
	}

	if text != "" {
		pieces = append(pieces, text)
	}

	return pieces
}
