package issue

import (
	"context"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

// remember keeps what a description said at this version. The issue row holds only the text as
// it stands now, so without this an edit erases what it replaced and nobody can see what a
// requirement was when the work started.
func (s *issuesService) remember(
	ctx context.Context,
	issue entity.Issue,
	decision entity.Decision,
	source entity.TriageSource,
	origin *entity.ImportOrigin,
) error {
	return s.revisions.Record(ctx, entity.IssueDescriptionRevision{
		WorkspaceID:     issue.WorkspaceID,
		IssueID:         issue.ID,
		IssueVersion:    issue.Version,
		Doc:             issue.DescriptionDoc,
		Markdown:        issue.Description,
		AuthorAccountID: decision.Actor.AccountID,
		Source:          entity.RevisionSourceOf(decision.Actor.Kind, source, origin),
		CreatedAt:       time.Now().UTC(),
	})
}
