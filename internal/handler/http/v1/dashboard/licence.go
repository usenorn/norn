package dashboard

import (
	"context"
	"net/http"

	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) GetInstanceLicence(
	ctx context.Context,
	_ api.GetInstanceLicenceRequestObject,
) (api.GetInstanceLicenceResponseObject, error) {
	if _, signedIn := h.currentAccountID(ctx); !signedIn {
		return newProblem(http.StatusUnauthorized, "a valid session is required"), nil
	}

	return api.GetInstanceLicence200JSONResponse(licenceReportDTO(h.licensing.Report())), nil
}

func licenceReportDTO(report service.LicenceReport) api.LicenceReport {
	dto := api.LicenceReport{
		Status:   api.LicenceStatus(report.Status),
		Features: make([]api.LicenceFeature, 0, len(report.Features)),
	}

	if report.Holder != "" {
		dto.Holder = &report.Holder
	}

	if !report.IssuedAt.IsZero() {
		issuedAt := report.IssuedAt
		dto.IssuedAt = &issuedAt
	}

	if !report.ExpiresAt.IsZero() {
		expiresAt := report.ExpiresAt
		dto.ExpiresAt = &expiresAt
	}

	if !report.GraceEndsAt.IsZero() {
		graceEndsAt := report.GraceEndsAt
		dto.GraceEndsAt = &graceEndsAt
	}

	for _, feature := range report.Features {
		dto.Features = append(dto.Features, api.LicenceFeature{
			Name:    string(feature.Name),
			Enabled: feature.Enabled,
		})
	}

	return dto
}
