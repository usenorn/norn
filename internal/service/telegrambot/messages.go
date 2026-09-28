package telegrambot

import (
	"fmt"
	"html"
	"strings"
	"unicode/utf8"

	"github.com/usenorn/norn/internal/entity"
)

const (
	deadlineLayout = "2 Jan 2006, 15:04 MST"
	ellipsis       = "…"
	titleBudget    = 200
	questionBudget = 1000
	answerBudget   = 400
	replyBudget    = 3000
)

type questionContext struct {
	agentName string
	reference string
	title     string
	issueURL  string
}

func (c questionContext) heading(verb string) string {
	return fmt.Sprintf(
		"<b>%s</b> %s on <a href=\"%s\">%s</a> · %s",
		escaped(c.agentName, titleBudget), verb, html.EscapeString(c.issueURL),
		html.EscapeString(c.reference), escaped(c.title, titleBudget),
	)
}

func questionText(c questionContext, question entity.IssueQuestion) string {
	var built strings.Builder

	built.WriteString(c.heading("asks"))
	built.WriteString("\n\n")
	built.WriteString(escaped(question.Question, questionBudget))
	built.WriteString("\n\n")

	if question.DefaultAnswer != "" {
		fmt.Fprintf(&built, "If nobody answers by %s, it goes with: <i>%s</i>",
			question.Deadline.UTC().Format(deadlineLayout), escaped(question.DefaultAnswer, answerBudget))
	} else {
		built.WriteString("It waits for an answer.")
	}

	switch {
	case len(question.Options) > 0 && question.AllowFreeText:
		built.WriteString("\n\nTap an option, or reply to this message with your own answer.")
	case len(question.Options) > 0:
		built.WriteString("\n\nTap an option to answer.")
	default:
		built.WriteString("\n\nReply to this message to answer.")
	}

	return built.String()
}

func settledText(c questionContext, question entity.IssueQuestion) string {
	var built strings.Builder

	built.WriteString(c.heading("asked"))
	built.WriteString("\n\n")
	built.WriteString(escaped(question.Question, questionBudget))
	built.WriteString("\n\n")

	switch {
	case question.Answered():
		fmt.Fprintf(&built, "Answered by %s: <i>%s</i>",
			escaped(nameOr(question.AnsweredByName), titleBudget), escaped(question.Answer, answerBudget))
	case question.State == entity.QuestionDismissed:
		fmt.Fprintf(&built, "Dismissed by %s.", escaped(nameOr(question.SettledByName), titleBudget))
	case question.DefaultAnswer != "":
		fmt.Fprintf(&built, "Nobody answered in time, so it went with: <i>%s</i>",
			escaped(question.DefaultAnswer, answerBudget))
	default:
		built.WriteString("Nobody answered in time.")
	}

	return built.String()
}

func replyText(text string) string {
	return escaped(strings.TrimSpace(text), replyBudget)
}

func plain(text string) string {
	return html.EscapeString(text)
}

func escaped(text string, limit int) string {
	budget := limit

	for {
		escaped := html.EscapeString(clipped(text, budget))
		if utf8.RuneCountInString(escaped) <= limit || budget <= 1 {
			return escaped
		}

		budget = budget * 3 / 4
	}
}

func nameOr(name string) string {
	if strings.TrimSpace(name) == "" {
		return "someone"
	}

	return name
}

func clipped(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}

	if limit <= 1 {
		return ellipsis
	}

	return string([]rune(text)[:limit-1]) + ellipsis
}
