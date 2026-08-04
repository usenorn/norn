package entity

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	IssueTitleMinLen     = 1
	IssueTitleMaxLen     = 200
	IssuePageDefaultSize = 50
	IssuePageMaxSize     = 200
	IssueCursorIDLen     = 36
)

var (
	ErrIssueNotFound       = errors.New("issue not found")
	ErrIssueCursorInvalid  = errors.New("issue cursor is invalid")
	ErrIssueReferenceTaken = errors.New("issue reference already used in this workspace")
)

type IssueState struct {
	ID       uuid.UUID
	Name     string
	Category StateCategory
	Position int
}

type IssueProgress struct {
	NotStarted int
	Active     int
	Complete   int
	Abandoned  int
}

func (p *IssueProgress) Add(category StateCategory, issues int) bool {
	switch category {
	case StateCategoryNotStarted:
		p.NotStarted += issues
	case StateCategoryActive:
		p.Active += issues
	case StateCategoryComplete:
		p.Complete += issues
	case StateCategoryAbandoned:
		p.Abandoned += issues
	default:
		return false
	}

	return true
}

type Issue struct {
	ID                 uuid.UUID
	WorkspaceID        uuid.UUID
	TeamID             uuid.UUID
	TeamKey            string
	ReferenceKey       string
	State              IssueState
	Labels             []Label
	Number             int
	Version            int
	FieldVersions      map[string]int
	Title              string
	Description        string
	Priority           IssuePriority
	AssigneeAccountID  uuid.UUID
	Estimate           int
	DueOn              string
	StateEnteredAt     time.Time
	CompletedAt        *time.Time
	Status             IssueStatus
	ArchivedAt         *time.Time
	ParentIssueID      uuid.UUID
	ParentReference    string
	Depth              int
	Children           IssueProgress
	Blocked            bool
	CreatedByAccountID uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (i Issue) Reference() string {
	return i.ReferenceKey + "-" + strconv.Itoa(i.Number)
}

var ErrIssueReferenceInvalid = errors.New("issue reference is not a team key and a number")

type IssueReference struct {
	Key    string
	Number int
}

func ParseIssueReference(raw string) (IssueReference, error) {
	key, digits, found := strings.Cut(strings.TrimSpace(raw), "-")
	if !found {
		return IssueReference{}, ErrIssueReferenceInvalid
	}

	key = strings.ToUpper(key)

	if !teamKeyPattern.MatchString(key) {
		return IssueReference{}, ErrIssueReferenceInvalid
	}

	number, err := strconv.Atoi(digits)
	if err != nil || number < 1 {
		return IssueReference{}, ErrIssueReferenceInvalid
	}

	return IssueReference{Key: key, Number: number}, nil
}

func ValidateIssueTitle(field, title string) FieldError {
	length := utf8.RuneCountInString(strings.TrimSpace(title))

	switch {
	case length < IssueTitleMinLen:
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case length > IssueTitleMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func (i Issue) Cursor() IssueCursor {
	return IssueCursor{CreatedAt: i.CreatedAt, IssueID: i.ID}
}

type IssueCursor struct {
	CreatedAt time.Time
	IssueID   uuid.UUID
}

func (c IssueCursor) Encode() string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte(c.IssueID.String() + c.CreatedAt.UTC().Format(time.RFC3339Nano)),
	)
}

func DecodeIssueCursor(raw string) (IssueCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) < IssueCursorIDLen {
		return IssueCursor{}, ErrIssueCursorInvalid
	}

	issueID, err := uuid.Parse(string(decoded[:IssueCursorIDLen]))
	if err != nil {
		return IssueCursor{}, ErrIssueCursorInvalid
	}

	createdAt, err := time.Parse(time.RFC3339Nano, string(decoded[IssueCursorIDLen:]))
	if err != nil {
		return IssueCursor{}, ErrIssueCursorInvalid
	}

	return IssueCursor{CreatedAt: createdAt, IssueID: issueID}, nil
}

type IssuePage struct {
	Cursor   *IssueCursor
	Limit    int
	Statuses []IssueStatus
}

func (p IssuePage) Normalized() IssuePage {
	if p.Limit <= 0 {
		p.Limit = IssuePageDefaultSize
	}

	if p.Limit > IssuePageMaxSize {
		p.Limit = IssuePageMaxSize
	}

	return p
}

func (p IssuePage) Lookahead() IssuePage {
	p.Limit++

	return p
}
