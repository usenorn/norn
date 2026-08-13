package entity

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	IssueDescriptionMaxLen = 20000
	IssueEstimateMax       = 1000
)

type IssuePriority string

const (
	IssuePriorityUrgent IssuePriority = "urgent"
	IssuePriorityHigh   IssuePriority = "high"
	IssuePriorityMedium IssuePriority = "medium"
	IssuePriorityLow    IssuePriority = "low"
	IssuePriorityNone   IssuePriority = "none"
)

func IssuePriorities() []IssuePriority {
	return []IssuePriority{
		IssuePriorityUrgent,
		IssuePriorityHigh,
		IssuePriorityMedium,
		IssuePriorityLow,
		IssuePriorityNone,
	}
}

func (p IssuePriority) Valid() bool {
	switch p {
	case IssuePriorityUrgent, IssuePriorityHigh, IssuePriorityMedium, IssuePriorityLow, IssuePriorityNone:
		return true
	default:
		return false
	}
}

func (p IssuePriority) Order() int {
	priorities := IssuePriorities()

	if index := slices.Index(priorities, p); index >= 0 {
		return index
	}

	return len(priorities)
}

func CompareIssueQueueOrder(a, b Issue) int {
	if order := a.Priority.Order() - b.Priority.Order(); order != 0 {
		return order
	}

	if rank := strings.Compare(a.Rank, b.Rank); rank != 0 {
		return rank
	}

	return a.CreatedAt.Compare(b.CreatedAt)
}

type StateTimestamps struct {
	StateEnteredAt time.Time
	CompletedAt    *time.Time
}

func ApplyStateTransition(from, to StateCategory, enteredAt time.Time, now time.Time) StateTimestamps {
	if from == to {
		return StateTimestamps{StateEnteredAt: enteredAt, CompletedAt: completedAt(from, enteredAt)}
	}

	return StateTimestamps{StateEnteredAt: now, CompletedAt: completedAt(to, now)}
}

func completedAt(category StateCategory, at time.Time) *time.Time {
	if category != StateCategoryComplete {
		return nil
	}

	moment := at

	return &moment
}

func ValidateIssueDescription(field, description string) FieldError {
	if strings.ContainsRune(description, 0) {
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	}

	if utf8.RuneCountInString(description) > IssueDescriptionMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}

func ValidateIssuePriority(field string, priority IssuePriority) FieldError {
	if !priority.Valid() {
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	}

	return FieldError{}
}

func ValidateIssueEstimate(field string, estimate int) FieldError {
	if estimate <= 0 || estimate > IssueEstimateMax {
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	}

	return FieldError{}
}
