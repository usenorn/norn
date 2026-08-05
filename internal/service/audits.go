package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=audits.go -destination=audit/mock_audits.go -package=audit -mock_names=Audit=MockAudit,AuditLog=MockAuditLog,AuditRetention=MockAuditRetention

type Audit interface {
	Record(ctx context.Context, entry entity.AuditEntry)
}

type AuditScope struct {
	WorkspaceID uuid.UUID
	Instance    bool
}

type ListAuditInput struct {
	Filter entity.AuditFilter
	Cursor string
}

type AuditPage struct {
	Events     []entity.AuditEntry
	NextCursor string
}

type AuditAvailability struct {
	Available     bool
	RetentionDays int
	Holder        string
}

type AuditLog interface {
	Availability(ctx context.Context) AuditAvailability
	List(ctx context.Context, scope AuditScope, input ListAuditInput) (AuditPage, error)
	Export(
		ctx context.Context,
		scope AuditScope,
		input ListAuditInput,
		emit func(entity.AuditEntry) error,
	) error
	SetAccess(
		ctx context.Context,
		workspaceID, accountID uuid.UUID,
		reads bool,
	) (entity.Membership, error)
}

type AuditRetention interface {
	Sweep(ctx context.Context) (int, error)
}
