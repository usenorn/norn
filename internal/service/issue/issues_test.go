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
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	activityrepo "github.com/usenorn/norn/internal/repository/activity"
	agentrepo "github.com/usenorn/norn/internal/repository/agent"
	agentproposalrepo "github.com/usenorn/norn/internal/repository/agentproposal"
	agentsettingrepo "github.com/usenorn/norn/internal/repository/agentsetting"
	checkrepo "github.com/usenorn/norn/internal/repository/check"
	cyclerepo "github.com/usenorn/norn/internal/repository/cycle"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	delegationrepo "github.com/usenorn/norn/internal/repository/issuedelegation"
	issuefollowerrepo "github.com/usenorn/norn/internal/repository/issuefollower"
	questionrepo "github.com/usenorn/norn/internal/repository/issuequestion"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	labelrepo "github.com/usenorn/norn/internal/repository/label"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	notificationeventrepo "github.com/usenorn/norn/internal/repository/notificationevent"
	projectrepo "github.com/usenorn/norn/internal/repository/project"
	scmrepo "github.com/usenorn/norn/internal/repository/scm"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	triagerepo "github.com/usenorn/norn/internal/repository/triage"
	workflowstaterepo "github.com/usenorn/norn/internal/repository/workflowstate"
	"github.com/usenorn/norn/internal/service"
	"github.com/usenorn/norn/internal/service/agenthold"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	"github.com/usenorn/norn/internal/service/checkgate"
	eventsvc "github.com/usenorn/norn/internal/service/event"
	issuesvc "github.com/usenorn/norn/internal/service/issue"
)

type harness struct {
	issues      *issuerepo.MockIssue
	states      *workflowstaterepo.MockWorkflowState
	activity    *activityrepo.MockActivity
	labels      *labelrepo.MockLabel
	accounts    *accountrepo.MockAccount
	memberships *membershiprepo.MockMembership
	cycles      *cyclerepo.MockCycle
	scope       *cyclerepo.MockCycleScopeChange
	projects    *projectrepo.MockProject
	teams       *teamrepo.MockTeam
	triage      *triagerepo.MockTriage
	notify      *notificationeventrepo.MockNotificationEvent
	delegations *delegationrepo.MockIssueDelegation
	questions   *questionrepo.MockIssueQuestion
	notified    []entity.NotificationEvent
	delegatedBy uuid.UUID
	asked       []entity.IssueQuestion
	events      *eventsvc.MockEvents
	followers   *issuefollowerrepo.MockIssueFollower
	jobs        *jobqueuerepo.MockJobProducer
	checks      *checkrepo.MockCheck
	evidence    *checkrepo.MockCheckEvidence
	codeLinks   *scmrepo.MockCodeLink
	transactor  *transactorrepo.MockTransactor
	authorizer  *authorizersvc.MockAuthorizer
	settings    *agentsettingrepo.MockAgentSetting
	proposals   *agentproposalrepo.MockAgentProposal
	agents      *agentrepo.MockAgent
	actor       entity.Actor
	blocking    []entity.Check
	holds       entity.AgentSettings
	service     service.Issues
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		issues:      issuerepo.NewMockIssue(ctrl),
		states:      workflowstaterepo.NewMockWorkflowState(ctrl),
		activity:    activityrepo.NewMockActivity(ctrl),
		labels:      labelrepo.NewMockLabel(ctrl),
		accounts:    accountrepo.NewMockAccount(ctrl),
		memberships: membershiprepo.NewMockMembership(ctrl),
		cycles:      cyclerepo.NewMockCycle(ctrl),
		scope:       cyclerepo.NewMockCycleScopeChange(ctrl),
		projects:    projectrepo.NewMockProject(ctrl),
		teams:       teamrepo.NewMockTeam(ctrl),
		triage:      triagerepo.NewMockTriage(ctrl),
		notify:      notificationeventrepo.NewMockNotificationEvent(ctrl),
		delegations: delegationrepo.NewMockIssueDelegation(ctrl),
		questions:   questionrepo.NewMockIssueQuestion(ctrl),
		events:      eventsvc.NewMockEvents(ctrl),
		followers:   issuefollowerrepo.NewMockIssueFollower(ctrl),
		jobs:        jobqueuerepo.NewMockJobProducer(ctrl),
		checks:      checkrepo.NewMockCheck(ctrl),
		evidence:    checkrepo.NewMockCheckEvidence(ctrl),
		codeLinks:   scmrepo.NewMockCodeLink(ctrl),
		transactor:  transactorrepo.NewMockTransactor(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		settings:    agentsettingrepo.NewMockAgentSetting(ctrl),
		proposals:   agentproposalrepo.NewMockAgentProposal(ctrl),
		agents:      agentrepo.NewMockAgent(ctrl),
		actor:       entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()},
	}

	h.transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.service = issuesvc.New(
		h.issues, h.states, h.activity, h.labels, h.accounts, h.memberships,
		h.cycles, h.scope, h.projects, h.teams, h.triage, h.notify, h.events,
		silentEmitter(ctrl), h.followers,
		h.jobs,
		agenthold.New(
			h.settings,
			h.proposals,
			h.agents,
			h.states,
			h.delegations,
			h.questions,
			h.notify,
			checkgate.New(h.checks, h.evidence, h.codeLinks),
		),
		checkgate.New(h.checks, h.evidence, h.codeLinks),
		h.authorizer, h.transactor,
	)

	h.triage.EXPECT().
		Settings(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.TriageSettings{}, entity.ErrTriageDisabled).
		AnyTimes()

	h.issues.EXPECT().LowestRank(gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()
	h.notify.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, event entity.NotificationEvent) error {
			h.notified = append(h.notified, event)

			return nil
		}).
		AnyTimes()

	h.delegations.EXPECT().
		Open(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, issueID uuid.UUID) (entity.IssueDelegation, error) {
			if h.delegatedBy == uuid.Nil {
				return entity.IssueDelegation{}, entity.ErrIssueDelegationNotFound
			}

			return entity.IssueDelegation{
				IssueID:              issueID,
				DelegatedByAccountID: h.delegatedBy,
			}, nil
		}).
		AnyTimes()
	h.questions.EXPECT().
		ListByIssue(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID) ([]entity.IssueQuestion, error) {
			return h.asked, nil
		}).
		AnyTimes()
	h.events.EXPECT().Publish(gomock.Any(), gomock.Any()).AnyTimes()
	h.followers.EXPECT().Follow(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	h.checks.EXPECT().
		ListByIssue(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID) ([]entity.Check, error) {
			return h.blocking, nil
		}).
		AnyTimes()

	h.evidence.EXPECT().
		Digest(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	h.codeLinks.EXPECT().
		ListByIssue(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	h.settings.EXPECT().
		Settings(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID) (entity.AgentSettings, error) {
			return h.holds, nil
		}).
		AnyTimes()

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

	cursor, err := entity.DecodeIssueQueryCursor(page.NextCursor)
	if err != nil {
		t.Fatalf("DecodeIssueQueryCursor: %v", err)
	}

	if cursor.IssueID != page.Issues[1].ID {
		t.Fatal("the cursor must be built from the last kept row, not the discarded lookahead row")
	}
}

func TestAListCursorNamesWhereTheNextPageStartsFrom(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{teamID}})
	h.issues.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(issuesOf(workspaceID, teamID, 3), nil)

	first, err := h.service.List(context.Background(), workspaceID, service.ListIssuesInput{Limit: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{teamID}})

	var resumed entity.IssuePage

	h.issues.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.TeamScope, page entity.IssuePage) ([]entity.Issue, error) {
			resumed = page

			return nil, nil
		})

	_, err = h.service.List(context.Background(), workspaceID, service.ListIssuesInput{
		Limit:  2,
		Cursor: first.NextCursor,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if resumed.QueryCursor == nil {
		t.Fatal("a cursor must reach the store as a keyset position, not a field the query ignores")
	}

	if resumed.QueryCursor.IssueID != first.Issues[1].ID {
		t.Fatalf(
			"the second page resumes from %s, want the last row of the first page",
			resumed.QueryCursor.IssueID,
		)
	}

	if len(resumed.QueryCursor.Keys) != len(resumed.Sort) {
		t.Fatalf(
			"the cursor carries %d sort keys against %d sort terms, so the keyset is silently discarded",
			len(resumed.QueryCursor.Keys),
			len(resumed.Sort),
		)
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

	if captured.Origin != nil {
		t.Fatal("an issue raised by hand carries an import origin, so the repository would date it from a source it does not have")
	}
}

func TestAnImportedIssueIsFiledUnderItsOriginalDateAndAuthor(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()
	actorID := uuid.New()
	sourceAuthorID := uuid.New()

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

	createdAt := time.Date(2019, time.April, 2, 9, 15, 0, 0, time.UTC)
	updatedAt := createdAt.Add(72 * time.Hour)
	origin := entity.NewImportOrigin(createdAt, updatedAt, sourceAuthorID)

	if _, err := h.service.Create(context.Background(), service.CreateIssueInput{
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Title:       "Offline queue drops edits on reconnect",
		Origin:      &origin,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if captured.CreatedByAccountID != sourceAuthorID {
		t.Fatalf(
			"author = %v, want the source author %v rather than whoever ran the import",
			captured.CreatedByAccountID, sourceAuthorID,
		)
	}

	if captured.Origin == nil {
		t.Fatal("the origin never reached the repository, so the row would be dated the moment the import ran")
	}

	gotCreated, gotUpdated := entity.OriginStamp(captured.Origin, time.Now().UTC())

	if !gotCreated.Equal(createdAt) || !gotUpdated.Equal(updatedAt) {
		t.Fatalf("stamp = (%v, %v), want (%v, %v)", gotCreated, gotUpdated, createdAt, updatedAt)
	}
}

func TestAnIssueCanBeRaisedStraightIntoAChosenStateProjectAndLabels(t *testing.T) {
	h := newHarness(t)

	workspaceID, teamID := uuid.New(), uuid.New()
	stateID, projectID, labelID := uuid.New(), uuid.New(), uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{teamID}})

	h.states.EXPECT().
		ListByTeamID(gomock.Any(), teamID).
		Return([]entity.WorkflowState{
			{ID: uuid.New(), TeamID: teamID, Name: "Backlog", IsDefault: true},
			{ID: stateID, TeamID: teamID, Name: "In progress"},
		}, nil)

	label := entity.Label{ID: labelID, WorkspaceID: workspaceID, Name: "Bug"}

	h.labels.EXPECT().ListByIDs(gomock.Any(), workspaceID, []uuid.UUID{labelID}).Return([]entity.Label{label}, nil)
	h.projects.EXPECT().
		GetByID(gomock.Any(), workspaceID, projectID).
		Return(entity.Project{ID: projectID, WorkspaceID: workspaceID}, nil)
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil)

	var (
		captured entity.Issue
		applied  []entity.Label
	)

	h.issues.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, issue entity.Issue) (entity.Issue, error) {
			captured = issue
			issue.ID = uuid.New()

			return issue, nil
		})

	h.labels.EXPECT().
		SetForIssue(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.Issue, labels []entity.Label) error {
			applied = labels

			return nil
		})

	created, err := h.service.Create(context.Background(), service.CreateIssueInput{
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Title:       "Offline queue drops edits on reconnect",
		StateID:     stateID,
		ProjectID:   projectID,
		LabelIDs:    []uuid.UUID{labelID},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if captured.State.ID != stateID {
		t.Errorf("state = %v, want the chosen state %v", captured.State.ID, stateID)
	}

	if captured.ProjectID != projectID {
		t.Errorf("project = %v, want %v", captured.ProjectID, projectID)
	}

	if len(applied) != 1 || applied[0].ID != labelID {
		t.Errorf("labels applied = %v, want the one label asked for", applied)
	}

	if len(created.Labels) != 1 {
		t.Errorf("the issue came back with %d labels, want the ones it was created with", len(created.Labels))
	}
}

func TestRaisingAnIssueIntoAStateAnotherTeamOwnsIsRefused(t *testing.T) {
	h := newHarness(t)

	workspaceID, teamID := uuid.New(), uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{teamID}})

	h.states.EXPECT().
		ListByTeamID(gomock.Any(), teamID).
		Return([]entity.WorkflowState{{ID: uuid.New(), TeamID: teamID, IsDefault: true}}, nil)

	_, err := h.service.Create(context.Background(), service.CreateIssueInput{
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Title:       "An issue",
		StateID:     uuid.New(),
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Create error = %v, want a ValidationError naming stateId", err)
	}
}

func TestRaisingAnIssueIntoAnArchivedProjectIsRefused(t *testing.T) {
	h := newHarness(t)

	workspaceID, teamID, projectID := uuid.New(), uuid.New(), uuid.New()
	archived := time.Now().UTC()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{teamID}})

	h.states.EXPECT().
		DefaultForTeam(gomock.Any(), teamID).
		Return(entity.WorkflowState{ID: uuid.New(), TeamID: teamID, IsDefault: true}, nil)
	h.projects.EXPECT().
		GetByID(gomock.Any(), workspaceID, projectID).
		Return(entity.Project{ID: projectID, WorkspaceID: workspaceID, ArchivedAt: &archived}, nil)

	_, err := h.service.Create(context.Background(), service.CreateIssueInput{
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Title:       "An issue",
		ProjectID:   projectID,
	})
	if !errors.Is(err, entity.ErrProjectArchived) {
		t.Fatalf("Create error = %v, want ErrProjectArchived", err)
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
		"service.IssuePage":    reflect.TypeOf(service.IssuePage{}),
		"service.ActivityPage": reflect.TypeOf(service.ActivityPage{}),
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
		EnqueueIssuePurge(
			gomock.Any(),
			entity.IssuePurgePayload{IssueID: issueID, WorkspaceID: workspaceID},
			gomock.Any(),
		).
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

func TestAnIssueDroppedBetweenTwoOthersLandsBetweenTheirRanks(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()
	scope := entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{teamID}}

	h.expectScope(workspaceID, scope)

	moving := issuesOf(workspaceID, teamID, 3)
	above, below, dragged := moving[0], moving[1], moving[2]
	above.Rank, below.Rank, dragged.Rank = "a", "b", "z"

	h.issues.EXPECT().
		GetVisible(gomock.Any(), workspaceID, above.ID, gomock.Any()).
		Return(above, nil)
	h.issues.EXPECT().
		GetVisible(gomock.Any(), workspaceID, below.ID, gomock.Any()).
		Return(below, nil)
	h.issues.EXPECT().
		LockByID(gomock.Any(), workspaceID, dragged.ID, gomock.Any()).
		Return(dragged, nil)

	var written entity.IssueChange

	h.issues.EXPECT().
		Update(gomock.Any(), dragged.ID, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uuid.UUID, _ int,
			change entity.IssueChange, _ *entity.StateTimestamps, _ time.Time,
		) error {
			written = change

			return nil
		})

	h.issues.EXPECT().
		GetVisible(gomock.Any(), workspaceID, dragged.ID, gomock.Any()).
		Return(dragged, nil).
		AnyTimes()
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	if _, err := h.service.Update(context.Background(), workspaceID, dragged.ID, service.UpdateIssueInput{
		ExpectedVersion: dragged.Version,
		AfterIssueID:    &above.ID,
		BeforeIssueID:   &below.ID,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if written.Rank == nil {
		t.Fatal("dropping between two issues wrote no rank, so the card returns to where it was")
	}

	if *written.Rank <= above.Rank || *written.Rank >= below.Rank {
		t.Fatalf(
			"the drop wrote rank %q, which does not sort between %q and %q",
			*written.Rank, above.Rank, below.Rank,
		)
	}
}

func TestADropWithNoNeighboursLeavesTheOrderAlone(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{teamID}})

	held := issuesOf(workspaceID, teamID, 1)[0]
	held.Rank = "i"
	title := "Renamed"

	h.issues.EXPECT().LockByID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(held, nil)

	var written entity.IssueChange

	h.issues.EXPECT().
		Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uuid.UUID, _ int,
			change entity.IssueChange, _ *entity.StateTimestamps, _ time.Time,
		) error {
			written = change

			return nil
		})

	h.issues.EXPECT().GetVisible(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(held, nil).AnyTimes()
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	if _, err := h.service.Update(context.Background(), workspaceID, held.ID, service.UpdateIssueInput{
		ExpectedVersion: held.Version,
		Title:           &title,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if written.Rank != nil {
		t.Fatalf(
			"an edit that named no neighbours rewrote the rank to %q, which would shuffle the "+
				"board every time somebody renames an issue",
			*written.Rank,
		)
	}
}
