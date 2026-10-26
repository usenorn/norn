package workspacedomain

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type domainsService struct {
	domains    repository.WorkspaceDomain
	ownership  repository.DomainOwnership
	authorizer service.Authorizer
	audit      service.Audit
}

func New(
	domains repository.WorkspaceDomain,
	ownership repository.DomainOwnership,
	authorizer service.Authorizer,
	audit service.Audit,
) service.WorkspaceDomains {
	return &domainsService{
		domains:    domains,
		ownership:  ownership,
		authorizer: authorizer,
		audit:      audit,
	}
}

func (s *domainsService) authorize(ctx context.Context, workspaceID uuid.UUID, action entity.Action) error {
	_, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceSSOConnection,
		Action:      action,
		WorkspaceID: workspaceID,
	})

	return err
}

func (s *domainsService) record(ctx context.Context, action entity.AuditAction, domain entity.WorkspaceDomain) {
	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  domain.WorkspaceID,
		Action:       action,
		ResourceKind: string(entity.ResourceSSOConnection),
		ResourceID:   domain.ID,
		ResourceName: domain.Domain,
		Detail:       map[string]string{"domain": domain.Domain},
	})
}

func (s *domainsService) List(ctx context.Context, workspaceID uuid.UUID) ([]entity.WorkspaceDomain, error) {
	if err := s.authorize(ctx, workspaceID, entity.ActionUpdate); err != nil {
		return nil, err
	}

	return s.domains.List(ctx, workspaceID)
}

func (s *domainsService) Add(ctx context.Context, workspaceID uuid.UUID, raw string) (entity.WorkspaceDomain, error) {
	if err := s.authorize(ctx, workspaceID, entity.ActionUpdate); err != nil {
		return entity.WorkspaceDomain{}, err
	}

	domain, problem := entity.ValidateWorkspaceDomain("domain", raw)
	if err := entity.NewValidationError(problem); err != nil {
		return entity.WorkspaceDomain{}, err
	}

	token, err := entity.NewWorkspaceDomainToken()
	if err != nil {
		return entity.WorkspaceDomain{}, err
	}

	added, err := s.domains.Add(ctx, entity.WorkspaceDomain{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		Domain:      domain,
		Token:       token,
	})
	if err != nil {
		return entity.WorkspaceDomain{}, err
	}

	s.record(ctx, entity.AuditSSODomainAdded, added)

	return added, nil
}

func (s *domainsService) Verify(ctx context.Context, workspaceID, id uuid.UUID) (entity.WorkspaceDomain, error) {
	if err := s.authorize(ctx, workspaceID, entity.ActionUpdate); err != nil {
		return entity.WorkspaceDomain{}, err
	}

	domain, err := s.domains.Get(ctx, workspaceID, id)
	if err != nil {
		return entity.WorkspaceDomain{}, err
	}

	published, err := s.ownership.PublishesTXT(ctx, domain.Domain, domain.Record())
	if err != nil {
		return entity.WorkspaceDomain{}, err
	}

	checked, err := s.domains.MarkChecked(ctx, workspaceID, id, published, time.Now().UTC())
	if err != nil {
		return entity.WorkspaceDomain{}, err
	}

	if checked.Verified() && !domain.Verified() {
		s.record(ctx, entity.AuditSSODomainVerified, checked)
	}

	return checked, nil
}

func (s *domainsService) Remove(ctx context.Context, workspaceID, id uuid.UUID) error {
	if err := s.authorize(ctx, workspaceID, entity.ActionDelete); err != nil {
		return err
	}

	domain, err := s.domains.Get(ctx, workspaceID, id)
	if err != nil {
		return err
	}

	if err := s.domains.Delete(ctx, workspaceID, id); err != nil {
		return err
	}

	s.record(ctx, entity.AuditSSODomainRemoved, domain)

	return nil
}
