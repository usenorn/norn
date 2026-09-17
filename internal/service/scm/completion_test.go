package scm_test

import (
	"context"
	stdsync "sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

type transitionKey struct {
	linkID     uuid.UUID
	transition entity.CodeChangeState
}

type issueChanges struct {
	h           *advanceHarness
	workspaceID uuid.UUID
	teamID      uuid.UUID
	todoID      uuid.UUID
	doneID      uuid.UUID

	mu           stdsync.Mutex
	issue        entity.Issue
	links        []entity.CodeLink
	connections  map[uuid.UUID]entity.SCMConnection
	repositories map[uuid.UUID]entity.SCMRepository
	deliveries   map[uuid.UUID]entity.CodeLink
	transitions  map[transitionKey]entity.CodeTransitionStatus
	childrenOpen bool
	moves        []entity.Actor
	claiming     chan struct{}
	claimed      chan struct{}
}

func newIssueChanges(t *testing.T) *issueChanges {
	t.Helper()

	h := newAdvanceHarness(t)

	c := &issueChanges{
		h:            h,
		workspaceID:  uuid.New(),
		teamID:       uuid.New(),
		todoID:       uuid.New(),
		doneID:       uuid.New(),
		connections:  map[uuid.UUID]entity.SCMConnection{},
		repositories: map[uuid.UUID]entity.SCMRepository{},
		deliveries:   map[uuid.UUID]entity.CodeLink{},
		transitions:  map[transitionKey]entity.CodeTransitionStatus{},
	}

	todoID := c.todoID

	c.issue = entity.Issue{
		ID:          uuid.New(),
		WorkspaceID: c.workspaceID,
		TeamID:      c.teamID,
		Version:     1,
		State:       entity.IssueState{ID: todoID, Name: "Todo"},
	}

	scope := entity.TeamScope{WorkspaceID: c.workspaceID, AllTeams: true, IncludePrivate: true}

	h.deliveries.EXPECT().GetByID(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, deliveryID uuid.UUID) (entity.SCMDelivery, error) {
			c.mu.Lock()
			defer c.mu.Unlock()

			return entity.SCMDelivery{
				ID:           deliveryID,
				RepositoryID: c.deliveries[deliveryID].RepositoryID,
				WorkspaceID:  c.workspaceID,
				Event:        "pull_request",
				Payload:      []byte(`{}`),
			}, nil
		},
	).AnyTimes()

	h.repositories.EXPECT().GetForDelivery(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, repositoryID uuid.UUID) (entity.SCMRepository, error) {
			c.mu.Lock()
			defer c.mu.Unlock()

			return c.repositories[repositoryID], nil
		},
	).AnyTimes()

	h.connections.EXPECT().GetForDelivery(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, connectionID uuid.UUID) (entity.SCMConnection, error) {
			c.mu.Lock()
			defer c.mu.Unlock()

			return c.connections[connectionID], nil
		},
	).AnyTimes()

	h.connections.EXPECT().Token(gomock.Any(), gomock.Any()).Return("token", nil).AnyTimes()
	h.forges.EXPECT().Lookup(gomock.Any()).Return(h.forge, nil).AnyTimes()

	h.forge.EXPECT().Translate(gomock.Any()).DoAndReturn(
		func(delivery entity.SCMDelivery) ([]service.ForgeEvent, error) {
			c.mu.Lock()
			defer c.mu.Unlock()

			link := c.deliveries[delivery.ID]

			return []service.ForgeEvent{{
				Kind: service.ForgeEventChangeChanged,
				Change: service.ForgeChange{
					ExternalID: link.ExternalID,
					Number:     link.Number,
					Title:      "fixes ENG-1 drop the cache",
					HeadBranch: "eng-1-drop-the-cache",
					State:      link.State,
				},
			}}, nil
		},
	).AnyTimes()

	h.forge.EXPECT().
		ChangedPaths(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]string{"services/api/cache.go"}, nil).
		AnyTimes()

	h.routes.EXPECT().ListByRepository(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, repositoryID uuid.UUID) (entity.SCMRoutes, error) {
			return entity.SCMRoutes{{TeamID: c.teamID, RepositoryID: repositoryID}}, nil
		},
	).AnyTimes()

	h.memberships.EXPECT().Get(gomock.Any(), c.workspaceID, gomock.Any()).DoAndReturn(
		func(_ context.Context, workspaceID, accountID uuid.UUID) (entity.Membership, error) {
			return entity.Membership{
				WorkspaceID: workspaceID,
				AccountID:   accountID,
				Role:        entity.MembershipRoleAdmin,
			}, nil
		},
	).AnyTimes()

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{Scope: scope, Role: entity.MembershipRoleAdmin}, nil).
		AnyTimes()

	h.issues.EXPECT().
		GetVisibleByReference(gomock.Any(), c.workspaceID, entity.IssueReference{Key: "ENG", Number: 1}, gomock.Any()).
		DoAndReturn(func(context.Context, uuid.UUID, entity.IssueReference, entity.TeamScope) (entity.Issue, error) {
			c.mu.Lock()
			defer c.mu.Unlock()

			return c.issue, nil
		}).
		AnyTimes()

	h.issues.EXPECT().
		GetVisible(gomock.Any(), c.workspaceID, c.issue.ID, gomock.Any()).
		DoAndReturn(func(context.Context, uuid.UUID, uuid.UUID, entity.TeamScope) (entity.Issue, error) {
			c.mu.Lock()
			defer c.mu.Unlock()

			return c.issue, nil
		}).
		AnyTimes()

	h.links.EXPECT().Upsert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, template entity.CodeLink) (entity.CodeLink, error) {
			c.mu.Lock()
			defer c.mu.Unlock()

			for _, link := range c.links {
				if link.ExternalID == template.ExternalID {
					return link, nil
				}
			}

			t.Errorf("a delivery upserted change %s the scenario never opened", template.ExternalID)

			return template, nil
		},
	).AnyTimes()

	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	h.links.EXPECT().
		ListByExternalID(gomock.Any(), c.workspaceID, gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uuid.UUID, _ entity.SCMProvider, _ string, externalID string,
		) ([]entity.CodeLink, error) {
			c.mu.Lock()
			defer c.mu.Unlock()

			for _, link := range c.links {
				if link.ExternalID == externalID {
					return []entity.CodeLink{link}, nil
				}
			}

			return nil, nil
		}).
		AnyTimes()

	h.links.EXPECT().ListByIssue(gomock.Any(), c.workspaceID, c.issue.ID).DoAndReturn(
		func(context.Context, uuid.UUID, uuid.UUID) ([]entity.CodeLink, error) {
			c.mu.Lock()
			defer c.mu.Unlock()

			return append([]entity.CodeLink(nil), c.links...), nil
		},
	).AnyTimes()

	h.rules.EXPECT().ListByTeam(gomock.Any(), c.workspaceID, c.teamID).Return(entity.SCMTransitionRules{{
		TeamID:      c.teamID,
		WorkspaceID: c.workspaceID,
		Trigger:     entity.CodeChangeMerged,
		StateID:     c.doneID,
	}}, nil).AnyTimes()

	h.states.EXPECT().ListByTeamID(gomock.Any(), c.teamID).Return([]entity.WorkflowState{
		{ID: todoID, Name: "Todo"},
		{ID: c.doneID, Name: "Done", IsCompletion: true},
	}, nil).AnyTimes()

	h.links.EXPECT().
		ClaimTransition(gomock.Any(), gomock.Any(), gomock.Any(), c.issue.ID, c.doneID, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, linkID uuid.UUID, transition entity.CodeChangeState, _, _ uuid.UUID, _ time.Time,
		) (bool, error) {
			if c.claiming != nil {
				c.claiming <- struct{}{}
				<-c.claimed
			}

			c.mu.Lock()
			defer c.mu.Unlock()

			key := transitionKey{linkID: linkID, transition: transition}
			if _, taken := c.transitions[key]; taken {
				return false, nil
			}

			c.transitions[key] = entity.CodeTransitionApplied

			return true, nil
		}).
		AnyTimes()

	h.links.EXPECT().DeferTransition(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(
			_ context.Context, linkID uuid.UUID, transition entity.CodeChangeState, _ entity.CodeTransitionBlock, _ time.Time,
		) error {
			c.mu.Lock()
			defer c.mu.Unlock()

			c.transitions[transitionKey{linkID: linkID, transition: transition}] = entity.CodeTransitionDeferred

			return nil
		},
	).AnyTimes()

	h.links.EXPECT().SettleTransition(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, linkID uuid.UUID, transition entity.CodeChangeState) error {
			c.mu.Lock()
			defer c.mu.Unlock()

			c.transitions[transitionKey{linkID: linkID, transition: transition}] = entity.CodeTransitionApplied

			return nil
		},
	).AnyTimes()

	h.links.EXPECT().ListDeferredTransitions(gomock.Any(), c.issue.ID).DoAndReturn(
		func(context.Context, uuid.UUID) ([]entity.CodeTransition, error) {
			c.mu.Lock()
			defer c.mu.Unlock()

			var deferred []entity.CodeTransition

			for key, status := range c.transitions {
				if status != entity.CodeTransitionDeferred {
					continue
				}

				deferred = append(deferred, entity.CodeTransition{
					LinkID:     key.linkID,
					IssueID:    c.issue.ID,
					Transition: key.transition,
					StateID:    c.doneID,
					Status:     status,
				})
			}

			return deferred, nil
		},
	).AnyTimes()

	h.issueWriter.EXPECT().Update(gomock.Any(), c.workspaceID, c.issue.ID, gomock.Any()).DoAndReturn(
		func(ctx context.Context, _, _ uuid.UUID, input service.UpdateIssueInput) (entity.Issue, error) {
			c.mu.Lock()
			defer c.mu.Unlock()

			if c.childrenOpen {
				return entity.Issue{}, entity.ErrIssueChildrenOpen
			}

			if input.ExpectedVersion != c.issue.Version {
				return entity.Issue{}, entity.ErrIssueStale
			}

			actor, _ := identity.Actor(ctx)
			c.moves = append(c.moves, actor)
			c.issue.State = entity.IssueState{ID: *input.StateID, Name: "Done"}
			c.issue.Version++

			return c.issue, nil
		},
	).AnyTimes()

	h.deliveries.EXPECT().Settle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	return c
}

func (c *issueChanges) repository(provider entity.SCMProvider, fullName string) entity.SCMRepository {
	c.mu.Lock()
	defer c.mu.Unlock()

	connection := entity.SCMConnection{
		ID:                   uuid.New(),
		WorkspaceID:          c.workspaceID,
		Provider:             provider,
		IntegrationAccountID: uuid.New(),
		OwnerAccountID:       uuid.New(),
		Status:               entity.SCMConnectionConnected,
	}

	repository := entity.SCMRepository{
		ID:           uuid.New(),
		ConnectionID: connection.ID,
		WorkspaceID:  c.workspaceID,
		Provider:     provider,
		FullName:     fullName,
		MirrorLabel:  "norn",
	}

	c.connections[connection.ID] = connection
	c.repositories[repository.ID] = repository

	return repository
}

func (c *issueChanges) open(repository entity.SCMRepository, resolving bool) entity.CodeLink {
	c.mu.Lock()
	defer c.mu.Unlock()

	link := entity.CodeLink{
		ID:             uuid.New(),
		WorkspaceID:    c.workspaceID,
		IssueID:        c.issue.ID,
		RepositoryID:   repository.ID,
		Provider:       repository.Provider,
		RepositoryName: repository.FullName,
		Kind:           entity.CodeLinkChange,
		ExternalID:     uuid.NewString(),
		Number:         len(c.links) + 1,
		State:          entity.CodeChangeOpen,
		Resolving:      resolving,
	}

	c.links = append(c.links, link)

	return link
}

func (c *issueChanges) settle(link entity.CodeLink, state entity.CodeChangeState) uuid.UUID {
	c.mu.Lock()
	defer c.mu.Unlock()

	at := time.Now().UTC().Add(time.Duration(len(c.deliveries)) * time.Minute)

	for index := range c.links {
		if c.links[index].ID != link.ID {
			continue
		}

		c.links[index].State = state
		switch state {
		case entity.CodeChangeMerged:
			c.links[index].MergedAt = &at
		case entity.CodeChangeClosed:
			c.links[index].ClosedAt = &at
		}

		link = c.links[index]
	}

	deliveryID := uuid.New()
	c.deliveries[deliveryID] = link

	return deliveryID
}

func (c *issueChanges) claimTogether(t *testing.T, claims int) {
	t.Helper()

	c.claiming = make(chan struct{}, claims)
	c.claimed = make(chan struct{})

	go func() {
		defer close(c.claimed)

		deadline := time.After(2 * time.Second)

		for range claims {
			select {
			case <-c.claiming:
			case <-deadline:
				t.Errorf("only some of %d deliveries reached the completion claim", claims)

				return
			}
		}
	}()
}

func (c *issueChanges) reopen() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.issue.State = entity.IssueState{ID: c.todoID, Name: "Todo"}
	c.issue.Version++
}

func (c *issueChanges) apply(t *testing.T, deliveryID uuid.UUID) {
	t.Helper()

	if err := c.h.sync.Apply(context.Background(), deliveryID); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}

func (c *issueChanges) resume(t *testing.T) {
	t.Helper()

	if err := c.h.sync.Resume(context.Background(), c.workspaceID, c.issue.ID); err != nil {
		t.Fatalf("Resume: %v", err)
	}
}

func (c *issueChanges) moved() []entity.Actor {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]entity.Actor(nil), c.moves...)
}

func (c *issueChanges) status(link entity.CodeLink) (entity.CodeTransitionStatus, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	status, found := c.transitions[transitionKey{linkID: link.ID, transition: entity.CodeChangeMerged}]

	return status, found
}

func TestTheFirstOfTwoMergesLeavesTheIssueWhereItIs(t *testing.T) {
	for _, pending := range []entity.CodeChangeState{
		entity.CodeChangeDraft,
		entity.CodeChangeOpen,
		entity.CodeChangeReopened,
		entity.CodeChangeConflicted,
	} {
		t.Run(string(pending), func(t *testing.T) {
			c := newIssueChanges(t)
			api := c.repository(entity.SCMProviderGitHub, "acme/api")
			web := c.repository(entity.SCMProviderGitHub, "acme/web")

			first := c.open(api, true)
			second := c.open(web, true)
			c.settle(second, pending)

			c.apply(t, c.settle(first, entity.CodeChangeMerged))

			if moves := c.moved(); len(moves) != 0 {
				t.Fatalf(
					"the issue moved %d times while another change resolving it was %s; it is not "+
						"done until that change lands too",
					len(moves), pending,
				)
			}

			if _, claimed := c.status(first); claimed {
				t.Fatal("the first merge claimed the completion, so the last one could never advance the issue")
			}
		})
	}
}

func TestAnIssueWhoseChangesWereAllClosedUnmergedIsNotDone(t *testing.T) {
	c := newIssueChanges(t)
	api := c.repository(entity.SCMProviderGitHub, "acme/api")

	first := c.open(api, true)
	second := c.open(api, true)

	c.apply(t, c.settle(first, entity.CodeChangeClosed))
	c.apply(t, c.settle(second, entity.CodeChangeClosed))

	if moves := c.moved(); len(moves) != 0 {
		t.Fatalf("the issue moved %d times though nothing resolving it was merged", len(moves))
	}
}

func TestTwoMergesLandingTogetherMoveTheIssueOnce(t *testing.T) {
	c := newIssueChanges(t)
	api := c.repository(entity.SCMProviderGitHub, "acme/api")
	web := c.repository(entity.SCMProviderGitHub, "acme/web")

	first := c.open(api, true)
	second := c.open(web, true)

	deliveries := []uuid.UUID{
		c.settle(first, entity.CodeChangeMerged),
		c.settle(second, entity.CodeChangeMerged),
	}

	c.claimTogether(t, len(deliveries))

	var wg stdsync.WaitGroup

	for _, deliveryID := range deliveries {
		wg.Go(func() {
			if err := c.h.sync.Apply(context.Background(), deliveryID); err != nil {
				t.Errorf("Apply: %v", err)
			}
		})
	}

	wg.Wait()

	if moves := c.moved(); len(moves) != 1 {
		t.Fatalf("two merges delivered together moved the issue %d times, want exactly once", len(moves))
	}

	firstStatus, firstClaimed := c.status(first)
	_, secondClaimed := c.status(second)

	if firstClaimed == secondClaimed {
		t.Fatalf(
			"completion claimed under first=%v second=%v; both deliveries must compete for the "+
				"one claim of the latest merge",
			firstClaimed, secondClaimed,
		)
	}

	if firstClaimed && firstStatus != entity.CodeTransitionApplied {
		t.Fatalf("the completion claim is %s, want applied", firstStatus)
	}
}

func TestTheLastChangeClosingInAnotherForgeCompletesTheIssueInItsOwnName(t *testing.T) {
	c := newIssueChanges(t)
	api := c.repository(entity.SCMProviderGitHub, "acme/api")
	web := c.repository(entity.SCMProviderGitLab, "acme/web")

	merged := c.open(api, true)
	abandoned := c.open(web, true)

	c.apply(t, c.settle(merged, entity.CodeChangeMerged))

	if moves := c.moved(); len(moves) != 0 {
		t.Fatalf("the issue moved %d times while a change in another forge was still open", len(moves))
	}

	c.apply(t, c.settle(abandoned, entity.CodeChangeClosed))

	moves := c.moved()
	if len(moves) != 1 {
		t.Fatalf("closing the last open change moved the issue %d times, want once", len(moves))
	}

	closing := c.connections[web.ConnectionID]
	if moves[0].ConnectionID == nil || *moves[0].ConnectionID != closing.ID {
		t.Fatalf(
			"the move was made as connection %v, want the %s connection whose delivery completed "+
				"the issue; attributing it to another forge names an actor that saw nothing",
			moves[0].ConnectionID, closing.Provider,
		)
	}

	if status, _ := c.status(abandoned); status != entity.CodeTransitionApplied {
		t.Fatalf("completion was not claimed under the change that settled last, status %q", status)
	}

	if _, claimed := c.status(merged); claimed {
		t.Fatal("completion was claimed under the earlier merge, not the change that settled last")
	}
}

func TestAChangeThatOnlyMentionsTheIssueDoesNotHoldItBack(t *testing.T) {
	c := newIssueChanges(t)
	api := c.repository(entity.SCMProviderGitHub, "acme/api")

	resolving := c.open(api, true)
	c.open(api, false)

	c.apply(t, c.settle(resolving, entity.CodeChangeMerged))

	if moves := c.moved(); len(moves) != 1 {
		t.Fatalf("the issue moved %d times, want once; a mention settles nothing", len(moves))
	}
}

func TestADeferredCompletionWaitsForAChangeOpenedAfterItAndAppliesOnce(t *testing.T) {
	for _, last := range []entity.CodeChangeState{entity.CodeChangeMerged, entity.CodeChangeClosed} {
		t.Run(string(last), func(t *testing.T) {
			c := newIssueChanges(t)
			api := c.repository(entity.SCMProviderGitHub, "acme/api")
			web := c.repository(entity.SCMProviderGitLab, "acme/web")

			first := c.open(api, true)
			second := c.open(web, true)

			c.mu.Lock()
			c.childrenOpen = true
			c.mu.Unlock()

			c.apply(t, c.settle(first, entity.CodeChangeMerged))
			c.apply(t, c.settle(second, entity.CodeChangeMerged))

			if status, _ := c.status(second); status != entity.CodeTransitionDeferred {
				t.Fatalf("completion under the latest merge is %q, want deferred while children are open", status)
			}

			late := c.open(api, true)

			c.mu.Lock()
			c.childrenOpen = false
			c.mu.Unlock()

			c.resume(t)

			if moves := c.moved(); len(moves) != 0 {
				t.Fatalf("resume moved the issue %d times while a newly opened change still resolves it", len(moves))
			}

			if status, _ := c.status(second); status != entity.CodeTransitionDeferred {
				t.Fatalf("resume settled the completion to %q before the late change landed", status)
			}

			c.apply(t, c.settle(late, last))

			if moves := c.moved(); len(moves) != 1 {
				t.Fatalf("the late change landing moved the issue %d times, want exactly once", len(moves))
			}

			if status, _ := c.status(second); status != entity.CodeTransitionApplied {
				t.Fatalf("the deferred completion is %q after it applied, want applied", status)
			}

			if _, claimed := c.status(late); claimed {
				t.Fatal("the late change opened a second completion claim beside the deferred one")
			}

			c.resume(t)
			c.apply(t, c.settle(late, last))

			if moves := c.moved(); len(moves) != 1 {
				t.Fatalf("replaying the resume and the delivery moved the issue %d times, want once", len(moves))
			}
		})
	}
}

func TestAReopenedIssueIsDoneAgainWhenItsNextChangeSettles(t *testing.T) {
	for _, last := range []entity.CodeChangeState{entity.CodeChangeMerged, entity.CodeChangeClosed} {
		t.Run(string(last), func(t *testing.T) {
			c := newIssueChanges(t)
			api := c.repository(entity.SCMProviderGitHub, "acme/api")

			first := c.open(api, true)
			c.apply(t, c.settle(first, entity.CodeChangeMerged))

			if moves := c.moved(); len(moves) != 1 {
				t.Fatalf("the first merge moved the issue %d times, want once", len(moves))
			}

			c.reopen()

			next := c.open(api, true)
			c.apply(t, c.settle(next, last))

			if moves := c.moved(); len(moves) != 2 {
				t.Fatalf(
					"the issue moved %d times after it was reopened and change %s, want Done again; "+
						"the old claim under the first merge must not swallow the new completion",
					len(moves), last,
				)
			}

			if status, _ := c.status(next); status != entity.CodeTransitionApplied {
				t.Fatalf("the new completion was not claimed under the change that settled last, status %q", status)
			}
		})
	}
}
