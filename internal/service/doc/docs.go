package doc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type docsService struct {
	docs       repository.Doc
	revisions  repository.DocRevision
	projects   repository.Project
	knowledge  service.KnowledgeAuthorizer
	transactor repository.Transactor
}

func New(
	docs repository.Doc,
	revisions repository.DocRevision,
	projects repository.Project,
	knowledge service.KnowledgeAuthorizer,
	transactor repository.Transactor,
) service.Docs {
	return &docsService{
		docs:       docs,
		revisions:  revisions,
		projects:   projects,
		knowledge:  knowledge,
		transactor: transactor,
	}
}

type authorization func(ctx context.Context, doc entity.Doc) (entity.KnowledgeDecision, error)

type alteration func(ctx context.Context, doc *entity.Doc, decision entity.KnowledgeDecision) (bool, error)

func concealed(err error) error {
	if errors.Is(err, entity.ErrKnowledgeNotFound) {
		return entity.ErrDocNotFound
	}

	return err
}

func described(markdown string, content *entity.Document) (string, entity.Document, error) {
	markdown, document, err := entity.DescribedWithin(entity.DocLimits, markdown, content)
	if err != nil {
		return "", entity.Document{}, err
	}

	if err := entity.NewValidationError(entity.ValidateDocContent(entity.DocFieldContent, markdown)); err != nil {
		return "", entity.Document{}, err
	}

	return markdown, document, nil
}

func (s *docsService) Create(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.CreateDocInput,
) (entity.Doc, error) {
	if err := entity.NewValidationError(entity.ValidateDocTitle(entity.DocFieldTitle, input.Title)); err != nil {
		return entity.Doc{}, err
	}

	markdown, content, err := described(input.Markdown, input.Content)
	if err != nil {
		return entity.Doc{}, err
	}

	decision, err := s.knowledge.Author(ctx, workspaceID, input.Audience)
	if err != nil {
		return entity.Doc{}, err
	}

	if err := s.audienceProject(ctx, workspaceID, input.Audience); err != nil {
		return entity.Doc{}, err
	}

	now := time.Now().UTC()
	doc := entity.Doc{
		DocSummary: entity.DocSummary{
			ID:          uuid.New(),
			WorkspaceID: workspaceID,
			ParentID:    input.ParentID,
			Title:       strings.TrimSpace(input.Title),
			Audience:    input.Audience,
			AuthorID:    decision.Reader.Identity.Principal,
			Status:      entity.DocStatusActive,
			Version:     1,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		Content:  content,
		Markdown: markdown,
	}

	var created entity.Doc

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if input.ParentID != nil {
			if err := s.placeBeneath(ctx, decision.Reader, doc.ID, *input.ParentID, 0); err != nil {
				return err
			}
		}

		if err := s.docs.Create(ctx, doc); err != nil {
			return err
		}

		created, err = s.record(ctx, doc, entity.DocRevisionCreated, decision)

		return err
	})
	if err != nil {
		return entity.Doc{}, err
	}

	return created, nil
}

func (s *docsService) Get(ctx context.Context, workspaceID, docID uuid.UUID) (entity.Doc, error) {
	doc, err := s.docs.GetByID(ctx, workspaceID, docID)
	if err != nil {
		return entity.Doc{}, err
	}

	if _, err := s.knowledge.Read(ctx, doc.Access()); err != nil {
		return entity.Doc{}, concealed(err)
	}

	return doc, nil
}

func (s *docsService) List(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.ListDocsInput,
) (service.DocList, error) {
	decision, err := s.knowledge.Reader(ctx, workspaceID)
	if err != nil {
		return service.DocList{}, err
	}

	page := entity.DocPage{Limit: input.Limit}.Normalized()

	if input.Cursor != "" {
		cursor, err := entity.DecodeDocCursor(input.Cursor)
		if err != nil {
			return service.DocList{}, err
		}

		page.Cursor = &cursor
	}

	reader := decision.Reader
	filter := repository.DocFilter{
		ParentID:    input.ParentID,
		Root:        input.Root,
		AllProjects: reader.AllProjects,
		ProjectIDs:  reader.ProjectIDs,
	}

	if reader.Identity.Personal {
		filter.PersonalAuthorID = reader.Identity.Principal
	}

	summaries, err := s.docs.List(ctx, workspaceID, filter, page.Lookahead())
	if err != nil {
		return service.DocList{}, err
	}

	var list service.DocList

	if len(summaries) > page.Limit {
		summaries = summaries[:page.Limit]
		list.NextCursor = summaries[page.Limit-1].Cursor().Encode()
	}

	list.Docs = make([]entity.DocSummary, 0, len(summaries))

	for _, summary := range summaries {
		if reader.Sees(summary.Access()) {
			list.Docs = append(list.Docs, summary)
		}
	}

	return list, nil
}

func (s *docsService) Update(
	ctx context.Context,
	workspaceID, docID uuid.UUID,
	input service.UpdateDocInput,
) (entity.Doc, error) {
	if input.Title != nil {
		if err := entity.NewValidationError(entity.ValidateDocTitle(entity.DocFieldTitle, *input.Title)); err != nil {
			return entity.Doc{}, err
		}
	}

	var (
		markdown string
		content  entity.Document
		rewrites = input.Markdown != nil || input.Content != nil
	)

	if rewrites {
		var written string
		if input.Markdown != nil {
			written = *input.Markdown
		}

		var err error

		if markdown, content, err = described(written, input.Content); err != nil {
			return entity.Doc{}, err
		}
	}

	return s.revise(ctx, workspaceID, docID, input.ExpectedVersion, entity.DocRevisionEdited, s.change,
		func(_ context.Context, doc *entity.Doc, _ entity.KnowledgeDecision) (bool, error) {
			changed := false

			if input.Title != nil && strings.TrimSpace(*input.Title) != doc.Title {
				doc.Title = strings.TrimSpace(*input.Title)
				changed = true
			}

			if rewrites && markdown != doc.Markdown {
				doc.Content, doc.Markdown = content, markdown
				changed = true
			}

			return changed, nil
		})
}

func (s *docsService) Move(
	ctx context.Context,
	workspaceID, docID uuid.UUID,
	input service.MoveDocInput,
) (entity.Doc, error) {
	return s.revise(ctx, workspaceID, docID, input.ExpectedVersion, entity.DocRevisionMoved, s.change,
		func(ctx context.Context, doc *entity.Doc, decision entity.KnowledgeDecision) (bool, error) {
			if sameParent(doc.ParentID, input.ParentID) {
				return false, nil
			}

			if input.ParentID != nil {
				height, err := s.docs.SubtreeHeight(ctx, workspaceID, doc.ID)
				if err != nil {
					return false, err
				}

				if err := s.placeBeneath(ctx, decision.Reader, doc.ID, *input.ParentID, height); err != nil {
					return false, err
				}
			}

			doc.ParentID = input.ParentID

			return true, nil
		})
}

func (s *docsService) Reshare(
	ctx context.Context,
	workspaceID, docID uuid.UUID,
	input service.ReshareDocInput,
) (entity.Doc, error) {
	reshare := func(ctx context.Context, doc entity.Doc) (entity.KnowledgeDecision, error) {
		return s.knowledge.Reshare(ctx, doc.Access(), input.Audience)
	}

	return s.revise(ctx, workspaceID, docID, input.ExpectedVersion, entity.DocRevisionReshared, reshare,
		func(ctx context.Context, doc *entity.Doc, _ entity.KnowledgeDecision) (bool, error) {
			if doc.Audience == input.Audience {
				return false, nil
			}

			if err := s.audienceProject(ctx, workspaceID, input.Audience); err != nil {
				return false, err
			}

			doc.Audience = input.Audience

			return true, nil
		})
}

func (s *docsService) Archive(
	ctx context.Context,
	workspaceID, docID uuid.UUID,
	expectedVersion int,
) (entity.Doc, error) {
	return s.revise(ctx, workspaceID, docID, expectedVersion, entity.DocRevisionArchived, s.change,
		func(ctx context.Context, doc *entity.Doc, _ entity.KnowledgeDecision) (bool, error) {
			held, err := s.docs.HasActiveChildren(ctx, workspaceID, doc.ID)
			if err != nil {
				return false, err
			}

			if held {
				return false, entity.ErrDocHasChildren
			}

			archivedAt := time.Now().UTC()
			doc.Status, doc.ArchivedAt = entity.DocStatusArchived, &archivedAt

			return true, nil
		})
}

func (s *docsService) Revisions(
	ctx context.Context,
	workspaceID, docID uuid.UUID,
	input service.ListDocRevisionsInput,
) (service.DocHistory, error) {
	if _, err := s.Get(ctx, workspaceID, docID); err != nil {
		return service.DocHistory{}, err
	}

	page := entity.DocRevisionPage{Limit: input.Limit}.Normalized()

	if input.Cursor != "" {
		cursor, err := entity.DecodeDocRevisionCursor(input.Cursor)
		if err != nil {
			return service.DocHistory{}, err
		}

		page.Cursor = &cursor
	}

	revisions, err := s.revisions.List(ctx, workspaceID, docID, page.Lookahead())
	if err != nil {
		return service.DocHistory{}, err
	}

	history := service.DocHistory{Revisions: revisions}

	if len(revisions) > page.Limit {
		history.Revisions = revisions[:page.Limit]
		history.NextCursor = history.Revisions[page.Limit-1].Cursor().Encode()
	}

	return history, nil
}

func (s *docsService) Revision(
	ctx context.Context,
	workspaceID, docID, revisionID uuid.UUID,
) (entity.DocRevision, error) {
	if _, err := s.Get(ctx, workspaceID, docID); err != nil {
		return entity.DocRevision{}, err
	}

	return s.revisions.GetByID(ctx, workspaceID, docID, revisionID)
}

func (s *docsService) Restore(
	ctx context.Context,
	workspaceID, docID, revisionID uuid.UUID,
	expectedVersion int,
) (entity.Doc, error) {
	return s.revise(ctx, workspaceID, docID, expectedVersion, entity.DocRevisionRestored, s.change,
		func(ctx context.Context, doc *entity.Doc, _ entity.KnowledgeDecision) (bool, error) {
			restored, err := s.revisions.GetByID(ctx, workspaceID, docID, revisionID)
			if err != nil {
				return false, err
			}

			if restored.Title == doc.Title && restored.Markdown == doc.Markdown {
				return false, nil
			}

			doc.Title, doc.Content, doc.Markdown = restored.Title, restored.Content, restored.Markdown

			return true, nil
		})
}

func (s *docsService) change(ctx context.Context, doc entity.Doc) (entity.KnowledgeDecision, error) {
	return s.knowledge.Change(ctx, doc.Access())
}

func (s *docsService) revise(
	ctx context.Context,
	workspaceID, docID uuid.UUID,
	expectedVersion int,
	action entity.DocRevisionAction,
	authorize authorization,
	apply alteration,
) (entity.Doc, error) {
	var revised entity.Doc

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		doc, err := s.docs.LockByID(ctx, workspaceID, docID)
		if err != nil {
			return err
		}

		decision, err := authorize(ctx, doc)
		if err != nil {
			return concealed(err)
		}

		if err := doc.Expect(expectedVersion); err != nil {
			return err
		}

		changed, err := apply(ctx, &doc, decision)
		if err != nil {
			return err
		}

		if !changed {
			revised = doc

			return nil
		}

		doc.Version++
		doc.UpdatedAt = time.Now().UTC()

		if err := s.docs.Update(ctx, doc, expectedVersion); err != nil {
			return err
		}

		revised, err = s.record(ctx, doc, action, decision)

		return err
	})
	if err != nil {
		return entity.Doc{}, err
	}

	return revised, nil
}

func (s *docsService) record(
	ctx context.Context,
	doc entity.Doc,
	action entity.DocRevisionAction,
	decision entity.KnowledgeDecision,
) (entity.Doc, error) {
	if err := s.revisions.Record(ctx, doc.Revision(action, entity.DocAttributionOf(decision))); err != nil {
		return entity.Doc{}, err
	}

	return s.docs.GetByID(ctx, doc.WorkspaceID, doc.ID)
}

func (s *docsService) placeBeneath(
	ctx context.Context,
	reader entity.KnowledgeReader,
	docID, parentID uuid.UUID,
	subtreeHeight int,
) error {
	parent, err := s.docs.LockByID(ctx, reader.WorkspaceID, parentID)
	if errors.Is(err, entity.ErrDocNotFound) {
		return unknownParent()
	}

	if err != nil {
		return err
	}

	if err := reader.Read(parent.Access()); err != nil {
		return unknownParent()
	}

	if parent.Archived() {
		return entity.ErrDocArchived
	}

	ancestry, err := s.docs.Ancestry(ctx, reader.WorkspaceID, parentID)
	if err != nil {
		return err
	}

	return entity.ValidateDocPlacement(docID, ancestry, subtreeHeight)
}

func unknownParent() error {
	return entity.NewValidationError(entity.FieldError{
		Field: entity.DocFieldParentID,
		Code:  entity.ValidationCodeUnsupportedValue,
	})
}

func (s *docsService) audienceProject(
	ctx context.Context,
	workspaceID uuid.UUID,
	audience entity.KnowledgeAudience,
) error {
	if audience.Visibility != entity.KnowledgeVisibilityProject {
		return nil
	}

	_, err := s.projects.GetByID(ctx, workspaceID, audience.ProjectID)
	if errors.Is(err, entity.ErrProjectNotFound) {
		return entity.NewValidationError(entity.FieldError{
			Field: entity.DocFieldProjectID,
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	return err
}

func sameParent(current, next *uuid.UUID) bool {
	if current == nil || next == nil {
		return current == next
	}

	return *current == *next
}
