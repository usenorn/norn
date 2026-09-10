package issuedraft

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type issueDraftsService struct {
	drafts     repository.IssueDraft
	authorizer service.Authorizer
}

func New(drafts repository.IssueDraft, authorizer service.Authorizer) service.IssueDrafts {
	return &issueDraftsService{drafts: drafts, authorizer: authorizer}
}

// A draft belongs to the person writing it and to nobody else, so every operation is scoped to
// the account the request authenticated as rather than to what the caller asks for.
func (s *issueDraftsService) owner(
	ctx context.Context,
	workspaceID uuid.UUID,
) (uuid.UUID, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return uuid.Nil, err
	}

	if decision.Actor.AccountID == uuid.Nil {
		return uuid.Nil, entity.AccessDeniedError{
			Reason:      entity.DenyReasonNoActor,
			Resource:    entity.ResourceIssue,
			Action:      entity.ActionRead,
			WorkspaceID: workspaceID,
			ActorKind:   decision.Actor.Kind,
		}
	}

	return decision.Actor.AccountID, nil
}

func (s *issueDraftsService) Save(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.SaveIssueDraftInput,
) (entity.IssueDraft, error) {
	account, err := s.owner(ctx, workspaceID)
	if err != nil {
		return entity.IssueDraft{}, err
	}

	if err := entity.NewValidationError(
		entity.ValidateIssueDraftTitle("title", input.Title),
	); err != nil {
		return entity.IssueDraft{}, err
	}

	markdown, document, err := entity.Described(input.Description, input.DescriptionDoc)
	if err != nil {
		return entity.IssueDraft{}, err
	}

	if input.DraftID == uuid.Nil {
		counted, err := s.drafts.Count(ctx, workspaceID, account)
		if err != nil {
			return entity.IssueDraft{}, err
		}

		if counted >= entity.IssueDraftMaxPerAccount {
			return entity.IssueDraft{}, entity.ErrTooManyIssueDrafts
		}
	}

	return s.drafts.Save(ctx, entity.IssueDraft{
		ID:                input.DraftID,
		WorkspaceID:       workspaceID,
		AccountID:         account,
		TeamID:            input.TeamID,
		Title:             input.Title,
		Description:       markdown,
		DescriptionDoc:    document,
		StateID:           input.StateID,
		ProjectID:         input.ProjectID,
		CycleID:           input.CycleID,
		AssigneeAccountID: input.AssigneeAccountID,
		ParentIssueID:     input.ParentIssueID,
		LabelIDs:          input.LabelIDs,
		AttachmentIDs:     input.AttachmentIDs,
		Priority:          input.Priority,
		Estimate:          input.Estimate,
		DueOn:             input.DueOn,
		UpdatedAt:         time.Now().UTC(),
	})
}

func (s *issueDraftsService) List(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]entity.IssueDraft, error) {
	account, err := s.owner(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	return s.drafts.List(ctx, workspaceID, account, entity.IssueDraftPageSize)
}

func (s *issueDraftsService) Remove(
	ctx context.Context,
	workspaceID, draftID uuid.UUID,
) error {
	account, err := s.owner(ctx, workspaceID)
	if err != nil {
		return err
	}

	return s.drafts.Remove(ctx, workspaceID, account, draftID)
}
