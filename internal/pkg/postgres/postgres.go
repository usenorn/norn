package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/usenorn/norn/internal/config"
)

type Client struct {
	*sql.DB

	pool *pgxpool.Pool
}

func New(cfg config.Postgres) (*Client, func(), error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, nil, fmt.Errorf("parse postgres dsn: %w", err)
	}

	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolConfig.ConnConfig.ConnectTimeout = cfg.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("create postgres pool: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, nil, fmt.Errorf("ping postgres: %w", err)
	}

	db := stdlib.OpenDBFromPool(pool)

	client := &Client{DB: db, pool: pool}

	cleanup := func() {
		_ = db.Close()
		pool.Close()
	}

	return client, cleanup, nil
}
