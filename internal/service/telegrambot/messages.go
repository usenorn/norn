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

type decisionKind struct {
	mark    string
	open    string
	settled string
}

var (
	kindQuestion    = decisionKind{mark: "❓", open: "Question", settled: "Question"}
	kindPlan        = decisionKind{mark: "📋", open: "Plan to approve", settled: "Plan"}
	kindReview      = decisionKind{mark: "🔍", open: "Changes to review", settled: "Changes"}
	kindPublication = decisionKind{mark: "⚠️", open: "Publication incomplete", settled: "Publication"}
)

func (c decisionContext) header(kind decisionKind, open bool) string {
	label := kind.settled
	if open {
		label = kind.open
	}

	return fmt.Sprintf(
		"%s <b>%s</b> · <a href=\"%s\">%s</a>\n<b>%s</b>\n\n",
		kind.mark, label, html.EscapeString(c.issueURL), html.EscapeString(c.reference),
		escaped(c.title, titleBudget),
	)
}

func (c decisionContext) agent() string {
	return escaped(c.agentName, titleBudget)
}

func questionText(c decisionContext, question entity.IssueQuestion) string {
	var built strings.Builder

	built.WriteString(c.header(kindQuestion, true))
	fmt.Fprintf(&built, "%s asks:\n\n", c.agent())
	built.WriteString(markup(question.Question, questionBudget))
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

	built.WriteString(c.header(kindQuestion, false))
	fmt.Fprintf(&built, "%s asked:\n\n", c.agent())
	built.WriteString(markup(question.Question, questionBudget))
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

	built.WriteString(c.header(kindPlan, true))
	fmt.Fprintf(&built, "%s proposes this plan. Nothing is built until it is approved.\n\n", c.agent())
	built.WriteString(markup(plan.Body, planBudget))
	built.WriteString("\n\n")
	fmt.Fprintf(&built, "<a href=\"%s\">Read the whole plan in Norn</a>", html.EscapeString(c.runURL+planFragment))
	built.WriteString("\n\nApprove the plan here, or reply to this message with what should change.")

	return built.String()
}

func reviewText(c decisionContext, snapshot entity.ExecutionSnapshot) string {
	var built strings.Builder

	built.WriteString(c.header(kindReview, true))
	fmt.Fprintf(&built,
		"%s finished. Approving pushes exactly these commits and opens their pull requests.\n\n",
		c.agent(),
	)

	writeRepositories(&built, snapshot.Repositories)

	if summary := strings.TrimSpace(snapshot.Summary); summary != "" {
		built.WriteString(markup(summary, questionBudget))
		built.WriteString("\n\n")
	}

	fmt.Fprintf(&built, "<a href=\"%s\">Review the changes in Norn</a>", html.EscapeString(c.reviewURL))
	built.WriteString("\n\nApprove and publish here, or reply to this message with what should change.")

	return built.String()
}

func writeRepositories(built *strings.Builder, repositories []entity.SnapshotRepository) {
	if len(repositories) == 0 {
		built.WriteString("No repository changed.\n\n")

		return
	}

	for _, repository := range repositories {
		fmt.Fprintf(built, "%s <b>%s</b>: %s, +%d −%d in %s\n",
			bullet, escaped(repository.Repository, titleBudget),
			counted(len(repository.Commits), "commit", "commits"),
			repository.Additions, repository.Deletions,
			counted(repository.FilesChanged, "file", "files"),
		)
	}

	built.WriteString("\n")
}

func counted(count int, one, many string) string {
	if count == 1 {
		return "1 " + one
	}

	return fmt.Sprintf("%d %s", count, many)
}

func planSettled(
	c decisionContext,
	execution entity.Execution,
	plans []entity.ExecutionPlan,
	revision int,
) (string, bool) {
	var built strings.Builder

	built.WriteString(c.header(kindPlan, false))

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

	built.WriteString(c.header(kindReview, false))

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

func publicationText(c decisionContext, changeset entity.ExecutionChangeSet) string {
	var built strings.Builder

	built.WriteString(c.header(kindPublication, true))
	fmt.Fprintf(&built, "%s could not publish everything that was approved.\n\n", c.agent())
	writePublication(&built, changeset)
	built.WriteString("\n\nRetry publishes only what is left. Give up leaves the run failed.")

	return built.String()
}

func publicationSettled(
	c decisionContext,
	execution entity.Execution,
	changeset entity.ExecutionChangeSet,
) (string, bool) {
	var built strings.Builder

	built.WriteString(c.header(kindPublication, false))

	switch {
	case execution.State == entity.ExecutionCompleted:
		writePublication(&built, changeset)
	case execution.State == entity.ExecutionFailed:
		built.WriteString("Publication was abandoned.")
	case execution.State != entity.ExecutionApproved:
		built.WriteString("These changes are no longer waiting to be published.")
	case !changeset.PublicationFailed():
		built.WriteString("Retrying publication.")
	default:
		return "", false
	}

	return built.String(), true
}

func writePublication(built *strings.Builder, changeset entity.ExecutionChangeSet) {
	for index, change := range changeset.Changes {
		if index > 0 {
			built.WriteString("\n")
		}

		fmt.Fprintf(built, "%s <b>%s</b>: ", bullet, escaped(change.Repository, titleBudget))

		switch change.Publication.State {
		case entity.PublicationPublished:
			fmt.Fprintf(built, "<a href=\"%s\">pull request</a>", html.EscapeString(change.PullRequestURL))
		case entity.PublicationPushed:
			built.WriteString("pushed")
		case entity.PublicationFailed:
			fmt.Fprintf(built, "%s failed: <i>%s</i>",
				publicationStep(change.Publication.Step), escaped(change.Publication.Error, answerBudget))
		default:
			built.WriteString("not published yet")
		}
	}
}

func publicationStep(step entity.PublicationStep) string {
	if step == entity.PublicationStepPullRequest {
		return "opening the pull request"
	}

	return "pushing"
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
