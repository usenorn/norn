package linear

import (
	"context"
	"strings"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const (
	stateStarted   = "started"
	stateCompleted = "completed"
	stateCanceled  = "canceled"
)

func (s *Source) fetchTeams(
	ctx context.Context,
	request service.ImportFetchRequest,
	settings Settings,
) (service.ImportFetchPage, error) {
	reply, err := query[teamsReply](ctx, s, request, settings, teamsQuery)
	if err != nil {
		return service.ImportFetchPage{}, err
	}

	records := make([]entity.ImportRecord, 0, len(reply.Teams.Nodes))

	for _, node := range reply.Teams.Nodes {
		record, err := recordOf(node.ID, "", node.CreatedAt, node.UpdatedAt, service.ImportTeamPayload{
			Key:  node.Key,
			Name: node.Name,
		})
		if err != nil {
			return service.ImportFetchPage{}, err
		}

		records = append(records, record)
	}

	return pageOf(records, reply.Teams.PageInfo), nil
}

func (s *Source) fetchWorkflowStates(
	ctx context.Context,
	request service.ImportFetchRequest,
	settings Settings,
) (service.ImportFetchPage, error) {
	reply, err := query[workflowStatesReply](ctx, s, request, settings, workflowStatesQuery)
	if err != nil {
		return service.ImportFetchPage{}, err
	}

	records := make([]entity.ImportRecord, 0, len(reply.WorkflowStates.Nodes))

	for _, node := range reply.WorkflowStates.Nodes {
		record, err := recordOf(
			node.ID, "", node.CreatedAt, node.UpdatedAt,
			service.ImportWorkflowStatePayload{
				Name:     node.Name,
				Category: category(node.Type),
				Team:     node.Team.key(),
			},
		)
		if err != nil {
			return service.ImportFetchPage{}, err
		}

		records = append(records, record)
	}

	return pageOf(records, reply.WorkflowStates.PageInfo), nil
}

func category(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case stateStarted:
		return string(entity.StateCategoryActive)
	case stateCompleted:
		return string(entity.StateCategoryComplete)
	case stateCanceled:
		return string(entity.StateCategoryAbandoned)
	default:
		return string(entity.StateCategoryNotStarted)
	}
}

func (s *Source) fetchLabelGroups(
	ctx context.Context,
	request service.ImportFetchRequest,
	settings Settings,
) (service.ImportFetchPage, error) {
	reply, err := query[labelsReply](ctx, s, request, settings, labelsQuery)
	if err != nil {
		return service.ImportFetchPage{}, err
	}

	records := make([]entity.ImportRecord, 0, len(reply.IssueLabels.Nodes))

	for _, node := range reply.IssueLabels.Nodes {
		if !node.IsGroup {
			continue
		}

		record, err := recordOf(node.ID, "", node.CreatedAt, node.UpdatedAt, service.ImportLabelGroupPayload{
			Name: node.Name,
		})
		if err != nil {
			return service.ImportFetchPage{}, err
		}

		records = append(records, record)
	}

	return pageOf(records, reply.IssueLabels.PageInfo), nil
}

func (s *Source) fetchLabels(
	ctx context.Context,
	request service.ImportFetchRequest,
	settings Settings,
) (service.ImportFetchPage, error) {
	reply, err := query[labelsReply](ctx, s, request, settings, labelsQuery)
	if err != nil {
		return service.ImportFetchPage{}, err
	}

	records := make([]entity.ImportRecord, 0, len(reply.IssueLabels.Nodes))

	for _, node := range reply.IssueLabels.Nodes {
		if node.IsGroup {
			continue
		}

		record, err := recordOf(node.ID, "", node.CreatedAt, node.UpdatedAt, service.ImportLabelPayload{
			Name:  node.Name,
			Color: node.Color,
			Group: node.Parent.key(),
			Team:  node.Team.key(),
		})
		if err != nil {
			return service.ImportFetchPage{}, err
		}

		records = append(records, record)
	}

	return pageOf(records, reply.IssueLabels.PageInfo), nil
}

func (s *Source) fetchProjects(
	ctx context.Context,
	request service.ImportFetchRequest,
	settings Settings,
) (service.ImportFetchPage, error) {
	reply, err := query[projectsReply](ctx, s, request, settings, projectsQuery)
	if err != nil {
		return service.ImportFetchPage{}, err
	}

	records := make([]entity.ImportRecord, 0, len(reply.Projects.Nodes))

	for _, node := range reply.Projects.Nodes {
		record, err := recordOf(node.ID, "", node.CreatedAt, node.UpdatedAt, service.ImportProjectPayload{
			Slug:        node.SlugID,
			Name:        node.Name,
			Description: node.Description,
			Lead:        node.Lead.imported(),
		})
		if err != nil {
			return service.ImportFetchPage{}, err
		}

		records = append(records, record)
	}

	return pageOf(records, reply.Projects.PageInfo), nil
}

func (s *Source) fetchCycles(
	ctx context.Context,
	request service.ImportFetchRequest,
	settings Settings,
) (service.ImportFetchPage, error) {
	reply, err := query[cyclesReply](ctx, s, request, settings, cyclesQuery)
	if err != nil {
		return service.ImportFetchPage{}, err
	}

	records := make([]entity.ImportRecord, 0, len(reply.Cycles.Nodes))

	for _, node := range reply.Cycles.Nodes {
		record, err := recordOf(node.ID, "", node.CreatedAt, node.UpdatedAt, service.ImportCyclePayload{
			Team:     node.Team.key(),
			Name:     node.Name,
			Number:   node.Number,
			StartsOn: calendarDay(node.StartsAt),
			EndsOn:   lastDayCovered(node.EndsAt),
			ClosedOn: calendarDay(node.CompletedAt),
		})
		if err != nil {
			return service.ImportFetchPage{}, err
		}

		records = append(records, record)
	}

	return pageOf(records, reply.Cycles.PageInfo), nil
}

func calendarDay(value string) string {
	moment := at(value)
	if moment == nil {
		return ""
	}

	return entity.FormatCalendarDate(*moment)
}

// lastDayCovered turns the instant a Linear cycle stops into the last day it covers. Linear
// ends a cycle where the next one begins, so its endsAt falls on the following day, while
// workspace_cycles excludes overlaps on daterange(starts_on, ends_on, '[]'): carried across
// unchanged, every consecutive pair would collide and the second of each would be refused.
func lastDayCovered(value string) string {
	moment := at(value)
	if moment == nil {
		return ""
	}

	return entity.FormatCalendarDate(moment.AddDate(0, 0, -1))
}

func (s *Source) fetchIssues(
	ctx context.Context,
	request service.ImportFetchRequest,
	settings Settings,
) (service.ImportFetchPage, error) {
	reply, err := query[issuesReply](ctx, s, request, settings, issuesQuery)
	if err != nil {
		return service.ImportFetchPage{}, err
	}

	records := make([]entity.ImportRecord, 0, len(reply.Issues.Nodes))

	for _, node := range reply.Issues.Nodes {
		warnTruncated(ctx, "labels", node.ID, node.Labels)

		labels := make([]string, 0, len(node.Labels.Nodes))

		for _, label := range node.Labels.Nodes {
			labels = append(labels, label.ID)
		}

		record, err := recordOf(
			node.ID, node.Parent.key(), node.CreatedAt, node.UpdatedAt,
			service.ImportIssuePayload{
				Title:       node.Title,
				Description: marked(node.Description, node.ID),
				Team:        node.Team.key(),
				State:       node.State.key(),
				Priority:    priority(node.Priority),
				Project:     node.Project.key(),
				Labels:      labels,
				Assignee:    node.Assignee.imported(),
				Author:      node.Creator.imported(),
				Estimate:    int(node.Estimate),
				DueOn:       node.DueDate,
				Cycle:       node.Cycle.key(),
			},
		)
		if err != nil {
			return service.ImportFetchPage{}, err
		}

		records = append(records, record)
	}

	return pageOf(records, reply.Issues.PageInfo), nil
}

// priority leaves zero unmapped. Linear numbers priority from one and reserves zero for an
// issue nobody prioritised, so a key here would put "no priority" in front of whoever maps
// the run as though it were a level the source had chosen.
func priority(level int) string {
	switch level {
	case 1:
		return string(entity.IssuePriorityUrgent)
	case 2:
		return string(entity.IssuePriorityHigh)
	case 3:
		return string(entity.IssuePriorityMedium)
	case 4:
		return string(entity.IssuePriorityLow)
	default:
		return ""
	}
}

func (s *Source) fetchComments(
	ctx context.Context,
	request service.ImportFetchRequest,
	settings Settings,
) (service.ImportFetchPage, error) {
	reply, err := query[commentsReply](ctx, s, request, settings, commentsQuery)
	if err != nil {
		return service.ImportFetchPage{}, err
	}

	records := make([]entity.ImportRecord, 0, len(reply.Comments.Nodes))

	for _, node := range reply.Comments.Nodes {
		record, err := recordOf(node.ID, "", node.CreatedAt, node.UpdatedAt, service.ImportCommentPayload{
			Issue:  node.Issue.key(),
			Body:   marked(node.Body, node.ID),
			Author: node.User.imported(),
			Parent: node.Parent.key(),
		})
		if err != nil {
			return service.ImportFetchPage{}, err
		}

		records = append(records, record)
	}

	return pageOf(records, reply.Comments.PageInfo), nil
}
