package telegrambot

import (
	"fmt"
	"html"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

const (
	deadlineLayout = "2 Jan 2006, 15:04 MST"
	ellipsis       = "…"
	titleBudget    = 200
	questionBudget = 1000
	answerBudget   = 400
	planBudget     = 1500
	replyBudget    = 3000
)

type decisionContext struct {
	agentName  string
	issueID    uuid.UUID
	reference  string
	title      string
	issueURL   string
	reviewsURL string
	runURL     string
	reviewURL  string
}

func (c decisionContext) heading(verb string) string {
	return fmt.Sprintf(
		"<b>%s</b> %s on <a href=\"%s\">%s</a> · %s",
		escaped(c.agentName, titleBudget), verb, html.EscapeString(c.issueURL),
		html.EscapeString(c.reference), escaped(c.title, titleBudget),
	)
}

func questionText(c decisionContext, question entity.IssueQuestion) string {
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

func settledText(c decisionContext, question entity.IssueQuestion) string {
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

func planText(c decisionContext, plan entity.ExecutionPlan) string {
	var built strings.Builder

	built.WriteString(c.heading("proposes a plan"))
	built.WriteString("\n\n")
	built.WriteString(escaped(strings.TrimSpace(plan.Body), planBudget))
	built.WriteString("\n\n")
	fmt.Fprintf(&built, "<a href=\"%s\">Read the whole plan in Norn</a>", html.EscapeString(c.runURL+planFragment))
	built.WriteString("\n\nApprove it here, or reply to this message with what should change.")

	return built.String()
}

func reviewText(c decisionContext, changeset entity.ExecutionChangeSet) string {
	var built strings.Builder

	built.WriteString(c.heading("finished its changes"))
	built.WriteString("\n\n")

	if summary := strings.TrimSpace(changeset.Result.Summary); summary != "" {
		built.WriteString(escaped(summary, questionBudget))
		built.WriteString("\n\n")
	}

	fmt.Fprintf(&built, "<a href=\"%s\">Review the changes in Norn</a>", html.EscapeString(c.reviewURL))
	built.WriteString("\n\nApprove them here, or reply to this message with what should change.")

	return built.String()
}

func planSettled(
	c decisionContext,
	execution entity.Execution,
	plans []entity.ExecutionPlan,
	revision int,
) (string, bool) {
	var built strings.Builder

	built.WriteString(c.heading("proposed a plan"))
	built.WriteString("\n\n")

	latest, _ := entity.LatestPlan(plans)

	for _, plan := range plans {
		if plan.Revision != revision {
			continue
		}

		switch {
		case plan.Approved():
			fmt.Fprintf(&built, "Plan approved by %s.", escaped(nameOr(plan.ApprovedByName), titleBudget))
		case plan.RevisionRequested():
			fmt.Fprintf(&built, "%s asked for changes: <i>%s</i>",
				escaped(nameOr(plan.RevisionRequestedByName), titleBudget),
				escaped(plan.RevisionFeedback, answerBudget))
		case plan.Revision < latest.Revision:
			built.WriteString("A newer revision of this plan replaced it.")
		case execution.State != entity.ExecutionAwaitingPlan:
			built.WriteString("This plan is no longer waiting for approval.")
		default:
			return "", false
		}

		return built.String(), true
	}

	built.WriteString("This plan is no longer waiting for approval.")

	return built.String(), true
}

func reviewSettled(
	c decisionContext,
	execution entity.Execution,
	reviews []entity.ExecutionReview,
	current, sent entity.ReviewHeads,
) (string, bool) {
	var built strings.Builder

	built.WriteString(c.heading("finished its changes"))
	built.WriteString("\n\n")

	for index := len(reviews) - 1; index >= 0; index-- {
		review := reviews[index]
		if review.Verdict == entity.VerdictComment || !review.Heads.Matches(sent) {
			continue
		}

		if review.Verdict == entity.VerdictApprove {
			fmt.Fprintf(&built, "Changes approved by %s.", escaped(nameOr(review.AuthorName), titleBudget))
		} else {
			fmt.Fprintf(&built, "%s requested changes", escaped(nameOr(review.AuthorName), titleBudget))
			if summary := strings.TrimSpace(review.Summary); summary != "" {
				fmt.Fprintf(&built, ": <i>%s</i>", escaped(summary, answerBudget))
			} else {
				built.WriteString(".")
			}
		}

		return built.String(), true
	}

	if execution.State == entity.ExecutionAwaitingReview && current.Matches(sent) {
		return "", false
	}

	built.WriteString("These changes are no longer waiting for review.")

	return built.String(), true
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
