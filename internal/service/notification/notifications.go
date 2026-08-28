package notification

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type notificationsService struct {
	notifications repository.Notification
	events        repository.NotificationEvent
	followers     repository.IssueFollower
	settings      repository.NotificationSetting
	issues        repository.Issue
	comments      repository.IssueComment
	teamMembers   repository.TeamMember
	workspaces    repository.Workspace
	mailer        repository.Mailer
	broadcast     service.Events
	authorizer    service.Authorizer
	app           config.App
}

func New(
	notifications repository.Notification,
	events repository.NotificationEvent,
	followers repository.IssueFollower,
	settings repository.NotificationSetting,
	issues repository.Issue,
	comments repository.IssueComment,
	teamMembers repository.TeamMember,
	workspaces repository.Workspace,
	mailer repository.Mailer,
	broadcast service.Events,
	authorizer service.Authorizer,
	app config.App,
) service.Notifications {
	return &notificationsService{
		notifications: notifications,
		events:        events,
		followers:     followers,
		settings:      settings,
		issues:        issues,
		comments:      comments,
		teamMembers:   teamMembers,
		workspaces:    workspaces,
		mailer:        mailer,
		broadcast:     broadcast,
		authorizer:    authorizer,
		app:           app,
	}
}

func (s *notificationsService) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	action entity.Action,
) (entity.Decision, error) {
	return s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceNotification,
		Action:      action,
		WorkspaceID: workspaceID,
	})
}

func (s *notificationsService) Directed(
	ctx context.Context,
	workspaceID, recipientID, subjectID uuid.UUID,
	limit int,
) ([]entity.DirectedNotice, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return nil, err
	}

	if recipientID == uuid.Nil {
		return nil, entity.NewValidationError(entity.FieldError{
			Field: "recipientId",
			Code:  entity.ValidationCodeRequired,
		})
	}

	if limit <= 0 {
		limit = entity.NotificationPageDefaultSize
	}

	if limit > entity.NotificationPageMaxSize {
		limit = entity.NotificationPageMaxSize
	}

	return s.notifications.Directed(
		ctx, workspaceID, decision.Actor.Authority(), recipientID, subjectID, limit,
	)
}

func (s *notificationsService) Inbox(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.ListNotificationsInput,
) (service.Inbox, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return service.Inbox{}, err
	}

	page := entity.NotificationPage{Filter: input.Filter, Limit: input.Limit}.Normalized()

	if input.Cursor != "" {
		cursor, err := entity.DecodeNotificationCursor(input.Cursor)
		if err != nil {
			return service.Inbox{}, err
		}

		page.Cursor = &cursor
	}

	notifications, err := s.notifications.ListInbox(
		ctx, workspaceID, decision.Actor.AccountID, page.Lookahead(),
	)
	if err != nil {
		return service.Inbox{}, err
	}

	unread, err := s.notifications.Unread(ctx, workspaceID, decision.Actor.AccountID)
	if err != nil {
		return service.Inbox{}, err
	}

	inbox := service.Inbox{Notifications: notifications, Unread: unread}

	if len(notifications) > page.Limit {
		inbox.Notifications = notifications[:page.Limit]
		inbox.NextCursor = inbox.Notifications[page.Limit-1].Cursor().Encode()
	}

	return inbox, nil
}

func (s *notificationsService) Unread(ctx context.Context, workspaceID uuid.UUID) (int, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return 0, err
	}

	return s.notifications.Unread(ctx, workspaceID, decision.Actor.AccountID)
}

func (s *notificationsService) MarkRead(
	ctx context.Context,
	workspaceID uuid.UUID,
	subject entity.NotificationSubject,
) error {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return err
	}

	if !subject.Kind.Valid() {
		return entity.ErrNotificationNotFound
	}

	return s.notifications.MarkRead(
		ctx, workspaceID, decision.Actor.AccountID, subject, time.Now().UTC(),
	)
}

func (s *notificationsService) MarkAllRead(ctx context.Context, workspaceID uuid.UUID) error {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return err
	}

	return s.notifications.MarkAllRead(ctx, workspaceID, decision.Actor.AccountID, time.Now().UTC())
}

func (s *notificationsService) Snooze(
	ctx context.Context,
	workspaceID uuid.UUID,
	subject entity.NotificationSubject,
	until time.Time,
) error {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return err
	}

	if !subject.Kind.Valid() {
		return entity.ErrNotificationNotFound
	}

	if !until.After(time.Now().UTC()) {
		return entity.ErrSnoozeNotInFuture
	}

	return s.notifications.Snooze(ctx, workspaceID, decision.Actor.AccountID, subject, until)
}

func (s *notificationsService) Following(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) (entity.IssueFollower, error) {
	decision, issue, err := s.onVisibleIssue(ctx, workspaceID, issueID, entity.ActionRead)
	if err != nil {
		return entity.IssueFollower{}, err
	}

	follower, err := s.followers.Get(ctx, issue.ID, decision.Actor.AccountID)
	if err != nil {
		return entity.IssueFollower{}, err
	}

	follower.WorkspaceID = workspaceID

	return follower, nil
}

func (s *notificationsService) Followers(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.IssueFollower, error) {
	_, issue, err := s.onVisibleIssue(ctx, workspaceID, issueID, entity.ActionRead)
	if err != nil {
		return nil, err
	}

	followers, err := s.followers.List(ctx, issue.ID)
	if err != nil {
		return nil, err
	}

	watching := make([]entity.IssueFollower, 0, len(followers))

	for _, follower := range followers {
		if follower.State != entity.FollowStateFollowing {
			continue
		}

		follower.WorkspaceID = workspaceID
		watching = append(watching, follower)
	}

	return watching, nil
}

func (s *notificationsService) SetFollowing(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	state entity.FollowState,
) (entity.IssueFollower, error) {
	if _, err := s.decide(ctx, workspaceID, entity.ActionManage); err != nil {
		return entity.IssueFollower{}, err
	}

	decision, issue, err := s.onVisibleIssue(ctx, workspaceID, issueID, entity.ActionRead)
	if err != nil {
		return entity.IssueFollower{}, err
	}

	if !state.Valid() {
		return entity.IssueFollower{}, entity.ErrNotificationNotFound
	}

	follower := entity.IssueFollower{
		IssueID:     issue.ID,
		WorkspaceID: workspaceID,
		AccountID:   decision.Actor.AccountID,
		State:       state,
	}

	if err := s.followers.SetState(ctx, follower); err != nil {
		return entity.IssueFollower{}, err
	}

	return follower, nil
}

func (s *notificationsService) onVisibleIssue(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	action entity.Action,
) (entity.Decision, entity.Issue, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      action,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Decision{}, entity.Issue{}, err
	}

	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return entity.Decision{}, entity.Issue{}, err
	}

	return decision, issue, nil
}

func (s *notificationsService) Settings(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) (service.NotificationSettingsView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return service.NotificationSettingsView{}, err
	}

	return s.view(ctx, workspaceID, decision.Actor.AccountID, teamID)
}

func (s *notificationsService) SaveSettings(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	preferences entity.NotificationPreferences,
) (service.NotificationSettingsView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return service.NotificationSettingsView{}, err
	}

	if err := s.settings.Save(ctx, entity.NotificationSettings{
		WorkspaceID: workspaceID,
		AccountID:   decision.Actor.AccountID,
		TeamID:      teamID,
		Preferences: preferences,
	}); err != nil {
		return service.NotificationSettingsView{}, err
	}

	return s.view(ctx, workspaceID, decision.Actor.AccountID, teamID)
}

func (s *notificationsService) ClearSettings(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) (service.NotificationSettingsView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return service.NotificationSettingsView{}, err
	}

	if err := s.settings.Clear(ctx, workspaceID, decision.Actor.AccountID, teamID); err != nil {
		return service.NotificationSettingsView{}, err
	}

	return s.view(ctx, workspaceID, decision.Actor.AccountID, teamID)
}

func (s *notificationsService) view(
	ctx context.Context,
	workspaceID, accountID, teamID uuid.UUID,
) (service.NotificationSettingsView, error) {
	settings, err := s.settings.List(ctx, workspaceID, accountID)
	if err != nil {
		return service.NotificationSettingsView{}, err
	}

	view := service.NotificationSettingsView{
		Global: entity.ResolveNotificationPreferences(settings, uuid.Nil),
	}

	view.Team = entity.ResolveNotificationPreferences(settings, teamID)

	for _, setting := range settings {
		if teamID != uuid.Nil && setting.TeamID == teamID {
			view.TeamOverridden = true
		}
	}

	return view, nil
}
