package auditexport

import (
	"errors"
	"net/http"

	"github.com/usenorn/norn/internal/entity"
)

func problem(err error) (int, string) {
	switch {
	case errors.Is(err, entity.ErrAuditUnlicensed):
		return http.StatusServiceUnavailable, err.Error()

	case errors.Is(err, entity.ErrAuditNotPermitted):
		return http.StatusForbidden, err.Error()

	case errors.Is(err, entity.ErrAccountForbidden):
		return http.StatusForbidden, err.Error()
	}

	var denied entity.AccessDeniedError

	if errors.As(err, &denied) {
		if denied.Reason == entity.DenyReasonNoActor {
			return http.StatusUnauthorized, "a valid session is required"
		}

		return http.StatusForbidden, denied.Error()
	}

	return http.StatusInternalServerError, ""
}
