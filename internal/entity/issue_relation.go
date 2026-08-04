package entity

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
)

type IssueRelationKind string

const (
	IssueRelationBlocks     IssueRelationKind = "blocks"
	IssueRelationDuplicates IssueRelationKind = "duplicates"
	IssueRelationRelatesTo  IssueRelationKind = "relates_to"
)

type IssueRelationView string

const (
	IssueRelationViewBlocks       IssueRelationView = "blocks"
	IssueRelationViewBlockedBy    IssueRelationView = "blocked_by"
	IssueRelationViewDuplicates   IssueRelationView = "duplicates"
	IssueRelationViewDuplicatedBy IssueRelationView = "duplicated_by"
	IssueRelationViewRelatesTo    IssueRelationView = "relates_to"
)

var (
	ErrIssueRelationSelf     = errors.New("an issue cannot be related to itself")
	ErrIssueRelationExists   = errors.New("these two issues already hold a relation")
	ErrIssueRelationNotFound = errors.New("issue relation not found")
)

func IssueRelationKinds() []IssueRelationKind {
	return []IssueRelationKind{IssueRelationBlocks, IssueRelationDuplicates, IssueRelationRelatesTo}
}

func (k IssueRelationKind) Valid() bool {
	return slices.Contains(IssueRelationKinds(), k)
}

func (k IssueRelationKind) Symmetric() bool {
	return k == IssueRelationRelatesTo
}

func IssueRelationViews() []IssueRelationView {
	return []IssueRelationView{
		IssueRelationViewBlocks,
		IssueRelationViewBlockedBy,
		IssueRelationViewDuplicates,
		IssueRelationViewDuplicatedBy,
		IssueRelationViewRelatesTo,
	}
}

func (v IssueRelationView) Valid() bool {
	return slices.Contains(IssueRelationViews(), v)
}

func (k IssueRelationKind) As(subjectIsSource bool) IssueRelationView {
	switch k {
	case IssueRelationBlocks:
		if subjectIsSource {
			return IssueRelationViewBlocks
		}

		return IssueRelationViewBlockedBy

	case IssueRelationDuplicates:
		if subjectIsSource {
			return IssueRelationViewDuplicates
		}

		return IssueRelationViewDuplicatedBy

	default:
		return IssueRelationViewRelatesTo
	}
}

func (v IssueRelationView) Stored() (IssueRelationKind, bool) {
	switch v {
	case IssueRelationViewBlocks:
		return IssueRelationBlocks, true
	case IssueRelationViewBlockedBy:
		return IssueRelationBlocks, false
	case IssueRelationViewDuplicates:
		return IssueRelationDuplicates, true
	case IssueRelationViewDuplicatedBy:
		return IssueRelationDuplicates, false
	default:
		return IssueRelationRelatesTo, true
	}
}

func NormalisePair(a, b uuid.UUID) (uuid.UUID, uuid.UUID) {
	if bytes.Compare(a[:], b[:]) > 0 {
		return b, a
	}

	return a, b
}

type StoredIssueRelation struct {
	ID                 uuid.UUID
	WorkspaceID        uuid.UUID
	SourceIssueID      uuid.UUID
	TargetIssueID      uuid.UUID
	Kind               IssueRelationKind
	CreatedByAccountID uuid.UUID
	CreatedAt          time.Time
}

func (r StoredIssueRelation) Counterpart(issueID uuid.UUID) (uuid.UUID, bool) {
	if r.SourceIssueID == issueID {
		return r.TargetIssueID, true
	}

	return r.SourceIssueID, false
}

type IssueRelation struct {
	ID        uuid.UUID
	Kind      IssueRelationView
	Issue     Issue
	CreatedAt time.Time
}

type IssueRelationExistsError struct {
	Kind      IssueRelationView
	Reference string
}

func (e IssueRelationExistsError) Error() string {
	return fmt.Sprintf("%s: already %s %s", ErrIssueRelationExists, e.Kind, e.Reference)
}

func (e IssueRelationExistsError) Unwrap() error {
	return ErrIssueRelationExists
}

type IssueRelationGroup struct {
	Kind      IssueRelationView
	Relations []IssueRelation
}

func GroupRelations(relations []IssueRelation) []IssueRelationGroup {
	groups := make([]IssueRelationGroup, 0, len(IssueRelationViews()))

	for _, view := range IssueRelationViews() {
		members := make([]IssueRelation, 0, len(relations))

		for _, relation := range relations {
			if relation.Kind == view {
				members = append(members, relation)
			}
		}

		if len(members) > 0 {
			groups = append(groups, IssueRelationGroup{Kind: view, Relations: members})
		}
	}

	return groups
}
