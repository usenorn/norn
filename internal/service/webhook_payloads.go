package service

import (
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

type WebhookEnvelope struct {
	ID          string       `json:"id"`
	Event       string       `json:"event"`
	OccurredAt  string       `json:"occurredAt"`
	WorkspaceID string       `json:"workspaceId"`
	Actor       WebhookActor `json:"actor"`
	Data        any          `json:"data"`
}

type WebhookActor struct {
	Kind      string `json:"kind"`
	AccountID string `json:"accountId,omitempty"`
	Name      string `json:"name,omitempty"`
}

type WebhookIssuePayload struct {
	ID              string   `json:"id"`
	Reference       string   `json:"reference"`
	Title           string   `json:"title"`
	Description     string   `json:"description,omitempty"`
	TeamID          string   `json:"teamId"`
	TeamKey         string   `json:"teamKey"`
	State           string   `json:"state"`
	StateCategory   string   `json:"stateCategory"`
	Status          string   `json:"status"`
	Priority        string   `json:"priority"`
	AssigneeID      string   `json:"assigneeId,omitempty"`
	Estimate        int      `json:"estimate,omitempty"`
	DueOn           string   `json:"dueOn,omitempty"`
	Labels          []string `json:"labels,omitempty"`
	ProjectID       string   `json:"projectId,omitempty"`
	CycleID         string   `json:"cycleId,omitempty"`
	ParentReference string   `json:"parentReference,omitempty"`
	Version         int      `json:"version"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
}

func WebhookIssue(issue entity.Issue) WebhookIssuePayload {
	payload := WebhookIssuePayload{
		ID:              issue.ID.String(),
		Reference:       issue.Reference(),
		Title:           issue.Title,
		Description:     issue.Description,
		TeamID:          issue.TeamID.String(),
		TeamKey:         issue.TeamKey,
		State:           issue.State.Name,
		StateCategory:   string(issue.State.Category),
		Status:          string(issue.Status),
		Priority:        string(issue.Priority),
		Estimate:        issue.Estimate,
		DueOn:           issue.DueOn,
		ParentReference: issue.ParentReference,
		Version:         issue.Version,
		CreatedAt:       stamp(issue.CreatedAt),
		UpdatedAt:       stamp(issue.UpdatedAt),
	}

	if issue.AssigneeAccountID != uuid.Nil {
		payload.AssigneeID = issue.AssigneeAccountID.String()
	}

	if issue.ProjectID != uuid.Nil {
		payload.ProjectID = issue.ProjectID.String()
	}

	if issue.CycleID != uuid.Nil {
		payload.CycleID = issue.CycleID.String()
	}

	for _, label := range issue.Labels {
		payload.Labels = append(payload.Labels, label.Name)
	}

	return payload
}

type WebhookCommentPayload struct {
	ID              string `json:"id"`
	IssueID         string `json:"issueId"`
	IssueReference  string `json:"issueReference"`
	AuthorID        string `json:"authorId,omitempty"`
	AuthorName      string `json:"authorName,omitempty"`
	Body            string `json:"body"`
	ParentCommentID string `json:"parentCommentId,omitempty"`
	CreatedAt       string `json:"createdAt"`
	EditedAt        string `json:"editedAt,omitempty"`
}

func WebhookComment(comment entity.IssueComment, issue entity.Issue) WebhookCommentPayload {
	payload := WebhookCommentPayload{
		ID:             comment.ID.String(),
		IssueID:        issue.ID.String(),
		IssueReference: issue.Reference(),
		AuthorName:     comment.AuthorName,
		Body:           comment.Body,
		CreatedAt:      stamp(comment.CreatedAt),
	}

	if comment.AuthorAccountID != uuid.Nil {
		payload.AuthorID = comment.AuthorAccountID.String()
	}

	if comment.ParentCommentID != uuid.Nil {
		payload.ParentCommentID = comment.ParentCommentID.String()
	}

	if comment.EditedAt != nil {
		payload.EditedAt = stamp(*comment.EditedAt)
	}

	return payload
}

type WebhookProjectPayload struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	State       string `json:"state"`
	Health      string `json:"health,omitempty"`
	LeadID      string `json:"leadId,omitempty"`
	TargetOn    string `json:"targetOn,omitempty"`
	Archived    bool   `json:"archived"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

func WebhookProject(project entity.Project) WebhookProjectPayload {
	payload := WebhookProjectPayload{
		ID:          project.ID.String(),
		Slug:        project.Slug,
		Name:        project.Name,
		Description: project.Description,
		State:       string(project.State),
		Health:      string(project.Health),
		TargetOn:    project.TargetOn,
		Archived:    project.ArchivedAt != nil,
		CreatedAt:   stamp(project.CreatedAt),
		UpdatedAt:   stamp(project.UpdatedAt),
	}

	if project.LeadAccountID != uuid.Nil {
		payload.LeadID = project.LeadAccountID.String()
	}

	return payload
}

type WebhookCyclePayload struct {
	ID       string `json:"id"`
	TeamID   string `json:"teamId"`
	TeamKey  string `json:"teamKey"`
	Number   int    `json:"number"`
	StartsOn string `json:"startsOn"`
	EndsOn   string `json:"endsOn"`
	ClosedAt string `json:"closedAt,omitempty"`
}

func WebhookCycle(cycle entity.Cycle) WebhookCyclePayload {
	payload := WebhookCyclePayload{
		ID:       cycle.ID.String(),
		TeamID:   cycle.TeamID.String(),
		TeamKey:  cycle.TeamKey,
		Number:   cycle.Number,
		StartsOn: cycle.StartsOn,
		EndsOn:   cycle.EndsOn,
	}

	if cycle.ClosedAt != nil {
		payload.ClosedAt = stamp(*cycle.ClosedAt)
	}

	return payload
}

type WebhookMembershipPayload struct {
	AccountID string `json:"accountId"`
	Role      string `json:"role"`
	Source    string `json:"source,omitempty"`
	TeamID    string `json:"teamId,omitempty"`
	TeamKey   string `json:"teamKey,omitempty"`
}

func WebhookMembership(membership entity.Membership) WebhookMembershipPayload {
	return WebhookMembershipPayload{
		AccountID: membership.AccountID.String(),
		Role:      string(membership.Role),
		Source:    string(membership.Source),
	}
}

func WebhookTeamMembership(accountID uuid.UUID, team entity.Team) WebhookMembershipPayload {
	return WebhookMembershipPayload{
		AccountID: accountID.String(),
		TeamID:    team.ID.String(),
		TeamKey:   team.Key,
	}
}

type WebhookTestPayload struct {
	WorkspaceID string `json:"workspaceId"`
	WebhookID   string `json:"webhookId"`
	Name        string `json:"name"`
	Message     string `json:"message"`
}

func WebhookTest(hook entity.Webhook) WebhookTestPayload {
	return WebhookTestPayload{
		WorkspaceID: hook.WorkspaceID.String(),
		WebhookID:   hook.ID.String(),
		Name:        hook.Name,
		Message:     "This is a test delivery from Norn. Nothing in your workspace changed.",
	}
}

func stamp(at time.Time) string {
	if at.IsZero() {
		return ""
	}

	return at.UTC().Format(time.RFC3339)
}
