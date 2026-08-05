package entity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func TestExpiryMovesThroughGraceRatherThanOffACliff(t *testing.T) {
	expiresAt := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	grace := 30 * 24 * time.Hour
	graceEnds := expiresAt.Add(grace)

	licence := entity.Licence{
		Holder:    "Northwind Studio",
		ExpiresAt: expiresAt,
		Features:  entity.LicenceFeatures{Audit: true},
	}

	cases := []struct {
		name    string
		now     time.Time
		status  entity.LicenceStatus
		permits bool
	}{
		{"well before expiry", expiresAt.Add(-90 * 24 * time.Hour), entity.LicenceActive, true},
		{"a second before expiry", expiresAt.Add(-time.Second), entity.LicenceActive, true},
		{"exactly at expiry", expiresAt, entity.LicenceGrace, true},
		{"a second after expiry", expiresAt.Add(time.Second), entity.LicenceGrace, true},
		{"midway through grace", expiresAt.Add(grace / 2), entity.LicenceGrace, true},
		{"a second before grace ends", graceEnds.Add(-time.Second), entity.LicenceGrace, true},
		{"exactly when grace ends", graceEnds, entity.LicenceExpired, false},
		{"a second after grace ends", graceEnds.Add(time.Second), entity.LicenceExpired, false},
		{"long after grace ends", graceEnds.Add(365 * 24 * time.Hour), entity.LicenceExpired, false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if status := licence.Status(testCase.now, grace); status != testCase.status {
				t.Errorf("status = %q, want %q", status, testCase.status)
			}

			if permits := licence.Permits(testCase.now, grace, entity.FeatureAudit); permits != testCase.permits {
				t.Errorf(
					"permits = %v, want %v. A renewal that lapses must not take a running "+
						"production instance down the same second.",
					permits, testCase.permits,
				)
			}
		})
	}
}

func TestGraceNeverResurrectsAFeatureTheLicenceNeverNamed(t *testing.T) {
	expiresAt := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	grace := 30 * 24 * time.Hour

	licence := entity.Licence{
		ExpiresAt: expiresAt,
		Features:  entity.LicenceFeatures{Audit: true},
	}

	if licence.Permits(expiresAt.Add(time.Hour), grace, entity.FeatureDirectory) {
		t.Error(
			"grace permitted directory synchronization, which this licence never covered. " +
				"Grace softens expiry; it does not widen the licence.",
		)
	}
}

func TestZeroGraceStopsAtExpiry(t *testing.T) {
	expiresAt := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)

	licence := entity.Licence{
		ExpiresAt: expiresAt,
		Features:  entity.LicenceFeatures{Audit: true},
	}

	if licence.Permits(expiresAt, 0, entity.FeatureAudit) {
		t.Error("with grace configured to zero the licence still permitted at expiry")
	}

	if status := licence.Status(expiresAt, 0); status != entity.LicenceExpired {
		t.Errorf("status with zero grace at expiry = %q, want expired", status)
	}
}

func TestAnUnlicensedErrorSaysWhichFeatureAndStillMatchesTheOldSentinels(t *testing.T) {
	cases := map[entity.Feature]error{
		entity.FeatureAudit:     entity.ErrAuditUnlicensed,
		entity.FeatureDirectory: entity.ErrDirectoryUnlicensed,
	}

	for feature, sentinel := range cases {
		err := feature.Unlicensed()

		var unlicensed entity.UnlicensedError

		if !errors.As(err, &unlicensed) || unlicensed.Feature != feature {
			t.Errorf("the error for %q does not carry its feature", feature)
		}

		if !errors.Is(err, sentinel) {
			t.Errorf(
				"the error for %q no longer matches its own sentinel, so the handler cannot "+
					"emit a code specific enough for a screen to say anything true",
				feature,
			)
		}

		if !errors.Is(err, entity.ErrUnlicensed) {
			t.Errorf(
				"the error for %q does not match ErrUnlicensed, so nothing can ask the generic "+
					"question 'is this feature simply not licensed here?'",
				feature,
			)
		}

		for other, otherSentinel := range cases {
			if other == feature {
				continue
			}

			if errors.Is(err, otherSentinel) {
				t.Errorf("the error for %q also reports as %q", feature, other)
			}
		}
	}
}
