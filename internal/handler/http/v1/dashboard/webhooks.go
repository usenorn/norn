package dashboard

import (
	"context"
	"encoding/json"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceWebhooks(
	ctx context.Context,
	request api.ListWorkspaceWebhooksRequestObject,
) (api.ListWorkspaceWebhooksResponseObject, error) {
	hooks, err := h.webhooks.List(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceWebhooks200JSONResponse(webhookDTOs(hooks)), nil
}

func (h *handler) CreateWorkspaceWebhook(
	ctx context.Context,
	request api.CreateWorkspaceWebhookRequestObject,
) (api.CreateWorkspaceWebhookResponseObject, error) {
	events := make([]entity.WebhookEvent, 0, len(request.Body.Events))
	for _, event := range request.Body.Events {
		events = append(events, entity.WebhookEvent(event))
	}

	hook, err := h.webhooks.Register(ctx, service.RegisterWebhookInput{
		WorkspaceID: request.WorkspaceId,
		Name:        request.Body.Name,
		URL:         request.Body.Url,
		Events:      events,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.CreateWorkspaceWebhook201JSONResponse{
		Webhook: webhookDTO(hook),
		Secret:  hook.Secret,
	}, nil
}

func (h *handler) GetWorkspaceWebhook(
	ctx context.Context,
	request api.GetWorkspaceWebhookRequestObject,
) (api.GetWorkspaceWebhookResponseObject, error) {
	hook, err := h.webhooks.Get(ctx, request.WorkspaceId, request.WebhookId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceWebhook200JSONResponse(webhookDTO(hook)), nil
}

func (h *handler) UpdateWorkspaceWebhook(
	ctx context.Context,
	request api.UpdateWorkspaceWebhookRequestObject,
) (api.UpdateWorkspaceWebhookResponseObject, error) {
	input := service.UpdateWebhookInput{
		Name:    request.Body.Name,
		URL:     request.Body.Url,
		Enabled: request.Body.Enabled,
	}

	if request.Body.Events != nil {
		events := make([]entity.WebhookEvent, 0, len(*request.Body.Events))
		for _, event := range *request.Body.Events {
			events = append(events, entity.WebhookEvent(event))
		}

		input.Events = events
	}

	hook, err := h.webhooks.Update(ctx, request.WorkspaceId, request.WebhookId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UpdateWorkspaceWebhook200JSONResponse(webhookDTO(hook)), nil
}

func (h *handler) DeleteWorkspaceWebhook(
	ctx context.Context,
	request api.DeleteWorkspaceWebhookRequestObject,
) (api.DeleteWorkspaceWebhookResponseObject, error) {
	if err := h.webhooks.Remove(ctx, request.WorkspaceId, request.WebhookId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DeleteWorkspaceWebhook204Response{}, nil
}

func (h *handler) RotateWorkspaceWebhookSecret(
	ctx context.Context,
	request api.RotateWorkspaceWebhookSecretRequestObject,
) (api.RotateWorkspaceWebhookSecretResponseObject, error) {
	hook, err := h.webhooks.RotateSecret(ctx, request.WorkspaceId, request.WebhookId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RotateWorkspaceWebhookSecret200JSONResponse{
		Webhook: webhookDTO(hook),
		Secret:  hook.Secret,
	}, nil
}

func (h *handler) SendWorkspaceWebhookTest(
	ctx context.Context,
	request api.SendWorkspaceWebhookTestRequestObject,
) (api.SendWorkspaceWebhookTestResponseObject, error) {
	delivery, err := h.webhookDeliveries.Test(ctx, request.WorkspaceId, request.WebhookId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SendWorkspaceWebhookTest202JSONResponse(webhookDeliveryDTO(delivery)), nil
}

func (h *handler) ListWorkspaceWebhookDeliveries(
	ctx context.Context,
	request api.ListWorkspaceWebhookDeliveriesRequestObject,
) (api.ListWorkspaceWebhookDeliveriesResponseObject, error) {
	page, err := h.webhookDeliveries.List(
		ctx,
		request.WorkspaceId,
		request.WebhookId,
		webhookDeliveryInput(request.Params),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceWebhookDeliveries200JSONResponse(webhookDeliveryPageDTO(page)), nil
}

func (h *handler) GetWorkspaceWebhookDelivery(
	ctx context.Context,
	request api.GetWorkspaceWebhookDeliveryRequestObject,
) (api.GetWorkspaceWebhookDeliveryResponseObject, error) {
	delivery, err := h.webhookDeliveries.Get(
		ctx, request.WorkspaceId, request.WebhookId, request.DeliveryId,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceWebhookDelivery200JSONResponse(webhookDeliveryDetailDTO(delivery)), nil
}

func (h *handler) ReplayWorkspaceWebhookDelivery(
	ctx context.Context,
	request api.ReplayWorkspaceWebhookDeliveryRequestObject,
) (api.ReplayWorkspaceWebhookDeliveryResponseObject, error) {
	delivery, err := h.webhookDeliveries.Replay(
		ctx, request.WorkspaceId, request.WebhookId, request.DeliveryId,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ReplayWorkspaceWebhookDelivery202JSONResponse(webhookDeliveryDTO(delivery)), nil
}

func webhookDTOs(hooks []entity.Webhook) []api.Webhook {
	dtos := make([]api.Webhook, len(hooks))
	for i, hook := range hooks {
		dtos[i] = webhookDTO(hook)
	}

	return dtos
}

func webhookDTO(hook entity.Webhook) api.Webhook {
	events := make([]api.WebhookEventType, 0, len(hook.Events))
	for _, event := range hook.Events {
		events = append(events, api.WebhookEventType(event))
	}

	dto := api.Webhook{
		Id:              hook.ID,
		Name:            hook.Name,
		Url:             hook.URL,
		Events:          events,
		SecretHint:      hook.SecretHint,
		Enabled:         hook.Enabled,
		DisabledAt:      hook.DisabledAt,
		FailureStreak:   int32(hook.FailureStreak),
		LastDeliveredAt: hook.LastDeliveredAt,
		CreatedAt:       hook.CreatedAt,
		UpdatedAt:       hook.UpdatedAt,
	}

	if hook.DisabledReason != "" {
		reason := api.WebhookDisableReason(hook.DisabledReason)
		dto.DisabledReason = &reason
	}

	return dto
}

func webhookDeliveryDTOs(deliveries []entity.WebhookDelivery) []api.WebhookDelivery {
	dtos := make([]api.WebhookDelivery, len(deliveries))
	for i, delivery := range deliveries {
		dtos[i] = webhookDeliveryDTO(delivery)
	}

	return dtos
}

func webhookDeliveryDTO(delivery entity.WebhookDelivery) api.WebhookDelivery {
	dto := api.WebhookDelivery{
		Id:          delivery.ID,
		Event:       api.WebhookDeliveryEventType(delivery.Event),
		State:       api.WebhookDeliveryState(delivery.State),
		Attempt:     int32(delivery.Attempt),
		SubjectKind: nilIfEmpty(delivery.SubjectKind),
		SubjectId:   auditID(delivery.SubjectID),
		ReplayOf:    auditID(delivery.ReplayOf),
		SettledAt:   delivery.SettledAt,
		CreatedAt:   delivery.CreatedAt,
	}

	if !delivery.NextAttempt.IsZero() {
		nextAttempt := delivery.NextAttempt
		dto.NextAttemptAt = &nextAttempt
	}

	return dto
}

func webhookAttemptDTO(attempt entity.WebhookAttempt) api.WebhookAttempt {
	dto := api.WebhookAttempt{
		Attempt:         int32(attempt.Attempt),
		RequestUrl:      attempt.RequestURL,
		ResolvedAddress: nilIfEmpty(attempt.ResolvedAddress),
		Outcome:         api.WebhookAttemptOutcome(attempt.Outcome),
		ResponseExcerpt: nilIfEmpty(attempt.ResponseExcerpt),
		Error:           nilIfEmpty(attempt.Error),
		StartedAt:       attempt.StartedAt,
		FinishedAt:      attempt.FinishedAt,
		ElapsedMs:       attempt.Elapsed().Milliseconds(),
	}

	if attempt.StatusCode > 0 {
		statusCode := int32(attempt.StatusCode)
		dto.StatusCode = &statusCode
	}

	return dto
}

func webhookDeliveryDetailDTO(delivery entity.WebhookDelivery) api.WebhookDeliveryDetail {
	attempts := make([]api.WebhookAttempt, len(delivery.Attempts))
	for i, attempt := range delivery.Attempts {
		attempts[i] = webhookAttemptDTO(attempt)
	}

	body := map[string]any{}
	if len(delivery.Body) > 0 {
		if err := json.Unmarshal(delivery.Body, &body); err != nil {
			body = map[string]any{}
		}
	}

	return api.WebhookDeliveryDetail{
		Delivery: webhookDeliveryDTO(delivery),
		Body:     body,
		Attempts: attempts,
	}
}

func webhookDeliveryPageDTO(page service.WebhookDeliveryPage) api.WebhookDeliveryPage {
	dto := api.WebhookDeliveryPage{Deliveries: webhookDeliveryDTOs(page.Deliveries)}

	if page.NextCursor != "" {
		dto.NextCursor = &page.NextCursor
	}

	return dto
}

func webhookDeliveryInput(params api.ListWorkspaceWebhookDeliveriesParams) service.ListWebhookDeliveriesInput {
	input := service.ListWebhookDeliveriesInput{}

	if params.Limit != nil {
		input.Limit = int(*params.Limit)
	}

	if params.Cursor != nil {
		input.Cursor = *params.Cursor
	}

	if params.Event != nil {
		events := make([]entity.WebhookEvent, 0, len(*params.Event))
		for _, event := range *params.Event {
			events = append(events, entity.WebhookEvent(event))
		}

		input.Events = events
	}

	if params.State != nil {
		states := make([]entity.WebhookDeliveryState, 0, len(*params.State))
		for _, state := range *params.State {
			states = append(states, entity.WebhookDeliveryState(state))
		}

		input.States = states
	}

	return input
}
