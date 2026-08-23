package previewgateway_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	gatewayrepo "github.com/usenorn/norn/internal/repository/previewgateway"
	grantrepo "github.com/usenorn/norn/internal/repository/previewgrant"
	"github.com/usenorn/norn/internal/service"
	gatewaysvc "github.com/usenorn/norn/internal/service/previewgateway"
)

type harness struct {
	gateways *gatewayrepo.MockPreviewGateway
	grants   *grantrepo.MockPreviewGrant
	service  service.PreviewGateways

	stored  map[uuid.UUID]entity.PreviewGateway
	secrets map[string]uuid.UUID
	access  map[string]entity.PreviewGateway
	seen    map[uuid.UUID]time.Time
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		gateways: gatewayrepo.NewMockPreviewGateway(ctrl),
		grants:   grantrepo.NewMockPreviewGrant(ctrl),
		stored:   map[uuid.UUID]entity.PreviewGateway{},
		secrets:  map[string]uuid.UUID{},
		access:   map[string]entity.PreviewGateway{},
		seen:     map[uuid.UUID]time.Time{},
	}

	h.gateways.EXPECT().
		Create(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, gateway entity.PreviewGateway, hash []byte,
		) (entity.PreviewGateway, error) {
			for _, held := range h.stored {
				if held.Name == gateway.Name {
					return entity.PreviewGateway{}, entity.ErrPreviewGatewayNameTaken
				}
			}

			gateway.ID = uuid.New()
			gateway.Status = entity.PreviewGatewayActive
			h.stored[gateway.ID] = gateway
			h.secrets[string(hash)] = gateway.ID

			return gateway, nil
		}).
		AnyTimes()

	h.gateways.EXPECT().
		ByCredential(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, hash []byte) (entity.PreviewGateway, error) {
			id, known := h.secrets[string(hash)]
			if !known {
				return entity.PreviewGateway{}, entity.ErrPreviewGatewayNotFound
			}

			return h.stored[id], nil
		}).
		AnyTimes()

	h.gateways.EXPECT().
		List(gomock.Any()).
		DoAndReturn(func(_ context.Context) ([]entity.PreviewGateway, error) {
			held := make([]entity.PreviewGateway, 0, len(h.stored))

			for _, gateway := range h.stored {
				held = append(held, gateway)
			}

			return held, nil
		}).
		AnyTimes()

	h.gateways.EXPECT().
		Revoke(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID) (entity.PreviewGateway, error) {
			gateway, known := h.stored[id]
			if !known {
				return entity.PreviewGateway{}, entity.ErrPreviewGatewayNotFound
			}

			gateway.Status = entity.PreviewGatewayRevoked
			h.stored[id] = gateway

			return gateway, nil
		}).
		AnyTimes()

	h.gateways.EXPECT().
		Seen(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID, at time.Time) error {
			h.seen[id] = at

			return nil
		}).
		AnyTimes()

	h.grants.EXPECT().
		GrantGateway(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, hash []byte, gateway entity.PreviewGateway, _ time.Duration,
		) error {
			h.access[string(hash)] = gateway

			return nil
		}).
		AnyTimes()

	h.grants.EXPECT().
		ResolveGateway(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, hash []byte) (entity.PreviewGateway, error) {
			gateway, known := h.access[string(hash)]
			if !known {
				return entity.PreviewGateway{}, entity.ErrPreviewGatewayCredentialInvalid
			}

			return gateway, nil
		}).
		AnyTimes()

	h.grants.EXPECT().
		RevokeGateway(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID) error {
			for hash, gateway := range h.access {
				if gateway.ID == id {
					delete(h.access, hash)
				}
			}

			return nil
		}).
		AnyTimes()

	h.service = gatewaysvc.New(h.gateways, h.grants, config.Previews{
		GatewayAccessTTL: 15 * time.Minute,
	})

	return h
}

func (h *harness) enrolled(t *testing.T, name string) string {
	t.Helper()

	_, secret, err := h.service.Enrol(context.Background(), name)
	if err != nil {
		t.Fatalf("enrol %s: %v", name, err)
	}

	return secret
}

func refusedWith(t *testing.T, err, wanted error) {
	t.Helper()

	if !errors.Is(err, wanted) {
		t.Fatalf("the request was refused with %v, want %v", err, wanted)
	}
}
