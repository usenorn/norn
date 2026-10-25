package memory

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
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
	search       repository.Knowledge
	notify       repository.NotificationEvent
	memberships  repository.Membership
	knowledge    service.KnowledgeAuthorizer
	authorizer   service.Authorizer
	indexes      service.KnowledgeIndexes
	vectors      service.KnowledgeEmbeddings
	transactor   repository.Transactor
	cfg          config.Knowledge
}

func New(
	memories repository.Memory,
	revisions repository.MemoryRevision,
	projects repository.Project,
	issues repository.Issue,
	comments repository.IssueComment,
	docs repository.Doc,
	docRevisions repository.DocRevision,
	search repository.Knowledge,
	notify repository.NotificationEvent,
	memberships repository.Membership,
	knowledge service.KnowledgeAuthorizer,
	authorizer service.Authorizer,
	indexes service.KnowledgeIndexes,
	vectors service.KnowledgeEmbeddings,
	transactor repository.Transactor,
	cfg config.Knowledge,
) service.Memories {
	return &memoriesService{
		memories:     memories,
		revisions:    revisions,
		projects:     projects,
		issues:       issues,
		comments:     comments,
		docs:         docs,
		docRevisions: docRevisions,
		search:       search,
		notify:       notify,
		memberships:  memberships,
		knowledge:    knowledge,
		authorizer:   authorizer,
		indexes:      indexes,
		vectors:      vectors,
		transactor:   transactor,
		cfg:          cfg,
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
) (service.MemoryWrite, error) {
	decision, draft, err := s.draft(ctx, workspaceID, input)
	if err != nil {
		return service.MemoryWrite{}, err
	}

	related, err := s.related(ctx, decision.Reader, draft)
	if err != nil {
		return service.MemoryWrite{}, err
	}

	draft.Status = draft.StatusAmong(related, draft.SettledBy(decision.Reader))

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if draft, err = s.insert(ctx, draft, decision); err != nil {
			return err
		}

		return s.propose(ctx, draft, decision)
	})
	if err != nil {
		return service.MemoryWrite{}, err
	}

	memory, err := s.shown(ctx, decision.Reader, draft)
	if err != nil {
		return service.MemoryWrite{}, err
	}

	return reconciling(memory, related), nil
}

func (s *memoriesService) Related(ctx context.Context, workspaceID, memoryID uuid.UUID) (service.MemoryWrite, error) {
	memory, err := s.memories.GetByID(ctx, workspaceID, memoryID)
	if err != nil {
		return service.MemoryWrite{}, err
	}

	decision, err := s.knowledge.Read(ctx, memory.Access())
	if err != nil {
		return service.MemoryWrite{}, concealed(err)
	}

	var related []entity.RelatedMemory

	if memory.Pending() {
		if related, err = s.related(ctx, decision.Reader, memory); err != nil {
			return service.MemoryWrite{}, err
		}
	}

	memory, err = s.shown(ctx, decision.Reader, memory)
	if err != nil {
		return service.MemoryWrite{}, err
	}

	return reconciling(memory, related), nil
}

func reconciling(memory entity.Memory, related []entity.RelatedMemory) service.MemoryWrite {
	write := service.MemoryWrite{Memory: memory, Related: related}

	if memory.Pending() && memory.Viewer.Edits {
		write.Choices = entity.MemoryChoices(related)
	}

	return write
}

func (s *memoriesService) related(
	ctx context.Context,
	reader entity.KnowledgeReader,
	draft entity.Memory,
) ([]entity.RelatedMemory, error) {
	lookup := repository.KnowledgeQuery{
		Query:            entity.ParseSearchQuery(draft.Statement),
		Statuses:         entity.MemoryRelatableStatuses(),
		On:               time.Now().UTC(),
		ProjectID:        draft.Applicability.ProjectID,
		Component:        draft.Applicability.Component,
		AllProjects:      reader.AllProjects,
		ProjectIDs:       reader.ProjectIDs,
		PersonalAuthorID: reader.PersonalAuthor(),
		Limit:            entity.MemoryRelatedLaneSize,
	}

	lexical, err := s.search.LexicalMemories(ctx, draft.WorkspaceID, lookup)
	if err != nil {
		return nil, err
	}

	lanes := []entity.KnowledgeLane{{Match: entity.KnowledgeMatchLexical, Candidates: lexical}}

	embedded, state, err := s.vectors.Embed(ctx, draft.WorkspaceID, draft.Statement)
	if err != nil {
		return nil, err
	}

	if state == entity.KnowledgeSemanticUsed {
		semantic, err := s.search.SemanticMemories(ctx, draft.WorkspaceID, lookup, repository.KnowledgeVector{
			Model:         embedded.Model,
			Vector:        embedded.Vector,
			MinSimilarity: s.cfg.RelatedMinSimilarity,
		})
		if err != nil {
			return nil, err
		}

		lanes = append(lanes, entity.KnowledgeLane{Match: entity.KnowledgeMatchSemantic, Candidates: semantic})
	}

	hits := slices.DeleteFunc(
		entity.FuseKnowledge(entity.KnowledgeLaneSize, lanes...),
		func(hit entity.KnowledgeHit) bool { return hit.ID == draft.ID },
	)
	if len(hits) == 0 {
		return nil, nil
	}

	ids := make([]uuid.UUID, 0, len(hits))
	for _, hit := range hits {
		ids = append(ids, hit.ID)
	}

	candidates, err := s.memories.ListByIDs(ctx, draft.WorkspaceID, ids)
	if err != nil {
		return nil, err
	}

	var related []entity.RelatedMemory

	for _, hit := range hits {
		at := slices.IndexFunc(candidates, func(candidate entity.Memory) bool { return candidate.ID == hit.ID })
		if at < 0 || !reader.Sees(candidates[at].Access()) {
			continue
		}

		candidate := candidates[at]

		relation, ok := draft.RelationTo(candidate, slices.Contains(hit.Matches, entity.KnowledgeMatchSemantic))
		if !ok {
			continue
		}

		candidate.Evidence = nil
		related = append(related, entity.RelatedMemory{Memory: candidate.ViewedBy(reader), Relation: relation})

		if len(related) == entity.MemoryRelatedMax {
			break
		}
	}

	return related, nil
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

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		replaced, err := s.memories.LockByID(ctx, workspaceID, memoryID)
		if err != nil {
			return err
		}

		if _, err := s.knowledge.Change(ctx, replaced.Access()); err != nil {
			return concealed(err)
		}

		if err := replaced.Expect(input.ExpectedVersion); err != nil {
			return err
		}

		if !replaced.Status.CanBecome(entity.MemoryStatusSuperseded) {
			return entity.MemoryTransitionError{From: replaced.Status, To: entity.MemoryStatusSuperseded}
		}

		replacement.SupersedesID = &replaced.ID

		if !replacement.SettledBy(decision.Reader) {
			replacement.Status = entity.MemoryStatusProposed

			if replacement, err = s.insert(ctx, replacement, decision); err != nil {
				return err
			}

			return s.propose(ctx, replacement, decision)
		}

		if replacement, err = s.insert(ctx, replacement, decision); err != nil {
			return err
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
	memory := entity.Memory{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		Audience:    input.Audience,
		Status:      entity.MemoryStatusAccepted,
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
	prior := memory.Evidence
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

	withheld, err := s.withheld(ctx, decision.Reader, memory.WorkspaceID, prior)
	if err != nil {
		return err
	}

	submitted := len(memory.Evidence)
	memory.Evidence = append(memory.Evidence, withheld...)

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

	return s.cite(ctx, decision.Reader, memory, memory.Evidence[:submitted], prior)
}

func (s *memoriesService) withheld(
	ctx context.Context,
	reader entity.KnowledgeReader,
	workspaceID uuid.UUID,
	prior []entity.MemoryEvidence,
) ([]entity.MemoryEvidence, error) {
	if len(prior) == 0 {
		return nil, nil
	}

	visible, err := s.visibleIssues(ctx, workspaceID, []entity.Memory{{Evidence: prior}})
	if err != nil {
		return nil, err
	}

	return slices.DeleteFunc(slices.Clone(prior), func(cited entity.MemoryEvidence) bool {
		return cited.State == entity.MemoryEvidenceUnavailable || cited.SeenBy(reader, visible)
	}), nil
}

func (s *memoriesService) cite(
	ctx context.Context,
	reader entity.KnowledgeReader,
	memory *entity.Memory,
	cited, prior []entity.MemoryEvidence,
) error {
	var scope *entity.TeamScope

	for i := range cited {
		evidence := &cited[i]
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

		for _, earlier := range prior {
			if earlier.Cites(*evidence) {
				evidence.IssueVersion, evidence.CommentVersion = earlier.IssueVersion, earlier.CommentVersion
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
		evidence.CommentVersion = &comment.Version
	}

	issue, err := s.issues.GetVisible(ctx, scope.WorkspaceID, *evidence.IssueID, scope)
	if errors.Is(err, entity.ErrIssueNotFound) {
		return unsupported(field)
	}

	if err != nil {
		return err
	}

	if evidence.Kind == entity.MemoryEvidenceIssue {
		evidence.IssueVersion = &issue.Version
	}

	return nil
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

	return s.record(ctx, memory.Revision(entity.MemoryRevisionCreated, entity.KnowledgeAttributionOf(decision)))
}

func (s *memoriesService) propose(ctx context.Context, memory entity.Memory, decision entity.KnowledgeDecision) error {
	if !memory.Pending() || memory.SettledBy(decision.Reader) {
		return nil
	}

	reviewers, err := s.reviewers(ctx, memory)
	if err != nil {
		return err
	}

	actor := decision.Decision.Actor

	for _, reviewer := range reviewers {
		if err := s.notify.Record(ctx, entity.NotificationEvent{
			WorkspaceID: memory.WorkspaceID,
			Subject:     entity.NotifyMemory(memory.ID),
			Kind:        entity.NotificationKindMemoryProposed,
			Actor:       actor.AccountID,
			ActorKind:   actor.Kind,
			Target:      reviewer,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *memoriesService) reviewers(ctx context.Context, memory entity.Memory) ([]uuid.UUID, error) {
	reviewers, err := s.memberships.ListActiveAdminAccountIDs(ctx, memory.WorkspaceID)
	if err != nil {
		return nil, err
	}

	if memory.Audience.Visibility != entity.KnowledgeVisibilityProject {
		return reviewers, nil
	}

	project, err := s.projects.GetByID(ctx, memory.WorkspaceID, memory.Audience.ProjectID)
	if err != nil {
		return nil, err
	}

	if project.LeadAccountID != uuid.Nil && !slices.Contains(reviewers, project.LeadAccountID) {
		reviewers = append(reviewers, project.LeadAccountID)
	}

	return reviewers, nil
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

func (s *memoriesService) Lineage(ctx context.Context, workspaceID, memoryID uuid.UUID) ([]entity.Memory, error) {
	lineage, err := s.memories.Lineage(ctx, workspaceID, memoryID, entity.MemoryLineageMax)
	if err != nil {
		return nil, err
	}

	at := slices.IndexFunc(lineage, func(memory entity.Memory) bool { return memory.ID == memoryID })
	if at < 0 {
		return nil, entity.ErrMemoryNotFound
	}

	decision, err := s.knowledge.Read(ctx, lineage[at].Access())
	if err != nil {
		return nil, concealed(err)
	}

	reader := decision.Reader
	lineage = slices.DeleteFunc(lineage, func(memory entity.Memory) bool { return !reader.Sees(memory.Access()) })

	for i := range lineage {
		lineage[i] = lineage[i].ViewedBy(reader)
	}

	return lineage, nil
}

func (s *memoriesService) List(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.ListMemoriesInput,
) (service.MemoryList, error) {
	for _, kind := range input.Kinds {
		if !kind.Valid() {
			return service.MemoryList{}, unsupported(entity.MemoryFieldKind)
		}
	}

	for _, status := range input.Statuses {
		if !status.Valid() {
			return service.MemoryList{}, unsupported("status")
		}
	}

	for _, visibility := range input.Visibilities {
		if !visibility.Valid() {
			return service.MemoryList{}, unsupported("visibility")
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

	filter := repository.MemoryFilter{
		Kinds:            input.Kinds,
		Statuses:         input.Statuses,
		AppliesProjectID: input.ProjectID,
		Component:        strings.TrimSpace(input.Component),
		AllProjects:      reader.AllProjects,
		ProjectIDs:       reader.ProjectIDs,
		PersonalAuthorID: reader.PersonalAuthor(),
		Visibilities:     input.Visibilities,
	}

	if input.Reviewable {
		review, reviews := reader.Reviewing()
		filter.Statuses = awaitingReview(input.Statuses)

		if !reviews || len(filter.Statuses) == 0 {
			return service.MemoryList{Memories: []entity.Memory{}}, nil
		}

		filter.ReviewedBy = &review
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

func awaitingReview(statuses []entity.MemoryStatus) []entity.MemoryStatus {
	pending := entity.MemoryPendingStatuses()
	if len(statuses) == 0 {
		return pending
	}

	return slices.DeleteFunc(slices.Clone(statuses), func(status entity.MemoryStatus) bool {
		return !slices.Contains(pending, status)
	})
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

func (s *memoriesService) Reconcile(
	ctx context.Context,
	workspaceID, memoryID uuid.UUID,
	input service.ReconcileMemoryInput,
) (service.MemoryReconciliation, error) {
	if err := reconcilable(memoryID, input); err != nil {
		return service.MemoryReconciliation{}, err
	}

	var (
		reconciled service.MemoryReconciliation
		reader     entity.KnowledgeReader
	)

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		proposal, target, err := s.lockPair(ctx, workspaceID, memoryID, input.Target)
		if err != nil {
			return err
		}

		decision, err := s.knowledge.Change(ctx, proposal.Access())
		if err != nil {
			return concealed(err)
		}

		reader = decision.Reader

		if err := proposal.Expect(input.ExpectedVersion); err != nil {
			return err
		}

		if !proposal.Pending() {
			return entity.ErrMemoryNotPending
		}

		var targetDecision entity.KnowledgeDecision

		if target != nil {
			if targetDecision, err = s.knowledge.Change(ctx, target.Access()); err != nil {
				return concealed(err)
			}

			if err := target.Expect(input.Target.ExpectedVersion); err != nil {
				return err
			}
		}

		action, err := s.resolve(ctx, input, &proposal, target, decision.Reader, targetDecision)
		if err != nil {
			return err
		}

		reconciled.Target = target
		reconciled.Memory, err = s.save(ctx, proposal, input.ExpectedVersion, action, decision)

		return err
	})
	if err != nil {
		return service.MemoryReconciliation{}, err
	}

	if reconciled.Memory, err = s.shown(ctx, reader, reconciled.Memory); err != nil {
		return service.MemoryReconciliation{}, err
	}

	if reconciled.Target != nil {
		target, err := s.shown(ctx, reader, *reconciled.Target)
		if err != nil {
			return service.MemoryReconciliation{}, err
		}

		reconciled.Target = &target
	}

	return reconciled, nil
}

func (s *memoriesService) resolve(
	ctx context.Context,
	input service.ReconcileMemoryInput,
	proposal, target *entity.Memory,
	reader entity.KnowledgeReader,
	decision entity.KnowledgeDecision,
) (entity.MemoryRevisionAction, error) {
	settled := proposal.SettledBy(reader)

	switch input.Resolution {
	case entity.MemoryResolutionUpdate:
		adopted, err := s.adopt(ctx, *target, *proposal, input.Target.ExpectedVersion, decision)
		if err != nil {
			return "", err
		}

		*target = adopted

		return entity.MemoryRevisionWithdrawn, proposal.Become(entity.MemoryStatusWithdrawn)
	case entity.MemoryResolutionSupersede:
		if !settled {
			if !target.Status.CanBecome(entity.MemoryStatusSuperseded) {
				return "", entity.MemoryTransitionError{From: target.Status, To: entity.MemoryStatusSuperseded}
			}

			proposal.SupersedesID = &target.ID

			return entity.MemoryRevisionReconciled, nil
		}

		if err := target.SupersededBy(proposal.ID); err != nil {
			return "", err
		}

		proposal.SupersedesID = &target.ID

		if err := proposal.Become(entity.MemoryStatusAccepted); err != nil {
			return "", err
		}

		retired, err := s.save(ctx, *target, input.Target.ExpectedVersion, entity.MemoryRevisionSuperseded, decision)
		if err != nil {
			return "", err
		}

		*target = retired

		return entity.MemoryRevisionAccepted, nil
	case entity.MemoryResolutionNoChange:
		return entity.MemoryRevisionWithdrawn, proposal.Become(entity.MemoryStatusWithdrawn)
	default:
		proposal.SupersedesID = nil

		if !settled {
			return entity.MemoryRevisionReconciled, nil
		}

		return entity.MemoryRevisionAccepted, proposal.Become(entity.MemoryStatusAccepted)
	}
}

func reconcilable(memoryID uuid.UUID, input service.ReconcileMemoryInput) error {
	switch {
	case !input.Resolution.Valid():
		return unsupported(entity.MemoryFieldResolution)
	case input.Resolution.Targeted() && input.Target == nil:
		return entity.NewValidationError(entity.FieldError{
			Field: entity.MemoryFieldTarget, Code: entity.ValidationCodeRequired,
		})
	case !input.Resolution.Targeted() && input.Target != nil, input.Target != nil && input.Target.ID == memoryID:
		return unsupported(entity.MemoryFieldTarget)
	default:
		return nil
	}
}

func (s *memoriesService) lockPair(
	ctx context.Context,
	workspaceID, memoryID uuid.UUID,
	target *service.MemoryTarget,
) (entity.Memory, *entity.Memory, error) {
	if target == nil {
		proposal, err := s.memories.LockByID(ctx, workspaceID, memoryID)

		return proposal, nil, err
	}

	ids := []uuid.UUID{memoryID, target.ID}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })

	locked := make(map[uuid.UUID]entity.Memory, len(ids))

	for _, id := range ids {
		memory, err := s.memories.LockByID(ctx, workspaceID, id)
		if err != nil {
			return entity.Memory{}, nil, err
		}

		locked[id] = memory
	}

	existing := locked[target.ID]

	return locked[memoryID], &existing, nil
}

func (s *memoriesService) adopt(
	ctx context.Context,
	target, proposal entity.Memory,
	expectedVersion int,
	decision entity.KnowledgeDecision,
) (entity.Memory, error) {
	corrected := target

	if err := s.write(ctx, decision, &corrected, contentOf(proposal)); err != nil {
		return entity.Memory{}, err
	}

	if corrected.Restates(target) {
		return target, nil
	}

	return s.save(ctx, corrected, expectedVersion, entity.MemoryRevisionCorrected, decision)
}

func contentOf(memory entity.Memory) service.MemoryContent {
	from := memory.EffectiveFrom

	return service.MemoryContent{
		Kind:           memory.Kind,
		Statement:      memory.Statement,
		Rationale:      memory.Rationale,
		Conditions:     memory.Conditions,
		Applicability:  memory.Applicability,
		Basis:          memory.Basis,
		Evidence:       memory.Evidence,
		EffectiveFrom:  &from,
		EffectiveUntil: memory.EffectiveUntil,
	}
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
		func(ctx context.Context, memory *entity.Memory, decision entity.KnowledgeDecision) (bool, error) {
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

			if memory.Status != entity.MemoryStatusAccepted || memory.SettledBy(decision.Reader) {
				return true, nil
			}

			if err := memory.Become(entity.MemoryStatusProposed); err != nil {
				return false, err
			}

			return true, s.propose(ctx, *memory, decision)
		})
}

func (s *memoriesService) Accept(
	ctx context.Context,
	workspaceID, memoryID uuid.UUID,
	input service.AcceptMemoryInput,
) (entity.Memory, error) {
	var (
		accepted entity.Memory
		reader   entity.KnowledgeReader
	)

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		memory, decision, err := s.review(ctx, workspaceID, memoryID, input.ExpectedVersion)
		if err != nil {
			return err
		}

		reader = decision.Reader
		version := input.ExpectedVersion

		if input.Content != nil {
			corrected := memory

			if err := s.write(ctx, decision, &corrected, *input.Content); err != nil {
				return err
			}

			if !corrected.Restates(memory) {
				if memory, err = s.save(ctx, corrected, version, entity.MemoryRevisionCorrected, decision); err != nil {
					return err
				}

				version = memory.Version
			}
		}

		if memory.SupersedesID != nil {
			if err := s.retire(ctx, workspaceID, *memory.SupersedesID, memory.ID); err != nil {
				return err
			}
		}

		if err := memory.Become(entity.MemoryStatusAccepted); err != nil {
			return err
		}

		accepted, err = s.save(ctx, memory, version, entity.MemoryRevisionAccepted, decision)

		return err
	})
	if err != nil {
		return entity.Memory{}, err
	}

	return s.shown(ctx, reader, accepted)
}

func (s *memoriesService) Reject(
	ctx context.Context,
	workspaceID, memoryID uuid.UUID,
	input service.RejectMemoryInput,
) (entity.Memory, error) {
	if err := entity.ValidateMemoryReason(input.Reason); err != nil {
		return entity.Memory{}, err
	}

	var (
		rejected entity.Memory
		reader   entity.KnowledgeReader
	)

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		memory, decision, err := s.review(ctx, workspaceID, memoryID, input.ExpectedVersion)
		if err != nil {
			return err
		}

		reader = decision.Reader

		if err := memory.Become(entity.MemoryStatusRejected); err != nil {
			return err
		}

		if err := s.bump(ctx, &memory, input.ExpectedVersion); err != nil {
			return err
		}

		rejected, err = s.record(ctx, memory.Rejection(input.Reason, entity.KnowledgeAttributionOf(decision)))

		return err
	})
	if err != nil {
		return entity.Memory{}, err
	}

	return s.shown(ctx, reader, rejected)
}

func (s *memoriesService) review(
	ctx context.Context,
	workspaceID, memoryID uuid.UUID,
	expectedVersion int,
) (entity.Memory, entity.KnowledgeDecision, error) {
	memory, err := s.memories.LockByID(ctx, workspaceID, memoryID)
	if err != nil {
		return entity.Memory{}, entity.KnowledgeDecision{}, err
	}

	decision, err := s.knowledge.Change(ctx, memory.Access())
	if err != nil {
		return entity.Memory{}, entity.KnowledgeDecision{}, concealed(err)
	}

	if err := memory.Expect(expectedVersion); err != nil {
		return entity.Memory{}, entity.KnowledgeDecision{}, err
	}

	if err := memory.Review(decision.Reader); err != nil {
		return entity.Memory{}, entity.KnowledgeDecision{}, err
	}

	return memory, decision, nil
}

func (s *memoriesService) retire(ctx context.Context, workspaceID, replacedID, replacementID uuid.UUID) error {
	replaced, err := s.memories.LockByID(ctx, workspaceID, replacedID)
	if err != nil {
		return err
	}

	decision, err := s.knowledge.Change(ctx, replaced.Access())
	if err != nil {
		return concealed(err)
	}

	version := replaced.Version

	if err := replaced.SupersededBy(replacementID); err != nil {
		return err
	}

	_, err = s.save(ctx, replaced, version, entity.MemoryRevisionSuperseded, decision)

	return err
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
	if err := s.bump(ctx, &memory, expectedVersion); err != nil {
		return entity.Memory{}, err
	}

	return s.record(ctx, memory.Revision(action, entity.KnowledgeAttributionOf(decision)))
}

func (s *memoriesService) bump(ctx context.Context, memory *entity.Memory, expectedVersion int) error {
	memory.Version++
	memory.UpdatedAt = time.Now().UTC()

	return s.memories.Update(ctx, *memory, expectedVersion)
}

func (s *memoriesService) record(ctx context.Context, revision entity.MemoryRevision) (entity.Memory, error) {
	if err := s.revisions.Record(ctx, revision); err != nil {
		return entity.Memory{}, err
	}

	s.indexes.Schedule(ctx, revision.WorkspaceID)

	return s.memories.GetByID(ctx, revision.WorkspaceID, revision.MemoryID)
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
	seen := make([]entity.MemoryEvidence, 0, len(evidence))

	for _, cited := range evidence {
		if cited.State == entity.MemoryEvidenceUnavailable || !cited.SeenBy(reader, visibleIssues) {
			cited = cited.Concealed()
		}

		seen = append(seen, cited)
	}

	return seen
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
