package runnerchannel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

const Path = "/v1/runners/channel"

const outbound = 32

type Edge struct {
	channels service.RunnerChannels
	cfg      config.Runner
}

func New(channels service.RunnerChannels, cfg config.Runner) *Edge {
	return &Edge{channels: channels, cfg: cfg}
}

func (e *Edge) Serve(w http.ResponseWriter, r *http.Request) {
	if !e.cfg.ChannelEnabled {
		middleware.WriteProblem(
			w, r, http.StatusNotFound, "the runner channel is not enabled on this instance",
		)

		return
	}

	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		middleware.WriteProblem(
			w, r, http.StatusUnauthorized, "open the channel with the ticket from /v1/runners/token",
		)

		return
	}

	reported := r.URL.Query().Get("version")

	if !channelv1.RunnerSupported(reported, e.cfg.MinimumVersion) {
		e.writeOutdated(w, r, reported)

		return
	}

	handshake, settled := context.WithTimeout(r.Context(), entity.ChannelHandshakeTimeout)

	session, err := e.channels.Open(handshake, ticket)

	settled()

	if err != nil {
		status, detail := refusal(err)
		middleware.WriteProblem(w, r, status, detail)

		return
	}

	ctx := logging.With(r.Context(), "runner_id", session.Runner.ID.String())

	defer e.channels.Close(context.WithoutCancel(ctx), session)

	socket, err := websocket.Accept(w, r, nil)
	if err != nil {
		logging.From(ctx).InfoContext(
			ctx, "a runner channel upgrade failed", slog.String("error", err.Error()),
		)

		return
	}

	socket.SetReadLimit(entity.ChannelMaxMessageBytes)

	defer func() { _ = socket.CloseNow() }()

	held := &connection{
		socket:   socket,
		channels: e.channels,
		session:  session,
		writes:   make(chan channelv1.Envelope, outbound),
		inflight: map[string]string{},
	}

	held.run(ctx)
}

type connection struct {
	socket   *websocket.Conn
	channels service.RunnerChannels
	session  service.ChannelSession

	writes chan channelv1.Envelope

	mu       sync.Mutex
	inflight map[string]string
}

func (c *connection) run(ctx context.Context) {
	ctx, stop := context.WithCancel(ctx)
	defer stop()

	finished := make(chan error, 4)

	go func() { finished <- c.readPump(ctx) }()
	go func() { finished <- c.writePump(ctx) }()
	go func() { finished <- c.spoolPump(ctx) }()
	go func() { finished <- c.leasePump(ctx) }()

	err := <-finished

	stop()

	c.hangUp(ctx, err)
}

func (c *connection) hangUp(ctx context.Context, err error) {
	switch {
	case err == nil, errors.Is(err, context.Canceled):
		_ = c.socket.Close(websocket.StatusNormalClosure, "")
	case errors.Is(err, entity.ErrChannelDisplaced):
		_ = c.socket.Close(websocket.StatusPolicyViolation, "another connection took this channel")
	case errors.Is(err, entity.ErrRunnerRevoked):
		_ = c.socket.Close(websocket.StatusPolicyViolation, "this runner has been revoked")
	case errors.Is(err, entity.ErrChannelEnvelopeInvalid), errors.Is(err, entity.ErrChannelTypeUnknown),
		errors.Is(err, entity.ErrChannelTypeNotInbound):
		_ = c.socket.Close(websocket.StatusUnsupportedData, err.Error())
	default:
		logging.From(ctx).InfoContext(
			ctx, "a runner channel ended", slog.String("error", err.Error()),
		)
		_ = c.socket.Close(websocket.StatusInternalError, "")
	}
}

func (c *connection) readPump(ctx context.Context) error {
	for {
		kind, raw, err := c.socket.Read(ctx)
		if err != nil {
			return err
		}

		if kind != websocket.MessageText {
			return entity.ErrChannelEnvelopeInvalid
		}

		var inbound channelv1.Envelope

		if err := json.Unmarshal(raw, &inbound); err != nil {
			return entity.ErrChannelEnvelopeInvalid
		}

		if inbound.V != channelv1.Version {
			return entity.ErrChannelEnvelopeInvalid
		}

		if inbound.Acknowledging() {
			if err := c.settle(ctx, inbound.AckID); err != nil {
				return err
			}

			continue
		}

		if err := c.channels.Receive(ctx, c.session, inbound.Message()); err != nil {
			return err
		}

		select {
		case c.writes <- channelv1.Acknowledgement(inbound.ID, time.Now()):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *connection) writePump(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case frame := <-c.writes:
			body, err := json.Marshal(frame)
			if err != nil {
				return err
			}

			if err := c.socket.Write(ctx, websocket.MessageText, body); err != nil {
				return err
			}
		}
	}
}

func (c *connection) spoolPump(ctx context.Context) error {
	cursor := c.session.Cursor

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		spooled, next, err := c.channels.Deliver(ctx, c.session, cursor)
		if err != nil {
			return err
		}

		cursor = next

		for _, waiting := range spooled {
			c.remember(waiting.Message.ID, waiting.Cursor)

			select {
			case c.writes <- channelv1.Frame(waiting.Message):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

func (c *connection) leasePump(ctx context.Context) error {
	lease := time.NewTicker(entity.ChannelHeartbeat)
	defer lease.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-lease.C:
			if err := c.channels.Verify(ctx, c.session); err != nil {
				return err
			}
		}
	}
}

func (c *connection) remember(messageID, cursor string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.inflight[messageID] = cursor
}

func (c *connection) settle(ctx context.Context, messageID string) error {
	c.mu.Lock()
	cursor, delivered := c.inflight[messageID]
	delete(c.inflight, messageID)
	c.mu.Unlock()

	if !delivered {
		return nil
	}

	return c.channels.Acknowledge(ctx, c.session, cursor)
}

func (e *Edge) writeOutdated(w http.ResponseWriter, r *http.Request, reported string) {
	middleware.WriteRunnerProblem(
		w, r, http.StatusUpgradeRequired, api.RunnerProblemCodeRunnerOutdated,
		fmt.Sprintf(
			"this runner is %s and norn needs %s or newer. Take the new one with: %s",
			named(reported), e.cfg.MinimumVersion, channelv1.InstallRunner,
		),
	)
}

func named(reported string) string {
	if reported == "" {
		return "an unnamed build"
	}

	return reported
}

func refusal(err error) (int, string) {
	switch {
	case errors.Is(err, entity.ErrChannelTicketInvalid), errors.Is(err, entity.ErrRunnerCredentialInvalid):
		return http.StatusUnauthorized, "that channel ticket is not valid; exchange a fresh one"
	case errors.Is(err, entity.ErrRunnerRevoked):
		return http.StatusUnauthorized, entity.ErrRunnerRevoked.Error()
	case errors.Is(err, entity.ErrAgentDisabled):
		return http.StatusForbidden, entity.ErrAgentDisabled.Error()
	default:
		return http.StatusInternalServerError, ""
	}
}
