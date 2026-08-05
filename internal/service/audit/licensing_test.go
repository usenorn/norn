package audit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	auditrepo "github.com/usenorn/norn/internal/repository/audit"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	"github.com/usenorn/norn/internal/service"
	auditsvc "github.com/usenorn/norn/internal/service/audit"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	licensingsvc "github.com/usenorn/norn/internal/service/licensing"
)

const grace = 30 * 24 * time.Hour

func licenceExpiring(in time.Duration) entity.Licence {
	return entity.Licence{
		Holder:    "Northwind Studio",
		ExpiresAt: time.Now().Add(in),
		Features:  entity.LicenceFeatures{Audit: true},
	}
}

func licensingFor(licence entity.Licence) service.Licensing {
	return licensingsvc.New(licence, config.Licence{Grace: grace})
}

func readerFor(t *testing.T, licence entity.Licence) service.AuditLog {
	t.Helper()

	ctrl := gomock.NewController(t)

	recorder := auditsvc.NewMockAudit(ctrl)
	recorder.EXPECT().Record(gomock.Any(), gomock.Any()).AnyTimes()

	return auditsvc.NewReader(
		auditrepo.NewMockAudit(ctrl),
		membershiprepo.NewMockMembership(ctrl),
		authorizersvc.NewMockAuthorizer(ctrl),
		recorder,
		licensingFor(licence),
		config.Audit{Retention: 365 * 24 * time.Hour},
	)
}

func TestTheAuditLogRefusesWithoutALicenceRatherThanReturningNothing(t *testing.T) {
	reader := readerFor(t, entity.Licence{})

	_, err := reader.List(context.Background(), service.AuditScope{WorkspaceID: uuid.New()}, service.ListAuditInput{})

	if !errors.Is(err, entity.ErrAuditUnlicensed) {
		t.Fatalf(
			"List error = %v, want ErrAuditUnlicensed. An empty page would read as "+
				"'nothing has happened here', which is a different and untrue statement.",
			err,
		)
	}

	if !errors.Is(err, entity.ErrUnlicensed) {
		t.Error("the refusal does not match the generic unlicensed sentinel")
	}

	export := reader.Export(
		context.Background(),
		service.AuditScope{WorkspaceID: uuid.New()},
		service.ListAuditInput{},
		func(entity.AuditEntry) error { return nil },
	)

	if !errors.Is(export, entity.ErrAuditUnlicensed) {
		t.Errorf("Export error = %v, want ErrAuditUnlicensed", export)
	}

	if available := reader.Availability(context.Background()); available.Available {
		t.Error("an unlicensed instance reports the audit log as available")
	}
}

func TestTheAuditLogKeepsAnsweringThroughGrace(t *testing.T) {
	reader := readerFor(t, licenceExpiring(-time.Hour))

	if available := reader.Availability(context.Background()); !available.Available {
		t.Fatal(
			"an hour past expiry the audit log already reports unavailable. A lapsed renewal " +
				"must not take a running production instance down the same second.",
		)
	}
}

func TestTheAuditLogStopsOnceGraceHasPassed(t *testing.T) {
	reader := readerFor(t, licenceExpiring(-grace-time.Hour))

	if available := reader.Availability(context.Background()); available.Available {
		t.Error("the audit log is still available well past the end of grace")
	}
}

func TestTheRetentionSweepRemovesNothingWhileNobodyCanRead(t *testing.T) {
	ctrl := gomock.NewController(t)

	events := auditrepo.NewMockAuditRetention(ctrl)
	events.EXPECT().
		DropBefore(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(0)

	sweeper := auditsvc.NewSweeper(
		events,
		licensingFor(entity.Licence{}),
		config.Audit{Retention: 365 * 24 * time.Hour, SweepBatch: 100},
	)

	swept, err := sweeper.Sweep(context.Background())
	if err != nil {
		t.Fatalf("Sweep error = %v", err)
	}

	if swept != 0 {
		t.Fatalf(
			"the sweep removed %d records on an unlicensed instance. Recording continues while "+
				"unlicensed, so pruning what nobody can read would leave a hole that re-licensing "+
				"can never fill.",
			swept,
		)
	}
}

func TestTheRetentionSweepResumesOnceLicensed(t *testing.T) {
	ctrl := gomock.NewController(t)

	events := auditrepo.NewMockAuditRetention(ctrl)
	events.EXPECT().
		DropBefore(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(0, nil).
		Times(1)

	sweeper := auditsvc.NewSweeper(
		events,
		licensingFor(licenceExpiring(24*time.Hour)),
		config.Audit{Retention: 365 * 24 * time.Hour, SweepBatch: 100},
	)

	if _, err := sweeper.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep error = %v", err)
	}
}
