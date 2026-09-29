package entity

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	ReviewCommentBodyMaxLen  = 4000
	ReviewSummaryMaxLen      = ExecutionFeedbackMaxLen
	ReviewPathMaxLen         = 1024
	ReviewHunkMaxLines       = 12
	ReviewHunkMaxLen         = 4000
	ReviewCommentsPerReview  = 100
	ReviewCommentsPerRunMax  = 1000
	ReviewFeedbackExcerptSHA = 7
)

var (
	ErrReviewCommentNotFound = errors.New("review comment not found")
	ErrReviewCommentNotYours = errors.New("only the person who wrote that comment may change it")
	ErrReviewCommentAnchor   = errors.New("that line is not part of the changes under review")
	ErrReviewCommentReply    = errors.New("a reply answers a thread, not another reply")
	ErrReviewCommentsFull    = errors.New("this run already carries as many review comments as it can")
	ErrReviewStale           = errors.New("the changes moved on since this review was started")
	ErrReviewEmpty           = errors.New("asking for changes needs a summary or at least one comment")
	ErrReviewClosed          = errors.New("this run's changes are no longer under review")
)

type ReviewSide string

const (
	ReviewSideOld ReviewSide = "old"
	ReviewSideNew ReviewSide = "new"
)

func ReviewSides() []ReviewSide {
	return []ReviewSide{ReviewSideOld, ReviewSideNew}
}

func (s ReviewSide) Valid() bool {
	return slices.Contains(ReviewSides(), s)
}

type ExecutionReviewVerdict string

const (
	VerdictComment        ExecutionReviewVerdict = "comment"
	VerdictApprove        ExecutionReviewVerdict = "approve"
	VerdictRequestChanges ExecutionReviewVerdict = "request_changes"
)

func ExecutionReviewVerdicts() []ExecutionReviewVerdict {
	return []ExecutionReviewVerdict{VerdictComment, VerdictApprove, VerdictRequestChanges}
}

func (v ExecutionReviewVerdict) Valid() bool {
	return slices.Contains(ExecutionReviewVerdicts(), v)
}

type ReviewHeads map[string]string

func HeadsOf(changes []ExecutionChange) ReviewHeads {
	heads := make(ReviewHeads, len(changes))

	for _, change := range changes {
		heads[change.Repository] = change.HeadSHA
	}

	return heads
}

func (h ReviewHeads) Matches(other ReviewHeads) bool {
	return maps.Equal(h, other)
}

type ReviewAnchor struct {
	Repository string
	Path       string
	Side       ReviewSide
	Line       int
	HeadSHA    string
	Hunk       string
}

type ExecutionReviewComment struct {
	ID              uuid.UUID
	ExecutionID     string
	WorkspaceID     uuid.UUID
	ReviewID        uuid.UUID
	ParentID        uuid.UUID
	Anchor          ReviewAnchor
	Body            string
	AuthorAccountID uuid.UUID
	AuthorName      string
	CreatedAt       time.Time
	EditedAt        *time.Time
	ResolvedAt      *time.Time
	ResolvedByID    uuid.UUID
	ResolvedByName  string
}

func (c ExecutionReviewComment) Pending() bool {
	return c.ReviewID == uuid.Nil
}

func (c ExecutionReviewComment) Reply() bool {
	return c.ParentID != uuid.Nil
}

func (c ExecutionReviewComment) Resolved() bool {
	return c.ResolvedAt != nil
}

func (c ExecutionReviewComment) Outdated(heads ReviewHeads) bool {
	return heads[c.Anchor.Repository] != c.Anchor.HeadSHA
}

func (c ExecutionReviewComment) VisibleTo(accountID uuid.UUID) bool {
	return !c.Pending() || c.AuthorAccountID == accountID
}

type ExecutionReview struct {
	ID              uuid.UUID
	ExecutionID     string
	WorkspaceID     uuid.UUID
	Verdict         ExecutionReviewVerdict
	Summary         string
	Heads           ReviewHeads
	AuthorAccountID uuid.UUID
	AuthorName      string
	SubmittedAt     time.Time
}

func ValidateReviewAnchor(anchor ReviewAnchor) error {
	var fields []FieldError

	switch {
	case strings.TrimSpace(anchor.Repository) == "":
		fields = append(fields, FieldError{Field: "repository", Code: ValidationCodeRequired})
	case strings.TrimSpace(anchor.Path) == "":
		fields = append(fields, FieldError{Field: "path", Code: ValidationCodeRequired})
	case utf8.RuneCountInString(anchor.Path) > ReviewPathMaxLen:
		fields = append(fields, FieldError{Field: "path", Code: ValidationCodeTooLong})
	}

	if !anchor.Side.Valid() {
		fields = append(fields, FieldError{Field: "side", Code: ValidationCodeUnsupportedValue})
	}

	if anchor.Line < 1 {
		fields = append(fields, FieldError{Field: "line", Code: ValidationCodeOutOfRange})
	}

	if utf8.RuneCountInString(anchor.Hunk) > ReviewHunkMaxLen {
		fields = append(fields, FieldError{Field: "hunk", Code: ValidationCodeTooLong})
	}

	if len(fields) == 0 {
		return nil
	}

	return NewValidationError(fields...)
}

func ValidateReviewCommentBody(field, body string) FieldError {
	trimmed := strings.TrimSpace(body)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case utf8.RuneCountInString(trimmed) > ReviewCommentBodyMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateReviewSummary(field, summary string) FieldError {
	if utf8.RuneCountInString(strings.TrimSpace(summary)) > ReviewSummaryMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}

func TrimReviewHunk(hunk string) string {
	lines := strings.Split(strings.TrimRight(hunk, "\n"), "\n")

	if len(lines) > ReviewHunkMaxLines {
		lines = lines[len(lines)-ReviewHunkMaxLines:]
	}

	return strings.Join(lines, "\n")
}

func ComposeReviewFeedback(summary string, comments []ExecutionReviewComment) string {
	var feedback strings.Builder

	feedback.WriteString("A reviewer asked for changes before this work is published.\n")

	if trimmed := strings.TrimSpace(summary); trimmed != "" {
		feedback.WriteString("\n")
		feedback.WriteString(trimmed)
		feedback.WriteString("\n")
	}

	if len(comments) > 0 {
		feedback.WriteString("\nComments on the diff:\n")
	}

	for index, comment := range comments {
		anchor := comment.Anchor

		fmt.Fprintf(
			&feedback, "\n%d. %s:%s line %d (%s side, at %s)\n",
			index+1, anchor.Repository, anchor.Path, anchor.Line, anchor.Side,
			shortSHA(anchor.HeadSHA),
		)

		if anchor.Hunk != "" {
			feedback.WriteString("```diff\n")
			feedback.WriteString(anchor.Hunk)
			feedback.WriteString("\n```\n")
		}

		feedback.WriteString(strings.TrimSpace(comment.Body))
		feedback.WriteString("\n")
	}

	feedback.WriteString(
		"\nAddress every point, commit the changes, and finish again when the work is ready " +
			"for another review.",
	)

	return feedback.String()
}

func shortSHA(sha string) string {
	if len(sha) <= ReviewFeedbackExcerptSHA {
		return sha
	}

	return sha[:ReviewFeedbackExcerptSHA]
}
