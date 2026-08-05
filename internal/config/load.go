package config

import (
	"fmt"
	"net/url"
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

	return validateStorage(cfg)
}

func validateStorage(cfg Config) error {
	switch cfg.Storage.Backend {
	case StorageBackendFilesystem:
		if cfg.Storage.Root == "" {
			return fmt.Errorf("storage.root is required when storage.backend is %q", StorageBackendFilesystem)
		}
	case StorageBackendS3:
		if err := separateOrigins(cfg.App.BaseURL, storageOrigin(cfg.Storage)); err != nil {
			return err
		}
	default:
		return fmt.Errorf(
			"storage.backend (%q) must be %q or %q",
			cfg.Storage.Backend, StorageBackendFilesystem, StorageBackendS3,
		)
	}

	if cfg.Attachments.MaxFileBytes <= 0 {
		return fmt.Errorf("attachments.max_file_bytes (%d) must be positive", cfg.Attachments.MaxFileBytes)
	}

	if cfg.Attachments.MaxWorkspaceBytes < 0 {
		return fmt.Errorf(
			"attachments.max_workspace_bytes (%d) must not be negative; 0 means unlimited",
			cfg.Attachments.MaxWorkspaceBytes,
		)
	}

	if cfg.Attachments.MaxWorkspaceBytes > 0 && cfg.Attachments.MaxWorkspaceBytes < cfg.Attachments.MaxFileBytes {
		return fmt.Errorf(
			"attachments.max_workspace_bytes (%d) is below attachments.max_file_bytes (%d), so no file could ever be stored",
			cfg.Attachments.MaxWorkspaceBytes, cfg.Attachments.MaxFileBytes,
		)
	}

	if cfg.Attachments.UploadTTL <= 0 || cfg.Attachments.LinkTTL <= 0 {
		return fmt.Errorf("attachments.upload_ttl and attachments.link_ttl must be positive")
	}

	return nil
}

func storageOrigin(cfg Storage) string {
	if cfg.PublicBaseURL != "" {
		return cfg.PublicBaseURL
	}

	return cfg.Endpoint
}

func separateOrigins(app, storage string) error {
	appURL, err := url.Parse(app)
	if err != nil {
		return fmt.Errorf("app.base_url is not a URL: %w", err)
	}

	storageURL, err := url.Parse(storage)
	if err != nil {
		return fmt.Errorf("storage endpoint is not a URL: %w", err)
	}

	if appURL.Host != "" && appURL.Host == storageURL.Host {
		return fmt.Errorf(
			"storage must not share a host with app.base_url (%s); uploaded files would then be served by an origin that holds the session cookie",
			appURL.Host,
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

	v.SetDefault("saml.certificate_sweep_schedule", "0 8 * * *")

	v.SetDefault("cycles.generation_schedule", "5 0 * * *")
	v.SetDefault("saml.request_timeout", 10*time.Second)
	v.SetDefault("saml.max_response_size", 1<<20)
	v.SetDefault("saml.state_ttl", 10*time.Minute)
	v.SetDefault("saml.replay_ttl", 30*time.Minute)
	v.SetDefault("saml.max_clock_skew", 3*time.Minute)
	v.SetDefault("saml.max_issue_delay", 90*time.Second)
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

	v.SetDefault("storage.backend", StorageBackendFilesystem)
	v.SetDefault("storage.root", "./data/blobs")
	v.SetDefault("storage.endpoint", "http://127.0.0.1:3900")
	v.SetDefault("storage.region", "garage")
	v.SetDefault("storage.bucket", "norn-local")
	v.SetDefault("storage.access_key_id", "")
	v.SetDefault("storage.secret_access_key", "")
	v.SetDefault("storage.use_path_style", true)
	v.SetDefault("storage.public_base_url", "")
	v.SetDefault("storage.timeout", 30*time.Second)
	v.SetDefault("attachments.max_file_bytes", int64(25<<20))
	v.SetDefault("attachments.max_workspace_bytes", int64(0))
	v.SetDefault("attachments.upload_ttl", 15*time.Minute)
	v.SetDefault("attachments.link_ttl", 5*time.Minute)
	v.SetDefault("attachments.transfer_timeout", 10*time.Minute)
	v.SetDefault("attachments.reclaim_schedule", "*/5 * * * *")
	v.SetDefault("realtime.enabled", true)
	v.SetDefault("notifications.fanout_schedule", "* * * * *")
	v.SetDefault("notifications.digest_schedule", "*/15 * * * *")
	v.SetDefault("api_tokens.expiry_sweep_schedule", "0 9 * * *")
	v.SetDefault("attachments.reclaim_batch", 200)

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
