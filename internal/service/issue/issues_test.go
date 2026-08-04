package issue_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	cyclerepo "github.com/usenorn/norn/internal/repository/cycle"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	issueactivityrepo "github.com/usenorn/norn/internal/repository/issueactivity"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	labelrepo "github.com/usenorn/norn/internal/repository/label"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	projectrepo "github.com/usenorn/norn/internal/repository/project"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	workflowstaterepo "github.com/usenorn/norn/internal/repository/workflowstate"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	issuesvc "github.com/usenorn/norn/internal/service/issue"
)

type harness struct {
	issues      *issuerepo.MockIssue
	states      *workflowstaterepo.MockWorkflowState
	activity    *issueactivityrepo.MockIssueActivity
	labels      *labelrepo.MockLabel
	memberships *membershiprepo.MockMembership
	cycles      *cyclerepo.MockCycle
	scope       *cyclerepo.MockCycleScopeChange
	projects    *projectrepo.MockProject
	jobs        *jobqueuerepo.MockJobProducer
	transactor  *transactorrepo.MockTransactor
	authorizer  *authorizersvc.MockAuthorizer
	service     service.Issues
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		issues:      issuerepo.NewMockIssue(ctrl),
		states:      workflowstaterepo.NewMockWorkflowState(ctrl),
		activity:    issueactivityrepo.NewMockIssueActivity(ctrl),
		labels:      labelrepo.NewMockLabel(ctrl),
		memberships: membershiprepo.NewMockMembership(ctrl),
		cycles:      cyclerepo.NewMockCycle(ctrl),
		scope:       cyclerepo.NewMockCycleScopeChange(ctrl),
		projects:    projectrepo.NewMockProject(ctrl),
		jobs:        jobqueuerepo.NewMockJobProducer(ctrl),
		transactor:  transactorrepo.NewMockTransactor(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
	}

	h.transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.service = issuesvc.New(
		h.issues, h.states, h.activity, h.labels, h.memberships,
		h.cycles, h.scope, h.projects, h.jobs, h.authorizer, h.transactor,
	)

	return h
}

func (h *harness) expectScope(workspaceID uuid.UUID, scope entity.TeamScope) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()},
			Scope: scope,
		}, nil)
}

func (h *harness) expectRefused(err error) {
	h.authorizer.EXPECT().Decide(gomock.Any(), gomock.Any()).Return(entity.Decision{}, err)
}

func issuesOf(workspaceID, teamID uuid.UUID, count int) []entity.Issue {
	issues := make([]entity.Issue, 0, count)
	base := time.Now().UTC()

	for i := range count {
		issues = append(issues, entity.Issue{
			ID:          uuid.New(),
			WorkspaceID: workspaceID,
			TeamID:      teamID,
			TeamKey:     "MOB",
			Number:      i + 1,
			Title:       "An issue",
			CreatedAt:   base.Add(-time.Duration(i) * time.Minute),
		})
	}

	return issues
}

func TestAFullPageCarriesACursorWhenMoreVisibleIssuesExist(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{teamID}})

	var requested entity.IssuePage

	h.issues.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.TeamScope, page entity.IssuePage) ([]entity.Issue, error) {
			requested = page

			return issuesOf(workspaceID, teamID, 3), nil
		})

	page, err := h.service.List(context.Background(), workspaceID, service.ListIssuesInput{Limit: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if requested.Limit != 3 {
		t.Fatalf("the store was asked for %d rows, want the page size plus the lookahead row", requested.Limit)
	}

	if len(page.Issues) != 2 {
		t.Fatalf("returned %d issues, want a full page of 2", len(page.Issues))
	}

	if page.NextCursor == "" {
		t.Fatal("a full page must carry a cursor when more visible rows exist")
	}

	cursor, err := entity.DecodeIssueCursor(page.NextCursor)
	if err != nil {
		t.Fatalf("DecodeIssueCursor: %v", err)
	}

	if cursor.IssueID != page.Issues[1].ID {
		t.Fatal("the cursor must be built from the last kept row, not the discarded lookahead row")
	}
}

func TestAPartialPageCarriesNoCursor(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{teamID}})
	h.issues.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(issuesOf(workspaceID, teamID, 1), nil)

	page, err := h.service.List(context.Background(), workspaceID, service.ListIssuesInput{Limit: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if page.NextCursor != "" {
		t.Fatal("the absence of a next page is the only end-of-list signal")
	}
}

func TestTheVisibleTeamSetReachesTheStoreRatherThanBeingFilteredAfterwards(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	visible := uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{visible}})

	var captured entity.TeamScope

	h.issues.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, scope entity.TeamScope, _ entity.IssuePage) ([]entity.Issue, error) {
			captured = scope

			return nil, nil
		})

	if _, err := h.service.List(context.Background(), workspaceID, service.ListIssuesInput{}); err != nil {
		t.Fatalf("List: %v", err)
	}

	if captured.AllTeams {
		t.Fatal("an ordinary member must not be given the whole workspace")
	}

	if len(captured.TeamIDs) != 1 || captured.TeamIDs[0] != visible {
		t.Fatalf("the store was given %v, want exactly the visible team set", captured.TeamIDs)
	}
}

func TestAWorkspaceAdminIsGivenEveryTeam(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})

	var captured entity.TeamScope

	h.issues.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, scope entity.TeamScope, _ entity.IssuePage) ([]entity.Issue, error) {
			captured = scope

			return nil, nil
		})

	if _, err := h.service.List(context.Background(), workspaceID, service.ListIssuesInput{}); err != nil {
		t.Fatalf("List: %v", err)
	}

	if !captured.AllTeams {
		t.Fatal("an administrator sees every team's issues")
	}
}

func TestANonMemberIsRefusedBeforeTheStoreIsTouched(t *testing.T) {
	h := newHarness(t)

	h.expectRefused(entity.AccessDeniedError{
		Reason:   entity.DenyReasonNotAMember,
		Resource: entity.ResourceIssue,
	})

	_, err := h.service.List(context.Background(), uuid.New(), service.ListIssuesInput{})

	if !errors.Is(err, entity.ErrIssueNotFound) {
		t.Fatalf("List error = %v, want the issue resource to conceal itself", err)
	}

	if errors.Is(err, entity.ErrAccountForbidden) {
		t.Fatal("issues must not answer forbidden, which would confirm the workspace has content")
	}
}

func TestRaisingAnIssueInAnInvisibleTeamReportsTheTeamMissing(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{uuid.New()}})

	_, err := h.service.Create(context.Background(), service.CreateIssueInput{
		WorkspaceID: workspaceID,
		TeamID:      uuid.New(),
		Title:       "An issue",
	})

	if !errors.Is(err, entity.ErrTeamNotFound) {
		t.Fatalf("Create error = %v, want ErrTeamNotFound", err)
	}
}

func TestAnIssueIsRaisedAgainstTheActorWhoRaisedIt(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()
	actorID := uuid.New()

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: actorID},
			Scope: entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{teamID}},
		}, nil)

	h.states.EXPECT().
		DefaultForTeam(gomock.Any(), teamID).
		Return(entity.WorkflowState{ID: uuid.New(), TeamID: teamID, IsDefault: true}, nil)
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil)

	var captured entity.Issue

	h.issues.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, issue entity.Issue) (entity.Issue, error) {
			captured = issue

			return issue, nil
		})

	if _, err := h.service.Create(context.Background(), service.CreateIssueInput{
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Title:       "An issue",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if captured.CreatedByAccountID != actorID {
		t.Fatalf("author = %v, want the acting account %v", captured.CreatedByAccountID, actorID)
	}
}

func TestAnEmptyTitleIsRefused(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{teamID}})

	_, err := h.service.Create(context.Background(), service.CreateIssueInput{
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Title:       "   ",
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Create error = %v, want a ValidationError", err)
	}
}

func TestAMalformedCursorIsAValidationError(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})

	_, err := h.service.List(context.Background(), workspaceID, service.ListIssuesInput{Cursor: "not-a-cursor"})

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("List error = %v, want a ValidationError", err)
	}
}

func TestNoLayerExposesAnIssueCountingOperation(t *testing.T) {
	forbidden := []string{"count", "total", "quota", "seat", "limit"}

	surfaces := map[string]reflect.Type{
		"repository.Issue": reflect.TypeOf((*repository.Issue)(nil)).Elem(),
		"service.Issues":   reflect.TypeOf((*service.Issues)(nil)).Elem(),
	}

	for name, surface := range surfaces {
		for i := range surface.NumMethod() {
			method := strings.ReplaceAll(strings.ToLower(surface.Method(i).Name), "account", "")

			for _, word := range forbidden {
				if strings.Contains(method, word) {
					t.Errorf("%s exposes %q, which would put an issue count on the wire", name, surface.Method(i).Name)
				}
			}
		}
	}

	pages := map[string]reflect.Type{
		"service.IssuePage":         reflect.TypeOf(service.IssuePage{}),
		"service.IssueActivityPage": reflect.TypeOf(service.IssueActivityPage{}),
	}

	for name, page := range pages {
		for i := range page.NumField() {
			field := strings.ToLower(page.Field(i).Name)

			for _, word := range forbidden {
				if strings.Contains(field, word) {
					t.Errorf("%s carries %q, which would put a total on the wire", name, page.Field(i).Name)
				}
			}
		}
	}
}

func TestProgressIsOnlyEverFourPerCategoryTallies(t *testing.T) {
	progress := reflect.TypeOf(entity.IssueProgress{})

	if progress.NumField() != len(entity.StateCategories()) {
		t.Fatalf(
			"entity.IssueProgress has %d fields, want exactly one per workflow category; "+
				"a per-team or per-assignee breakdown would leak what the team guard conceals",
			progress.NumField(),
		)
	}

	for i := range progress.NumField() {
		if progress.Field(i).Type.Kind() != reflect.Int {
			t.Errorf(
				"entity.IssueProgress.%s is %s, want a plain per-category tally",
				progress.Field(i).Name,
				progress.Field(i).Type,
			)
		}
	}
}

func TestDeletingAnIssueSchedulesItsPurgeForWhenTheGraceElapses(t *testing.T) {
	h := newHarness(t)

	workspaceID, teamID, issueID := uuid.New(), uuid.New(), uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{teamID}})

	h.issues.EXPECT().
		LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(entity.Issue{ID: issueID, TeamID: teamID, Version: 3, Status: entity.IssueStatusActive}, nil)

	var scheduled entity.IssueLifecycle

	h.issues.EXPECT().
		SetStatus(gomock.Any(), issueID, 3, gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uuid.UUID, _ int, lifecycle entity.IssueLifecycle, _ time.Time,
		) error {
			scheduled = lifecycle

			return nil
		})

	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil)

	var processAt time.Time

	h.jobs.EXPECT().
		EnqueueIssuePurge(gomock.Any(), entity.IssuePurgePayload{IssueID: issueID}, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.IssuePurgePayload, at time.Time) error {
			processAt = at

			return nil
		})

	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(entity.Issue{}, nil)

	if _, err := h.service.SetStatus(context.Background(), workspaceID, issueID, service.SetIssueStatusInput{
		ExpectedVersion: 3,
		Status:          entity.IssueStatusPendingDeletion,
	}); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	if scheduled.PurgeAfter == nil || !processAt.Equal(*scheduled.PurgeAfter) {
		t.Fatalf(
			"the job runs at %s but the row says the purge is due at %v. A job that fires early "+
				"would find the grace unexpired and silently do nothing, so the issue is never collected.",
			processAt, scheduled.PurgeAfter,
		)
	}
}

func TestArchivingAnIssueNeverSchedulesAPurge(t *testing.T) {
	h := newHarness(t)

	workspaceID, teamID, issueID := uuid.New(), uuid.New(), uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{teamID}})

	h.issues.EXPECT().
		LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(entity.Issue{ID: issueID, TeamID: teamID, Version: 1, Status: entity.IssueStatusActive}, nil)
	h.issues.EXPECT().SetStatus(gomock.Any(), issueID, 1, gomock.Any(), gomock.Any()).Return(nil)
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil)
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(entity.Issue{}, nil)

	if _, err := h.service.SetStatus(context.Background(), workspaceID, issueID, service.SetIssueStatusInput{
		ExpectedVersion: 1,
		Status:          entity.IssueStatusArchived,
	}); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
}

func TestRestoringADeletedIssueThatWasArchivedReturnsItToTheArchive(t *testing.T) {
	h := newHarness(t)

	workspaceID, teamID, issueID := uuid.New(), uuid.New(), uuid.New()
	archivedAt := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{teamID}})

	h.issues.EXPECT().
		LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(entity.Issue{
			ID:         issueID,
			TeamID:     teamID,
			Version:    5,
			Status:     entity.IssueStatusPendingDeletion,
			ArchivedAt: &archivedAt,
		}, nil)

	var restored entity.IssueLifecycle

	h.issues.EXPECT().
		SetStatus(gomock.Any(), issueID, 5, gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uuid.UUID, _ int, lifecycle entity.IssueLifecycle, _ time.Time,
		) error {
			restored = lifecycle

			return nil
		})

	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil)
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(entity.Issue{}, nil)

	if _, err := h.service.SetStatus(context.Background(), workspaceID, issueID, service.SetIssueStatusInput{
		ExpectedVersion: 5,
		Status:          entity.IssueStatusActive,
	}); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	if restored.Status != entity.IssueStatusArchived {
		t.Fatalf(
			"restored to %q. An issue that was archived before it was deleted belongs back in the "+
				"archive; returning it to the active list resurfaces work nobody asked to see again.",
			restored.Status,
		)
	}

	if restored.PurgeAfter != nil {
		t.Fatal("restoring left the purge scheduled")
	}
}

func TestAListWithNoStatusAsksOnlyForActiveIssues(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})

	var requested []entity.IssueStatus

	h.issues.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.TeamScope, page entity.IssuePage) ([]entity.Issue, error) {
			requested = page.Statuses

			return nil, nil
		})

	if _, err := h.service.List(context.Background(), workspaceID, service.ListIssuesInput{}); err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(requested) != 1 || requested[0] != entity.IssueStatusActive {
		t.Fatalf(
			"asked for %v. An unfiltered list must not surface archived or deleted issues alongside "+
				"live work.",
			requested,
		)
	}
}
