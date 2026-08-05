package dashboard

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceAudit(
	ctx context.Context,
	request api.ListWorkspaceAuditRequestObject,
) (api.ListWorkspaceAuditResponseObject, error) {
	page, err := h.auditLog.List(
		ctx,
		service.AuditScope{WorkspaceID: request.WorkspaceId},
		auditInput(auditQuery{
			Limit:        request.Params.Limit,
			Cursor:       request.Params.Cursor,
			Actor:        request.Params.Actor,
			Action:       request.Params.Action,
			ResourceKind: request.Params.ResourceKind,
			ResourceID:   request.Params.ResourceId,
			From:         request.Params.From,
			To:           request.Params.To,
		}),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceAudit200JSONResponse(
		auditPageDTO(page, h.auditLog.Availability(ctx).RetentionDays),
	), nil
}

func (h *handler) ListInstanceAudit(
	ctx context.Context,
	request api.ListInstanceAuditRequestObject,
) (api.ListInstanceAuditResponseObject, error) {
	page, err := h.auditLog.List(
		ctx,
		service.AuditScope{Instance: true},
		auditInput(auditQuery{
			Limit:        request.Params.Limit,
			Cursor:       request.Params.Cursor,
			Actor:        request.Params.Actor,
			Action:       request.Params.Action,
			ResourceKind: request.Params.ResourceKind,
			ResourceID:   request.Params.ResourceId,
			From:         request.Params.From,
			To:           request.Params.To,
		}),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListInstanceAudit200JSONResponse(
		auditPageDTO(page, h.auditLog.Availability(ctx).RetentionDays),
	), nil
}

func (h *handler) GetWorkspaceAuditAvailability(
	ctx context.Context,
	_ api.GetWorkspaceAuditAvailabilityRequestObject,
) (api.GetWorkspaceAuditAvailabilityResponseObject, error) {
	available := h.auditLog.Availability(ctx)

	dto := api.AuditAvailability{
		Available:     available.Available,
		RetentionDays: int32(available.RetentionDays),
	}

	if available.Holder != "" {
		dto.Holder = &available.Holder
	}

	return api.GetWorkspaceAuditAvailability200JSONResponse(dto), nil
}

func (h *handler) SetWorkspaceMemberAuditAccess(
	ctx context.Context,
	request api.SetWorkspaceMemberAuditAccessRequestObject,
) (api.SetWorkspaceMemberAuditAccessResponseObject, error) {
	membership, err := h.auditLog.SetAccess(
		ctx, request.WorkspaceId, request.AccountId, request.Body.ReadsAudit,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetWorkspaceMemberAuditAccess200JSONResponse(
		membershipDTO(entity.WorkspaceMember{Membership: membership}),
	), nil
}

type auditQuery struct {
	Limit        *api.AuditLimit
	Cursor       *api.AuditCursor
	Actor        *api.AuditActorFilter
	Action       *api.AuditActionFilter
	ResourceKind *api.AuditResourceKind
	ResourceID   *api.AuditResourceId
	From         *api.AuditFrom
	To           *api.AuditTo
}

func auditInput(query auditQuery) service.ListAuditInput {
	filter := entity.AuditFilter{}

	if query.Limit != nil {
		filter.Limit = int(*query.Limit)
	}

	if query.Actor != nil {
		filter.ActorAccountID = *query.Actor
	}

	if query.Action != nil {
		actions := make([]entity.AuditAction, 0, len(*query.Action))
		for _, action := range *query.Action {
			actions = append(actions, entity.AuditAction(action))
		}

		filter.Actions = actions
	}

	if query.ResourceKind != nil {
		filter.ResourceKind = *query.ResourceKind
	}

	if query.ResourceID != nil {
		filter.ResourceID = *query.ResourceID
	}

	filter.From = query.From
	filter.To = query.To

	input := service.ListAuditInput{Filter: filter}

	if query.Cursor != nil {
		input.Cursor = *query.Cursor
	}

	return input
}

func auditPageDTO(page service.AuditPage, retentionDays int) api.AuditPage {
	dto := api.AuditPage{
		Events:        auditEventDTOs(page.Events),
		RetentionDays: int32(retentionDays),
	}

	if page.NextCursor != "" {
		dto.NextCursor = &page.NextCursor
	}

	return dto
}

func auditEventDTOs(events []entity.AuditEntry) []api.AuditEvent {
	dtos := make([]api.AuditEvent, len(events))
	for i, event := range events {
		dtos[i] = auditEventDTO(event)
	}

	return dtos
}

func auditEventDTO(event entity.AuditEntry) api.AuditEvent {
	dto := api.AuditEvent{
		Id:           event.ID,
		OccurredAt:   event.OccurredAt,
		Action:       api.AuditAction(event.Action),
		Outcome:      api.AuditOutcome(event.Outcome),
		Actor:        auditActorDTO(event.Actor),
		ResourceKind: nilIfEmpty(event.ResourceKind),
		ResourceName: nilIfEmpty(event.ResourceName),
		UserAgent:    nilIfEmpty(event.UserAgent),
		WorkspaceId:  auditID(event.WorkspaceID),
		ResourceId:   auditID(event.ResourceID),
	}

	dto.WorkspaceName = nilIfEmpty(event.WorkspaceName)

	if event.SourceIP.IsValid() {
		address := event.SourceIP.String()
		dto.SourceIp = &address
	}

	if len(event.Detail) > 0 {
		detail := event.Detail
		dto.Detail = &detail
	}

	return dto
}

func auditActorDTO(actor entity.AuditActor) api.AuditActor {
	dto := api.AuditActor{
		Kind:       api.ActivityActorKind(actor.Kind),
		AuthMethod: api.AuditAuthMethod(actor.How()),
		Name:       nilIfEmpty(actor.Name),
		TokenName:  nilIfEmpty(actor.TokenName),
		AgentName:  nilIfEmpty(actor.AgentName),
		AccountId:  auditID(actor.AccountID),
		TokenId:    actor.TokenID,
		AgentId:    actor.AgentID,
	}

	return dto
}

func auditID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}

	return &id
}
