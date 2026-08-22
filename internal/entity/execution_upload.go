package entity

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	ExecutionKeyPrefix = "executions"

	ExecutionArtifactNameMaxLen  = 255
	ExecutionEntrySourceMaxLen   = 128
	ExecutionContentTypeMaxLen   = 128
	ExecutionEntryTypeMaxLen     = 64
	ExecutionChunkMaxEntries     = 5000
	ExecutionChunkPageDefault    = 10
	ExecutionChunkPageMax        = 50
	ExecutionRetentionDaysMin    = 1
	ExecutionRetentionDaysMax    = 3650
	ExecutionArtifactGenericType = AttachmentGenericType
)

var (
	ErrExecutionUploadTooLarge  = errors.New("that upload is larger than this instance accepts")
	ErrExecutionUploadExhausted = errors.New("this execution has uploaded as much as it may")
	ErrExecutionUploadEmpty     = errors.New("that upload carries nothing to store")
	ErrExecutionUploadCrowded   = errors.New(
		"that batch carries more entries than this server takes at once",
	)
	ErrExecutionSequenceInvalid = errors.New(
		"a batch needs a position in its stream, counting from one",
	)
	ErrExecutionUploadRecorded   = errors.New("this batch has already been recorded")
	ErrExecutionChunkConflict    = errors.New("a different batch was already recorded at that position")
	ErrExecutionTelemetryMinimal = errors.New(
		"this workspace keeps summaries only, so a full transcript is not accepted",
	)
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

type ExecutionStream string

const (
	ExecutionStreamLogs       ExecutionStream = "logs"
	ExecutionStreamTranscript ExecutionStream = "transcript"
)

func ExecutionStreams() []ExecutionStream {
	return []ExecutionStream{ExecutionStreamLogs, ExecutionStreamTranscript}
}

func (s ExecutionStream) Valid() bool {
	return slices.Contains(ExecutionStreams(), s)
}

type TelemetryMode string

const (
	TelemetryFull    TelemetryMode = "full"
	TelemetryMinimal TelemetryMode = "minimal"
)

func TelemetryModes() []TelemetryMode {
	return []TelemetryMode{TelemetryFull, TelemetryMinimal}
}

func (m TelemetryMode) Valid() bool {
	return slices.Contains(TelemetryModes(), m)
}

func (m TelemetryMode) Keeps(stream ExecutionStream) bool {
	return m != TelemetryMinimal || stream != ExecutionStreamTranscript
}

type ExecutionChunk struct {
	ID          uuid.UUID
	ExecutionID string
	WorkspaceID uuid.UUID
	Stream      ExecutionStream
	Sequence    int64
	Digest      string
	Bytes       int64
	Entries     int
	ObjectKey   string
	FirstAt     time.Time
	LastAt      time.Time
	ReceivedAt  time.Time
}

type ExecutionStreamCursor struct {
	Stream       ExecutionStream
	LastSequence int64
	Chunks       int
	Entries      int64
	Bytes        int64
}

type ExecutionLogEntry struct {
	At     time.Time `json:"at"`
	Stream string    `json:"stream,omitempty"`
	Source string    `json:"source,omitempty"`
	Text   string    `json:"text"`
}

type ExecutionTranscriptEntry struct {
	At      time.Time      `json:"at"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

type ExecutionLogChunk struct {
	ExecutionChunk
	Entries []ExecutionLogEntry
}

type ExecutionTranscriptChunk struct {
	ExecutionChunk
	Entries []ExecutionTranscriptEntry
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

type ExecutionChunkPage struct {
	Limit int
	After int64
}

func (p ExecutionChunkPage) Normalized() ExecutionChunkPage {
	if p.Limit <= 0 {
		p.Limit = ExecutionChunkPageDefault
	}

	if p.Limit > ExecutionChunkPageMax {
		p.Limit = ExecutionChunkPageMax
	}

	if p.After < 0 {
		p.After = 0
	}

	return p
}

type WorkspaceExecutionPolicy struct {
	WorkspaceID         uuid.UUID
	Telemetry           TelemetryMode
	UploadRetentionDays int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (p WorkspaceExecutionPolicy) Normalised(retention time.Duration) WorkspaceExecutionPolicy {
	if !p.Telemetry.Valid() {
		p.Telemetry = TelemetryFull
	}

	if p.UploadRetentionDays <= 0 {
		p.UploadRetentionDays = ExecutionRetentionDays(retention)
	}

	return p
}

func ExecutionRetentionDays(window time.Duration) int {
	days := int(window / (24 * time.Hour))
	if days < ExecutionRetentionDaysMin {
		return ExecutionRetentionDaysMin
	}

	return days
}

func ValidateUploadRetentionDays(field string, days int) FieldError {
	switch {
	case days < ExecutionRetentionDaysMin:
		return FieldError{Field: field, Code: ValidationCodeOutOfRange}
	case days > ExecutionRetentionDaysMax:
		return FieldError{Field: field, Code: ValidationCodeOutOfRange}
	default:
		return FieldError{}
	}
}

func ValidateTelemetryMode(field string, mode TelemetryMode) FieldError {
	if !mode.Valid() {
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	}

	return FieldError{}
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

func ExecutionChunkKey(
	workspaceID uuid.UUID,
	executionID string,
	stream ExecutionStream,
	digest string,
) string {
	return executionRunPrefix(workspaceID, executionID) + "/" + string(stream) + "/" + digest + ".json.gz"
}

func ExecutionArtifactKey(workspaceID uuid.UUID, executionID string, artifactID uuid.UUID) string {
	return executionRunPrefix(workspaceID, executionID) + "/artifacts/" + artifactID.String()
}
