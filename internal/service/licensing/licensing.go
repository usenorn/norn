package licensing

import (
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type licensingService struct {
	licence entity.Licence
	cfg     config.Licence
}

func New(licence entity.Licence, cfg config.Licence) service.Licensing {
	return &licensingService{licence: licence, cfg: cfg}
}

func (s *licensingService) Permits(feature entity.Feature) error {
	if s.licence.Permits(time.Now(), s.cfg.Grace, feature) {
		return nil
	}

	return feature.Unlicensed()
}

func (s *licensingService) Report() service.LicenceReport {
	now := time.Now()

	report := service.LicenceReport{
		Status:   s.licence.Status(now, s.cfg.Grace),
		Holder:   s.licence.Holder,
		Features: make([]service.FeatureState, 0, len(entity.Features())),
	}

	if s.licence.Present() {
		report.IssuedAt = s.licence.IssuedAt
		report.ExpiresAt = s.licence.ExpiresAt
		report.GraceEndsAt = s.licence.GraceEndsAt(s.cfg.Grace)
	}

	for _, feature := range entity.Features() {
		report.Features = append(report.Features, service.FeatureState{
			Name:    feature,
			Enabled: s.licence.Permits(now, s.cfg.Grace, feature),
		})
	}

	return report
}
