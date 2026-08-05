package issuefollower

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const followQuery = `
INSERT INTO workspace_issue_followers (issue_id, workspace_id, account_id, state)
VALUES ($1, $2, $3, 'following')
ON CONFLICT (issue_id, account_id) DO NOTHING`

const setStateQuery = `
INSERT INTO workspace_issue_followers (issue_id, workspace_id, account_id, state)
VALUES ($1, $2, $3, $4)
ON CONFLICT (issue_id, account_id)
DO UPDATE SET state = excluded.state, updated_at = now()`

const followerQuery = `
SELECT issue_id, workspace_id, account_id, state
FROM workspace_issue_followers
WHERE issue_id = $1 AND account_id = $2`

const followersQuery = `
SELECT issue_id, workspace_id, account_id, state
FROM workspace_issue_followers
WHERE issue_id = $1
ORDER BY account_id`

type followerRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.IssueFollower {
	return &followerRepository{db: db}
}

func (r *followerRepository) Follow(ctx context.Context, follower entity.IssueFollower) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, followQuery,
		follower.IssueID.String(), follower.WorkspaceID.String(), follower.AccountID.String(),
	); err != nil {
		return fmt.Errorf("follow issue: %w", err)
	}

	return nil
}

func (r *followerRepository) SetState(ctx context.Context, follower entity.IssueFollower) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, setStateQuery,
		follower.IssueID.String(), follower.WorkspaceID.String(),
		follower.AccountID.String(), string(follower.State),
	); err != nil {
		return fmt.Errorf("set issue follow state: %w", err)
	}

	return nil
}

func (r *followerRepository) Get(ctx context.Context, issueID, accountID uuid.UUID) (entity.IssueFollower, error) {
	follower, err := scan(r.db.Querier(ctx).QueryRowContext(
		ctx, followerQuery, issueID.String(), accountID.String(),
	))

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return entity.IssueFollower{IssueID: issueID, AccountID: accountID}, nil
	case err != nil:
		return entity.IssueFollower{}, fmt.Errorf("read issue follower: %w", err)
	default:
		return follower, nil
	}
}

func (r *followerRepository) List(ctx context.Context, issueID uuid.UUID) ([]entity.IssueFollower, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, followersQuery, issueID.String())
	if err != nil {
		return nil, fmt.Errorf("read issue followers: %w", err)
	}

	defer func() { _ = rows.Close() }()

	followers := make([]entity.IssueFollower, 0)

	for rows.Next() {
		follower, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("scan issue follower: %w", err)
		}

		followers = append(followers, follower)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read issue followers: %w", err)
	}

	return followers, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scan(row scanner) (entity.IssueFollower, error) {
	var issue, workspace, account, state string

	if err := row.Scan(&issue, &workspace, &account, &state); err != nil {
		return entity.IssueFollower{}, err
	}

	identifiers := make([]uuid.UUID, 0, 3)

	for _, raw := range []string{issue, workspace, account} {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return entity.IssueFollower{}, err
		}

		identifiers = append(identifiers, parsed)
	}

	return entity.IssueFollower{
		IssueID:     identifiers[0],
		WorkspaceID: identifiers[1],
		AccountID:   identifiers[2],
		State:       entity.FollowState(state),
	}, nil
}
