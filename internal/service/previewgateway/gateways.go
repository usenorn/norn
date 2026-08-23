package previewgateway

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type gatewaysService struct {
	gateways repository.PreviewGateway
	grants   repository.PreviewGrant
	previews config.Previews
}

func New(
	gateways repository.PreviewGateway,
	grants repository.PreviewGrant,
	previews config.Previews,
) service.PreviewGateways {
	return &gatewaysService{gateways: gateways, grants: grants, previews: previews}
}

func (s *gatewaysService) Enrol(
	ctx context.Context,
	name string,
) (entity.PreviewGateway, string, error) {
	if err := entity.NewValidationError(
		entity.ValidatePreviewGatewayName("name", name),
	); err != nil {
		return entity.PreviewGateway{}, "", err
	}

	secret, hash, err := entity.NewPreviewGatewaySecret(entity.PreviewGatewaySecretPrefix)
	if err != nil {
		return entity.PreviewGateway{}, "", err
	}

	enrolled, err := s.gateways.Create(ctx, entity.PreviewGateway{
		Name: strings.TrimSpace(name),
	}, hash)
	if err != nil {
		return entity.PreviewGateway{}, "", err
	}

	return enrolled, secret, nil
}

func (s *gatewaysService) Exchange(
	ctx context.Context,
	secret string,
) (service.PreviewGatewayAccess, error) {
	if !strings.HasPrefix(secret, entity.PreviewGatewaySecretPrefix) {
		return service.PreviewGatewayAccess{}, entity.ErrPreviewGatewayCredentialInvalid
	}

	gateway, err := s.gateways.ByCredential(ctx, entity.HashPreviewGatewaySecret(secret))
	if err != nil {
		if errors.Is(err, entity.ErrPreviewGatewayNotFound) {
			return service.PreviewGatewayAccess{}, entity.ErrPreviewGatewayCredentialInvalid
		}

		return service.PreviewGatewayAccess{}, err
	}

	if gateway.Status != entity.PreviewGatewayActive {
		return service.PreviewGatewayAccess{}, entity.ErrPreviewGatewayRevoked
	}

	token, hash, err := entity.NewPreviewGatewaySecret(entity.PreviewGatewayAccessPrefix)
	if err != nil {
		return service.PreviewGatewayAccess{}, err
	}

	ttl := s.previews.GatewayAccessTTL

	if err := s.grants.GrantGateway(ctx, hash, gateway, ttl); err != nil {
		return service.PreviewGatewayAccess{}, err
	}

	now := time.Now().UTC()

	if err := s.gateways.Seen(ctx, gateway.ID, now); err != nil {
		return service.PreviewGatewayAccess{}, err
	}

	return service.PreviewGatewayAccess{Token: token, ExpiresAt: now.Add(ttl)}, nil
}

func (s *gatewaysService) Authenticate(
	ctx context.Context,
	token string,
) (entity.PreviewGateway, error) {
	if !entity.LooksLikePreviewGatewayToken(token) {
		return entity.PreviewGateway{}, entity.ErrPreviewGatewayCredentialInvalid
	}

	return s.grants.ResolveGateway(ctx, entity.HashPreviewGatewaySecret(token))
}

func (s *gatewaysService) List(ctx context.Context) ([]entity.PreviewGateway, error) {
	return s.gateways.List(ctx)
}

func (s *gatewaysService) Revoke(
	ctx context.Context,
	gatewayID uuid.UUID,
) (entity.PreviewGateway, error) {
	revoked, err := s.gateways.Revoke(ctx, gatewayID)
	if err != nil {
		return entity.PreviewGateway{}, err
	}

	if err := s.grants.RevokeGateway(ctx, gatewayID); err != nil {
		return entity.PreviewGateway{}, err
	}

	return revoked, nil
}
