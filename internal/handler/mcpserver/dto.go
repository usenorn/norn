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
	TriageState     string     `json:"triageState,omitempty"`
	Priority        string     `json:"priority"`
	AssigneeID      string     `json:"assigneeId,omitempty"`
	CreatedByID     string     `json:"createdById,omitempty"`
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
		TriageState:     string(issue.TriageState),
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

	if issue.CreatedByAccountID != uuid.Nil {
		dto.CreatedByID = issue.CreatedByAccountID.String()
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
	TeamIDs      []string          `json:"teamIds,omitempty"`
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

	for _, teamID := range project.TeamIDs {
		dto.TeamIDs = append(dto.TeamIDs, teamID.String())
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

type questionDTO struct {
	ID         string   `json:"id"`
	Question   string   `json:"question"`
	Default    string   `json:"default"`
	Options    []string `json:"options,omitempty"`
	Deadline   string   `json:"deadline"`
	Answered   bool     `json:"answered"`
	Expired    bool     `json:"expired"`
	Standing   string   `json:"standing"`
	AnsweredBy string   `json:"answeredBy,omitempty"`
}

func questionDTOFrom(question entity.IssueQuestion) questionDTO {
	return questionDTO{
		ID:         question.ID.String(),
		Question:   question.Question,
		Default:    question.DefaultAnswer,
		Options:    question.Options,
		Deadline:   question.Deadline.Format(time.RFC3339),
		Answered:   question.Answered(),
		Expired:    question.Expired(time.Now().UTC()),
		Standing:   question.Standing(),
		AnsweredBy: question.AnsweredByName,
	}
}

func questionDTOs(questions []entity.IssueQuestion) []questionDTO {
	dtos := make([]questionDTO, 0, len(questions))

	for _, question := range questions {
		dtos = append(dtos, questionDTOFrom(question))
	}

	return dtos
}

type relatedIssueDTO struct {
	ID        string   `json:"id"`
	Reference string   `json:"reference"`
	Title     string   `json:"title"`
	State     stateRef `json:"state"`
	Status    string   `json:"status"`
}

type relationDTO struct {
	ID    string          `json:"id"`
	Kind  string          `json:"kind"`
	Issue relatedIssueDTO `json:"issue"`
}

func relationDTOFrom(relation entity.IssueRelation) relationDTO {
	return relationDTO{
		ID:   relation.ID.String(),
		Kind: string(relation.Kind),
		Issue: relatedIssueDTO{
			ID:        relation.Issue.ID.String(),
			Reference: relation.Issue.Reference(),
			Title:     relation.Issue.Title,
			State: stateRef{
				ID:       relation.Issue.State.ID.String(),
				Name:     relation.Issue.State.Name,
				Category: string(relation.Issue.State.Category),
			},
			Status: string(relation.Issue.Status),
		},
	}
}

func relationDTOsFrom(groups []entity.IssueRelationGroup) []relationDTO {
	var dtos []relationDTO

	for _, group := range groups {
		for _, relation := range group.Relations {
			dtos = append(dtos, relationDTOFrom(relation))
		}
	}

	return dtos
}

type docAudienceDTO struct {
	Visibility string `json:"visibility"`
	ProjectID  string `json:"projectId,omitempty"`
}

type docSummaryDTO struct {
	ID         string         `json:"id"`
	Title      string         `json:"title"`
	ParentID   string         `json:"parentId,omitempty"`
	Audience   docAudienceDTO `json:"audience"`
	AuthorID   string         `json:"authorId,omitempty"`
	AuthorName string         `json:"authorName,omitempty"`
	Status     string         `json:"status"`
	Version    int            `json:"version"`
	UpdatedAt  string         `json:"updatedAt"`
}

type docDTO struct {
	docSummaryDTO
	Markdown string `json:"markdown"`
}

type docMatchDTO struct {
	docSummaryDTO
	Excerpt string `json:"excerpt"`
}

func docSummaryDTOFrom(summary entity.DocSummary) docSummaryDTO {
	dto := docSummaryDTO{
		ID:         summary.ID.String(),
		Title:      summary.Title,
		Audience:   docAudienceDTO{Visibility: string(summary.Audience.Visibility)},
		AuthorName: summary.AuthorName,
		Status:     string(summary.Status),
		Version:    summary.Version,
		UpdatedAt:  summary.UpdatedAt.Format(time.RFC3339),
	}

	if summary.ParentID != nil {
		dto.ParentID = summary.ParentID.String()
	}

	if summary.Audience.Visibility == entity.KnowledgeVisibilityProject {
		dto.Audience.ProjectID = summary.Audience.ProjectID.String()
	}

	if summary.AuthorID != uuid.Nil {
		dto.AuthorID = summary.AuthorID.String()
	}

	return dto
}

type linkedProjectDTO struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type docLinkDTO struct {
	ID      string            `json:"id"`
	Target  string            `json:"target"`
	Issue   *relatedIssueDTO  `json:"issue,omitempty"`
	Project *linkedProjectDTO `json:"project,omitempty"`
}

func docLinkDTOFrom(link entity.DocLink) docLinkDTO {
	dto := docLinkDTO{ID: link.ID.String(), Target: string(link.Target)}

	switch link.Target {
	case entity.DocLinkTargetIssue:
		dto.Issue = &relatedIssueDTO{
			ID:        link.Issue.ID.String(),
			Reference: link.Issue.Reference(),
			Title:     link.Issue.Title,
			State: stateRef{
				ID:       link.Issue.State.ID.String(),
				Name:     link.Issue.State.Name,
				Category: string(link.Issue.State.Category),
			},
			Status: string(link.Issue.Status),
		}
	case entity.DocLinkTargetProject:
		dto.Project = &linkedProjectDTO{
			ID:   link.Project.ID.String(),
			Slug: link.Project.Slug,
			Name: link.Project.Name,
		}
	}

	return dto
}

func docDTOFrom(doc entity.Doc) docDTO {
	return docDTO{docSummaryDTO: docSummaryDTOFrom(doc.DocSummary), Markdown: doc.Markdown}
}

type memoryApplicabilityDTO struct {
	ProjectID string `json:"projectId,omitempty"`
	Component string `json:"component,omitempty"`
}

type memoryProvenanceDTO struct {
	Source  string `json:"source"`
	AgentID string `json:"agentId,omitempty"`
	TokenID string `json:"tokenId,omitempty"`
}

type memoryEvidenceDTO struct {
	Kind          string `json:"kind"`
	IssueID       string `json:"issueId,omitempty"`
	CommentID     string `json:"commentId,omitempty"`
	DocID         string `json:"docId,omitempty"`
	DocRevisionID string `json:"docRevisionId,omitempty"`
	Excerpt       string `json:"excerpt,omitempty"`
	State         string `json:"state"`
}

type memoryDTO struct {
	ID             string                 `json:"id"`
	Kind           string                 `json:"kind"`
	Statement      string                 `json:"statement"`
	Rationale      string                 `json:"rationale,omitempty"`
	Conditions     string                 `json:"conditions,omitempty"`
	Status         string                 `json:"status"`
	Basis          string                 `json:"basis"`
	Audience       docAudienceDTO         `json:"audience"`
	AppliesTo      memoryApplicabilityDTO `json:"appliesTo"`
	AuthorID       string                 `json:"authorId,omitempty"`
	AuthorName     string                 `json:"authorName,omitempty"`
	Provenance     memoryProvenanceDTO    `json:"provenance"`
	Evidence       []memoryEvidenceDTO    `json:"evidence,omitempty"`
	EffectiveFrom  string                 `json:"effectiveFrom"`
	EffectiveUntil string                 `json:"effectiveUntil,omitempty"`
	SupersedesID   string                 `json:"supersedesId,omitempty"`
	SupersededByID string                 `json:"supersededById,omitempty"`
	Version        int                    `json:"version"`
	UpdatedAt      string                 `json:"updatedAt"`
}

type memoryLinkDTO struct {
	ID        string `json:"id"`
	Statement string `json:"statement"`
	Status    string `json:"status"`
	Version   int    `json:"version"`
}

type knowledgePassageDTO struct {
	DocID    string `json:"docId"`
	DocTitle string `json:"docTitle"`
	Heading  string `json:"heading,omitempty"`
	Excerpt  string `json:"excerpt"`
}

type knowledgeHitDTO struct {
	Kind    string               `json:"kind"`
	Memory  *memoryDTO           `json:"memory,omitempty"`
	Passage *knowledgePassageDTO `json:"passage,omitempty"`
	Matches []string             `json:"matches"`
	Score   float64              `json:"score"`
}

type relatedMemoryDTO struct {
	Memory   memoryDTO `json:"memory"`
	Relation string    `json:"relation"`
}

type memoryChoiceDTO struct {
	Resolution string `json:"resolution"`
	Target     string `json:"target,omitempty"`
}

func optionalID(id *uuid.UUID) string {
	if id == nil {
		return ""
	}

	return id.String()
}

func memoryDTOFrom(memory entity.Memory) memoryDTO {
	dto := memoryDTO{
		ID:         memory.ID.String(),
		Kind:       string(memory.Kind),
		Statement:  memory.Statement,
		Rationale:  memory.Rationale,
		Conditions: memory.Conditions,
		Status:     string(memory.Status),
		Basis:      string(memory.Basis),
		Audience:   docAudienceDTO{Visibility: string(memory.Audience.Visibility)},
		AppliesTo: memoryApplicabilityDTO{
			ProjectID: optionalID(memory.Applicability.ProjectID),
			Component: memory.Applicability.Component,
		},
		AuthorName: memory.AuthorName,
		Provenance: memoryProvenanceDTO{
			Source:  string(memory.By.Source),
			AgentID: optionalID(memory.By.AgentID),
			TokenID: optionalID(memory.By.TokenID),
		},
		EffectiveFrom:  memory.EffectiveFrom.Format(time.DateOnly),
		SupersedesID:   optionalID(memory.SupersedesID),
		SupersededByID: optionalID(memory.SupersededByID),
		Version:        memory.Version,
		UpdatedAt:      memory.UpdatedAt.Format(time.RFC3339),
	}

	if memory.Audience.Visibility == entity.KnowledgeVisibilityProject {
		dto.Audience.ProjectID = memory.Audience.ProjectID.String()
	}

	if memory.AuthorID != uuid.Nil {
		dto.AuthorID = memory.AuthorID.String()
	}

	if memory.EffectiveUntil != nil {
		dto.EffectiveUntil = memory.EffectiveUntil.Format(time.DateOnly)
	}

	for _, evidence := range memory.Evidence {
		dto.Evidence = append(dto.Evidence, memoryEvidenceDTO{
			Kind:          string(evidence.Kind),
			IssueID:       optionalID(evidence.IssueID),
			CommentID:     optionalID(evidence.CommentID),
			DocID:         optionalID(evidence.DocID),
			DocRevisionID: optionalID(evidence.DocRevisionID),
			Excerpt:       evidence.Excerpt,
			State:         string(evidence.State),
		})
	}

	return dto
}

func relatedMemoryDTOsFrom(related []entity.RelatedMemory) []relatedMemoryDTO {
	dtos := make([]relatedMemoryDTO, 0, len(related))
	for _, candidate := range related {
		dtos = append(dtos, relatedMemoryDTO{Memory: memoryDTOFrom(candidate.Memory), Relation: string(candidate.Relation)})
	}

	return dtos
}

func memoryChoiceDTOsFrom(choices []entity.MemoryChoice) []memoryChoiceDTO {
	dtos := make([]memoryChoiceDTO, 0, len(choices))
	for _, choice := range choices {
		dtos = append(dtos, memoryChoiceDTO{Resolution: string(choice.Resolution), Target: optionalID(choice.TargetID)})
	}

	return dtos
}

func memoryLinkDTOFrom(memory entity.Memory) memoryLinkDTO {
	return memoryLinkDTO{
		ID:        memory.ID.String(),
		Statement: memory.Statement,
		Status:    string(memory.Status),
		Version:   memory.Version,
	}
}

func knowledgeHitDTOFrom(hit entity.KnowledgeHit) knowledgeHitDTO {
	dto := knowledgeHitDTO{Kind: string(hit.Kind), Matches: make([]string, 0, len(hit.Matches)), Score: hit.Score}

	for _, match := range hit.Matches {
		dto.Matches = append(dto.Matches, string(match))
	}

	if hit.Memory != nil {
		memory := memoryDTOFrom(*hit.Memory)
		dto.Memory = &memory
	}

	if hit.Passage != nil {
		dto.Passage = &knowledgePassageDTO{
			DocID:    hit.Passage.DocID.String(),
			DocTitle: hit.Passage.DocTitle,
			Heading:  hit.Passage.Heading,
			Excerpt:  hit.Passage.Excerpt,
		}
	}

	return dto
}
