package dashboard

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/usenorn/norn/internal/entity"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

const (
	problemContentType = "application/problem+json"
	problemType        = "about:blank"
	retryAfterHeader   = "Retry-After"
)

type problemResponse struct {
	status     int
	body       any
	retryAfter int
}

func baseProblem(status int, detail string) api.Problem {
	problem := api.Problem{
		Type:   problemType,
		Title:  http.StatusText(status),
		Status: int32(status),
	}

	if detail != "" {
		problem.Detail = &detail
	}

	return problem
}

func newProblem(status int, detail string) problemResponse {
	return problemResponse{status: status, body: baseProblem(status, detail)}
}

func (r problemResponse) write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", problemContentType)

	if r.retryAfter > 0 {
		w.Header().Set(retryAfterHeader, strconv.Itoa(r.retryAfter))
	}

	w.WriteHeader(r.status)

	return json.NewEncoder(w).Encode(r.body)
}

func problemFor(err error) (problemResponse, bool) {
	var validation entity.ValidationError
	if errors.As(err, &validation) {
		problem := baseProblem(http.StatusUnprocessableEntity, validation.Error())

		fields := make([]api.FieldError, len(validation.Fields))
		for i, field := range validation.Fields {
			fields[i] = api.FieldError{Field: field.Field, Code: field.Code}
		}

		problem.Errors = &fields

		return problemResponse{status: http.StatusUnprocessableEntity, body: problem}, true
	}

	var lastAdmin entity.LastWorkspaceAdminError
	if errors.As(err, &lastAdmin) {
		return newProblem(http.StatusConflict, lastAdmin.Error()), true
	}

	var locked entity.AccountLockedError
	if errors.As(err, &locked) {
		base := baseProblem(http.StatusLocked, locked.Error())

		return problemResponse{
			status: http.StatusLocked,
			body: api.AccountLockedProblem{
				Code:      api.AccountLockedProblemCodeAccountLocked,
				Detail:    base.Detail,
				Instance:  base.Instance,
				Status:    base.Status,
				Title:     base.Title,
				Type:      base.Type,
				UnlocksAt: locked.UnlocksAt,
			},
		}, true
	}

	var invalid entity.InvalidCredentialsError
	if errors.As(err, &invalid) {
		base := baseProblem(http.StatusUnauthorized, invalid.Error())

		return problemResponse{
			status: http.StatusUnauthorized,
			body: api.InvalidCredentialsProblem{
				AttemptsLeft: int32(invalid.AttemptsLeft),
				Code:         api.InvalidCredentials,
				Detail:       base.Detail,
				Instance:     base.Instance,
				Status:       base.Status,
				Title:        base.Title,
				Type:         base.Type,
			},
		}, true
	}

	switch {
	case errors.Is(err, entity.ErrAccountNotFound),
		errors.Is(err, entity.ErrEmailChangeNotFound),
		errors.Is(err, entity.ErrWorkspaceNotFound),
		errors.Is(err, entity.ErrMembershipNotFound),
		errors.Is(err, entity.ErrAvatarMissing):
		return newProblem(http.StatusNotFound, err.Error()), true

	case errors.Is(err, entity.ErrAccountForbidden),
		errors.Is(err, entity.ErrWorkspaceAuthMethodNotPermitted):
		return newProblem(http.StatusForbidden, err.Error()), true

	case errors.Is(err, entity.ErrAccountInvalidCredentials),
		errors.Is(err, entity.ErrSessionRevoked):
		return newProblem(http.StatusUnauthorized, err.Error()), true

	case errors.Is(err, entity.ErrSessionNotFound):
		return newProblem(http.StatusNotFound, err.Error()), true

	case errors.Is(err, entity.ErrAccountEmailTaken),
		errors.Is(err, entity.ErrWorkspaceSlugTaken),
		errors.Is(err, entity.ErrMembershipExists),
		errors.Is(err, entity.ErrEmailChangePending),
		errors.Is(err, entity.ErrEmailChangeAlreadyDone),
		errors.Is(err, entity.ErrAccountPasswordSet),
		errors.Is(err, entity.ErrAccountPasswordNotSet),
		errors.Is(err, entity.ErrAccountStatusTransition),
		errors.Is(err, entity.ErrAccountDeactivated),
		errors.Is(err, entity.ErrAccountDeleted):
		return newProblem(http.StatusConflict, err.Error()), true

	case errors.Is(err, entity.ErrEmailChangeExpired),
		errors.Is(err, entity.ErrEmailChangeTokenInvalid),
		errors.Is(err, entity.ErrEmailChangeSameAddress):
		return newProblem(http.StatusBadRequest, err.Error()), true

	case errors.Is(err, entity.ErrAvatarTooLarge):
		return newProblem(http.StatusRequestEntityTooLarge, err.Error()), true

	case errors.Is(err, entity.ErrAvatarUnsupportedType):
		return newProblem(http.StatusUnsupportedMediaType, err.Error()), true

	case errors.Is(err, entity.ErrSignInRateLimited):
		base := baseProblem(http.StatusTooManyRequests, err.Error())

		return problemResponse{
			status: http.StatusTooManyRequests,
			body: api.RateLimitedProblem{
				Code:     api.RateLimited,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
			retryAfter: int(entity.SignInAddressCooldown.Seconds()),
		}, true

	case errors.Is(err, entity.ErrMailDeliveryNotConfigured):
		base := baseProblem(http.StatusServiceUnavailable, err.Error())

		return problemResponse{
			status: http.StatusServiceUnavailable,
			body: api.MailUnavailableProblem{
				Code:     api.MailUnavailableProblemCodeMailUnavailable,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true

	case errors.Is(err, entity.ErrPasswordBreachCheckUnavailable):
		base := baseProblem(http.StatusServiceUnavailable, entity.ErrPasswordBreachCheckUnavailable.Error())

		return problemResponse{
			status: http.StatusServiceUnavailable,
			body: api.BreachCheckUnavailableProblem{
				Code:     api.BreachCheckUnavailableProblemCodeBreachCheckUnavailable,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true

	case errors.Is(err, entity.ErrPasswordResetAlreadyUsed):
		base := baseProblem(http.StatusConflict, err.Error())

		return problemResponse{
			status: http.StatusConflict,
			body: api.ResetLinkUsedProblem{
				Code:     api.ResetLinkUsed,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true

	case errors.Is(err, entity.ErrPasswordResetExpired),
		errors.Is(err, entity.ErrPasswordResetTokenInvalid),
		errors.Is(err, entity.ErrPasswordResetNotFound):
		base := baseProblem(http.StatusBadRequest, err.Error())

		return problemResponse{
			status: http.StatusBadRequest,
			body: api.ResetLinkExpiredProblem{
				Code:     api.ResetLinkExpired,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true

	case errors.Is(err, entity.ErrWorkspacePasswordAuthDisabled):
		return newProblem(http.StatusConflict, err.Error()), true

	default:
		return problemResponse{}, false
	}
}

func unauthorized() problemResponse {
	return newProblem(http.StatusUnauthorized, "a valid session is required")
}

func (r problemResponse) VisitRegisterAccountResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitSignInResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitSignOutResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitRequestPasswordResetResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitConfirmPasswordResetResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetCurrentAccountResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUpdateCurrentAccountResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDeleteCurrentAccountResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDeactivateCurrentAccountResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetPasswordResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitChangePasswordResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitGetPendingEmailChangeResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRequestEmailChangeResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitConfirmEmailChangeResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUploadAvatarResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitRemoveAvatarResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitListWorkspacesResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitCreateWorkspaceResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceMembersResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitAddWorkspaceMemberResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitChangeWorkspaceMemberRoleResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRemoveWorkspaceMemberResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListSessionsResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitRevokeSessionResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitRevokeAllSessionsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceAuthPolicyResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetWorkspaceAuthPolicyResponse(w http.ResponseWriter) error {
	return r.write(w)
}
