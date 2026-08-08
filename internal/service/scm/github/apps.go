package github

import (
	"context"
	"net/http"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/forge"
)

func (f *Forge) appTarget(app entity.SCMApp, now time.Time) (entity.SCMTarget, error) {
	signed, err := forge.AppJWT(app.PrivateKey, app.ExternalAppID, now)
	if err != nil {
		return entity.SCMTarget{}, err
	}

	return entity.SCMTarget{
		Provider: entity.SCMProviderGitHub,
		BaseURL:  app.BaseURL,
		Token:    signed,
	}, nil
}

func (f *Forge) MintInstallationToken(
	ctx context.Context,
	app entity.SCMApp,
	installationID string,
) (entity.SCMCredential, error) {
	if installationID == "" {
		return entity.SCMCredential{}, entity.ErrSCMInstallationNotFound
	}

	target, err := f.appTarget(app, time.Now())
	if err != nil {
		return entity.SCMCredential{}, err
	}

	response, err := f.call(
		ctx,
		target,
		http.MethodPost,
		"/app/installations/"+installationID+"/access_tokens",
		nil,
	)
	if err != nil {
		return entity.SCMCredential{}, err
	}

	if response.Status == http.StatusNotFound || response.Status == http.StatusUnauthorized {
		return entity.SCMCredential{}, entity.ErrSCMInstallationNotFound
	}

	var body struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}

	if err := f.decode(response, target, &body); err != nil {
		return entity.SCMCredential{}, err
	}

	if body.Token == "" {
		return entity.SCMCredential{}, entity.ErrSCMAppTokenUnavailable
	}

	return entity.SCMCredential{Token: body.Token, ExpiresAt: body.ExpiresAt}, nil
}
