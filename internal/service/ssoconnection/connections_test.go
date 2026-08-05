package ssoconnection_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func stageOf(t *testing.T, err error) entity.SSOStage {
	t.Helper()

	failure, ok := entity.AsSSOError(err)
	if !ok {
		t.Fatalf("error %v is not an OIDC failure, so the screen cannot say where it broke", err)
	}

	return failure.Stage
}

func (h *harness) expectExchange(workspaceID uuid.UUID, provisioning bool, claims entity.OIDCClaims) {
	h.states.EXPECT().
		Take(gomock.Any(), "the-state").
		Return(entity.OIDCState{
			Purpose:     entity.SSOPurposeLogin,
			WorkspaceID: workspaceID,
			Nonce:       "the-nonce",
			Verifier:    "the-verifier",
		}, nil)

	h.workspaces.EXPECT().
		GetByID(gomock.Any(), workspaceID).
		Return(entity.Workspace{ID: workspaceID, Slug: "northwind"}, nil)

	h.connections.EXPECT().
		GetOIDC(gomock.Any(), workspaceID).
		Return(connection(workspaceID, provisioning), nil)

	h.provider.EXPECT().
		Exchange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(claims, nil)
}

func (h *harness) complete() (entity.SSOExchange, error) {
	return h.service.Complete(context.Background(), service.CompleteOIDCInput{
		State: "the-state",
		Code:  "the-code",
	})
}

func TestAMemberSigningInGetsASessionMarkedAsSSO(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	account := entity.Account{ID: uuid.New(), Status: entity.AccountStatusActive, Email: "ada@example.com"}

	h.expectExchange(workspaceID, false, verifiedClaims("Ada@Example.com"))
	h.accounts.EXPECT().GetByEmail(gomock.Any(), "ada@example.com").Return(account, nil)
	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, account.ID).
		Return(entity.Membership{WorkspaceID: workspaceID, AccountID: account.ID}, nil)

	var started service.StartSessionInput

	h.sessions.EXPECT().
		Start(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input service.StartSessionInput) (service.IssuedSession, error) {
			started = input

			return service.IssuedSession{
				Session: entity.Session{ID: uuid.New(), AccountID: input.AccountID, AuthMethod: input.AuthMethod},
				Token:   "session-token",
			}, nil
		})

	exchange, err := h.complete()
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if started.AuthMethod != entity.SessionAuthMethodSSO {
		t.Fatalf(
			"the session was started with auth method %q. A workspace that later requires SSO "+
				"reads this field, so a session minted here has to say so.",
			started.AuthMethod,
		)
	}

	if started.AccountID != account.ID {
		t.Fatalf("signed in as %s, want the matched account %s", started.AccountID, account.ID)
	}

	if exchange.Token != "session-token" {
		t.Fatal("the session token was not handed back, so no cookie can be set")
	}

	if exchange.Provisioned {
		t.Fatal("an existing member was reported as newly provisioned")
	}
}

func TestAnAccountThatIsNotAMemberIsTurnedAwayEvenThoughItExists(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	account := entity.Account{ID: uuid.New(), Status: entity.AccountStatusActive, Email: "ada@example.com"}

	h.expectExchange(workspaceID, true, verifiedClaims("ada@example.com"))
	h.accounts.EXPECT().GetByEmail(gomock.Any(), "ada@example.com").Return(account, nil)
	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, account.ID).
		Return(entity.Membership{}, entity.ErrMembershipNotFound)

	_, err := h.complete()
	if err == nil {
		t.Fatal(
			"someone who authenticated with the provider but was never invited to this " +
				"workspace was let in. Provisioning creates accounts; it must not widen membership.",
		)
	}

	if stage := stageOf(t, err); stage != entity.SSOStageMatching {
		t.Fatalf("refused at stage %q, want %q", stage, entity.SSOStageMatching)
	}

	if !strings.Contains(err.Error(), "ada@example.com") {
		t.Errorf("the refusal does not name the address: %v", err)
	}
}

func TestAnUnknownAddressIsRefusedWhenProvisioningIsOff(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()

	h.expectExchange(workspaceID, false, verifiedClaims("newcomer@example.com"))
	h.accounts.EXPECT().
		GetByEmail(gomock.Any(), "newcomer@example.com").
		Return(entity.Account{}, entity.ErrAccountNotFound)

	_, err := h.complete()
	if err == nil {
		t.Fatal("an unknown address was admitted with provisioning turned off")
	}

	if stage := stageOf(t, err); stage != entity.SSOStageProvisioning {
		t.Fatalf("refused at stage %q, want %q", stage, entity.SSOStageProvisioning)
	}
}

func TestAnUnknownAddressIsGivenAnAccountAndAMembershipWhenProvisioningIsOn(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	created := entity.Account{ID: uuid.New(), Status: entity.AccountStatusActive, Email: "newcomer@example.com"}

	h.expectExchange(workspaceID, true, verifiedClaims("NewComer@Example.com"))
	h.accounts.EXPECT().
		GetByEmail(gomock.Any(), "newcomer@example.com").
		Return(entity.Account{}, entity.ErrAccountNotFound)

	var madeAccount entity.Account

	h.accounts.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			madeAccount = account

			return created, nil
		})

	var madeMembership entity.Membership

	h.memberships.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, membership entity.Membership) (entity.Membership, error) {
			madeMembership = membership

			return membership, nil
		})

	h.sessions.EXPECT().
		Start(gomock.Any(), gomock.Any()).
		Return(service.IssuedSession{Session: entity.Session{ID: uuid.New()}, Token: "session-token"}, nil)

	exchange, err := h.complete()
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if madeAccount.Email != "newcomer@example.com" {
		t.Errorf("the account was created with email %q, want the normalised address", madeAccount.Email)
	}

	if madeAccount.PasswordHash != "" {
		t.Error(
			"a provisioned account was given a password hash. It authenticates through the " +
				"provider and has no password to set.",
		)
	}

	if madeAccount.Status != entity.AccountStatusActive {
		t.Errorf("the account was created with status %q, want active", madeAccount.Status)
	}

	if madeAccount.DisplayName != "Ada Lovelace" {
		t.Errorf("display name %q, want the name claim", madeAccount.DisplayName)
	}

	if madeMembership.WorkspaceID != workspaceID || madeMembership.AccountID != created.ID {
		t.Error("the new account was not admitted to the workspace it signed into")
	}

	if madeMembership.Role != entity.MembershipRoleMember {
		t.Errorf(
			"a provisioned member was given role %q. Provisioning must never hand out more "+
				"than the lowest ordinary role.",
			madeMembership.Role,
		)
	}

	if madeMembership.Source != entity.MembershipSourceManual {
		t.Errorf(
			"membership source %q, want manual. Signing in through a provider is authentication, "+
				"not provisioning by a directory: nothing syncs these members, so marking them "+
				"directory would lock an administrator out of a membership no directory governs. "+
				"Who arrived through the provider is told by the single sign-on identity link, "+
				"which the members screen already shows.",
			madeMembership.Source,
		)
	}

	if !exchange.Provisioned {
		t.Error("the exchange does not report that an account was created")
	}
}

func TestAProvisionedAccountFallsBackToTheAddressWhenTheProviderSendsNoName(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	claims := verifiedClaims("newcomer@example.com")
	claims.Name = ""

	h.expectExchange(workspaceID, true, claims)
	h.accounts.EXPECT().
		GetByEmail(gomock.Any(), "newcomer@example.com").
		Return(entity.Account{}, entity.ErrAccountNotFound)

	var made entity.Account

	h.accounts.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			made = account
			account.ID = uuid.New()

			return account, nil
		})
	h.memberships.EXPECT().Create(gomock.Any(), gomock.Any()).Return(entity.Membership{}, nil)
	h.sessions.EXPECT().
		Start(gomock.Any(), gomock.Any()).
		Return(service.IssuedSession{Token: "session-token"}, nil)

	if _, err := h.complete(); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if made.DisplayName != "newcomer" {
		t.Fatalf("display name %q, want the local part of the address", made.DisplayName)
	}
}

func TestADeactivatedAccountCannotComeBackInThroughTheProvider(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	account := entity.Account{
		ID:     uuid.New(),
		Status: entity.AccountStatusDeactivated,
		Email:  "ada@example.com",
	}

	h.expectExchange(workspaceID, true, verifiedClaims("ada@example.com"))
	h.accounts.EXPECT().GetByEmail(gomock.Any(), "ada@example.com").Return(account, nil)

	_, err := h.complete()
	if err == nil {
		t.Fatal(
			"a deactivated account signed in through the provider. Deactivating someone has to " +
				"close every door, not just the password one.",
		)
	}

	if stage := stageOf(t, err); stage != entity.SSOStageMatching {
		t.Fatalf("refused at stage %q, want %q", stage, entity.SSOStageMatching)
	}
}

func TestAnUnverifiedAddressNeverReachesTheAccountLookup(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	unverified := false
	claims := verifiedClaims("ada@example.com")
	claims.EmailVerified = &unverified

	h.expectExchange(workspaceID, true, claims)

	_, err := h.complete()
	if err == nil {
		t.Fatal("the provider said the address was unverified and Norn signed the person in anyway")
	}

	if stage := stageOf(t, err); stage != entity.SSOStageClaims {
		t.Fatalf("refused at stage %q, want %q", stage, entity.SSOStageClaims)
	}
}

func TestATestRoundTripRecordsVerificationAndMintsNoSession(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()

	h.states.EXPECT().
		Take(gomock.Any(), "the-state").
		Return(entity.OIDCState{
			Purpose:     entity.SSOPurposeTest,
			WorkspaceID: workspaceID,
			Nonce:       "the-nonce",
			Verifier:    "the-verifier",
		}, nil)

	h.workspaces.EXPECT().
		GetByID(gomock.Any(), workspaceID).
		Return(entity.Workspace{ID: workspaceID, Slug: "northwind"}, nil)

	h.connections.EXPECT().
		GetOIDC(gomock.Any(), workspaceID).
		Return(connection(workspaceID, false), nil)

	h.provider.EXPECT().
		Exchange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(verifiedClaims("admin@example.com"), nil)

	h.connections.EXPECT().MarkVerified(gomock.Any(), workspaceID, gomock.Any()).Return(nil)

	exchange, err := h.complete()
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if exchange.Token != "" || exchange.Session.ID != uuid.Nil {
		t.Fatal(
			"testing the connection signed the admin in. A test proves the provider works; it " +
				"is not an authentication event, and the admin is already signed in.",
		)
	}

	if exchange.Purpose != entity.SSOPurposeTest {
		t.Fatalf("purpose %q, want test", exchange.Purpose)
	}
}

func TestATestRoundTripDoesNotNeedTheAdminToBeAMember(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()

	h.states.EXPECT().
		Take(gomock.Any(), "the-state").
		Return(entity.OIDCState{Purpose: entity.SSOPurposeTest, WorkspaceID: workspaceID}, nil)
	h.workspaces.EXPECT().
		GetByID(gomock.Any(), workspaceID).
		Return(entity.Workspace{ID: workspaceID, Slug: "northwind"}, nil)
	h.connections.EXPECT().GetOIDC(gomock.Any(), workspaceID).Return(connection(workspaceID, false), nil)
	h.provider.EXPECT().
		Exchange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(verifiedClaims("someone-else@example.com"), nil)
	h.connections.EXPECT().MarkVerified(gomock.Any(), workspaceID, gomock.Any()).Return(nil)

	if _, err := h.complete(); err != nil {
		t.Fatalf(
			"a test failed because the identity that came back was not a workspace member: %v. "+
				"The admin may well authenticate as a different directory user than the one "+
				"they use in Norn.",
			err,
		)
	}
}

func TestAFailedExchangeDoesNotRecordVerification(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()

	h.states.EXPECT().
		Take(gomock.Any(), "the-state").
		Return(entity.OIDCState{Purpose: entity.SSOPurposeTest, WorkspaceID: workspaceID}, nil)
	h.workspaces.EXPECT().
		GetByID(gomock.Any(), workspaceID).
		Return(entity.Workspace{ID: workspaceID, Slug: "northwind"}, nil)
	h.connections.EXPECT().GetOIDC(gomock.Any(), workspaceID).Return(connection(workspaceID, false), nil)
	h.provider.EXPECT().
		Exchange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.OIDCClaims{}, entity.SSOFailure(
			entity.SSOStageTokenExchange,
			"The provider refused to exchange the sign-in code for a token.",
			errors.New("invalid_client: client secret mismatch"),
		))

	_, err := h.complete()
	if err == nil {
		t.Fatal("a failed exchange was reported as success")
	}

	if stage := stageOf(t, err); stage != entity.SSOStageTokenExchange {
		t.Fatalf("stage %q, want %q", stage, entity.SSOStageTokenExchange)
	}

	failure, _ := entity.AsSSOError(err)
	if !strings.Contains(failure.Detail, "client secret mismatch") {
		t.Fatalf(
			"detail %q loses the provider's own words, which is the only thing that tells an "+
				"admin which of a dozen settings is wrong",
			failure.Detail,
		)
	}
}

func TestAStateIsSpentOnFirstUse(t *testing.T) {
	h := newHarness(t)

	h.states.EXPECT().
		Take(gomock.Any(), "the-state").
		Return(entity.OIDCState{}, entity.ErrSSOStateNotFound)

	_, err := h.complete()
	if !errors.Is(err, entity.ErrSSOStateNotFound) {
		t.Fatalf(
			"replaying a spent state gave %v. Take must delete on read, so a captured callback "+
				"URL cannot be used twice.",
			err,
		)
	}
}

func TestBeginningALoginStoresEverythingTheCallbackWillNeed(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()

	h.workspaces.EXPECT().
		GetBySlug(gomock.Any(), "northwind").
		Return(entity.Workspace{ID: workspaceID, Slug: "northwind"}, nil)
	h.connections.EXPECT().GetOIDC(gomock.Any(), workspaceID).Return(connection(workspaceID, false), nil)

	var (
		stored entity.OIDCState
		key    string
	)

	h.states.EXPECT().
		Put(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, state string, attempt entity.OIDCState) error {
			key, stored = state, attempt

			return nil
		})

	var sent entity.OIDCAuthorization

	h.provider.EXPECT().
		AuthCodeURL(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.OIDCConnection, attempt entity.OIDCAuthorization) string {
			sent = attempt

			return "https://login.example.com/auth?state=" + attempt.State
		})

	url, err := h.service.BeginLogin(context.Background(), service.BeginOIDCLoginInput{
		WorkspaceSlug: "northwind",
		ReturnTo:      "/northwind/issues",
	})
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}

	if url == "" {
		t.Fatal("no authorization URL was produced")
	}

	if stored.Purpose != entity.SSOPurposeLogin {
		t.Errorf("stored purpose %q, want login", stored.Purpose)
	}

	if stored.WorkspaceID != workspaceID {
		t.Error("the stored state does not name the workspace, so the callback cannot find the provider")
	}

	if stored.ReturnTo != "/northwind/issues" {
		t.Errorf("return target %q was not kept", stored.ReturnTo)
	}

	if sent.State != key {
		t.Error("the state sent to the provider is not the one stored, so the callback will never match")
	}

	if sent.Nonce != stored.Nonce || sent.Verifier != stored.Verifier {
		t.Error("the nonce or PKCE verifier sent to the provider differs from the stored one")
	}

	if stored.Verifier == "" || stored.Nonce == "" {
		t.Fatal("PKCE verifier or nonce is empty, so neither protection is actually in force")
	}

	if stored.Verifier == stored.Nonce {
		t.Fatal("the nonce and the PKCE verifier are the same value")
	}
}

func TestTwoSignInAttemptsNeverShareAState(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()

	h.workspaces.EXPECT().
		GetBySlug(gomock.Any(), "northwind").
		Return(entity.Workspace{ID: workspaceID}, nil).
		Times(2)
	h.connections.EXPECT().
		GetOIDC(gomock.Any(), workspaceID).
		Return(connection(workspaceID, false), nil).
		Times(2)
	h.provider.EXPECT().AuthCodeURL(gomock.Any(), gomock.Any(), gomock.Any()).Return("url").Times(2)

	seen := make([]entity.OIDCState, 0, 2)
	keys := make([]string, 0, 2)

	h.states.EXPECT().
		Put(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, state string, attempt entity.OIDCState) error {
			keys = append(keys, state)
			seen = append(seen, attempt)

			return nil
		}).
		Times(2)

	for range 2 {
		if _, err := h.service.BeginLogin(context.Background(), service.BeginOIDCLoginInput{
			WorkspaceSlug: "northwind",
		}); err != nil {
			t.Fatalf("BeginLogin: %v", err)
		}
	}

	if keys[0] == keys[1] || seen[0].Nonce == seen[1].Nonce || seen[0].Verifier == seen[1].Verifier {
		t.Fatal(
			"two sign-in attempts produced the same state, nonce or verifier, so one person's " +
				"callback could complete another's attempt",
		)
	}
}

func TestASignInCannotBeSentBackToAnotherSite(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()

	for _, target := range []string{
		"https://evil.example.com/steal",
		"//evil.example.com/steal",
		"http://evil.example.com",
	} {
		h.workspaces.EXPECT().
			GetBySlug(gomock.Any(), "northwind").
			Return(entity.Workspace{ID: workspaceID}, nil)
		h.connections.EXPECT().GetOIDC(gomock.Any(), workspaceID).Return(connection(workspaceID, false), nil)
		h.provider.EXPECT().AuthCodeURL(gomock.Any(), gomock.Any(), gomock.Any()).Return("url")

		var stored entity.OIDCState

		h.states.EXPECT().
			Put(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ string, attempt entity.OIDCState) error {
				stored = attempt

				return nil
			})

		if _, err := h.service.BeginLogin(context.Background(), service.BeginOIDCLoginInput{
			WorkspaceSlug: "northwind",
			ReturnTo:      target,
		}); err != nil {
			t.Fatalf("BeginLogin: %v", err)
		}

		if stored.ReturnTo != "" {
			t.Errorf(
				"%q was kept as a return target. A signed-in user would land on someone else's "+
					"site straight out of the sign-in flow.",
				target,
			)
		}
	}
}

func TestSavingWithoutRetypingTheSecretKeepsTheStoredOne(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	h.allow(workspaceID)

	existing := connection(workspaceID, false)
	h.connections.EXPECT().GetOIDC(gomock.Any(), workspaceID).Return(existing, nil)

	var saved entity.OIDCConnection

	h.connections.EXPECT().
		SaveOIDC(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, c entity.OIDCConnection) (entity.OIDCConnection, error) {
			saved = c

			return c, nil
		})

	if _, err := h.service.Save(context.Background(), service.SaveOIDCConnectionInput{
		WorkspaceID:  workspaceID,
		Issuer:       existing.Endpoints.Issuer,
		Endpoints:    &existing.Endpoints,
		ClientID:     "norn",
		Provisioning: true,
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if saved.ClientSecret != existing.ClientSecret {
		t.Fatal(
			"editing a setting without retyping the secret wiped it. The API never returns the " +
				"secret, so the form cannot send it back and every edit would break the provider.",
		)
	}

	if !saved.Provisioning {
		t.Error("the edited setting was not saved")
	}
}

func TestAFirstSaveWithNoSecretIsRefusedRatherThanStoredEmpty(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	h.allow(workspaceID)

	h.connections.EXPECT().
		GetOIDC(gomock.Any(), workspaceID).
		Return(entity.OIDCConnection{}, entity.ErrSSOConnectionNotFound)

	endpoints := connection(workspaceID, false).Endpoints

	_, err := h.service.Save(context.Background(), service.SaveOIDCConnectionInput{
		WorkspaceID: workspaceID,
		Issuer:      endpoints.Issuer,
		Endpoints:   &endpoints,
		ClientID:    "norn",
	})
	if err == nil {
		t.Fatal("a connection was created with no client secret at all")
	}

	if stage := stageOf(t, err); stage != entity.SSOStageEndpoints {
		t.Fatalf("stage %q, want %q", stage, entity.SSOStageEndpoints)
	}
}

func TestTypingEndpointsInByHandSkipsDiscoveryAndIsRecordedAsManual(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	h.allow(workspaceID)

	endpoints := entity.OIDCEndpoints{
		Issuer:                "https://login.example.com",
		AuthorizationEndpoint: "https://login.example.com/custom/auth",
		TokenEndpoint:         "https://login.example.com/custom/token",
		JWKSURI:               "https://login.example.com/custom/certs",
	}

	var saved entity.OIDCConnection

	h.connections.EXPECT().
		SaveOIDC(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, c entity.OIDCConnection) (entity.OIDCConnection, error) {
			saved = c

			return c, nil
		})

	if _, err := h.service.Save(context.Background(), service.SaveOIDCConnectionInput{
		WorkspaceID:  workspaceID,
		Issuer:       endpoints.Issuer,
		Endpoints:    &endpoints,
		ClientID:     "norn",
		ClientSecret: "s3cr3t",
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if saved.Discovered {
		t.Error("hand-entered endpoints were recorded as discovered")
	}

	if saved.Endpoints.AuthorizationEndpoint != endpoints.AuthorizationEndpoint {
		t.Error("the hand-entered authorization endpoint was overwritten")
	}
}

func TestSavingWithoutEndpointsDiscoversThemFromTheIssuer(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	h.allow(workspaceID)

	discovered := connection(workspaceID, false).Endpoints

	h.provider.EXPECT().
		Discover(gomock.Any(), "https://login.example.com").
		Return(discovered, nil)

	var saved entity.OIDCConnection

	h.connections.EXPECT().
		SaveOIDC(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, c entity.OIDCConnection) (entity.OIDCConnection, error) {
			saved = c

			return c, nil
		})

	if _, err := h.service.Save(context.Background(), service.SaveOIDCConnectionInput{
		WorkspaceID:  workspaceID,
		Issuer:       "https://login.example.com",
		ClientID:     "norn",
		ClientSecret: "s3cr3t",
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if !saved.Discovered {
		t.Error("a discovered connection was not recorded as discovered")
	}

	if saved.Endpoints.TokenEndpoint != discovered.TokenEndpoint {
		t.Error("the discovered token endpoint was not stored")
	}
}

func TestADiscoveryFailureIsReportedRatherThanSavedHalfWay(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	h.allow(workspaceID)

	h.provider.EXPECT().
		Discover(gomock.Any(), gomock.Any()).
		Return(entity.OIDCEndpoints{}, entity.SSOFailure(
			entity.SSOStageDiscovery,
			"Norn could not read the discovery document at that issuer.",
			errors.New("404 Not Found"),
		))

	_, err := h.service.Save(context.Background(), service.SaveOIDCConnectionInput{
		WorkspaceID:  workspaceID,
		Issuer:       "https://login.example.com",
		ClientID:     "norn",
		ClientSecret: "s3cr3t",
	})
	if err == nil {
		t.Fatal("a connection was saved even though discovery failed")
	}

	if stage := stageOf(t, err); stage != entity.SSOStageDiscovery {
		t.Fatalf("stage %q, want %q", stage, entity.SSOStageDiscovery)
	}
}

func TestScopesAlwaysIncludeOpenidNoMatterWhatWasSubmitted(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	h.allow(workspaceID)

	endpoints := connection(workspaceID, false).Endpoints

	var saved entity.OIDCConnection

	h.connections.EXPECT().
		SaveOIDC(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, c entity.OIDCConnection) (entity.OIDCConnection, error) {
			saved = c

			return c, nil
		})

	if _, err := h.service.Save(context.Background(), service.SaveOIDCConnectionInput{
		WorkspaceID:  workspaceID,
		Issuer:       endpoints.Issuer,
		Endpoints:    &endpoints,
		ClientID:     "norn",
		ClientSecret: "s3cr3t",
		Scopes:       []string{"email"},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if len(saved.Scopes) == 0 || saved.Scopes[0] != "openid" {
		t.Fatalf(
			"scopes %v were saved without openid. Without it the provider returns no ID token "+
				"and every sign-in fails at the id_token stage.",
			saved.Scopes,
		)
	}
}

func TestBeginningALoginNeverAsksWhetherTheCallerMayDoIt(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.AccessRequest) (entity.Decision, error) {
			t.Error(
				"BeginLogin consulted the authorizer. Nobody is signed in yet, so any " +
					"authorization check here can only ever refuse and single sign-in would " +
					"be impossible to start.",
			)

			return entity.Decision{}, nil
		}).
		AnyTimes()

	h.workspaces.EXPECT().
		GetBySlug(gomock.Any(), "northwind").
		Return(entity.Workspace{ID: workspaceID}, nil)
	h.connections.EXPECT().GetOIDC(gomock.Any(), workspaceID).Return(connection(workspaceID, false), nil)
	h.states.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	h.provider.EXPECT().AuthCodeURL(gomock.Any(), gomock.Any(), gomock.Any()).Return("url")

	if _, err := h.service.BeginLogin(context.Background(), service.BeginOIDCLoginInput{
		WorkspaceSlug: "northwind",
	}); err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}
}

func TestAWorkspaceWithNoProviderLooksTheSameAsOneThatDoesNotExist(t *testing.T) {
	h := newHarness(t)

	h.workspaces.EXPECT().
		GetBySlug(gomock.Any(), "nowhere").
		Return(entity.Workspace{}, entity.ErrWorkspaceNotFound)

	_, err := h.service.BeginLogin(context.Background(), service.BeginOIDCLoginInput{
		WorkspaceSlug: "nowhere",
	})

	if !errors.Is(err, entity.ErrSSOConnectionNotFound) {
		t.Fatalf(
			"a missing workspace gave %v rather than the same answer as a workspace with no "+
				"provider. An unauthenticated caller must not be able to enumerate slugs here.",
			err,
		)
	}
}

func TestAnExchangeSaysWhichWorkspaceItBelongedTo(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	account := entity.Account{ID: uuid.New(), Status: entity.AccountStatusActive, Email: "ada@example.com"}

	h.expectExchange(workspaceID, false, verifiedClaims("ada@example.com"))
	h.accounts.EXPECT().GetByEmail(gomock.Any(), "ada@example.com").Return(account, nil)
	h.memberships.EXPECT().Get(gomock.Any(), workspaceID, account.ID).Return(entity.Membership{}, nil)
	h.sessions.EXPECT().
		Start(gomock.Any(), gomock.Any()).
		Return(service.IssuedSession{Token: "session-token"}, nil)

	exchange, err := h.complete()
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if exchange.WorkspaceSlug != "northwind" {
		t.Fatalf(
			"the exchange came back with slug %q. Every screen a person lands on afterwards is "+
				"under /<workspace>/, so without the slug the callback can only redirect to a "+
				"route that does not exist.",
			exchange.WorkspaceSlug,
		)
	}
}

func TestAFailedExchangeStillSaysWhichWorkspaceItWasFor(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	unverified := false
	claims := verifiedClaims("mallory@example.com")
	claims.EmailVerified = &unverified

	h.expectExchange(workspaceID, true, claims)

	exchange, err := h.complete()
	if err == nil {
		t.Fatal("an unverified address was admitted")
	}

	if exchange.WorkspaceSlug != "northwind" {
		t.Fatalf(
			"a failed exchange came back with slug %q. The failure screen offers Try again, and "+
				"without the workspace that link cannot point back at the same provider.",
			exchange.WorkspaceSlug,
		)
	}
}
