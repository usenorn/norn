package apitoken_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func expiring(in time.Duration, notified *int) entity.APIToken {
	expiresAt := time.Now().UTC().Add(in)

	return entity.APIToken{
		ID:               uuid.New(),
		AccountID:        uuid.New(),
		Name:             "CI pipeline",
		Scopes:           readScopes(),
		Grants:           wholeWorkspace(uuid.New()),
		ExpiresAt:        &expiresAt,
		ExpiryNoticeDays: notified,
	}
}

func (h *harness) expectOwnerAndWorkspace(token entity.APIToken) {
	h.accounts.EXPECT().
		GetByID(gomock.Any(), token.AccountID).
		Return(entity.Account{
			ID:     token.AccountID,
			Email:  "rae@northwind.co",
			Status: entity.AccountStatusActive,
		}, nil).
		AnyTimes()

	h.workspaces.EXPECT().
		GetByID(gomock.Any(), gomock.Any()).
		Return(entity.Workspace{Name: "Northwind Studio"}, nil).
		AnyTimes()
}

func TestATokenNearingExpiryWarnsItsOwnerOncePerThreshold(t *testing.T) {
	h := newHarness(t)

	token := expiring(6*24*time.Hour, nil)
	h.expectOwnerAndWorkspace(token)

	h.tokens.EXPECT().ListExpiring(gomock.Any(), gomock.Any()).Return([]entity.APIToken{token}, nil)

	var sent entity.Mail

	h.mailer.EXPECT().
		Send(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, mail entity.Mail) error {
			sent = mail

			return nil
		})

	var recorded int

	h.tokens.EXPECT().
		RecordExpiryNotice(gomock.Any(), token.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, days int) error {
			recorded = days

			return nil
		})

	if err := h.service.SweepExpiring(context.Background()); err != nil {
		t.Fatalf("SweepExpiring: %v", err)
	}

	if recorded != 7 {
		t.Fatalf("recorded threshold = %d, want 7", recorded)
	}

	if !strings.Contains(sent.PlainBody, token.Name) {
		t.Error("the warning does not name the token, so its owner cannot tell which one to replace")
	}

	if sent.To != "rae@northwind.co" {
		t.Errorf("warning sent to %q, want the token's owner", sent.To)
	}
}

func TestATokenAlreadyWarnedAtAThresholdIsNotWarnedAgain(t *testing.T) {
	h := newHarness(t)

	already := 7
	token := expiring(6*24*time.Hour, &already)
	h.expectOwnerAndWorkspace(token)

	h.tokens.EXPECT().ListExpiring(gomock.Any(), gomock.Any()).Return([]entity.APIToken{token}, nil)

	if err := h.service.SweepExpiring(context.Background()); err != nil {
		t.Fatalf("SweepExpiring: %v", err)
	}
}

func TestCrossingATighterThresholdWarnsAgain(t *testing.T) {
	h := newHarness(t)

	already := 7
	token := expiring(20*time.Hour, &already)
	h.expectOwnerAndWorkspace(token)

	h.tokens.EXPECT().ListExpiring(gomock.Any(), gomock.Any()).Return([]entity.APIToken{token}, nil)
	h.mailer.EXPECT().Send(gomock.Any(), gomock.Any()).Return(nil)

	var recorded int

	h.tokens.EXPECT().
		RecordExpiryNotice(gomock.Any(), token.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, days int) error {
			recorded = days

			return nil
		})

	if err := h.service.SweepExpiring(context.Background()); err != nil {
		t.Fatalf("SweepExpiring: %v", err)
	}

	if recorded != 1 {
		t.Fatalf("recorded threshold = %d, want 1 now that the token has a day left", recorded)
	}
}

func TestAFailedWarningIsNotRecordedSoTheNextSweepRetries(t *testing.T) {
	h := newHarness(t)

	token := expiring(6*24*time.Hour, nil)
	h.expectOwnerAndWorkspace(token)

	h.tokens.EXPECT().ListExpiring(gomock.Any(), gomock.Any()).Return([]entity.APIToken{token}, nil)
	h.mailer.EXPECT().Send(gomock.Any(), gomock.Any()).Return(errors.New("smtp is down"))

	err := h.service.SweepExpiring(context.Background())
	if err == nil {
		t.Fatal("a warning that could not be sent was reported as a clean sweep")
	}

	// No RecordExpiryNotice expectation: recording it would mark the owner as warned about
	// something they were never told, and no later sweep would ever try again.
}

func TestOneUnreachableOwnerDoesNotStopTheRestBeingWarned(t *testing.T) {
	h := newHarness(t)

	failing := expiring(6*24*time.Hour, nil)
	healthy := expiring(6*24*time.Hour, nil)

	h.accounts.EXPECT().
		GetByID(gomock.Any(), gomock.Any()).
		Return(entity.Account{Email: "rae@northwind.co", Status: entity.AccountStatusActive}, nil).
		AnyTimes()
	h.workspaces.EXPECT().
		GetByID(gomock.Any(), gomock.Any()).
		Return(entity.Workspace{Name: "Northwind Studio"}, nil).
		AnyTimes()

	h.tokens.EXPECT().
		ListExpiring(gomock.Any(), gomock.Any()).
		Return([]entity.APIToken{failing, healthy}, nil)

	gomock.InOrder(
		h.mailer.EXPECT().Send(gomock.Any(), gomock.Any()).Return(errors.New("smtp is down")),
		h.mailer.EXPECT().Send(gomock.Any(), gomock.Any()).Return(nil),
	)

	h.tokens.EXPECT().RecordExpiryNotice(gomock.Any(), healthy.ID, 7).Return(nil)

	if err := h.service.SweepExpiring(context.Background()); err == nil {
		t.Fatal("the sweep hid a failure it should have reported")
	}
}

func TestADeactivatedOwnerIsNotMailedAboutTheirToken(t *testing.T) {
	h := newHarness(t)

	token := expiring(6*24*time.Hour, nil)

	h.accounts.EXPECT().
		GetByID(gomock.Any(), token.AccountID).
		Return(entity.Account{ID: token.AccountID, Status: entity.AccountStatusDeactivated}, nil)

	h.tokens.EXPECT().ListExpiring(gomock.Any(), gomock.Any()).Return([]entity.APIToken{token}, nil)
	h.tokens.EXPECT().RecordExpiryNotice(gomock.Any(), token.ID, 7).Return(nil)

	if err := h.service.SweepExpiring(context.Background()); err != nil {
		t.Fatalf("SweepExpiring: %v", err)
	}
}
