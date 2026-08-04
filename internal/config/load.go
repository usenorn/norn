package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

const envPrefix = "NORN"

func New(cfgFile string) (Config, error) {
	if err := loadDotenv(); err != nil {
		return Config{}, fmt.Errorf("load %s: %w", dotenvFile, err)
	}

	v := viper.New()

	setDefaults(v)

	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)

		if err := v.ReadInConfig(); err != nil {
			return Config{}, fmt.Errorf("read config file %q: %w", cfgFile, err)
		}
	}

	var cfg Config

	decode := viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	))

	if err := v.Unmarshal(&cfg, decode); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func validate(cfg Config) error {
	if cfg.Session.RefreshInterval >= cfg.Session.IdleTimeout {
		return fmt.Errorf(
			"session.refresh_interval (%s) must be shorter than session.idle_timeout (%s)",
			cfg.Session.RefreshInterval, cfg.Session.IdleTimeout,
		)
	}

	if cfg.Session.IdleTimeout > cfg.Session.AbsoluteLifetime {
		return fmt.Errorf(
			"session.idle_timeout (%s) must not exceed session.absolute_lifetime (%s)",
			cfg.Session.IdleTimeout, cfg.Session.AbsoluteLifetime,
		)
	}

	if cfg.Session.MaxPerAccount < 1 {
		return fmt.Errorf("session.max_per_account (%d) must be at least 1", cfg.Session.MaxPerAccount)
	}

	if (cfg.SMTP.Host == "") != (cfg.SMTP.FromAddress == "") {
		return fmt.Errorf("smtp.host and smtp.from_address must be set together")
	}

	if cfg.Password.BreachCheckEnabled && cfg.Password.BreachCheckEndpoint == "" {
		return fmt.Errorf("password.breach_check_endpoint is required when password.breach_check_enabled is set")
	}

	if cfg.Password.BreachCheckEnabled && cfg.Password.BreachCheckTimeout <= 0 {
		return fmt.Errorf(
			"password.breach_check_timeout (%s) must be positive",
			cfg.Password.BreachCheckTimeout,
		)
	}

	return nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "norn")
	v.SetDefault("app.env", "development")
	v.SetDefault("app.version", "dev")
	v.SetDefault("app.base_url", "http://localhost:5173")
	v.SetDefault("app.log_level", "info")

	v.SetDefault("instance.signups_open", true)
	v.SetDefault("instance.password_auth", true)
	v.SetDefault("instance.self_hosted", false)

	v.SetDefault("http.addr", "127.0.0.1:8080")
	v.SetDefault("http.read_header_timeout", 5*time.Second)
	v.SetDefault("http.read_timeout", 30*time.Second)
	v.SetDefault("http.write_timeout", 30*time.Second)
	v.SetDefault("http.idle_timeout", 120*time.Second)
	v.SetDefault("http.request_timeout", 25*time.Second)
	v.SetDefault("http.shutdown_timeout", 20*time.Second)
	v.SetDefault("http.max_request_bytes", int64(4<<20))

	v.SetDefault("postgres.dsn", "postgres://norn:norn@127.0.0.1:5433/norn?sslmode=disable")
	v.SetDefault("postgres.max_conns", int32(10))
	v.SetDefault("postgres.min_conns", int32(1))
	v.SetDefault("postgres.max_conn_lifetime", time.Hour)
	v.SetDefault("postgres.max_conn_idle_time", 30*time.Minute)
	v.SetDefault("postgres.connect_timeout", 10*time.Second)

	v.SetDefault("valkey.addr", "127.0.0.1:6381")
	v.SetDefault("valkey.username", "")
	v.SetDefault("valkey.password", "")
	v.SetDefault("valkey.db", 0)
	v.SetDefault("valkey.pool_size", 10)
	v.SetDefault("valkey.dial_timeout", 5*time.Second)
	v.SetDefault("valkey.read_timeout", 3*time.Second)
	v.SetDefault("valkey.write_timeout", 3*time.Second)

	v.SetDefault("security.encryption_key", "")

	v.SetDefault("oidc.request_timeout", 10*time.Second)
	v.SetDefault("oidc.max_response_size", 1<<20)
	v.SetDefault("oidc.state_ttl", 10*time.Minute)
	v.SetDefault("asynq.addr", "127.0.0.1:6381")
	v.SetDefault("asynq.username", "")
	v.SetDefault("asynq.password", "")
	v.SetDefault("asynq.db", 1)
	v.SetDefault("asynq.concurrency", 10)
	v.SetDefault("asynq.queues", map[string]int{"default": 6, "mail": 3})
	v.SetDefault("asynq.shutdown_timeout", 20*time.Second)
	v.SetDefault("asynq.max_retry", 5)

	v.SetDefault("smtp.host", "")
	v.SetDefault("smtp.port", 1025)
	v.SetDefault("smtp.username", "")
	v.SetDefault("smtp.password", "")
	v.SetDefault("smtp.auth_type", "none")
	v.SetDefault("smtp.tls_policy", "none")
	v.SetDefault("smtp.from_address", "")
	v.SetDefault("smtp.from_name", "Norn")
	v.SetDefault("smtp.timeout", 15*time.Second)

	v.SetDefault("password.breach_check_enabled", true)
	v.SetDefault("password.breach_check_endpoint", "https://api.pwnedpasswords.com/range")
	v.SetDefault("password.breach_check_timeout", 5*time.Second)

	v.SetDefault("workspace.deletion_grace_period", 720*time.Hour)

	v.SetDefault("storage.endpoint", "http://127.0.0.1:3900")
	v.SetDefault("storage.region", "garage")
	v.SetDefault("storage.bucket", "norn-local")
	v.SetDefault("storage.access_key_id", "")
	v.SetDefault("storage.secret_access_key", "")
	v.SetDefault("storage.use_path_style", true)
	v.SetDefault("storage.public_base_url", "")
	v.SetDefault("storage.timeout", 30*time.Second)

	v.SetDefault("session.cookie_name", "norn_session")
	v.SetDefault("session.cookie_path", "/")
	v.SetDefault("session.domain", "")
	v.SetDefault("session.secure", false)
	v.SetDefault("session.same_site", "lax")
	v.SetDefault("session.key_prefix", "session:")
	v.SetDefault("session.idle_timeout", 168*time.Hour)
	v.SetDefault("session.absolute_lifetime", 720*time.Hour)
	v.SetDefault("session.refresh_interval", time.Minute)
	v.SetDefault("session.max_per_account", 20)

	v.SetDefault("geoip.database_path", "")

	v.SetDefault("casbin.table_name", "casbin_rule")
	v.SetDefault("casbin.watcher_channel", "casbin:policy")
}
