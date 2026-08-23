package preview

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type previewsService struct {
	previews   repository.Preview
	shares     repository.PreviewShare
	grants     repository.PreviewGrant
	executions repository.Execution
	runs       service.Executions
	events     service.Events
	authorizer service.Authorizer
	audit      service.Audit
	transactor repository.Transactor
	app        config.App
	settings   config.Previews
}

func New(
	previews repository.Preview,
	shares repository.PreviewShare,
	grants repository.PreviewGrant,
	executions repository.Execution,
	runs service.Executions,
	events service.Events,
	authorizer service.Authorizer,
	audit service.Audit,
	transactor repository.Transactor,
	app config.App,
	settings config.Previews,
) service.Previews {
	return &previewsService{
		previews:   previews,
		shares:     shares,
		grants:     grants,
		executions: executions,
		runs:       runs,
		events:     events,
		authorizer: authorizer,
		audit:      audit,
		transactor: transactor,
		app:        app,
		settings:   settings,
	}
}

func (s *previewsService) ForExecution(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
) ([]service.PreviewDetail, error) {
	if _, err := s.runs.Visible(ctx, workspaceID, executionID); err != nil {
		return nil, err
	}

	previews, err := s.previews.ByExecution(ctx, executionID)
	if err != nil {
		return nil, err
	}

	links, err := s.shares.ByExecution(ctx, executionID)
	if err != nil {
		return nil, err
	}

	details := make([]service.PreviewDetail, 0, len(previews))

	for _, preview := range previews {
		details = append(details, service.PreviewDetail{
			Preview: preview,
			URL:     preview.URL(s.settings.Scheme),
			Links:   linksOf(preview.ID, links),
		})
	}

	return details, nil
}

func (s *previewsService) named(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID, name string,
	action entity.Action,
) (entity.PreviewSession, error) {
	if action == entity.ActionRead {
		if _, err := s.runs.Visible(ctx, workspaceID, executionID); err != nil {
			return entity.PreviewSession{}, err
		}
	} else if _, err := s.runs.Manageable(ctx, workspaceID, executionID); err != nil {
		return entity.PreviewSession{}, err
	}

	return s.previews.ByName(ctx, executionID, name)
}

func linksOf(previewID uuid.UUID, links []entity.PreviewShareLink) []entity.PreviewShareLink {
	held := make([]entity.PreviewShareLink, 0, len(links))

	for _, link := range links {
		if link.PreviewID == previewID {
			held = append(held, link)
		}
	}

	return held
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
