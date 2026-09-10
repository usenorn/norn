package issue

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *issuesService) shapedBy(ctx context.Context, input *service.CreateIssueInput) error {
	template, err := s.templates.GetByID(ctx, input.WorkspaceID, input.TemplateID)
	if err != nil {
		return err
	}

	if template.TeamID != uuid.Nil && template.TeamID != input.TeamID {
		return entity.ErrIssueTemplateNotFound
	}

	if absent := template.Missing(entity.TemplateChoices{
		AssigneeAccountID: chosen(input.AssigneeAccountID, template.AssigneeAccountID),
		ProjectID:         chosen(input.ProjectID, template.ProjectID),
		CycleID:           input.CycleID,
		LabelIDs:          labelled(input.LabelIDs, template.LabelIDs),
		Priority:          priced(input.Priority, template.Priority),
		Estimate:          counted(input.Estimate, template.Estimate),
		DueOn:             input.DueOn,
	}); len(absent) > 0 {
		return entity.TemplateFieldsMissingError{Fields: absent}
	}

	if input.Title == "" {
		input.Title = template.Title
	}

	if input.Description == "" && input.DescriptionDoc == nil && template.BodyDoc.Type != "" {
		body := template.BodyDoc
		input.DescriptionDoc = &body
	}

	input.AssigneeAccountID = chosen(input.AssigneeAccountID, template.AssigneeAccountID)
	input.ProjectID = chosen(input.ProjectID, template.ProjectID)
	input.StateID = chosen(input.StateID, template.StateID)
	input.LabelIDs = labelled(input.LabelIDs, template.LabelIDs)
	input.Priority = priced(input.Priority, template.Priority)
	input.Estimate = counted(input.Estimate, template.Estimate)

	return nil
}

func chosen(sent, offered uuid.UUID) uuid.UUID {
	if sent != uuid.Nil {
		return sent
	}

	return offered
}

func labelled(sent, offered []uuid.UUID) []uuid.UUID {
	if len(sent) > 0 {
		return sent
	}

	return offered
}

func priced(sent, offered entity.IssuePriority) entity.IssuePriority {
	if sent != "" && sent != entity.IssuePriorityNone {
		return sent
	}

	return offered
}

func counted(sent, offered int) int {
	if sent > 0 {
		return sent
	}

	return offered
}
