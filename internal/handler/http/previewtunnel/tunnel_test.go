package previewtunnel_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/previewtunnel"
	"github.com/usenorn/norn/internal/service/previewproxy"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func serving(t *testing.T) (*previewproxy.MockPreviewProxy, *httptest.Server) {
	t.Helper()

	proxies := previewproxy.NewMockPreviewProxy(gomock.NewController(t))
	edge := previewtunnel.New(proxies, config.Gateway{Heartbeat: time.Second})
	server := httptest.NewServer(http.HandlerFunc(edge.Serve))

	t.Cleanup(server.Close)

	return proxies, server
}

func dial(t *testing.T, address, query string) (int, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	socket, response, err := websocket.Dial(
		ctx, "ws"+address[len("http"):]+channelv1.TunnelPath+query, nil,
	)
	if err == nil {
		_ = socket.CloseNow()

		return http.StatusSwitchingProtocols, nil
	}

	if response == nil {
		return 0, err
	}

	defer func() { _ = response.Body.Close() }()

	return response.StatusCode, err
}

func TestATunnelWithNoTicketIsNeverUpgraded(t *testing.T) {
	proxies, server := serving(t)

	proxies.EXPECT().Accept(gomock.Any(), gomock.Any()).Times(0)
	proxies.EXPECT().Hold(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	status, err := dial(t, server.URL, "")

	if status != http.StatusUnauthorized {
		t.Fatalf(
			"opening a tunnel with no ticket answered %d (%v), want 401; anything that "+
				"upgrades first would let anybody hold a machine's slot",
			status, err,
		)
	}
}

func TestATicketNornRefusesNeverBecomesATunnel(t *testing.T) {
	proxies, server := serving(t)

	proxies.EXPECT().
		Accept(gomock.Any(), "nru_forged").
		Return(entity.TunnelClaim{}, entity.ErrRunnerCredentialInvalid)
	proxies.EXPECT().Hold(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	status, err := dial(t, server.URL, "?ticket=nru_forged")

	if status != http.StatusUnauthorized {
		t.Fatalf(
			"a ticket norn refused answered %d (%v), want 401; a gateway that holds a tunnel "+
				"for an unproven machine will route previews into it",
			status, err,
		)
	}
}

func TestARevokedMachineIsToldSoRatherThanLeftRetrying(t *testing.T) {
	proxies, server := serving(t)

	proxies.EXPECT().
		Accept(gomock.Any(), gomock.Any()).
		Return(entity.TunnelClaim{}, entity.ErrRunnerRevoked)

	status, _ := dial(t, server.URL, "?ticket=nru_revoked")

	if status != http.StatusForbidden {
		t.Fatalf(
			"a revoked machine answered %d, want 403; a machine that reads a refusal as "+
				"temporary reconnects for ever",
			status,
		)
	}
}

func TestAnAcceptedTicketHandsTheSocketToTheMachineItNames(t *testing.T) {
	proxies, server := serving(t)

	runnerID := uuid.New()
	held := make(chan uuid.UUID, 1)

	proxies.EXPECT().
		Accept(gomock.Any(), "nru_good").
		Return(entity.TunnelClaim{RunnerID: runnerID, WorkspaceID: uuid.New()}, nil)
	proxies.EXPECT().
		Hold(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, id uuid.UUID, _ net.Conn) error {
			held <- id

			<-ctx.Done()

			return ctx.Err()
		})

	status, err := dial(t, server.URL, "?ticket=nru_good")
	if status != http.StatusSwitchingProtocols {
		t.Fatalf("a good ticket answered %d (%v), want an upgrade", status, err)
	}

	select {
	case holding := <-held:
		if holding != runnerID {
			t.Fatalf(
				"the tunnel was held for %s, want %s; a socket filed under the wrong machine "+
					"routes one workspace's previews into another's",
				holding, runnerID,
			)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the socket was upgraded but never held, so no preview could travel down it")
	}
}
