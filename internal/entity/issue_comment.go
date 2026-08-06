package entity

import (
	"encoding/base64"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	CommentBodyMaxLen      = IssueDescriptionMaxLen
	CommentMaxMentions     = 20
	CommentPageDefaultSize = 25
	CommentPageMaxSize     = 100
	CommentCursorIDLen     = 36
)

var (
	ErrIssueCommentNotFound     = errors.New("comment not found")
	ErrIssueCommentDeleted      = errors.New("comment has been deleted")
	ErrIssueCommentNotAuthor    = errors.New("only the person who wrote a comment may edit it")
	ErrIssueCommentNotDeletable = errors.New("only the author or an administrator may delete a comment")
	ErrIssueCommentNotReplyable = errors.New("a reply can only be left on a top-level comment")
	ErrTooManyMentions          = errors.New("too many people mentioned in one comment")
	ErrCommentCursorInvalid     = errors.New("comment cursor is invalid")
)

type CommentReaction string

const (
	CommentReactionUp        CommentReaction = "up"
	CommentReactionDown      CommentReaction = "down"
	CommentReactionCelebrate CommentReaction = "celebrate"
	CommentReactionThinking  CommentReaction = "thinking"
	CommentReactionEyes      CommentReaction = "eyes"
	CommentReactionHeart     CommentReaction = "heart"
)

func CommentReactions() []CommentReaction {
	return []CommentReaction{
		CommentReactionUp,
		CommentReactionDown,
		CommentReactionCelebrate,
		CommentReactionThinking,
		CommentReactionEyes,
		CommentReactionHeart,
	}
}

func (r CommentReaction) Valid() bool {
	return slices.Contains(CommentReactions(), r)
}

type MentionKind string

const (
	MentionKindAccount MentionKind = "account"
	MentionKindTeam    MentionKind = "team"
)

func MentionKinds() []MentionKind {
	return []MentionKind{MentionKindAccount, MentionKindTeam}
}

func (k MentionKind) Valid() bool {
	return slices.Contains(MentionKinds(), k)
}

type CommentMention struct {
	Kind      MentionKind
	AccountID uuid.UUID
	TeamID    uuid.UUID
	Name      string
	Visible   bool
}

func (m CommentMention) TargetID() uuid.UUID {
	if m.Kind == MentionKindTeam {
		return m.TeamID
	}

	return m.AccountID
}

type CommentReactionTally struct {
	Reaction CommentReaction
	Accounts []uuid.UUID
}

func (t CommentReactionTally) Includes(accountID uuid.UUID) bool {
	return slices.Contains(t.Accounts, accountID)
}

type IssueComment struct {
	ID              uuid.UUID
	WorkspaceID     uuid.UUID
	IssueID         uuid.UUID
	ParentCommentID uuid.UUID
	AuthorAccountID uuid.UUID
	AuthorName      string
	AuthorKind      AccountKind
	Body            string
	EditedAt        *time.Time
	DeletedAt       *time.Time
	Mentions        []CommentMention
	Reactions       []CommentReactionTally
	Replies         []IssueComment
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Origin          *ImportOrigin
}

func (c IssueComment) Deleted() bool {
	return c.DeletedAt != nil
}

func (c IssueComment) Edited() bool {
	return c.EditedAt != nil
}

func (c IssueComment) Root() bool {
	return c.ParentCommentID == uuid.Nil
}

func (c IssueComment) WrittenBy(accountID uuid.UUID) bool {
	return c.AuthorAccountID != uuid.Nil && c.AuthorAccountID == accountID
}

func (c IssueComment) EditableBy(accountID uuid.UUID) bool {
	return !c.Deleted() && c.WrittenBy(accountID)
}

func (c IssueComment) DeletableBy(accountID uuid.UUID, role MembershipRole) bool {
	if c.Deleted() {
		return false
	}

	return c.WrittenBy(accountID) || role == MembershipRoleAdmin
}

func ValidateCommentBody(field, body string) FieldError {
	if strings.ContainsRune(body, 0) {
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	}

	if strings.TrimSpace(body) == "" {
		return FieldError{Field: field, Code: ValidationCodeRequired}
	}

	if utf8.RuneCountInString(body) > CommentBodyMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}

func ValidateMentionCount(field string, mentions int) FieldError {
	if mentions > CommentMaxMentions {
		return FieldError{Field: field, Code: ValidationCodeOutOfRange}
	}

	return FieldError{}
}

type CommentCursor struct {
	CreatedAt time.Time
	CommentID uuid.UUID
}

func (c IssueComment) Cursor() CommentCursor {
	return CommentCursor{CreatedAt: c.CreatedAt, CommentID: c.ID}
}

func (c CommentCursor) Encode() string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte(c.CommentID.String() + c.CreatedAt.UTC().Format(time.RFC3339Nano)),
	)
}

func DecodeCommentCursor(raw string) (CommentCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) < CommentCursorIDLen {
		return CommentCursor{}, ErrCommentCursorInvalid
	}

	commentID, err := uuid.Parse(string(decoded[:CommentCursorIDLen]))
	if err != nil {
		return CommentCursor{}, ErrCommentCursorInvalid
	}

	createdAt, err := time.Parse(time.RFC3339Nano, string(decoded[CommentCursorIDLen:]))
	if err != nil {
		return CommentCursor{}, ErrCommentCursorInvalid
	}

	return CommentCursor{CreatedAt: createdAt, CommentID: commentID}, nil
}

type CommentPage struct {
	Cursor *CommentCursor
	Limit  int
}

func (p CommentPage) Normalized() CommentPage {
	if p.Limit <= 0 {
		p.Limit = CommentPageDefaultSize
	}

	if p.Limit > CommentPageMaxSize {
		p.Limit = CommentPageMaxSize
	}

	return p
}

func (p CommentPage) Lookahead() CommentPage {
	p.Limit++

	return p
}
