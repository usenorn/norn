package entity

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	ExecutionKeyPrefix = "executions"

	ExecutionArtifactNameMaxLen  = 255
	ExecutionContentTypeMaxLen   = 128
	ExecutionArtifactGenericType = AttachmentGenericType
)

var (
	ErrExecutionUploadTooLarge   = errors.New("that upload is larger than this instance accepts")
	ErrExecutionUploadExhausted  = errors.New("this execution has uploaded as much as it may")
	ErrExecutionUploadEmpty      = errors.New("that upload carries nothing to store")
	ErrExecutionUploadRecorded   = errors.New("this upload has already been recorded")
	ErrExecutionArtifactNotFound = errors.New("that artifact is not on this execution")
)

type ExecutionUploadTooLargeError struct {
	SizeBytes int64
	MaxBytes  int64
}

func (e ExecutionUploadTooLargeError) Error() string {
	return fmt.Sprintf("%s: %d of %d", ErrExecutionUploadTooLarge, e.SizeBytes, e.MaxBytes)
}

func (e ExecutionUploadTooLargeError) Unwrap() error {
	return ErrExecutionUploadTooLarge
}

type ExecutionUploadExhaustedError struct {
	SizeBytes     int64
	UploadedBytes int64
	MaxBytes      int64
}

func (e ExecutionUploadExhaustedError) Error() string {
	return fmt.Sprintf("%s: %d stored of %d", ErrExecutionUploadExhausted, e.UploadedBytes, e.MaxBytes)
}

func (e ExecutionUploadExhaustedError) Unwrap() error {
	return ErrExecutionUploadExhausted
}

type ExecutionArtifact struct {
	ID          uuid.UUID
	ExecutionID string
	WorkspaceID uuid.UUID
	Name        string
	ContentType string
	Bytes       int64
	Digest      string
	ObjectKey   string
	CreatedAt   time.Time
}

func ValidateExecutionArtifactName(field, name string) FieldError {
	trimmed := strings.TrimSpace(name)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case utf8.RuneCountInString(trimmed) > ExecutionArtifactNameMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

// ExecutionBlobPrefix nests an execution's objects under the workspace before the run, because
// purging a workspace sweeps by workspace prefix: a run-first key would survive the workspace it
// belonged to with nothing left that names it.
func ExecutionBlobPrefix(workspaceID uuid.UUID) string {
	return ExecutionKeyPrefix + "/" + workspaceID.String()
}

func executionRunPrefix(workspaceID uuid.UUID, executionID string) string {
	return ExecutionBlobPrefix(workspaceID) + "/" + executionID
}

func ExecutionArtifactKey(workspaceID uuid.UUID, executionID string, artifactID uuid.UUID) string {
	return executionRunPrefix(workspaceID, executionID) + "/artifacts/" + artifactID.String()
}
