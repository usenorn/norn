package imports_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func mappingFor(
	plan entity.MappingPlan,
	kind entity.ImportMappingKind,
	key string,
	t *testing.T,
) entity.ImportMapping {
	t.Helper()

	mapping, found := plan.Lookup(kind, key)
	if !found {
		t.Fatalf("the plan has no %s concept for %q, so nobody can decide it", kind, key)
	}

	return mapping
}

type workspaceOf struct {
	team      entity.Team
	states    []entity.WorkflowState
	known     entity.WorkspaceMember
	alsoKnown entity.WorkspaceMember
}

func (w workspaceOf) stateNamed(t *testing.T, name string) entity.WorkflowState {
	t.Helper()

	for _, state := range w.states {
		if state.Name == name {
			return state
		}
	}

	t.Fatalf("this workspace has no state named %q to collide with", name)

	return entity.WorkflowState{}
}

func workspaceKnowing(t *testing.T, h *harness) workspaceOf {
	t.Helper()

	held := workspaceOf{
		team:      teamNamed("Platform"),
		known:     memberNamed("Rae Fenn", rae.Email),
		alsoKnown: memberNamed("Ida Vance", ida.Email),
	}

	held.states = entity.DefaultWorkflowStates(h.run().WorkspaceID, held.team.ID)

	for index := range held.states {
		held.states[index].ID = uuid.New()
	}

	h.scopedTo(held.team.ID)
	h.workspaceHolds(
		held.known, held.alsoKnown,
		memberNamed("Pat Ord", "pat@northwind.co"),
		memberNamed("Wes Lund", "wes@northwind.co"),
	)
	h.teamsVisible(held.team)
	h.statesOn(held.team.ID, held.states...)
	h.labelsHeld()

	return held
}

func TestOnlyThePeopleThisWorkspaceRecognisesAreSuggested(t *testing.T) {
	h := newHarness(t).backed()
	source := newStaticSource(t)

	h.stageEverything(source)

	held := workspaceKnowing(t, h)

	plan, err := h.imports.Mappings(context.Background(), h.run().WorkspaceID, h.run().ID)
	if err != nil {
		t.Fatalf("derive mappings: %v", err)
	}

	suggested := map[string]uuid.UUID{
		rae.Key: held.known.Membership.AccountID,
		ida.Key: held.alsoKnown.Membership.AccountID,
	}

	for _, person := range []service.ImportUser{rae, ida, otto, sam} {
		mapping := mappingFor(plan, entity.ImportMapUser, person.Key, t)

		if mapping.SuggestedTargetID != suggested[person.Key] {
			t.Errorf(
				"%s was matched to %v, want %v. A suggestion is offered on an address this "+
					"workspace already knows and on nothing else; guessing at %s would attribute "+
					"somebody else's work to whoever happened to be nearest.",
				person.Email, mapping.SuggestedTargetID, suggested[person.Key], person.Email,
			)
		}

		if mapping.SuggestedTargetID != uuid.Nil && mapping.SuggestedReason != entity.SuggestedByEmail {
			t.Errorf("%s was suggested for the reason %q, want the email match", person.Email, mapping.SuggestedReason)
		}

		if !mapping.Undecided() {
			t.Errorf(
				"%s reads as decided on the strength of a suggestion alone. A suggestion is what "+
					"the requester is shown; accepting it is a separate act, and an import that "+
					"treated the two as one would attribute work nobody confirmed.",
				person.Email,
			)
		}
	}

	if plan.Complete() {
		t.Fatal("a plan nobody has answered reports itself complete, so an import could run unmapped")
	}
}

func TestAStateThisWorkspaceAlreadyCarriesIsOfferedRatherThanRemade(t *testing.T) {
	h := newHarness(t).backed()

	h.stageEverything(newStaticSource(t))

	held := workspaceKnowing(t, h)

	plan, err := h.imports.Mappings(context.Background(), h.run().WorkspaceID, h.run().ID)
	if err != nil {
		t.Fatalf("derive mappings: %v", err)
	}

	done := mappingFor(plan, entity.ImportMapState, sourceStateDone, t)
	wanted := held.stateNamed(t, "Done").ID

	if done.SuggestedTargetID != wanted {
		t.Errorf(
			"the source's Done state was matched to %v, want the workspace's own %v. Every team "+
				"is seeded with these six, so an import that offered no match would leave a "+
				"second Done beside the first on every board.",
			done.SuggestedTargetID, wanted,
		)
	}

	if done.SuggestedReason == "" {
		t.Error("the match carries no reason, so the requester is asked to trust it blind")
	}
}

func TestAnImportWillNotRunWhileAnyConceptIsStillUnanswered(t *testing.T) {
	h := newHarness(t).backed().scopedTo()

	h.at(entity.ImportMapped)
	h.world.mappings = []entity.ImportMapping{
		{Kind: entity.ImportMapUser, SourceKey: rae.Key, SourceEmail: rae.Email},
	}

	_, err := h.imports.Execute(context.Background(), h.run().WorkspaceID, h.run().ID,
		service.ExecuteImportInput{PreviewDigest: "whatever"})

	if !errors.Is(err, entity.ErrImportUnmapped) {
		t.Fatalf(
			"execute returned %v with a concept still unanswered. Every unmapped concept is a row "+
				"the import would guess at, and the guess is only visible once it is already made.",
			err,
		)
	}
}

func TestWorkCanOnlyBeAttributedToSomebodyStillInThisWorkspace(t *testing.T) {
	left := uuid.New()
	stranger := uuid.New()

	cases := []struct {
		name    string
		target  uuid.UUID
		held    entity.Membership
		failure error
	}{
		{
			name:    "somebody who was never here",
			target:  stranger,
			failure: entity.ErrMembershipNotFound,
		},
		{
			name:   "somebody who has since been deactivated",
			target: left,
			held:   entity.Membership{AccountID: left, DeactivatedAt: pointer(time.Now().UTC())},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t).backed().scopedTo()

			h.members.EXPECT().
				Get(gomock.Any(), h.run().WorkspaceID, testCase.target).
				Return(testCase.held, testCase.failure)

			_, err := h.imports.Map(context.Background(), h.run().WorkspaceID, h.run().ID,
				service.MapImportInput{Decisions: []entity.ImportMapping{{
					Kind:      entity.ImportMapUser,
					SourceKey: rae.Key,
					Decision:  entity.ImportDecisionMap,
					TargetID:  testCase.target,
				}}})

			if !errors.Is(err, entity.ErrImportMappingNotMember) {
				t.Fatalf(
					"mapping returned %v. Every comment an import writes names its author, and the "+
						"column insists that author is a member of this workspace — so a mapping "+
						"onto somebody who is not has to be refused now, while there is still a "+
						"person to ask, rather than halfway through applying.",
					err,
				)
			}

			if len(h.world.mappings) != 0 {
				t.Errorf("the refused decision was saved anyway: %v", h.world.mappings)
			}
		})
	}
}

func TestATeamTheRequesterCannotReachIsNotAValidTarget(t *testing.T) {
	h := newHarness(t).backed()

	mine := uuid.New()
	theirs := uuid.New()

	h.scopedTo(mine)

	_, err := h.imports.Map(context.Background(), h.run().WorkspaceID, h.run().ID,
		service.MapImportInput{Decisions: []entity.ImportMapping{{
			Kind:      entity.ImportMapTeam,
			SourceKey: sourceTeam,
			Decision:  entity.ImportDecisionMap,
			TargetID:  theirs,
		}}})

	if !errors.Is(err, entity.ErrTeamNotFound) {
		t.Fatalf(
			"mapping a source team onto a team outside the requester's scope returned %v. A "+
				"private team the requester cannot see must not become visible by being named "+
				"as an import target.",
			err,
		)
	}
}

func TestSomebodyWhoCannotMakeTeamsMustSayWhereTheWorkGoes(t *testing.T) {
	cases := []struct {
		name     string
		decision entity.ImportDecision
	}{
		{"leaving the team unanswered", ""},
		{"asking for a new team", entity.ImportDecisionCreate},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t).backed()

			h.stage(entity.ImportTeam,
				fetched(t, sourceTeam, "", service.ImportTeamPayload{Key: "CORE", Name: "Core"}))
			h.allow(entity.Decision{Role: entity.MembershipRoleMember})

			workspaceKnowing(t, h)

			decisions := []entity.ImportMapping{}

			if testCase.decision != "" {
				decisions = append(decisions, entity.ImportMapping{
					Kind:      entity.ImportMapTeam,
					SourceKey: sourceTeam,
					Decision:  testCase.decision,
				})
			}

			_, err := h.imports.Map(context.Background(), h.run().WorkspaceID, h.run().ID,
				service.MapImportInput{Decisions: decisions})

			if !errors.Is(err, entity.ErrImportTeamMappingRequired) {
				t.Fatalf(
					"mapping returned %v. Creating a team is an administrator's act; a member who "+
						"could have an import make one has found a way around the check that "+
						"stops them making one directly.",
					err,
				)
			}
		})
	}
}

func TestADecisionSurvivesTheNextTimeThePlanIsDerived(t *testing.T) {
	h := newHarness(t).backed()

	h.stageEverything(newStaticSource(t))

	held := workspaceKnowing(t, h)

	chosen := uuid.New()

	h.decided(entity.ImportMapping{
		Kind:      entity.ImportMapUser,
		SourceKey: rae.Key,
		Decision:  entity.ImportDecisionMap,
		TargetID:  chosen,
	})

	plan, err := h.imports.Mappings(context.Background(), h.run().WorkspaceID, h.run().ID)
	if err != nil {
		t.Fatalf("derive mappings: %v", err)
	}

	mapping := mappingFor(plan, entity.ImportMapUser, rae.Key, t)

	if mapping.TargetID != chosen {
		t.Fatalf(
			"re-deriving the plan moved %s onto %v, having been told %v. The plan is derived "+
				"afresh every time the requester opens it, and an answer that a later derivation "+
				"can overwrite is an answer that quietly reverts while somebody is still deciding "+
				"the rest.",
			rae.Email, mapping.TargetID, chosen,
		)
	}

	if mapping.Decision != entity.ImportDecisionMap {
		t.Errorf("the decision came back as %q, want it left alone", mapping.Decision)
	}

	if mapping.SuggestedTargetID != held.known.Membership.AccountID {
		t.Errorf(
			"the plan stopped offering the address match once a decision was taken; the suggestion " +
				"is what tells the requester the decision they took differs from the obvious one",
		)
	}
}

func pointer[T any](value T) *T { return &value }
