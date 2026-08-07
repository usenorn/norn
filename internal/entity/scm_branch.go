package entity

import (
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
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

// BranchName fills a template with what a person needs to start work. The result is what
// they copy and hand to git, so it is trimmed to what git and both forges accept rather than
// left to fail at the point of use.
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

// SlugifyBranchPart keeps letters and digits and turns everything else into a single dash.
// git refuses a ref with a space, a colon, two consecutive dots or a trailing dot, and a
// title is written by a person who has no reason to know that.
func SlugifyBranchPart(value string) string {
	var builder strings.Builder

	dashed := false

	for _, symbol := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case unicode.IsLetter(symbol) || unicode.IsDigit(symbol):
			builder.WriteRune(symbol)

			dashed = false
		case !dashed && builder.Len() > 0:
			builder.WriteRune('-')

			dashed = true
		}
	}

	slug := strings.Trim(builder.String(), "-")

	if len(slug) > SCMBranchSlugMax {
		slug = strings.Trim(slug[:SCMBranchSlugMax], "-")
	}

	return slug
}

func TrimBranchName(name string) string {
	trimmed := strings.Trim(strings.TrimSpace(name), "-/")

	if len(trimmed) > SCMBranchNameMax {
		trimmed = strings.Trim(trimmed[:SCMBranchNameMax], "-/")
	}

	return trimmed
}

func ValidateSCMBranchTemplate(field, template string) FieldError {
	trimmed := strings.TrimSpace(template)

	switch {
	case trimmed == "":
		return FieldError{}
	case len(trimmed) > SCMBranchTemplateMax:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	case !strings.Contains(trimmed, "{reference}"):
		// Without the reference the branch names no issue, and naming the issue is the whole
		// reason the branch is generated here rather than typed.
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	default:
		return FieldError{}
	}
}

func IssueReferenceText(key string, number int) string {
	return key + "-" + strconv.Itoa(number)
}
