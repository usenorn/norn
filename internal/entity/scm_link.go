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

// CodeChecks is what the platform's own checks say about a change. Norn reflects it and
// never runs anything: a red build is something to see on the issue, not something to fix
// from here.
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

// ReviewVerdict is one person's answer on a change. Keeping it per reviewer rather than
// reducing it to a single state is what lets the issue say who is blocking and who is not.
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

// Outcome reduces a set of reviews to the state a change is in. Changes requested outranks
// an approval however recent the approval is, because a change nobody addressed is not ready
// whatever else was said about it.
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

// ResolveChangeState combines what the change itself says with what its reviewers said. The
// order is mechanical first: a merged or closed change is finished whatever anybody thought
// of it, a draft is not up for review yet, and a change that will not merge is blocked on
// something no reviewer can fix. Only then does the review outcome speak.
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
// one. Only a change carries a lifecycle; a branch or a commit has no state to enter. A
// change that merely mentions an issue never drives it either — naming a ticket for context
// is not a claim to settle it, and only `fixes`, `closes` or `resolves` makes that claim.
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
