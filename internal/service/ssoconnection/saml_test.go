package ssoconnection_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/samlkey"
	"github.com/usenorn/norn/internal/service"
)

func samlConnection(t *testing.T, workspaceID uuid.UUID, idpInitiated, provisioning bool) entity.SAMLConnection {
	t.Helper()

	pair, err := samlkey.Generate("https://norn.example.com/v1/sso/saml/northwind/metadata", time.Now())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	return entity.SAMLConnection{
		WorkspaceID: workspaceID,
		Descriptor: entity.SAMLDescriptor{
			EntityID:     "https://login.example.com/realms/norn",
			SSOURL:       "https://login.example.com/protocol/saml",
			Certificates: []string{samlkey.MarshalCertificate(pair.Certificate)},
			ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
		},
		SPEntityID:        "https://norn.example.com/v1/sso/saml/northwind/metadata",
		SPPrivateKey:      samlkey.MarshalPrivateKey(pair.PrivateKey),
		SPCertificate:     samlkey.MarshalCertificate(pair.Certificate),
		AllowIDPInitiated: idpInitiated,
		Provisioning:      provisioning,
	}
}

func (h *harness) expectWorkspace(workspaceID uuid.UUID) {
	workspace := entity.Workspace{ID: workspaceID, Slug: "northwind", Name: "Northwind"}
	h.workspaces.EXPECT().GetBySlug(gomock.Any(), "northwind").Return(workspace, nil).AnyTimes()
	h.workspaces.EXPECT().GetByID(gomock.Any(), workspaceID).Return(workspace, nil).AnyTimes()
}

func TestAProviderInitiatedSignInIsRefusedUnlessItWasTurnedOn(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	h.expectWorkspace(workspaceID)
	h.connections.EXPECT().
		GetSAML(gomock.Any(), workspaceID).
		Return(samlConnection(t, workspaceID, false, false), nil)

	_, err := h.service.CompleteSAML(context.Background(), service.CompleteSAMLInput{
		WorkspaceSlug: "northwind",
		Response:      []byte("<Response/>"),
	})
	if err == nil {
		t.Fatal(
			"an unsolicited assertion was accepted on a workspace that never turned provider-" +
				"initiated sign-in on. Norn has no request of its own to tie it to, so accepting " +
				"it by default would widen the attack surface without anyone choosing to.",
		)
	}

	if stage := stageOf(t, err); stage != entity.SSOStageResponse {
		t.Fatalf("stage %q, want %q", stage, entity.SSOStageResponse)
	}
}

func TestAProviderInitiatedSignInIsAttemptedOnceItIsTurnedOn(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	h.expectWorkspace(workspaceID)
	h.connections.EXPECT().
		GetSAML(gomock.Any(), workspaceID).
		Return(samlConnection(t, workspaceID, true, false), nil)

	_, err := h.service.CompleteSAML(context.Background(), service.CompleteSAMLInput{
		WorkspaceSlug: "northwind",
		Response:      []byte("<Response/>"),
	})
	if err == nil {
		t.Fatal("a malformed unsolicited assertion was accepted")
	}

	if stage := stageOf(t, err); stage == entity.SSOStageResponse {
		failure, _ := entity.AsSSOError(err)
		if failure.Message == "This workspace does not accept sign-ins started at the provider. Start from Norn instead." {
			t.Fatal(
				"provider-initiated sign-in is still refused outright even though the workspace " +
					"turned it on",
			)
		}
	}
}

func TestASpentRelayStateOnAWorkspaceWithoutIDPInitiatedIsNotSilentlyDowngraded(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	h.expectWorkspace(workspaceID)
	h.connections.EXPECT().
		GetSAML(gomock.Any(), workspaceID).
		Return(samlConnection(t, workspaceID, false, false), nil)
	h.requests.EXPECT().
		Take(gomock.Any(), "spent").
		Return(entity.SAMLAttempt{}, entity.ErrSSOStateNotFound)

	_, err := h.service.CompleteSAML(context.Background(), service.CompleteSAMLInput{
		WorkspaceSlug: "northwind",
		RelayState:    "spent",
		Response:      []byte("<Response/>"),
	})

	if !errors.Is(err, entity.ErrSSOStateNotFound) {
		t.Fatalf(
			"a replayed relay state gave %v. Without provider-initiated sign-in enabled it must "+
				"stay a spent-state refusal rather than quietly becoming an unsolicited sign-in.",
			err,
		)
	}
}

func TestPublishedMetadataCarriesNornsCertificateAndCallbackAddress(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	h.expectWorkspace(workspaceID)
	h.connections.EXPECT().
		GetSAML(gomock.Any(), workspaceID).
		Return(samlConnection(t, workspaceID, false, false), nil)

	document, err := h.service.PublishSAMLMetadata(context.Background(), "northwind")
	if err != nil {
		t.Fatalf("PublishSAMLMetadata: %v", err)
	}

	metadata := string(document)

	for _, needle := range []string{
		"EntityDescriptor",
		"SPSSODescriptor",
		"AssertionConsumerService",
		"https://norn.example.com/v1/sso/saml/northwind/acs",
		"X509Certificate",
	} {
		if !strings.Contains(metadata, needle) {
			t.Errorf("the published metadata has no %s, so a provider could not register Norn", needle)
		}
	}

	if strings.Contains(metadata, "PRIVATE KEY") {
		t.Fatal("the published metadata contains the private key")
	}
}

func TestSavingASAMLProviderKeepsTheKeypairItAlreadyPublished(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	h.allow(workspaceID)
	h.expectWorkspace(workspaceID)

	existing := samlConnection(t, workspaceID, false, false)
	h.connections.EXPECT().GetSAML(gomock.Any(), workspaceID).Return(existing, nil)

	var saved entity.SAMLConnection

	h.connections.EXPECT().
		SaveSAML(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, c entity.SAMLConnection) (entity.SAMLConnection, error) {
			saved = c

			return c, nil
		})

	descriptor := existing.Descriptor

	if _, err := h.service.SaveSAML(context.Background(), service.SaveSAMLConnectionInput{
		WorkspaceID: workspaceID,
		Descriptor:  &descriptor,
	}); err != nil {
		t.Fatalf("SaveSAML: %v", err)
	}

	if string(saved.SPPrivateKey) != string(existing.SPPrivateKey) {
		t.Fatal(
			"editing a setting regenerated Norn's keypair. The certificate is already registered " +
				"at the provider, so rotating it on every save would break the connection each time.",
		)
	}
}

func TestAFirstSaveGeneratesAKeypairSoMetadataCanBePublished(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	h.allow(workspaceID)
	h.expectWorkspace(workspaceID)

	h.connections.EXPECT().
		GetSAML(gomock.Any(), workspaceID).
		Return(entity.SAMLConnection{}, entity.ErrSSOConnectionNotFound)

	var saved entity.SAMLConnection

	h.connections.EXPECT().
		SaveSAML(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, c entity.SAMLConnection) (entity.SAMLConnection, error) {
			saved = c

			return c, nil
		})

	descriptor := samlConnection(t, workspaceID, false, false).Descriptor

	if _, err := h.service.SaveSAML(context.Background(), service.SaveSAMLConnectionInput{
		WorkspaceID: workspaceID,
		Descriptor:  &descriptor,
	}); err != nil {
		t.Fatalf("SaveSAML: %v", err)
	}

	if len(saved.SPPrivateKey) == 0 || saved.SPCertificate == "" {
		t.Fatal("no keypair was generated, so Norn has no metadata to hand the provider")
	}

	if saved.SPEntityID != "https://norn.example.com/v1/sso/saml/northwind/metadata" {
		t.Fatalf("entity id %q, want the workspace-scoped metadata address", saved.SPEntityID)
	}
}

func TestAnExpiringCertificateWarnsEveryAdminOnceAndRecordsThat(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	h.expectWorkspace(workspaceID)

	connection := samlConnection(t, workspaceID, false, false)
	connection.Descriptor.ExpiresAt = time.Now().Add(6 * 24 * time.Hour)

	h.connections.EXPECT().
		ListSAMLCertificates(gomock.Any()).
		Return([]entity.SAMLConnection{connection}, nil)

	h.memberships.EXPECT().
		ListPageByWorkspaceID(gomock.Any(), workspaceID, gomock.Any()).
		Return([]entity.WorkspaceMember{
			{Membership: entity.Membership{Role: entity.MembershipRoleAdmin}, Email: "boss@example.com"},
			{Membership: entity.Membership{Role: entity.MembershipRoleAdmin}, Email: "other@example.com"},
			{Membership: entity.Membership{Role: entity.MembershipRoleMember}, Email: "dev@example.com"},
		}, nil)

	sent := make([]entity.Mail, 0)

	h.mailer.EXPECT().
		Send(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, mail entity.Mail) error {
			sent = append(sent, mail)

			return nil
		}).
		Times(2)

	var recorded int

	h.connections.EXPECT().
		RecordExpiryNotice(gomock.Any(), workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, days int) error {
			recorded = days

			return nil
		})

	if err := h.service.SweepCertificates(context.Background()); err != nil {
		t.Fatalf("SweepCertificates: %v", err)
	}

	if len(sent) != 2 {
		t.Fatalf("%d messages sent, want one per administrator and none to ordinary members", len(sent))
	}

	for _, mail := range sent {
		if mail.To == "dev@example.com" {
			t.Fatal("an ordinary member was warned; only administrators can act on this")
		}

		if !strings.Contains(mail.PlainBody, "/northwind/settings/authentication") {
			t.Error("the message does not link to the screen that fixes it")
		}
	}

	if recorded != 7 {
		t.Fatalf("recorded threshold %d, want 7 so the next sweep does not send again", recorded)
	}
}

func TestASweepSaysNothingWhenNoCertificateIsNear(t *testing.T) {
	h := newHarness(t)

	connection := samlConnection(t, uuid.New(), false, false)
	connection.Descriptor.ExpiresAt = time.Now().Add(200 * 24 * time.Hour)

	h.connections.EXPECT().
		ListSAMLCertificates(gomock.Any()).
		Return([]entity.SAMLConnection{connection}, nil)

	if err := h.service.SweepCertificates(context.Background()); err != nil {
		t.Fatalf("SweepCertificates: %v", err)
	}
}

func TestASweepDoesNotWarnTwiceAtTheSameThreshold(t *testing.T) {
	h := newHarness(t)

	already := 7
	connection := samlConnection(t, uuid.New(), false, false)
	connection.Descriptor.ExpiresAt = time.Now().Add(5 * 24 * time.Hour)
	connection.ExpiryNoticeDays = &already

	h.connections.EXPECT().
		ListSAMLCertificates(gomock.Any()).
		Return([]entity.SAMLConnection{connection}, nil)

	if err := h.service.SweepCertificates(context.Background()); err != nil {
		t.Fatalf("SweepCertificates: %v", err)
	}
}

func TestSigningInThroughTheProviderLinksTheAccountToThatIdentity(t *testing.T) {
	h := newHarnessWithoutLinking(t)

	workspaceID := uuid.New()
	account := entity.Account{ID: uuid.New(), Status: entity.AccountStatusActive, Email: "ada@example.com"}

	h.expectExchange(workspaceID, false, verifiedClaims("ada@example.com"))
	h.accounts.EXPECT().GetByEmail(gomock.Any(), "ada@example.com").Return(account, nil)
	h.memberships.EXPECT().Get(gomock.Any(), workspaceID, account.ID).Return(entity.Membership{}, nil)
	h.sessions.EXPECT().
		Start(gomock.Any(), gomock.Any()).
		Return(service.IssuedSession{Token: "session-token"}, nil)

	h.identities.EXPECT().
		Get(gomock.Any(), workspaceID, account.ID).
		Return(entity.SSOIdentity{}, entity.ErrSSOIdentityNotFound)

	var linked entity.SSOIdentity

	h.identities.EXPECT().
		Link(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, identity entity.SSOIdentity) error {
			linked = identity

			return nil
		})

	if _, err := h.complete(); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if linked.AccountID != account.ID || linked.WorkspaceID != workspaceID {
		t.Fatal("the link was recorded against the wrong account or workspace")
	}

	if linked.Subject != "provider-subject" {
		t.Fatalf(
			"the link stored subject %q. Matching by email alone means whoever can get that "+
				"address issued at the provider inherits the account; the subject is what makes "+
				"the binding durable.",
			linked.Subject,
		)
	}
}

func TestADifferentProviderIdentityIsRefusedForAnAlreadyLinkedAccount(t *testing.T) {
	h := newHarnessWithoutLinking(t)

	workspaceID := uuid.New()
	account := entity.Account{ID: uuid.New(), Status: entity.AccountStatusActive, Email: "ada@example.com"}

	h.expectExchange(workspaceID, false, verifiedClaims("ada@example.com"))
	h.accounts.EXPECT().GetByEmail(gomock.Any(), "ada@example.com").Return(account, nil)
	h.memberships.EXPECT().Get(gomock.Any(), workspaceID, account.ID).Return(entity.Membership{}, nil)

	h.identities.EXPECT().
		Get(gomock.Any(), workspaceID, account.ID).
		Return(entity.SSOIdentity{
			WorkspaceID: workspaceID,
			AccountID:   account.ID,
			Subject:     "somebody-else",
		}, nil)

	_, err := h.complete()
	if err == nil {
		t.Fatal(
			"a provider identity that is not the linked one was accepted. An identity-provider " +
				"rebuild, or somebody newly holding that email address, would silently take over " +
				"the Norn account.",
		)
	}

	if stage := stageOf(t, err); stage != entity.SSOStageMatching {
		t.Fatalf("stage %q, want %q", stage, entity.SSOStageMatching)
	}
}
