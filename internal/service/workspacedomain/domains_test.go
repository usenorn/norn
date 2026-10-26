package workspacedomain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	domainownershiprepo "github.com/usenorn/norn/internal/repository/domainownership"
	workspacedomainrepo "github.com/usenorn/norn/internal/repository/workspacedomain"
	"github.com/usenorn/norn/internal/service"
	auditsvc "github.com/usenorn/norn/internal/service/audit"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	workspacedomainsvc "github.com/usenorn/norn/internal/service/workspacedomain"
)

type harness struct {
	domains   *workspacedomainrepo.MockWorkspaceDomain
	ownership *domainownershiprepo.MockDomainOwnership
	service   service.WorkspaceDomains

	admins  map[uuid.UUID]bool
	audited []entity.AuditEntry
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)
	authorizer := authorizersvc.NewMockAuthorizer(ctrl)
	audit := auditsvc.NewMockAudit(ctrl)

	h := &harness{
		domains:   workspacedomainrepo.NewMockWorkspaceDomain(ctrl),
		ownership: domainownershiprepo.NewMockDomainOwnership(ctrl),
		admins:    map[uuid.UUID]bool{},
	}

	authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			if request.Resource != entity.ResourceSSOConnection || !h.admins[request.WorkspaceID] {
				return entity.Decision{}, entity.ErrAccountForbidden
			}

			return entity.Decision{Role: entity.MembershipRoleAdmin}, nil
		}).
		AnyTimes()

	audit.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, entry entity.AuditEntry) { h.audited = append(h.audited, entry) }).
		AnyTimes()

	h.service = workspacedomainsvc.New(h.domains, h.ownership, authorizer, audit)

	return h
}

func (h *harness) actions() []entity.AuditAction {
	actions := make([]entity.AuditAction, 0, len(h.audited))
	for _, entry := range h.audited {
		actions = append(actions, entry.Action)
	}

	return actions
}

func TestAnAdminClaimsANormalisedDomainWithAFreshToken(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	h.admins[workspaceID] = true

	var stored entity.WorkspaceDomain

	h.domains.EXPECT().Add(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, domain entity.WorkspaceDomain) (entity.WorkspaceDomain, error) {
			stored = domain

			return domain, nil
		})

	added, err := h.service.Add(context.Background(), workspaceID, " Northwind.CO. ")
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	if stored.Domain != "northwind.co" || stored.WorkspaceID != workspaceID || stored.Token == "" || stored.ID == uuid.Nil {
		t.Fatalf("stored %+v", stored)
	}

	if added.Verified() {
		t.Error("a new claim was verified before anyone checked it")
	}

	if got := h.actions(); len(got) != 1 || got[0] != entity.AuditSSODomainAdded {
		t.Errorf("audited %v", got)
	}
}

func TestAPersonalMailDomainCannotBeClaimed(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	h.admins[workspaceID] = true

	_, err := h.service.Add(context.Background(), workspaceID, "gmail.com")

	var validation entity.ValidationError
	if !errors.As(err, &validation) || len(validation.Fields) != 1 ||
		validation.Fields[0].Code != entity.ValidationCodePersonalDomain {
		t.Fatalf("err = %v, want a personal_domain validation error", err)
	}

	if len(h.audited) != 0 {
		t.Errorf("a refused claim was audited: %v", h.actions())
	}
}

func TestOnlyAnAdminTouchesDomains(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	workspaceID := uuid.New()

	if _, err := h.service.List(ctx, workspaceID); !errors.Is(err, entity.ErrAccountForbidden) {
		t.Errorf("list: err = %v", err)
	}

	if _, err := h.service.Add(ctx, workspaceID, "northwind.co"); !errors.Is(err, entity.ErrAccountForbidden) {
		t.Errorf("add: err = %v", err)
	}

	if _, err := h.service.Verify(ctx, workspaceID, uuid.New()); !errors.Is(err, entity.ErrAccountForbidden) {
		t.Errorf("verify: err = %v", err)
	}

	if err := h.service.Remove(ctx, workspaceID, uuid.New()); !errors.Is(err, entity.ErrAccountForbidden) {
		t.Errorf("remove: err = %v", err)
	}
}

func TestADomainIsVerifiedOnlyWhenItPublishesItsRecord(t *testing.T) {
	cases := []struct {
		name         string
		published    bool
		wantVerified bool
		wantAudit    []entity.AuditAction
	}{
		{name: "the record is missing", published: false, wantVerified: false},
		{
			name:         "the record is published",
			published:    true,
			wantVerified: true,
			wantAudit:    []entity.AuditAction{entity.AuditSSODomainVerified},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			workspaceID := uuid.New()
			h.admins[workspaceID] = true
			claim := entity.WorkspaceDomain{ID: uuid.New(), WorkspaceID: workspaceID, Domain: "northwind.co", Token: "abc"}

			h.domains.EXPECT().Get(gomock.Any(), workspaceID, claim.ID).Return(claim, nil)
			h.ownership.EXPECT().PublishesTXT(gomock.Any(), "northwind.co", "norn-verification=abc").Return(tc.published, nil)
			h.domains.EXPECT().MarkChecked(gomock.Any(), workspaceID, claim.ID, tc.published, gomock.Any()).
				DoAndReturn(func(_ context.Context, _, _ uuid.UUID, verified bool, at time.Time) (entity.WorkspaceDomain, error) {
					checked := claim
					checked.CheckedAt = &at
					if verified {
						checked.VerifiedAt = &at
					}

					return checked, nil
				})

			checked, err := h.service.Verify(context.Background(), workspaceID, claim.ID)
			if err != nil {
				t.Fatalf("verify: %v", err)
			}

			if checked.Verified() != tc.wantVerified || checked.CheckedAt == nil {
				t.Errorf("verified %v checkedAt %v", checked.Verified(), checked.CheckedAt)
			}

			got := h.actions()
			if len(got) != len(tc.wantAudit) || (len(got) == 1 && got[0] != tc.wantAudit[0]) {
				t.Errorf("audited %v, want %v", got, tc.wantAudit)
			}
		})
	}
}

func TestAFailedLookupLeavesTheClaimUnchecked(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	h.admins[workspaceID] = true
	claim := entity.WorkspaceDomain{ID: uuid.New(), WorkspaceID: workspaceID, Domain: "northwind.co", Token: "abc"}

	h.domains.EXPECT().Get(gomock.Any(), workspaceID, claim.ID).Return(claim, nil)
	h.ownership.EXPECT().PublishesTXT(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(false, entity.ErrWorkspaceDomainLookupFailed)

	if _, err := h.service.Verify(context.Background(), workspaceID, claim.ID); !errors.Is(err, entity.ErrWorkspaceDomainLookupFailed) {
		t.Fatalf("err = %v, want ErrWorkspaceDomainLookupFailed", err)
	}
}

func TestRemovingAClaimIsAuditedWithItsDomain(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	h.admins[workspaceID] = true
	claim := entity.WorkspaceDomain{ID: uuid.New(), WorkspaceID: workspaceID, Domain: "northwind.co"}

	h.domains.EXPECT().Get(gomock.Any(), workspaceID, claim.ID).Return(claim, nil)
	h.domains.EXPECT().Delete(gomock.Any(), workspaceID, claim.ID).Return(nil)

	if err := h.service.Remove(context.Background(), workspaceID, claim.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if len(h.audited) != 1 || h.audited[0].Action != entity.AuditSSODomainRemoved ||
		h.audited[0].Detail["domain"] != "northwind.co" || h.audited[0].ResourceID != claim.ID {
		t.Fatalf("audited %+v", h.audited)
	}
}
