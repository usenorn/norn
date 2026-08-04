package entity

import (
	"encoding/base64"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

const (
	IssueActivityPageDefaultSize = 50
	IssueActivityPageMaxSize     = 200
	IssueActivityCursorIDLen     = 36
)

var ErrIssueActivityCursorInvalid = errors.New("issue activity cursor is invalid")

type IssueActivityKind string

const (
	IssueActivityKindCreated         IssueActivityKind = "created"
	IssueActivityKindStateChanged    IssueActivityKind = "state_changed"
	IssueActivityKindPropertyChanged IssueActivityKind = "property_changed"
	IssueActivityKindTeamMoved       IssueActivityKind = "team_moved"
	IssueActivityKindArchived        IssueActivityKind = "archived"
	IssueActivityKindUnarchived      IssueActivityKind = "unarchived"
	IssueActivityKindDeleted         IssueActivityKind = "deleted"
	IssueActivityKindRestored        IssueActivityKind = "restored"
	IssueActivityKindChildAdded      IssueActivityKind = "child_added"
	IssueActivityKindChildRemoved    IssueActivityKind = "child_removed"
	IssueActivityKindRelationAdded   IssueActivityKind = "relation_added"
	IssueActivityKindRelationRemoved IssueActivityKind = "relation_removed"
)

func IssueActivityKinds() []IssueActivityKind {
	return []IssueActivityKind{
		IssueActivityKindCreated,
		IssueActivityKindStateChanged,
		IssueActivityKindPropertyChanged,
		IssueActivityKindTeamMoved,
		IssueActivityKindArchived,
		IssueActivityKindUnarchived,
		IssueActivityKindDeleted,
		IssueActivityKindRestored,
		IssueActivityKindChildAdded,
		IssueActivityKindChildRemoved,
		IssueActivityKindRelationAdded,
		IssueActivityKindRelationRemoved,
	}
}

func (k IssueActivityKind) Valid() bool {
	return slices.Contains(IssueActivityKinds(), k)
}

type IssueActivity struct {
	ID             uuid.UUID
	WorkspaceID    uuid.UUID
	IssueID        uuid.UUID
	ActorAccountID uuid.UUID
	ActorName      string
	Kind           IssueActivityKind
	FromStateID    uuid.UUID
	ToStateID      uuid.UUID
	FromState      string
	ToState        string
	Field          string
	FromValue      string
	ToValue        string
	Version        int
	CreatedAt      time.Time
}

func (a IssueActivity) Cursor() IssueActivityCursor {
	return IssueActivityCursor{CreatedAt: a.CreatedAt, ActivityID: a.ID}
}

type IssueActivityCursor struct {
	CreatedAt  time.Time
	ActivityID uuid.UUID
}

func (c IssueActivityCursor) Encode() string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte(c.ActivityID.String() + c.CreatedAt.UTC().Format(time.RFC3339Nano)),
	)
}

func DecodeIssueActivityCursor(raw string) (IssueActivityCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) < IssueActivityCursorIDLen {
		return IssueActivityCursor{}, ErrIssueActivityCursorInvalid
	}

	activityID, err := uuid.Parse(string(decoded[:IssueActivityCursorIDLen]))
	if err != nil {
		return IssueActivityCursor{}, ErrIssueActivityCursorInvalid
	}

	createdAt, err := time.Parse(time.RFC3339Nano, string(decoded[IssueActivityCursorIDLen:]))
	if err != nil {
		return IssueActivityCursor{}, ErrIssueActivityCursorInvalid
	}

	return IssueActivityCursor{CreatedAt: createdAt, ActivityID: activityID}, nil
}

type IssueActivityPage struct {
	Cursor *IssueActivityCursor
	Limit  int
}

func (p IssueActivityPage) Normalized() IssueActivityPage {
	if p.Limit <= 0 {
		p.Limit = IssueActivityPageDefaultSize
	}

	if p.Limit > IssueActivityPageMaxSize {
		p.Limit = IssueActivityPageMaxSize
	}

	return p
}

func (p IssueActivityPage) Lookahead() IssueActivityPage {
	p.Limit++

	return p
}
