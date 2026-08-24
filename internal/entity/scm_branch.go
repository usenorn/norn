package entity

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

const (
	SCMBranchTemplateDefault = "{handle}/{reference}-{title}"
	SCMBranchTemplateMax     = 200
	SCMBranchSlugMax         = 60
	SCMBranchNameMax         = 240
)

type SCMTeamSettings struct {
	TeamID         uuid.UUID
	WorkspaceID    uuid.UUID
	BranchTemplate string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (s SCMTeamSettings) Template() string {
	if strings.TrimSpace(s.BranchTemplate) == "" {
		return SCMBranchTemplateDefault
	}

	return s.BranchTemplate
}

func (s SCMTeamSettings) BranchName(handle, reference, title string) string {
	filled := strings.NewReplacer(
		"{handle}", SlugifyBranchPart(handle),
		"{reference}", strings.ToLower(strings.TrimSpace(reference)),
		"{title}", SlugifyBranchPart(title),
	).Replace(s.Template())

	return TrimBranchName(filled)
}

func BranchNameFor(settings SCMTeamSettings, handle string, issue Issue, reference string) string {
	if handle == "" {
		handle = "norn"
	}

	return settings.BranchName(handle, reference, issue.Title)
}

func SlugifyBranchPart(value string) string {
	return channelv1.Slug(value, SCMBranchSlugMax)
}

func TrimBranchName(name string) string {
	trimmed := strings.Trim(strings.TrimSpace(name), "-/")

	return strings.Trim(cutAtRuneBoundary(trimmed, SCMBranchNameMax), "-/")
}

func cutAtRuneBoundary(value string, limit int) string {
	if len(value) <= limit {
		return value
	}

	for limit > 0 && !utf8.RuneStart(value[limit]) {
		limit--
	}

	return value[:limit]
}

func ValidateSCMBranchTemplate(field, template string) FieldError {
	trimmed := strings.TrimSpace(template)

	switch {
	case trimmed == "":
		return FieldError{}
	case len(trimmed) > SCMBranchTemplateMax:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	case !strings.Contains(trimmed, "{reference}"):
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	default:
		return FieldError{}
	}
}

func IssueReferenceText(key string, number int) string {
	return key + "-" + strconv.Itoa(number)
}
