package changeset

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type changeSetsService struct {
	changesets repository.ChangeSet
	issues     repository.Issue
	executions service.Executions
	source     service.SourceControl
	events     service.Events
	authorizer service.Authorizer
	transactor repository.Transactor
}

func New(
	changesets repository.ChangeSet,
	issues repository.Issue,
	executions service.Executions,
	source service.SourceControl,
	events service.Events,
	authorizer service.Authorizer,
	transactor repository.Transactor,
) service.ChangeSets {
	return &changeSetsService{
		changesets: changesets,
		issues:     issues,
		executions: executions,
		source:     source,
		events:     events,
		authorizer: authorizer,
		transactor: transactor,
	}
}

func (s *changeSetsService) ForIssue(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) (entity.IssueChangeSet, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.IssueChangeSet{}, err
	}

	if _, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope); err != nil {
		return entity.IssueChangeSet{}, err
	}

	return s.changesets.ByIssue(ctx, workspaceID, issueID)
}

func decode(payload []byte, into any) error {
	if len(payload) == 0 {
		return nil
	}

	if err := json.Unmarshal(payload, into); err != nil {
		return entity.ErrChannelEnvelopeInvalid
	}

	return nil
}
