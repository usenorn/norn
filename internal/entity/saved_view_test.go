package entity_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestAStoredViewNamesNothingItCanOutlive(t *testing.T) {
	labelID := uuid.New()
	stateID := uuid.New()

	view := entity.SavedView{
		Name: "Urgent design work",
		Filter: entity.IssueFilter{All: []entity.IssueFilter{
			{Field: entity.IssueFilterFieldLabel, Op: entity.IssueFilterOpHasAny, Values: []string{labelID.String()}},
			{Field: entity.IssueFilterFieldState, Op: entity.IssueFilterOpIs, Values: []string{stateID.String()}},
		}},
	}

	stored, err := json.Marshal(view.Filter)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, id := range []uuid.UUID{labelID, stateID} {
		if !strings.Contains(string(stored), id.String()) {
			t.Fatalf("the stored expression lost the id %s:\n%s", id, stored)
		}
	}

	for _, name := range []string{"Needs spec", "In progress", "Urgent design work"} {
		if strings.Contains(string(stored), name) {
			t.Fatalf(
				"the stored expression carries the name %q:\n%s\nRenaming that label tomorrow "+
					"would make this view mean something it did not mean today. An id is the only "+
					"thing that survives a rename.",
				name, stored,
			)
		}
	}
}

func TestAReferenceIsMissingUntilSomethingProvesOtherwise(t *testing.T) {
	gone := uuid.New()

	filter := entity.IssueFilter{
		Field: entity.IssueFilterFieldLabel, Op: entity.IssueFilterOpHasAny, Values: []string{gone.String()},
	}

	if err := filter.Validate(); err != nil {
		t.Fatalf(
			"a filter naming an id that no longer exists was refused: %v\n"+
				"A dangling id is still a well-formed id. The view has to degrade, never error.",
			err,
		)
	}

	references := entity.IssueFilterReferences(filter)
	if len(references) != 1 {
		t.Fatalf("collected %d references, want 1: %+v", len(references), references)
	}

	if references[0].State != entity.IssueFilterReferenceMissing || references[0].Name != "" {
		t.Fatalf(
			"a freshly collected reference reads %+v, want missing with no name. Missing is the "+
				"starting point so that a resolver which fails, times out, or is never wired "+
				"degrades the view instead of inventing a name for it.",
			references[0],
		)
	}
}

func TestReferencesAreCollectedOnlyForFieldsThatNameSomething(t *testing.T) {
	id := uuid.New().String()

	naming := []entity.IssueFilterField{
		entity.IssueFilterFieldTeam,
		entity.IssueFilterFieldState,
		entity.IssueFilterFieldAssignee,
		entity.IssueFilterFieldCreator,
		entity.IssueFilterFieldProject,
		entity.IssueFilterFieldCycle,
		entity.IssueFilterFieldLabel,
	}

	for _, field := range naming {
		op := entity.IssueFilterOpIs
		if field == entity.IssueFilterFieldLabel {
			op = entity.IssueFilterOpHasAny
		}

		references := entity.IssueFilterReferences(entity.IssueFilter{
			Field: field, Op: op, Values: []string{id},
		})

		if len(references) != 1 {
			t.Errorf("%q names something but collected %d references", field, len(references))
		}
	}

	for _, filter := range []entity.IssueFilter{
		{Field: entity.IssueFilterFieldPriority, Op: entity.IssueFilterOpIs, Values: []string{"urgent"}},
		{Field: entity.IssueFilterFieldStateCategory, Op: entity.IssueFilterOpIs, Values: []string{"active"}},
		{Field: entity.IssueFilterFieldStatus, Op: entity.IssueFilterOpIs, Values: []string{"active"}},
		{Field: entity.IssueFilterFieldDueOn, Op: entity.IssueFilterOpBefore, Values: []string{"2026-09-01"}},
		{Field: entity.IssueFilterFieldEstimate, Op: entity.IssueFilterOpEq, Values: []string{"3"}},
		{Field: entity.IssueFilterFieldBlocked, Op: entity.IssueFilterOpIsTrue},
	} {
		if references := entity.IssueFilterReferences(filter); len(references) != 0 {
			t.Errorf(
				"%q carries its own meaning but produced %+v. Only a reference to another record "+
					"can go missing; an enum cannot.",
				filter.Field, references,
			)
		}
	}
}

func TestTheSameReferenceIsOnlyReportedOnce(t *testing.T) {
	id := uuid.New().String()

	references := entity.IssueFilterReferences(entity.IssueFilter{Any: []entity.IssueFilter{
		{Field: entity.IssueFilterFieldLabel, Op: entity.IssueFilterOpHasAny, Values: []string{id, id}},
		{Not: &entity.IssueFilter{
			Field: entity.IssueFilterFieldLabel, Op: entity.IssueFilterOpHasNone, Values: []string{id},
		}},
	}})

	if len(references) != 1 {
		t.Fatalf("the same label mentioned three times produced %d chips: %+v", len(references), references)
	}
}

func TestASharedViewMustNameExactlyTheTeamItIsSharedWith(t *testing.T) {
	teamID := uuid.New()

	for name, view := range map[string]entity.SavedView{
		"shared with a team but naming none": {
			Name: "Ours", Sharing: entity.SavedViewSharingTeam,
		},
		"personal but still naming a team": {
			Name: "Mine", Sharing: entity.SavedViewSharingPersonal, TeamID: teamID,
		},
		"workspace wide but still naming a team": {
			Name: "Everyone", Sharing: entity.SavedViewSharingWorkspace, TeamID: teamID,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := view.Validate(); err == nil {
				t.Fatal(
					"the view was accepted. A view that says who it is shared with, and separately " +
						"points at a team, can be read two ways by two queries.",
				)
			}
		})
	}

	for name, view := range map[string]entity.SavedView{
		"personal":  {Name: "Mine", Sharing: entity.SavedViewSharingPersonal},
		"workspace": {Name: "Everyone", Sharing: entity.SavedViewSharingWorkspace},
		"team":      {Name: "Ours", Sharing: entity.SavedViewSharingTeam, TeamID: teamID},
	} {
		t.Run(name, func(t *testing.T) {
			if err := view.Validate(); err != nil {
				t.Fatalf("a well-formed %s view was refused: %v", name, err)
			}
		})
	}
}

func TestAnAdministratorMayEditAnySharedViewAndReadNobodysPersonalOne(t *testing.T) {
	author := uuid.New()
	admin := uuid.New()
	teamID := uuid.New()

	personal := entity.SavedView{AuthorID: author, Sharing: entity.SavedViewSharingPersonal}
	shared := entity.SavedView{AuthorID: author, Sharing: entity.SavedViewSharingWorkspace}

	if personal.VisibleTo(admin, nil) {
		t.Error(
			"an administrator can see someone else's personal view. Administration is a power over " +
				"the workspace's shared furniture, not a key to a colleague's own shortcuts.",
		)
	}

	if personal.EditableBy(admin, entity.MembershipRoleAdmin) {
		t.Error("an administrator can edit someone else's personal view")
	}

	if !shared.EditableBy(admin, entity.MembershipRoleAdmin) {
		t.Error(
			"an administrator cannot edit a shared view. Someone has to be able to fix or retire a " +
				"view that outlived the person who made it.",
		)
	}

	if shared.EditableBy(uuid.New(), entity.MembershipRoleMember) {
		t.Error("a member who did not write a shared view can edit it")
	}

	if !shared.EditableBy(author, entity.MembershipRoleViewer) {
		t.Error("the author cannot edit their own view once it is shared")
	}

	team := entity.SavedView{AuthorID: author, Sharing: entity.SavedViewSharingTeam, TeamID: teamID}

	if team.VisibleTo(uuid.New(), nil) {
		t.Error("a view shared with a team is visible to someone on none of its teams")
	}

	if !team.VisibleTo(uuid.New(), []uuid.UUID{teamID}) {
		t.Error("a view shared with a team is invisible to someone on that team")
	}
}

func TestAnUnauthoredViewBelongsToNobody(t *testing.T) {
	orphan := entity.SavedView{Sharing: entity.SavedViewSharingWorkspace}

	if orphan.AuthoredBy(uuid.Nil) {
		t.Fatal(
			"a view with no author reads as authored by the nil account, so anyone holding a nil " +
				"account id would inherit it",
		)
	}

	if !orphan.EditableBy(uuid.New(), entity.MembershipRoleAdmin) {
		t.Error("nobody can tidy up a shared view whose author has left")
	}
}
