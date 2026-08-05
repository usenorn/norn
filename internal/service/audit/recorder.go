package audit

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type recorder struct {
	events     repository.Audit
	accounts   repository.Account
	workspaces repository.Workspace
}

func NewRecorder(
	events repository.Audit,
	accounts repository.Account,
	workspaces repository.Workspace,
) service.Audit {
	return &recorder{events: events, accounts: accounts, workspaces: workspaces}
}

func (r *recorder) Record(ctx context.Context, entry entity.AuditEntry) {
	entry = r.attributed(ctx, entry)

	postgres.AfterCommit(ctx, func(ctx context.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				r.failed(ctx, entry, recovered)
			}
		}()

		r.write(ctx, entry)
	})
}

func (r *recorder) attributed(ctx context.Context, entry entity.AuditEntry) entity.AuditEntry {
	if entry.Actor.Kind == "" {
		if actor, ok := identity.Actor(ctx); ok {
			entry.Actor = entity.ActorAudited(actor, entry.Actor.Name)
		} else {
			entry.Actor.Kind = entity.ActorKindSystem
		}
	}

	client := identity.Client(ctx)

	if !entry.SourceIP.IsValid() {
		entry.SourceIP = client.IP
	}

	if entry.UserAgent == "" {
		entry.UserAgent = client.UserAgent
	}

	if entry.Outcome == "" {
		entry.Outcome = entity.AuditSucceeded
	}

	return entry
}

func (r *recorder) write(ctx context.Context, entry entity.AuditEntry) {
	entry = r.named(ctx, entry)

	if err := r.events.Record(ctx, entry); err != nil {
		r.failed(ctx, entry, err)
	}
}

func (r *recorder) named(ctx context.Context, entry entity.AuditEntry) entity.AuditEntry {
	if entry.Actor.Name == "" && entry.Actor.AccountID != uuid.Nil {
		if account, err := r.accounts.GetByID(ctx, entry.Actor.AccountID); err == nil {
			entry.Actor.Name = account.DisplayName
			if entry.Actor.Name == "" {
				entry.Actor.Name = account.Email
			}
		}
	}

	if entry.WorkspaceName == "" && entry.WorkspaceID != uuid.Nil {
		if workspace, err := r.workspaces.GetByID(ctx, entry.WorkspaceID); err == nil {
			entry.WorkspaceName = workspace.Name
		}
	}

	return entry
}

func (r *recorder) failed(ctx context.Context, entry entity.AuditEntry, cause any) {
	logging.From(ctx).ErrorContext(
		ctx,
		"audit write failed",
		"audit_action", string(entry.Action),
		"audit_outcome", string(entry.Outcome),
		"workspace_id", entry.WorkspaceID.String(),
		"actor_account_id", entry.Actor.AccountID.String(),
		"cause", cause,
	)
}
