package entity

import (
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"

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
	return channelv1.Slug(latinise(value), SCMBranchSlugMax)
}

// A branch name reaches remote refs, CI job names, image tags and URLs, and those treat
// anything outside ASCII as a problem to solve rather than a name. Latin letters lose their
// marks, Cyrillic is transliterated, and an alphabet with no rule here leaves nothing — which
// is correct: the template then yields the reference alone rather than an unusable name.
func latinise(value string) string {
	var builder strings.Builder

	for _, symbol := range norm.NFKD.String(value) {
		switch {
		case symbol < utf8.RuneSelf:
			builder.WriteRune(symbol)
		case unicode.Is(unicode.Mn, symbol):
		default:
			builder.WriteString(cyrillic[unicode.ToLower(symbol)])
		}
	}

	return builder.String()
}

var cyrillic = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e", 'ж': "zh",
	'з': "z", 'и': "i", 'й': "i", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o",
	'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "h", 'ц': "ts",
	'ч': "ch", 'ш': "sh", 'щ': "sch", 'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu",
	'я': "ya", 'і': "i", 'ї': "yi", 'є': "ye", 'ґ': "g",
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
