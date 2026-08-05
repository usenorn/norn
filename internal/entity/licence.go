package entity

import (
	"errors"
	"time"
)

var (
	ErrLicenceMalformed    = errors.New("licence key is malformed")
	ErrLicenceForged       = errors.New("licence key was not issued for this product")
	ErrUnlicensed          = errors.New("this feature is not available on this instance")
	ErrAuditUnlicensed     = errors.New("the audit log is not available on this instance")
	ErrDirectoryUnlicensed = errors.New("directory synchronization is not available on this instance")
)

type Feature string

const (
	FeatureAudit     Feature = "audit"
	FeatureDirectory Feature = "directory"
)

func Features() []Feature {
	return []Feature{FeatureAudit, FeatureDirectory}
}

func (f Feature) Valid() bool {
	return f == FeatureAudit || f == FeatureDirectory
}

func (f Feature) Unlicensed() error {
	return UnlicensedError{Feature: f}
}

type UnlicensedError struct {
	Feature Feature
}

func (e UnlicensedError) Error() string {
	return e.sentinel().Error()
}

func (e UnlicensedError) Is(target error) bool {
	return target == ErrUnlicensed || target == e.sentinel()
}

func (e UnlicensedError) sentinel() error {
	switch e.Feature {
	case FeatureAudit:
		return ErrAuditUnlicensed
	case FeatureDirectory:
		return ErrDirectoryUnlicensed
	default:
		return ErrUnlicensed
	}
}

type LicenceStatus string

const (
	LicenceAbsent  LicenceStatus = "absent"
	LicenceActive  LicenceStatus = "active"
	LicenceGrace   LicenceStatus = "grace"
	LicenceExpired LicenceStatus = "expired"
)

type LicenceFeatures struct {
	Audit     bool
	Directory bool
}

type Licence struct {
	Holder    string
	IssuedAt  time.Time
	ExpiresAt time.Time
	Features  LicenceFeatures
}

func (l Licence) Present() bool {
	return !l.ExpiresAt.IsZero()
}

func (l Licence) GraceEndsAt(grace time.Duration) time.Time {
	if !l.Present() {
		return time.Time{}
	}

	return l.ExpiresAt.Add(grace)
}

func (l Licence) Status(now time.Time, grace time.Duration) LicenceStatus {
	switch {
	case !l.Present():
		return LicenceAbsent
	case now.Before(l.ExpiresAt):
		return LicenceActive
	case now.Before(l.GraceEndsAt(grace)):
		return LicenceGrace
	default:
		return LicenceExpired
	}
}

func (l Licence) Enables(feature Feature) bool {
	switch feature {
	case FeatureAudit:
		return l.Features.Audit
	case FeatureDirectory:
		return l.Features.Directory
	default:
		return false
	}
}

func (l Licence) Permits(now time.Time, grace time.Duration, feature Feature) bool {
	switch l.Status(now, grace) {
	case LicenceActive, LicenceGrace:
		return l.Enables(feature)
	default:
		return false
	}
}
