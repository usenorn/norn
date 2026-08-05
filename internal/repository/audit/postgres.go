package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const recordAuditQuery = `
INSERT INTO audit_events (
    id, occurred_at, workspace_id, workspace_name, action, outcome,
    actor_account_id, actor_kind, actor_name,
    actor_token_id, actor_token_name, actor_agent_id, actor_agent_name,
    auth_method, source_ip, user_agent,
    resource_kind, resource_id, resource_name, detail
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)`

const auditColumns = `
SELECT e.id,
       e.occurred_at,
       coalesce(e.workspace_id::text, ''),
       coalesce(e.workspace_name, ''),
       e.action,
       e.outcome,
       coalesce(e.actor_account_id::text, ''),
       e.actor_kind,
       coalesce(e.actor_name, ''),
       coalesce(e.actor_token_id::text, ''),
       coalesce(e.actor_token_name, ''),
       coalesce(e.actor_agent_id::text, ''),
       coalesce(e.actor_agent_name, ''),
       e.auth_method,
       coalesce(host(e.source_ip), ''),
       coalesce(e.user_agent, ''),
       e.resource_kind,
       coalesce(e.resource_id::text, ''),
       coalesce(e.resource_name, ''),
       coalesce(e.detail::text, '')
FROM audit_events e`

const dropExpiredQuery = `
DELETE FROM audit_events
WHERE id IN (SELECT id FROM audit_events WHERE occurred_at < $1 LIMIT $2)`

type auditRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Audit {
	return &auditRepository{db: db}
}

func NewRetention(db *postgres.Client) repository.AuditRetention {
	return &auditRepository{db: db}
}

func (r *auditRepository) Record(ctx context.Context, entry entity.AuditEntry) error {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}

	if entry.OccurredAt.IsZero() {
		entry.OccurredAt = time.Now().UTC()
	}

	var detail any

	if len(entry.Detail) > 0 {
		encoded, err := json.Marshal(entry.Detail)
		if err != nil {
			return fmt.Errorf("encode audit detail: %w", err)
		}

		detail = encoded
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		recordAuditQuery,
		entry.ID.String(),
		entry.OccurredAt,
		optionalID(entry.WorkspaceID),
		nullIfEmpty(entry.WorkspaceName),
		string(entry.Action),
		string(entry.Outcome),
		optionalID(entry.Actor.AccountID),
		string(entry.Actor.Kind),
		nullIfEmpty(entry.Actor.Name),
		optionalPointer(entry.Actor.TokenID),
		nullIfEmpty(entry.Actor.TokenName),
		optionalPointer(entry.Actor.AgentID),
		nullIfEmpty(entry.Actor.AgentName),
		string(entry.Actor.How()),
		optionalIP(entry.SourceIP),
		nullIfEmpty(entry.UserAgent),
		entry.ResourceKind,
		optionalID(entry.ResourceID),
		nullIfEmpty(entry.ResourceName),
		detail,
	); err != nil {
		return fmt.Errorf("record audit event: %w", err)
	}

	return nil
}

func (r *auditRepository) List(
	ctx context.Context,
	filter entity.AuditFilter,
) ([]entity.AuditEntry, error) {
	filter = filter.Normalized()

	where := make([]string, 0, 8)
	args := make([]any, 0, 8)

	bind := func(value any) string {
		args = append(args, value)

		return fmt.Sprintf("$%d", len(args))
	}

	if !filter.InstanceWide {
		where = append(where, "e.workspace_id = "+bind(filter.WorkspaceID.String()))
	}

	if filter.ActorAccountID != uuid.Nil {
		where = append(where, "e.actor_account_id = "+bind(filter.ActorAccountID.String()))
	}

	if len(filter.Actions) > 0 {
		named := make([]string, 0, len(filter.Actions))
		for _, action := range filter.Actions {
			named = append(named, bind(string(action)))
		}

		where = append(where, "e.action IN ("+strings.Join(named, ", ")+")")
	}

	if filter.ResourceKind != "" {
		where = append(where, "e.resource_kind = "+bind(filter.ResourceKind))
	}

	if filter.ResourceID != uuid.Nil {
		where = append(where, "e.resource_id = "+bind(filter.ResourceID.String()))
	}

	if filter.From != nil {
		where = append(where, "e.occurred_at >= "+bind(*filter.From))
	}

	if filter.To != nil {
		where = append(where, "e.occurred_at < "+bind(*filter.To))
	}

	if filter.Cursor != nil {
		where = append(where, fmt.Sprintf(
			"(e.occurred_at, e.id) < (%s, %s::uuid)",
			bind(filter.Cursor.OccurredAt), bind(filter.Cursor.ID.String()),
		))
	}

	query := auditColumns

	if len(where) > 0 {
		query += "\nWHERE " + strings.Join(where, "\n  AND ")
	}

	query += "\nORDER BY e.occurred_at DESC, e.id DESC\nLIMIT " + bind(filter.Limit)

	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query audit events: %w", err)
	}

	defer func() { _ = rows.Close() }()

	entries := make([]entity.AuditEntry, 0, filter.Limit)

	for rows.Next() {
		entry, err := scan(rows)
		if err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read audit events: %w", err)
	}

	return entries, nil
}

func (r *auditRepository) DropBefore(
	ctx context.Context,
	cutoff time.Time,
	batch int,
) (int, error) {
	result, err := r.db.Querier(ctx).ExecContext(ctx, dropExpiredQuery, cutoff, batch)
	if err != nil {
		return 0, fmt.Errorf("drop expired audit events: %w", err)
	}

	dropped, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("drop expired audit events: %w", err)
	}

	return int(dropped), nil
}

func scan(rows *sql.Rows) (entity.AuditEntry, error) {
	var (
		rawID, rawWorkspace          string
		workspaceName, action        string
		outcome, rawActor, actorKind string
		actorName, rawToken          string
		tokenName, rawAgent          string
		agentName, authMethod        string
		sourceIP, userAgent          string
		resourceKind, rawResource    string
		resourceName, detail         string
		occurredAt                   time.Time
	)

	if err := rows.Scan(
		&rawID, &occurredAt, &rawWorkspace, &workspaceName, &action, &outcome,
		&rawActor, &actorKind, &actorName, &rawToken, &tokenName, &rawAgent, &agentName,
		&authMethod, &sourceIP, &userAgent,
		&resourceKind, &rawResource, &resourceName, &detail,
	); err != nil {
		return entity.AuditEntry{}, fmt.Errorf("scan audit event: %w", err)
	}

	entry := entity.AuditEntry{
		OccurredAt:    occurredAt,
		WorkspaceName: workspaceName,
		Action:        entity.AuditAction(action),
		Outcome:       entity.AuditOutcome(outcome),
		UserAgent:     userAgent,
		ResourceKind:  resourceKind,
		ResourceName:  resourceName,
		Actor: entity.AuditActor{
			Kind:       entity.ActorKind(actorKind),
			Name:       actorName,
			TokenName:  tokenName,
			AgentName:  agentName,
			AuthMethod: entity.SessionAuthMethod(authMethod),
		},
	}

	id, err := uuid.Parse(rawID)
	if err != nil {
		return entity.AuditEntry{}, fmt.Errorf("parse audit event id: %w", err)
	}

	entry.ID = id

	for target, raw := range map[*uuid.UUID]string{
		&entry.WorkspaceID:     rawWorkspace,
		&entry.Actor.AccountID: rawActor,
		&entry.ResourceID:      rawResource,
	} {
		if raw == "" {
			continue
		}

		parsed, err := uuid.Parse(raw)
		if err != nil {
			return entity.AuditEntry{}, fmt.Errorf("parse audit event identifier: %w", err)
		}

		*target = parsed
	}

	for target, raw := range map[**uuid.UUID]string{
		&entry.Actor.TokenID: rawToken,
		&entry.Actor.AgentID: rawAgent,
	} {
		if raw == "" {
			continue
		}

		parsed, err := uuid.Parse(raw)
		if err != nil {
			return entity.AuditEntry{}, fmt.Errorf("parse audit event identifier: %w", err)
		}

		*target = &parsed
	}

	if sourceIP != "" {
		if addr, err := netip.ParseAddr(sourceIP); err == nil {
			entry.SourceIP = addr
		}
	}

	if detail != "" {
		if err := json.Unmarshal([]byte(detail), &entry.Detail); err != nil {
			return entity.AuditEntry{}, fmt.Errorf("decode audit detail: %w", err)
		}
	}

	return entry, nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}

	return value
}

func optionalID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}

	return id.String()
}

func optionalPointer(id *uuid.UUID) any {
	if id == nil || *id == uuid.Nil {
		return nil
	}

	return id.String()
}

func optionalIP(ip netip.Addr) any {
	if !ip.IsValid() {
		return nil
	}

	return ip.String()
}
