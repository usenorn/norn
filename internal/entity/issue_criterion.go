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
	CriterionIDMaxLen       = 64
	EvidenceLabelMaxLen     = 200
	EvidenceURLMaxLen       = 2048
	EvidencePerIssueMax     = 200
	EvidencePerCriterionMax = 20
)

var (
	ErrEvidenceNotFound  = errors.New("evidence not found")
	ErrCriterionNotFound = errors.New("this issue has no acceptance criterion with that identifier")
	ErrTooMuchEvidence   = errors.New("too much evidence on one acceptance criterion")
)

type EvidenceKind string

const (
	EvidenceTest        EvidenceKind = "test"
	EvidenceScreenshot  EvidenceKind = "screenshot"
	EvidencePullRequest EvidenceKind = "pull_request"
	EvidencePerson      EvidenceKind = "person"
)

func EvidenceKinds() []EvidenceKind {
	return []EvidenceKind{EvidenceTest, EvidenceScreenshot, EvidencePullRequest, EvidencePerson}
}

func (k EvidenceKind) Valid() bool {
	return slices.Contains(EvidenceKinds(), k)
}

func (k EvidenceKind) NeedsAddress() bool {
	return k != EvidencePerson
}

type CriterionEvidence struct {
	ID                    uuid.UUID
	WorkspaceID           uuid.UUID
	IssueID               uuid.UUID
	CriterionID           string
	CriterionText         string
	Kind                  EvidenceKind
	Label                 string
	URL                   string
	AttachmentID          uuid.UUID
	DescriptionRevisionID uuid.UUID
	RecordedByAccountID   uuid.UUID
	RecordedByName        string
	RecordedAt            time.Time
}

func (e CriterionEvidence) Stale(criterion string) bool {
	return e.CriterionText != "" && e.CriterionText != criterion
}

type AcceptanceCriterion struct {
	ID       string
	Text     string
	Checked  bool
	Evidence []CriterionEvidence
}

func (c AcceptanceCriterion) Proven() bool {
	for _, held := range c.Evidence {
		if !held.Stale(c.Text) {
			return true
		}
	}

	return false
}

func AcceptanceCriteria(document Document) []AcceptanceCriterion {
	var found []AcceptanceCriterion

	eachNode(document.Content, func(node Node) {
		if node.Type != NodeTaskItem {
			return
		}

		id := attrString(node, "id")
		if id == "" {
			return
		}

		found = append(found, AcceptanceCriterion{
			ID:      id,
			Text:    strings.TrimSpace(DocumentText(NewDocument(node.Content...))),
			Checked: attrBool(node, "checked"),
		})
	})

	return found
}

func WithEvidence(
	criteria []AcceptanceCriterion,
	evidence []CriterionEvidence,
) []AcceptanceCriterion {
	held := map[string][]CriterionEvidence{}

	for _, entry := range evidence {
		held[entry.CriterionID] = append(held[entry.CriterionID], entry)
	}

	for at := range criteria {
		criteria[at].Evidence = held[criteria[at].ID]
	}

	return criteria
}

func ValidateEvidence(kind EvidenceKind, label, url string) []FieldError {
	var failures []FieldError

	if !kind.Valid() {
		failures = append(failures, FieldError{Field: "kind", Code: ValidationCodeUnsupportedValue})
	}

	if strings.TrimSpace(label) == "" {
		failures = append(failures, FieldError{Field: "label", Code: ValidationCodeRequired})
	}

	if utf8.RuneCountInString(label) > EvidenceLabelMaxLen {
		failures = append(failures, FieldError{Field: "label", Code: ValidationCodeTooLong})
	}

	if utf8.RuneCountInString(url) > EvidenceURLMaxLen {
		failures = append(failures, FieldError{Field: "url", Code: ValidationCodeTooLong})
	}

	if kind.NeedsAddress() && strings.TrimSpace(url) == "" {
		failures = append(failures, FieldError{Field: "url", Code: ValidationCodeRequired})
	}

	return failures
}
