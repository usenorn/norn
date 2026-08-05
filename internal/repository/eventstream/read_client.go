package eventstream

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/pkg/valkey"
)

type ReadClient struct {
	*valkey.Client
}

func NewReadClient(cfg config.Valkey) (*ReadClient, func(), error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  -1,
		WriteTimeout: cfg.WriteTimeout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()

		return nil, nil, fmt.Errorf("ping valkey event reader: %w", err)
	}

	cleanup := func() {
		_ = client.Close()
	}

	return &ReadClient{Client: &valkey.Client{UniversalClient: client}}, cleanup, nil
}
