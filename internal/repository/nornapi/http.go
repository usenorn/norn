package nornapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
)

const detailLimit = 4 << 10

type nornRepository struct {
	client *http.Client
	server string
}

func New(cfg config.Gateway) repository.Norn {
	return &nornRepository{
		client: &http.Client{
			Timeout: cfg.RequestTimeout,
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				MaxIdleConnsPerHost: 64,
				IdleConnTimeout:     90 * time.Second,
				ForceAttemptHTTP2:   true,
			},
		},
		server: strings.TrimSuffix(cfg.Server, "/"),
	}
}

type tokenAnswer struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type introspectAnswer struct {
	Verdict     string     `json:"verdict"`
	ExecutionID string     `json:"executionId"`
	RunnerID    string     `json:"runnerId"`
	Preview     string     `json:"preview"`
	Mode        string     `json:"mode"`
	Path        string     `json:"path"`
	Reason      string     `json:"reason"`
	Redirect    string     `json:"redirect"`
	ExpiresAt   *time.Time `json:"expiresAt"`
}

type grantAnswer struct {
	Grant     string    `json:"grant"`
	ExpiresAt time.Time `json:"expiresAt"`
	Cookie    string    `json:"cookie"`
	Path      string    `json:"path"`
}

type tunnelAnswer struct {
	RunnerID    string `json:"runnerId"`
	WorkspaceID string `json:"workspaceId"`
	Runner      string `json:"runner"`
}

func (r *nornRepository) Exchange(
	ctx context.Context,
	secret string,
) (entity.PreviewGatewayToken, error) {
	var answered tokenAnswer

	if err := r.call(ctx, entity.PreviewGatewayTokenPath, secret, nil, &answered); err != nil {
		return entity.PreviewGatewayToken{}, err
	}

	return entity.PreviewGatewayToken{
		Token:     answered.Token,
		ExpiresAt: answered.ExpiresAt,
	}, nil
}

func (r *nornRepository) Introspect(
	ctx context.Context,
	token string,
	ask entity.PreviewAsk,
) (entity.PreviewReply, error) {
	asked := map[string]string{
		"host":      ask.Host,
		"grant":     ask.Grant,
		"userAgent": ask.UserAgent,
	}

	if ask.IP.IsValid() {
		asked["ip"] = ask.IP.String()
	}

	var answered introspectAnswer

	if err := r.call(ctx, entity.PreviewGatewayIntrospectPath, token, asked, &answered); err != nil {
		return entity.PreviewReply{}, err
	}

	reply := entity.PreviewReply{
		Verdict:     entity.PreviewVerdict(answered.Verdict),
		ExecutionID: answered.ExecutionID,
		Preview:     answered.Preview,
		Mode:        entity.PreviewMode(answered.Mode),
		Path:        answered.Path,
		Reason:      answered.Reason,
		Redirect:    answered.Redirect,
	}

	if answered.RunnerID != "" {
		runnerID, err := uuid.Parse(answered.RunnerID)
		if err != nil {
			return entity.PreviewReply{}, fmt.Errorf("parse runner id: %w", err)
		}

		reply.RunnerID = runnerID
	}

	if answered.ExpiresAt != nil {
		reply.ExpiresAt = *answered.ExpiresAt
	}

	return reply, nil
}

func (r *nornRepository) Session(
	ctx context.Context,
	token, ticket string,
) (entity.PreviewGrantReply, error) {
	return r.granted(ctx, entity.PreviewGatewaySessionPath, token, map[string]string{"ticket": ticket})
}

func (r *nornRepository) Redeem(
	ctx context.Context,
	token, host, share, passcode string,
) (entity.PreviewGrantReply, error) {
	return r.granted(ctx, entity.PreviewGatewaySharePath, token, map[string]string{
		"host":     host,
		"token":    share,
		"passcode": passcode,
	})
}

func (r *nornRepository) granted(
	ctx context.Context,
	path, token string,
	body map[string]string,
) (entity.PreviewGrantReply, error) {
	var answered grantAnswer

	if err := r.call(ctx, path, token, body, &answered); err != nil {
		return entity.PreviewGrantReply{}, err
	}

	return entity.PreviewGrantReply{
		Grant:     answered.Grant,
		Cookie:    answered.Cookie,
		Path:      answered.Path,
		ExpiresAt: answered.ExpiresAt,
	}, nil
}

func (r *nornRepository) Tunnel(
	ctx context.Context,
	token, ticket string,
) (entity.TunnelClaim, error) {
	var answered tunnelAnswer

	if err := r.call(
		ctx, entity.PreviewGatewayTunnelPath, token, map[string]string{"ticket": ticket}, &answered,
	); err != nil {
		return entity.TunnelClaim{}, err
	}

	runnerID, err := uuid.Parse(answered.RunnerID)
	if err != nil {
		return entity.TunnelClaim{}, fmt.Errorf("parse runner id: %w", err)
	}

	workspaceID, err := uuid.Parse(answered.WorkspaceID)
	if err != nil {
		return entity.TunnelClaim{}, fmt.Errorf("parse workspace id: %w", err)
	}

	return entity.TunnelClaim{
		RunnerID:    runnerID,
		WorkspaceID: workspaceID,
		Runner:      answered.Runner,
	}, nil
}

func (r *nornRepository) call(
	ctx context.Context,
	path, token string,
	body any,
	into any,
) error {
	payload := []byte("{}")

	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request for norn: %w", err)
		}

		payload = encoded
	}

	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, r.server+path, bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("build request for norn: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := r.client.Do(request)
	if err != nil {
		return fmt.Errorf("ask norn: %w", err)
	}

	defer func() { _ = response.Body.Close() }()

	if response.StatusCode >= http.StatusBadRequest {
		return refusal(response)
	}

	if err := json.NewDecoder(response.Body).Decode(into); err != nil {
		return fmt.Errorf("read what norn answered: %w", err)
	}

	return nil
}

func refusal(response *http.Response) error {
	detail := ""

	if body, err := io.ReadAll(io.LimitReader(response.Body, detailLimit)); err == nil {
		var problem struct {
			Detail string `json:"detail"`
			Title  string `json:"title"`
		}

		if json.Unmarshal(body, &problem) == nil {
			detail = problem.Detail
			if detail == "" {
				detail = problem.Title
			}
		}
	}

	switch response.StatusCode {
	case http.StatusUnauthorized:
		return entity.ErrPreviewGatewayCredentialInvalid
	case http.StatusForbidden:
		return entity.ErrPreviewSharePasscode
	case http.StatusUnprocessableEntity:
		return entity.ErrPreviewSharePasscodeNeeded
	case http.StatusNotFound:
		return entity.ErrPreviewShareNotFound
	case http.StatusGone:
		return entity.ErrPreviewShareExpired
	case http.StatusTooManyRequests:
		return entity.ErrPreviewShareGuessed
	}

	if detail == "" {
		detail = response.Status
	}

	return errors.New(detail)
}
