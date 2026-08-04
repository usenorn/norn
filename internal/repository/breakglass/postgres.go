package breakglass

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const discardQuery = `DELETE FROM workspace_break_glass_codes WHERE workspace_id = $1`

const issueQuery = `
INSERT INTO workspace_break_glass_codes (workspace_id, code_hash, issued_by)
VALUES ($1, $2, $3)`

const redeemQuery = `
UPDATE workspace_break_glass_codes
SET redeemed_at = now(), redeemed_from = $3
WHERE workspace_id = $1 AND code_hash = $2 AND redeemed_at IS NULL`

const unredeemedQuery = `
SELECT count(*)
FROM workspace_break_glass_codes
WHERE workspace_id = $1 AND redeemed_at IS NULL`

type codeRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.BreakGlass {
	return &codeRepository{db: db}
}

func (r *codeRepository) Replace(
	ctx context.Context,
	workspaceID uuid.UUID,
	issuedBy *uuid.UUID,
	hashes [][]byte,
) error {
	if err := r.Discard(ctx, workspaceID); err != nil {
		return err
	}

	var issuer any
	if issuedBy != nil {
		issuer = issuedBy.String()
	}

	for _, hash := range hashes {
		if _, err := r.db.Querier(ctx).ExecContext(
			ctx,
			issueQuery,
			workspaceID.String(),
			hash,
			issuer,
		); err != nil {
			return fmt.Errorf("issue recovery code: %w", err)
		}
	}

	return nil
}

func (r *codeRepository) Redeem(
	ctx context.Context,
	workspaceID uuid.UUID,
	hash []byte,
	from string,
) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		redeemQuery,
		workspaceID.String(),
		hash,
		from,
	)
	if err != nil {
		return fmt.Errorf("redeem recovery code: %w", err)
	}

	redeemed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read redeemed recovery code rows: %w", err)
	}

	if redeemed == 0 {
		return entity.ErrBreakGlassCodeInvalid
	}

	return nil
}

func (r *codeRepository) Discard(ctx context.Context, workspaceID uuid.UUID) error {
	if _, err := r.db.Querier(ctx).ExecContext(ctx, discardQuery, workspaceID.String()); err != nil {
		return fmt.Errorf("discard recovery codes: %w", err)
	}

	return nil
}

func (r *codeRepository) CountUnredeemed(
	ctx context.Context,
	workspaceID uuid.UUID,
) (int, error) {
	var count int

	if err := r.db.Querier(ctx).
		QueryRowContext(ctx, unredeemedQuery, workspaceID.String()).
		Scan(&count); err != nil {
		return 0, fmt.Errorf("count recovery codes: %w", err)
	}

	return count, nil
}
