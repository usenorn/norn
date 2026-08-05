package service

import (
	"time"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=licensings.go -destination=licensing/mock_licensings.go -package=licensing -mock_names=Licensing=MockLicensing

type FeatureState struct {
	Name    entity.Feature
	Enabled bool
}

type LicenceReport struct {
	Status      entity.LicenceStatus
	Holder      string
	IssuedAt    time.Time
	ExpiresAt   time.Time
	GraceEndsAt time.Time
	Features    []FeatureState
}

type Licensing interface {
	Permits(feature entity.Feature) error
	Report() LicenceReport
}
