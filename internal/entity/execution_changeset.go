package entity

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

const (
	ExecutionSummaryMaxLen     = 16000
	ExecutionCheckMaxLen       = 200
	ExecutionCheckDetailMaxLen = 4000
	ExecutionPullRequestMaxLen = 2000
	ExecutionPublicationMaxLen = 4000
	ExecutionRevisionMaxLen    = 64
	ExecutionChangesMax        = 50
	ExecutionValidationsMax    = 100
)

var (
	ErrExecutionResultNotFound = errors.New("this run has not reported what it changed")
	ErrExecutionResultStale    = errors.New("a newer report is already on record for this run")
	ErrExecutionChangeNotFound = errors.New("this run has not reported a change in that repository")
)

type PublicationState string

const (
	PublicationNone      PublicationState = ""
	PublicationPending   PublicationState = channelv1.PublicationPending
	PublicationPushed    PublicationState = channelv1.PublicationPushed
	PublicationPublished PublicationState = channelv1.PublicationPublished
	PublicationFailed    PublicationState = channelv1.PublicationFailed
)

func PublicationStates() []PublicationState {
	return []PublicationState{
		PublicationPending, PublicationPushed, PublicationPublished, PublicationFailed,
	}
}

func (s PublicationState) Valid() bool {
	return slices.Contains(PublicationStates(), s)
}

type PublicationStep string

const (
	PublicationStepNone        PublicationStep = ""
	PublicationStepPush        PublicationStep = channelv1.PublicationStepPush
	PublicationStepPullRequest PublicationStep = channelv1.PublicationStepPullRequest
)

func (s PublicationStep) Valid() bool {
	return s == PublicationStepNone || s == PublicationStepPush || s == PublicationStepPullRequest
}

type ExecutionPublication struct {
	State       PublicationState
	Step        PublicationStep
	Error       string
	SHA         string
	Revision    int
	PublishedAt *time.Time
}

func (p ExecutionPublication) Failed() bool {
	return p.State == PublicationFailed
}

type RepositoryPublication struct {
	Repository     string
	PullRequestURL string
	Publication    ExecutionPublication
}

type ValidationStatus string

const (
	ValidationPassed  ValidationStatus = channelv1.ValidationPassed
	ValidationFailed  ValidationStatus = channelv1.ValidationFailed
	ValidationSkipped ValidationStatus = channelv1.ValidationSkipped
)

func ValidationStatuses() []ValidationStatus {
	return []ValidationStatus{ValidationPassed, ValidationFailed, ValidationSkipped}
}

func (s ValidationStatus) Valid() bool {
	return slices.Contains(ValidationStatuses(), s)
}

type ExecutionResult struct {
	ExecutionID string
	WorkspaceID uuid.UUID
	Summary     string
	ReportedAt  time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ExecutionChange struct {
	ID             uuid.UUID
	ExecutionID    string
	WorkspaceID    uuid.UUID
	Repository     string
	Branch         string
	BaseSHA        string
	HeadSHA        string
	Commits        int
	Additions      int
	Deletions      int
	FilesChanged   int
	DiffArtifactID uuid.UUID
	PullRequestURL string
	CodeLinkID     uuid.UUID
	Publication    ExecutionPublication
	ReportedAt     time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ExecutionValidation struct {
	ID          uuid.UUID
	ExecutionID string
	WorkspaceID uuid.UUID
	Check       string
	Status      ValidationStatus
	Detail      string
	ArtifactID  uuid.UUID
	ReportedAt  time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ExecutionChangeSet struct {
	ExecutionID string
	Result      ExecutionResult
	Changes     []ExecutionChange
	Validations []ExecutionValidation
}

func (c ExecutionChangeSet) PublicationFailed() bool {
	return slices.ContainsFunc(c.Changes, func(change ExecutionChange) bool {
		return change.Publication.Failed()
	})
}

func (c ExecutionChangeSet) Empty() bool {
	return c.Result.ExecutionID == "" && len(c.Changes) == 0 && len(c.Validations) == 0
}

type ExecutionChangeSummary struct {
	Repositories int
	Commits      int
	Additions    int
	Deletions    int
	FilesChanged int
	PullRequests int
}

type ExecutionListing struct {
	Execution Execution
	Change    ExecutionChangeSummary
}

type IssueRepositoryChange struct {
	ExecutionChange
	Attempt int
}

type IssueChangeSet struct {
	IssueID uuid.UUID
	Changes []IssueRepositoryChange
}

func ValidateExecutionSummary(field, summary string) FieldError {
	if utf8.RuneCountInString(summary) > ExecutionSummaryMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}

func ValidateExecutionChange(field string, change ExecutionChange) error {
	return NewValidationError(
		requiredText(field+".repo", change.Repository, CodebaseRepositoryMaxLen),
		optionalText(field+".branch", change.Branch, CodebaseBranchMaxLen),
		optionalText(field+".baseSha", change.BaseSHA, ExecutionRevisionMaxLen),
		optionalText(field+".headSha", change.HeadSHA, ExecutionRevisionMaxLen),
		optionalText(field+".pullRequestUrl", change.PullRequestURL, ExecutionPullRequestMaxLen),
		notNegative(field+".commits", change.Commits),
		notNegative(field+".additions", change.Additions),
		notNegative(field+".deletions", change.Deletions),
		notNegative(field+".filesChanged", change.FilesChanged),
	)
}

func ValidateRepositoryPublication(field string, published RepositoryPublication) error {
	state := FieldError{}
	if !published.Publication.State.Valid() {
		state = FieldError{Field: field + ".state", Code: ValidationCodeUnsupportedValue}
	}

	step := FieldError{}
	if !published.Publication.Step.Valid() {
		step = FieldError{Field: field + ".step", Code: ValidationCodeUnsupportedValue}
	}

	return NewValidationError(
		requiredText(field+".repo", published.Repository, CodebaseRepositoryMaxLen),
		requiredText(field+".sha", published.Publication.SHA, ExecutionRevisionMaxLen),
		optionalText(field+".failure", published.Publication.Error, ExecutionPublicationMaxLen),
		optionalText(
			field+".pullRequestUrl", published.PullRequestURL, ExecutionPullRequestMaxLen,
		),
		state,
		step,
	)
}

func ValidateExecutionValidation(field string, validation ExecutionValidation) error {
	status := FieldError{}
	if !validation.Status.Valid() {
		status = FieldError{Field: field + ".status", Code: ValidationCodeUnsupportedValue}
	}

	return NewValidationError(
		requiredText(field+".check", validation.Check, ExecutionCheckMaxLen),
		optionalText(field+".detail", validation.Detail, ExecutionCheckDetailMaxLen),
		status,
	)
}

func requiredText(field, value string, limit int) FieldError {
	trimmed := strings.TrimSpace(value)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case utf8.RuneCountInString(trimmed) > limit:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func optionalText(field, value string, limit int) FieldError {
	if utf8.RuneCountInString(strings.TrimSpace(value)) > limit {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}

func notNegative(field string, value int) FieldError {
	if value < 0 {
		return FieldError{Field: field, Code: ValidationCodeOutOfRange}
	}

	return FieldError{}
}
