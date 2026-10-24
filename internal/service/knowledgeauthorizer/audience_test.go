package knowledgeauthorizer_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	projectrepo "github.com/usenorn/norn/internal/repository/project"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	"github.com/usenorn/norn/internal/service/knowledgeauthorizer"
)

type harness struct {
	authorizer *authorizersvc.MockAuthorizer
	projects   *projectrepo.MockProject
	service    service.KnowledgeAuthorizer

	workspaceID uuid.UUID
	projectID   uuid.UUID
	author      uuid.UUID
	requests    []entity.AccessRequest
	listedFor   []uuid.UUID
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		projects:    projectrepo.NewMockProject(ctrl),
		workspaceID: uuid.New(),
		projectID:   uuid.New(),
		author:      uuid.New(),
	}

	h.service = knowledgeauthorizer.New(h.authorizer, h.projects)

	return h
}

func (h *harness) actAs(actor entity.Actor, role entity.MembershipRole) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			h.requests = append(h.requests, request)

			return entity.Decision{Actor: actor, Role: role, Workspace: entity.Workspace{ID: h.workspaceID}}, nil
		}).
		AnyTimes()
}

func (h *harness) refuse(err error) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			h.requests = append(h.requests, request)

			return entity.Decision{}, err
		}).
		AnyTimes()
}

func (h *harness) onProjects(projects map[uuid.UUID][]entity.Project) {
	h.projects.EXPECT().
		ListByWorkspaceID(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, filter repository.ProjectFilter) ([]entity.Project, error) {
			if filter.ForAccountID == nil || !filter.IncludeArchived || filter.State != "" {
				return nil, errors.New("the audience was not loaded as every project the account is on, archived or not")
			}

			h.listedFor = append(h.listedFor, *filter.ForAccountID)

			return projects[*filter.ForAccountID], nil
		}).
		AnyTimes()
}

func (h *harness) item(audience entity.KnowledgeAudience) entity.KnowledgeAccess {
	return entity.KnowledgeAccess{WorkspaceID: h.workspaceID, AuthorID: h.author, Audience: audience}
}

func (h *harness) items() map[entity.KnowledgeVisibility]entity.KnowledgeAccess {
	return map[entity.KnowledgeVisibility]entity.KnowledgeAccess{
		entity.KnowledgeVisibilityPersonal:  h.item(entity.PersonalAudience()),
		entity.KnowledgeVisibilityProject:   h.item(entity.ProjectAudience(h.projectID)),
		entity.KnowledgeVisibilityWorkspace: h.item(entity.WorkspaceAudience()),
	}
}

func agentActing(owner uuid.UUID, scope entity.AgentScope, projectID *uuid.UUID) entity.Actor {
	return entity.Agent{
		ID:             uuid.New(),
		AccountID:      uuid.New(),
		OwnerAccountID: owner,
		Scope:          scope,
		ProjectID:      projectID,
	}.Attribute(entity.Actor{Kind: entity.ActorKindAgent, AccountID: uuid.New()})
}

type actorCase struct {
	name  string
	actor func(h *harness) entity.Actor
	role  entity.MembershipRole
	onP   bool
	sees  []entity.KnowledgeVisibility
}

var everyActor = []actorCase{
	{
		name:  "the author",
		actor: func(h *harness) entity.Actor { return entity.Actor{Kind: entity.ActorKindUser, AccountID: h.author} },
		role:  entity.MembershipRoleMember,
		onP:   true,
		sees: []entity.KnowledgeVisibility{
			entity.KnowledgeVisibilityPersonal, entity.KnowledgeVisibilityProject, entity.KnowledgeVisibilityWorkspace,
		},
	},
	{
		name:  "a member on the project",
		actor: func(*harness) entity.Actor { return entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()} },
		role:  entity.MembershipRoleMember,
		onP:   true,
		sees:  []entity.KnowledgeVisibility{entity.KnowledgeVisibilityProject, entity.KnowledgeVisibilityWorkspace},
	},
	{
		name:  "a member off the project",
		actor: func(*harness) entity.Actor { return entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()} },
		role:  entity.MembershipRoleMember,
		sees:  []entity.KnowledgeVisibility{entity.KnowledgeVisibilityWorkspace},
	},
	{
		name:  "an admin off the project",
		actor: func(*harness) entity.Actor { return entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()} },
		role:  entity.MembershipRoleAdmin,
		sees:  []entity.KnowledgeVisibility{entity.KnowledgeVisibilityProject, entity.KnowledgeVisibilityWorkspace},
	},
	{
		name:  "the author's personal agent",
		actor: func(h *harness) entity.Actor { return agentActing(h.author, entity.AgentScopeMember, nil) },
		role:  entity.MembershipRoleMember,
		onP:   true,
		sees: []entity.KnowledgeVisibility{
			entity.KnowledgeVisibilityPersonal, entity.KnowledgeVisibilityProject, entity.KnowledgeVisibilityWorkspace,
		},
	},
	{
		name: "a project agent the author shares",
		actor: func(h *harness) entity.Actor {
			return agentActing(h.author, entity.AgentScopeProject, &h.projectID)
		},
		role: entity.MembershipRoleMember,
		onP:  true,
		sees: []entity.KnowledgeVisibility{entity.KnowledgeVisibilityProject, entity.KnowledgeVisibilityWorkspace},
	},
	{
		name: "a project agent whose owner left the project",
		actor: func(h *harness) entity.Actor {
			return agentActing(uuid.New(), entity.AgentScopeProject, &h.projectID)
		},
		role: entity.MembershipRoleMember,
		sees: []entity.KnowledgeVisibility{entity.KnowledgeVisibilityWorkspace},
	},
	{
		name:  "a workspace agent the author shares",
		actor: func(h *harness) entity.Actor { return agentActing(h.author, entity.AgentScopeWorkspace, nil) },
		role:  entity.MembershipRoleAdmin,
		onP:   true,
		sees:  []entity.KnowledgeVisibility{entity.KnowledgeVisibilityWorkspace},
	},
}

func (h *harness) setUp(tc actorCase) entity.Actor {
	actor := tc.actor(h)
	h.actAs(actor, tc.role)

	onProject := map[uuid.UUID][]entity.Project{}
	if tc.onP {
		onProject[actor.Authority()] = []entity.Project{{ID: h.projectID, WorkspaceID: h.workspaceID}}
	}

	h.onProjects(onProject)

	return actor
}

func TestEveryActorReadsExactlyItsAudience(t *testing.T) {
	for _, tc := range everyActor {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.setUp(tc)

			for visibility, item := range h.items() {
				_, err := h.service.Read(t.Context(), item)

				switch sees := slices.Contains(tc.sees, visibility); {
				case sees && err != nil:
					t.Errorf("reading %s knowledge = %v, want it shown", visibility, err)
				case !sees && err != entity.ErrKnowledgeNotFound:
					t.Errorf("reading %s knowledge = %v, want not found", visibility, err)
				}
			}

			for _, request := range h.requests {
				if request.Resource != entity.ResourceKnowledge || request.Action != entity.ActionRead ||
					request.WorkspaceID != h.workspaceID {
					t.Errorf("a read was authorized as %+v", request)
				}
			}
		})
	}
}

func TestAnItemOutsideTheAudienceIsIndistinguishableFromOneThatDoesNotExist(t *testing.T) {
	absent := entity.ErrKnowledgeNotFound

	for _, tc := range everyActor {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.setUp(tc)

			for visibility, item := range h.items() {
				if slices.Contains(tc.sees, visibility) {
					continue
				}

				_, readErr := h.service.Read(t.Context(), item)
				_, changeErr := h.service.Change(t.Context(), item)
				_, reshareErr := h.service.Reshare(t.Context(), item, entity.WorkspaceAudience())

				for operation, err := range map[string]error{"read": readErr, "change": changeErr, "reshare": reshareErr} {
					if err != absent || err.Error() != absent.Error() {
						t.Errorf(
							"%s of %s knowledge outside the audience = %v, but a missing item answers %v. "+
								"The difference tells the caller the item exists.",
							operation, visibility, err, absent,
						)
					}
				}
			}
		})
	}

	t.Run("a workspace outsider", func(t *testing.T) {
		h := newHarness(t)
		h.refuse(entity.AccessDeniedError{
			Reason:      entity.DenyReasonNotAMember,
			Resource:    entity.ResourceKnowledge,
			Action:      entity.ActionRead,
			WorkspaceID: h.workspaceID,
		})

		for visibility, item := range h.items() {
			_, err := h.service.Read(t.Context(), item)
			if !errors.Is(err, absent) || err.Error() != absent.Error() {
				t.Errorf("an outsider reading %s knowledge = %v, want %v", visibility, err, absent)
			}
		}
	})
}

func TestTheAudienceFilterNamesOnlyProjectsEveryDriverMayRead(t *testing.T) {
	for _, tc := range everyActor {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			actor := h.setUp(tc)

			decision, err := h.service.Reader(t.Context(), h.workspaceID)
			if err != nil {
				t.Fatalf("Reader: %v", err)
			}

			reader := decision.Reader
			wantProject := slices.Contains(tc.sees, entity.KnowledgeVisibilityProject)

			if reader.Reaches(h.projectID) != wantProject {
				t.Errorf("the filter reaches the project = %v, want %v", reader.Reaches(h.projectID), wantProject)
			}

			if reader.Reaches(uuid.New()) != (tc.role == entity.MembershipRoleAdmin && !reader.Identity.Shared) {
				t.Error("the filter reached a project nobody put this actor on")
			}

			for _, listed := range h.listedFor {
				if listed != actor.Authority() {
					t.Errorf("projects were loaded for %s, want the actor's authority %s", listed, actor.Authority())
				}
			}

			if reader.Identity.Shared && actor.AgentScope != entity.AgentScopeProject && len(h.listedFor) > 0 {
				t.Error("a shared agent with no project loaded its owner's projects")
			}
		})
	}
}

func TestTheProjectLeadMaintainsTheProjectsKnowledge(t *testing.T) {
	h := newHarness(t)
	lead := uuid.New()

	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: lead}, entity.MembershipRoleMember)
	h.onProjects(map[uuid.UUID][]entity.Project{
		lead: {{ID: h.projectID, WorkspaceID: h.workspaceID, LeadAccountID: lead}},
	})

	if _, err := h.service.Change(t.Context(), h.item(entity.ProjectAudience(h.projectID))); err != nil {
		t.Fatalf("the lead changing project knowledge = %v, want allowed", err)
	}

	if _, err := h.service.Change(t.Context(), h.item(entity.WorkspaceAudience())); !errors.Is(err, entity.ErrKnowledgeNotEditable) {
		t.Fatalf("the lead changing workspace knowledge = %v, want not editable", err)
	}

	for _, request := range h.requests {
		if request.Resource != entity.ResourceKnowledge || request.Action != entity.ActionManage {
			t.Errorf("a change was authorized as %+v", request)
		}
	}
}

func TestStartingKnowledgeChecksTheAudienceItIsFor(t *testing.T) {
	h := newHarness(t)
	viewer := uuid.New()

	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: viewer}, entity.MembershipRoleViewer)
	h.onProjects(map[uuid.UUID][]entity.Project{
		viewer: {{ID: h.projectID, WorkspaceID: h.workspaceID}},
	})

	if _, err := h.service.Author(t.Context(), h.workspaceID, entity.PersonalAudience()); err != nil {
		t.Fatalf("a viewer keeping a personal note = %v, want allowed", err)
	}

	_, err := h.service.Author(t.Context(), h.workspaceID, entity.ProjectAudience(h.projectID))
	if !errors.Is(err, entity.ErrKnowledgeNotShareable) {
		t.Fatalf("a viewer publishing to their project = %v, want not shareable", err)
	}
}

func TestARefusedDecisionIsReturnedUntouched(t *testing.T) {
	h := newHarness(t)
	h.refuse(entity.ErrAgentRateLimited)

	if _, err := h.service.Author(t.Context(), h.workspaceID, entity.WorkspaceAudience()); !errors.Is(err, entity.ErrAgentRateLimited) {
		t.Fatalf("Author = %v, want the authorizer's refusal", err)
	}
}
