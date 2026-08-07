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

// CodeChangeState is where a change stands on the forge. Review and conflict are states a
// change is in, not events that happened to it, which is what lets a rule fire on entering
// one and lets the issue read correctly after a reload.
type CodeChangeState string

const (
	CodeChangeDraft            CodeChangeState = "draft"
	CodeChangeOpen             CodeChangeState = "open"
	CodeChangeReviewRequested  CodeChangeState = "review_requested"
	CodeChangeChangesRequested CodeChangeState = "changes_requested"
	CodeChangeApproved         CodeChangeState = "approved"
	CodeChangeMerged           CodeChangeState = "merged"
	CodeChangeClosed           CodeChangeState = "closed"
	CodeChangeReopened         CodeChangeState = "reopened"
	CodeChangeConflicted       CodeChangeState = "conflicted"
)

func CodeChangeStates() []CodeChangeState {
	return []CodeChangeState{
		CodeChangeDraft,
		CodeChangeOpen,
		CodeChangeReviewRequested,
		CodeChangeChangesRequested,
		CodeChangeApproved,
		CodeChangeMerged,
		CodeChangeClosed,
		CodeChangeReopened,
		CodeChangeConflicted,
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

func (s CodeChangeState) InReview() bool {
	return s == CodeChangeReviewRequested ||
		s == CodeChangeChangesRequested ||
		s == CodeChangeApproved
}

type CodeChangeAction string

const (
	CodeChangeActionNone CodeChangeAction = ""
	CodeChangeActionCI   CodeChangeAction = "checks_failed"
)

type CodeLink struct {
	ID              uuid.UUID
	WorkspaceID     uuid.UUID
	IssueID         uuid.UUID
	RepositoryID    uuid.UUID
	Provider        SCMProvider
	RepositoryName  string
	Kind            CodeLinkKind
	ExternalID      string
	Number          int
	Title           string
	URL             string
	State           CodeChangeState
	Action          CodeChangeAction
	Author          string
	HeadBranch      string
	BaseBranch      string
	Paths           []string
	DetectedIn      string
	Resolving       bool
	SourceUpdatedAt *time.Time
	MergedAt        *time.Time
	ClosedAt        *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (l CodeLink) Disconnected() bool {
	return l.RepositoryID == uuid.Nil
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

// CodeLinkTransition records that a link has already driven the issue for one state. The
// single flag it replaces could only say that something had happened once, so a second rule
// on the same link had no way to know whether its own turn had come.
type CodeLinkTransition struct {
	LinkID     uuid.UUID
	Transition CodeChangeState
	IssueID    uuid.UUID
	AppliedAt  time.Time
}

// SCMTransitionRule moves a team's issue to a workflow state when its change reaches one on
// the forge.
type SCMTransitionRule struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	TeamID      uuid.UUID
	Trigger     CodeChangeState
	StateID     uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type SCMTransitionRules []SCMTransitionRule

// For reports the rule that a link entering the given state should drive, if the team has
// one. Only a change carries a lifecycle; a branch or a commit has no state to enter.
func (rules SCMTransitionRules) For(link CodeLink) (SCMTransitionRule, bool) {
	if link.Kind != CodeLinkChange {
		return SCMTransitionRule{}, false
	}

	for _, rule := range rules {
		if rule.Trigger == link.State {
			return rule, true
		}
	}

	return SCMTransitionRule{}, false
}

func (r SCMTransitionRule) TargetState(states []WorkflowState) (WorkflowState, bool) {
	for _, state := range states {
		if state.ID == r.StateID {
			return state, true
		}
	}

	return WorkflowState{}, false
}
