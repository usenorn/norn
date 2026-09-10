package entity

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	IssueTemplateNameMaxLen = 128
	IssueTemplateMaxPerTeam = 50
)

var (
	ErrIssueTemplateNotFound = errors.New("template not found")
	ErrIssueTemplateNameUsed = errors.New("a template with this name already exists here")
	ErrTooManyIssueTemplates = errors.New("too many templates on one team")
)

// TemplateField names a property a template insists on before an issue may be raised from it.
// The set is closed: a template can only require something the creation screen actually offers.
type TemplateField string

const (
	TemplateFieldAssignee TemplateField = "assignee"
	TemplateFieldProject  TemplateField = "project"
	TemplateFieldCycle    TemplateField = "cycle"
	TemplateFieldEstimate TemplateField = "estimate"
	TemplateFieldDueOn    TemplateField = "dueOn"
	TemplateFieldLabels   TemplateField = "labels"
	TemplateFieldPriority TemplateField = "priority"
)

func TemplateFields() []TemplateField {
	return []TemplateField{
		TemplateFieldAssignee,
		TemplateFieldProject,
		TemplateFieldCycle,
		TemplateFieldEstimate,
		TemplateFieldDueOn,
		TemplateFieldLabels,
		TemplateFieldPriority,
	}
}

func (f TemplateField) Valid() bool {
	return slices.Contains(TemplateFields(), f)
}

// IssueTemplate is a shape a team keeps for the issues it raises again and again — the prose it
// starts from and the properties it comes with. A template with no team belongs to the whole
// workspace.
type IssueTemplate struct {
	ID                 uuid.UUID
	WorkspaceID        uuid.UUID
	TeamID             uuid.UUID
	Name               string
	Description        string
	Title              string
	Body               string
	BodyDoc            Document
	RequiredFields     []TemplateField
	StateID            uuid.UUID
	ProjectID          uuid.UUID
	AssigneeAccountID  uuid.UUID
	LabelIDs           []uuid.UUID
	Priority           IssuePriority
	Estimate           int
	Position           int
	CreatedByAccountID uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Missing reads which of the properties a template insists on the caller has not chosen, so the
// refusal names them all at once rather than one per attempt.
func (t IssueTemplate) Missing(chosen TemplateChoices) []TemplateField {
	var absent []TemplateField

	for _, field := range t.RequiredFields {
		switch field {
		case TemplateFieldAssignee:
			if chosen.AssigneeAccountID == uuid.Nil {
				absent = append(absent, field)
			}
		case TemplateFieldProject:
			if chosen.ProjectID == uuid.Nil {
				absent = append(absent, field)
			}
		case TemplateFieldCycle:
			if chosen.CycleID == uuid.Nil {
				absent = append(absent, field)
			}
		case TemplateFieldEstimate:
			if chosen.Estimate <= 0 {
				absent = append(absent, field)
			}
		case TemplateFieldDueOn:
			if chosen.DueOn == "" {
				absent = append(absent, field)
			}
		case TemplateFieldLabels:
			if len(chosen.LabelIDs) == 0 {
				absent = append(absent, field)
			}
		case TemplateFieldPriority:
			if chosen.Priority == "" || chosen.Priority == IssuePriorityNone {
				absent = append(absent, field)
			}
		}
	}

	return absent
}

type TemplateChoices struct {
	AssigneeAccountID uuid.UUID
	ProjectID         uuid.UUID
	CycleID           uuid.UUID
	LabelIDs          []uuid.UUID
	Priority          IssuePriority
	Estimate          int
	DueOn             string
}

func ValidateTemplateName(field, name string) FieldError {
	length := utf8.RuneCountInString(strings.TrimSpace(name))

	switch {
	case length == 0:
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case length > IssueTemplateNameMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateTemplateFields(field string, required []TemplateField) FieldError {
	for _, named := range required {
		if !named.Valid() {
			return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
		}
	}

	return FieldError{}
}

type TemplateFieldsMissingError struct {
	Fields []TemplateField
}

func (e TemplateFieldsMissingError) Error() string {
	named := make([]string, 0, len(e.Fields))
	for _, field := range e.Fields {
		named = append(named, string(field))
	}

	return "this template requires " + strings.Join(named, ", ")
}
