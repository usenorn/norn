package audit

import (
	"context"
	"strconv"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type reader struct {
	events      repository.Audit
	memberships repository.Membership
	authorizer  service.Authorizer
	audit       service.Audit
	licensing   service.Licensing
	cfg         config.Audit
}

func NewReader(
	events repository.Audit,
	memberships repository.Membership,
	authorizer service.Authorizer,
	audit service.Audit,
	licensing service.Licensing,
	cfg config.Audit,
) service.AuditLog {
	return &reader{
		events:      events,
		memberships: memberships,
		authorizer:  authorizer,
		audit:       audit,
		licensing:   licensing,
		cfg:         cfg,
	}
}

func (r *reader) Availability(_ context.Context) service.AuditAvailability {
	return service.AuditAvailability{
		Available:     r.licensing.Permits(entity.FeatureAudit) == nil,
		RetentionDays: r.cfg.RetentionDays(),
	}
}

func (r *reader) List(
	ctx context.Context,
	scope service.AuditScope,
	input service.ListAuditInput,
) (service.AuditPage, error) {
	filter, err := r.authorized(ctx, scope, input)
	if err != nil {
		return service.AuditPage{}, err
	}

	found, err := r.events.List(ctx, filter.Lookahead())
	if err != nil {
		return service.AuditPage{}, err
	}

	page := service.AuditPage{Events: found}

	if len(found) > filter.Limit {
		page.Events = found[:filter.Limit]
		page.NextCursor = page.Events[len(page.Events)-1].Cursor().Encode()
	}

	return page, nil
}

func (r *reader) Export(
	ctx context.Context,
	scope service.AuditScope,
	input service.ListAuditInput,
	emit func(entity.AuditEntry) error,
) error {
	filter, err := r.authorized(ctx, scope, input)
	if err != nil {
		return err
	}

	filter.Limit = entity.AuditExportBatch
	exported := 0

	for {
		batch, err := r.events.List(ctx, filter)
		if err != nil {
			return err
		}

		for _, entry := range batch {
			if err := emit(entry); err != nil {
				return err
			}

			exported++
		}

		if len(batch) < filter.Limit {
			break
		}

		cursor := batch[len(batch)-1].Cursor()
		filter.Cursor = &cursor
	}

	r.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  scope.WorkspaceID,
		Action:       entity.AuditExported,
		ResourceKind: string(entity.ResourceAuditLog),
		Detail:       map[string]string{"records": strconv.Itoa(exported)},
	})

	return nil
}

func (r *reader) authorized(
	ctx context.Context,
	scope service.AuditScope,
	input service.ListAuditInput,
) (entity.AuditFilter, error) {
	if err := r.licensing.Permits(entity.FeatureAudit); err != nil {
		return entity.AuditFilter{}, err
	}

	filter := input.Filter.Normalized()
	filter.InstanceWide = scope.Instance
	filter.WorkspaceID = scope.WorkspaceID

	if input.Cursor != "" {
		cursor, err := entity.DecodeAuditCursor(input.Cursor)
		if err != nil {
			return entity.AuditFilter{}, err
		}

		filter.Cursor = &cursor
	}

	if scope.Instance {
		if _, err := r.authorizer.Decide(ctx, entity.AccessRequest{
			Resource: entity.ResourceInstance,
			Action:   entity.ActionRead,
		}); err != nil {
			return entity.AuditFilter{}, err
		}

		return filter, nil
	}

	decision, err := r.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAuditLog,
		Action:      entity.ActionRead,
		WorkspaceID: scope.WorkspaceID,
	})
	if err != nil {
		return entity.AuditFilter{}, err
	}

	membership, err := r.memberships.Get(ctx, scope.WorkspaceID, decision.Actor.Authority())
	if err != nil {
		return entity.AuditFilter{}, err
	}

	if !membership.ReadsAudit {
		r.audit.Record(ctx, entity.AuditEntry{
			WorkspaceID:  scope.WorkspaceID,
			Action:       entity.AuditAccessDenied,
			Outcome:      entity.AuditDenied,
			ResourceKind: string(entity.ResourceAuditLog),
			Detail:       map[string]string{"reason": "audit_access_not_granted"},
		})

		return entity.AuditFilter{}, entity.ErrAuditNotPermitted
	}

	return filter, nil
}

func (r *reader) SetAccess(
	ctx context.Context,
	workspaceID, accountID uuid.UUID,
	reads bool,
) (entity.Membership, error) {
	if _, err := r.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAuditLog,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
	}); err != nil {
		return entity.Membership{}, err
	}

	membership, err := r.memberships.SetAuditAccess(ctx, workspaceID, accountID, reads)
	if err != nil {
		return entity.Membership{}, err
	}

	r.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditAuditAccess,
		ResourceKind: string(entity.ResourceMembership),
		ResourceID:   accountID,
		Detail:       map[string]string{"reads_audit": strconv.FormatBool(reads)},
	})

	return membership, nil
}
