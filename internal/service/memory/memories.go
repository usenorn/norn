package memory

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type memoriesService struct {
	memories     repository.Memory
	revisions    repository.MemoryRevision
	projects     repository.Project
	issues       repository.Issue
	comments     repository.IssueComment
	docs         repository.Doc
	docRevisions repository.DocRevision
	knowledge    service.KnowledgeAuthorizer
	authorizer   service.Authorizer
	indexes      service.KnowledgeIndexes
	transactor   repository.Transactor
}

func New(
	memories repository.Memory,
	revisions repository.MemoryRevision,
	projects repository.Project,
	issues repository.Issue,
	comments repository.IssueComment,
	docs repository.Doc,
	docRevisions repository.DocRevision,
	knowledge service.KnowledgeAuthorizer,
	authorizer service.Authorizer,
	indexes service.KnowledgeIndexes,
	transactor repository.Transactor,
) service.Memories {
	return &memoriesService{
		memories:     memories,
		revisions:    revisions,
		projects:     projects,
		issues:       issues,
		comments:     comments,
		docs:         docs,
		docRevisions: docRevisions,
		knowledge:    knowledge,
		authorizer:   authorizer,
		indexes:      indexes,
		transactor:   transactor,
	}
}

type authorization func(ctx context.Context, memory entity.Memory) (entity.KnowledgeDecision, error)

type alteration func(ctx context.Context, memory *entity.Memory, decision entity.KnowledgeDecision) (bool, error)

func concealed(err error) error {
	if errors.Is(err, entity.ErrKnowledgeNotFound) {
		return entity.ErrMemoryNotFound
	}

	return err
}

func denied(err error) bool {
	var refusal entity.AccessDeniedError

	return errors.As(err, &refusal)
}

func unsupported(field string) error {
	return entity.NewValidationError(entity.FieldError{Field: field, Code: entity.ValidationCodeUnsupportedValue})
}

func (s *memoriesService) Create(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.CreateMemoryInput,
) (entity.Memory, error) {
	decision, draft, err := s.draft(ctx, workspaceID, input)
	if err != nil {
		return entity.Memory{}, err
	}

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		draft, err = s.insert(ctx, draft, decision)

		return err
	})
	if err != nil {
		return entity.Memory{}, err
	}

	return s.shown(ctx, decision.Reader, draft)
}

func (s *memoriesService) Supersede(
	ctx context.Context,
	workspaceID, memoryID uuid.UUID,
	input service.SupersedeMemoryInput,
) (entity.Memory, error) {
	decision, replacement, err := s.draft(ctx, workspaceID, input.Replacement)
	if err != nil {
		return entity.Memory{}, err
	}

	retires := replacement.Status == entity.MemoryStatusAccepted

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		replaced, err := s.memories.LockByID(ctx, workspaceID, memoryID)
		if err != nil {
			return err
		}

		if retires {
			_, err = s.knowledge.Change(ctx, replaced.Access())
		} else {
			_, err = s.knowledge.Read(ctx, replaced.Access())
		}

		if err != nil {
			return concealed(err)
		}

		if err := replaced.Expect(input.ExpectedVersion); err != nil {
			return err
		}

		if !replaced.Status.CanBecome(entity.MemoryStatusSuperseded) {
			return entity.MemoryTransitionError{From: replaced.Status, To: entity.MemoryStatusSuperseded}
		}

		replacement.SupersedesID = &replaced.ID

		if replacement, err = s.insert(ctx, replacement, decision); err != nil {
			return err
		}

		if !retires {
			return nil
		}

		if err := replaced.SupersededBy(replacement.ID); err != nil {
			return err
		}

		if _, err := s.save(ctx, replaced, input.ExpectedVersion, entity.MemoryRevisionSuperseded, decision); err != nil {
			return err
		}

		replacement, err = s.memories.GetByID(ctx, workspaceID, replacement.ID)

		return err
	})
	if err != nil {
		return entity.Memory{}, err
	}

	return s.shown(ctx, decision.Reader, replacement)
}

func (s *memoriesService) draft(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.CreateMemoryInput,
) (entity.KnowledgeDecision, entity.Memory, error) {
	decision, err := s.knowledge.Author(ctx, workspaceID, input.Audience)
	if err != nil {
		return entity.KnowledgeDecision{}, entity.Memory{}, err
	}

	if err := s.audienceProject(ctx, workspaceID, input.Audience); err != nil {
		return entity.KnowledgeDecision{}, entity.Memory{}, err
	}

	now := time.Now().UTC()
	actor := decision.Decision.Actor
	memory := entity.Memory{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		Audience:    input.Audience,
		Status:      entity.InitialMemoryStatus(input.Audience, actor.Kind),
		AuthorID:    decision.Reader.Identity.Principal,
		By:          entity.KnowledgeAttributionOf(decision),
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.write(ctx, decision, &memory, input.MemoryContent); err != nil {
		return entity.KnowledgeDecision{}, entity.Memory{}, err
	}

	return decision, memory, nil
}

func (s *memoriesService) write(
	ctx context.Context,
	decision entity.KnowledgeDecision,
	memory *entity.Memory,
	content service.MemoryContent,
) error {
	memory.Kind = content.Kind
	memory.Statement = strings.TrimSpace(content.Statement)
	memory.Rationale = strings.TrimSpace(content.Rationale)
	memory.Conditions = strings.TrimSpace(content.Conditions)
	memory.Applicability = entity.KnowledgeApplicability{
		ProjectID: content.Applicability.ProjectID,
		Component: strings.TrimSpace(content.Applicability.Component),
	}
	memory.Basis = content.Basis
	memory.EffectiveUntil = nil
	memory.Evidence = slices.Clone(content.Evidence)

	if memory.Basis == "" {
		memory.Basis = entity.MemoryBasisObservation
		if decision.Decision.Actor.Kind == entity.ActorKindAgent {
			memory.Basis = entity.MemoryBasisInference
		}
	}

	switch {
	case content.EffectiveFrom != nil:
		memory.EffectiveFrom = entity.MemoryDay(*content.EffectiveFrom)
	case memory.EffectiveFrom.IsZero():
		memory.EffectiveFrom = entity.MemoryDay(memory.CreatedAt)
	}

	if content.EffectiveUntil != nil {
		until := entity.MemoryDay(*content.EffectiveUntil)
		memory.EffectiveUntil = &until
	}

	for i := range memory.Evidence {
		memory.Evidence[i].Excerpt = strings.TrimSpace(memory.Evidence[i].Excerpt)
	}

	if err := memory.Validate(); err != nil {
		return err
	}

	if memory.Applicability.ProjectID != nil {
		_, err := s.projects.GetByID(ctx, memory.WorkspaceID, *memory.Applicability.ProjectID)
		if errors.Is(err, entity.ErrProjectNotFound) {
			return unsupported(entity.MemoryFieldAppliesTo)
		}

		if err != nil {
			return err
		}
	}

	return s.cite(ctx, decision.Reader, memory)
}

func (s *memoriesService) cite(ctx context.Context, reader entity.KnowledgeReader, memory *entity.Memory) error {
	var scope *entity.TeamScope

	for i := range memory.Evidence {
		evidence := &memory.Evidence[i]
		field := entity.MemoryEvidenceField(i)

		switch evidence.Kind {
		case entity.MemoryEvidenceIssue, entity.MemoryEvidenceComment:
			if scope == nil {
				decided, err := s.issueScope(ctx, memory.WorkspaceID)
				if err != nil {
					return err
				}

				scope = &decided
			}

			if err := s.citeIssue(ctx, *scope, evidence, field); err != nil {
				return err
			}
		case entity.MemoryEvidenceDocRevision:
			if err := s.citeDocRevision(ctx, reader, evidence, field); err != nil {
				return err
			}

			if !memory.Audience.Within(evidence.DocAudience) {
				return entity.ErrKnowledgeNotShareable
			}
		}
	}

	return nil
}

func (s *memoriesService) issueScope(ctx context.Context, workspaceID uuid.UUID) (entity.TeamScope, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if denied(err) {
		return entity.TeamScope{WorkspaceID: workspaceID}, nil
	}

	if err != nil {
		return entity.TeamScope{}, err
	}

	return decision.Scope, nil
}

func (s *memoriesService) citeIssue(
	ctx context.Context,
	scope entity.TeamScope,
	evidence *entity.MemoryEvidence,
	field string,
) error {
	if evidence.Kind == entity.MemoryEvidenceComment {
		comment, err := s.comments.GetByID(ctx, scope.WorkspaceID, *evidence.CommentID)
		if errors.Is(err, entity.ErrIssueCommentNotFound) || (err == nil && comment.DeletedAt != nil) {
			return unsupported(field)
		}

		if err != nil {
			return err
		}

		evidence.IssueID = &comment.IssueID
	}

	err := s.issues.VisibleExists(ctx, scope.WorkspaceID, *evidence.IssueID, scope)
	if errors.Is(err, entity.ErrIssueNotFound) {
		return unsupported(field)
	}

	return err
}

func (s *memoriesService) citeDocRevision(
	ctx context.Context,
	reader entity.KnowledgeReader,
	evidence *entity.MemoryEvidence,
	field string,
) error {
	doc, err := s.docs.GetByID(ctx, reader.WorkspaceID, *evidence.DocID)
	if errors.Is(err, entity.ErrDocNotFound) {
		return unsupported(field)
	}

	if err != nil {
		return err
	}

	if !reader.Sees(doc.Access()) {
		return unsupported(field)
	}

	_, err = s.docRevisions.GetByID(ctx, reader.WorkspaceID, doc.ID, *evidence.DocRevisionID)
	if errors.Is(err, entity.ErrDocRevisionNotFound) {
		return unsupported(field)
	}

	if err != nil {
		return err
	}

	evidence.DocAudience, evidence.DocAuthorID = doc.Audience, doc.AuthorID

	return nil
}

func (s *memoriesService) insert(
	ctx context.Context,
	memory entity.Memory,
	decision entity.KnowledgeDecision,
) (entity.Memory, error) {
	if err := s.memories.Create(ctx, memory); err != nil {
		return entity.Memory{}, err
	}

	return s.record(ctx, memory, entity.MemoryRevisionCreated, decision)
}

func (s *memoriesService) Get(ctx context.Context, workspaceID, memoryID uuid.UUID) (entity.Memory, error) {
	memory, err := s.memories.GetByID(ctx, workspaceID, memoryID)
	if err != nil {
		return entity.Memory{}, err
	}

	decision, err := s.knowledge.Read(ctx, memory.Access())
	if err != nil {
		return entity.Memory{}, concealed(err)
	}

	return s.shown(ctx, decision.Reader, memory)
}

func (s *memoriesService) List(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.ListMemoriesInput,
) (service.MemoryList, error) {
	if input.Kind != nil && !input.Kind.Valid() {
		return service.MemoryList{}, unsupported(entity.MemoryFieldKind)
	}

	for _, status := range input.Statuses {
		if !status.Valid() {
			return service.MemoryList{}, unsupported("status")
		}
	}

	decision, err := s.knowledge.Reader(ctx, workspaceID)
	if err != nil {
		return service.MemoryList{}, err
	}

	page := entity.MemoryPage{Limit: input.Limit}.Normalized()

	if input.Cursor != "" {
		cursor, err := entity.DecodeMemoryCursor(input.Cursor)
		if err != nil {
			return service.MemoryList{}, err
		}

		page.Cursor = &cursor
	}

	reader := decision.Reader
	var kinds []entity.MemoryKind
	if input.Kind != nil {
		kinds = []entity.MemoryKind{*input.Kind}
	}

	filter := repository.MemoryFilter{
		Kinds:            kinds,
		Statuses:         input.Statuses,
		AppliesProjectID: input.ProjectID,
		Component:        strings.TrimSpace(input.Component),
		AllProjects:      reader.AllProjects,
		ProjectIDs:       reader.ProjectIDs,
		PersonalAuthorID: personalAuthor(reader),
	}

	memories, err := s.memories.List(ctx, workspaceID, filter, page.Lookahead())
	if err != nil {
		return service.MemoryList{}, err
	}

	var list service.MemoryList

	if len(memories) > page.Limit {
		memories = memories[:page.Limit]
		list.NextCursor = memories[page.Limit-1].Cursor().Encode()
	}

	memories = slices.DeleteFunc(memories, func(memory entity.Memory) bool { return !reader.Sees(memory.Access()) })

	visible, err := s.visibleIssues(ctx, workspaceID, memories)
	if err != nil {
		return service.MemoryList{}, err
	}

	list.Memories = make([]entity.Memory, 0, len(memories))

	for _, memory := range memories {
		memory.Evidence = seenEvidence(reader, visible, memory.Evidence)
		list.Memories = append(list.Memories, memory.ViewedBy(reader))
	}

	return list, nil
}

func personalAuthor(reader entity.KnowledgeReader) uuid.UUID {
	if reader.Identity.Personal {
		return reader.Identity.Principal
	}

	return uuid.Nil
}

func (s *memoriesService) Correct(
	ctx context.Context,
	workspaceID, memoryID uuid.UUID,
	input service.CorrectMemoryInput,
) (entity.Memory, error) {
	return s.revise(ctx, workspaceID, memoryID, input.ExpectedVersion, entity.MemoryRevisionCorrected, s.change,
		func(ctx context.Context, memory *entity.Memory, decision entity.KnowledgeDecision) (bool, error) {
			corrected := *memory

			if err := s.write(ctx, decision, &corrected, input.MemoryContent); err != nil {
				return false, err
			}

			if corrected.Restates(*memory) {
				return false, nil
			}

			*memory = corrected

			return true, nil
		})
}

func (s *memoriesService) Withdraw(
	ctx context.Context,
	workspaceID, memoryID uuid.UUID,
	expectedVersion int,
) (entity.Memory, error) {
	return s.revise(ctx, workspaceID, memoryID, expectedVersion, entity.MemoryRevisionWithdrawn, s.change,
		func(_ context.Context, memory *entity.Memory, _ entity.KnowledgeDecision) (bool, error) {
			return true, memory.Become(entity.MemoryStatusWithdrawn)
		})
}

func (s *memoriesService) Reshare(
	ctx context.Context,
	workspaceID, memoryID uuid.UUID,
	input service.ReshareMemoryInput,
) (entity.Memory, error) {
	reshare := func(ctx context.Context, memory entity.Memory) (entity.KnowledgeDecision, error) {
		return s.knowledge.Reshare(ctx, memory.Access(), input.Audience)
	}

	return s.revise(ctx, workspaceID, memoryID, input.ExpectedVersion, entity.MemoryRevisionReshared, reshare,
		func(ctx context.Context, memory *entity.Memory, _ entity.KnowledgeDecision) (bool, error) {
			if memory.Audience == input.Audience {
				return false, nil
			}

			if err := s.audienceProject(ctx, workspaceID, input.Audience); err != nil {
				return false, err
			}

			for _, evidence := range memory.Evidence {
				if evidence.Kind == entity.MemoryEvidenceDocRevision && !input.Audience.Within(evidence.DocAudience) {
					return false, entity.ErrKnowledgeNotShareable
				}
			}

			memory.Audience = input.Audience

			return true, nil
		})
}

func (s *memoriesService) Revisions(
	ctx context.Context,
	workspaceID, memoryID uuid.UUID,
	input service.ListMemoryRevisionsInput,
) (service.MemoryHistory, error) {
	if _, err := s.Get(ctx, workspaceID, memoryID); err != nil {
		return service.MemoryHistory{}, err
	}

	page := entity.MemoryRevisionPage{Limit: input.Limit}.Normalized()

	if input.Cursor != "" {
		cursor, err := entity.DecodeMemoryRevisionCursor(input.Cursor)
		if err != nil {
			return service.MemoryHistory{}, err
		}

		page.Cursor = &cursor
	}

	revisions, err := s.revisions.List(ctx, workspaceID, memoryID, page.Lookahead())
	if err != nil {
		return service.MemoryHistory{}, err
	}

	history := service.MemoryHistory{Revisions: revisions}

	if len(revisions) > page.Limit {
		history.Revisions = revisions[:page.Limit]
		history.NextCursor = history.Revisions[page.Limit-1].Cursor().Encode()
	}

	return history, nil
}

func (s *memoriesService) Revision(
	ctx context.Context,
	workspaceID, memoryID, revisionID uuid.UUID,
) (entity.MemoryRevision, error) {
	memory, err := s.memories.GetByID(ctx, workspaceID, memoryID)
	if err != nil {
		return entity.MemoryRevision{}, err
	}

	decision, err := s.knowledge.Read(ctx, memory.Access())
	if err != nil {
		return entity.MemoryRevision{}, concealed(err)
	}

	revision, err := s.revisions.GetByID(ctx, workspaceID, memoryID, revisionID)
	if err != nil {
		return entity.MemoryRevision{}, err
	}

	visible, err := s.visibleIssues(ctx, workspaceID, []entity.Memory{{Evidence: revision.Evidence}})
	if err != nil {
		return entity.MemoryRevision{}, err
	}

	revision.Evidence = seenEvidence(decision.Reader, visible, revision.Evidence)

	return revision, nil
}

func (s *memoriesService) change(ctx context.Context, memory entity.Memory) (entity.KnowledgeDecision, error) {
	return s.knowledge.Change(ctx, memory.Access())
}

func (s *memoriesService) revise(
	ctx context.Context,
	workspaceID, memoryID uuid.UUID,
	expectedVersion int,
	action entity.MemoryRevisionAction,
	authorize authorization,
	apply alteration,
) (entity.Memory, error) {
	var (
		revised entity.Memory
		reader  entity.KnowledgeReader
	)

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		memory, err := s.memories.LockByID(ctx, workspaceID, memoryID)
		if err != nil {
			return err
		}

		decision, err := authorize(ctx, memory)
		if err != nil {
			return concealed(err)
		}

		reader = decision.Reader

		if err := memory.Expect(expectedVersion); err != nil {
			return err
		}

		changed, err := apply(ctx, &memory, decision)
		if err != nil {
			return err
		}

		if !changed {
			revised = memory

			return nil
		}

		revised, err = s.save(ctx, memory, expectedVersion, action, decision)

		return err
	})
	if err != nil {
		return entity.Memory{}, err
	}

	return s.shown(ctx, reader, revised)
}

func (s *memoriesService) save(
	ctx context.Context,
	memory entity.Memory,
	expectedVersion int,
	action entity.MemoryRevisionAction,
	decision entity.KnowledgeDecision,
) (entity.Memory, error) {
	memory.Version++
	memory.UpdatedAt = time.Now().UTC()

	if err := s.memories.Update(ctx, memory, expectedVersion); err != nil {
		return entity.Memory{}, err
	}

	return s.record(ctx, memory, action, decision)
}

func (s *memoriesService) record(
	ctx context.Context,
	memory entity.Memory,
	action entity.MemoryRevisionAction,
	decision entity.KnowledgeDecision,
) (entity.Memory, error) {
	if err := s.revisions.Record(ctx, memory.Revision(action, entity.KnowledgeAttributionOf(decision))); err != nil {
		return entity.Memory{}, err
	}

	s.indexes.Schedule(ctx, memory.WorkspaceID)

	return s.memories.GetByID(ctx, memory.WorkspaceID, memory.ID)
}

func (s *memoriesService) shown(
	ctx context.Context,
	reader entity.KnowledgeReader,
	memory entity.Memory,
) (entity.Memory, error) {
	visible, err := s.visibleIssues(ctx, memory.WorkspaceID, []entity.Memory{memory})
	if err != nil {
		return entity.Memory{}, err
	}

	memory.Evidence = seenEvidence(reader, visible, memory.Evidence)

	return memory.ViewedBy(reader), nil
}

func (s *memoriesService) visibleIssues(
	ctx context.Context,
	workspaceID uuid.UUID,
	memories []entity.Memory,
) ([]uuid.UUID, error) {
	var cited []uuid.UUID

	for _, memory := range memories {
		for _, evidence := range memory.Evidence {
			if evidence.IssueID != nil && !slices.Contains(cited, *evidence.IssueID) {
				cited = append(cited, *evidence.IssueID)
			}
		}
	}

	if len(cited) == 0 {
		return nil, nil
	}

	scope, err := s.issueScope(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	issues, err := s.issues.ListVisibleByIDs(ctx, scope, cited)
	if err != nil {
		return nil, err
	}

	visible := make([]uuid.UUID, 0, len(issues))
	for _, issue := range issues {
		visible = append(visible, issue.ID)
	}

	return visible, nil
}

func seenEvidence(
	reader entity.KnowledgeReader,
	visibleIssues []uuid.UUID,
	evidence []entity.MemoryEvidence,
) []entity.MemoryEvidence {
	return slices.DeleteFunc(slices.Clone(evidence), func(cited entity.MemoryEvidence) bool {
		return !cited.SeenBy(reader, visibleIssues)
	})
}

func (s *memoriesService) audienceProject(
	ctx context.Context,
	workspaceID uuid.UUID,
	audience entity.KnowledgeAudience,
) error {
	if audience.Visibility != entity.KnowledgeVisibilityProject {
		return nil
	}

	_, err := s.projects.GetByID(ctx, workspaceID, audience.ProjectID)
	if errors.Is(err, entity.ErrProjectNotFound) {
		return unsupported(entity.MemoryFieldProjectID)
	}

	return err
}
