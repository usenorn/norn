package doc_test

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
	docrevisionrepo "github.com/usenorn/norn/internal/repository/docrevision"
	projectrepo "github.com/usenorn/norn/internal/repository/project"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	"github.com/usenorn/norn/internal/service"
	docsvc "github.com/usenorn/norn/internal/service/doc"
	knowledgeauthorizersvc "github.com/usenorn/norn/internal/service/knowledgeauthorizer"
)

type harness struct {
	docs       *docrepo.MockDoc
	revisions  *docrevisionrepo.MockDocRevision
	projects   *projectrepo.MockProject
	knowledge  *knowledgeauthorizersvc.MockKnowledgeAuthorizer
	transactor *transactorrepo.MockTransactor
	service    service.Docs

	workspaceID uuid.UUID
	projectID   uuid.UUID
	accountID   uuid.UUID
	actor       entity.Actor
	reader      entity.KnowledgeReader
	stored      map[uuid.UUID]entity.Doc
	recorded    []entity.DocRevision
	listed      []repository.DocFilter
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		docs:        docrepo.NewMockDoc(ctrl),
		revisions:   docrevisionrepo.NewMockDocRevision(ctrl),
		projects:    projectrepo.NewMockProject(ctrl),
		knowledge:   knowledgeauthorizersvc.NewMockKnowledgeAuthorizer(ctrl),
		transactor:  transactorrepo.NewMockTransactor(ctrl),
		workspaceID: uuid.New(),
		projectID:   uuid.New(),
		accountID:   uuid.New(),
		stored:      map[uuid.UUID]entity.Doc{},
	}

	h.service = docsvc.New(h.docs, h.revisions, h.projects, h.knowledge, h.transactor)

	h.transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).
		AnyTimes()

	h.projects.EXPECT().GetByID(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, projectID uuid.UUID) (entity.Project, error) {
			if projectID != h.projectID {
				return entity.Project{}, entity.ErrProjectNotFound
			}

			return entity.Project{ID: projectID, WorkspaceID: h.workspaceID}, nil
		}).
		AnyTimes()

	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: h.accountID}, entity.MembershipRoleMember)
	h.store()
	h.history()
	h.judge()

	return h
}

func (h *harness) actAs(actor entity.Actor, role entity.MembershipRole) {
	h.actor = actor
	h.reader = entity.KnowledgeReader{
		WorkspaceID: h.workspaceID,
		Identity:    entity.KnowledgeIdentityOf(actor, role),
		ProjectIDs:  []uuid.UUID{h.projectID},
	}
}

func (h *harness) decision() entity.KnowledgeDecision {
	return entity.KnowledgeDecision{
		Decision: entity.Decision{Actor: h.actor, Workspace: entity.Workspace{ID: h.workspaceID}},
		Reader:   h.reader,
	}
}

func (h *harness) judge() {
	decided := func(err error) (entity.KnowledgeDecision, error) {
		if err != nil {
			return entity.KnowledgeDecision{}, err
		}

		return h.decision(), nil
	}

	h.knowledge.EXPECT().Reader(gomock.Any(), h.workspaceID).
		DoAndReturn(func(context.Context, uuid.UUID) (entity.KnowledgeDecision, error) { return decided(nil) }).
		AnyTimes()
	h.knowledge.EXPECT().Read(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, access entity.KnowledgeAccess) (entity.KnowledgeDecision, error) {
			return decided(h.reader.Read(access))
		}).
		AnyTimes()
	h.knowledge.EXPECT().Author(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, audience entity.KnowledgeAudience) (entity.KnowledgeDecision, error) {
			return decided(h.reader.Author(audience))
		}).
		AnyTimes()
	h.knowledge.EXPECT().Change(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, access entity.KnowledgeAccess) (entity.KnowledgeDecision, error) {
			return decided(h.reader.Change(access))
		}).
		AnyTimes()
	h.knowledge.EXPECT().Reshare(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, access entity.KnowledgeAccess, to entity.KnowledgeAudience) (entity.KnowledgeDecision, error) {
			return decided(h.reader.Reshare(access, to))
		}).
		AnyTimes()
}

func (h *harness) find(docID uuid.UUID) (entity.Doc, error) {
	doc, ok := h.stored[docID]
	if !ok {
		return entity.Doc{}, entity.ErrDocNotFound
	}

	for _, revision := range h.recorded {
		if revision.DocID == docID && !slices.Contains(doc.ContributorIDs, revision.By.AccountID) {
			doc.ContributorIDs = append(doc.ContributorIDs, revision.By.AccountID)
		}
	}

	return doc, nil
}

func (h *harness) store() {
	read := func(_ context.Context, _, docID uuid.UUID) (entity.Doc, error) { return h.find(docID) }

	h.docs.EXPECT().GetByID(gomock.Any(), h.workspaceID, gomock.Any()).DoAndReturn(read).AnyTimes()
	h.docs.EXPECT().LockByID(gomock.Any(), h.workspaceID, gomock.Any()).DoAndReturn(read).AnyTimes()
	h.docs.EXPECT().Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, doc entity.Doc) error {
			h.stored[doc.ID] = doc

			return nil
		}).
		AnyTimes()
	h.docs.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, doc entity.Doc, expectedVersion int) error {
			if h.stored[doc.ID].Version != expectedVersion {
				return entity.ErrDocStale
			}

			h.stored[doc.ID] = doc

			return nil
		}).
		AnyTimes()
	h.docs.EXPECT().List(gomock.Any(), h.workspaceID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, filter repository.DocFilter, _ entity.DocPage) ([]entity.DocSummary, error) {
			h.listed = append(h.listed, filter)

			summaries := make([]entity.DocSummary, 0, len(h.stored))
			for _, doc := range h.stored {
				summaries = append(summaries, doc.DocSummary)
			}

			return summaries, nil
		}).
		AnyTimes()
	h.docs.EXPECT().Ancestry(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, docID uuid.UUID) ([]uuid.UUID, error) {
			var ancestry []uuid.UUID

			for current := &docID; current != nil && len(ancestry) <= entity.DocTreeMaxDepth; current = h.stored[*current].ParentID {
				ancestry = append(ancestry, *current)
			}

			return ancestry, nil
		}).
		AnyTimes()
	h.docs.EXPECT().SubtreeHeight(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, docID uuid.UUID) (int, error) { return h.height(docID), nil }).
		AnyTimes()
	h.docs.EXPECT().HasActiveChildren(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, docID uuid.UUID) (bool, error) {
			for _, doc := range h.stored {
				if doc.ParentID != nil && *doc.ParentID == docID && !doc.Archived() {
					return true, nil
				}
			}

			return false, nil
		}).
		AnyTimes()
}

func (h *harness) height(docID uuid.UUID) int {
	tallest := 0

	for _, doc := range h.stored {
		if doc.ParentID != nil && *doc.ParentID == docID {
			tallest = max(tallest, h.height(doc.ID)+1)
		}
	}

	return tallest
}

func (h *harness) history() {
	h.revisions.EXPECT().Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, revision entity.DocRevision) error {
			h.recorded = append(h.recorded, revision)

			return nil
		}).
		AnyTimes()
	h.revisions.EXPECT().GetByID(gomock.Any(), h.workspaceID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, docID, revisionID uuid.UUID) (entity.DocRevision, error) {
			for _, revision := range h.recorded {
				if revision.ID == revisionID && revision.DocID == docID {
					return revision, nil
				}
			}

			return entity.DocRevision{}, entity.ErrDocRevisionNotFound
		}).
		AnyTimes()
	h.revisions.EXPECT().List(gomock.Any(), h.workspaceID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, docID uuid.UUID, page entity.DocRevisionPage) ([]entity.DocRevisionSummary, error) {
			var summaries []entity.DocRevisionSummary

			for index := len(h.recorded) - 1; index >= 0 && len(summaries) < page.Limit; index-- {
				if h.recorded[index].DocID == docID {
					summaries = append(summaries, h.recorded[index].DocRevisionSummary)
				}
			}

			return summaries, nil
		}).
		AnyTimes()
}

func (h *harness) create(t *testing.T, title string, audience entity.KnowledgeAudience, parentID *uuid.UUID) entity.Doc {
	t.Helper()

	doc, err := h.service.Create(context.Background(), h.workspaceID, service.CreateDocInput{
		Title:    title,
		Markdown: title + " body",
		ParentID: parentID,
		Audience: audience,
	})
	if err != nil {
		t.Fatalf("create %q: %v", title, err)
	}

	return doc
}

func text(value string) *string {
	return &value
}

func TestCreatingADocRecordsItsFirstRevisionAttributedToWhoWroteIt(t *testing.T) {
	owner := uuid.New()
	connection := uuid.New()
	agent := entity.Agent{ID: uuid.New(), AccountID: uuid.New(), OwnerAccountID: owner, Scope: entity.AgentScopeMember}

	for name, testCase := range map[string]struct {
		actor      entity.Actor
		principal  uuid.UUID
		agentID    *uuid.UUID
		connection *uuid.UUID
		source     entity.RevisionSource
	}{
		"a person in the dashboard": {
			actor:     entity.Actor{Kind: entity.ActorKindUser, AccountID: owner},
			principal: owner,
			source:    entity.RevisionSourcePerson,
		},
		"a person through a connection": {
			actor: entity.Actor{
				Kind: entity.ActorKindToken, AccountID: owner, OwnerAccountID: owner, ConnectionID: &connection,
			},
			principal:  owner,
			connection: &connection,
			source:     entity.RevisionSourcePerson,
		},
		"an agent acting for its owner": {
			actor:     agent.Attribute(entity.Actor{Kind: entity.ActorKindAgent, AccountID: agent.AccountID}),
			principal: owner,
			agentID:   &agent.ID,
			source:    entity.RevisionSourceAgent,
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.actAs(testCase.actor, entity.MembershipRoleMember)

			doc := h.create(t, "Onboarding", entity.WorkspaceAudience(), nil)

			if doc.Version != 1 || doc.AuthorID != testCase.principal || doc.Markdown != "Onboarding body" {
				t.Fatalf("created doc %+v, want version 1 written by %s", doc.DocSummary, testCase.principal)
			}

			if len(h.recorded) != 1 {
				t.Fatalf("creating a doc recorded %d revisions, want 1", len(h.recorded))
			}

			first := h.recorded[0]
			if first.Action != entity.DocRevisionCreated || first.Version != 1 || first.By.AccountID != testCase.principal ||
				first.By.Source != testCase.source {
				t.Fatalf("first revision %+v", first.DocRevisionSummary)
			}

			if !sameID(first.By.AgentID, testCase.agentID) || !sameID(first.By.ConnectionID, testCase.connection) {
				t.Fatalf("first revision lost the agent or connection: %+v", first.By)
			}
		})
	}
}

func TestADocIsCreatedOnlyForAnAudienceTheAuthorMayReach(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: h.accountID}, entity.MembershipRoleViewer)

	_, err := h.service.Create(context.Background(), h.workspaceID, service.CreateDocInput{
		Title:    "Pricing",
		Audience: entity.WorkspaceAudience(),
	})
	if !errors.Is(err, entity.ErrKnowledgeNotShareable) {
		t.Fatalf("a viewer publishing to the workspace got %v, want ErrKnowledgeNotShareable", err)
	}

	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: h.accountID}, entity.MembershipRoleAdmin)
	h.reader.AllProjects = true

	_, err = h.service.Create(context.Background(), h.workspaceID, service.CreateDocInput{
		Title:    "Pricing",
		Audience: entity.ProjectAudience(uuid.New()),
	})

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) || invalid.Fields[0].Field != entity.DocFieldProjectID {
		t.Fatalf("a doc shared with a project that does not exist got %v", err)
	}

	if len(h.stored) != 0 || len(h.recorded) != 0 {
		t.Fatalf("a refused create still wrote %d docs and %d revisions", len(h.stored), len(h.recorded))
	}
}

func TestAStaleEditIsRefusedAndNothingIsWritten(t *testing.T) {
	h := newHarness(t)
	doc := h.create(t, "Runbook", entity.WorkspaceAudience(), nil)

	if _, err := h.service.Update(context.Background(), h.workspaceID, doc.ID, service.UpdateDocInput{
		ExpectedVersion: 1,
		Title:           text("Runbook v2"),
	}); err != nil {
		t.Fatalf("first edit: %v", err)
	}

	_, err := h.service.Update(context.Background(), h.workspaceID, doc.ID, service.UpdateDocInput{
		ExpectedVersion: 1,
		Markdown:        text("Somebody else's body"),
	})

	var stale entity.DocStaleError
	if !errors.As(err, &stale) || stale.Version != 2 {
		t.Fatalf("an edit against version 1 of a version-2 doc got %v, want a stale error naming 2", err)
	}

	if current := h.stored[doc.ID]; current.Markdown != "Runbook body" || current.Version != 2 {
		t.Fatalf("the stale edit was written anyway: %+v", current)
	}

	if len(h.recorded) != 2 {
		t.Fatalf("the stale edit left a revision behind: %d revisions", len(h.recorded))
	}
}

func TestEveryEditIsItsOwnRevision(t *testing.T) {
	h := newHarness(t)
	doc := h.create(t, "Runbook", entity.WorkspaceAudience(), nil)

	for version, body := range []string{"First pass.", "Second pass."} {
		updated, err := h.service.Update(context.Background(), h.workspaceID, doc.ID, service.UpdateDocInput{
			ExpectedVersion: version + 1,
			Markdown:        text(body),
		})
		if err != nil {
			t.Fatalf("edit %d: %v", version+1, err)
		}

		if updated.Version != version+2 || updated.Markdown != body {
			t.Fatalf("edit %d produced %+v", version+1, updated.DocSummary)
		}
	}

	if len(h.recorded) != 3 {
		t.Fatalf("two quick edits by the same person left %d revisions, want 3: nothing is coalesced", len(h.recorded))
	}

	if h.recorded[2].Action != entity.DocRevisionEdited || h.recorded[2].Markdown != "Second pass." || h.recorded[2].Version != 3 {
		t.Fatalf("the last revision does not record the last edit: %+v", h.recorded[2])
	}

	unchanged, err := h.service.Update(context.Background(), h.workspaceID, doc.ID, service.UpdateDocInput{
		ExpectedVersion: 3,
		Markdown:        text("Second pass."),
	})
	if err != nil || unchanged.Version != 3 || len(h.recorded) != 3 {
		t.Fatalf("an edit that changes nothing moved the doc to %d with %d revisions (%v)", unchanged.Version, len(h.recorded), err)
	}
}

func TestOnlySomebodyWhoMayChangeADocCanEditIt(t *testing.T) {
	h := newHarness(t)
	doc := h.create(t, "Runbook", entity.WorkspaceAudience(), nil)
	personal := h.create(t, "Notes", entity.PersonalAudience(), nil)

	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()}, entity.MembershipRoleMember)

	if _, err := h.service.Update(context.Background(), h.workspaceID, doc.ID, service.UpdateDocInput{
		ExpectedVersion: 1,
		Title:           text("Hijacked"),
	}); !errors.Is(err, entity.ErrKnowledgeNotEditable) {
		t.Fatalf("a member who neither wrote nor maintains the doc got %v", err)
	}

	if _, err := h.service.Update(context.Background(), h.workspaceID, personal.ID, service.UpdateDocInput{
		ExpectedVersion: 9,
		Title:           text("Hijacked"),
	}); !errors.Is(err, entity.ErrDocNotFound) {
		t.Fatalf("editing somebody's personal doc with a stale version got %v, want not found and no hint it exists", err)
	}

	if _, err := h.service.Get(context.Background(), h.workspaceID, personal.ID); !errors.Is(err, entity.ErrDocNotFound) {
		t.Fatalf("reading somebody's personal doc got %v, want not found", err)
	}
}

func TestADocMovesOnlyWhereTheTreeHoldsIt(t *testing.T) {
	h := newHarness(t)
	root := h.create(t, "Root", entity.WorkspaceAudience(), nil)
	child := h.create(t, "Child", entity.WorkspaceAudience(), &root.ID)

	if _, err := h.service.Move(context.Background(), h.workspaceID, root.ID, service.MoveDocInput{
		ExpectedVersion: 1,
		ParentID:        &child.ID,
	}); !errors.Is(err, entity.ErrDocParentCycle) {
		t.Fatalf("moving a doc beneath its own child got %v", err)
	}

	deepest := root
	for depth := 2; depth <= entity.DocTreeMaxDepth; depth++ {
		parentID := deepest.ID
		deepest = h.create(t, "Deep", entity.WorkspaceAudience(), &parentID)
	}

	parent := deepest.ID

	if _, err := h.service.Move(context.Background(), h.workspaceID, child.ID, service.MoveDocInput{
		ExpectedVersion: 1,
		ParentID:        &parent,
	}); !errors.Is(err, entity.ErrDocTreeTooDeep) {
		t.Fatalf("moving a doc past the deepest level got %v", err)
	}

	other := h.create(t, "Other", entity.WorkspaceAudience(), nil)

	moved, err := h.service.Move(context.Background(), h.workspaceID, child.ID, service.MoveDocInput{
		ExpectedVersion: 1,
		ParentID:        &other.ID,
	})
	if err != nil {
		t.Fatalf("move beneath a sibling root: %v", err)
	}

	if moved.ParentID == nil || *moved.ParentID != other.ID || moved.Version != 2 {
		t.Fatalf("moved doc %+v", moved.DocSummary)
	}

	if last := h.recorded[len(h.recorded)-1]; last.Action != entity.DocRevisionMoved || last.ParentID == nil || *last.ParentID != other.ID {
		t.Fatalf("the move left no revision: %+v", last.DocRevisionSummary)
	}

	toRoot, err := h.service.Move(context.Background(), h.workspaceID, child.ID, service.MoveDocInput{ExpectedVersion: 2})
	if err != nil || toRoot.ParentID != nil {
		t.Fatalf("moving the doc to the root got %+v (%v)", toRoot.DocSummary, err)
	}
}

func TestReSharingGoesThroughThePermissionModel(t *testing.T) {
	h := newHarness(t)
	doc := h.create(t, "Plan", entity.PersonalAudience(), nil)

	reshared, err := h.service.Reshare(context.Background(), h.workspaceID, doc.ID, service.ReshareDocInput{
		ExpectedVersion: 1,
		Audience:        entity.ProjectAudience(h.projectID),
	})
	if err != nil {
		t.Fatalf("the author sharing a personal doc with their project: %v", err)
	}

	if reshared.Audience != entity.ProjectAudience(h.projectID) || h.recorded[1].Action != entity.DocRevisionReshared {
		t.Fatalf("reshare produced %+v and revision %s", reshared.Audience, h.recorded[1].Action)
	}

	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()}, entity.MembershipRoleMember)

	if _, err := h.service.Reshare(context.Background(), h.workspaceID, doc.ID, service.ReshareDocInput{
		ExpectedVersion: 2,
		Audience:        entity.WorkspaceAudience(),
	}); !errors.Is(err, entity.ErrKnowledgeNotEditable) {
		t.Fatalf("somebody else widening the doc got %v", err)
	}

	if h.stored[doc.ID].Audience != entity.ProjectAudience(h.projectID) {
		t.Fatal("a refused reshare changed the audience")
	}
}

func TestArchivingKeepsHistoryAndWaitsForChildren(t *testing.T) {
	h := newHarness(t)
	parent := h.create(t, "Parent", entity.WorkspaceAudience(), nil)
	child := h.create(t, "Child", entity.WorkspaceAudience(), &parent.ID)

	if _, err := h.service.Archive(context.Background(), h.workspaceID, parent.ID, 1); !errors.Is(err, entity.ErrDocHasChildren) {
		t.Fatalf("archiving a doc with a live child got %v", err)
	}

	archived, err := h.service.Archive(context.Background(), h.workspaceID, child.ID, 1)
	if err != nil {
		t.Fatalf("archive child: %v", err)
	}

	if !archived.Archived() || archived.ArchivedAt == nil || archived.Version != 2 {
		t.Fatalf("archived doc %+v", archived.DocSummary)
	}

	if _, err := h.service.Archive(context.Background(), h.workspaceID, parent.ID, 1); err != nil {
		t.Fatalf("archiving the parent once its child is archived: %v", err)
	}

	history, err := h.service.Revisions(context.Background(), h.workspaceID, child.ID, service.ListDocRevisionsInput{})
	if err != nil {
		t.Fatalf("history of an archived doc: %v", err)
	}

	if len(history.Revisions) != 2 || history.Revisions[0].Action != entity.DocRevisionArchived ||
		history.Revisions[1].Action != entity.DocRevisionCreated {
		t.Fatalf("the archived doc's history is %+v", history.Revisions)
	}

	if _, err := h.service.Update(context.Background(), h.workspaceID, child.ID, service.UpdateDocInput{
		ExpectedVersion: 2,
		Title:           text("Revived"),
	}); !errors.Is(err, entity.ErrDocArchived) {
		t.Fatalf("editing an archived doc got %v", err)
	}

	if _, err := h.service.Create(context.Background(), h.workspaceID, service.CreateDocInput{
		Title:    "Orphan",
		ParentID: &parent.ID,
		Audience: entity.WorkspaceAudience(),
	}); !errors.Is(err, entity.ErrDocArchived) {
		t.Fatalf("creating a doc beneath an archived parent got %v", err)
	}
}

func TestRestoringBringsBackTitleAndContentButNotTheAudience(t *testing.T) {
	h := newHarness(t)
	doc := h.create(t, "Policy", entity.PersonalAudience(), nil)
	original := h.recorded[0]

	if _, err := h.service.Update(context.Background(), h.workspaceID, doc.ID, service.UpdateDocInput{
		ExpectedVersion: 1,
		Title:           text("Policy, rewritten"),
		Markdown:        text("Everything changed."),
	}); err != nil {
		t.Fatalf("edit: %v", err)
	}

	if _, err := h.service.Reshare(context.Background(), h.workspaceID, doc.ID, service.ReshareDocInput{
		ExpectedVersion: 2,
		Audience:        entity.WorkspaceAudience(),
	}); err != nil {
		t.Fatalf("reshare: %v", err)
	}

	restored, err := h.service.Restore(context.Background(), h.workspaceID, doc.ID, original.ID, 3)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	if restored.Title != "Policy" || restored.Markdown != "Policy body" || restored.Version != 4 {
		t.Fatalf("restore produced %+v / %q", restored.DocSummary, restored.Markdown)
	}

	if restored.Audience != entity.WorkspaceAudience() {
		t.Fatalf("restore narrowed the audience back to %+v", restored.Audience)
	}

	last := h.recorded[len(h.recorded)-1]
	if last.Action != entity.DocRevisionRestored || last.By.Source != entity.RevisionSourceRestore {
		t.Fatalf("the restore is recorded as %s from %s", last.Action, last.By.Source)
	}

	if _, err := h.service.Restore(context.Background(), h.workspaceID, doc.ID, original.ID, 3); !errors.Is(err, entity.ErrDocStale) {
		t.Fatalf("a restore against a stale version got %v", err)
	}

	other := h.create(t, "Other", entity.WorkspaceAudience(), nil)

	if _, err := h.service.Restore(context.Background(), h.workspaceID, other.ID, original.ID, 1); !errors.Is(err, entity.ErrDocRevisionNotFound) {
		t.Fatalf("restoring one doc from another's revision got %v", err)
	}

	if _, err := h.service.Revision(context.Background(), h.workspaceID, other.ID, original.ID); !errors.Is(err, entity.ErrDocRevisionNotFound) {
		t.Fatalf("reading one doc's revision through another got %v", err)
	}
}

func TestListingAsksThePermissionModelWhoIsReading(t *testing.T) {
	h := newHarness(t)
	h.create(t, "Shared", entity.WorkspaceAudience(), nil)
	h.create(t, "Mine", entity.PersonalAudience(), nil)

	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()}, entity.MembershipRoleMember)

	list, err := h.service.List(context.Background(), h.workspaceID, service.ListDocsInput{Root: true})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	filter := h.listed[len(h.listed)-1]
	if !filter.Root || filter.PersonalAuthorID != h.reader.Identity.Principal || !slices.Equal(filter.ProjectIDs, h.reader.ProjectIDs) {
		t.Fatalf("the listing did not carry the reader's reach: %+v", filter)
	}

	if len(list.Docs) != 1 || list.Docs[0].Title != "Shared" {
		t.Fatalf("somebody else's personal doc slipped through the listing: %+v", list.Docs)
	}

	if _, err := h.service.List(context.Background(), h.workspaceID, service.ListDocsInput{Cursor: "%%%"}); !errors.Is(err, entity.ErrDocCursorInvalid) {
		t.Fatalf("a garbled cursor got %v", err)
	}
}

func sameID(got, want *uuid.UUID) bool {
	if got == nil || want == nil {
		return got == want
	}

	return *got == *want
}
