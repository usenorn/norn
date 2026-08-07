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

type CodeChecks string

const (
	CodeChecksUnknown CodeChecks = ""
	CodeChecksPending CodeChecks = "pending"
	CodeChecksPassing CodeChecks = "passing"
	CodeChecksFailing CodeChecks = "failing"
)

func CodeChecksStates() []CodeChecks {
	return []CodeChecks{CodeChecksPending, CodeChecksPassing, CodeChecksFailing}
}

func (c CodeChecks) Valid() bool {
	return c == CodeChecksUnknown || slices.Contains(CodeChecksStates(), c)
}

type ReviewVerdict string

const (
	ReviewRequested        ReviewVerdict = "requested"
	ReviewCommented        ReviewVerdict = "commented"
	ReviewApproved         ReviewVerdict = "approved"
	ReviewChangesRequested ReviewVerdict = "changes_requested"
	ReviewDismissed        ReviewVerdict = "dismissed"
)

func ReviewVerdicts() []ReviewVerdict {
	return []ReviewVerdict{
		ReviewRequested,
		ReviewCommented,
		ReviewApproved,
		ReviewChangesRequested,
		ReviewDismissed,
	}
}

func (v ReviewVerdict) Valid() bool {
	return slices.Contains(ReviewVerdicts(), v)
}

func (v ReviewVerdict) Blocking() bool {
	return v == ReviewChangesRequested
}

type CodeReviewer struct {
	LinkID     uuid.UUID
	Login      string
	Verdict    ReviewVerdict
	URL        string
	ReviewedAt *time.Time
	UpdatedAt  time.Time
}

type CodeReviewers []CodeReviewer

func (reviewers CodeReviewers) Outcome() (CodeChangeState, bool) {
	var approved, requested bool

	for _, reviewer := range reviewers {
		switch reviewer.Verdict {
		case ReviewChangesRequested:
			return CodeChangeChangesRequested, true
		case ReviewApproved:
			approved = true
		case ReviewRequested:
			requested = true
		}
	}

	switch {
	case approved:
		return CodeChangeApproved, true
	case requested:
		return CodeChangeReviewRequested, true
	default:
		return "", false
	}
}

func ResolveChangeState(base CodeChangeState, reviewers CodeReviewers) CodeChangeState {
	switch base {
	case CodeChangeMerged, CodeChangeClosed, CodeChangeDraft, CodeChangeConflicted:
		return base
	}

	if outcome, found := reviewers.Outcome(); found {
		return outcome
	}

	return base
}

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
	Checks          CodeChecks
	Author          string
	HeadBranch      string
	BaseBranch      string
	Paths           []string
	DetectedIn      string
	Resolving       bool
	MergeCommitSHA  string
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

type CodeLinkTransition struct {
	LinkID     uuid.UUID
	Transition CodeChangeState
	IssueID    uuid.UUID
	AppliedAt  time.Time
}

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

func (rules SCMTransitionRules) For(link CodeLink) (SCMTransitionRule, bool) {
	if link.Kind != CodeLinkChange || !link.Resolving {
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
