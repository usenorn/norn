package entity

import (
	"encoding/base64"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

const (
	NotificationPageDefaultSize = 25
	NotificationPageMaxSize     = 100
	NotificationCursorIDLen     = 36
	NotificationDigestWindow    = 15 * time.Minute
	NotificationFanOutBatchSize = 200
)

var (
	ErrNotificationCursorInvalid = errors.New("notification cursor is invalid")
	ErrNotificationNotFound      = errors.New("notification not found")
	ErrSnoozeNotInFuture         = errors.New("snooze time is not in the future")
)

type NotificationKind string

const (
	NotificationKindAssigned     NotificationKind = "assigned"
	NotificationKindMentioned    NotificationKind = "mentioned"
	NotificationKindCommented    NotificationKind = "commented"
	NotificationKindStateChanged NotificationKind = "state_changed"
	NotificationKindMembership   NotificationKind = "membership"
	NotificationKindCheckFailed  NotificationKind = "check_failed"
	NotificationKindGapDeclared  NotificationKind = "gap_declared"

	NotificationKindApprovalWaiting NotificationKind = "approval_waiting"
)

func NotificationKinds() []NotificationKind {
	return []NotificationKind{
		NotificationKindAssigned,
		NotificationKindMentioned,
		NotificationKindCommented,
		NotificationKindStateChanged,
		NotificationKindMembership,
		NotificationKindCheckFailed,
		NotificationKindGapDeclared,
		NotificationKindApprovalWaiting,
	}
}

func (k NotificationKind) Valid() bool {
	return slices.Contains(NotificationKinds(), k)
}

func (k NotificationKind) Recordable() bool {
	return k.Valid() && k != NotificationKindMentioned
}

type NotificationSubjectKind string

const (
	NotificationSubjectIssue   NotificationSubjectKind = "issue"
	NotificationSubjectProject NotificationSubjectKind = "project"
	NotificationSubjectTeam    NotificationSubjectKind = "team"
)

func (k NotificationSubjectKind) Valid() bool {
	switch k {
	case NotificationSubjectIssue, NotificationSubjectProject, NotificationSubjectTeam:
		return true
	default:
		return false
	}
}

type NotificationSubject struct {
	Kind NotificationSubjectKind
	ID   uuid.UUID
}

func NotifyIssue(issueID uuid.UUID) NotificationSubject {
	return NotificationSubject{Kind: NotificationSubjectIssue, ID: issueID}
}

func NotifyProject(projectID uuid.UUID) NotificationSubject {
	return NotificationSubject{Kind: NotificationSubjectProject, ID: projectID}
}

func NotifyTeam(teamID uuid.UUID) NotificationSubject {
	return NotificationSubject{Kind: NotificationSubjectTeam, ID: teamID}
}

type NotificationReason string

const (
	NotificationReasonMentioned  NotificationReason = "mentioned"
	NotificationReasonApproval   NotificationReason = "approval"
	NotificationReasonAssigned   NotificationReason = "assigned"
	NotificationReasonMembership NotificationReason = "membership"
	NotificationReasonFollowing  NotificationReason = "following"
)

func NotificationReasons() []NotificationReason {
	return []NotificationReason{
		NotificationReasonMentioned,
		NotificationReasonApproval,
		NotificationReasonAssigned,
		NotificationReasonMembership,
		NotificationReasonFollowing,
	}
}

func (r NotificationReason) Valid() bool {
	return slices.Contains(NotificationReasons(), r)
}

func (r NotificationReason) Rank() int {
	return slices.Index(NotificationReasons(), r)
}

func (r NotificationReason) Governs(kind NotificationKind) NotificationKind {
	if r == NotificationReasonMentioned {
		return NotificationKindMentioned
	}

	return kind
}

func (r NotificationReason) Strongest(other NotificationReason) NotificationReason {
	if !other.Valid() {
		return r
	}

	if !r.Valid() || other.Rank() < r.Rank() {
		return other
	}

	return r
}

type FollowState string

const (
	FollowStateFollowing FollowState = "following"
	FollowStateMuted     FollowState = "muted"
)

func (s FollowState) Valid() bool {
	return s == FollowStateFollowing || s == FollowStateMuted
}

func (s FollowState) Following() bool {
	return s == FollowStateFollowing
}

type IssueFollower struct {
	IssueID     uuid.UUID
	WorkspaceID uuid.UUID
	AccountID   uuid.UUID
	State       FollowState
}

type NotificationEvent struct {
	ID           uuid.UUID
	WorkspaceID  uuid.UUID
	Subject      NotificationSubject
	Kind         NotificationKind
	Actor        uuid.UUID
	ActorKind    ActorKind
	Target       uuid.UUID
	CommentID    uuid.UUID
	BulkActionID uuid.UUID
	CreatedAt    time.Time
}

type NotificationChannels struct {
	Inbox bool
	Email bool
}

type NotificationPreferences struct {
	Assigned     NotificationChannels
	Mentioned    NotificationChannels
	Commented    NotificationChannels
	StateChanged NotificationChannels
	Membership   NotificationChannels
	Checks       NotificationChannels
	Approvals    NotificationChannels
	Agents       NotificationChannels
}

func DefaultNotificationPreferences() NotificationPreferences {
	return NotificationPreferences{
		Assigned:     NotificationChannels{Inbox: true, Email: true},
		Mentioned:    NotificationChannels{Inbox: true, Email: true},
		Commented:    NotificationChannels{Inbox: true, Email: false},
		StateChanged: NotificationChannels{Inbox: true, Email: false},
		Membership:   NotificationChannels{Inbox: true, Email: false},
		Checks:       NotificationChannels{Inbox: true, Email: false},
		Approvals:    NotificationChannels{Inbox: true, Email: true},
		Agents:       NotificationChannels{Inbox: true, Email: true},
	}
}

func (p NotificationPreferences) For(kind NotificationKind) NotificationChannels {
	switch kind {
	case NotificationKindAssigned:
		return p.Assigned
	case NotificationKindMentioned:
		return p.Mentioned
	case NotificationKindCommented:
		return p.Commented
	case NotificationKindStateChanged:
		return p.StateChanged
	case NotificationKindMembership:
		return p.Membership
	case NotificationKindCheckFailed, NotificationKindGapDeclared:
		return p.Checks
	case NotificationKindApprovalWaiting:
		return p.Approvals
	default:
		return NotificationChannels{}
	}
}

func (p NotificationPreferences) Delivers(kind NotificationKind, actor ActorKind) NotificationChannels {
	channels := p.For(kind)
	if actor != ActorKindAgent {
		return channels
	}

	return NotificationChannels{
		Inbox: channels.Inbox && p.Agents.Inbox,
		Email: channels.Email && p.Agents.Email,
	}
}

type NotificationSettings struct {
	WorkspaceID uuid.UUID
	AccountID   uuid.UUID
	TeamID      uuid.UUID
	Preferences NotificationPreferences
}

func (s NotificationSettings) Global() bool {
	return s.TeamID == uuid.Nil
}

func ResolveNotificationPreferences(settings []NotificationSettings, teamID uuid.UUID) NotificationPreferences {
	resolved := DefaultNotificationPreferences()

	for _, setting := range settings {
		if setting.Global() {
			resolved = setting.Preferences
		}
	}

	if teamID == uuid.Nil {
		return resolved
	}

	for _, setting := range settings {
		if setting.TeamID == teamID {
			return setting.Preferences
		}
	}

	return resolved
}

type Notification struct {
	WorkspaceID  uuid.UUID
	AccountID    uuid.UUID
	Subject      NotificationSubject
	Reason       NotificationReason
	Kind         NotificationKind
	Actor        uuid.UUID
	ActorName    string
	ActorKind    ActorKind
	Title        string
	Reference    string
	TeamKey      string
	UnreadCount  int
	LastEventAt  time.Time
	ReadThrough  time.Time
	SnoozedUntil time.Time
}

func (n Notification) Unread() bool {
	return n.UnreadCount > 0
}

func (n Notification) Cursor() NotificationCursor {
	return NotificationCursor{LastEventAt: n.LastEventAt, SubjectID: n.Subject.ID}
}

type NotificationFilter string

const (
	NotificationFilterUnread NotificationFilter = "unread"
	NotificationFilterAll    NotificationFilter = "all"
)

func (f NotificationFilter) Valid() bool {
	return f == NotificationFilterUnread || f == NotificationFilterAll
}

func (f NotificationFilter) Normalized() NotificationFilter {
	if f.Valid() {
		return f
	}

	return NotificationFilterUnread
}

type NotificationCursor struct {
	LastEventAt time.Time
	SubjectID   uuid.UUID
}

func (c NotificationCursor) Encode() string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte(c.SubjectID.String() + c.LastEventAt.UTC().Format(time.RFC3339Nano)),
	)
}

func DecodeNotificationCursor(raw string) (NotificationCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) < NotificationCursorIDLen {
		return NotificationCursor{}, ErrNotificationCursorInvalid
	}

	subjectID, err := uuid.Parse(string(decoded[:NotificationCursorIDLen]))
	if err != nil {
		return NotificationCursor{}, ErrNotificationCursorInvalid
	}

	lastEventAt, err := time.Parse(time.RFC3339Nano, string(decoded[NotificationCursorIDLen:]))
	if err != nil {
		return NotificationCursor{}, ErrNotificationCursorInvalid
	}

	return NotificationCursor{LastEventAt: lastEventAt, SubjectID: subjectID}, nil
}

type NotificationPage struct {
	Cursor *NotificationCursor
	Filter NotificationFilter
	Limit  int
}

func (p NotificationPage) Normalized() NotificationPage {
	if p.Limit <= 0 {
		p.Limit = NotificationPageDefaultSize
	}

	if p.Limit > NotificationPageMaxSize {
		p.Limit = NotificationPageMaxSize
	}

	p.Filter = p.Filter.Normalized()

	return p
}

func (p NotificationPage) Lookahead() NotificationPage {
	p.Limit++

	return p
}

type NotificationDigestClaim struct {
	WorkspaceID uuid.UUID
	AccountID   uuid.UUID
	Window      time.Time
	Email       string
	Timezone    string
}

func NotificationDigestWindowAt(now time.Time) time.Time {
	filling := now.UTC().Truncate(NotificationDigestWindow)

	return filling.Add(-2 * NotificationDigestWindow)
}
