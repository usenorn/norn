package valkey

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/usenorn/norn/internal/config"
)

type Client struct {
	redis.UniversalClient
}

func New(cfg config.Valkey) (*Client, func(), error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()

		return nil, nil, fmt.Errorf("ping valkey: %w", err)
	}

	cleanup := func() {
		_ = client.Close()
	}

	return &Client{UniversalClient: client}, cleanup, nil
}
