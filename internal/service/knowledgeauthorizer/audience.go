package knowledgeauthorizer

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type audienceAuthorizer struct {
	authorizer service.Authorizer
	projects   repository.Project
}

func New(authorizer service.Authorizer, projects repository.Project) service.KnowledgeAuthorizer {
	return &audienceAuthorizer{authorizer: authorizer, projects: projects}
}

func (a *audienceAuthorizer) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	action entity.Action,
) (entity.KnowledgeDecision, error) {
	decision, err := a.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceKnowledge,
		Action:      action,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return entity.KnowledgeDecision{}, err
	}

	reader := entity.KnowledgeReader{
		WorkspaceID: workspaceID,
		Identity:    entity.KnowledgeIdentityOf(decision.Actor, decision.Role),
	}

	if reader.Identity.Maintainer() {
		reader.AllProjects = true

		return entity.KnowledgeDecision{Decision: decision, Reader: reader}, nil
	}

	if err := a.reach(ctx, decision.Actor, &reader); err != nil {
		return entity.KnowledgeDecision{}, err
	}

	return entity.KnowledgeDecision{Decision: decision, Reader: reader}, nil
}

func (a *audienceAuthorizer) reach(ctx context.Context, actor entity.Actor, reader *entity.KnowledgeReader) error {
	reader.ProjectIDs = []uuid.UUID{}

	if reader.Identity.Shared && actor.AgentScope != entity.AgentScopeProject {
		return nil
	}

	authority := actor.Authority()

	projects, err := a.projects.ListByWorkspaceID(ctx, reader.WorkspaceID, repository.ProjectFilter{
		ForAccountID:    &authority,
		IncludeArchived: true,
	})
	if err != nil {
		return err
	}

	if reader.Identity.Shared {
		if actor.AgentProjectID != nil && slices.ContainsFunc(projects, func(project entity.Project) bool {
			return project.ID == *actor.AgentProjectID
		}) {
			reader.ProjectIDs = append(reader.ProjectIDs, *actor.AgentProjectID)
		}

		return nil
	}

	for _, project := range projects {
		reader.ProjectIDs = append(reader.ProjectIDs, project.ID)

		if project.LedBy(reader.Identity.Principal) {
			reader.LedProjectIDs = append(reader.LedProjectIDs, project.ID)
		}
	}

	return nil
}

func (a *audienceAuthorizer) Reader(ctx context.Context, workspaceID uuid.UUID) (entity.KnowledgeDecision, error) {
	return a.decide(ctx, workspaceID, entity.ActionRead)
}

func (a *audienceAuthorizer) Read(
	ctx context.Context,
	access entity.KnowledgeAccess,
) (entity.KnowledgeDecision, error) {
	decision, err := a.decide(ctx, access.WorkspaceID, entity.ActionRead)
	if err != nil {
		return entity.KnowledgeDecision{}, err
	}

	if err := decision.Reader.Read(access); err != nil {
		return entity.KnowledgeDecision{}, err
	}

	return decision, nil
}

func (a *audienceAuthorizer) Author(
	ctx context.Context,
	workspaceID uuid.UUID,
	audience entity.KnowledgeAudience,
) (entity.KnowledgeDecision, error) {
	decision, err := a.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.KnowledgeDecision{}, err
	}

	if err := decision.Reader.Author(audience); err != nil {
		return entity.KnowledgeDecision{}, err
	}

	return decision, nil
}

func (a *audienceAuthorizer) Change(
	ctx context.Context,
	access entity.KnowledgeAccess,
) (entity.KnowledgeDecision, error) {
	decision, err := a.decide(ctx, access.WorkspaceID, entity.ActionManage)
	if err != nil {
		return entity.KnowledgeDecision{}, err
	}

	if err := decision.Reader.Change(access); err != nil {
		return entity.KnowledgeDecision{}, err
	}

	return decision, nil
}

func (a *audienceAuthorizer) Reshare(
	ctx context.Context,
	access entity.KnowledgeAccess,
	to entity.KnowledgeAudience,
) (entity.KnowledgeDecision, error) {
	decision, err := a.decide(ctx, access.WorkspaceID, entity.ActionManage)
	if err != nil {
		return entity.KnowledgeDecision{}, err
	}

	if err := decision.Reader.Reshare(access, to); err != nil {
		return entity.KnowledgeDecision{}, err
	}

	return decision, nil
}
