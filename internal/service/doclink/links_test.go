package doclink_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	docrepo "github.com/usenorn/norn/internal/repository/doc"
	doclinkrepo "github.com/usenorn/norn/internal/repository/doclink"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	projectrepo "github.com/usenorn/norn/internal/repository/project"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	doclinksvc "github.com/usenorn/norn/internal/service/doclink"
	knowledgeauthorizersvc "github.com/usenorn/norn/internal/service/knowledgeauthorizer"
)

type harness struct {
	links      *doclinkrepo.MockDocLink
	issues     *issuerepo.MockIssue
	knowledge  *knowledgeauthorizersvc.MockKnowledgeAuthorizer
	authorizer *authorizersvc.MockAuthorizer
	service    service.DocLinks

	workspaceID uuid.UUID
	projectID   uuid.UUID
	issueID     uuid.UUID
	authorID    uuid.UUID
	reader      entity.KnowledgeReader
	actor       entity.Actor
	denied      map[entity.Resource]bool
	stored      map[uuid.UUID]entity.Doc
	listed      []repository.DocFilter
	created     []entity.StoredDocLink
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)
	docs := docrepo.NewMockDoc(ctrl)
	projects := projectrepo.NewMockProject(ctrl)

	h := &harness{
		links:       doclinkrepo.NewMockDocLink(ctrl),
		issues:      issuerepo.NewMockIssue(ctrl),
		knowledge:   knowledgeauthorizersvc.NewMockKnowledgeAuthorizer(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		workspaceID: uuid.New(),
		projectID:   uuid.New(),
		issueID:     uuid.New(),
		authorID:    uuid.New(),
		denied:      map[entity.Resource]bool{},
		stored:      map[uuid.UUID]entity.Doc{},
	}

	h.service = doclinksvc.New(docs, h.links, h.issues, projects, h.knowledge, h.authorizer)

	docs.EXPECT().GetByID(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, docID uuid.UUID) (entity.Doc, error) {
			doc, ok := h.stored[docID]
			if !ok {
				return entity.Doc{}, entity.ErrDocNotFound
			}

			return doc, nil
		}).
		AnyTimes()
	docs.EXPECT().List(gomock.Any(), h.workspaceID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, filter repository.DocFilter, _ entity.DocPage) ([]entity.DocSummary, error) {
			h.listed = append(h.listed, filter)

			summaries := make([]entity.DocSummary, 0, len(h.stored))
			for _, doc := range h.stored {
				summaries = append(summaries, doc.DocSummary)
			}

			return summaries, nil
		}).
		AnyTimes()

	h.issues.EXPECT().GetVisible(gomock.Any(), h.workspaceID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, issueID uuid.UUID, _ entity.TeamScope) (entity.Issue, error) {
			if issueID != h.issueID {
				return entity.Issue{}, entity.ErrIssueNotFound
			}

			return entity.Issue{ID: issueID, ReferenceKey: "ENG", Number: 4, Title: "Public issue"}, nil
		}).
		AnyTimes()
	projects.EXPECT().GetByID(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, projectID uuid.UUID) (entity.Project, error) {
			if projectID != h.projectID {
				return entity.Project{}, entity.ErrProjectNotFound
			}

			return entity.Project{ID: projectID, Slug: "handbook", Name: "Handbook"}, nil
		}).
		AnyTimes()
	h.links.EXPECT().Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, link entity.StoredDocLink) error {
			h.created = append(h.created, link)

			return nil
		}).
		AnyTimes()

	h.authorizer.EXPECT().Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			if h.denied[request.Resource] {
				return entity.Decision{}, entity.AccessDeniedError{Resource: request.Resource, Action: request.Action}
			}

			return entity.Decision{
				Actor: h.actor,
				Scope: entity.TeamScope{WorkspaceID: h.workspaceID, AllTeams: true},
			}, nil
		}).
		AnyTimes()

	decided := func(err error) (entity.KnowledgeDecision, error) {
		if err != nil {
			return entity.KnowledgeDecision{}, err
		}

		return entity.KnowledgeDecision{Decision: entity.Decision{Actor: h.actor}, Reader: h.reader}, nil
	}

	h.knowledge.EXPECT().Reader(gomock.Any(), h.workspaceID).
		DoAndReturn(func(context.Context, uuid.UUID) (entity.KnowledgeDecision, error) {
			if h.denied[entity.ResourceKnowledge] {
				return decided(entity.AccessDeniedError{Resource: entity.ResourceKnowledge, Action: entity.ActionRead})
			}

			return decided(nil)
		}).
		AnyTimes()
	h.knowledge.EXPECT().Read(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, access entity.KnowledgeAccess) (entity.KnowledgeDecision, error) {
			return decided(h.reader.Read(access))
		}).
		AnyTimes()
	h.knowledge.EXPECT().Change(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, access entity.KnowledgeAccess) (entity.KnowledgeDecision, error) {
			return decided(h.reader.Change(access))
		}).
		AnyTimes()

	h.actAs(h.authorID)

	return h
}

func (h *harness) actAs(accountID uuid.UUID, projectIDs ...uuid.UUID) {
	h.actor = entity.Actor{Kind: entity.ActorKindUser, AccountID: accountID}
	h.reader = entity.KnowledgeReader{
		WorkspaceID: h.workspaceID,
		Identity:    entity.KnowledgeIdentityOf(h.actor, entity.MembershipRoleMember),
		ProjectIDs:  projectIDs,
	}
}

func (h *harness) doc(title string, audience entity.KnowledgeAudience) entity.Doc {
	doc := entity.Doc{DocSummary: entity.DocSummary{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		Title:       title,
		Audience:    audience,
		AuthorID:    h.authorID,
		Status:      entity.DocStatusActive,
		Version:     1,
	}}
	h.stored[doc.ID] = doc

	return doc
}

func titles(summaries []entity.DocSummary) []string {
	listed := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		listed = append(listed, summary.Title)
	}

	slices.Sort(listed)

	return listed
}

func TestARestrictedDocLinkedFromAPublicIssueLeavesNoTraceForAnOutsider(t *testing.T) {
	outsider, member := uuid.New(), uuid.New()

	for name, testCase := range map[string]struct {
		accountID  uuid.UUID
		projectIDs func(h *harness) []uuid.UUID
		want       []string
	}{
		"an outsider to the project": {
			accountID:  outsider,
			projectIDs: func(*harness) []uuid.UUID { return nil },
			want:       []string{"Workspace guide"},
		},
		"a project member": {
			accountID:  member,
			projectIDs: func(h *harness) []uuid.UUID { return []uuid.UUID{h.projectID} },
			want:       []string{"Project plan", "Workspace guide"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.doc("Workspace guide", entity.WorkspaceAudience())
			h.doc("Project plan", entity.ProjectAudience(h.projectID))
			h.doc("Author's notes", entity.PersonalAudience())
			h.actAs(testCase.accountID, testCase.projectIDs(h)...)

			onIssue, err := h.service.ForIssue(context.Background(), h.workspaceID, h.issueID)
			if err != nil {
				t.Fatalf("docs on the issue: %v", err)
			}

			onProject, err := h.service.ForProject(context.Background(), h.workspaceID, h.projectID)
			if err != nil {
				t.Fatalf("docs on the project: %v", err)
			}

			for place, got := range map[string][]string{"issue": titles(onIssue), "project": titles(onProject)} {
				if !slices.Equal(got, testCase.want) {
					t.Fatalf("the %s showed %v, want %v", place, got, testCase.want)
				}
			}

			if len(h.listed) != 2 || h.listed[0].LinkedIssueID == nil || *h.listed[0].LinkedIssueID != h.issueID ||
				h.listed[1].LinkedProjectID == nil || *h.listed[1].LinkedProjectID != h.projectID {
				t.Fatalf("the listings did not narrow to the linked target: %+v", h.listed)
			}

			for _, filter := range h.listed {
				if filter.PersonalAuthorID != testCase.accountID || filter.AllProjects ||
					!slices.Equal(filter.ProjectIDs, testCase.projectIDs(h)) {
					t.Fatalf("the listing was not narrowed to the viewer: %+v", filter)
				}
			}
		})
	}
}

func TestTheAuthorSeesTheirPersonalDocOnTheIssue(t *testing.T) {
	h := newHarness(t)
	h.doc("Author's notes", entity.PersonalAudience())

	docs, err := h.service.ForIssue(context.Background(), h.workspaceID, h.issueID)
	if err != nil || !slices.Equal(titles(docs), []string{"Author's notes"}) {
		t.Fatalf("the author saw %v, %v", titles(docs), err)
	}
}

func TestAViewerShutOutOfKnowledgeSeesAnEmptyListRatherThanAnError(t *testing.T) {
	h := newHarness(t)
	h.doc("Workspace guide", entity.WorkspaceAudience())
	h.denied[entity.ResourceKnowledge] = true

	docs, err := h.service.ForIssue(context.Background(), h.workspaceID, h.issueID)
	if err != nil || docs == nil || len(docs) != 0 {
		t.Fatalf("got %v, %v; want an empty list", docs, err)
	}

	if len(h.listed) != 0 {
		t.Fatalf("docs were queried for a viewer who reads none: %+v", h.listed)
	}
}

func TestAnIssueOrProjectTheViewerCannotReachShowsNoDocs(t *testing.T) {
	h := newHarness(t)
	h.doc("Workspace guide", entity.WorkspaceAudience())

	if _, err := h.service.ForIssue(context.Background(), h.workspaceID, uuid.New()); !errors.Is(err, entity.ErrIssueNotFound) {
		t.Fatalf("an unseen issue returned %v", err)
	}

	h.denied[entity.ResourceProject] = true

	if _, err := h.service.ForProject(context.Background(), h.workspaceID, h.projectID); !errors.Is(err, entity.ErrProjectNotFound) {
		t.Fatalf("a project the viewer may not read returned %v", err)
	}

	if len(h.listed) != 0 {
		t.Fatalf("docs were listed past a refusal: %+v", h.listed)
	}
}

func TestOnlySomeoneWhoEditsTheDocMayLinkIt(t *testing.T) {
	h := newHarness(t)
	restricted := h.doc("Project plan", entity.ProjectAudience(h.projectID))
	shared := h.doc("Workspace guide", entity.WorkspaceAudience())
	archived := h.doc("Old plan", entity.WorkspaceAudience())
	archived.Status = entity.DocStatusArchived
	h.stored[archived.ID] = archived

	input := service.LinkDocInput{IssueID: h.issueID}

	h.actAs(uuid.New())

	if _, err := h.service.Link(context.Background(), h.workspaceID, restricted.ID, input); !errors.Is(err, entity.ErrDocNotFound) {
		t.Fatalf("linking a doc outside the audience returned %v", err)
	}

	if _, err := h.service.Link(context.Background(), h.workspaceID, shared.ID, input); !errors.Is(err, entity.ErrKnowledgeNotEditable) {
		t.Fatalf("linking a doc the viewer only reads returned %v", err)
	}

	if err := h.service.Unlink(context.Background(), h.workspaceID, shared.ID, uuid.New()); !errors.Is(err, entity.ErrKnowledgeNotEditable) {
		t.Fatalf("unlinking from a doc the viewer only reads returned %v", err)
	}

	h.actAs(h.authorID)

	if _, err := h.service.Link(context.Background(), h.workspaceID, archived.ID, input); !errors.Is(err, entity.ErrDocArchived) {
		t.Fatalf("linking an archived doc returned %v", err)
	}

	if len(h.created) != 0 {
		t.Fatalf("a refused link was stored: %+v", h.created)
	}
}

func TestLinkingRecordsTheTargetAndWhoLinkedIt(t *testing.T) {
	h := newHarness(t)
	doc := h.doc("Project plan", entity.ProjectAudience(h.projectID))
	h.actAs(h.authorID, h.projectID)

	link, err := h.service.Link(context.Background(), h.workspaceID, doc.ID, service.LinkDocInput{IssueID: h.issueID})
	if err != nil {
		t.Fatalf("link issue: %v", err)
	}

	if link.Target != entity.DocLinkTargetIssue || link.Issue.Reference() != "ENG-4" {
		t.Fatalf("the link came back as %+v", link)
	}

	if _, err := h.service.Link(context.Background(), h.workspaceID, doc.ID, service.LinkDocInput{ProjectID: h.projectID}); err != nil {
		t.Fatalf("link project: %v", err)
	}

	if len(h.created) != 2 ||
		h.created[0].IssueID != h.issueID || h.created[0].DocID != doc.ID || h.created[0].CreatedByAccountID != h.authorID ||
		h.created[1].ProjectID != h.projectID || h.created[1].IssueID != uuid.Nil {
		t.Fatalf("stored %+v", h.created)
	}

	if _, err := h.service.Link(context.Background(), h.workspaceID, doc.ID, service.LinkDocInput{IssueID: uuid.New()}); !errors.Is(err, entity.ErrIssueNotFound) {
		t.Fatalf("linking an issue the viewer cannot see returned %v", err)
	}

	var invalid entity.ValidationError
	if _, err := h.service.Link(context.Background(), h.workspaceID, doc.ID, service.LinkDocInput{}); !errors.As(err, &invalid) {
		t.Fatalf("a link with no target returned %v", err)
	}

	if len(h.created) != 2 {
		t.Fatalf("a refused link was stored: %+v", h.created)
	}
}

func TestADocListsOnlyTheLinksTheViewerMayRead(t *testing.T) {
	h := newHarness(t)
	doc := h.doc("Workspace guide", entity.WorkspaceAudience())
	h.denied[entity.ResourceProject] = true

	h.links.EXPECT().ListForDoc(gomock.Any(), h.workspaceID, doc.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, scope entity.TeamScope) ([]entity.DocLink, error) {
			if !scope.AllTeams {
				t.Fatalf("the issue scope was not passed through: %+v", scope)
			}

			return []entity.DocLink{
				{ID: uuid.New(), Target: entity.DocLinkTargetIssue},
				{ID: uuid.New(), Target: entity.DocLinkTargetProject},
			}, nil
		})

	links, err := h.service.List(context.Background(), h.workspaceID, doc.ID)
	if err != nil || len(links) != 1 || links[0].Target != entity.DocLinkTargetIssue {
		t.Fatalf("a viewer who may not read projects saw %+v, %v", links, err)
	}

	restricted := h.doc("Project plan", entity.ProjectAudience(h.projectID))
	h.actAs(uuid.New())

	if _, err := h.service.List(context.Background(), h.workspaceID, restricted.ID); !errors.Is(err, entity.ErrDocNotFound) {
		t.Fatalf("listing links of a doc outside the audience returned %v", err)
	}
}
