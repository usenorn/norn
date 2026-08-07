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
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	scmrepo "github.com/usenorn/norn/internal/repository/scm"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	workflowstaterepo "github.com/usenorn/norn/internal/repository/workflowstate"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	issuesvc "github.com/usenorn/norn/internal/service/issue"
	issuecommentsvc "github.com/usenorn/norn/internal/service/issuecomment"
	"github.com/usenorn/norn/internal/service/scm"
)

type advanceHarness struct {
	connections *scmrepo.MockSCMConnection
	deliveries  *scmrepo.MockSCMDelivery
	links       *scmrepo.MockCodeLink
	mirrors     *scmrepo.MockIssueMirror
	settings    *scmrepo.MockSCMTeamSetting
	states      *workflowstaterepo.MockWorkflowState
	issues      *issuerepo.MockIssue
	activity    *activityrepo.MockActivity
	memberships *membershiprepo.MockMembership
	forges      *scm.MockForges
	forge       *scm.MockForge
	authorizer  *authorizersvc.MockAuthorizer
	issueWriter *issuesvc.MockIssues
	comments    *issuecommentsvc.MockIssueComments
	jobs        *jobqueuerepo.MockJobProducer
	sync        service.SourceControlSync
}

func newAdvanceHarness(t *testing.T) *advanceHarness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &advanceHarness{
		connections: scmrepo.NewMockSCMConnection(ctrl),
		deliveries:  scmrepo.NewMockSCMDelivery(ctrl),
		links:       scmrepo.NewMockCodeLink(ctrl),
		mirrors:     scmrepo.NewMockIssueMirror(ctrl),
		settings:    scmrepo.NewMockSCMTeamSetting(ctrl),
		states:      workflowstaterepo.NewMockWorkflowState(ctrl),
		issues:      issuerepo.NewMockIssue(ctrl),
		activity:    activityrepo.NewMockActivity(ctrl),
		memberships: membershiprepo.NewMockMembership(ctrl),
		forges:      scm.NewMockForges(ctrl),
		forge:       scm.NewMockForge(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		issueWriter: issuesvc.NewMockIssues(ctrl),
		comments:    issuecommentsvc.NewMockIssueComments(ctrl),
		jobs:        jobqueuerepo.NewMockJobProducer(ctrl),
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
		h.deliveries,
		h.links,
		h.mirrors,
		h.settings,
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
		ownerID      = uuid.New()
		integration  = uuid.New()
		todoID       = uuid.New()
		doneID       = uuid.New()
	)

	connection := entity.SCMConnection{
		ID:                   connectionID,
		WorkspaceID:          workspaceID,
		TeamID:               teamID,
		Provider:             entity.SCMProviderGitHub,
		Repository:           "acme/api",
		IntegrationAccountID: integration,
		OwnerAccountID:       ownerID,
		MirrorLabel:          "norn",
		Status:               entity.SCMConnectionConnected,
	}

	issue := entity.Issue{
		ID:          issueID,
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Version:     1,
		State:       entity.IssueState{ID: todoID, Name: "Todo"},
	}

	link := entity.CodeLink{
		ID:           linkID,
		WorkspaceID:  workspaceID,
		IssueID:      issueID,
		ConnectionID: connectionID,
		Provider:     entity.SCMProviderGitHub,
		Repository:   "acme/api",
		Kind:         entity.CodeLinkChange,
		ExternalID:   "900123",
		Number:       14,
		State:        entity.CodeChangeMerged,
	}

	delivery := entity.SCMDelivery{
		ID:           uuid.New(),
		ConnectionID: connectionID,
		WorkspaceID:  workspaceID,
		Event:        "pull_request",
		Payload:      []byte(`{}`),
	}

	scope := entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true, IncludePrivate: true}

	h.deliveries.EXPECT().GetByID(gomock.Any(), delivery.ID).Return(delivery, nil)
	h.connections.EXPECT().GetForDelivery(gomock.Any(), connectionID).Return(connection, nil)
	h.forges.EXPECT().Lookup(entity.SCMProviderGitHub).Return(h.forge, nil).AnyTimes()

	h.forge.EXPECT().Translate(gomock.Any()).Return([]service.ForgeEvent{{
		Kind: service.ForgeEventChangeChanged,
		Change: service.ForgeChange{
			ExternalID: "900123",
			Number:     14,
			Title:      "ENG-1 drop the cache",
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

	h.settings.EXPECT().
		Settings(gomock.Any(), workspaceID, teamID).
		Return(entity.SCMTeamSettings{TeamID: teamID, WorkspaceID: workspaceID, AdvanceOnMerge: true}, nil)

	h.states.EXPECT().ListByTeamID(gomock.Any(), teamID).Return([]entity.WorkflowState{
		{ID: todoID, Name: "Todo"},
		{ID: doneID, Name: "Done", IsCompletion: true},
	}, nil)

	// The whole point: the issue is moved, and to the state the team's completion marks.
	h.issueWriter.EXPECT().
		Update(gomock.Any(), workspaceID, issueID, gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, _ uuid.UUID,
			input service.UpdateIssueInput,
		) (entity.Issue, error) {
			if input.StateID == nil || *input.StateID != doneID {
				t.Errorf("Update moved the issue to %v, want the team's completion state", input.StateID)
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

	h.links.EXPECT().MarkAdvanced(gomock.Any(), linkID).Return(nil)
	// The log has to say the delivery did something, not merely that it was processed.
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
