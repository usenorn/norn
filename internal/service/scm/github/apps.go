package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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

func (f *Forge) webBase(baseURL string) string {
	return entity.SCMApp{BaseURL: f.base(entity.SCMTarget{BaseURL: baseURL})}.WebURL()
}

func (f *Forge) ManifestTarget(baseURL, organization string) string {
	web := f.webBase(baseURL)

	if organization == "" {
		return web + "/settings/apps/new"
	}

	return web + "/organizations/" + url.PathEscape(organization) + "/settings/apps/new"
}

func (f *Forge) ConvertManifest(
	ctx context.Context,
	baseURL, code string,
) (entity.SCMApp, error) {
	if code == "" {
		return entity.SCMApp{}, entity.ErrSCMAppRefused
	}

	target := entity.SCMTarget{Provider: entity.SCMProviderGitHub, BaseURL: baseURL}

	response, err := f.call(
		ctx,
		target,
		http.MethodPost,
		"/app-manifests/"+url.PathEscape(code)+"/conversions",
		nil,
	)
	if err != nil {
		return entity.SCMApp{}, err
	}

	if response.Status == http.StatusNotFound || response.Status == http.StatusUnprocessableEntity {
		return entity.SCMApp{}, entity.ErrSCMAppRefused
	}

	var body struct {
		ID            int64  `json:"id"`
		Slug          string `json:"slug"`
		ClientID      string `json:"client_id"`
		ClientSecret  string `json:"client_secret"`
		PEM           string `json:"pem"`
		WebhookSecret string `json:"webhook_secret"`
	}

	if err := f.decode(response, target, &body); err != nil {
		return entity.SCMApp{}, err
	}

	if body.ID == 0 || body.PEM == "" || body.WebhookSecret == "" {
		return entity.SCMApp{}, entity.ErrSCMAppRefused
	}

	return entity.SCMApp{
		Provider:      entity.SCMProviderGitHub,
		BaseURL:       baseURL,
		Slug:          body.Slug,
		ExternalAppID: strconv.FormatInt(body.ID, 10),
		ClientID:      body.ClientID,
		ClientSecret:  body.ClientSecret,
		PrivateKey:    body.PEM,
		WebhookSecret: body.WebhookSecret,
	}, nil
}

func (f *Forge) AuthorizeURL(app entity.SCMApp, state, redirect string) string {
	query := url.Values{}
	query.Set("client_id", app.ClientID)
	query.Set("state", state)
	query.Set("redirect_uri", redirect)

	return f.webBase(app.BaseURL) + "/login/oauth/authorize?" + query.Encode()
}

func (f *Forge) ExchangeCode(
	ctx context.Context,
	app entity.SCMApp,
	code, redirect string,
) (string, error) {
	if code == "" {
		return "", entity.ErrSCMAppRefused
	}

	form := url.Values{}
	form.Set("client_id", app.ClientID)
	form.Set("client_secret", app.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirect)

	response, err := f.client.Do(ctx, forge.Request{
		Provider: entity.SCMProviderGitHub,
		Method:   http.MethodPost,
		URL:      f.webBase(app.BaseURL) + "/login/oauth/access_token",
		Header: http.Header{
			"Accept":       {"application/json"},
			"Content-Type": {"application/x-www-form-urlencoded"},
		},
		Body: []byte(form.Encode()),
	})
	if err != nil {
		return "", err
	}

	var body struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}

	target := entity.SCMTarget{Provider: entity.SCMProviderGitHub, BaseURL: app.BaseURL}

	if err := f.decode(response, target, &body); err != nil {
		return "", err
	}

	if body.AccessToken == "" {
		return "", entity.ErrSCMAppRefused
	}

	return body.AccessToken, nil
}

func (f *Forge) Installations(
	ctx context.Context,
	app entity.SCMApp,
	userToken string,
) ([]entity.SCMInstallation, error) {
	target := entity.SCMTarget{
		Provider: entity.SCMProviderGitHub,
		BaseURL:  app.BaseURL,
		Token:    userToken,
	}

	response, err := f.call(
		ctx,
		target,
		http.MethodGet,
		"/user/installations?per_page="+strconv.Itoa(f.pageSize),
		nil,
	)
	if err != nil {
		return nil, err
	}

	var body struct {
		Installations []struct {
			ID      int64 `json:"id"`
			AppID   int64 `json:"app_id"`
			Account struct {
				Login string `json:"login"`
				Type  string `json:"type"`
			} `json:"account"`
		} `json:"installations"`
	}

	if err := f.decode(response, target, &body); err != nil {
		return nil, err
	}

	installations := make([]entity.SCMInstallation, 0, len(body.Installations))

	for _, found := range body.Installations {
		if strconv.FormatInt(found.AppID, 10) != app.ExternalAppID {
			continue
		}

		installations = append(installations, entity.SCMInstallation{
			ExternalID:   strconv.FormatInt(found.ID, 10),
			AccountLogin: found.Account.Login,
			AccountKind:  strings.ToLower(found.Account.Type),
		})
	}

	return installations, nil
}

func (f *Forge) InstallationRepositories(
	ctx context.Context,
	app entity.SCMApp,
	credential string,
) ([]entity.SCMRemoteRepository, error) {
	target := entity.SCMTarget{
		Provider: entity.SCMProviderGitHub,
		BaseURL:  app.BaseURL,
		Token:    credential,
	}

	found := make([]entity.SCMRemoteRepository, 0)
	path := "/installation/repositories?per_page=" + strconv.Itoa(f.pageSize)

	for path != "" {
		response, err := f.call(ctx, target, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}

		var body struct {
			Repositories []struct {
				ID            int64  `json:"id"`
				FullName      string `json:"full_name"`
				HTMLURL       string `json:"html_url"`
				DefaultBranch string `json:"default_branch"`
				Private       bool   `json:"private"`
				Permissions   struct {
					Admin bool `json:"admin"`
				} `json:"permissions"`
			} `json:"repositories"`
		}

		if err := f.decode(response, target, &body); err != nil {
			return nil, err
		}

		for _, repository := range body.Repositories {
			found = append(found, entity.SCMRemoteRepository{
				ExternalID:    strconv.FormatInt(repository.ID, 10),
				FullName:      repository.FullName,
				URL:           repository.HTMLURL,
				DefaultBranch: repository.DefaultBranch,
				Private:       repository.Private,
				CanAdmin:      repository.Permissions.Admin,
			})
		}

		path = response.Link("next")
	}

	return found, nil
}

func (f *Forge) Route(body []byte) (entity.SCMDeliveryRoute, error) {
	var payload struct {
		Installation struct {
			ID int64 `json:"id"`
		} `json:"installation"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return entity.SCMDeliveryRoute{}, entity.ErrSCMSignatureInvalid
	}

	if payload.Installation.ID == 0 || payload.Repository.FullName == "" {
		return entity.SCMDeliveryRoute{}, entity.ErrSCMSignatureInvalid
	}

	return entity.SCMDeliveryRoute{
		InstallationID: strconv.FormatInt(payload.Installation.ID, 10),
		FullName:       payload.Repository.FullName,
	}, nil
}
