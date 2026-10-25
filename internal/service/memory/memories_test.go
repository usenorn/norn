package memory_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	docrepo "github.com/usenorn/norn/internal/repository/doc"
	docrevisionrepo "github.com/usenorn/norn/internal/repository/docrevision"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	issuecommentrepo "github.com/usenorn/norn/internal/repository/issuecomment"
	memoryrepo "github.com/usenorn/norn/internal/repository/memory"
	memoryrevisionrepo "github.com/usenorn/norn/internal/repository/memoryrevision"
	projectrepo "github.com/usenorn/norn/internal/repository/project"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	knowledgeauthorizersvc "github.com/usenorn/norn/internal/service/knowledgeauthorizer"
	knowledgeindexsvc "github.com/usenorn/norn/internal/service/knowledgeindex"
	memorysvc "github.com/usenorn/norn/internal/service/memory"
)

type harness struct {
	memories   *memoryrepo.MockMemory
	revisions  *memoryrevisionrepo.MockMemoryRevision
	knowledge  *knowledgeauthorizersvc.MockKnowledgeAuthorizer
	transactor *transactorrepo.MockTransactor
	service    service.Memories

	workspaceID   uuid.UUID
	projectID     uuid.UUID
	accountID     uuid.UUID
	actor         entity.Actor
	reader        entity.KnowledgeReader
	stored        map[uuid.UUID]entity.Memory
	recorded      []entity.MemoryRevision
	listed        []repository.MemoryFilter
	visibleIssues []uuid.UUID
	comments      map[uuid.UUID]entity.IssueComment
	docs          map[uuid.UUID]entity.Doc
	docRevisions  map[uuid.UUID]uuid.UUID
	scheduled     []uuid.UUID
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		memories:     memoryrepo.NewMockMemory(ctrl),
		revisions:    memoryrevisionrepo.NewMockMemoryRevision(ctrl),
		knowledge:    knowledgeauthorizersvc.NewMockKnowledgeAuthorizer(ctrl),
		transactor:   transactorrepo.NewMockTransactor(ctrl),
		workspaceID:  uuid.New(),
		projectID:    uuid.New(),
		accountID:    uuid.New(),
		stored:       map[uuid.UUID]entity.Memory{},
		comments:     map[uuid.UUID]entity.IssueComment{},
		docs:         map[uuid.UUID]entity.Doc{},
		docRevisions: map[uuid.UUID]uuid.UUID{},
	}

	projects := projectrepo.NewMockProject(ctrl)
	issues := issuerepo.NewMockIssue(ctrl)
	comments := issuecommentrepo.NewMockIssueComment(ctrl)
	docs := docrepo.NewMockDoc(ctrl)
	docRevisions := docrevisionrepo.NewMockDocRevision(ctrl)
	authorizer := authorizersvc.NewMockAuthorizer(ctrl)
	indexes := knowledgeindexsvc.NewMockKnowledgeIndexes(ctrl)

	indexes.EXPECT().Schedule(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, workspaceID uuid.UUID) { h.scheduled = append(h.scheduled, workspaceID) }).
		AnyTimes()

	h.service = memorysvc.New(
		h.memories, h.revisions, projects, issues, comments, docs, docRevisions, h.knowledge, authorizer, indexes,
		h.transactor,
	)

	h.transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).
		AnyTimes()

	projects.EXPECT().GetByID(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, projectID uuid.UUID) (entity.Project, error) {
			if projectID != h.projectID {
				return entity.Project{}, entity.ErrProjectNotFound
			}

			return entity.Project{ID: projectID, WorkspaceID: h.workspaceID}, nil
		}).
		AnyTimes()

	authorizer.EXPECT().Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			return entity.Decision{Actor: h.actor, Scope: entity.TeamScope{WorkspaceID: request.WorkspaceID}}, nil
		}).
		AnyTimes()

	issues.EXPECT().VisibleExists(gomock.Any(), h.workspaceID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, issueID uuid.UUID, _ entity.TeamScope) error {
			if !slices.Contains(h.visibleIssues, issueID) {
				return entity.ErrIssueNotFound
			}

			return nil
		}).
		AnyTimes()
	issues.EXPECT().ListVisibleByIDs(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.TeamScope, issueIDs []uuid.UUID) ([]entity.Issue, error) {
			var visible []entity.Issue

			for _, issueID := range issueIDs {
				if slices.Contains(h.visibleIssues, issueID) {
					visible = append(visible, entity.Issue{ID: issueID})
				}
			}

			return visible, nil
		}).
		AnyTimes()

	comments.EXPECT().GetByID(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, commentID uuid.UUID) (entity.IssueComment, error) {
			comment, ok := h.comments[commentID]
			if !ok {
				return entity.IssueComment{}, entity.ErrIssueCommentNotFound
			}

			return comment, nil
		}).
		AnyTimes()

	docs.EXPECT().GetByID(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, docID uuid.UUID) (entity.Doc, error) {
			doc, ok := h.docs[docID]
			if !ok {
				return entity.Doc{}, entity.ErrDocNotFound
			}

			return doc, nil
		}).
		AnyTimes()
	docRevisions.EXPECT().GetByID(gomock.Any(), h.workspaceID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, docID, revisionID uuid.UUID) (entity.DocRevision, error) {
			if h.docRevisions[revisionID] != docID {
				return entity.DocRevision{}, entity.ErrDocRevisionNotFound
			}

			return entity.DocRevision{}, nil
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

func (h *harness) actAsAgent(scope entity.AgentScope) uuid.UUID {
	agentID, connectionID, tokenID := uuid.New(), uuid.New(), uuid.New()
	h.actAs(entity.Actor{
		Kind:           entity.ActorKindAgent,
		AccountID:      uuid.New(),
		TokenID:        &tokenID,
		AgentID:        &agentID,
		AgentScope:     scope,
		OwnerAccountID: h.accountID,
		ConnectionID:   &connectionID,
	}, entity.MembershipRoleMember)

	return agentID
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

func (h *harness) find(memoryID uuid.UUID) (entity.Memory, error) {
	memory, ok := h.stored[memoryID]
	if !ok {
		return entity.Memory{}, entity.ErrMemoryNotFound
	}

	memory.ContributorIDs = nil

	for _, revision := range h.recorded {
		if revision.MemoryID == memoryID && !slices.Contains(memory.ContributorIDs, revision.By.AccountID) {
			memory.ContributorIDs = append(memory.ContributorIDs, revision.By.AccountID)
		}
	}

	return memory, nil
}

func (h *harness) store() {
	read := func(_ context.Context, _, memoryID uuid.UUID) (entity.Memory, error) { return h.find(memoryID) }

	h.memories.EXPECT().GetByID(gomock.Any(), h.workspaceID, gomock.Any()).DoAndReturn(read).AnyTimes()
	h.memories.EXPECT().LockByID(gomock.Any(), h.workspaceID, gomock.Any()).DoAndReturn(read).AnyTimes()
	h.memories.EXPECT().Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, memory entity.Memory) error {
			h.stored[memory.ID] = memory

			return nil
		}).
		AnyTimes()
	h.memories.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, memory entity.Memory, expectedVersion int) error {
			if h.stored[memory.ID].Version != expectedVersion {
				return entity.ErrMemoryStale
			}

			h.stored[memory.ID] = memory

			return nil
		}).
		AnyTimes()
	h.memories.EXPECT().List(gomock.Any(), h.workspaceID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, filter repository.MemoryFilter, _ entity.MemoryPage) ([]entity.Memory, error) {
			h.listed = append(h.listed, filter)

			memories := make([]entity.Memory, 0, len(h.stored))
			for id := range h.stored {
				memory, _ := h.find(id)
				memories = append(memories, memory)
			}

			return memories, nil
		}).
		AnyTimes()
}

func (h *harness) history() {
	h.revisions.EXPECT().Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, revision entity.MemoryRevision) error {
			h.recorded = append(h.recorded, revision)

			return nil
		}).
		AnyTimes()
	h.revisions.EXPECT().GetByID(gomock.Any(), h.workspaceID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, memoryID, revisionID uuid.UUID) (entity.MemoryRevision, error) {
			for _, revision := range h.recorded {
				if revision.ID == revisionID && revision.MemoryID == memoryID {
					return revision, nil
				}
			}

			return entity.MemoryRevision{}, entity.ErrMemoryRevisionNotFound
		}).
		AnyTimes()
	h.revisions.EXPECT().List(gomock.Any(), h.workspaceID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, memoryID uuid.UUID, page entity.MemoryRevisionPage) ([]entity.MemoryRevisionSummary, error) {
			var summaries []entity.MemoryRevisionSummary

			for index := len(h.recorded) - 1; index >= 0 && len(summaries) < page.Limit; index-- {
				if h.recorded[index].MemoryID == memoryID {
					summaries = append(summaries, h.recorded[index].MemoryRevisionSummary)
				}
			}

			return summaries, nil
		}).
		AnyTimes()
}

func content(statement string) service.MemoryContent {
	return service.MemoryContent{
		Kind:       entity.MemoryKindConstraint,
		Statement:  statement,
		Rationale:  "the team runs it on laptops",
		Conditions: "local prototypes only",
	}
}

func (h *harness) create(t *testing.T, statement string, audience entity.KnowledgeAudience) entity.Memory {
	t.Helper()

	memory, err := h.service.Create(context.Background(), h.workspaceID, service.CreateMemoryInput{
		MemoryContent: content(statement),
		Audience:      audience,
	})
	if err != nil {
		t.Fatalf("create %q: %v", statement, err)
	}

	return memory
}

func (h *harness) revisionsOf(memoryID uuid.UUID) []entity.MemoryRevision {
	var revisions []entity.MemoryRevision

	for _, revision := range h.recorded {
		if revision.MemoryID == memoryID {
			revisions = append(revisions, revision)
		}
	}

	return revisions
}

func TestAPersonalMemoryIsAcceptedOnCreation(t *testing.T) {
	h := newHarness(t)

	memory := h.create(t, "I prefer tabs", entity.PersonalAudience())

	if memory.Status != entity.MemoryStatusAccepted || memory.AuthorID != h.accountID || memory.Version != 1 {
		t.Fatalf("got %s by %s at version %d, want accepted by %s at version 1",
			memory.Status, memory.AuthorID, memory.Version, h.accountID)
	}

	if memory.Basis != entity.MemoryBasisObservation || !memory.EffectiveFrom.Equal(entity.MemoryDay(time.Now())) {
		t.Fatalf("a person's memory defaulted to %s from %v", memory.Basis, memory.EffectiveFrom)
	}

	revisions := h.revisionsOf(memory.ID)
	if len(revisions) != 1 || revisions[0].Action != entity.MemoryRevisionCreated || revisions[0].By.Source != entity.RevisionSourcePerson {
		t.Fatalf("creation recorded %+v, want one created revision by a person", revisions)
	}
}

func TestAnAgentsSharedMemoryIsAcceptedAsItsOwnInference(t *testing.T) {
	for name, testCase := range map[string]struct {
		agent bool
		want  entity.MemoryStatus
		basis entity.MemoryBasis
	}{
		"a person":                 {agent: false, want: entity.MemoryStatusAccepted, basis: entity.MemoryBasisObservation},
		"an agent acting for them": {agent: true, want: entity.MemoryStatusAccepted, basis: entity.MemoryBasisInference},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)

			var agentID uuid.UUID
			if testCase.agent {
				agentID = h.actAsAgent(entity.AgentScopeMember)
			}

			memory := h.create(t, "Ship weekly", entity.WorkspaceAudience())

			if memory.Status != testCase.want || memory.Basis != testCase.basis {
				t.Fatalf("got %s on %s, want %s on %s", memory.Status, memory.Basis, testCase.want, testCase.basis)
			}

			if memory.AuthorID != h.accountID {
				t.Fatalf("the memory is authored by %s, want the person %s", memory.AuthorID, h.accountID)
			}

			if testCase.agent && (memory.By.AgentID == nil || *memory.By.AgentID != agentID ||
				memory.By.ConnectionID == nil || memory.By.TokenID == nil || *memory.By.TokenID != *h.actor.TokenID) {
				t.Fatalf("an agent's memory lost its provenance: %+v", memory.By)
			}
		})
	}
}

func TestAViewerCannotStartASharedMemory(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: h.accountID}, entity.MembershipRoleViewer)

	_, err := h.service.Create(context.Background(), h.workspaceID, service.CreateMemoryInput{
		MemoryContent: content("Ship weekly"),
		Audience:      entity.WorkspaceAudience(),
	})
	if !errors.Is(err, entity.ErrKnowledgeNotShareable) {
		t.Fatalf("a viewer's workspace memory got %v, want ErrKnowledgeNotShareable", err)
	}

	if len(h.stored) != 0 || len(h.recorded) != 0 {
		t.Fatalf("a refused memory left %d memories and %d revisions behind", len(h.stored), len(h.recorded))
	}
}

func TestEvidenceTheAuthorCannotSeeIsRefused(t *testing.T) {
	visible, hidden := uuid.New(), uuid.New()
	hiddenComment := uuid.New()
	strangersDoc, strangersRevision := uuid.New(), uuid.New()

	for name, evidence := range map[string]entity.MemoryEvidence{
		"an issue outside their teams":  {Kind: entity.MemoryEvidenceIssue, IssueID: &hidden},
		"a comment on such an issue":    {Kind: entity.MemoryEvidenceComment, CommentID: &hiddenComment},
		"an unknown comment":            {Kind: entity.MemoryEvidenceComment, CommentID: &visible},
		"someone else's personal doc":   {Kind: entity.MemoryEvidenceDocRevision, DocID: &strangersDoc, DocRevisionID: &strangersRevision},
		"a revision of a different doc": {Kind: entity.MemoryEvidenceDocRevision, DocID: &strangersDoc, DocRevisionID: &visible},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.visibleIssues = []uuid.UUID{visible}
			h.comments[hiddenComment] = entity.IssueComment{ID: hiddenComment, IssueID: hidden}
			h.docs[strangersDoc] = entity.Doc{DocSummary: entity.DocSummary{
				ID: strangersDoc, WorkspaceID: h.workspaceID, AuthorID: uuid.New(), Audience: entity.PersonalAudience(),
			}}
			h.docRevisions[strangersRevision] = strangersDoc

			input := content("Ship weekly")
			input.Evidence = []entity.MemoryEvidence{
				{Kind: entity.MemoryEvidenceIssue, IssueID: &visible},
				evidence,
			}

			_, err := h.service.Create(context.Background(), h.workspaceID, service.CreateMemoryInput{
				MemoryContent: input,
				Audience:      entity.WorkspaceAudience(),
			})

			var validation entity.ValidationError
			if !errors.As(err, &validation) || validation.Fields[0].Field != "evidence[1]" {
				t.Fatalf("got %v, want a validation error on evidence[1]", err)
			}

			if len(h.stored) != 0 {
				t.Fatal("a memory citing hidden evidence was stored")
			}
		})
	}
}

func TestACitedPersonalDocKeepsTheMemoryPersonal(t *testing.T) {
	h := newHarness(t)
	docID, revisionID := uuid.New(), uuid.New()
	h.docs[docID] = entity.Doc{DocSummary: entity.DocSummary{
		ID: docID, WorkspaceID: h.workspaceID, AuthorID: h.accountID, Audience: entity.PersonalAudience(),
	}}
	h.docRevisions[revisionID] = docID

	input := content("Use SQLite")
	input.Evidence = []entity.MemoryEvidence{{Kind: entity.MemoryEvidenceDocRevision, DocID: &docID, DocRevisionID: &revisionID}}

	_, err := h.service.Create(context.Background(), h.workspaceID, service.CreateMemoryInput{
		MemoryContent: input,
		Audience:      entity.WorkspaceAudience(),
	})
	if !errors.Is(err, entity.ErrKnowledgeNotShareable) {
		t.Fatalf("a workspace memory drawn from a personal doc got %v, want ErrKnowledgeNotShareable", err)
	}

	memory, err := h.service.Create(context.Background(), h.workspaceID, service.CreateMemoryInput{
		MemoryContent: input,
		Audience:      entity.PersonalAudience(),
	})
	if err != nil {
		t.Fatalf("a personal memory drawn from their own personal doc: %v", err)
	}

	if len(memory.Evidence) != 1 || memory.Evidence[0].DocAudience.Visibility != entity.KnowledgeVisibilityPersonal {
		t.Fatalf("the cited doc read back as %+v", memory.Evidence)
	}

	_, err = h.service.Reshare(context.Background(), h.workspaceID, memory.ID, service.ReshareMemoryInput{
		ExpectedVersion: 1,
		Audience:        entity.WorkspaceAudience(),
	})
	if !errors.Is(err, entity.ErrKnowledgeNotShareable) {
		t.Fatalf("resharing it with the workspace got %v, want ErrKnowledgeNotShareable", err)
	}
}

func TestACorrectionKeepsTheMemoryAndAppendsAnAttributedRevision(t *testing.T) {
	h := newHarness(t)
	memory := h.create(t, "Use SQLite", entity.WorkspaceAudience())
	agentID := h.actAsAgent(entity.AgentScopeMember)

	corrected := content("Use SQLite for local prototypes")
	corrected.Applicability = entity.KnowledgeApplicability{ProjectID: &h.projectID, Component: "storage"}

	got, err := h.service.Correct(context.Background(), h.workspaceID, memory.ID, service.CorrectMemoryInput{
		MemoryContent:   corrected,
		ExpectedVersion: 1,
	})
	if err != nil {
		t.Fatalf("correct: %v", err)
	}

	if got.ID != memory.ID || got.Version != 2 || got.Statement != "Use SQLite for local prototypes" {
		t.Fatalf("the correction read back as %s at version %d: %q", got.ID, got.Version, got.Statement)
	}

	if got.Applicability.Component != "storage" || got.Status != entity.MemoryStatusAccepted {
		t.Fatalf("the correction lost its applicability or status: %+v", got)
	}

	revisions := h.revisionsOf(memory.ID)
	if len(revisions) != 2 || revisions[0].Statement != "Use SQLite" {
		t.Fatalf("history is %+v, want the original and the correction", revisions)
	}

	latest := revisions[1]
	if latest.Action != entity.MemoryRevisionCorrected || latest.By.AgentID == nil || *latest.By.AgentID != agentID ||
		latest.By.ConnectionID == nil || latest.By.Source != entity.RevisionSourceAgent {
		t.Fatalf("the correction is attributed as %+v, want the agent and its connection", latest)
	}
}

func TestACorrectionThatChangesNothingRecordsNothing(t *testing.T) {
	h := newHarness(t)
	memory := h.create(t, "Use SQLite", entity.WorkspaceAudience())

	got, err := h.service.Correct(context.Background(), h.workspaceID, memory.ID, service.CorrectMemoryInput{
		MemoryContent:   content("  Use SQLite "),
		ExpectedVersion: 1,
	})
	if err != nil {
		t.Fatalf("correct: %v", err)
	}

	if got.Version != 1 || len(h.recorded) != 1 {
		t.Fatalf("a no-op correction moved the memory to version %d with %d revisions", got.Version, len(h.recorded))
	}
}

func TestAStaleCorrectionIsRefusedAndNothingIsWritten(t *testing.T) {
	h := newHarness(t)
	memory := h.create(t, "Use SQLite", entity.WorkspaceAudience())

	if _, err := h.service.Correct(context.Background(), h.workspaceID, memory.ID, service.CorrectMemoryInput{
		MemoryContent: content("Use Postgres"), ExpectedVersion: 1,
	}); err != nil {
		t.Fatalf("first correction: %v", err)
	}

	_, err := h.service.Correct(context.Background(), h.workspaceID, memory.ID, service.CorrectMemoryInput{
		MemoryContent: content("Use MySQL"), ExpectedVersion: 1,
	})

	var stale entity.MemoryStaleError
	if !errors.As(err, &stale) || stale.Version != 2 {
		t.Fatalf("a correction against version 1 got %v, want a stale error naming 2", err)
	}

	if h.stored[memory.ID].Statement != "Use Postgres" || len(h.recorded) != 2 {
		t.Fatalf("the stale correction wrote %q and %d revisions", h.stored[memory.ID].Statement, len(h.recorded))
	}
}

func TestOnlyAnEditorMayCorrectAMemory(t *testing.T) {
	h := newHarness(t)
	memory := h.create(t, "Use SQLite", entity.WorkspaceAudience())
	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()}, entity.MembershipRoleMember)

	_, err := h.service.Correct(context.Background(), h.workspaceID, memory.ID, service.CorrectMemoryInput{
		MemoryContent: content("Use MySQL"), ExpectedVersion: 1,
	})
	if !errors.Is(err, entity.ErrKnowledgeNotEditable) {
		t.Fatalf("a stranger's correction got %v, want ErrKnowledgeNotEditable", err)
	}
}

func TestAWithdrawnMemoryIsKeptButCannotBeChangedAgain(t *testing.T) {
	h := newHarness(t)
	memory := h.create(t, "Use SQLite", entity.WorkspaceAudience())

	withdrawn, err := h.service.Withdraw(context.Background(), h.workspaceID, memory.ID, 1)
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}

	if withdrawn.Status != entity.MemoryStatusWithdrawn || withdrawn.Version != 2 {
		t.Fatalf("withdrawal left %s at version %d", withdrawn.Status, withdrawn.Version)
	}

	if revisions := h.revisionsOf(memory.ID); revisions[len(revisions)-1].Action != entity.MemoryRevisionWithdrawn {
		t.Fatalf("withdrawal recorded %s", revisions[len(revisions)-1].Action)
	}

	if _, err := h.service.Withdraw(context.Background(), h.workspaceID, memory.ID, 2); !errors.Is(err, entity.ErrMemoryClosed) {
		t.Fatalf("withdrawing twice got %v, want ErrMemoryClosed", err)
	}

	if _, err := h.service.Correct(context.Background(), h.workspaceID, memory.ID, service.CorrectMemoryInput{
		MemoryContent: content("Use MySQL"), ExpectedVersion: 2,
	}); !errors.Is(err, entity.ErrMemoryClosed) {
		t.Fatalf("correcting a withdrawn memory got %v, want ErrMemoryClosed", err)
	}

	if _, err := h.service.Get(context.Background(), h.workspaceID, memory.ID); err != nil {
		t.Fatalf("a withdrawn memory can no longer be read: %v", err)
	}
}

func TestAnAcceptedReplacementSupersedesWithoutDeleting(t *testing.T) {
	h := newHarness(t)
	replaced := h.create(t, "Use SQLite", entity.WorkspaceAudience())

	replacement, err := h.service.Supersede(context.Background(), h.workspaceID, replaced.ID, service.SupersedeMemoryInput{
		Replacement:     service.CreateMemoryInput{MemoryContent: content("Use Postgres"), Audience: entity.WorkspaceAudience()},
		ExpectedVersion: 1,
	})
	if err != nil {
		t.Fatalf("supersede: %v", err)
	}

	if replacement.Status != entity.MemoryStatusAccepted || replacement.SupersedesID == nil || *replacement.SupersedesID != replaced.ID {
		t.Fatalf("the replacement read back as %s superseding %v", replacement.Status, replacement.SupersedesID)
	}

	old, err := h.service.Get(context.Background(), h.workspaceID, replaced.ID)
	if err != nil {
		t.Fatalf("the replaced memory can no longer be read: %v", err)
	}

	if old.Status != entity.MemoryStatusSuperseded || old.SupersededByID == nil || *old.SupersededByID != replacement.ID {
		t.Fatalf("the replaced memory is %s, replaced by %v", old.Status, old.SupersededByID)
	}

	revisions := h.revisionsOf(replaced.ID)
	if len(revisions) != 2 || revisions[1].Action != entity.MemoryRevisionSuperseded || revisions[1].Statement != "Use SQLite" {
		t.Fatalf("the replaced memory's history is %+v", revisions)
	}

	_, err = h.service.Supersede(context.Background(), h.workspaceID, replaced.ID, service.SupersedeMemoryInput{
		Replacement:     service.CreateMemoryInput{MemoryContent: content("Use MySQL"), Audience: entity.WorkspaceAudience()},
		ExpectedVersion: 2,
	})
	if !errors.Is(err, entity.ErrMemoryClosed) {
		t.Fatalf("superseding a superseded memory got %v, want ErrMemoryClosed", err)
	}
}

func TestAnAgentsReplacementRetiresTheMemoryItSupersedes(t *testing.T) {
	h := newHarness(t)
	replaced := h.create(t, "Use SQLite", entity.WorkspaceAudience())
	h.actAsAgent(entity.AgentScopeMember)

	replacement, err := h.service.Supersede(context.Background(), h.workspaceID, replaced.ID, service.SupersedeMemoryInput{
		Replacement:     service.CreateMemoryInput{MemoryContent: content("Use Postgres"), Audience: entity.WorkspaceAudience()},
		ExpectedVersion: 1,
	})
	if err != nil {
		t.Fatalf("supersede as an agent: %v", err)
	}

	if replacement.Status != entity.MemoryStatusAccepted || replacement.Basis != entity.MemoryBasisInference {
		t.Fatalf("the replacement read back as %s on %s", replacement.Status, replacement.Basis)
	}

	if old := h.stored[replaced.ID]; old.Status != entity.MemoryStatusSuperseded || old.SupersededByID == nil || *old.SupersededByID != replacement.ID {
		t.Fatalf("the replaced memory is %s, replaced by %v", old.Status, old.SupersededByID)
	}
}

func TestAProposalCannotBeSuperseded(t *testing.T) {
	h := newHarness(t)
	proposal := h.create(t, "Cache everything", entity.WorkspaceAudience())
	proposal.Status = entity.MemoryStatusProposed
	h.stored[proposal.ID] = proposal

	_, err := h.service.Supersede(context.Background(), h.workspaceID, proposal.ID, service.SupersedeMemoryInput{
		Replacement:     service.CreateMemoryInput{MemoryContent: content("Cache reads"), Audience: entity.WorkspaceAudience()},
		ExpectedVersion: 1,
	})
	if !errors.Is(err, entity.ErrMemoryTransitionInvalid) {
		t.Fatalf("superseding a proposal got %v, want a transition error", err)
	}

	if len(h.stored) != 1 {
		t.Fatalf("a refused supersession stored %d memories", len(h.stored))
	}
}

func TestHistoryListsEveryChangeNewestFirst(t *testing.T) {
	h := newHarness(t)
	memory := h.create(t, "Use SQLite", entity.WorkspaceAudience())

	if _, err := h.service.Correct(context.Background(), h.workspaceID, memory.ID, service.CorrectMemoryInput{
		MemoryContent: content("Use SQLite locally"), ExpectedVersion: 1,
	}); err != nil {
		t.Fatalf("correct: %v", err)
	}

	if _, err := h.service.Withdraw(context.Background(), h.workspaceID, memory.ID, 2); err != nil {
		t.Fatalf("withdraw: %v", err)
	}

	history, err := h.service.Revisions(context.Background(), h.workspaceID, memory.ID, service.ListMemoryRevisionsInput{Limit: 2})
	if err != nil {
		t.Fatalf("revisions: %v", err)
	}

	if len(history.Revisions) != 2 || history.Revisions[0].Action != entity.MemoryRevisionWithdrawn ||
		history.Revisions[1].Action != entity.MemoryRevisionCorrected || history.NextCursor == "" {
		t.Fatalf("the first page is %+v with cursor %q", history.Revisions, history.NextCursor)
	}

	first := h.revisionsOf(memory.ID)[0]

	revision, err := h.service.Revision(context.Background(), h.workspaceID, memory.ID, first.ID)
	if err != nil {
		t.Fatalf("revision: %v", err)
	}

	if revision.Statement != "Use SQLite" || revision.By.AccountID != h.accountID {
		t.Fatalf("the first revision read back as %q by %s", revision.Statement, revision.By.AccountID)
	}
}

func TestSomeoneElsesPersonalMemoryAndItsHistoryAreConcealed(t *testing.T) {
	h := newHarness(t)
	memory := h.create(t, "I prefer tabs", entity.PersonalAudience())
	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()}, entity.MembershipRoleAdmin)

	if _, err := h.service.Get(context.Background(), h.workspaceID, memory.ID); !errors.Is(err, entity.ErrMemoryNotFound) {
		t.Fatalf("reading it got %v, want ErrMemoryNotFound", err)
	}

	if _, err := h.service.Revisions(context.Background(), h.workspaceID, memory.ID, service.ListMemoryRevisionsInput{}); !errors.Is(err, entity.ErrMemoryNotFound) {
		t.Fatalf("its history got %v, want ErrMemoryNotFound", err)
	}

	if _, err := h.service.Revision(context.Background(), h.workspaceID, memory.ID, h.recorded[0].ID); !errors.Is(err, entity.ErrMemoryNotFound) {
		t.Fatalf("its first revision got %v, want ErrMemoryNotFound", err)
	}

	if _, err := h.service.Withdraw(context.Background(), h.workspaceID, memory.ID, 1); !errors.Is(err, entity.ErrMemoryNotFound) {
		t.Fatalf("withdrawing it got %v, want ErrMemoryNotFound", err)
	}

	list, err := h.service.List(context.Background(), h.workspaceID, service.ListMemoriesInput{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(list.Memories) != 0 {
		t.Fatalf("the list showed %d memories it should conceal", len(list.Memories))
	}
}

func TestTheListHidesEvidenceTheReaderCannotSee(t *testing.T) {
	h := newHarness(t)
	visible, hidden := uuid.New(), uuid.New()
	h.visibleIssues = []uuid.UUID{visible, hidden}

	input := content("Ship weekly")
	input.Evidence = []entity.MemoryEvidence{
		{Kind: entity.MemoryEvidenceIssue, IssueID: &visible},
		{Kind: entity.MemoryEvidenceIssue, IssueID: &hidden},
		{Kind: entity.MemoryEvidenceExcerpt, Excerpt: "  we shipped twice in March "},
	}

	if _, err := h.service.Create(context.Background(), h.workspaceID, service.CreateMemoryInput{
		MemoryContent: input,
		Audience:      entity.WorkspaceAudience(),
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()}, entity.MembershipRoleMember)
	h.visibleIssues = []uuid.UUID{visible}

	list, err := h.service.List(context.Background(), h.workspaceID, service.ListMemoriesInput{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(list.Memories) != 1 {
		t.Fatalf("listed %d memories, want 1", len(list.Memories))
	}

	evidence := list.Memories[0].Evidence
	if len(evidence) != 2 || *evidence[0].IssueID != visible || evidence[1].Excerpt != "we shipped twice in March" {
		t.Fatalf("the reader sees evidence %+v", evidence)
	}

	if filter := h.listed[0]; filter.PersonalAuthorID != h.reader.Identity.Principal || !slices.Equal(filter.ProjectIDs, []uuid.UUID{h.projectID}) {
		t.Fatalf("the list asked the store for %+v", filter)
	}
}

func TestEveryMemoryWriteSchedulesAKnowledgeRefresh(t *testing.T) {
	h := newHarness(t)

	memory := h.create(t, "Deploys go out on Tuesdays", entity.WorkspaceAudience())

	if _, err := h.service.Withdraw(context.Background(), h.workspaceID, memory.ID, memory.Version); err != nil {
		t.Fatalf("Withdraw: %v", err)
	}

	if !slices.Equal(h.scheduled, []uuid.UUID{h.workspaceID, h.workspaceID}) {
		t.Fatalf("scheduled refreshes for %v, want one for the creation and one for the withdrawal", h.scheduled)
	}
}
