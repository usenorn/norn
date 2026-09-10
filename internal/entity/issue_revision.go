package entity

import (
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

const (
	IssueRevisionPageDefaultSize = 25
	IssueRevisionPageMaxSize     = 100

	IssueRevisionSameSitting = 5 * time.Minute
)

var ErrIssueRevisionNotFound = errors.New("description revision not found")

type RevisionSource string

const (
	RevisionSourcePerson  RevisionSource = "person"
	RevisionSourceAgent   RevisionSource = "agent"
	RevisionSourceImport  RevisionSource = "import"
	RevisionSourceIntake  RevisionSource = "intake"
	RevisionSourceRestore RevisionSource = "restore"
)

func RevisionSources() []RevisionSource {
	return []RevisionSource{
		RevisionSourcePerson,
		RevisionSourceAgent,
		RevisionSourceImport,
		RevisionSourceIntake,
		RevisionSourceRestore,
	}
}

func (s RevisionSource) Valid() bool {
	return slices.Contains(RevisionSources(), s)
}

func RevisionSourceOf(kind ActorKind, source TriageSource, origin *ImportOrigin) RevisionSource {
	switch {
	case OriginAttributed(origin):
		return RevisionSourceImport
	case source == TriageSourceEmail:
		return RevisionSourceIntake
	case kind == ActorKindAgent:
		return RevisionSourceAgent
	default:
		return RevisionSourcePerson
	}
}

func (r IssueDescriptionRevision) Continues(
	author uuid.UUID,
	source RevisionSource,
	now time.Time,
) bool {
	return r.AuthorAccountID == author &&
		r.Source == source &&
		now.Sub(r.CreatedAt) < IssueRevisionSameSitting
}

type IssueDescriptionRevision struct {
	ID              uuid.UUID
	WorkspaceID     uuid.UUID
	IssueID         uuid.UUID
	IssueVersion    int
	Doc             Document
	Markdown        string
	AuthorAccountID uuid.UUID
	AuthorName      string
	Source          RevisionSource
	CreatedAt       time.Time
}
