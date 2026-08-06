package project_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	activityrepo "github.com/usenorn/norn/internal/repository/activity"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	notificationeventrepo "github.com/usenorn/norn/internal/repository/notificationevent"
	projectrepo "github.com/usenorn/norn/internal/repository/project"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	projectsvc "github.com/usenorn/norn/internal/service/project"
)

type harness struct {
	projects    *projectrepo.MockProject
	members     *projectrepo.MockProjectMember
	statuses    *projectrepo.MockProjectStatusUpdate
	activity    *activityrepo.MockActivity
	accounts    *accountrepo.MockAccount
	memberships *membershiprepo.MockMembership
	notify      *notificationeventrepo.MockNotificationEvent
	authorizer  *authorizersvc.MockAuthorizer
	transactor  *transactorrepo.MockTransactor
	service     service.Projects

	workspaceID uuid.UUID
	projectID   uuid.UUID
	leadID      uuid.UUID
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		projects:    projectrepo.NewMockProject(ctrl),
		members:     projectrepo.NewMockProjectMember(ctrl),
		statuses:    projectrepo.NewMockProjectStatusUpdate(ctrl),
		activity:    activityrepo.NewMockActivity(ctrl),
		accounts:    accountrepo.NewMockAccount(ctrl),
		memberships: membershiprepo.NewMockMembership(ctrl),
		notify:      notificationeventrepo.NewMockNotificationEvent(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		transactor:  transactorrepo.NewMockTransactor(ctrl),
		workspaceID: uuid.New(),
		projectID:   uuid.New(),
		leadID:      uuid.New(),
	}

	h.transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	h.service = projectsvc.New(
		h.projects, h.members, h.statuses, h.activity, h.accounts, h.memberships, h.notify, h.authorizer, h.transactor,
	)

	return h
}

func (h *harness) actAs(role entity.MembershipRole, accountID uuid.UUID) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor:     entity.Actor{Kind: entity.ActorKindUser, AccountID: accountID},
			Role:      role,
			Workspace: entity.Workspace{ID: h.workspaceID},
			Scope:     entity.TeamScope{WorkspaceID: h.workspaceID, AllTeams: role == entity.MembershipRoleAdmin},
		}, nil).
		AnyTimes()
}

func (h *harness) project(state entity.ProjectState) entity.Project {
	return entity.Project{
		ID:            h.projectID,
		WorkspaceID:   h.workspaceID,
		Slug:          "checkout-rebuild",
		Name:          "Checkout rebuild",
		State:         state,
		LeadAccountID: h.leadID,
	}
}

func TestOnlyAFinishedProjectCanBeArchived(t *testing.T) {
	for _, state := range []entity.ProjectState{
		entity.ProjectStatePlanned,
		entity.ProjectStateActive,
		entity.ProjectStatePaused,
	} {
		t.Run(string(state), func(t *testing.T) {
			h := newHarness(t)
			h.actAs(entity.MembershipRoleAdmin, uuid.New())

			h.projects.EXPECT().
				GetByID(gomock.Any(), h.workspaceID, h.projectID).
				Return(h.project(state), nil)

			h.projects.EXPECT().Archive(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			_, err := h.service.Archive(context.Background(), h.workspaceID, h.projectID)

			if !errors.Is(err, entity.ErrProjectNotFinished) {
				t.Fatalf(
					"archiving a %s project returned %v, want ErrProjectNotFinished; archiving is "+
						"for work that has reached an end, not a way to hide work in flight",
					state, err,
				)
			}
		})
	}

	for _, state := range []entity.ProjectState{
		entity.ProjectStateCompleted,
		entity.ProjectStateCancelled,
	} {
		t.Run(string(state), func(t *testing.T) {
			h := newHarness(t)
			h.actAs(entity.MembershipRoleAdmin, uuid.New())

			project := h.project(state)
			archivedAt := time.Now().UTC()
			project.ArchivedAt = &archivedAt

			h.projects.EXPECT().
				GetByID(gomock.Any(), h.workspaceID, h.projectID).
				Return(h.project(state), nil)
			h.projects.EXPECT().
				Archive(gomock.Any(), h.projectID, gomock.Any()).
				Return(project, nil)
			h.projects.EXPECT().
				HasConcealedWork(gomock.Any(), gomock.Any(), h.projectID).
				Return(false, nil)

			view, err := h.service.Archive(context.Background(), h.workspaceID, h.projectID)
			if err != nil {
				t.Fatalf("archiving a %s project returned %v", state, err)
			}

			if !view.Project.Archived() {
				t.Errorf("the %s project came back unarchived", state)
			}
		})
	}
}

func TestTheLeadMayChangeTheirOwnProjectAndOtherMembersMayNot(t *testing.T) {
	t.Run("lead", func(t *testing.T) {
		h := newHarness(t)
		h.actAs(entity.MembershipRoleMember, h.leadID)

		h.projects.EXPECT().
			GetByID(gomock.Any(), h.workspaceID, h.projectID).
			Return(h.project(entity.ProjectStateActive), nil)
		h.statuses.EXPECT().
			Record(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, update entity.ProjectStatusUpdate) (entity.ProjectStatusUpdate, error) {
				return update, nil
			})

		if _, err := h.service.PostStatus(
			context.Background(),
			h.workspaceID,
			h.projectID,
			service.PostProjectStatusInput{Health: entity.ProjectHealthAtRisk, Body: "Auth slipped a week."},
		); err != nil {
			t.Fatalf(
				"the lead could not post their own project's status: %v — a lead who cannot say "+
					"how their project is going is a label, not a role",
				err,
			)
		}
	})

	t.Run("another member", func(t *testing.T) {
		h := newHarness(t)
		h.actAs(entity.MembershipRoleMember, uuid.New())

		h.projects.EXPECT().
			GetByID(gomock.Any(), h.workspaceID, h.projectID).
			Return(h.project(entity.ProjectStateActive), nil)
		h.statuses.EXPECT().Record(gomock.Any(), gomock.Any()).Times(0)

		_, err := h.service.PostStatus(
			context.Background(),
			h.workspaceID,
			h.projectID,
			service.PostProjectStatusInput{Health: entity.ProjectHealthOnTrack, Body: "Looks fine to me."},
		)

		if !errors.Is(err, entity.ErrProjectNotLead) {
			t.Fatalf("a member who is not the lead posted a status, or got %v instead of ErrProjectNotLead", err)
		}
	})

	t.Run("an admin who is not the lead", func(t *testing.T) {
		h := newHarness(t)
		h.actAs(entity.MembershipRoleAdmin, uuid.New())

		h.projects.EXPECT().
			GetByID(gomock.Any(), h.workspaceID, h.projectID).
			Return(h.project(entity.ProjectStateActive), nil)
		h.statuses.EXPECT().
			Record(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, update entity.ProjectStatusUpdate) (entity.ProjectStatusUpdate, error) {
				return update, nil
			})

		if _, err := h.service.PostStatus(
			context.Background(),
			h.workspaceID,
			h.projectID,
			service.PostProjectStatusInput{Health: entity.ProjectHealthOffTrack, Body: "Stalled."},
		); err != nil {
			t.Fatalf("an administrator could not post a status: %v", err)
		}
	})
}

func TestWhoeverStartsAProjectIsInIt(t *testing.T) {
	h := newHarness(t)
	founder := uuid.New()
	h.actAs(entity.MembershipRoleMember, founder)

	var joined entity.ProjectMembership

	h.projects.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, project entity.Project) (entity.Project, error) {
			project.ID = h.projectID

			return project, nil
		})
	h.members.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, membership entity.ProjectMembership) (entity.ProjectMembership, error) {
			joined = membership

			return membership, nil
		})
	h.members.EXPECT().ListByProjectID(gomock.Any(), h.projectID).Return(nil, nil).AnyTimes()
	h.projects.EXPECT().
		HasConcealedWork(gomock.Any(), gomock.Any(), h.projectID).
		Return(false, nil).
		AnyTimes()

	if _, err := h.service.Create(context.Background(), service.CreateProjectInput{
		WorkspaceID: h.workspaceID,
		Slug:        "offline-support",
		Name:        "Offline support",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if joined.AccountID != founder {
		t.Fatalf("the project was left to nobody: member = %v, want the creator %v", joined.AccountID, founder)
	}

	if joined.ProjectID != h.projectID {
		t.Fatalf("membership was filed under %v, want the new project %v", joined.ProjectID, h.projectID)
	}
}

func TestAnArchivedProjectTakesNoFurtherChanges(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.MembershipRoleAdmin, uuid.New())

	archivedAt := time.Now().UTC()
	project := h.project(entity.ProjectStateCompleted)
	project.ArchivedAt = &archivedAt

	h.projects.EXPECT().
		GetByID(gomock.Any(), h.workspaceID, h.projectID).
		Return(project, nil)
	h.members.EXPECT().Create(gomock.Any(), gomock.Any()).Times(0)

	_, err := h.service.AddMember(context.Background(), h.workspaceID, h.projectID, uuid.New())

	if !errors.Is(err, entity.ErrProjectArchived) {
		t.Fatalf("adding a member to an archived project returned %v, want ErrProjectArchived", err)
	}
}

func TestDeletingAProjectIsRefusedToEvenItsOwnLead(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.MembershipRoleMember, h.leadID)

	h.projects.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

	if err := h.service.Remove(context.Background(), h.workspaceID, h.projectID); !errors.Is(
		err, entity.ErrProjectNotLead,
	) {
		t.Fatalf(
			"the lead deleted the project, or got %v; deletion detaches every issue at once, so "+
				"it stays with workspace administrators",
			err,
		)
	}
}

func TestAProjectReportsWhetherItHoldsWorkTheCallerCannotSee(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.MembershipRoleMember, uuid.New())

	h.projects.EXPECT().
		GetByID(gomock.Any(), h.workspaceID, h.projectID).
		Return(h.project(entity.ProjectStateActive), nil)
	h.projects.EXPECT().
		HasConcealedWork(gomock.Any(), gomock.Any(), h.projectID).
		Return(true, nil)

	view, err := h.service.Get(context.Background(), h.workspaceID, h.projectID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if !view.ConcealedWork {
		t.Fatal(
			"the project did not report that it holds work on a team the caller cannot see; " +
				"without that flag the figures they are shown look like the whole project",
		)
	}
}

func TestProjectsNeverPutACountOnTheWire(t *testing.T) {
	carriesInt := func(t reflect.Type) bool {
		for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
			t = t.Elem()
		}

		if t.Kind() == reflect.Int {
			return true
		}

		if t.Kind() != reflect.Struct {
			return false
		}

		for i := range t.NumField() {
			if t.Field(i).Type.Kind() == reflect.Int {
				return true
			}
		}

		return false
	}

	surfaces := map[string]reflect.Type{
		"repository.Project": reflect.TypeOf((*repository.Project)(nil)).Elem(),
		"service.Projects":   reflect.TypeOf((*service.Projects)(nil)).Elem(),
	}

	for name, surface := range surfaces {
		for i := range surface.NumMethod() {
			method := surface.Method(i)

			for out := range method.Type.NumOut() {
				if carriesInt(method.Type.Out(out)) {
					t.Errorf(
						"%s.%s returns %s, which carries a count. A project's progress is the "+
							"per-category tally on the issue surface, computed inside the caller's "+
							"team scope; a count here would report work the caller cannot see.",
						name, method.Name, method.Type.Out(out),
					)
				}
			}
		}
	}

	for _, model := range []reflect.Type{
		reflect.TypeOf(entity.Project{}),
		reflect.TypeOf(entity.ProjectMembership{}),
		reflect.TypeOf(entity.ProjectStatusUpdate{}),
	} {
		for i := range model.NumField() {
			if model.Field(i).Type.Kind() == reflect.Int {
				t.Errorf(
					"%s carries an int field %q, which would travel to every reader",
					model, model.Field(i).Name,
				)
			}
		}
	}
}
