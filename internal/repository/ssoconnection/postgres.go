package ssoconnection

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/sqlboiler/v4/types"
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/crypter"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const protocolQuery = `SELECT protocol FROM workspace_sso_connections WHERE workspace_id = $1`

const upsertParentQuery = `
INSERT INTO workspace_sso_connections (
    workspace_id, protocol, provisioning, admin_group, verified_at, created_at, updated_at
)
VALUES ($1, $2, $3, $5, NULL, $4, $4)
ON CONFLICT (workspace_id) DO UPDATE SET
    protocol     = excluded.protocol,
    provisioning = excluded.provisioning,
    admin_group  = excluded.admin_group,
    verified_at  = NULL,
    updated_at   = excluded.updated_at`

const verifiedQuery = `
SELECT verified_at IS NOT NULL
FROM workspace_sso_connections
WHERE workspace_id = $1`

const deleteParentQuery = `DELETE FROM workspace_sso_connections WHERE workspace_id = $1`

const markVerifiedQuery = `
UPDATE workspace_sso_connections
SET verified_at = $2, updated_at = $2
WHERE workspace_id = $1`

const recordNoticeQuery = `
UPDATE workspace_saml_connections
SET expiry_notice_days = $2, updated_at = now()
WHERE workspace_id = $1`

const oidcColumns = `o.workspace_id, o.issuer, o.discovered, o.authorization_endpoint, o.token_endpoint,
       o.jwks_uri, o.userinfo_endpoint, o.client_id, o.client_secret_sealed, o.scopes,
       o.groups_claim, s.provisioning, s.admin_group, s.verified_at, s.created_at, s.updated_at`

const oidcByWorkspaceQuery = `
SELECT ` + oidcColumns + `
FROM workspace_oidc_connections o
JOIN workspace_sso_connections s ON s.workspace_id = o.workspace_id
WHERE o.workspace_id = $1`

const upsertOIDCQuery = `
INSERT INTO workspace_oidc_connections (
    workspace_id, issuer, discovered, authorization_endpoint, token_endpoint,
    jwks_uri, userinfo_endpoint, client_id, client_secret_sealed, scopes, groups_claim
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (workspace_id) DO UPDATE SET
    issuer                 = excluded.issuer,
    discovered             = excluded.discovered,
    authorization_endpoint = excluded.authorization_endpoint,
    token_endpoint         = excluded.token_endpoint,
    jwks_uri               = excluded.jwks_uri,
    userinfo_endpoint      = excluded.userinfo_endpoint,
    client_id              = excluded.client_id,
    client_secret_sealed   = excluded.client_secret_sealed,
    scopes                 = excluded.scopes,
    groups_claim           = excluded.groups_claim`

const samlColumns = `m.workspace_id, m.entity_id, m.sso_url, m.slo_url, m.metadata_url,
       m.idp_certificates, m.idp_certificate_expires_at, m.sp_entity_id, m.sp_private_key_sealed,
       m.sp_certificate, m.allow_idp_initiated, m.email_attribute, m.name_attribute,
       m.groups_attribute, m.expiry_notice_days, s.provisioning, s.admin_group, s.verified_at,
       s.created_at, s.updated_at`

const samlByWorkspaceQuery = `
SELECT ` + samlColumns + `
FROM workspace_saml_connections m
JOIN workspace_sso_connections s ON s.workspace_id = m.workspace_id
WHERE m.workspace_id = $1`

const samlCertificatesQuery = `
SELECT ` + samlColumns + `
FROM workspace_saml_connections m
JOIN workspace_sso_connections s ON s.workspace_id = m.workspace_id
ORDER BY m.idp_certificate_expires_at`

const upsertSAMLQuery = `
INSERT INTO workspace_saml_connections (
    workspace_id, entity_id, sso_url, slo_url, metadata_url, idp_certificates,
    idp_certificate_expires_at, sp_entity_id, sp_private_key_sealed, sp_certificate,
    allow_idp_initiated, email_attribute, name_attribute, groups_attribute, expiry_notice_days
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NULL)
ON CONFLICT (workspace_id) DO UPDATE SET
    entity_id                  = excluded.entity_id,
    sso_url                    = excluded.sso_url,
    slo_url                    = excluded.slo_url,
    metadata_url               = excluded.metadata_url,
    idp_certificates           = excluded.idp_certificates,
    idp_certificate_expires_at = excluded.idp_certificate_expires_at,
    sp_entity_id               = excluded.sp_entity_id,
    sp_private_key_sealed      = excluded.sp_private_key_sealed,
    sp_certificate             = excluded.sp_certificate,
    allow_idp_initiated        = excluded.allow_idp_initiated,
    email_attribute            = excluded.email_attribute,
    name_attribute             = excluded.name_attribute,
    groups_attribute           = excluded.groups_attribute,
    expiry_notice_days         = NULL,
    updated_at                 = now()`

type connectionRepository struct {
	db      *postgres.Client
	crypter *crypter.Crypter
}

func New(db *postgres.Client, sealer *crypter.Crypter) repository.SSOConnection {
	return &connectionRepository{db: db, crypter: sealer}
}

type scanner interface {
	Scan(dest ...any) error
}

func (r *connectionRepository) Protocol(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.SSOProtocol, error) {
	var protocol string

	err := r.db.Querier(ctx).
		QueryRowContext(ctx, protocolQuery, workspaceID.String()).
		Scan(&protocol)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", entity.ErrSSOConnectionNotFound
		}

		return "", fmt.Errorf("read sso protocol: %w", err)
	}

	return entity.SSOProtocol(protocol), nil
}

func (r *connectionRepository) seal(secret []byte) ([]byte, error) {
	sealed, err := r.crypter.Seal(secret)
	if err != nil {
		if errors.Is(err, crypter.ErrKeyMissing) {
			return nil, entity.ErrSSOEncryptionKeyMissing
		}

		return nil, err
	}

	return sealed, nil
}

func (r *connectionRepository) open(sealed []byte) ([]byte, error) {
	secret, err := r.crypter.Open(sealed)
	if err != nil {
		if errors.Is(err, crypter.ErrKeyMissing) {
			return nil, entity.ErrSSOEncryptionKeyMissing
		}

		return nil, fmt.Errorf("open stored secret: %w", err)
	}

	return secret, nil
}

func (r *connectionRepository) claim(
	ctx context.Context,
	workspaceID uuid.UUID,
	protocol entity.SSOProtocol,
	provisioning bool,
	adminGroup string,
) error {
	held, err := r.Protocol(ctx, workspaceID)
	if err != nil && !errors.Is(err, entity.ErrSSOConnectionNotFound) {
		return err
	}

	if err == nil && held != protocol {
		if _, err := r.db.Querier(ctx).ExecContext(ctx, deleteParentQuery, workspaceID.String()); err != nil {
			return fmt.Errorf("replace sso connection: %w", err)
		}
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		upsertParentQuery,
		workspaceID.String(),
		string(protocol),
		provisioning,
		time.Now().UTC(),
		adminGroup,
	); err != nil {
		return fmt.Errorf("save sso connection: %w", err)
	}

	return nil
}

func (r *connectionRepository) scanOIDC(row scanner) (entity.OIDCConnection, error) {
	var (
		connection entity.OIDCConnection
		workspace  string
		sealed     []byte
		scopes     types.StringArray
		verified   sql.NullTime
	)

	if err := row.Scan(
		&workspace,
		&connection.Endpoints.Issuer,
		&connection.Discovered,
		&connection.Endpoints.AuthorizationEndpoint,
		&connection.Endpoints.TokenEndpoint,
		&connection.Endpoints.JWKSURI,
		&connection.Endpoints.UserinfoEndpoint,
		&connection.ClientID,
		&sealed,
		&scopes,
		&connection.GroupsClaim,
		&connection.Provisioning,
		&connection.AdminGroup,
		&verified,
		&connection.CreatedAt,
		&connection.UpdatedAt,
	); err != nil {
		return entity.OIDCConnection{}, err
	}

	parsed, err := uuid.Parse(workspace)
	if err != nil {
		return entity.OIDCConnection{}, fmt.Errorf("parse oidc connection workspace id: %w", err)
	}

	connection.WorkspaceID = parsed
	connection.Scopes = scopes

	secret, err := r.open(sealed)
	if err != nil {
		return entity.OIDCConnection{}, err
	}

	connection.ClientSecret = string(secret)

	if verified.Valid {
		at := verified.Time
		connection.VerifiedAt = &at
	}

	return connection, nil
}

func (r *connectionRepository) GetOIDC(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.OIDCConnection, error) {
	connection, err := r.scanOIDC(r.db.Querier(ctx).QueryRowContext(
		ctx,
		oidcByWorkspaceQuery,
		workspaceID.String(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.OIDCConnection{}, entity.ErrSSOConnectionNotFound
		}

		return entity.OIDCConnection{}, fmt.Errorf("find oidc connection: %w", err)
	}

	return connection, nil
}

func (r *connectionRepository) SaveOIDC(
	ctx context.Context,
	connection entity.OIDCConnection,
) (entity.OIDCConnection, error) {
	sealed, err := r.seal([]byte(connection.ClientSecret))
	if err != nil {
		return entity.OIDCConnection{}, err
	}

	if err := r.claim(ctx, connection.WorkspaceID, entity.SSOProtocolOIDC, connection.Provisioning, connection.AdminGroup); err != nil {
		return entity.OIDCConnection{}, err
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		upsertOIDCQuery,
		connection.WorkspaceID.String(),
		connection.Endpoints.Issuer,
		connection.Discovered,
		connection.Endpoints.AuthorizationEndpoint,
		connection.Endpoints.TokenEndpoint,
		connection.Endpoints.JWKSURI,
		connection.Endpoints.UserinfoEndpoint,
		connection.ClientID,
		sealed,
		types.StringArray(entity.NormalizeOIDCScopes(connection.Scopes)),
		connection.GroupsClaim,
	); err != nil {
		return entity.OIDCConnection{}, fmt.Errorf("save oidc connection: %w", err)
	}

	return r.GetOIDC(ctx, connection.WorkspaceID)
}

func (r *connectionRepository) scanSAML(row scanner) (entity.SAMLConnection, error) {
	var (
		connection   entity.SAMLConnection
		workspace    string
		certificates types.StringArray
		sealed       []byte
		notice       sql.NullInt64
		verified     sql.NullTime
	)

	if err := row.Scan(
		&workspace,
		&connection.Descriptor.EntityID,
		&connection.Descriptor.SSOURL,
		&connection.Descriptor.SLOURL,
		&connection.MetadataURL,
		&certificates,
		&connection.Descriptor.ExpiresAt,
		&connection.SPEntityID,
		&sealed,
		&connection.SPCertificate,
		&connection.AllowIDPInitiated,
		&connection.Mapping.Email,
		&connection.Mapping.Name,
		&connection.Mapping.Groups,
		&notice,
		&connection.Provisioning,
		&connection.AdminGroup,
		&verified,
		&connection.CreatedAt,
		&connection.UpdatedAt,
	); err != nil {
		return entity.SAMLConnection{}, err
	}

	parsed, err := uuid.Parse(workspace)
	if err != nil {
		return entity.SAMLConnection{}, fmt.Errorf("parse saml connection workspace id: %w", err)
	}

	connection.WorkspaceID = parsed
	connection.Descriptor.Certificates = certificates

	key, err := r.open(sealed)
	if err != nil {
		return entity.SAMLConnection{}, err
	}

	connection.SPPrivateKey = key

	if notice.Valid {
		days := int(notice.Int64)
		connection.ExpiryNoticeDays = &days
	}

	if verified.Valid {
		at := verified.Time
		connection.VerifiedAt = &at
	}

	return connection, nil
}

func (r *connectionRepository) GetSAML(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.SAMLConnection, error) {
	connection, err := r.scanSAML(r.db.Querier(ctx).QueryRowContext(
		ctx,
		samlByWorkspaceQuery,
		workspaceID.String(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.SAMLConnection{}, entity.ErrSSOConnectionNotFound
		}

		return entity.SAMLConnection{}, fmt.Errorf("find saml connection: %w", err)
	}

	return connection, nil
}

func (r *connectionRepository) SaveSAML(
	ctx context.Context,
	connection entity.SAMLConnection,
) (entity.SAMLConnection, error) {
	sealed, err := r.seal(connection.SPPrivateKey)
	if err != nil {
		return entity.SAMLConnection{}, err
	}

	if err := r.claim(ctx, connection.WorkspaceID, entity.SSOProtocolSAML, connection.Provisioning, connection.AdminGroup); err != nil {
		return entity.SAMLConnection{}, err
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		upsertSAMLQuery,
		connection.WorkspaceID.String(),
		connection.Descriptor.EntityID,
		connection.Descriptor.SSOURL,
		connection.Descriptor.SLOURL,
		connection.MetadataURL,
		types.StringArray(connection.Descriptor.Certificates),
		connection.Descriptor.ExpiresAt.UTC(),
		connection.SPEntityID,
		sealed,
		connection.SPCertificate,
		connection.AllowIDPInitiated,
		connection.Mapping.Email,
		connection.Mapping.Name,
		connection.Mapping.Groups,
	); err != nil {
		return entity.SAMLConnection{}, fmt.Errorf("save saml connection: %w", err)
	}

	return r.GetSAML(ctx, connection.WorkspaceID)
}

func (r *connectionRepository) Delete(ctx context.Context, workspaceID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, deleteParentQuery, workspaceID.String())
	if err != nil {
		return fmt.Errorf("delete sso connection: %w", err)
	}

	removed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted sso connection rows: %w", err)
	}

	if removed == 0 {
		return entity.ErrSSOConnectionNotFound
	}

	return nil
}

func (r *connectionRepository) MarkVerified(
	ctx context.Context,
	workspaceID uuid.UUID,
	at time.Time,
) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		markVerifiedQuery,
		workspaceID.String(),
		at.UTC(),
	)
	if err != nil {
		return fmt.Errorf("mark sso connection verified: %w", err)
	}

	marked, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read verified sso connection rows: %w", err)
	}

	if marked == 0 {
		return entity.ErrSSOConnectionNotFound
	}

	return nil
}

func (r *connectionRepository) ListSAMLCertificates(
	ctx context.Context,
) ([]entity.SAMLConnection, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, samlCertificatesQuery)
	if err != nil {
		return nil, fmt.Errorf("list saml connections: %w", err)
	}

	defer func() { _ = rows.Close() }()

	connections := make([]entity.SAMLConnection, 0)

	for rows.Next() {
		connection, err := r.scanSAML(rows)
		if err != nil {
			return nil, fmt.Errorf("scan saml connection: %w", err)
		}

		connections = append(connections, connection)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate saml connections: %w", err)
	}

	return connections, nil
}

func (r *connectionRepository) RecordExpiryNotice(
	ctx context.Context,
	workspaceID uuid.UUID,
	days int,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		recordNoticeQuery,
		workspaceID.String(),
		days,
	); err != nil {
		return fmt.Errorf("record certificate expiry notice: %w", err)
	}

	return nil
}

func (r *connectionRepository) Verified(
	ctx context.Context,
	workspaceID uuid.UUID,
) (bool, error) {
	var verified bool

	err := r.db.Querier(ctx).
		QueryRowContext(ctx, verifiedQuery, workspaceID.String()).
		Scan(&verified)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, entity.ErrSSOConnectionNotFound
		}

		return false, fmt.Errorf("read sso verification: %w", err)
	}

	return verified, nil
}
