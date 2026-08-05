package notification

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
)

type candidate struct {
	accountID uuid.UUID
	reason    entity.NotificationReason
}

func (s *notificationsService) FanOut(ctx context.Context) (int, error) {
	events, err := s.events.ClaimPending(ctx, entity.NotificationFanOutBatchSize)
	if err != nil {
		return 0, err
	}

	delivered := 0

	for _, event := range events {
		count, err := s.fanOut(ctx, event)
		if err != nil {
			return delivered, err
		}

		delivered += count
	}

	return delivered, nil
}

func (s *notificationsService) fanOut(ctx context.Context, event entity.NotificationEvent) (int, error) {
	if event.BulkActionID != uuid.Nil {
		return 0, nil
	}

	teamID, err := s.subjectTeam(ctx, event)
	if err != nil {
		return 0, err
	}

	candidates, err := s.candidates(ctx, event)
	if err != nil || len(candidates) == 0 {
		return 0, err
	}

	accountIDs := make([]uuid.UUID, 0, len(candidates))
	for _, candidate := range candidates {
		accountIDs = append(accountIDs, candidate.accountID)
	}

	audience, err := s.notifications.Audience(ctx, event.WorkspaceID, teamID, accountIDs)
	if err != nil {
		return 0, err
	}

	if len(audience) == 0 {
		return 0, nil
	}

	settings, err := s.settings.ListFor(ctx, event.WorkspaceID, teamID, audience)
	if err != nil {
		return 0, err
	}

	deliveries := make([]repository.NotificationDelivery, 0, len(audience))

	for _, candidate := range candidates {
		if !slices.Contains(audience, candidate.accountID) {
			continue
		}

		preferences := entity.ResolveNotificationPreferences(
			settingsFor(settings, candidate.accountID), teamID,
		)

		channels := preferences.Delivers(candidate.reason.Governs(event.Kind), event.ActorKind)
		if !channels.Inbox && !channels.Email {
			continue
		}

		deliveries = append(deliveries, repository.NotificationDelivery{
			EventID:   event.ID,
			AccountID: candidate.accountID,
			Reason:    candidate.reason,
			Channels:  channels,
		})
	}

	if len(deliveries) == 0 {
		return 0, nil
	}

	slices.SortFunc(deliveries, func(a, b repository.NotificationDelivery) int {
		return strings.Compare(a.AccountID.String(), b.AccountID.String())
	})

	if err := s.notifications.Deliver(ctx, event.WorkspaceID, deliveries); err != nil {
		return 0, err
	}

	s.announce(ctx, event, deliveries)

	return len(deliveries), nil
}

func (s *notificationsService) subjectTeam(
	ctx context.Context,
	event entity.NotificationEvent,
) (uuid.UUID, error) {
	switch event.Subject.Kind {
	case entity.NotificationSubjectTeam:
		return event.Subject.ID, nil
	case entity.NotificationSubjectProject:
		return uuid.Nil, nil
	default:
		issue, err := s.issues.GetVisible(
			ctx, event.WorkspaceID, event.Subject.ID,
			entity.TeamScope{WorkspaceID: event.WorkspaceID, AllTeams: true},
		)
		if err != nil {
			return uuid.Nil, err
		}

		return issue.TeamID, nil
	}
}

func (s *notificationsService) candidates(
	ctx context.Context,
	event entity.NotificationEvent,
) ([]candidate, error) {
	reasons := make(map[uuid.UUID]entity.NotificationReason)

	add := func(accountID uuid.UUID, reason entity.NotificationReason) {
		if accountID == uuid.Nil || accountID == event.Actor {
			return
		}

		if existing, seen := reasons[accountID]; seen {
			reasons[accountID] = existing.Strongest(reason)

			return
		}

		reasons[accountID] = reason
	}

	mentioned, err := s.mentioned(ctx, event)
	if err != nil {
		return nil, err
	}

	for _, accountID := range mentioned {
		add(accountID, entity.NotificationReasonMentioned)
	}

	if event.Target != uuid.Nil {
		add(event.Target, targetReason(event.Kind))
	}

	if event.Subject.Kind == entity.NotificationSubjectIssue {
		followers, err := s.followers.List(ctx, event.Subject.ID)
		if err != nil {
			return nil, err
		}

		for _, follower := range followers {
			if !follower.State.Following() {
				continue
			}

			add(follower.AccountID, entity.NotificationReasonFollowing)
		}
	}

	candidates := make([]candidate, 0, len(reasons))
	for accountID, reason := range reasons {
		candidates = append(candidates, candidate{accountID: accountID, reason: reason})
	}

	return candidates, nil
}

func targetReason(kind entity.NotificationKind) entity.NotificationReason {
	if kind == entity.NotificationKindMembership {
		return entity.NotificationReasonMembership
	}

	return entity.NotificationReasonAssigned
}

func (s *notificationsService) mentioned(
	ctx context.Context,
	event entity.NotificationEvent,
) ([]uuid.UUID, error) {
	if event.CommentID == uuid.Nil {
		return nil, nil
	}

	mentions, err := s.comments.Mentioned(ctx, event.CommentID)
	if err != nil {
		return nil, err
	}

	accountIDs := make([]uuid.UUID, 0, len(mentions))

	for _, mention := range mentions {
		if mention.Kind == entity.MentionKindAccount {
			accountIDs = append(accountIDs, mention.AccountID)

			continue
		}

		members, err := s.teamMembers.ListByTeamID(ctx, mention.TeamID)
		if err != nil {
			return nil, err
		}

		for _, member := range members {
			accountIDs = append(accountIDs, member.AccountID)
		}
	}

	return accountIDs, nil
}

func settingsFor(settings []entity.NotificationSettings, accountID uuid.UUID) []entity.NotificationSettings {
	owned := make([]entity.NotificationSettings, 0, 2)

	for _, setting := range settings {
		if setting.AccountID == accountID {
			owned = append(owned, setting)
		}
	}

	return owned
}

func (s *notificationsService) announce(
	ctx context.Context,
	event entity.NotificationEvent,
	deliveries []repository.NotificationDelivery,
) {
	for _, delivery := range deliveries {
		if !delivery.Channels.Inbox {
			continue
		}

		s.broadcast.Publish(ctx, entity.Event{
			WorkspaceID: event.WorkspaceID,
			Kind:        entity.EventNotificationArrived,
			SubjectID:   event.Subject.ID,
			IssueID:     event.Subject.ID,
			AccountID:   delivery.AccountID,
			ActorID:     event.Actor,
		})
	}
}
