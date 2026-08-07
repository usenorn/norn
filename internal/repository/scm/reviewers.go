package scm

import (
	"context"
	"fmt"

	"github.com/aarondl/sqlboiler/v4/types"
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

const deleteReviewersQuery = `
DELETE FROM workspace_code_link_reviewers
WHERE link_id = $1`

const insertReviewerQuery = `
INSERT INTO workspace_code_link_reviewers (link_id, login, verdict, url, reviewed_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (link_id, login) DO UPDATE
SET verdict = EXCLUDED.verdict,
    url = EXCLUDED.url,
    reviewed_at = EXCLUDED.reviewed_at,
    updated_at = now()`

// ReplaceReviewers writes the whole set rather than merging into it. A forge reports the
// reviews a change currently has, and somebody removed from the reviewers has to disappear
// here too, which a merge could never do.
func (r *linkRepository) ReplaceReviewers(
	ctx context.Context,
	linkID uuid.UUID,
	reviewers entity.CodeReviewers,
) error {
	querier := r.db.Querier(ctx)

	if _, err := querier.ExecContext(ctx, deleteReviewersQuery, linkID); err != nil {
		return fmt.Errorf("clear the reviewers of a change: %w", err)
	}

	for _, reviewer := range reviewers {
		if _, err := querier.ExecContext(
			ctx,
			insertReviewerQuery,
			linkID,
			reviewer.Login,
			reviewer.Verdict,
			reviewer.URL,
			reviewer.ReviewedAt,
		); err != nil {
			return fmt.Errorf("record the reviewer of a change: %w", err)
		}
	}

	return nil
}

const listReviewersQuery = `
SELECT link_id, login, verdict, url, reviewed_at, updated_at
FROM workspace_code_link_reviewers
WHERE link_id = ANY($1::uuid[])
ORDER BY link_id, login`

func (r *linkRepository) ListReviewers(
	ctx context.Context,
	linkIDs []uuid.UUID,
) (map[uuid.UUID]entity.CodeReviewers, error) {
	found := make(map[uuid.UUID]entity.CodeReviewers, len(linkIDs))

	if len(linkIDs) == 0 {
		return found, nil
	}

	rows, err := r.db.Querier(ctx).QueryContext(ctx, listReviewersQuery, uuidArray(linkIDs))
	if err != nil {
		return nil, fmt.Errorf("list the reviewers of a change: %w", err)
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var reviewer entity.CodeReviewer

		if err := rows.Scan(
			&reviewer.LinkID,
			&reviewer.Login,
			&reviewer.Verdict,
			&reviewer.URL,
			&reviewer.ReviewedAt,
			&reviewer.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("read the reviewer of a change: %w", err)
		}

		found[reviewer.LinkID] = append(found[reviewer.LinkID], reviewer)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list the reviewers of a change: %w", err)
	}

	return found, nil
}

func uuidArray(values []uuid.UUID) types.StringArray {
	text := make(types.StringArray, len(values))
	for i, value := range values {
		text[i] = value.String()
	}

	return text
}
