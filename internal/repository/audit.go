package repository

import (
	"context"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=audit.go -destination=audit/mock_audit.go -package=audit -mock_names=Audit=MockAudit,AuditRetention=MockAuditRetention

type Audit interface {
	Record(ctx context.Context, entry entity.AuditEntry) error
	List(ctx context.Context, filter entity.AuditFilter) ([]entity.AuditEntry, error)
}

type AuditRetention interface {
	DropBefore(ctx context.Context, cutoff time.Time, batch int) (int, error)
}
