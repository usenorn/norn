package entity

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	EventStreamWindow     = 10000
	EventSubscriberBuffer = 64
	EventHeartbeat        = 20 * time.Second
	EventBlockSeconds     = 30
	EventScopeRefresh     = 2 * time.Minute
	EventRetryDelay       = 2 * time.Second
	EventHandshakeTimeout = 10 * time.Second
)

var (
	ErrEventCursorInvalid = errors.New("event cursor is invalid")
	ErrEventStreamLapsed  = errors.New("the event a client resumed from is no longer in the stream")
)

type EventKind string

const (
	EventIssueCreated        EventKind = "issue.created"
	EventIssueUpdated        EventKind = "issue.updated"
	EventIssueDeleted        EventKind = "issue.deleted"
	EventCommentPosted       EventKind = "comment.posted"
	EventCommentEdited       EventKind = "comment.edited"
	EventCommentDeleted      EventKind = "comment.deleted"
	EventNotificationArrived EventKind = "notification.arrived"
	EventMembershipChanged   EventKind = "membership.changed"
	EventExecutionUpdated    EventKind = "execution.updated"
	EventExecutionEvent      EventKind = "execution.event"
	EventExecutionChangeSet  EventKind = "execution.changeset"
	EventQuestionAsked       EventKind = "question.asked"
	EventQuestionSettled     EventKind = "question.settled"
)

func EventKinds() []EventKind {
	return []EventKind{
		EventIssueCreated,
		EventIssueUpdated,
		EventIssueDeleted,
		EventCommentPosted,
		EventCommentEdited,
		EventCommentDeleted,
		EventNotificationArrived,
		EventMembershipChanged,
		EventExecutionUpdated,
		EventExecutionEvent,
		EventExecutionChangeSet,
		EventQuestionAsked,
		EventQuestionSettled,
	}
}

func (k EventKind) Valid() bool {
	return slices.Contains(EventKinds(), k)
}

func (k EventKind) Rescopes() bool {
	return k == EventMembershipChanged
}

type Event struct {
	ID          string
	WorkspaceID uuid.UUID
	Kind        EventKind
	TeamID      uuid.UUID
	SubjectID   uuid.UUID
	IssueID     uuid.UUID
	AccountID   uuid.UUID
	ActorID     uuid.UUID
	Payload     []byte
}

func (e Event) Reaches(accountID uuid.UUID, scope TeamScope) bool {
	if e.AccountID != uuid.Nil && e.AccountID != accountID {
		return false
	}

	if e.TeamID == uuid.Nil {
		return true
	}

	return scope.Covers(e.TeamID)
}

type EventTopic string

const (
	EventTopicWorkspace EventTopic = "workspace"
	EventTopicIssue     EventTopic = "issue"
	EventTopicInbox     EventTopic = "inbox"
)

func (t EventTopic) Valid() bool {
	switch t {
	case EventTopicWorkspace, EventTopicIssue, EventTopicInbox:
		return true
	default:
		return false
	}
}

type EventSubscription struct {
	Topics  []EventTopic
	IssueID uuid.UUID
}

func ParseEventSubscription(topics string, issueID uuid.UUID) EventSubscription {
	subscription := EventSubscription{IssueID: issueID}

	for _, raw := range strings.Split(topics, ",") {
		topic := EventTopic(strings.TrimSpace(raw))

		if topic.Valid() && !slices.Contains(subscription.Topics, topic) {
			subscription.Topics = append(subscription.Topics, topic)
		}
	}

	if len(subscription.Topics) == 0 {
		subscription.Topics = DefaultEventTopics()
	}

	return subscription
}

func DefaultEventTopics() []EventTopic {
	return []EventTopic{EventTopicWorkspace, EventTopicInbox}
}

func (s EventSubscription) topics() []EventTopic {
	if len(s.Topics) == 0 {
		return DefaultEventTopics()
	}

	return s.Topics
}

func (s EventSubscription) Wants(event Event) bool {
	topics := s.topics()

	switch {
	case event.Kind == EventNotificationArrived:
		return slices.Contains(topics, EventTopicInbox)
	case event.IssueID != uuid.Nil && s.IssueID == event.IssueID:
		return slices.Contains(topics, EventTopicIssue) ||
			slices.Contains(topics, EventTopicWorkspace)
	default:
		return slices.Contains(topics, EventTopicWorkspace)
	}
}
