package taskqueue

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/observability/logging"
)

type Client struct {
	*asynq.Client
}

type Inspector struct {
	*asynq.Inspector
}

type Server struct {
	*asynq.Server
}

func redisOpt(cfg config.Asynq) asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:     cfg.Addr,
		Username: cfg.Username,
		Password: cfg.Password,
		DB:       cfg.DB,
	}
}

func NewClient(cfg config.Asynq) (*Client, func(), error) {
	client := asynq.NewClient(redisOpt(cfg))

	if err := client.Ping(); err != nil {
		_ = client.Close()

		return nil, nil, fmt.Errorf("ping task queue: %w", err)
	}

	cleanup := func() {
		_ = client.Close()
	}

	return &Client{Client: client}, cleanup, nil
}

func NewInspector(cfg config.Asynq) (*Inspector, func(), error) {
	inspector := asynq.NewInspector(redisOpt(cfg))

	if _, err := inspector.Queues(); err != nil {
		_ = inspector.Close()

		return nil, nil, fmt.Errorf("inspect task queues: %w", err)
	}

	cleanup := func() {
		_ = inspector.Close()
	}

	return &Inspector{Inspector: inspector}, cleanup, nil
}

func NewServer(cfg config.Asynq, logger *slog.Logger) *Server {
	server := asynq.NewServer(redisOpt(cfg), asynq.Config{
		Concurrency:     cfg.Concurrency,
		Queues:          cfg.Queues,
		ShutdownTimeout: cfg.ShutdownTimeout,
		Logger:          serverLogger{logger: logger},
		LogLevel:        asynq.InfoLevel,
		BaseContext: func() context.Context {
			return logging.Into(context.Background(), logger)
		},
	})

	return &Server{Server: server}
}
