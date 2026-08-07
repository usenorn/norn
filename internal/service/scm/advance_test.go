package scm_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	activityrepo "github.com/usenorn/norn/internal/repository/activity"
	agentrepo "github.com/usenorn/norn/internal/repository/agent"
	agentproposalrepo "github.com/usenorn/norn/internal/repository/agentproposal"
	agentsettingrepo "github.com/usenorn/norn/internal/repository/agentsetting"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	labelrepo "github.com/usenorn/norn/internal/repository/label"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	scmrepo "github.com/usenorn/norn/internal/repository/scm"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	workflowstaterepo "github.com/usenorn/norn/internal/repository/workflowstate"
	"github.com/usenorn/norn/internal/service"
	"github.com/usenorn/norn/internal/service/agenthold"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	issuesvc "github.com/usenorn/norn/internal/service/issue"
	issuecommentsvc "github.com/usenorn/norn/internal/service/issuecomment"
	"github.com/usenorn/norn/internal/service/scm"
)

type advanceHarness struct {
	connections  *scmrepo.MockSCMConnection
	repositories *scmrepo.MockSCMRepository
	routes       *scmrepo.MockSCMRoute
	rules        *scmrepo.MockSCMTransitionRule
	teamSettings *scmrepo.MockSCMTeamSetting
	identities   *scmrepo.MockSCMIdentity
	conflicts    *scmrepo.MockMirrorConflict
	labels       *labelrepo.MockLabel
	agents       *agentrepo.MockAgent
	settings     *agentsettingrepo.MockAgentSetting
	proposals    *agentproposalrepo.MockAgentProposal
	releases     *scmrepo.MockSCMRelease
	deployments  *scmrepo.MockSCMDeployment
	deliveries   *scmrepo.MockSCMDelivery
	links        *scmrepo.MockCodeLink
	mirrors      *scmrepo.MockIssueMirror
	states       *workflowstaterepo.MockWorkflowState
	issues       *issuerepo.MockIssue
	activity     *activityrepo.MockActivity
	memberships  *membershiprepo.MockMembership
	forges       *scm.MockForges
	forge        *scm.MockForge
	authorizer   *authorizersvc.MockAuthorizer
	issueWriter  *issuesvc.MockIssues
	comments     *issuecommentsvc.MockIssueComments
	jobs         *jobqueuerepo.MockJobProducer
	sync         service.SourceControlSync
}

func newAdvanceHarness(t *testing.T) *advanceHarness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &advanceHarness{
		connections:  scmrepo.NewMockSCMConnection(ctrl),
		repositories: scmrepo.NewMockSCMRepository(ctrl),
		routes:       scmrepo.NewMockSCMRoute(ctrl),
		rules:        scmrepo.NewMockSCMTransitionRule(ctrl),
		teamSettings: scmrepo.NewMockSCMTeamSetting(ctrl),
		identities:   scmrepo.NewMockSCMIdentity(ctrl),
		conflicts:    scmrepo.NewMockMirrorConflict(ctrl),
		labels:       labelrepo.NewMockLabel(ctrl),
		agents:       agentrepo.NewMockAgent(ctrl),
		settings:     agentsettingrepo.NewMockAgentSetting(ctrl),
		proposals:    agentproposalrepo.NewMockAgentProposal(ctrl),
		releases:     scmrepo.NewMockSCMRelease(ctrl),
		deployments:  scmrepo.NewMockSCMDeployment(ctrl),
		deliveries:   scmrepo.NewMockSCMDelivery(ctrl),
		links:        scmrepo.NewMockCodeLink(ctrl),
		mirrors:      scmrepo.NewMockIssueMirror(ctrl),
		states:       workflowstaterepo.NewMockWorkflowState(ctrl),
		issues:       issuerepo.NewMockIssue(ctrl),
		activity:     activityrepo.NewMockActivity(ctrl),
		memberships:  membershiprepo.NewMockMembership(ctrl),
		forges:       scm.NewMockForges(ctrl),
		forge:        scm.NewMockForge(ctrl),
		authorizer:   authorizersvc.NewMockAuthorizer(ctrl),
		issueWriter:  issuesvc.NewMockIssues(ctrl),
		comments:     issuecommentsvc.NewMockIssueComments(ctrl),
		jobs:         jobqueuerepo.NewMockJobProducer(ctrl),
	}

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.sync = scm.NewSync(
		h.connections,
		h.repositories,
		h.routes,
		h.rules,
		h.teamSettings,
		h.identities,
		h.conflicts,
		h.labels,
		h.agents,
		agenthold.New(h.settings, h.proposals, h.agents),
		h.releases,
		h.deployments,
		h.deliveries,
		h.links,
		h.mirrors,
		h.states,
		h.issues,
		h.activity,
		h.memberships,
		h.forges,
		h.authorizer,
		h.issueWriter,
		h.comments,
		h.jobs,
		transactor,
		config.SourceControl{ReconcileBatch: 10, CallsPerCycle: 5, MaxAttempts: 3},
		config.App{BaseURL: "https://norn.example"},
	)

	return h
}

func TestAMergedChangeMovesItsIssueToWhereTheTeamAsked(t *testing.T) {
	h := newAdvanceHarness(t)

	var (
		workspaceID  = uuid.New()
		teamID       = uuid.New()
		issueID      = uuid.New()
		linkID       = uuid.New()
		connectionID = uuid.New()
		repositoryID = uuid.New()
		ownerID      = uuid.New()
		integration  = uuid.New()
		todoID       = uuid.New()
		doneID       = uuid.New()
	)

	connection := entity.SCMConnection{
		ID:                   connectionID,
		WorkspaceID:          workspaceID,
		Provider:             entity.SCMProviderGitHub,
		IntegrationAccountID: integration,
		OwnerAccountID:       ownerID,
		Status:               entity.SCMConnectionConnected,
	}

	stored := entity.SCMRepository{
		ID:           repositoryID,
		ConnectionID: connectionID,
		WorkspaceID:  workspaceID,
		Provider:     entity.SCMProviderGitHub,
		FullName:     "acme/api",
		MirrorLabel:  "norn",
	}

	issue := entity.Issue{
		ID:          issueID,
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Version:     1,
		State:       entity.IssueState{ID: todoID, Name: "Todo"},
	}

	link := entity.CodeLink{
		ID:             linkID,
		WorkspaceID:    workspaceID,
		IssueID:        issueID,
		RepositoryID:   repositoryID,
		Provider:       entity.SCMProviderGitHub,
		RepositoryName: "acme/api",
		Kind:           entity.CodeLinkChange,
		ExternalID:     "900123",
		Number:         14,
		State:          entity.CodeChangeMerged,
		Resolving:      true,
	}

	delivery := entity.SCMDelivery{
		ID:           uuid.New(),
		RepositoryID: repositoryID,
		WorkspaceID:  workspaceID,
		Event:        "pull_request",
		Payload:      []byte(`{}`),
	}

	scope := entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true, IncludePrivate: true}

	h.deliveries.EXPECT().GetByID(gomock.Any(), delivery.ID).Return(delivery, nil)
	h.repositories.EXPECT().GetForDelivery(gomock.Any(), repositoryID).Return(stored, nil)
	h.connections.EXPECT().GetForDelivery(gomock.Any(), connectionID).Return(connection, nil)
	h.connections.EXPECT().Token(gomock.Any(), connectionID).Return("token", nil)
	h.forges.EXPECT().Lookup(entity.SCMProviderGitHub).Return(h.forge, nil).AnyTimes()

	h.forge.EXPECT().
		ChangedPaths(gomock.Any(), gomock.Any(), 14).
		Return([]string{"services/api/cache.go"}, nil)

	h.routes.EXPECT().
		ListByRepository(gomock.Any(), repositoryID).
		Return(entity.SCMRoutes{{TeamID: teamID, RepositoryID: repositoryID}}, nil).
		AnyTimes()

	h.forge.EXPECT().Translate(gomock.Any()).Return([]service.ForgeEvent{{
		Kind: service.ForgeEventChangeChanged,
		Change: service.ForgeChange{
			ExternalID: "900123",
			Number:     14,
			Title:      "fixes ENG-1 drop the cache",
			HeadBranch: "eng-1-drop-the-cache",
			State:      entity.CodeChangeMerged,
		},
	}}, nil)

	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, ownerID).
		Return(entity.Membership{WorkspaceID: workspaceID, AccountID: ownerID, Role: entity.MembershipRoleAdmin}, nil)

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{Scope: scope, Role: entity.MembershipRoleAdmin}, nil)

	h.issues.EXPECT().
		GetVisibleByReference(gomock.Any(), workspaceID, entity.IssueReference{Key: "ENG", Number: 1}, scope).
		Return(issue, nil).
		AnyTimes()

	h.links.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(link, nil).AnyTimes()
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	h.links.EXPECT().
		ListByExternalID(gomock.Any(), workspaceID, entity.SCMProviderGitHub, "acme/api", "900123").
		Return([]entity.CodeLink{link}, nil)

	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, issueID, scope).Return(issue, nil)

	h.rules.EXPECT().
		ListByTeam(gomock.Any(), workspaceID, teamID).
		Return(entity.SCMTransitionRules{{
			TeamID:      teamID,
			WorkspaceID: workspaceID,
			Trigger:     entity.CodeChangeMerged,
			StateID:     doneID,
		}}, nil)

	h.states.EXPECT().ListByTeamID(gomock.Any(), teamID).Return([]entity.WorkflowState{
		{ID: todoID, Name: "Todo"},
		{ID: doneID, Name: "Done", IsCompletion: true},
	}, nil)

	h.issueWriter.EXPECT().
		Update(gomock.Any(), workspaceID, issueID, gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, _ uuid.UUID,
			input service.UpdateIssueInput,
		) (entity.Issue, error) {
			if input.StateID == nil || *input.StateID != doneID {
				t.Errorf("Update moved the issue to %v, want the state the team routed merges to", input.StateID)
			}

			if input.ExpectedVersion != issue.Version {
				t.Errorf(
					"Update offered version %d, want %d — without the version a person editing "+
						"at that moment is overwritten rather than left alone",
					input.ExpectedVersion, issue.Version,
				)
			}

			return issue, nil
		})

	h.links.EXPECT().
		ClaimTransition(gomock.Any(), linkID, entity.CodeChangeMerged, issueID, gomock.Any()).
		Return(true, nil)
	h.deliveries.EXPECT().
		Settle(gomock.Any(), delivery.ID, entity.SCMDeliveryApplied, gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ uuid.UUID,
			_ entity.SCMDeliveryOutcome,
			detail string,
			_ time.Time,
		) error {
			if !strings.Contains(detail, "advanced 1") {
				t.Errorf(
					"the delivery log recorded %q; without a count of what happened, a link that "+
						"never formed and one that formed read the same",
					detail,
				)
			}

			return nil
		})

	if err := h.sync.Apply(context.Background(), delivery.ID); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}
