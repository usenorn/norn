package issuetemplate_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestATemplateNamesEveryPropertyItInsistsOnAtOnce(t *testing.T) {
	template := entity.IssueTemplate{
		RequiredFields: []entity.TemplateField{
			entity.TemplateFieldAssignee,
			entity.TemplateFieldEstimate,
			entity.TemplateFieldDueOn,
		},
	}

	absent := template.Missing(entity.TemplateChoices{AssigneeAccountID: uuid.New()})

	if len(absent) != 2 {
		t.Fatalf(
			"the template named %v as missing. A refusal that names one thing at a time makes "+
				"somebody guess their way through the form.",
			absent,
		)
	}

	if absent[0] != entity.TemplateFieldEstimate || absent[1] != entity.TemplateFieldDueOn {
		t.Fatalf("the template named %v, want the estimate and the due date", absent)
	}
}

func TestAPriorityOfNoneDoesNotSatisfyATemplateThatInsistsOnOne(t *testing.T) {
	template := entity.IssueTemplate{
		RequiredFields: []entity.TemplateField{entity.TemplateFieldPriority},
	}

	if absent := template.Missing(entity.TemplateChoices{
		Priority: entity.IssuePriorityNone,
	}); len(absent) != 1 {
		t.Fatalf(
			"a priority of none satisfied a template that insists on one. None is the absence of " +
				"a priority, which is what the template is asking somebody to decide.",
		)
	}

	if absent := template.Missing(entity.TemplateChoices{
		Priority: entity.IssuePriorityLow,
	}); len(absent) != 0 {
		t.Fatalf("a chosen priority was still reported missing: %v", absent)
	}
}

func TestATemplateThatInsistsOnNothingRefusesNothing(t *testing.T) {
	if absent := (entity.IssueTemplate{}).Missing(entity.TemplateChoices{}); len(absent) != 0 {
		t.Fatalf("a template with no requirements reported %v missing", absent)
	}
}
