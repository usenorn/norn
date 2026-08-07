package entity

import (
	"encoding/base64"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

const (
	ActivityPageDefaultSize = 50
	ActivityPageMaxSize     = 200
	ActivityCursorIDLen     = 36
)

var ErrActivityCursorInvalid = errors.New("activity cursor is invalid")

type ActivityKind string

const (
	ActivityKindCreated           ActivityKind = "created"
	ActivityKindStateChanged      ActivityKind = "state_changed"
	ActivityKindPropertyChanged   ActivityKind = "property_changed"
	ActivityKindTeamMoved         ActivityKind = "team_moved"
	ActivityKindArchived          ActivityKind = "archived"
	ActivityKindUnarchived        ActivityKind = "unarchived"
	ActivityKindDeleted           ActivityKind = "deleted"
	ActivityKindRestored          ActivityKind = "restored"
	ActivityKindChildAdded        ActivityKind = "child_added"
	ActivityKindChildRemoved      ActivityKind = "child_removed"
	ActivityKindRelationAdded     ActivityKind = "relation_added"
	ActivityKindRelationRemoved   ActivityKind = "relation_removed"
	ActivityKindTriaged           ActivityKind = "triaged"
	ActivityKindCommented         ActivityKind = "commented"
	ActivityKindCommentDeleted    ActivityKind = "comment_deleted"
	ActivityKindMemberAdded       ActivityKind = "member_added"
	ActivityKindMemberRemoved     ActivityKind = "member_removed"
	ActivityKindAttachmentAdded   ActivityKind = "attachment_added"
	ActivityKindAttachmentRemoved ActivityKind = "attachment_removed"
	ActivityKindCodeLinked        ActivityKind = "code_linked"
	ActivityKindCodeUnlinked      ActivityKind = "code_unlinked"
)

func ActivityKinds() []ActivityKind {
	return []ActivityKind{
		ActivityKindCreated,
		ActivityKindStateChanged,
		ActivityKindPropertyChanged,
		ActivityKindTeamMoved,
		ActivityKindArchived,
		ActivityKindUnarchived,
		ActivityKindDeleted,
		ActivityKindRestored,
		ActivityKindChildAdded,
		ActivityKindChildRemoved,
		ActivityKindRelationAdded,
		ActivityKindRelationRemoved,
		ActivityKindTriaged,
		ActivityKindCommented,
		ActivityKindCommentDeleted,
		ActivityKindMemberAdded,
		ActivityKindMemberRemoved,
		ActivityKindAttachmentAdded,
		ActivityKindAttachmentRemoved,
		ActivityKindCodeLinked,
		ActivityKindCodeUnlinked,
	}
}

func (k ActivityKind) Valid() bool {
	return slices.Contains(ActivityKinds(), k)
}

const (
	ActivityFieldAttachment = "attachment"
	ActivityFieldMember     = "member"
	ActivityFieldName       = "name"
	ActivityFieldLead       = "lead"
	ActivityFieldTarget     = "targetOn"
)

type ActivitySubjectKind string

const (
	ActivitySubjectIssue   ActivitySubjectKind = "issue"
	ActivitySubjectProject ActivitySubjectKind = "project"
)

type ActivitySubject struct {
	Kind ActivitySubjectKind
	ID   uuid.UUID
}

func IssueSubject(issueID uuid.UUID) ActivitySubject {
	return ActivitySubject{Kind: ActivitySubjectIssue, ID: issueID}
}

func ProjectSubject(projectID uuid.UUID) ActivitySubject {
	return ActivitySubject{Kind: ActivitySubjectProject, ID: projectID}
}

type ActivityOrder string

const (
	ActivityOrderOldest ActivityOrder = "oldest"
	ActivityOrderNewest ActivityOrder = "newest"
)

func (o ActivityOrder) Valid() bool {
	return o == ActivityOrderOldest || o == ActivityOrderNewest
}

func (o ActivityOrder) Normalized() ActivityOrder {
	if o.Valid() {
		return o
	}

	return ActivityOrderOldest
}

type Activity struct {
	ID           uuid.UUID
	OperationID  uuid.UUID
	WorkspaceID  uuid.UUID
	Subject      ActivitySubject
	Actor        ActivityAttribution
	ActorName    string
	Kind         ActivityKind
	FromState    string
	ToState      string
	Field        string
	FromValue    string
	ToValue      string
	Version      int
	BulkActionID uuid.UUID
	CreatedAt    time.Time
}

func (a Activity) Leader() bool {
	return a.ID == a.OperationID
}

type ActivityEvent struct {
	ID           uuid.UUID
	WorkspaceID  uuid.UUID
	Subject      ActivitySubject
	Actor        ActivityAttribution
	ActorName    string
	BulkActionID uuid.UUID
	Changes      []Activity
	CreatedAt    time.Time
}

func NewActivityEvent(leader Activity) ActivityEvent {
	return ActivityEvent{
		ID:           leader.OperationID,
		WorkspaceID:  leader.WorkspaceID,
		Subject:      leader.Subject,
		Actor:        leader.Actor,
		ActorName:    leader.ActorName,
		BulkActionID: leader.BulkActionID,
		CreatedAt:    leader.CreatedAt,
	}
}

func (e ActivityEvent) Cursor() ActivityCursor {
	return ActivityCursor{CreatedAt: e.CreatedAt, OperationID: e.ID}
}

type ActivityCursor struct {
	CreatedAt   time.Time
	OperationID uuid.UUID
}

func (c ActivityCursor) Encode() string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte(c.OperationID.String() + c.CreatedAt.UTC().Format(time.RFC3339Nano)),
	)
}

func DecodeActivityCursor(raw string) (ActivityCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) < ActivityCursorIDLen {
		return ActivityCursor{}, ErrActivityCursorInvalid
	}

	operationID, err := uuid.Parse(string(decoded[:ActivityCursorIDLen]))
	if err != nil {
		return ActivityCursor{}, ErrActivityCursorInvalid
	}

	createdAt, err := time.Parse(time.RFC3339Nano, string(decoded[ActivityCursorIDLen:]))
	if err != nil {
		return ActivityCursor{}, ErrActivityCursorInvalid
	}

	return ActivityCursor{CreatedAt: createdAt, OperationID: operationID}, nil
}

type ActivityPage struct {
	Cursor *ActivityCursor
	Order  ActivityOrder
	Limit  int
}

func (p ActivityPage) Normalized() ActivityPage {
	if p.Limit <= 0 {
		p.Limit = ActivityPageDefaultSize
	}

	if p.Limit > ActivityPageMaxSize {
		p.Limit = ActivityPageMaxSize
	}

	p.Order = p.Order.Normalized()

	return p
}

func (p ActivityPage) Lookahead() ActivityPage {
	p.Limit++

	return p
}
