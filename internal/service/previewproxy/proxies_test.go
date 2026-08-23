package previewproxy_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository/nornapi"
	tunnelrepo "github.com/usenorn/norn/internal/repository/tunnel"
	"github.com/usenorn/norn/internal/service"
	"github.com/usenorn/norn/internal/service/previewproxy"
)

const settle = 2 * time.Second

func settings() config.Gateway {
	return config.Gateway{
		Secret:              "ngr_secret",
		Server:              "https://app.norn.test",
		MaxStreamsPerRunner: 8,
		RequestTimeout:      settle,
		StreamOpenTimeout:   settle,
		Heartbeat:           time.Second,
		RefreshLead:         50 * time.Millisecond,
		RetryMin:            10 * time.Millisecond,
		RetryMax:            100 * time.Millisecond,
	}
}

func build(t *testing.T, cfg config.Gateway) (*nornapi.MockNorn, service.PreviewProxy) {
	t.Helper()

	norn := nornapi.NewMockNorn(gomock.NewController(t))

	return norn, previewproxy.New(norn, tunnelrepo.New(cfg), cfg)
}

func running(t *testing.T, proxies service.PreviewProxy) {
	t.Helper()

	ctx, stop := context.WithCancel(context.Background())

	go proxies.Run(ctx)

	t.Cleanup(stop)
}

func within(t *testing.T, is func() bool, why string) {
	t.Helper()

	deadline := time.Now().Add(settle)

	for time.Now().Before(deadline) {
		if is() {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatal(why)
}

func TestAGatewayThatHasNoTokenRefusesRatherThanAskingNornAnyway(t *testing.T) {
	norn, proxies := build(t, settings())

	norn.EXPECT().Introspect(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	_, err := proxies.Route(context.Background(), entity.PreviewAsk{Host: "web-exec.norn.ink"})

	if !errors.Is(err, entity.ErrGatewayUnready) {
		t.Fatalf(
			"a gateway holding no credential answered %v, want %v; asking norn with an empty "+
				"bearer would read as a viewer with no session and send everybody to sign in",
			err, entity.ErrGatewayUnready,
		)
	}

	if proxies.Ready() {
		t.Fatal("a gateway with no credential calls itself ready, so a probe would say it is up")
	}
}

func TestTheGatewayTradesForAFreshTokenBeforeItsOwnLapses(t *testing.T) {
	cfg := settings()
	norn, proxies := build(t, cfg)

	var traded atomic.Int64

	norn.EXPECT().
		Exchange(gomock.Any(), "ngr_secret").
		DoAndReturn(func(context.Context, string) (entity.PreviewGatewayToken, error) {
			traded.Add(1)

			return entity.PreviewGatewayToken{
				Token:     "nga_token",
				ExpiresAt: time.Now().UTC().Add(cfg.RefreshLead + 20*time.Millisecond),
			}, nil
		}).
		MinTimes(2)

	running(t, proxies)

	within(t, func() bool { return traded.Load() >= 2 },
		"the gateway traded its secret once and stopped; a token that is allowed to lapse "+
			"turns every preview into a sign-in loop until somebody restarts the process",
	)
}

func TestAGatewayNornWillNotAnswerKeepsTryingRatherThanGivingUp(t *testing.T) {
	norn, proxies := build(t, settings())

	var asked atomic.Int64

	norn.EXPECT().
		Exchange(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, string) (entity.PreviewGatewayToken, error) {
			asked.Add(1)

			return entity.PreviewGatewayToken{}, errors.New("norn is down")
		}).
		MinTimes(2)

	running(t, proxies)

	within(t, func() bool { return asked.Load() >= 2 },
		"the gateway stopped asking after norn refused it once, so a restart of norn would "+
			"leave every preview dark until somebody restarted the gateway too",
	)

	if proxies.Ready() {
		t.Fatal("a gateway that never got a token calls itself ready")
	}
}
