package apitoken

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/sqlboiler/v4/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const (
	uniqueViolation = "23505"
	nameUniqueIndex = "api_tokens_live_name_key"
)

const tokenColumns = `
	t.id, t.account_id, t.name, t.token_hash, t.scopes,
	t.expires_at, t.revoked_at, t.last_used_at, t.expiry_notice_days,
	t.created_at, t.updated_at`

const insertTokenQuery = `
	INSERT INTO api_tokens (id, account_id, name, token_hash, scopes, expires_at)
	VALUES ($1, $2, $3, $4, $5, $6)`

const insertGrantQuery = `
	INSERT INTO api_token_grants (token_id, workspace_id, all_teams)
	VALUES ($1, $2, $3)
	RETURNING id`

const insertGrantTeamQuery = `
	INSERT INTO api_token_grant_teams (grant_id, team_id) VALUES ($1, $2)`

const grantsQuery = `
	SELECT g.token_id, g.workspace_id, g.all_teams, gt.team_id
	FROM api_token_grants g
	LEFT JOIN api_token_grant_teams gt ON gt.grant_id = g.id
	WHERE g.token_id = ANY($1::uuid[])
	ORDER BY g.workspace_id, gt.team_id`

type apiTokenRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.APIToken {
	return &apiTokenRepository{db: db}
}

func (r *apiTokenRepository) Create(ctx context.Context, token entity.APIToken) (entity.APIToken, error) {
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}

	var expiresAt any
	if token.ExpiresAt != nil {
		expiresAt = *token.ExpiresAt
	}

	executor := r.db.Querier(ctx)

	if _, err := executor.ExecContext(
		ctx,
		insertTokenQuery,
		token.ID.String(),
		token.AccountID.String(),
		token.Name,
		token.TokenHash,
		types.StringArray(token.Scopes.Strings()),
		expiresAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == nameUniqueIndex {
			return entity.APIToken{}, entity.ErrAPITokenNameTaken
		}

		return entity.APIToken{}, fmt.Errorf("insert api token: %w", err)
	}

	for _, grant := range token.Grants {
		var grantID string

		if err := executor.QueryRowContext(
			ctx,
			insertGrantQuery,
			token.ID.String(),
			grant.WorkspaceID.String(),
			grant.AllTeams,
		).Scan(&grantID); err != nil {
			return entity.APIToken{}, fmt.Errorf("insert api token grant: %w", err)
		}

		for _, teamID := range grant.TeamIDs {
			if _, err := executor.ExecContext(
				ctx, insertGrantTeamQuery, grantID, teamID.String(),
			); err != nil {
				return entity.APIToken{}, fmt.Errorf("insert api token grant team: %w", err)
			}
		}
	}

	return r.GetByID(ctx, token.ID)
}

func (r *apiTokenRepository) GetByTokenHash(ctx context.Context, tokenHash []byte) (entity.APIToken, error) {
	return r.one(ctx, `SELECT`+tokenColumns+` FROM api_tokens t WHERE t.token_hash = $1`, tokenHash)
}

func (r *apiTokenRepository) GetByID(ctx context.Context, tokenID uuid.UUID) (entity.APIToken, error) {
	return r.one(ctx, `SELECT`+tokenColumns+` FROM api_tokens t WHERE t.id = $1`, tokenID.String())
}

func (r *apiTokenRepository) ListByOwner(ctx context.Context, accountID uuid.UUID) ([]entity.APIToken, error) {
	return r.many(
		ctx,
		`SELECT`+tokenColumns+`
		FROM api_tokens t
		WHERE t.account_id = $1 AND t.revoked_at IS NULL
		ORDER BY t.created_at DESC`,
		accountID.String(),
	)
}

func (r *apiTokenRepository) ListByWorkspaceGrant(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]entity.APIToken, error) {
	return r.many(
		ctx,
		`SELECT`+tokenColumns+`
		FROM api_tokens t
		WHERE t.revoked_at IS NULL
		  AND EXISTS (
		      SELECT 1 FROM api_token_grants g
		      WHERE g.token_id = t.id AND g.workspace_id = $1
		  )
		ORDER BY t.created_at DESC`,
		workspaceID.String(),
	)
}

func (r *apiTokenRepository) ListExpiring(ctx context.Context, notAfter time.Time) ([]entity.APIToken, error) {
	return r.many(
		ctx,
		`SELECT`+tokenColumns+`
		FROM api_tokens t
		WHERE t.revoked_at IS NULL
		  AND t.expires_at IS NOT NULL
		  AND t.expires_at <= $1
		ORDER BY t.expires_at`,
		notAfter,
	)
}

func (r *apiTokenRepository) Revoke(ctx context.Context, tokenID uuid.UUID, revokedAt time.Time) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`UPDATE api_tokens SET revoked_at = $2, updated_at = now()
		 WHERE id = $1 AND revoked_at IS NULL`,
		tokenID.String(),
		revokedAt,
	)
	if err != nil {
		return fmt.Errorf("revoke api token: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke api token: %w", err)
	}

	if affected == 0 {
		return entity.ErrAPITokenNotFound
	}

	return nil
}

func (r *apiTokenRepository) RevokeAllByAccount(
	ctx context.Context,
	accountID uuid.UUID,
	revokedAt time.Time,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`UPDATE api_tokens SET revoked_at = $2, updated_at = now()
		 WHERE account_id = $1 AND revoked_at IS NULL`,
		accountID.String(),
		revokedAt,
	); err != nil {
		return fmt.Errorf("revoke every api token for an account: %w", err)
	}

	return nil
}

func (r *apiTokenRepository) RecordUsage(ctx context.Context, tokenID uuid.UUID, usedAt time.Time) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`UPDATE api_tokens SET last_used_at = $2 WHERE id = $1`,
		tokenID.String(),
		usedAt,
	); err != nil {
		return fmt.Errorf("record api token usage: %w", err)
	}

	return nil
}

func (r *apiTokenRepository) RecordExpiryNotice(ctx context.Context, tokenID uuid.UUID, days int) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`UPDATE api_tokens SET expiry_notice_days = $2, updated_at = now() WHERE id = $1`,
		tokenID.String(),
		days,
	); err != nil {
		return fmt.Errorf("record api token expiry notice: %w", err)
	}

	return nil
}

func (r *apiTokenRepository) one(ctx context.Context, query string, args ...any) (entity.APIToken, error) {
	tokens, err := r.many(ctx, query, args...)
	if err != nil {
		return entity.APIToken{}, err
	}

	if len(tokens) == 0 {
		return entity.APIToken{}, entity.ErrAPITokenNotFound
	}

	return tokens[0], nil
}

func (r *apiTokenRepository) many(ctx context.Context, query string, args ...any) ([]entity.APIToken, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query api tokens: %w", err)
	}

	defer func() { _ = rows.Close() }()

	tokens := make([]entity.APIToken, 0)
	index := make(map[uuid.UUID]int)
	ids := make([]string, 0)

	for rows.Next() {
		token, err := scanToken(rows)
		if err != nil {
			return nil, err
		}

		index[token.ID] = len(tokens)
		ids = append(ids, token.ID.String())
		tokens = append(tokens, token)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read api tokens: %w", err)
	}

	if len(tokens) == 0 {
		return tokens, nil
	}

	if err := r.attachGrants(ctx, tokens, index, ids); err != nil {
		return nil, err
	}

	return tokens, nil
}

func (r *apiTokenRepository) attachGrants(
	ctx context.Context,
	tokens []entity.APIToken,
	index map[uuid.UUID]int,
	ids []string,
) error {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, grantsQuery, types.StringArray(ids))
	if err != nil {
		return fmt.Errorf("query api token grants: %w", err)
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			rawToken, rawWorkspace string
			allTeams               bool
			rawTeam                sql.NullString
		)

		if err := rows.Scan(&rawToken, &rawWorkspace, &allTeams, &rawTeam); err != nil {
			return fmt.Errorf("scan api token grant: %w", err)
		}

		tokenID, err := uuid.Parse(rawToken)
		if err != nil {
			return fmt.Errorf("parse api token grant token id: %w", err)
		}

		workspaceID, err := uuid.Parse(rawWorkspace)
		if err != nil {
			return fmt.Errorf("parse api token grant workspace id: %w", err)
		}

		position, ok := index[tokenID]
		if !ok {
			continue
		}

		token := &tokens[position]

		if !token.Grants.Covers(workspaceID) {
			token.Grants = append(token.Grants, entity.APITokenGrant{
				WorkspaceID: workspaceID,
				AllTeams:    allTeams,
			})
		}

		if !rawTeam.Valid {
			continue
		}

		teamID, err := uuid.Parse(rawTeam.String)
		if err != nil {
			return fmt.Errorf("parse api token grant team id: %w", err)
		}

		for position := range token.Grants {
			if token.Grants[position].WorkspaceID == workspaceID {
				token.Grants[position].TeamIDs = append(token.Grants[position].TeamIDs, teamID)
			}
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("read api token grants: %w", err)
	}

	return nil
}

func scanToken(rows *sql.Rows) (entity.APIToken, error) {
	var (
		rawID, rawAccount    string
		name                 string
		tokenHash            []byte
		scopes               types.StringArray
		expiresAt, revokedAt sql.NullTime
		lastUsedAt           sql.NullTime
		noticeDays           sql.NullInt64
		createdAt, updatedAt time.Time
	)

	if err := rows.Scan(
		&rawID, &rawAccount, &name, &tokenHash, &scopes,
		&expiresAt, &revokedAt, &lastUsedAt, &noticeDays,
		&createdAt, &updatedAt,
	); err != nil {
		return entity.APIToken{}, fmt.Errorf("scan api token: %w", err)
	}

	id, err := uuid.Parse(rawID)
	if err != nil {
		return entity.APIToken{}, fmt.Errorf("parse api token id: %w", err)
	}

	accountID, err := uuid.Parse(rawAccount)
	if err != nil {
		return entity.APIToken{}, fmt.Errorf("parse api token account id: %w", err)
	}

	token := entity.APIToken{
		ID:        id,
		AccountID: accountID,
		Name:      name,
		TokenHash: tokenHash,
		Scopes:    entity.NewAPIScopeSet(scopes),
		Grants:    entity.APITokenGrants{},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	if expiresAt.Valid {
		token.ExpiresAt = &expiresAt.Time
	}

	if revokedAt.Valid {
		token.RevokedAt = &revokedAt.Time
	}

	if lastUsedAt.Valid {
		token.LastUsedAt = &lastUsedAt.Time
	}

	if noticeDays.Valid {
		days := int(noticeDays.Int64)
		token.ExpiryNoticeDays = &days
	}

	return token, nil
}
