package memory

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func (s *memoriesService) ConfirmEvidence(
	ctx context.Context,
	workspaceID, memoryID uuid.UUID,
	expectedVersion int,
) (entity.Memory, error) {
	return s.revise(ctx, workspaceID, memoryID, expectedVersion, entity.MemoryRevisionConfirmed, s.change,
		func(ctx context.Context, memory *entity.Memory, decision entity.KnowledgeDecision) (bool, error) {
			visible, err := s.visibleIssues(ctx, workspaceID, []entity.Memory{*memory})
			if err != nil {
				return false, err
			}

			confirmed := false

			for i := range memory.Evidence {
				evidence := &memory.Evidence[i]
				if evidence.State != entity.MemoryEvidenceChanged || !evidence.SeenBy(decision.Reader, visible) {
					continue
				}

				if err := s.repin(ctx, decision.Reader, memory, evidence); err != nil {
					return false, err
				}

				confirmed = true
			}

			return confirmed, nil
		})
}

func (s *memoriesService) repin(
	ctx context.Context,
	reader entity.KnowledgeReader,
	memory *entity.Memory,
	evidence *entity.MemoryEvidence,
) error {
	switch evidence.Kind {
	case entity.MemoryEvidenceIssue:
		scope, err := s.issueScope(ctx, memory.WorkspaceID)
		if err != nil {
			return err
		}

		issue, err := s.issues.GetVisible(ctx, memory.WorkspaceID, *evidence.IssueID, scope)
		if err != nil {
			return err
		}

		evidence.IssueVersion = &issue.Version
	case entity.MemoryEvidenceComment:
		comment, err := s.comments.GetByID(ctx, memory.WorkspaceID, *evidence.CommentID)
		if err != nil {
			return err
		}

		evidence.CommentVersion = &comment.Version
	case entity.MemoryEvidenceDocRevision:
		doc, err := s.docs.GetByID(ctx, memory.WorkspaceID, *evidence.DocID)
		if err != nil {
			return err
		}

		latest, err := s.docRevisions.List(ctx, memory.WorkspaceID, doc.ID, entity.DocRevisionPage{Limit: 1})
		if err != nil {
			return err
		}

		if len(latest) == 0 {
			return entity.ErrDocRevisionNotFound
		}

		evidence.DocRevisionID = &latest[0].ID
		evidence.DocAudience, evidence.DocAuthorID = doc.Audience, doc.AuthorID

		return redacts(reader, *memory, *evidence, nil)
	}

	return nil
}
