package mcpserver

import (
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type workspaceDTO struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

func workspaceDTOFrom(workspace entity.Workspace) workspaceDTO {
	return workspaceDTO{
		ID:   workspace.ID.String(),
		Slug: workspace.Slug,
		Name: workspace.Name,
	}
}

type stateDTO struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	IsDefault    bool   `json:"isDefault"`
	IsCompletion bool   `json:"isCompletion"`
}

func stateDTOFrom(state entity.WorkflowState) stateDTO {
	return stateDTO{
		ID:           state.ID.String(),
		Name:         state.Name,
		Category:     string(state.Category),
		IsDefault:    state.IsDefault,
		IsCompletion: state.IsCompletion,
	}
}

type teamDTO struct {
	ID         string     `json:"id"`
	Key        string     `json:"key"`
	Name       string     `json:"name"`
	Visibility string     `json:"visibility"`
	States     []stateDTO `json:"states,omitempty"`
}

type labelDTO struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Color  string `json:"color"`
	TeamID string `json:"teamId,omitempty"`
}

func labelDTOFrom(label entity.Label) labelDTO {
	dto := labelDTO{
		ID:    label.ID.String(),
		Name:  label.Name,
		Color: string(label.Color),
	}

	if label.TeamID != uuid.Nil {
		dto.TeamID = label.TeamID.String()
	}

	return dto
}

type memberDTO struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	Kind        string `json:"kind"`
}

func memberDTOFrom(member entity.WorkspaceMember) memberDTO {
	return memberDTO{
		AccountID:   member.Membership.AccountID.String(),
		DisplayName: member.DisplayName,
		Email:       member.Email,
		Role:        string(member.Membership.Role),
		Kind:        string(member.AccountKind),
	}
}

type issueDTO struct {
	ID              string     `json:"id"`
	Reference       string     `json:"reference"`
	Title           string     `json:"title"`
	Description     string     `json:"description,omitempty"`
	TeamID          string     `json:"teamId"`
	TeamKey         string     `json:"teamKey"`
	State           stateRef   `json:"state"`
	Status          string     `json:"status"`
	Priority        string     `json:"priority"`
	AssigneeID      string     `json:"assigneeId,omitempty"`
	Estimate        int        `json:"estimate,omitempty"`
	DueOn           string     `json:"dueOn,omitempty"`
	CycleID         string     `json:"cycleId,omitempty"`
	CycleNumber     int        `json:"cycleNumber,omitempty"`
	ProjectID       string     `json:"projectId,omitempty"`
	ProjectName     string     `json:"projectName,omitempty"`
	ParentReference string     `json:"parentReference,omitempty"`
	Blocked         bool       `json:"blocked"`
	Labels          []labelDTO `json:"labels,omitempty"`
	Version         int        `json:"version"`
	CreatedAt       string     `json:"createdAt"`
	UpdatedAt       string     `json:"updatedAt"`
}

type stateRef struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

func issueDTOFrom(issue entity.Issue) issueDTO {
	dto := issueDTO{
		ID:              issue.ID.String(),
		Reference:       issue.Reference(),
		Title:           issue.Title,
		Description:     issue.Description,
		TeamID:          issue.TeamID.String(),
		TeamKey:         issue.TeamKey,
		Status:          string(issue.Status),
		Priority:        string(issue.Priority),
		Estimate:        issue.Estimate,
		DueOn:           issue.DueOn,
		ProjectName:     issue.ProjectName,
		ParentReference: issue.ParentReference,
		Blocked:         issue.Blocked,
		Version:         issue.Version,
		CreatedAt:       issue.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       issue.UpdatedAt.Format(time.RFC3339),
		State: stateRef{
			ID:       issue.State.ID.String(),
			Name:     issue.State.Name,
			Category: string(issue.State.Category),
		},
	}

	if issue.AssigneeAccountID != uuid.Nil {
		dto.AssigneeID = issue.AssigneeAccountID.String()
	}

	if issue.CycleID != uuid.Nil {
		dto.CycleID = issue.CycleID.String()
		dto.CycleNumber = issue.CycleNumber
	}

	if issue.ProjectID != uuid.Nil {
		dto.ProjectID = issue.ProjectID.String()
	}

	for _, label := range issue.Labels {
		dto.Labels = append(dto.Labels, labelDTOFrom(label))
	}

	return dto
}

func issueDTOsFrom(issues []entity.Issue) []issueDTO {
	dtos := make([]issueDTO, 0, len(issues))

	for _, issue := range issues {
		dtos = append(dtos, issueDTOFrom(issue))
	}

	return dtos
}

type replyDTO struct {
	ID         string `json:"id"`
	AuthorName string `json:"authorName"`
	AuthorKind string `json:"authorKind"`
	Body       string `json:"body"`
	CreatedAt  string `json:"createdAt"`
	EditedAt   string `json:"editedAt,omitempty"`
}

type commentDTO struct {
	ID              string     `json:"id"`
	AuthorName      string     `json:"authorName"`
	AuthorKind      string     `json:"authorKind"`
	Body            string     `json:"body"`
	ParentCommentID string     `json:"parentCommentId,omitempty"`
	CreatedAt       string     `json:"createdAt"`
	EditedAt        string     `json:"editedAt,omitempty"`
	Replies         []replyDTO `json:"replies,omitempty"`
}

func replyDTOFrom(comment entity.IssueComment) replyDTO {
	dto := replyDTO{
		ID:         comment.ID.String(),
		AuthorName: comment.AuthorName,
		AuthorKind: string(comment.AuthorKind),
		Body:       comment.Body,
		CreatedAt:  comment.CreatedAt.Format(time.RFC3339),
	}

	if comment.EditedAt != nil {
		dto.EditedAt = comment.EditedAt.Format(time.RFC3339)
	}

	return dto
}

func commentDTOFrom(comment entity.IssueComment) commentDTO {
	dto := commentDTO{
		ID:         comment.ID.String(),
		AuthorName: comment.AuthorName,
		AuthorKind: string(comment.AuthorKind),
		Body:       comment.Body,
		CreatedAt:  comment.CreatedAt.Format(time.RFC3339),
	}

	if comment.ParentCommentID != uuid.Nil {
		dto.ParentCommentID = comment.ParentCommentID.String()
	}

	if comment.EditedAt != nil {
		dto.EditedAt = comment.EditedAt.Format(time.RFC3339)
	}

	for _, reply := range comment.Replies {
		dto.Replies = append(dto.Replies, replyDTOFrom(reply))
	}

	return dto
}

type projectStatusDTO struct {
	Health     string `json:"health"`
	Body       string `json:"body"`
	AuthorName string `json:"authorName"`
	CreatedAt  string `json:"createdAt"`
}

func projectStatusDTOFrom(update entity.ProjectStatusUpdate) projectStatusDTO {
	return projectStatusDTO{
		Health:     string(update.Health),
		Body:       update.Body,
		AuthorName: update.AuthorName,
		CreatedAt:  update.CreatedAt.Format(time.RFC3339),
	}
}

type projectDTO struct {
	ID           string            `json:"id"`
	Slug         string            `json:"slug"`
	Name         string            `json:"name"`
	Description  string            `json:"description,omitempty"`
	State        string            `json:"state"`
	Health       string            `json:"health,omitempty"`
	LeadName     string            `json:"leadName,omitempty"`
	TargetOn     string            `json:"targetOn,omitempty"`
	Archived     bool              `json:"archived"`
	LatestStatus *projectStatusDTO `json:"latestStatus,omitempty"`
}

func projectDTOFrom(view service.ProjectView) projectDTO {
	project := view.Project

	dto := projectDTO{
		ID:          project.ID.String(),
		Slug:        project.Slug,
		Name:        project.Name,
		Description: project.Description,
		State:       string(project.State),
		Health:      string(project.Health),
		LeadName:    project.LeadName,
		TargetOn:    project.TargetOn,
		Archived:    project.ArchivedAt != nil,
	}

	if view.LatestStatus != nil {
		status := projectStatusDTOFrom(*view.LatestStatus)
		dto.LatestStatus = &status
	}

	return dto
}

type cycleDTO struct {
	ID       string `json:"id"`
	TeamID   string `json:"teamId"`
	TeamKey  string `json:"teamKey"`
	Number   int    `json:"number"`
	StartsOn string `json:"startsOn"`
	EndsOn   string `json:"endsOn"`
	Phase    string `json:"phase"`
	ClosedAt string `json:"closedAt,omitempty"`
}

func cycleDTOFrom(view service.CycleView) cycleDTO {
	cycle := view.Cycle

	dto := cycleDTO{
		ID:       cycle.ID.String(),
		TeamID:   cycle.TeamID.String(),
		TeamKey:  cycle.TeamKey,
		Number:   cycle.Number,
		StartsOn: cycle.StartsOn,
		EndsOn:   cycle.EndsOn,
		Phase:    string(view.Phase),
	}

	if cycle.ClosedAt != nil {
		dto.ClosedAt = cycle.ClosedAt.Format(time.RFC3339)
	}

	return dto
}

type searchResultDTO struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Title     string `json:"title"`
	Excerpt   string `json:"excerpt,omitempty"`
	Reference string `json:"reference,omitempty"`
	TeamKey   string `json:"teamKey,omitempty"`
	Slug      string `json:"slug,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

func searchResultDTOFrom(result entity.SearchResult) searchResultDTO {
	dto := searchResultDTO{
		Kind:      string(result.Kind),
		ID:        result.ID.String(),
		Title:     result.Title,
		Excerpt:   result.Excerpt,
		Reference: result.Reference,
		TeamKey:   result.TeamKey,
		Slug:      result.Slug,
	}

	if !result.UpdatedAt.IsZero() {
		dto.UpdatedAt = result.UpdatedAt.Format(time.RFC3339)
	}

	return dto
}

type checkDTO struct {
	ID                   string        `json:"id"`
	Statement            string        `json:"statement"`
	Method               string        `json:"method"`
	Proof                string        `json:"proof"`
	State                string        `json:"state"`
	Approval             string        `json:"approval"`
	Blocking             bool          `json:"blocking"`
	Awaiting             string        `json:"awaiting,omitempty"`
	AwaitingBecause      string        `json:"awaitingBecause,omitempty"`
	Resolution           string        `json:"resolution,omitempty"`
	ResolutionReason     string        `json:"resolutionReason,omitempty"`
	TimeLimitSeconds     int           `json:"timeLimitSeconds"`
	AddedAfterDelegation bool          `json:"addedAfterDelegation,omitempty"`
	AuthoredBy           string        `json:"authoredBy"`
	Evidence             []evidenceDTO `json:"evidence"`
}

func checkDTOFrom(report entity.CheckReport) checkDTO {
	dto := checkDTO{
		ID:                   report.Check.ID.String(),
		Statement:            report.Check.Statement,
		Method:               string(report.Check.Method),
		Proof:                report.Check.Proof,
		State:                string(report.State),
		Approval:             string(report.Check.Approval),
		Blocking:             report.Blocks(),
		Awaiting:             string(report.Awaiting()),
		AwaitingBecause:      awaitingLine(report),
		TimeLimitSeconds:     int(report.Check.Window() / time.Second),
		AddedAfterDelegation: report.Check.AddedAfterDelegation,
		AuthoredBy:           string(report.Check.AuthorKind),
		Evidence:             make([]evidenceDTO, 0, len(report.Evidence)),
	}

	if report.Check.Resolved() {
		dto.Resolution = string(report.Check.Resolution)
		dto.ResolutionReason = report.Check.ResolutionReason
	}

	for _, record := range report.Evidence {
		dto.Evidence = append(dto.Evidence, evidenceDTOFrom(record))
	}

	return dto
}

func checkDTOs(reports []entity.CheckReport) []checkDTO {
	dtos := make([]checkDTO, 0, len(reports))

	for _, report := range reports {
		dtos = append(dtos, checkDTOFrom(report))
	}

	return dtos
}

type evidenceDTO struct {
	ID              string    `json:"id"`
	Verdict         string    `json:"verdict"`
	Channel         string    `json:"channel"`
	Command         string    `json:"command,omitempty"`
	Output          string    `json:"output"`
	OutputTruncated bool      `json:"outputTruncated,omitempty"`
	Redactions      int       `json:"redactions,omitempty"`
	ExitCode        *int      `json:"exitCode,omitempty"`
	SubmittedBy     string    `json:"submittedBy"`
	ObservedAt      time.Time `json:"observedAt"`
	ReceivedAt      time.Time `json:"receivedAt"`
	Expired         bool      `json:"expired,omitempty"`
	ExpiredBecause  string    `json:"expiredBecause,omitempty"`
	Bearing         string    `json:"bearing"`
}

func evidenceDTOFrom(record entity.EvidenceRecord) evidenceDTO {
	dto := evidenceDTO{
		ID:              record.Evidence.ID.String(),
		Verdict:         string(record.Evidence.Verdict),
		Channel:         string(record.Evidence.Channel),
		Command:         record.Evidence.Command,
		Output:          record.Evidence.Output,
		OutputTruncated: record.Evidence.Truncated,
		Redactions:      record.Evidence.Redactions,
		ExitCode:        record.Evidence.ExitCode,
		SubmittedBy:     submitter(record.Evidence),
		ObservedAt:      record.Evidence.ObservedAt,
		ReceivedAt:      record.Evidence.ReceivedAt,
		Expired:         record.Expiry.Expired(),
		Bearing:         bearing(record.Evidence.Verdict),
	}

	if record.Expiry == entity.EvidenceTimedOut {
		dto.ExpiredBecause = "older than the check's time limit"
	}

	return dto
}

func submitter(evidence entity.Evidence) string {
	kind := string(evidence.Actor.Kind)

	if evidence.ActorName == "" {
		return kind
	}

	return evidence.ActorName + " (" + kind + ")"
}

func bearing(verdict entity.EvidenceVerdict) string {
	switch {
	case verdict.Proves():
		return "proves"
	case verdict.Disproves():
		return "disproves"
	case verdict.RestsOnAbsence():
		return "neither; absence of a failure is not a positive result"
	default:
		return "neither"
	}
}

type summaryDTO struct {
	Total      int `json:"total"`
	Proven     int `json:"proven"`
	Unproven   int `json:"unproven"`
	Failed     int `json:"failed"`
	Waived     int `json:"waived"`
	Gaps       int `json:"gaps"`
	Expired    int `json:"expired"`
	Unapproved int `json:"unapproved"`
	Blocking   int `json:"blocking"`
}

func summaryDTOFrom(summary entity.CheckSummary) summaryDTO {
	return summaryDTO{
		Total:      summary.Total,
		Proven:     summary.Proven,
		Unproven:   summary.Unproven,
		Failed:     summary.Failed,
		Waived:     summary.Waived,
		Gaps:       summary.Gaps,
		Expired:    summary.Expired,
		Unapproved: summary.Unapproved,
		Blocking:   summary.Blocking,
	}
}
