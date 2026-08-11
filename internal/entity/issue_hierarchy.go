package entity

import (
	"errors"
	"fmt"
	"strings"
)

const (
	IssueMaxDepth  = 5
	IssueRootDepth = 1
)

var (
	ErrIssueParentCycle     = errors.New("an issue cannot be filed under itself")
	ErrIssueParentNotActive = errors.New("an issue cannot be filed under an archived or deleted issue")
	ErrIssueParentTooDeep   = errors.New("the hierarchy would be nested deeper than allowed")
	ErrIssueChildrenOpen    = errors.New("issue has children that are still open")
)

type IssueTooDeepError struct {
	Depth int
	Max   int
}

func (e IssueTooDeepError) Error() string {
	return fmt.Sprintf("%s: %d beyond %d", ErrIssueParentTooDeep, e.Depth, e.Max)
}

func (e IssueTooDeepError) Unwrap() error {
	return ErrIssueParentTooDeep
}

type IssueChildrenOpenError struct {
	Children []Issue
}

func (e IssueChildrenOpenError) Error() string {
	references := make([]string, 0, len(e.Children))

	for _, child := range e.Children {
		references = append(references, child.Reference())
	}

	return fmt.Sprintf("%s: %s", ErrIssueChildrenOpen, strings.Join(references, ", "))
}

func (e IssueChildrenOpenError) Unwrap() error {
	return ErrIssueChildrenOpen
}

func FitsWithinDepth(parentDepth, subtreeHeight int) bool {
	return parentDepth+1+subtreeHeight <= IssueMaxDepth
}

func OpenChildren(progress IssueProgress) int {
	return progress.NotStarted + progress.Active
}

func OpenCategory(category StateCategory) bool {
	return category == StateCategoryNotStarted || category == StateCategoryActive
}

func OpenIssues(issues []Issue) []Issue {
	open := make([]Issue, 0, len(issues))

	for _, issue := range issues {
		if OpenCategory(issue.State.Category) {
			open = append(open, issue)
		}
	}

	return open
}
