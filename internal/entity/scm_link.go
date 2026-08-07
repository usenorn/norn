package entity

import (
	"slices"
	"time"

	"github.com/google/uuid"
)

type CodeLinkKind string

const (
	CodeLinkBranch CodeLinkKind = "branch"
	CodeLinkCommit CodeLinkKind = "commit"
	CodeLinkChange CodeLinkKind = "change"
)

func CodeLinkKinds() []CodeLinkKind {
	return []CodeLinkKind{CodeLinkBranch, CodeLinkCommit, CodeLinkChange}
}

func (k CodeLinkKind) Valid() bool {
	return slices.Contains(CodeLinkKinds(), k)
}

type CodeChangeState string

const (
	CodeChangeOpen     CodeChangeState = "open"
	CodeChangeDraft    CodeChangeState = "draft"
	CodeChangeInReview CodeChangeState = "in_review"
	CodeChangeMerged   CodeChangeState = "merged"
	CodeChangeClosed   CodeChangeState = "closed"
)

func CodeChangeStates() []CodeChangeState {
	return []CodeChangeState{
		CodeChangeOpen,
		CodeChangeDraft,
		CodeChangeInReview,
		CodeChangeMerged,
		CodeChangeClosed,
	}
}

func (s CodeChangeState) Valid() bool {
	return slices.Contains(CodeChangeStates(), s)
}

func (s CodeChangeState) Merged() bool {
	return s == CodeChangeMerged
}

func (s CodeChangeState) Settled() bool {
	return s == CodeChangeMerged || s == CodeChangeClosed
}

type CodeLink struct {
	ID              uuid.UUID
	WorkspaceID     uuid.UUID
	IssueID         uuid.UUID
	ConnectionID    uuid.UUID
	Provider        SCMProvider
	Repository      string
	Kind            CodeLinkKind
	ExternalID      string
	Number          int
	Title           string
	URL             string
	State           CodeChangeState
	Author          string
	DetectedIn      string
	AdvancedIssue   bool
	SourceUpdatedAt *time.Time
	MergedAt        *time.Time
	ClosedAt        *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (l CodeLink) Disconnected() bool {
	return l.ConnectionID == uuid.Nil
}

func (l CodeLink) Supersedes(observed *time.Time) bool {
	if observed == nil {
		return true
	}

	if l.SourceUpdatedAt == nil {
		return false
	}

	return !l.SourceUpdatedAt.Before(*observed)
}

type SCMTeamSettings struct {
	TeamID         uuid.UUID
	WorkspaceID    uuid.UUID
	AdvanceOnMerge bool
	MergedStateID  uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (s SCMTeamSettings) Advances(link CodeLink) bool {
	return s.AdvanceOnMerge && link.Kind == CodeLinkChange && link.State.Merged() && !link.AdvancedIssue
}

func (s SCMTeamSettings) TargetState(states []WorkflowState) (WorkflowState, bool) {
	if s.MergedStateID != uuid.Nil {
		for _, state := range states {
			if state.ID == s.MergedStateID {
				return state, true
			}
		}

		return WorkflowState{}, false
	}

	for _, state := range states {
		if state.IsCompletion {
			return state, true
		}
	}

	return WorkflowState{}, false
}
