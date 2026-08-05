package entity

import (
	"errors"
	"time"
)

var (
	ErrLicenceMalformed    = errors.New("licence key is malformed")
	ErrLicenceForged       = errors.New("licence key was not issued for this product")
	ErrAuditUnlicensed     = errors.New("the audit log is not available on this instance")
	ErrDirectoryUnlicensed = errors.New("directory synchronization is not available on this instance")
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

func (l Licence) Valid(now time.Time) bool {
	return !l.ExpiresAt.IsZero() && now.Before(l.ExpiresAt)
}

func (l Licence) Permits(now time.Time, feature func(LicenceFeatures) bool) bool {
	return l.Valid(now) && feature(l.Features)
}

func AuditFeature(features LicenceFeatures) bool {
	return features.Audit
}

func DirectoryFeature(features LicenceFeatures) bool {
	return features.Directory
}
