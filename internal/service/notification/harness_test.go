package notification_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	issuecommentrepo "github.com/usenorn/norn/internal/repository/issuecomment"
	issuefollowerrepo "github.com/usenorn/norn/internal/repository/issuefollower"
	mailerrepo "github.com/usenorn/norn/internal/repository/mailer"
	notificationrepo "github.com/usenorn/norn/internal/repository/notification"
	notificationeventrepo "github.com/usenorn/norn/internal/repository/notificationevent"
	notificationsettingrepo "github.com/usenorn/norn/internal/repository/notificationsetting"
	teammemberrepo "github.com/usenorn/norn/internal/repository/teammember"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	notificationsvc "github.com/usenorn/norn/internal/service/notification"
)

type harness struct {
	notifications *notificationrepo.MockNotification
	events        *notificationeventrepo.MockNotificationEvent
	followers     *issuefollowerrepo.MockIssueFollower
	settings      *notificationsettingrepo.MockNotificationSetting
	issues        *issuerepo.MockIssue
	comments      *issuecommentrepo.MockIssueComment
	teamMembers   *teammemberrepo.MockTeamMember
	workspaces    *workspacerepo.MockWorkspace
	mailer        *mailerrepo.MockMailer
	authorizer    *authorizersvc.MockAuthorizer
	service       service.Notifications

	workspaceID uuid.UUID
	teamID      uuid.UUID
	issueID     uuid.UUID
	actorID     uuid.UUID
	readerID    uuid.UUID
}

func newHarness(t *testing.T, smtp config.SMTP) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		notifications: notificationrepo.NewMockNotification(ctrl),
		events:        notificationeventrepo.NewMockNotificationEvent(ctrl),
		followers:     issuefollowerrepo.NewMockIssueFollower(ctrl),
		settings:      notificationsettingrepo.NewMockNotificationSetting(ctrl),
		issues:        issuerepo.NewMockIssue(ctrl),
		comments:      issuecommentrepo.NewMockIssueComment(ctrl),
		teamMembers:   teammemberrepo.NewMockTeamMember(ctrl),
		workspaces:    workspacerepo.NewMockWorkspace(ctrl),
		mailer:        mailerrepo.NewMockMailer(ctrl),
		authorizer:    authorizersvc.NewMockAuthorizer(ctrl),
		workspaceID:   uuid.New(),
		teamID:        uuid.New(),
		issueID:       uuid.New(),
		actorID:       uuid.New(),
		readerID:      uuid.New(),
	}

	h.service = notificationsvc.New(
		h.notifications, h.events, h.followers, h.settings, h.issues, h.comments,
		h.teamMembers, h.workspaces, h.mailer, h.authorizer, smtp, config.App{BaseURL: "https://norn.test"},
	)

	return h
}

func (h *harness) issueEvent(kind entity.NotificationKind, actor entity.ActorKind) entity.NotificationEvent {
	return entity.NotificationEvent{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		Subject:     entity.NotifyIssue(h.issueID),
		Kind:        kind,
		Actor:       h.actorID,
		ActorKind:   actor,
	}
}

func (h *harness) expectPendingEvents(events ...entity.NotificationEvent) {
	h.events.EXPECT().
		ClaimPending(gomock.Any(), entity.NotificationFanOutBatchSize).
		Return(events, nil)
}

func (h *harness) expectIssueOnTeam() {
	h.issues.EXPECT().
		GetVisible(gomock.Any(), h.workspaceID, h.issueID, gomock.Any()).
		Return(entity.Issue{ID: h.issueID, WorkspaceID: h.workspaceID, TeamID: h.teamID}, nil).
		AnyTimes()
}

func (h *harness) expectFollowers(accountIDs ...uuid.UUID) {
	followers := make([]entity.IssueFollower, 0, len(accountIDs))

	for _, accountID := range accountIDs {
		followers = append(followers, entity.IssueFollower{
			IssueID:     h.issueID,
			WorkspaceID: h.workspaceID,
			AccountID:   accountID,
			State:       entity.FollowStateFollowing,
		})
	}

	h.followers.EXPECT().List(gomock.Any(), h.issueID).Return(followers, nil).AnyTimes()
}

func (h *harness) expectAudience(visible ...uuid.UUID) {
	h.notifications.EXPECT().
		Audience(gomock.Any(), h.workspaceID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, candidates []uuid.UUID) ([]uuid.UUID, error) {
			allowed := make([]uuid.UUID, 0, len(candidates))

			for _, candidate := range candidates {
				for _, permitted := range visible {
					if candidate == permitted {
						allowed = append(allowed, candidate)
					}
				}
			}

			return allowed, nil
		}).
		AnyTimes()
}

func (h *harness) expectSettings(settings ...entity.NotificationSettings) {
	h.settings.EXPECT().
		ListFor(gomock.Any(), h.workspaceID, gomock.Any(), gomock.Any()).
		Return(settings, nil).
		AnyTimes()
}

func (h *harness) captureDeliveries() *[]repository.NotificationDelivery {
	captured := make([]repository.NotificationDelivery, 0)

	h.notifications.EXPECT().
		Deliver(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, deliveries []repository.NotificationDelivery) error {
			captured = append(captured, deliveries...)

			return nil
		}).
		AnyTimes()

	return &captured
}
