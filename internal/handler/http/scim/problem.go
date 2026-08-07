package scim

import (
	"errors"
	"net/http"

	"github.com/usenorn/norn/internal/entity"
)

func problem(err error) (int, string, string) {
	var lastAdmin entity.LastWorkspaceAdminError

	switch {
	case errors.Is(err, entity.ErrDirectoryUnlicensed):
		return http.StatusServiceUnavailable, "", err.Error()

	case errors.Is(err, entity.ErrDirectoryTokenInvalid):
		return http.StatusUnauthorized, "", err.Error()

	case errors.Is(err, entity.ErrDirectoryDisabled):
		return http.StatusForbidden, "", err.Error()

	case errors.Is(err, entity.ErrDirectoryNotConnected):
		return http.StatusForbidden, "", err.Error()

	case errors.Is(err, entity.ErrDirectoryUserNotFound),
		errors.Is(err, entity.ErrDirectoryGroupNotFound):
		return http.StatusNotFound, "invalidValue", err.Error()

	case errors.Is(err, entity.ErrDirectoryUserExists),
		errors.Is(err, entity.ErrDirectoryGroupExists):
		return http.StatusConflict, "uniqueness", err.Error()

	case errors.Is(err, entity.ErrDirectoryUserNameRequired):
		return http.StatusBadRequest, "invalidValue", err.Error()

	case errors.Is(err, entity.ErrDirectoryPatchUnsupported):
		return http.StatusBadRequest, "invalidPath", err.Error()

	case errors.As(err, &lastAdmin):
		return http.StatusConflict, "mutability",
			"this would leave the workspace without an administrator"

	case errors.Is(err, entity.ErrAccountDeactivated),
		errors.Is(err, entity.ErrDirectoryAccountNotClaimable):
		return http.StatusConflict, "mutability", err.Error()
	}

	return http.StatusInternalServerError, "", ""
}
