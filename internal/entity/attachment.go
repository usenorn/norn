package entity

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	AttachmentKeyPrefix     = "attachments"
	AttachmentSniffLength   = 512
	AttachmentNameMaxLen    = 255
	AttachmentStorageUnset  = 0
	AttachmentGenericType   = "application/octet-stream"
	AttachmentDispositionIn = "inline"
	AttachmentDispositionAt = "attachment"
)

var (
	ErrAttachmentNotFound   = errors.New("attachment not found")
	ErrAttachmentNotPending = errors.New("attachment has already been settled")
	ErrAttachmentMissing    = errors.New("no file was uploaded for this attachment")
	ErrAttachmentTooLarge   = errors.New("file exceeds the maximum size")
	ErrStorageExhausted     = errors.New("workspace storage is full")
)

type AttachmentTooLargeError struct {
	SizeBytes int64
	MaxBytes  int64
}

func (e AttachmentTooLargeError) Error() string {
	return fmt.Sprintf("%s: %d of %d", ErrAttachmentTooLarge, e.SizeBytes, e.MaxBytes)
}

func (e AttachmentTooLargeError) Unwrap() error {
	return ErrAttachmentTooLarge
}

type StorageExhaustedError struct {
	SizeBytes   int64
	StoredBytes int64
	MaxBytes    int64
}

func (e StorageExhaustedError) Error() string {
	return fmt.Sprintf("%s: %d stored of %d", ErrStorageExhausted, e.StoredBytes, e.MaxBytes)
}

func (e StorageExhaustedError) Unwrap() error {
	return ErrStorageExhausted
}

type AttachmentStatus string

const (
	AttachmentStatusPending   AttachmentStatus = "pending"
	AttachmentStatusStored    AttachmentStatus = "stored"
	AttachmentStatusDiscarded AttachmentStatus = "discarded"
)

func AttachmentStatuses() []AttachmentStatus {
	return []AttachmentStatus{
		AttachmentStatusPending,
		AttachmentStatusStored,
		AttachmentStatusDiscarded,
	}
}

func (s AttachmentStatus) Valid() bool {
	return slices.Contains(AttachmentStatuses(), s)
}

type Attachment struct {
	ID           uuid.UUID
	WorkspaceID  uuid.UUID
	IssueID      uuid.UUID
	CommentID    uuid.UUID
	UploaderID   uuid.UUID
	UploaderName string
	ObjectKey    string
	FileName     string
	ContentType  string
	SizeBytes    int64
	Status       AttachmentStatus
	ReclaimAfter *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (a Attachment) Stored() bool {
	return a.Status == AttachmentStatusStored
}

func (a Attachment) Orphaned() bool {
	return a.IssueID == uuid.Nil
}

func (a Attachment) Inline() bool {
	return AttachmentInline(a.ContentType)
}

func (a Attachment) Disposition() string {
	if a.Inline() {
		return AttachmentDispositionIn
	}

	return AttachmentDispositionAt
}

var attachmentInlineTypes = []string{"image/png", "image/jpeg", "image/gif", "image/webp"}

func AttachmentInline(contentType string) bool {
	return slices.Contains(attachmentInlineTypes, contentType)
}

func AttachmentServedType(sniffed string) string {
	kind, _, _ := strings.Cut(sniffed, ";")
	kind = strings.TrimSpace(kind)

	if AttachmentInline(kind) {
		return kind
	}

	return AttachmentGenericType
}

func AttachmentKey(workspaceID, attachmentID uuid.UUID) string {
	return AttachmentKeyPrefix + "/" + workspaceID.String() + "/" + attachmentID.String()
}

func AttachmentPrefix(workspaceID uuid.UUID) string {
	return AttachmentKeyPrefix + "/" + workspaceID.String()
}

func ValidateAttachmentName(field, name string) FieldError {
	trimmed := strings.TrimSpace(name)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case utf8.RuneCountInString(trimmed) > AttachmentNameMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	case strings.ContainsAny(trimmed, "\x00\r\n"):
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	}

	return FieldError{}
}

func AttachmentFileName(name string) string {
	cleaned := strings.TrimSpace(name)
	cleaned = strings.ReplaceAll(cleaned, "\\", "/")

	if slash := strings.LastIndex(cleaned, "/"); slash >= 0 {
		cleaned = cleaned[slash+1:]
	}

	return strings.TrimSpace(cleaned)
}

type WorkspaceStorage struct {
	WorkspaceID uuid.UUID
	StoredBytes int64
	MaxBytes    int64
	UpdatedAt   time.Time
}

func (s WorkspaceStorage) Unlimited() bool {
	return s.MaxBytes == AttachmentStorageUnset
}

func (s WorkspaceStorage) Remaining() int64 {
	if s.Unlimited() {
		return 0
	}

	if s.StoredBytes >= s.MaxBytes {
		return 0
	}

	return s.MaxBytes - s.StoredBytes
}

var blobKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

func ValidBlobKey(key string) bool {
	if key == "" || !blobKeyPattern.MatchString(key) {
		return false
	}

	for _, segment := range strings.Split(key, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}

	return true
}
