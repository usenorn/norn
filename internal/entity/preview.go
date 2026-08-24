package entity

import (
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

const (
	PreviewNameMaxLen    = 30
	PreviewServiceMaxLen = 64
	PreviewPathMaxLen    = 512
	PreviewPortMin       = 1
	PreviewPortMax       = 65535
	PreviewsMax          = 8

	PreviewRetentionLongest = 24 * time.Hour
)

var (
	ErrPreviewNotFound      = errors.New("this run has no preview by that name")
	ErrPreviewClosed        = errors.New("this preview is no longer open")
	ErrPreviewStale         = errors.New("a newer report is already on record for this preview")
	ErrPreviewCrowded       = errors.New("this run already holds as many previews as it may")
	ErrPreviewNotRoutable   = errors.New("this server serves no preview domain")
	ErrPreviewNotExtendable = errors.New(
		"a run only keeps its preview longer while it is waiting for review",
	)
)

var previewName = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

type PreviewState string

const (
	PreviewOpen   PreviewState = channelv1.PreviewOpen
	PreviewClosed PreviewState = channelv1.PreviewClosed
)

func PreviewStates() []PreviewState {
	return []PreviewState{PreviewOpen, PreviewClosed}
}

func (s PreviewState) Valid() bool {
	return slices.Contains(PreviewStates(), s)
}

type PreviewMode string

const (
	PreviewBySubdomain PreviewMode = channelv1.PreviewBySubdomain
	PreviewByPath      PreviewMode = channelv1.PreviewByPath
)

func PreviewModes() []PreviewMode {
	return []PreviewMode{PreviewBySubdomain, PreviewByPath}
}

func (m PreviewMode) Valid() bool {
	return slices.Contains(PreviewModes(), m)
}

type PreviewSession struct {
	ID          uuid.UUID
	ExecutionID string
	WorkspaceID uuid.UUID
	Name        string
	Service     string
	Path        string
	Port        int
	Mode        PreviewMode
	Host        string
	State       PreviewState
	OpenedAt    time.Time
	ClosedAt    time.Time
	ReportedAt  time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (p PreviewSession) Open() bool {
	return p.State == PreviewOpen
}

func (p PreviewSession) URL(scheme string) string {
	if p.Host == "" {
		return ""
	}

	address := scheme + "://" + p.Host
	if p.Mode == PreviewByPath {
		address += "/" + strconv.Itoa(p.Port)
	}

	return address + p.Path
}

func PreviewHost(execution Execution, port int, mode PreviewMode, domain string) string {
	return channelv1.PreviewHost(
		execution.IssueReference, execution.IssueTitle,
		execution.ID, port, string(mode), domain,
	)
}

func ValidatePreviewName(field, name string) FieldError {
	trimmed := strings.TrimSpace(name)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case len(trimmed) > PreviewNameMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	case !previewName.MatchString(trimmed):
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	default:
		return FieldError{}
	}
}

func ValidatePreviewSession(field string, preview PreviewSession) error {
	mode := FieldError{}
	if !preview.Mode.Valid() {
		mode = FieldError{Field: field + ".mode", Code: ValidationCodeUnsupportedValue}
	}

	state := FieldError{}
	if !preview.State.Valid() {
		state = FieldError{Field: field + ".state", Code: ValidationCodeUnsupportedValue}
	}

	path := FieldError{}
	if preview.Path != "" && !strings.HasPrefix(preview.Path, "/") {
		path = FieldError{Field: field + ".path", Code: ValidationCodeMalformed}
	}

	port := FieldError{}
	if preview.Port < PreviewPortMin || preview.Port > PreviewPortMax {
		port = FieldError{Field: field + ".port", Code: ValidationCodeOutOfRange}
	}

	return NewValidationError(
		ValidatePreviewName(field+".name", preview.Name),
		requiredText(field+".service", preview.Service, PreviewServiceMaxLen),
		optionalText(field+".path", preview.Path, PreviewPathMaxLen),
		path,
		port,
		mode,
		state,
	)
}
