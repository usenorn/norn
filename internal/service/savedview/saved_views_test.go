package savedview_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	issuefilterreferencerepo "github.com/usenorn/norn/internal/repository/issuefilterreference"
	savedviewrepo "github.com/usenorn/norn/internal/repository/savedview"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	savedviewsvc "github.com/usenorn/norn/internal/service/savedview"
)

type harness struct {
	views      *savedviewrepo.MockSavedView
	references *issuefilterreferencerepo.MockIssueFilterReference
	teams      *teamrepo.MockTeam
	authorizer *authorizersvc.MockAuthorizer
	transactor *transactorrepo.MockTransactor
	service    service.SavedViews

	workspaceID uuid.UUID
	viewID      uuid.UUID
	authorID    uuid.UUID
	teamID      uuid.UUID
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		views:       savedviewrepo.NewMockSavedView(ctrl),
		references:  issuefilterreferencerepo.NewMockIssueFilterReference(ctrl),
		teams:       teamrepo.NewMockTeam(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		transactor:  transactorrepo.NewMockTransactor(ctrl),
		workspaceID: uuid.New(),
		viewID:      uuid.New(),
		authorID:    uuid.New(),
		teamID:      uuid.New(),
	}

	h.transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.service = savedviewsvc.New(h.views, h.references, h.teams, h.authorizer, h.transactor)

	return h
}

func (h *harness) actAs(role entity.MembershipRole, accountID uuid.UUID, scope entity.TeamScope) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: accountID},
			Role:  role,
			Scope: scope,
		}, nil).
		AnyTimes()
}

func (h *harness) onTeams(teamIDs ...uuid.UUID) {
	teams := make([]entity.Team, 0, len(teamIDs))
	for _, id := range teamIDs {
		teams = append(teams, entity.Team{ID: id, WorkspaceID: h.workspaceID})
	}

	h.teams.EXPECT().
		ListByWorkspaceMember(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(teams, nil).
		AnyTimes()
}

func (h *harness) holds(view entity.SavedView) {
	h.views.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(view, nil).AnyTimes()
	h.views.EXPECT().LockByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(view, nil).AnyTimes()
}

func (h *harness) view(sharing entity.SavedViewSharing) entity.SavedView {
	view := entity.SavedView{
		ID:          h.viewID,
		WorkspaceID: h.workspaceID,
		AuthorID:    h.authorID,
		Sharing:     sharing,
		Name:        "Urgent and unassigned",
	}

	if sharing == entity.SavedViewSharingTeam {
		view.TeamID = h.teamID
		view.TeamName = "Mobile"
	}

	return view
}

func TestTwoPeopleOpeningOneSharedViewSeeItThroughTheirOwnEyes(t *testing.T) {
	stateID := uuid.New()

	shared := entity.SavedView{
		ID:          uuid.New(),
		WorkspaceID: uuid.New(),
		AuthorID:    uuid.New(),
		Sharing:     entity.SavedViewSharingWorkspace,
		Name:        "Everything urgent",
		Filter: entity.IssueFilter{
			Field: entity.IssueFilterFieldState, Op: entity.IssueFilterOpIs, Values: []string{stateID.String()},
		},
	}

	open := func(t *testing.T, scope entity.TeamScope, state entity.IssueFilterReferenceState, name string) service.SavedViewDetail {
		t.Helper()

		h := newHarness(t)
		h.actAs(entity.MembershipRoleMember, uuid.New(), scope)
		h.onTeams()
		h.holds(shared)

		h.references.EXPECT().
			Resolve(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				_ context.Context, _ uuid.UUID, _ entity.TeamScope, wanted []entity.IssueFilterReference,
			) ([]entity.IssueFilterReference, error) {
				resolved := make([]entity.IssueFilterReference, 0, len(wanted))

				for _, reference := range wanted {
					reference.State = state
					reference.Name = name
					resolved = append(resolved, reference)
				}

				return resolved, nil
			})

		detail, err := h.service.Get(context.Background(), shared.WorkspaceID, shared.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}

		return detail
	}

	insider := open(t, entity.TeamScope{AllTeams: true}, entity.IssueFilterReferenceResolved, "In progress")
	outsider := open(t, entity.TeamScope{}, entity.IssueFilterReferenceRestricted, "")

	if !reflect.DeepEqual(insider.Summary.View.Filter, outsider.Summary.View.Filter) {
		t.Fatalf(
			"the two of them were handed different expressions:\n%+v\n%+v\nThe view is one object "+
				"for both; what differs is what each of them may see through it. If the filter "+
				"itself differs, the view has quietly become two views.",
			insider.Summary.View.Filter, outsider.Summary.View.Filter,
		)
	}

	if outsider.References[0].Name != "" {
		t.Fatalf(
			"the outsider was told the state is called %q. They cannot see that team, so the name "+
				"must never reach them; that it is referenced at all is the only thing they learn.",
			outsider.References[0].Name,
		)
	}

	if insider.References[0].Name == "" {
		t.Fatal("the insider was not told what the view filters on, so the chip cannot name anything")
	}
}

func TestARestrictedReferenceIsNotTheSameFactAsAMissingOne(t *testing.T) {
	if entity.IssueFilterReferenceRestricted == entity.IssueFilterReferenceMissing {
		t.Fatal(
			"restricted and missing collapsed into one state. Someone opening a shared view and " +
				"reading 'no longer exists' about a label that is merely private will delete that " +
				"clause to fix it, and silently widen a view the rest of the team relies on.",
		)
	}
}

func TestEditingASharedViewIsRefusedToEveryoneButItsAuthorAndAnAdministrator(t *testing.T) {
	name := "Renamed"

	for label, actor := range map[string]struct {
		role    entity.MembershipRole
		account uuid.UUID
		allowed bool
	}{
		"its author":       {entity.MembershipRoleMember, uuid.UUID{}, true},
		"an administrator": {entity.MembershipRoleAdmin, uuid.New(), true},
		"another member":   {entity.MembershipRoleMember, uuid.New(), false},
		"a viewer":         {entity.MembershipRoleViewer, uuid.New(), false},
	} {
		t.Run(label, func(t *testing.T) {
			h := newHarness(t)

			account := actor.account
			if account == (uuid.UUID{}) {
				account = h.authorID
			}

			h.actAs(actor.role, account, entity.TeamScope{AllTeams: true})
			h.onTeams()
			h.holds(h.view(entity.SavedViewSharingWorkspace))

			if actor.allowed {
				h.views.EXPECT().
					UpdateSettings(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(h.view(entity.SavedViewSharingWorkspace), nil)
			}

			_, err := h.service.Update(
				context.Background(), h.workspaceID, h.viewID,
				service.UpdateSavedViewInput{Name: &name},
			)

			if actor.allowed && err != nil {
				t.Fatalf("%s was refused: %v", label, err)
			}

			if !actor.allowed && !errors.Is(err, entity.ErrSavedViewNotOwner) {
				t.Fatalf("%s was allowed to edit a shared view (%v)", label, err)
			}
		})
	}
}

func TestTheEditableFlagAndTheRefusalCannotDisagree(t *testing.T) {
	name := "Renamed"

	for _, sharing := range entity.SavedViewSharings() {
		for _, role := range []entity.MembershipRole{
			entity.MembershipRoleAdmin, entity.MembershipRoleMember, entity.MembershipRoleViewer,
		} {
			for _, isAuthor := range []bool{true, false} {
				h := newHarness(t)

				account := uuid.New()
				if isAuthor {
					account = h.authorID
				}

				h.actAs(role, account, entity.TeamScope{AllTeams: true})
				h.onTeams(h.teamID)

				view := h.view(sharing)
				h.holds(view)
				h.views.EXPECT().
					ListFor(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]entity.SavedView{view}, nil).
					AnyTimes()
				h.views.EXPECT().
					UpdateSettings(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(view, nil).
					AnyTimes()

				listed, err := h.service.List(context.Background(), h.workspaceID)
				if err != nil || len(listed) != 1 {
					continue
				}

				_, updateErr := h.service.Update(
					context.Background(), h.workspaceID, h.viewID,
					service.UpdateSavedViewInput{Name: &name},
				)

				if listed[0].Editable != (updateErr == nil) {
					t.Errorf(
						"a %s view, seen by a %s who %s its author, is reported editable=%v but "+
							"updating it returned %v. A button the screen offers and the server "+
							"refuses is a lie told twice.",
						sharing, role, map[bool]string{true: "is", false: "is not"}[isAuthor],
						listed[0].Editable, updateErr,
					)
				}
			}
		}
	}
}

func TestAViewerKeepsPersonalViewsAndSharesNone(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.MembershipRoleViewer, h.authorID, entity.TeamScope{AllTeams: true})
	h.onTeams()

	h.views.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(h.view(entity.SavedViewSharingPersonal), nil)

	if _, err := h.service.Create(context.Background(), service.CreateSavedViewInput{
		WorkspaceID: h.workspaceID, Name: "Mine", Sharing: entity.SavedViewSharingPersonal,
	}); err != nil {
		t.Fatalf(
			"a viewer could not keep a personal view: %v\nA view of their own changes nothing "+
				"anyone else sees; refusing it makes the role unusable rather than safe.",
			err,
		)
	}

	if _, err := h.service.Create(context.Background(), service.CreateSavedViewInput{
		WorkspaceID: h.workspaceID, Name: "Ours", Sharing: entity.SavedViewSharingWorkspace,
	}); !errors.Is(err, entity.ErrSavedViewNotShareable) {
		t.Fatalf(
			"a viewer shared a view with the workspace (%v). Sharing publishes an object into "+
				"other people's navigation, which is a write to something they all hold.",
			err,
		)
	}
}

func TestDeletingASharedViewIsRefusedUntilItsSharingIsAcknowledged(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.MembershipRoleMember, h.authorID, entity.TeamScope{AllTeams: true})
	h.onTeams(h.teamID)
	h.holds(h.view(entity.SavedViewSharingTeam))

	err := h.service.Remove(context.Background(), h.workspaceID, h.viewID, "")

	var shared entity.SavedViewSharedError
	if !errors.As(err, &shared) {
		t.Fatalf("removing a team-shared view without acknowledgement returned %v", err)
	}

	if shared.TeamName == "" {
		t.Fatal(
			"the refusal does not name the team. The dialog has to say what it is about to take " +
				"away and from whom, or the acknowledgement is a formality.",
		)
	}

	h.views.EXPECT().Delete(gomock.Any(), h.viewID).Return(nil)

	if err := h.service.Remove(
		context.Background(), h.workspaceID, h.viewID, entity.SavedViewSharingTeam,
	); err != nil {
		t.Fatalf("removing an acknowledged shared view failed: %v", err)
	}
}

func TestDeletingAViewWhoseSharingChangedUnderTheCallerIsRefused(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.MembershipRoleMember, h.authorID, entity.TeamScope{AllTeams: true})
	h.onTeams()
	h.holds(h.view(entity.SavedViewSharingWorkspace))

	err := h.service.Remove(
		context.Background(), h.workspaceID, h.viewID, entity.SavedViewSharingPersonal,
	)

	var shared entity.SavedViewSharedError
	if !errors.As(err, &shared) || shared.Sharing != entity.SavedViewSharingWorkspace {
		t.Fatalf(
			"the caller acknowledged removing a personal view and the row had since been shared "+
				"with the workspace, but the delete returned %v. Acknowledging what you were shown "+
				"is what makes this safe when someone widens a view while the dialog is open.",
			err,
		)
	}
}

func TestEachAccountOrdersTheSameViewsForItself(t *testing.T) {
	first := entity.SavedView{ID: uuid.New(), Sharing: entity.SavedViewSharingWorkspace, Name: "A"}
	second := entity.SavedView{ID: uuid.New(), Sharing: entity.SavedViewSharingWorkspace, Name: "B"}

	place := func(t *testing.T, order []uuid.UUID) []uuid.UUID {
		t.Helper()

		h := newHarness(t)
		h.actAs(entity.MembershipRoleMember, uuid.New(), entity.TeamScope{AllTeams: true})
		h.onTeams()

		h.views.EXPECT().
			ListFor(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entity.SavedView{first, second}, nil).
			AnyTimes()

		var placed []uuid.UUID

		h.views.EXPECT().
			Place(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, _, _ uuid.UUID, ids []uuid.UUID) error {
				placed = ids

				return nil
			})

		if _, err := h.service.Reorder(context.Background(), h.workspaceID, order); err != nil {
			t.Fatalf("Reorder: %v", err)
		}

		return placed
	}

	ada := place(t, []uuid.UUID{second.ID, first.ID})
	grace := place(t, []uuid.UUID{first.ID, second.ID})

	if reflect.DeepEqual(ada, grace) {
		t.Fatal(
			"two people reordering the same shared views ended up with one order. An order that " +
				"one person can impose on another is not their order, it is a property of the view.",
		)
	}
}

func TestReorderingAcceptsOnlyViewsTheCallerCanSee(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.MembershipRoleMember, uuid.New(), entity.TeamScope{AllTeams: true})
	h.onTeams()

	mine := entity.SavedView{ID: uuid.New(), Sharing: entity.SavedViewSharingWorkspace}

	h.views.EXPECT().
		ListFor(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entity.SavedView{mine}, nil).
		AnyTimes()

	for name, order := range map[string][]uuid.UUID{
		"a view they cannot see": {uuid.New()},
		"the same view twice":    {mine.ID, mine.ID},
	} {
		t.Run(name, func(t *testing.T) {
			var validation entity.ValidationError

			if _, err := h.service.Reorder(context.Background(), h.workspaceID, order); !errors.As(err, &validation) {
				t.Fatalf("reordering with %s returned %v, want a validation error", name, err)
			}
		})
	}
}

func TestReorderingSurvivesSomeoneElseSharingAViewMidFlight(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.MembershipRoleMember, uuid.New(), entity.TeamScope{AllTeams: true})
	h.onTeams()

	known := entity.SavedView{ID: uuid.New(), Sharing: entity.SavedViewSharingWorkspace, Name: "Known"}
	arrived := entity.SavedView{ID: uuid.New(), Sharing: entity.SavedViewSharingWorkspace, Name: "Arrived"}

	h.views.EXPECT().
		ListFor(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entity.SavedView{known, arrived}, nil).
		AnyTimes()

	h.views.EXPECT().Place(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	if _, err := h.service.Reorder(context.Background(), h.workspaceID, []uuid.UUID{known.ID}); err != nil {
		t.Fatalf(
			"a reorder naming only the views the caller had on screen was refused: %v\n"+
				"Saved views are an open set that colleagues add to. Demanding a full permutation "+
				"would make one person sharing a view break everyone else's in-flight reorder.",
			err,
		)
	}
}

func TestAPersonalViewIsInvisibleToEveryoneElseIncludingAdministrators(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.MembershipRoleAdmin, uuid.New(), entity.TeamScope{AllTeams: true})
	h.onTeams()
	h.holds(h.view(entity.SavedViewSharingPersonal))

	if _, err := h.service.Get(context.Background(), h.workspaceID, h.viewID); !errors.Is(err, entity.ErrSavedViewNotFound) {
		t.Fatalf(
			"an administrator read somebody else's personal view (%v). Administration is a power "+
				"over the workspace's shared furniture, not a key to a colleague's own shortcuts.",
			err,
		)
	}
}

func TestSharingWithATeamTheCallerCannotSeeIsAFieldError(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.MembershipRoleMember, h.authorID, entity.TeamScope{TeamIDs: []uuid.UUID{}})
	h.onTeams()

	h.teams.EXPECT().
		GetByID(gomock.Any(), gomock.Any()).
		Return(entity.Team{ID: h.teamID, WorkspaceID: h.workspaceID}, nil)

	var validation entity.ValidationError

	_, err := h.service.Create(context.Background(), service.CreateSavedViewInput{
		WorkspaceID: h.workspaceID,
		Name:        "Theirs",
		Sharing:     entity.SavedViewSharingTeam,
		TeamID:      h.teamID,
	})

	if !errors.As(err, &validation) {
		t.Fatalf(
			"sharing with an invisible team returned %v, want a validation error on teamId. "+
				"A 404 would confirm the team exists; a field error says only that this is not a "+
				"team they may choose.",
			err,
		)
	}
}

var _ = repository.SavedViewSettings{}
