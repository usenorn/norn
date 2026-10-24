package entity_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

type knowledgeWorld struct {
	workspaceID uuid.UUID
	projectID   uuid.UUID
	author      uuid.UUID
	contributor uuid.UUID
	items       map[string]entity.KnowledgeAccess
}

func newKnowledgeWorld() knowledgeWorld {
	world := knowledgeWorld{
		workspaceID: uuid.New(),
		projectID:   uuid.New(),
		author:      uuid.New(),
		contributor: uuid.New(),
	}

	world.items = map[string]entity.KnowledgeAccess{
		"personal":  world.item(entity.PersonalAudience()),
		"project":   world.item(entity.ProjectAudience(world.projectID)),
		"workspace": world.item(entity.WorkspaceAudience()),
	}

	return world
}

func (w knowledgeWorld) item(audience entity.KnowledgeAudience) entity.KnowledgeAccess {
	return entity.KnowledgeAccess{
		WorkspaceID:    w.workspaceID,
		AuthorID:       w.author,
		Audience:       audience,
		ContributorIDs: []uuid.UUID{w.contributor},
	}
}

func (w knowledgeWorld) reader(actor entity.Actor, role entity.MembershipRole) entity.KnowledgeReader {
	return entity.KnowledgeReader{
		WorkspaceID: w.workspaceID,
		Identity:    entity.KnowledgeIdentityOf(actor, role),
	}
}

func person(accountID uuid.UUID) entity.Actor {
	return entity.Actor{Kind: entity.ActorKindUser, AccountID: accountID}
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

func TestKnowledgeIsReadAndChangedOnlyByItsAudience(t *testing.T) {
	world := newKnowledgeWorld()
	outsider := uuid.New()
	admin := uuid.New()
	lead := uuid.New()
	project := []uuid.UUID{world.projectID}

	withProjects := func(reader entity.KnowledgeReader, projectIDs ...uuid.UUID) entity.KnowledgeReader {
		reader.ProjectIDs = projectIDs
		return reader
	}

	cases := []struct {
		name     string
		reader   entity.KnowledgeReader
		sees     []string
		edits    []string
		reshares []string
	}{
		{
			name:     "the author",
			reader:   withProjects(world.reader(person(world.author), entity.MembershipRoleMember), project...),
			sees:     []string{"personal", "project", "workspace"},
			edits:    []string{"personal", "project", "workspace"},
			reshares: []string{"personal", "project", "workspace"},
		},
		{
			name:   "a contributor on the project",
			reader: withProjects(world.reader(person(world.contributor), entity.MembershipRoleMember), project...),
			sees:   []string{"project", "workspace"},
			edits:  []string{"project", "workspace"},
		},
		{
			name:   "a member off the project",
			reader: world.reader(person(outsider), entity.MembershipRoleMember),
			sees:   []string{"workspace"},
		},
		{
			name:   "a viewer on the project",
			reader: withProjects(world.reader(person(uuid.New()), entity.MembershipRoleViewer), project...),
			sees:   []string{"project", "workspace"},
		},
		{
			name: "the project lead",
			reader: func() entity.KnowledgeReader {
				reader := withProjects(world.reader(person(lead), entity.MembershipRoleMember), project...)
				reader.LedProjectIDs = project

				return reader
			}(),
			sees:     []string{"project", "workspace"},
			edits:    []string{"project"},
			reshares: []string{"project"},
		},
		{
			name: "an admin off the project",
			reader: func() entity.KnowledgeReader {
				reader := world.reader(person(admin), entity.MembershipRoleAdmin)
				reader.AllProjects = true

				return reader
			}(),
			sees:     []string{"project", "workspace"},
			edits:    []string{"project", "workspace"},
			reshares: []string{"project", "workspace"},
		},
		{
			name: "the author's personal agent",
			reader: withProjects(
				world.reader(agentActing(world.author, entity.AgentScopeMember, nil), entity.MembershipRoleMember),
				project...,
			),
			sees:     []string{"personal", "project", "workspace"},
			edits:    []string{"personal", "project", "workspace"},
			reshares: []string{"personal", "project", "workspace"},
		},
		{
			name:   "another member's personal agent",
			reader: world.reader(agentActing(outsider, entity.AgentScopeMember, nil), entity.MembershipRoleMember),
			sees:   []string{"workspace"},
		},
		{
			name: "a project agent the author shares",
			reader: withProjects(
				world.reader(
					agentActing(world.author, entity.AgentScopeProject, &world.projectID),
					entity.MembershipRoleMember,
				),
				project...,
			),
			sees: []string{"project", "workspace"},
		},
		{
			name: "a workspace agent an admin shares",
			reader: world.reader(
				agentActing(admin, entity.AgentScopeWorkspace, nil),
				entity.MembershipRoleAdmin,
			),
			sees: []string{"workspace"},
		},
		{
			name: "a member of another workspace",
			reader: entity.KnowledgeReader{
				WorkspaceID: uuid.New(),
				Identity:    entity.KnowledgeIdentityOf(person(world.author), entity.MembershipRoleAdmin),
				AllProjects: true,
			},
		},
	}

	for _, tc := range cases {
		for _, visibility := range entity.KnowledgeVisibilities() {
			item := world.items[string(visibility)]
			sees := slices.Contains(tc.sees, string(visibility))
			edits := slices.Contains(tc.edits, string(visibility))
			reshares := slices.Contains(tc.reshares, string(visibility))

			if got := tc.reader.Sees(item); got != sees {
				t.Errorf("%s sees %s knowledge = %v, want %v", tc.name, visibility, got, sees)
			}

			if got := tc.reader.Edits(item); got != edits {
				t.Errorf("%s edits %s knowledge = %v, want %v", tc.name, visibility, got, edits)
			}

			if got := tc.reader.Reshares(item); got != reshares {
				t.Errorf("%s reshares %s knowledge = %v, want %v", tc.name, visibility, got, reshares)
			}

			readErr := tc.reader.Read(item)
			changeErr := tc.reader.Change(item)

			switch {
			case !sees:
				if readErr != entity.ErrKnowledgeNotFound || changeErr != entity.ErrKnowledgeNotFound {
					t.Errorf(
						"%s reading or changing %s knowledge it cannot see = %v / %v, want not found both times. "+
							"Anything else tells an outsider the item exists.",
						tc.name, visibility, readErr, changeErr,
					)
				}
			case !edits:
				if readErr != nil || !errors.Is(changeErr, entity.ErrKnowledgeNotEditable) {
					t.Errorf("%s on %s knowledge = %v / %v, want readable but not editable",
						tc.name, visibility, readErr, changeErr)
				}
			default:
				if readErr != nil || changeErr != nil {
					t.Errorf("%s on %s knowledge = %v / %v, want readable and editable",
						tc.name, visibility, readErr, changeErr)
				}
			}
		}
	}
}

func TestWhereKnowledgeAppliesNeverDecidesWhoReadsIt(t *testing.T) {
	world := newKnowledgeWorld()
	projectMember := entity.KnowledgeReader{
		WorkspaceID: world.workspaceID,
		Identity:    entity.KnowledgeIdentityOf(person(uuid.New()), entity.MembershipRoleMember),
		ProjectIDs:  []uuid.UUID{world.projectID},
	}
	outsider := entity.KnowledgeReader{
		WorkspaceID: world.workspaceID,
		Identity:    entity.KnowledgeIdentityOf(person(uuid.New()), entity.MembershipRoleMember),
	}

	private := world.item(entity.PersonalAudience())
	private.Applicability = entity.KnowledgeApplicability{ProjectID: &world.projectID, Component: "billing"}

	if projectMember.Sees(private) {
		t.Fatal("a personal note about the project was shown to the project's members because of its label")
	}

	open := world.item(entity.WorkspaceAudience())
	open.Applicability = entity.KnowledgeApplicability{ProjectID: &world.projectID}

	if !outsider.Sees(open) {
		t.Fatal("workspace knowledge about one project was hidden from members who are off that project")
	}
}

func TestOnlyAPersonActingForThemselvesReadsTheirPersonalKnowledge(t *testing.T) {
	owner := uuid.New()
	integration := uuid.New()
	projectID := uuid.New()

	cases := []struct {
		name      string
		actor     entity.Actor
		principal uuid.UUID
		personal  bool
	}{
		{name: "a signed-in person", actor: person(owner), principal: owner, personal: true},
		{
			name:      "a person's own token",
			actor:     entity.Actor{Kind: entity.ActorKindToken, AccountID: owner, OwnerAccountID: owner},
			principal: owner,
			personal:  true,
		},
		{
			name:      "a personal agent",
			actor:     agentActing(owner, entity.AgentScopeMember, nil),
			principal: owner,
			personal:  true,
		},
		{
			name:  "a project agent",
			actor: agentActing(owner, entity.AgentScopeProject, &projectID),
		},
		{
			name:  "a workspace agent",
			actor: agentActing(owner, entity.AgentScopeWorkspace, nil),
		},
		{
			name: "an agent whose scope was never stamped",
			actor: entity.Actor{
				Kind:           entity.ActorKindAgent,
				AccountID:      uuid.New(),
				OwnerAccountID: owner,
			},
		},
		{
			name: "an integration someone connected",
			actor: entity.Actor{
				Kind:           entity.ActorKindToken,
				AccountID:      integration,
				OwnerAccountID: owner,
			},
			principal: integration,
		},
		{name: "nobody", actor: entity.Actor{}},
	}

	for _, tc := range cases {
		identity := entity.KnowledgeIdentityOf(tc.actor, entity.MembershipRoleAdmin)

		if identity.Personal != tc.personal || identity.Shared == tc.personal {
			t.Errorf("%s: personal = %v, shared = %v, want personal %v", tc.name, identity.Personal, identity.Shared, tc.personal)
		}

		if tc.personal && identity.Principal != tc.principal {
			t.Errorf("%s acts as %s, want %s", tc.name, identity.Principal, tc.principal)
		}

		if tc.principal != uuid.Nil && identity.Principal != tc.principal {
			t.Errorf("%s acts as %s, want %s", tc.name, identity.Principal, tc.principal)
		}

		if !tc.personal && (identity.Principal == owner || identity.Maintainer()) {
			t.Errorf(
				"%s borrowed its owner's standing over knowledge. Whoever drives it would read or "+
					"maintain what only the owner may.",
				tc.name,
			)
		}
	}
}

func TestWhoMayStartKnowledgeForAnAudience(t *testing.T) {
	workspaceID := uuid.New()
	projectID := uuid.New()
	elsewhere := uuid.New()

	reader := func(actor entity.Actor, role entity.MembershipRole) entity.KnowledgeReader {
		return entity.KnowledgeReader{
			WorkspaceID: workspaceID,
			Identity:    entity.KnowledgeIdentityOf(actor, role),
			ProjectIDs:  []uuid.UUID{projectID},
		}
	}

	member := reader(person(uuid.New()), entity.MembershipRoleMember)
	viewer := reader(person(uuid.New()), entity.MembershipRoleViewer)
	shared := reader(agentActing(uuid.New(), entity.AgentScopeProject, &projectID), entity.MembershipRoleMember)

	cases := []struct {
		name     string
		reader   entity.KnowledgeReader
		audience entity.KnowledgeAudience
		want     error
	}{
		{name: "a member, personally", reader: member, audience: entity.PersonalAudience()},
		{name: "a member, for their project", reader: member, audience: entity.ProjectAudience(projectID)},
		{name: "a member, for the workspace", reader: member, audience: entity.WorkspaceAudience()},
		{
			name:     "a member, for a project they are off",
			reader:   member,
			audience: entity.ProjectAudience(elsewhere),
			want:     entity.ErrKnowledgeNotShareable,
		},
		{name: "a viewer, personally", reader: viewer, audience: entity.PersonalAudience()},
		{
			name:     "a viewer, for their project",
			reader:   viewer,
			audience: entity.ProjectAudience(projectID),
			want:     entity.ErrKnowledgeNotShareable,
		},
		{
			name:     "a viewer, for the workspace",
			reader:   viewer,
			audience: entity.WorkspaceAudience(),
			want:     entity.ErrKnowledgeNotShareable,
		},
		{
			name:     "a shared agent, personally",
			reader:   shared,
			audience: entity.PersonalAudience(),
			want:     entity.ErrKnowledgeNotShareable,
		},
		{name: "a shared agent, for its project", reader: shared, audience: entity.ProjectAudience(projectID)},
	}

	for _, tc := range cases {
		if err := tc.reader.Author(tc.audience); !errors.Is(err, tc.want) || (tc.want == nil && err != nil) {
			t.Errorf("%s: Author = %v, want %v", tc.name, err, tc.want)
		}
	}

	var invalid entity.ValidationError

	for name, audience := range map[string]entity.KnowledgeAudience{
		"no visibility":           {},
		"a project with no id":    {Visibility: entity.KnowledgeVisibilityProject},
		"a workspace with an id":  {Visibility: entity.KnowledgeVisibilityWorkspace, ProjectID: projectID},
		"personal with a project": {Visibility: entity.KnowledgeVisibilityPersonal, ProjectID: projectID},
	} {
		if err := member.Author(audience); !errors.As(err, &invalid) {
			t.Errorf("%s: Author = %v, want a validation error", name, err)
		}
	}
}

func TestWideningAnAudienceIsADeliberateActByAnEditor(t *testing.T) {
	world := newKnowledgeWorld()
	project := []uuid.UUID{world.projectID}
	elsewhere := uuid.New()

	author := entity.KnowledgeReader{
		WorkspaceID: world.workspaceID,
		Identity:    entity.KnowledgeIdentityOf(person(world.author), entity.MembershipRoleMember),
		ProjectIDs:  project,
	}
	contributor := entity.KnowledgeReader{
		WorkspaceID: world.workspaceID,
		Identity:    entity.KnowledgeIdentityOf(person(world.contributor), entity.MembershipRoleMember),
		ProjectIDs:  project,
	}
	admin := entity.KnowledgeReader{
		WorkspaceID: world.workspaceID,
		Identity:    entity.KnowledgeIdentityOf(person(uuid.New()), entity.MembershipRoleAdmin),
		AllProjects: true,
	}
	viewerAuthor := entity.KnowledgeReader{
		WorkspaceID: world.workspaceID,
		Identity:    entity.KnowledgeIdentityOf(person(world.author), entity.MembershipRoleViewer),
		ProjectIDs:  project,
	}
	authorsAgent := entity.KnowledgeReader{
		WorkspaceID: world.workspaceID,
		Identity: entity.KnowledgeIdentityOf(
			agentActing(world.author, entity.AgentScopeProject, &world.projectID),
			entity.MembershipRoleMember,
		),
		ProjectIDs: project,
	}

	cases := []struct {
		name   string
		reader entity.KnowledgeReader
		from   string
		to     entity.KnowledgeAudience
		want   error
	}{
		{name: "the author opens a personal note to the workspace", reader: author, from: "personal", to: entity.WorkspaceAudience()},
		{name: "the author takes project knowledge back", reader: author, from: "project", to: entity.PersonalAudience()},
		{
			name:   "the author moves project knowledge to a project they are off",
			reader: author,
			from:   "project",
			to:     entity.ProjectAudience(elsewhere),
			want:   entity.ErrKnowledgeNotShareable,
		},
		{
			name:   "a contributor opens project knowledge to the workspace",
			reader: contributor,
			from:   "project",
			to:     entity.WorkspaceAudience(),
			want:   entity.ErrKnowledgeNotShareable,
		},
		{name: "an admin opens project knowledge to the workspace", reader: admin, from: "project", to: entity.WorkspaceAudience()},
		{name: "an admin narrows workspace knowledge to a project", reader: admin, from: "workspace", to: entity.ProjectAudience(world.projectID)},
		{
			name:   "an admin makes someone else's knowledge personal",
			reader: admin,
			from:   "workspace",
			to:     entity.PersonalAudience(),
			want:   entity.ErrKnowledgeNotShareable,
		},
		{
			name:   "an admin re-shares a personal note they cannot see",
			reader: admin,
			from:   "personal",
			to:     entity.WorkspaceAudience(),
			want:   entity.ErrKnowledgeNotFound,
		},
		{
			name:   "an author who is now a viewer widens their note",
			reader: viewerAuthor,
			from:   "personal",
			to:     entity.ProjectAudience(world.projectID),
			want:   entity.ErrKnowledgeNotShareable,
		},
		{name: "an author who is now a viewer takes knowledge back", reader: viewerAuthor, from: "project", to: entity.PersonalAudience()},
		{
			name:   "a shared agent of the author widens project knowledge",
			reader: authorsAgent,
			from:   "project",
			to:     entity.WorkspaceAudience(),
			want:   entity.ErrKnowledgeNotEditable,
		},
	}

	for _, tc := range cases {
		err := tc.reader.Reshare(world.items[tc.from], tc.to)
		if !errors.Is(err, tc.want) || (tc.want == nil && err != nil) {
			t.Errorf("%s: Reshare = %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestDerivedKnowledgeKeepsTheNarrowestAudienceOfItsSources(t *testing.T) {
	projectID := uuid.New()
	other := uuid.New()

	cases := []struct {
		name    string
		sources []entity.KnowledgeAudience
		want    entity.KnowledgeAudience
		err     error
	}{
		{
			name:    "workspace sources",
			sources: []entity.KnowledgeAudience{entity.WorkspaceAudience(), entity.WorkspaceAudience()},
			want:    entity.WorkspaceAudience(),
		},
		{
			name:    "a project source among workspace ones",
			sources: []entity.KnowledgeAudience{entity.WorkspaceAudience(), entity.ProjectAudience(projectID)},
			want:    entity.ProjectAudience(projectID),
		},
		{
			name: "one personal source",
			sources: []entity.KnowledgeAudience{
				entity.ProjectAudience(projectID), entity.PersonalAudience(), entity.WorkspaceAudience(),
			},
			want: entity.PersonalAudience(),
		},
		{
			name:    "two projects",
			sources: []entity.KnowledgeAudience{entity.ProjectAudience(projectID), entity.ProjectAudience(other)},
			err:     entity.ErrKnowledgeAudienceMixed,
		},
	}

	for _, tc := range cases {
		got, err := entity.DerivedAudience(tc.sources[0], tc.sources[1:]...)
		if !errors.Is(err, tc.err) || (tc.err == nil && err != nil) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.err)
			continue
		}

		if got != tc.want {
			t.Errorf("%s: derived audience = %+v, want %+v", tc.name, got, tc.want)
		}

		for _, source := range tc.sources {
			if tc.err == nil && !got.Within(source) {
				t.Errorf("%s: the derived audience %+v is wider than a source %+v", tc.name, got, source)
			}
		}
	}

	if entity.WorkspaceAudience().Within(entity.ProjectAudience(projectID)) {
		t.Error("a workspace summary of project knowledge was accepted as no wider than its source")
	}
}
