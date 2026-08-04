package entity

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
)

const (
	IssueFieldTitle       = "title"
	IssueFieldState       = "state"
	IssueFieldTeam        = "team"
	IssueFieldLabels      = "labels"
	IssueFieldDescription = "description"
	IssueFieldPriority    = "priority"
	IssueFieldAssignee    = "assignee"
	IssueFieldEstimate    = "estimate"
	IssueFieldDueOn       = "dueOn"
	IssueFieldCycle       = "cycle"
	IssueFieldProject     = "project"
	IssueFieldStatus      = "status"
	IssueFieldParent      = "parent"
	IssueFieldChildren    = "children"
	IssueFieldRelations   = "relations"
)

var ErrIssueStale = errors.New("issue changed since it was read")

type IssueStaleError struct {
	Version   int
	Conflicts []string
}

func (e IssueStaleError) Error() string {
	return fmt.Sprintf("%s: %s at version %d", ErrIssueStale, strings.Join(e.Conflicts, ", "), e.Version)
}

func (e IssueStaleError) Unwrap() error {
	return ErrIssueStale
}

func IssueFields() []string {
	return []string{
		IssueFieldTitle,
		IssueFieldState,
		IssueFieldTeam,
		IssueFieldLabels,
		IssueFieldDescription,
		IssueFieldPriority,
		IssueFieldAssignee,
		IssueFieldEstimate,
		IssueFieldDueOn,
		IssueFieldCycle,
		IssueFieldProject,
		IssueFieldStatus,
		IssueFieldParent,
		IssueFieldChildren,
		IssueFieldRelations,
	}
}

var issueFieldCoupling = [][]string{
	{IssueFieldTeam, IssueFieldState, IssueFieldLabels},
}

func coupled(field string) []string {
	for _, group := range issueFieldCoupling {
		if slices.Contains(group, field) {
			return group
		}
	}

	return []string{field}
}

func IssueConflicts(fieldVersions map[string]int, version, observed int, touched []string) []string {
	conflicts := make([]string, 0, len(touched))

	for _, field := range touched {
		for _, member := range coupled(field) {
			if fieldVersions[member] > observed && !slices.Contains(conflicts, member) {
				conflicts = append(conflicts, member)
			}
		}
	}

	if len(conflicts) > 0 {
		slices.Sort(conflicts)

		return conflicts
	}

	if version <= observed {
		return nil
	}

	if attributed(fieldVersions, observed) {
		return nil
	}

	return slices.Clone(touched)
}

func attributed(fieldVersions map[string]int, observed int) bool {
	for _, at := range fieldVersions {
		if at > observed {
			return true
		}
	}

	return false
}

type IssueChange struct {
	Title       *string
	StateID     *uuid.UUID
	Description *string
	Priority    *IssuePriority
	Assignee    *uuid.UUID
	Estimate    *int
	DueOn       *string
	CycleID     *uuid.UUID
	ProjectID   *uuid.UUID

	ClearAssignee bool
	ClearEstimate bool
	ClearDueOn    bool
	ClearCycle    bool
	ClearProject  bool
}

func (c IssueChange) Touched() []string {
	fields := make([]string, 0, len(IssueFields()))

	if c.Title != nil {
		fields = append(fields, IssueFieldTitle)
	}

	if c.StateID != nil {
		fields = append(fields, IssueFieldState)
	}

	if c.Description != nil {
		fields = append(fields, IssueFieldDescription)
	}

	if c.Priority != nil {
		fields = append(fields, IssueFieldPriority)
	}

	if c.Assignee != nil || c.ClearAssignee {
		fields = append(fields, IssueFieldAssignee)
	}

	if c.Estimate != nil || c.ClearEstimate {
		fields = append(fields, IssueFieldEstimate)
	}

	if c.DueOn != nil || c.ClearDueOn {
		fields = append(fields, IssueFieldDueOn)
	}

	if c.CycleID != nil || c.ClearCycle {
		fields = append(fields, IssueFieldCycle)
	}

	if c.ProjectID != nil || c.ClearProject {
		fields = append(fields, IssueFieldProject)
	}

	return fields
}

func (c IssueChange) FieldVersions(version int) map[string]int {
	stamped := map[string]int{}

	for _, field := range c.Touched() {
		stamped[field] = version
	}

	return stamped
}
