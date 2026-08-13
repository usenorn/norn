package scm_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type openedChange struct {
	workspaceID uuid.UUID
	issueID     uuid.UUID
	linkID      uuid.UUID
	reviewID    uuid.UUID
	deliveryID  uuid.UUID
	issue       entity.Issue
}

func (h *advanceHarness) openedChange(t *testing.T) openedChange {
	t.Helper()

	var (
		workspaceID  = uuid.New()
		teamID       = uuid.New()
		issueID      = uuid.New()
		linkID       = uuid.New()
		connectionID = uuid.New()
		repositoryID = uuid.New()
		ownerID      = uuid.New()
		todoID       = uuid.New()
		reviewID     = uuid.New()
	)

	connection := entity.SCMConnection{
		ID:             connectionID,
		WorkspaceID:    workspaceID,
		Provider:       entity.SCMProviderGitHub,
		OwnerAccountID: ownerID,
		Status:         entity.SCMConnectionConnected,
	}

	stored := entity.SCMRepository{
		ID:           repositoryID,
		ConnectionID: connectionID,
		WorkspaceID:  workspaceID,
		Provider:     entity.SCMProviderGitHub,
		FullName:     "acme/api",
	}

	issue := entity.Issue{
		ID:           issueID,
		WorkspaceID:  workspaceID,
		TeamID:       teamID,
		ReferenceKey: "ENG",
		Number:       1,
		Title:        "Drop the cache",
		Version:      1,
		State:        entity.IssueState{ID: todoID, Name: "Todo"},
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
		State:          entity.CodeChangeOpen,
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

	h.forge.EXPECT().ChangedPaths(gomock.Any(), gomock.Any(), 14).Return(nil, nil)

	h.routes.EXPECT().
		ListByRepository(gomock.Any(), repositoryID).
		Return(entity.SCMRoutes{}, nil).
		AnyTimes()

	h.forge.EXPECT().Translate(gomock.Any()).Return([]service.ForgeEvent{{
		Kind: service.ForgeEventChangeChanged,
		Change: service.ForgeChange{
			ExternalID: "900123",
			Number:     14,
			Title:      "Drop the cache",
			HeadBranch: "vlad/eng-1-drop-the-cache",
			State:      entity.CodeChangeOpen,
		},
	}}, nil)

	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, ownerID).
		Return(entity.Membership{
			WorkspaceID: workspaceID,
			AccountID:   ownerID,
			Role:        entity.MembershipRoleAdmin,
		}, nil)

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

	h.workspaces.EXPECT().
		GetByID(gomock.Any(), workspaceID).
		Return(entity.Workspace{ID: workspaceID, Slug: "northwind"}, nil).
		AnyTimes()

	h.rules.EXPECT().
		ListByTeam(gomock.Any(), workspaceID, teamID).
		Return(entity.SCMTransitionRules{{
			TeamID:      teamID,
			WorkspaceID: workspaceID,
			Trigger:     entity.CodeChangeOpen,
			StateID:     reviewID,
		}}, nil)

	h.states.EXPECT().ListByTeamID(gomock.Any(), teamID).Return([]entity.WorkflowState{
		{ID: todoID, Name: "Todo"},
		{ID: reviewID, Name: "In review", Category: entity.StateCategoryActive, Position: 2},
	}, nil)

	h.deliveries.EXPECT().
		Settle(gomock.Any(), delivery.ID, gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	return openedChange{
		workspaceID: workspaceID,
		issueID:     issueID,
		linkID:      linkID,
		reviewID:    reviewID,
		deliveryID:  delivery.ID,
		issue:       issue,
	}
}

func TestAnOpenedChangeTellsTheForgeWhichIssueItResolves(t *testing.T) {
	h := newAdvanceHarness(t)
	opened := h.openedChange(t)

	h.links.EXPECT().
		ClaimAnnouncement(gomock.Any(), opened.linkID, gomock.Any()).
		Return(true, nil)

	var announced string

	h.forge.EXPECT().
		PostChangeComment(gomock.Any(), gomock.Any(), 14, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.SCMTarget, _ int, body string) error {
			announced = body

			return nil
		})

	h.links.EXPECT().
		ClaimTransition(gomock.Any(), opened.linkID, entity.CodeChangeOpen, opened.issueID, opened.reviewID, gomock.Any()).
		Return(true, nil)

	h.issueWriter.EXPECT().
		Update(gomock.Any(), opened.workspaceID, opened.issueID, gomock.Any()).
		Return(opened.issue, nil)

	if err := h.sync.Apply(context.Background(), opened.deliveryID); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	for _, want := range []string{
		"ENG-1",
		"Drop the cache",
		"https://norn.example/northwind/issues/ENG-1",
	} {
		if !strings.Contains(announced, want) {
			t.Errorf(
				"the comment left on the change is %q, want it to carry %q — a reviewer reading "+
					"the change has to be able to reach the issue from it",
				announced, want,
			)
		}
	}
}

func TestAChangeIsOnlyAnnouncedOnceHoweverOftenTheForgeRedelivers(t *testing.T) {
	h := newAdvanceHarness(t)
	opened := h.openedChange(t)

	h.links.EXPECT().
		ClaimAnnouncement(gomock.Any(), opened.linkID, gomock.Any()).
		Return(false, nil)

	h.links.EXPECT().
		ClaimTransition(gomock.Any(), opened.linkID, entity.CodeChangeOpen, opened.issueID, opened.reviewID, gomock.Any()).
		Return(true, nil)

	h.issueWriter.EXPECT().
		Update(gomock.Any(), opened.workspaceID, opened.issueID, gomock.Any()).
		Return(opened.issue, nil)

	if err := h.sync.Apply(context.Background(), opened.deliveryID); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}

func TestAnAnnouncementTheForgeRefusedIsTriedAgain(t *testing.T) {
	h := newAdvanceHarness(t)
	opened := h.openedChange(t)

	h.links.EXPECT().
		ClaimAnnouncement(gomock.Any(), opened.linkID, gomock.Any()).
		Return(true, nil)

	h.forge.EXPECT().
		PostChangeComment(gomock.Any(), gomock.Any(), 14, gomock.Any()).
		Return(errors.New("the forge said no"))

	h.links.EXPECT().ReleaseAnnouncement(gomock.Any(), opened.linkID).Return(nil)

	h.links.EXPECT().
		ClaimTransition(gomock.Any(), opened.linkID, entity.CodeChangeOpen, opened.issueID, opened.reviewID, gomock.Any()).
		Return(true, nil)

	h.issueWriter.EXPECT().
		Update(gomock.Any(), opened.workspaceID, opened.issueID, gomock.Any()).
		Return(opened.issue, nil)

	if err := h.sync.Apply(context.Background(), opened.deliveryID); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}
