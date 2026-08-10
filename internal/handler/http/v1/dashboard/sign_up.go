package dashboard

import (
	"context"
	"errors"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/pkg/httpcookie"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) RequestSignUp(ctx context.Context, request api.RequestSignUpRequestObject) (api.RequestSignUpResponseObject, error) {
	input := service.RequestSignUpInput{
		Email:       request.Body.Email,
		DisplayName: request.Body.DisplayName,
		Password:    request.Body.Password,
		Client:      middleware.ClientFrom(ctx),
	}

	if request.Body.Timezone != nil {
		input.Timezone = *request.Body.Timezone
	}

	requested, err := h.accounts.RequestSignUp(ctx, input)
	if err != nil {
		if errors.Is(err, entity.ErrAccountEmailTaken) {
			return signUpUnusableProblem(api.SignUpEmailTaken, err), nil
		}

		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RequestSignUp202JSONResponse(signUpRequestedDTO(requested)), nil
}

func (h *handler) ConfirmSignUp(ctx context.Context, request api.ConfirmSignUpRequestObject) (api.ConfirmSignUpResponseObject, error) {
	confirmed, err := h.accounts.ConfirmSignUp(ctx, service.ConfirmSignUpInput{
		Token:  request.Body.Token,
		Client: middleware.ClientFrom(ctx),
	})
	if err != nil {
		if errors.Is(err, entity.ErrAccountEmailTaken) {
			return signUpUnusableProblem(api.SignUpEmailTaken, err), nil
		}

		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	httpcookie.Pending(ctx).Add(
		middleware.IssuedSessionCookie(h.session, confirmed.Session.Session, confirmed.Session.Token),
	)

	return api.ConfirmSignUp200JSONResponse{
		Account: accountDTO(confirmed.Account),
		Slot:    confirmed.Session.Session.Slot,
	}, nil
}
