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

	// A description written over several minutes is one rewriting, not thirty. Beyond this the
	// writer has stopped and come back, which is a version worth telling apart.
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

// RevisionSourceOf reads where a description came from. It is the actor rather than the person:
// a description an agent wrote and somebody approved is still the agent's writing, and the
// history is read to answer who wrote this, not who allowed it.
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

// IssueDescriptionRevision is what a description was at one point, kept so an edit can be
// compared, attributed and undone. An issue keeps its current text in its own row; this is the
// trail behind it.
// Continues reads whether a new version of a description belongs to the entry already open
// rather than starting another: the same writer, the same origin, still in the same sitting.
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
