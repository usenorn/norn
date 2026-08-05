package events

import (
	"errors"
	"net/http"

	"github.com/usenorn/norn/internal/entity"
)

func problem(err error) (int, string) {
	var denied entity.AccessDeniedError

	if errors.As(err, &denied) {
		if denied.Reason == entity.DenyReasonNoActor {
			return http.StatusUnauthorized, "a valid session is required"
		}

		return http.StatusForbidden, denied.Error()
	}

	return http.StatusInternalServerError, ""
}
