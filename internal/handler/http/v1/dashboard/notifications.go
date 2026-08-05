package dashboard

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceNotifications(
	ctx context.Context,
	request api.ListWorkspaceNotificationsRequestObject,
) (api.ListWorkspaceNotificationsResponseObject, error) {
	input := service.ListNotificationsInput{}

	if request.Params.Filter != nil {
		input.Filter = entity.NotificationFilter(*request.Params.Filter)
	}

	if request.Params.Limit != nil {
		input.Limit = int(*request.Params.Limit)
	}

	if request.Params.Cursor != nil {
		input.Cursor = *request.Params.Cursor
	}

	inbox, err := h.notifications.Inbox(ctx, request.WorkspaceId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceNotifications200JSONResponse(notificationPageDTO(inbox)), nil
}

func (h *handler) ReadAllWorkspaceNotifications(
	ctx context.Context,
	request api.ReadAllWorkspaceNotificationsRequestObject,
) (api.ReadAllWorkspaceNotificationsResponseObject, error) {
	if err := h.notifications.MarkAllRead(ctx, request.WorkspaceId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ReadAllWorkspaceNotifications204Response{}, nil
}

func (h *handler) ReadWorkspaceNotification(
	ctx context.Context,
	request api.ReadWorkspaceNotificationRequestObject,
) (api.ReadWorkspaceNotificationResponseObject, error) {
	subject := entity.NotificationSubject{
		Kind: entity.NotificationSubjectKind(request.SubjectKind),
		ID:   request.SubjectId,
	}

	if err := h.notifications.MarkRead(ctx, request.WorkspaceId, subject); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ReadWorkspaceNotification204Response{}, nil
}

func (h *handler) SnoozeWorkspaceNotification(
	ctx context.Context,
	request api.SnoozeWorkspaceNotificationRequestObject,
) (api.SnoozeWorkspaceNotificationResponseObject, error) {
	subject := entity.NotificationSubject{
		Kind: entity.NotificationSubjectKind(request.SubjectKind),
		ID:   request.SubjectId,
	}

	if err := h.notifications.Snooze(
		ctx, request.WorkspaceId, subject, request.Body.Until,
	); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SnoozeWorkspaceNotification204Response{}, nil
}

func (h *handler) GetWorkspaceIssueFollow(
	ctx context.Context,
	request api.GetWorkspaceIssueFollowRequestObject,
) (api.GetWorkspaceIssueFollowResponseObject, error) {
	follower, err := h.notifications.Following(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceIssueFollow200JSONResponse(issueFollowDTO(follower)), nil
}

func (h *handler) SetWorkspaceIssueFollow(
	ctx context.Context,
	request api.SetWorkspaceIssueFollowRequestObject,
) (api.SetWorkspaceIssueFollowResponseObject, error) {
	follower, err := h.notifications.SetFollowing(
		ctx, request.WorkspaceId, request.IssueId, entity.FollowState(request.Body.State),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetWorkspaceIssueFollow200JSONResponse(issueFollowDTO(follower)), nil
}

func (h *handler) GetWorkspaceNotificationSettings(
	ctx context.Context,
	request api.GetWorkspaceNotificationSettingsRequestObject,
) (api.GetWorkspaceNotificationSettingsResponseObject, error) {
	view, err := h.notifications.Settings(ctx, request.WorkspaceId, uuid.Nil)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceNotificationSettings200JSONResponse(notificationSettingsDTO(view)), nil
}

func (h *handler) SetWorkspaceNotificationSettings(
	ctx context.Context,
	request api.SetWorkspaceNotificationSettingsRequestObject,
) (api.SetWorkspaceNotificationSettingsResponseObject, error) {
	view, err := h.notifications.SaveSettings(
		ctx, request.WorkspaceId, uuid.Nil, notificationPreferences(*request.Body),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetWorkspaceNotificationSettings200JSONResponse(notificationSettingsDTO(view)), nil
}

func (h *handler) GetWorkspaceTeamNotificationSettings(
	ctx context.Context,
	request api.GetWorkspaceTeamNotificationSettingsRequestObject,
) (api.GetWorkspaceTeamNotificationSettingsResponseObject, error) {
	view, err := h.notifications.Settings(ctx, request.WorkspaceId, request.TeamId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceTeamNotificationSettings200JSONResponse(notificationSettingsDTO(view)), nil
}

func (h *handler) SetWorkspaceTeamNotificationSettings(
	ctx context.Context,
	request api.SetWorkspaceTeamNotificationSettingsRequestObject,
) (api.SetWorkspaceTeamNotificationSettingsResponseObject, error) {
	view, err := h.notifications.SaveSettings(
		ctx, request.WorkspaceId, request.TeamId, notificationPreferences(*request.Body),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetWorkspaceTeamNotificationSettings200JSONResponse(notificationSettingsDTO(view)), nil
}

func (h *handler) ClearWorkspaceTeamNotificationSettings(
	ctx context.Context,
	request api.ClearWorkspaceTeamNotificationSettingsRequestObject,
) (api.ClearWorkspaceTeamNotificationSettingsResponseObject, error) {
	view, err := h.notifications.ClearSettings(ctx, request.WorkspaceId, request.TeamId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ClearWorkspaceTeamNotificationSettings200JSONResponse(notificationSettingsDTO(view)), nil
}
