package dashboard

import (
	"context"
	"errors"
	"net/http"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

const avatarFormField = "file"

func (h *handler) GetCurrentAccount(ctx context.Context, _ api.GetCurrentAccountRequestObject) (api.GetCurrentAccountResponseObject, error) {
	accountID, ok := h.currentAccountID(ctx)
	if !ok {
		return unauthorized(), nil
	}

	account, err := h.accounts.Get(ctx, accountID)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetCurrentAccount200JSONResponse(accountDTO(account, h.avatarURL(account.AvatarObjectKey))), nil
}

func (h *handler) UpdateCurrentAccount(ctx context.Context, request api.UpdateCurrentAccountRequestObject) (api.UpdateCurrentAccountResponseObject, error) {
	accountID, ok := h.currentAccountID(ctx)
	if !ok {
		return unauthorized(), nil
	}

	account, err := h.accounts.UpdateProfile(ctx, accountID, service.UpdateProfileInput{
		DisplayName: request.Body.DisplayName,
		Timezone:    request.Body.Timezone,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UpdateCurrentAccount200JSONResponse(accountDTO(account, h.avatarURL(account.AvatarObjectKey))), nil
}

func (h *handler) DeactivateCurrentAccount(ctx context.Context, _ api.DeactivateCurrentAccountRequestObject) (api.DeactivateCurrentAccountResponseObject, error) {
	accountID, ok := h.currentAccountID(ctx)
	if !ok {
		return unauthorized(), nil
	}

	account, err := h.accounts.Deactivate(ctx, accountID)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DeactivateCurrentAccount200JSONResponse(accountDTO(account, h.avatarURL(account.AvatarObjectKey))), nil
}

func (h *handler) DeleteCurrentAccount(ctx context.Context, _ api.DeleteCurrentAccountRequestObject) (api.DeleteCurrentAccountResponseObject, error) {
	accountID, ok := h.currentAccountID(ctx)
	if !ok {
		return unauthorized(), nil
	}

	if err := h.accounts.Delete(ctx, accountID); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DeleteCurrentAccount204Response{}, nil
}

func (h *handler) SetPassword(ctx context.Context, request api.SetPasswordRequestObject) (api.SetPasswordResponseObject, error) {
	accountID, ok := h.currentAccountID(ctx)
	if !ok {
		return unauthorized(), nil
	}

	issued, err := h.accounts.SetPassword(ctx, accountID, request.Body.Password)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	cookie := middleware.IssuedSessionCookie(h.session, issued.Token).String()

	return api.SetPassword204Response{Headers: api.SetPassword204ResponseHeaders{SetCookie: &cookie}}, nil
}

func (h *handler) ChangePassword(ctx context.Context, request api.ChangePasswordRequestObject) (api.ChangePasswordResponseObject, error) {
	accountID, ok := h.currentAccountID(ctx)
	if !ok {
		return unauthorized(), nil
	}

	issued, err := h.accounts.ChangePassword(ctx, accountID, request.Body.CurrentPassword, request.Body.NewPassword)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	cookie := middleware.IssuedSessionCookie(h.session, issued.Token).String()

	return api.ChangePassword204Response{Headers: api.ChangePassword204ResponseHeaders{SetCookie: &cookie}}, nil
}

func (h *handler) GetPendingEmailChange(ctx context.Context, _ api.GetPendingEmailChangeRequestObject) (api.GetPendingEmailChangeResponseObject, error) {
	accountID, ok := h.currentAccountID(ctx)
	if !ok {
		return unauthorized(), nil
	}

	change, err := h.accounts.PendingEmailChange(ctx, accountID)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetPendingEmailChange200JSONResponse(emailChangeDTO(change)), nil
}

func (h *handler) RequestEmailChange(ctx context.Context, request api.RequestEmailChangeRequestObject) (api.RequestEmailChangeResponseObject, error) {
	accountID, ok := h.currentAccountID(ctx)
	if !ok {
		return unauthorized(), nil
	}

	change, err := h.accounts.RequestEmailChange(ctx, accountID, request.Body.Email)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RequestEmailChange202JSONResponse(emailChangeDTO(change)), nil
}

func (h *handler) ConfirmEmailChange(ctx context.Context, request api.ConfirmEmailChangeRequestObject) (api.ConfirmEmailChangeResponseObject, error) {
	account, err := h.accounts.ConfirmEmailChange(ctx, request.Body.Token)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ConfirmEmailChange200JSONResponse(accountDTO(account, h.avatarURL(account.AvatarObjectKey))), nil
}

func (h *handler) UploadAvatar(ctx context.Context, request api.UploadAvatarRequestObject) (api.UploadAvatarResponseObject, error) {
	accountID, ok := h.currentAccountID(ctx)
	if !ok {
		return unauthorized(), nil
	}

	part, err := request.Body.NextPart()
	if err != nil {
		return newProblem(http.StatusBadRequest, "the request must carry a single "+avatarFormField+" part"), nil
	}
	defer func() { _ = part.Close() }()

	if part.FormName() != avatarFormField {
		return newProblem(http.StatusBadRequest, "the request must carry a single "+avatarFormField+" part"), nil
	}

	account, err := h.accounts.UploadAvatar(ctx, accountID, service.AvatarUpload{Body: part})
	if err != nil {
		var oversized *http.MaxBytesError
		if errors.As(err, &oversized) {
			return newProblem(http.StatusRequestEntityTooLarge, entity.ErrAvatarTooLarge.Error()), nil
		}

		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UploadAvatar200JSONResponse(accountDTO(account, h.avatarURL(account.AvatarObjectKey))), nil
}

func (h *handler) RemoveAvatar(ctx context.Context, _ api.RemoveAvatarRequestObject) (api.RemoveAvatarResponseObject, error) {
	accountID, ok := h.currentAccountID(ctx)
	if !ok {
		return unauthorized(), nil
	}

	account, err := h.accounts.RemoveAvatar(ctx, accountID)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RemoveAvatar200JSONResponse(accountDTO(account, h.avatarURL(account.AvatarObjectKey))), nil
}
