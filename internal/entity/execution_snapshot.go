package entity

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

const (
	ExecutionSnapshotCommitsMax       = channelv1.CommitsReported
	ExecutionSnapshotCommitSubjectMax = 500
	ExecutionSnapshotPreviewsMax      = 32
	ExecutionSnapshotPreviewNameMax   = 100
	ExecutionSnapshotPreviewPathMax   = 500
	ExecutionSnapshotPreviewReasonMax = 2000
	ExecutionSnapshotPortMax          = 65535

	PreviewFixReasonMax = 300
)

var ErrExecutionSnapshotNotFound = errors.New("this run has no review snapshot at that revision")

type SnapshotPreviewState string

const (
	SnapshotPreviewReady       SnapshotPreviewState = channelv1.PreviewReady
	SnapshotPreviewFailed      SnapshotPreviewState = channelv1.PreviewFailed
	SnapshotPreviewUnsupported SnapshotPreviewState = channelv1.PreviewUnsupported
)

func SnapshotPreviewStates() []SnapshotPreviewState {
	return []SnapshotPreviewState{
		SnapshotPreviewReady, SnapshotPreviewFailed, SnapshotPreviewUnsupported,
	}
}

func (s SnapshotPreviewState) Valid() bool {
	return slices.Contains(SnapshotPreviewStates(), s)
}

type SnapshotCommit struct {
	SHA     string
	Subject string
}

type SnapshotRepository struct {
	Repository     string
	Branch         string
	BaseSHA        string
	HeadSHA        string
	Commits        []SnapshotCommit
	Additions      int
	Deletions      int
	FilesChanged   int
	DiffArtifactID uuid.UUID
}

type SnapshotPreview struct {
	Name    string
	Service string
	Path    string
	State   SnapshotPreviewState
	Reason  string
	Port    int
}

type ExecutionSnapshot struct {
	ID           uuid.UUID
	ExecutionID  string
	WorkspaceID  uuid.UUID
	Revision     int
	Summary      string
	Repositories []SnapshotRepository
	Previews     []SnapshotPreview
	ReportedAt   time.Time
	CreatedAt    time.Time
}

func (s ExecutionSnapshot) Heads() ReviewHeads {
	heads := make(ReviewHeads, len(s.Repositories))

	for _, repository := range s.Repositories {
		heads[repository.Repository] = repository.HeadSHA
	}

	return heads
}

func (s ExecutionSnapshot) FailedPreviews() []SnapshotPreview {
	failed := make([]SnapshotPreview, 0, len(s.Previews))

	for _, preview := range s.Previews {
		if preview.State == SnapshotPreviewFailed {
			failed = append(failed, preview)
		}
	}

	return failed
}

func PreviewFixRequest(failed []SnapshotPreview) string {
	var built strings.Builder

	built.WriteString("These previews did not start, so nobody could try the change:\n\n")

	for _, preview := range failed {
		reason := strings.TrimSpace(preview.Reason)
		if reason == "" {
			reason = "no reason was given"
		}

		if runes := []rune(reason); len(runes) > PreviewFixReasonMax {
			reason = string(runes[:PreviewFixReasonMax]) + "…"
		}

		fmt.Fprintf(&built, "- %s: %s\n", preview.Name, reason)
	}

	built.WriteString("\nMake them start. If the fix is in the code, change it on this branch; " +
		"if it is in how the codebase is run, say what has to change.")

	request := built.String()
	if runes := []rune(request); len(runes) > ReviewSummaryMaxLen {
		request = string(runes[:ReviewSummaryMaxLen])
	}

	return request
}

type ExecutionRevision struct {
	Revision   int
	Additions  int
	Deletions  int
	ReportedAt time.Time
}

func ValidateSnapshotRepository(field string, repository SnapshotRepository) error {
	problems := []FieldError{
		requiredText(field+".repo", repository.Repository, CodebaseRepositoryMaxLen),
		optionalText(field+".branch", repository.Branch, CodebaseBranchMaxLen),
		optionalText(field+".baseSha", repository.BaseSHA, ExecutionRevisionMaxLen),
		optionalText(field+".headSha", repository.HeadSHA, ExecutionRevisionMaxLen),
		notNegative(field+".additions", repository.Additions),
		notNegative(field+".deletions", repository.Deletions),
		notNegative(field+".filesChanged", repository.FilesChanged),
	}

	if len(repository.Commits) > ExecutionSnapshotCommitsMax {
		problems = append(problems, FieldError{Field: field + ".commits", Code: ValidationCodeTooLong})
	}

	for _, commit := range repository.Commits {
		problems = append(problems,
			requiredText(field+".commits.sha", commit.SHA, ExecutionRevisionMaxLen),
			optionalText(field+".commits.subject", commit.Subject, ExecutionSnapshotCommitSubjectMax),
		)
	}

	return NewValidationError(problems...)
}

func ValidateSnapshotPreview(field string, preview SnapshotPreview) error {
	state := FieldError{}
	if !preview.State.Valid() {
		state = FieldError{Field: field + ".state", Code: ValidationCodeUnsupportedValue}
	}

	port := FieldError{}
	if preview.Port < 0 || preview.Port > ExecutionSnapshotPortMax {
		port = FieldError{Field: field + ".port", Code: ValidationCodeOutOfRange}
	}

	return NewValidationError(
		requiredText(field+".name", preview.Name, ExecutionSnapshotPreviewNameMax),
		requiredText(field+".service", preview.Service, ExecutionSnapshotPreviewNameMax),
		optionalText(field+".path", preview.Path, ExecutionSnapshotPreviewPathMax),
		optionalText(field+".reason", preview.Reason, ExecutionSnapshotPreviewReasonMax),
		state,
		port,
	)
}
