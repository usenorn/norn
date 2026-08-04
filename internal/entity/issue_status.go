package entity

import (
	"errors"
	"slices"
	"time"
)

type IssueStatus string

const (
	IssueStatusActive          IssueStatus = "active"
	IssueStatusArchived        IssueStatus = "archived"
	IssueStatusPendingDeletion IssueStatus = "pending_deletion"
)

const IssuePurgeGrace = 30 * 24 * time.Hour

var (
	ErrIssueStatusTransition = errors.New("issue cannot move between those states")
	ErrIssuePurgeNotDue      = errors.New("issue is not due to be purged")
)

func IssueStatuses() []IssueStatus {
	return []IssueStatus{IssueStatusActive, IssueStatusArchived, IssueStatusPendingDeletion}
}

func (s IssueStatus) Valid() bool {
	return slices.Contains(IssueStatuses(), s)
}

func (s IssueStatus) CanTransitionTo(target IssueStatus) bool {
	switch s {
	case IssueStatusActive:
		return target == IssueStatusArchived || target == IssueStatusPendingDeletion
	case IssueStatusArchived:
		return target == IssueStatusActive || target == IssueStatusPendingDeletion
	case IssueStatusPendingDeletion:
		return target == IssueStatusActive || target == IssueStatusArchived
	default:
		return false
	}
}

func RestoredIssueStatus(archivedAt *time.Time) IssueStatus {
	if archivedAt != nil {
		return IssueStatusArchived
	}

	return IssueStatusActive
}

type IssueLifecycle struct {
	Status              IssueStatus
	ArchivedAt          *time.Time
	DeletionRequestedAt *time.Time
	PurgeAfter          *time.Time
}

func ApplyIssueStatus(target IssueStatus, archivedAt *time.Time, now time.Time) IssueLifecycle {
	switch target {
	case IssueStatusArchived:
		archived := now
		if archivedAt != nil {
			archived = *archivedAt
		}

		return IssueLifecycle{Status: target, ArchivedAt: &archived}

	case IssueStatusPendingDeletion:
		purge := now.Add(IssuePurgeGrace)

		return IssueLifecycle{
			Status:              target,
			ArchivedAt:          archivedAt,
			DeletionRequestedAt: &now,
			PurgeAfter:          &purge,
		}

	default:
		return IssueLifecycle{Status: IssueStatusActive}
	}
}

func (l IssueLifecycle) PurgeDue(now time.Time) bool {
	return l.Status == IssueStatusPendingDeletion && l.PurgeAfter != nil && !now.Before(*l.PurgeAfter)
}

func RequestedIssueStatuses(requested []IssueStatus) []IssueStatus {
	if len(requested) == 0 {
		return []IssueStatus{IssueStatusActive}
	}

	chosen := make([]IssueStatus, 0, len(requested))

	for _, status := range requested {
		if status.Valid() && !slices.Contains(chosen, status) {
			chosen = append(chosen, status)
		}
	}

	return chosen
}
