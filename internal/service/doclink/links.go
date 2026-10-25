package doclink

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type linksService struct {
	docs       repository.Doc
	links      repository.DocLink
	issues     repository.Issue
	projects   repository.Project
	knowledge  service.KnowledgeAuthorizer
	authorizer service.Authorizer
}

func New(
	docs repository.Doc,
	links repository.DocLink,
	issues repository.Issue,
	projects repository.Project,
	knowledge service.KnowledgeAuthorizer,
	authorizer service.Authorizer,
) service.DocLinks {
	return &linksService{
		docs:       docs,
		links:      links,
		issues:     issues,
		projects:   projects,
		knowledge:  knowledge,
		authorizer: authorizer,
	}
}

func concealed(err error) error {
	if errors.Is(err, entity.ErrKnowledgeNotFound) {
		return entity.ErrDocNotFound
	}

	return err
}

func denied(err error) bool {
	var refusal entity.AccessDeniedError

	return errors.As(err, &refusal)
}

func (s *linksService) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	resource entity.Resource,
) (entity.Decision, error) {
	return s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    resource,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
}

func (s *linksService) changeable(ctx context.Context, workspaceID, docID uuid.UUID) (entity.KnowledgeDecision, error) {
	doc, err := s.docs.GetByID(ctx, workspaceID, docID)
	if err != nil {
		return entity.KnowledgeDecision{}, err
	}

	decision, err := s.knowledge.Change(ctx, doc.Access())
	if err != nil {
		return entity.KnowledgeDecision{}, concealed(err)
	}

	if doc.Archived() {
		return entity.KnowledgeDecision{}, entity.ErrDocArchived
	}

	return decision, nil
}

func (s *linksService) Link(
	ctx context.Context,
	workspaceID, docID uuid.UUID,
	input service.LinkDocInput,
) (entity.DocLink, error) {
	target, err := entity.DocLinkTargetOf(input.IssueID, input.ProjectID)
	if err != nil {
		return entity.DocLink{}, err
	}

	decision, err := s.changeable(ctx, workspaceID, docID)
	if err != nil {
		return entity.DocLink{}, err
	}

	link := entity.DocLink{ID: uuid.New(), DocID: docID, Target: target, CreatedAt: time.Now().UTC()}

	switch target {
	case entity.DocLinkTargetIssue:
		link.Issue, err = s.visibleIssue(ctx, workspaceID, input.IssueID)
	case entity.DocLinkTargetProject:
		link.Project, err = s.visibleProject(ctx, workspaceID, input.ProjectID)
	}

	if err != nil {
		return entity.DocLink{}, err
	}

	if err := s.links.Create(ctx, entity.StoredDocLink{
		ID:                 link.ID,
		WorkspaceID:        workspaceID,
		DocID:              docID,
		IssueID:            input.IssueID,
		ProjectID:          input.ProjectID,
		CreatedByAccountID: decision.Decision.Actor.AccountID,
		CreatedAt:          link.CreatedAt,
	}); err != nil {
		return entity.DocLink{}, err
	}

	return link, nil
}

func (s *linksService) visibleIssue(ctx context.Context, workspaceID, issueID uuid.UUID) (entity.Issue, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ResourceIssue)
	if denied(err) {
		return entity.Issue{}, entity.ErrIssueNotFound
	}

	if err != nil {
		return entity.Issue{}, err
	}

	return s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
}

func (s *linksService) visibleProject(ctx context.Context, workspaceID, projectID uuid.UUID) (entity.Project, error) {
	if _, err := s.decide(ctx, workspaceID, entity.ResourceProject); err != nil {
		if denied(err) {
			return entity.Project{}, entity.ErrProjectNotFound
		}

		return entity.Project{}, err
	}

	return s.projects.GetByID(ctx, workspaceID, projectID)
}

func (s *linksService) Unlink(ctx context.Context, workspaceID, docID, linkID uuid.UUID) error {
	if _, err := s.changeable(ctx, workspaceID, docID); err != nil {
		return err
	}

	return s.links.Delete(ctx, workspaceID, docID, linkID)
}

func (s *linksService) List(ctx context.Context, workspaceID, docID uuid.UUID) ([]entity.DocLink, error) {
	doc, err := s.docs.GetByID(ctx, workspaceID, docID)
	if err != nil {
		return nil, err
	}

	if _, err := s.knowledge.Read(ctx, doc.Access()); err != nil {
		return nil, concealed(err)
	}

	issues, err := s.decide(ctx, workspaceID, entity.ResourceIssue)
	if err != nil && !denied(err) {
		return nil, err
	}

	_, err = s.decide(ctx, workspaceID, entity.ResourceProject)
	if err != nil && !denied(err) {
		return nil, err
	}

	readsProjects := err == nil

	links, err := s.links.ListForDoc(ctx, workspaceID, docID, issues.Scope)
	if err != nil {
		return nil, err
	}

	return slices.DeleteFunc(links, func(link entity.DocLink) bool {
		return link.Target == entity.DocLinkTargetProject && !readsProjects
	}), nil
}

func (s *linksService) ForIssue(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.DocSummary, error) {
	if _, err := s.visibleIssue(ctx, workspaceID, issueID); err != nil {
		return nil, err
	}

	return s.backlinks(ctx, workspaceID, repository.DocFilter{LinkedIssueID: &issueID})
}

func (s *linksService) ForProject(ctx context.Context, workspaceID, projectID uuid.UUID) ([]entity.DocSummary, error) {
	if _, err := s.visibleProject(ctx, workspaceID, projectID); err != nil {
		return nil, err
	}

	return s.backlinks(ctx, workspaceID, repository.DocFilter{LinkedProjectID: &projectID})
}

func (s *linksService) backlinks(
	ctx context.Context,
	workspaceID uuid.UUID,
	filter repository.DocFilter,
) ([]entity.DocSummary, error) {
	decision, err := s.knowledge.Reader(ctx, workspaceID)
	if denied(err) {
		return []entity.DocSummary{}, nil
	}

	if err != nil {
		return nil, err
	}

	reader := decision.Reader
	filter.AllProjects = reader.AllProjects
	filter.ProjectIDs = reader.ProjectIDs
	filter.PersonalAuthorID = reader.PersonalAuthor()

	summaries, err := s.docs.List(ctx, workspaceID, filter, entity.DocPage{Limit: entity.DocPageMaxSize})
	if err != nil {
		return nil, err
	}

	return slices.DeleteFunc(summaries, func(summary entity.DocSummary) bool {
		return !reader.Sees(summary.Access())
	}), nil
}
