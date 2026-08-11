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
	IssueFieldRank        = "rank"
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
		IssueFieldRank,
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
	Rank        *string

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

	if c.Rank != nil {
		fields = append(fields, IssueFieldRank)
	}

	return fields
}

func (c IssueChange) Effective(issue Issue) []string {
	fields := make([]string, 0, len(IssueFields()))

	if c.Title != nil && *c.Title != issue.Title {
		fields = append(fields, IssueFieldTitle)
	}

	if c.StateID != nil && *c.StateID != issue.State.ID {
		fields = append(fields, IssueFieldState)
	}

	if c.Description != nil && *c.Description != issue.Description {
		fields = append(fields, IssueFieldDescription)
	}

	if c.Priority != nil && *c.Priority != issue.Priority {
		fields = append(fields, IssueFieldPriority)
	}

	if altered(c.Assignee, c.ClearAssignee, issue.AssigneeAccountID, uuid.Nil) {
		fields = append(fields, IssueFieldAssignee)
	}

	if altered(c.Estimate, c.ClearEstimate, issue.Estimate, 0) {
		fields = append(fields, IssueFieldEstimate)
	}

	if altered(c.DueOn, c.ClearDueOn, issue.DueOn, "") {
		fields = append(fields, IssueFieldDueOn)
	}

	if altered(c.CycleID, c.ClearCycle, issue.CycleID, uuid.Nil) {
		fields = append(fields, IssueFieldCycle)
	}

	if altered(c.ProjectID, c.ClearProject, issue.ProjectID, uuid.Nil) {
		fields = append(fields, IssueFieldProject)
	}

	if c.Rank != nil && *c.Rank != issue.Rank {
		fields = append(fields, IssueFieldRank)
	}

	return fields
}

func altered[T comparable](value *T, cleared bool, current, empty T) bool {
	if cleared {
		return current != empty
	}

	return value != nil && *value != current
}

func (c IssueChange) AgentActions(issue Issue) []AgentAction {
	changed := c.Effective(issue)

	actions := make([]AgentAction, 0, 2)

	if slices.Contains(changed, IssueFieldState) {
		actions = append(actions, AgentActionStateChange)
	}

	if slices.ContainsFunc(changed, func(field string) bool { return field != IssueFieldState }) {
		actions = append(actions, AgentActionIssueEdit)
	}

	return actions
}

func (c IssueChange) FieldVersions(version int) map[string]int {
	stamped := map[string]int{}

	for _, field := range c.Touched() {
		stamped[field] = version
	}

	return stamped
}
