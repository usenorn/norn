package authz

import (
	_ "embed"
	"fmt"

	sqladapter "github.com/Blank-Xu/sql-adapter"
	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/pkg/valkey"
)

const driverName = "pgx"

//go:embed model.conf
var modelSource string

type Enforcer struct {
	*casbin.SyncedEnforcer
}

func New(cfg config.Casbin, db *postgres.Client, cache *valkey.Client) (*Enforcer, func(), error) {
	policyModel, err := model.NewModelFromString(modelSource)
	if err != nil {
		return nil, nil, fmt.Errorf("parse casbin model: %w", err)
	}

	adapter, err := sqladapter.NewAdapter(db.DB, driverName, cfg.TableName)
	if err != nil {
		return nil, nil, fmt.Errorf("create casbin adapter: %w", err)
	}

	enforcer, err := casbin.NewSyncedEnforcer(policyModel, adapter)
	if err != nil {
		return nil, nil, fmt.Errorf("create casbin enforcer: %w", err)
	}

	policyWatcher, cleanup, err := newWatcher(cache, cfg.WatcherChannel)
	if err != nil {
		return nil, nil, err
	}

	if err := enforcer.SetWatcher(policyWatcher); err != nil {
		cleanup()

		return nil, nil, fmt.Errorf("attach casbin watcher: %w", err)
	}

	if err := policyWatcher.SetUpdateCallback(func(string) {
		_ = enforcer.LoadPolicy()
	}); err != nil {
		cleanup()

		return nil, nil, fmt.Errorf("set casbin watcher callback: %w", err)
	}

	return &Enforcer{SyncedEnforcer: enforcer}, cleanup, nil
}
