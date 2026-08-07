package config

import (
	"fmt"
	"net/netip"
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

	if cfg.Licence.Grace < 0 {
		return fmt.Errorf(
			"licence.grace (%s) cannot be negative; a licence that expires is given this long "+
				"before its features stop, and zero means they stop the moment it expires",
			cfg.Licence.Grace,
		)
	}

	if cfg.Audit.Retention <= 0 {
		return fmt.Errorf(
			"audit.retention (%s) must be positive; the audit log always ages out and there is no setting for keeping it forever",
			cfg.Audit.Retention,
		)
	}

	if cfg.Audit.SweepBatch < 1 {
		return fmt.Errorf("audit.sweep_batch (%d) must be at least 1", cfg.Audit.SweepBatch)
	}

	if err := validateWebhooks(cfg.Webhooks); err != nil {
		return err
	}

	if err := validateImports(cfg.Imports); err != nil {
		return err
	}

	if cfg.Imports.MaxUploadBytes > cfg.HTTP.MaxRequestBytes {
		return fmt.Errorf(
			"imports.max_upload_bytes (%d) is above http.max_request_bytes (%d). Every dashboard "+
				"route is capped by the second, so the first would be a promise the server refuses "+
				"to keep: an operator told their file may be that big would watch it rejected at the "+
				"smaller number with nothing naming which limit did it. Raise http.max_request_bytes "+
				"to carry larger imports",
			cfg.Imports.MaxUploadBytes, cfg.HTTP.MaxRequestBytes,
		)
	}

	if err := validateLinear(cfg.Linear); err != nil {
		return err
	}

	if err := validateSourceControl(cfg.SourceControl); err != nil {
		return err
	}

	if cfg.SourceControl.MaxDeliveryBytes > cfg.HTTP.MaxRequestBytes {
		return fmt.Errorf(
			"source_control.max_delivery_bytes (%d) is above http.max_request_bytes (%d). A forge "+
				"chooses its own payload size and retries what it could not deliver, so the smaller "+
				"limit would refuse every large delivery while the connection went on looking healthy. "+
				"Raise http.max_request_bytes to accept larger deliveries",
			cfg.SourceControl.MaxDeliveryBytes, cfg.HTTP.MaxRequestBytes,
		)
	}

	if cfg.Password.BreachCheckEnabled && cfg.Password.BreachCheckTimeout <= 0 {
		return fmt.Errorf(
			"password.breach_check_timeout (%s) must be positive",
			cfg.Password.BreachCheckTimeout,
		)
	}

	if err := validateForwarding(cfg.HTTP); err != nil {
		return err
	}

	return validateStorage(cfg)
}

func validateWebhooks(cfg Webhooks) error {
	if cfg.RequestTimeout <= 0 || cfg.DialTimeout <= 0 {
		return fmt.Errorf(
			"webhooks.request_timeout (%s) and webhooks.dial_timeout (%s) must be positive; a delivery "+
				"attempt has to give up rather than hold a worker against an endpoint that never answers",
			cfg.RequestTimeout, cfg.DialTimeout,
		)
	}

	if cfg.MaxResponseSize < 1<<10 {
		return fmt.Errorf(
			"webhooks.max_response_size (%d) must be at least 1024; the delivery log keeps an excerpt "+
				"of what a receiver said, and a smaller cap would record nothing useful",
			cfg.MaxResponseSize,
		)
	}

	if cfg.Retention <= 0 {
		return fmt.Errorf(
			"webhooks.retention (%s) must be positive; the delivery log always ages out and there is "+
				"no setting for keeping it forever",
			cfg.Retention,
		)
	}

	if cfg.SweepBatch < 1 || cfg.FanOutBatch < 1 {
		return fmt.Errorf(
			"webhooks.sweep_batch (%d) and webhooks.fan_out_batch (%d) must be at least 1",
			cfg.SweepBatch, cfg.FanOutBatch,
		)
	}

	for _, destination := range cfg.AllowedDestinations {
		if _, err := netip.ParsePrefix(strings.TrimSpace(destination)); err != nil {
			return fmt.Errorf(
				"webhooks.allowed_destinations entry %q is not a CIDR prefix: %w. This setting exists so "+
					"a self-hosted receiver on a private network can be reached, and every entry is a hole "+
					"punched in the guard that refuses internal addresses, so an entry that does not parse "+
					"would silently protect nothing",
				destination, err,
			)
		}
	}

	return nil
}

func validateSourceControl(cfg SourceControl) error {
	if cfg.PageSize < 1 || cfg.ReconcileBatch < 1 || cfg.CallsPerCycle < 1 || cfg.MaxAttempts < 1 {
		return fmt.Errorf(
			"source_control.page_size (%d), source_control.reconcile_batch (%d), "+
				"source_control.calls_per_cycle (%d) and source_control.max_attempts (%d) must be "+
				"at least 1. They bound the work one reconcile cycle asks of a forge, which is how "+
				"this instance stays inside a rate limit it does not control",
			cfg.PageSize, cfg.ReconcileBatch, cfg.CallsPerCycle, cfg.MaxAttempts,
		)
	}

	if cfg.MaxResponseSize < 1 {
		return fmt.Errorf(
			"source_control.max_response_size (%d) must be positive. A forge page is refused rather "+
				"than truncated, so a zero cap refuses every response",
			cfg.MaxResponseSize,
		)
	}

	if cfg.MinBackoff > cfg.MaxBackoff {
		return fmt.Errorf(
			"source_control.min_backoff (%s) is above source_control.max_backoff (%s), so a forge "+
				"asking to be left alone would be clamped to a wait shorter than the floor and "+
				"retried straight back into the limit",
			cfg.MinBackoff, cfg.MaxBackoff,
		)
	}

	if cfg.MaxCatchUp <= 0 {
		return fmt.Errorf(
			"source_control.max_catch_up (%s) must be positive. It bounds how far back a repaired "+
				"connection reads; without it a connection broken for a year would try to read a "+
				"year of history in one cycle",
			cfg.MaxCatchUp,
		)
	}

	for _, destination := range cfg.AllowedDestinations {
		if _, err := netip.ParsePrefix(strings.TrimSpace(destination)); err != nil {
			return fmt.Errorf(
				"source_control.allowed_destinations entry %q is not a CIDR prefix: %w. A connection "+
					"names its own host, so this is the allow-list that lets a self-hosted forge on a "+
					"private network be reached, and an entry that does not parse silently protects nothing",
				destination, err,
			)
		}
	}

	return nil
}

func validateForwarding(cfg HTTP) error {
	prefixes, err := cfg.TrustedPrefixes()
	if err != nil {
		return err
	}

	if cfg.ClientIPHeader != "" && len(prefixes) == 0 {
		return fmt.Errorf(
			"http.trusted_proxies is required when http.client_ip_header (%q) is set; without an "+
				"allow-list any caller could forge that header and evade the per-address sign-in throttle",
			cfg.ClientIPHeader,
		)
	}

	return nil
}

func validateImports(cfg Imports) error {
	if cfg.ChunkSize < 1 || cfg.PageSize < 1 || cfg.RescueBatch < 1 || cfg.MaxAttempts < 1 {
		return fmt.Errorf(
			"imports.chunk_size (%d), imports.page_size (%d), imports.rescue_batch (%d) and "+
				"imports.max_attempts (%d) must be at least 1. These size the work an import does at a "+
				"time; none of them caps how much it may carry, which is deliberately unbounded",
			cfg.ChunkSize, cfg.PageSize, cfg.RescueBatch, cfg.MaxAttempts,
		)
	}

	if cfg.SliceBudget <= 0 || cfg.LeaseTTL <= 0 {
		return fmt.Errorf(
			"imports.slice_budget (%s) and imports.lease_ttl (%s) must be positive; an import runs in "+
				"slices so that shutting the worker down loses at most one chunk",
			cfg.SliceBudget, cfg.LeaseTTL,
		)
	}

	if cfg.SliceBudget >= cfg.LeaseTTL {
		return fmt.Errorf(
			"imports.slice_budget (%s) is not shorter than imports.lease_ttl (%s). A slice that outlives "+
				"its lease is rescued while it is still running, and the two workers then write the same "+
				"chunk against each other",
			cfg.SliceBudget, cfg.LeaseTTL,
		)
	}

	if cfg.MinBackoff <= 0 || cfg.MaxBackoff < cfg.MinBackoff {
		return fmt.Errorf(
			"imports.min_backoff (%s) must be positive and imports.max_backoff (%s) must not be shorter; "+
				"a source that asks to be left alone is obeyed only within these bounds",
			cfg.MinBackoff, cfg.MaxBackoff,
		)
	}

	if cfg.RecordRetention <= 0 {
		return fmt.Errorf(
			"imports.record_retention (%s) must be positive; the staged copy of a source's rows is "+
				"working state and always ages out, while the ledger and the report do not",
			cfg.RecordRetention,
		)
	}

	if cfg.MaxAttachmentBytes < 1 || cfg.MaxUploadBytes < 1 {
		return fmt.Errorf(
			"imports.max_attachment_bytes (%d) and imports.max_upload_bytes (%d) must be positive. "+
				"Each bounds one object an import moves in a single read, and a worker that would "+
				"hold an unbounded body in memory has no way to refuse it",
			cfg.MaxAttachmentBytes, cfg.MaxUploadBytes,
		)
	}

	return nil
}

func validateLinear(cfg Linear) error {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return fmt.Errorf(
			"linear.endpoint is required; it is where an import reads a Linear workspace from, and " +
				"it is settable so an instance can be pointed at a test double without one",
		)
	}

	if cfg.RequestTimeout <= 0 {
		return fmt.Errorf(
			"linear.request_timeout (%s) must be positive; a page of issues that never answers "+
				"must give the worker back rather than hold its slot",
			cfg.RequestTimeout,
		)
	}

	if cfg.MaxResponseSize < 1<<20 {
		return fmt.Errorf(
			"linear.max_response_size (%d) must be at least 1048576. A page of issues carrying "+
				"descriptions is megabytes, and a cap below that refuses ordinary data rather than "+
				"protecting anything",
			cfg.MaxResponseSize,
		)
	}

	if cfg.PageSize < 1 || cfg.PageSize > 250 {
		return fmt.Errorf(
			"linear.page_size (%d) must be between 1 and 250, which is what the source itself accepts",
			cfg.PageSize,
		)
	}

	return nil
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
	v.SetDefault("licence.key", "")
	v.SetDefault("licence.grace", 30*24*time.Hour)
	v.SetDefault("audit.retention", 365*24*time.Hour)
	v.SetDefault("audit.sweep_schedule", "0 4 * * *")
	v.SetDefault("audit.sweep_batch", 5000)

	v.SetDefault("http.addr", "127.0.0.1:8080")
	v.SetDefault("http.read_header_timeout", 5*time.Second)
	v.SetDefault("http.read_timeout", 30*time.Second)
	v.SetDefault("http.write_timeout", 30*time.Second)
	v.SetDefault("http.idle_timeout", 120*time.Second)
	v.SetDefault("http.request_timeout", 25*time.Second)
	v.SetDefault("http.shutdown_timeout", 20*time.Second)
	v.SetDefault("http.max_request_bytes", int64(4<<20))
	v.SetDefault("http.client_ip_header", "")
	v.SetDefault("http.trusted_proxies", []string{})

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
	v.SetDefault("asynq.queues", map[string]int{"default": 6, "mail": 3, "webhook": 3, "import": 1, "scm": 2})
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

	v.SetDefault("worker.health_addr", "127.0.0.1:8090")
	v.SetDefault("worker.shutdown_timeout", 10*time.Second)
	v.SetDefault("mcp.enabled", false)
	v.SetDefault("mcp.access_token_ttl", time.Hour)
	v.SetDefault("mcp.refresh_token_ttl", 2160*time.Hour)
	v.SetDefault("mcp.auth_request_ttl", 10*time.Minute)
	v.SetDefault("mcp.auth_code_ttl", 2*time.Minute)
	v.SetDefault("mcp.requests_per_window", 300)
	v.SetDefault("mcp.rate_window", time.Minute)

	v.SetDefault("webhooks.fan_out_schedule", "* * * * *")
	v.SetDefault("webhooks.fan_out_batch", 200)
	v.SetDefault("webhooks.request_timeout", 10*time.Second)
	v.SetDefault("webhooks.dial_timeout", 5*time.Second)
	v.SetDefault("webhooks.max_response_size", int64(64<<10))
	v.SetDefault("webhooks.secret_grace", 24*time.Hour)
	v.SetDefault("webhooks.retention", 30*24*time.Hour)
	v.SetDefault("webhooks.sweep_schedule", "0 5 * * *")
	v.SetDefault("webhooks.sweep_batch", 5000)
	v.SetDefault("webhooks.allowed_destinations", []string{})

	v.SetDefault("imports.chunk_size", 25)
	v.SetDefault("imports.page_size", 200)
	v.SetDefault("imports.slice_budget", 45*time.Second)
	v.SetDefault("imports.lease_ttl", 2*time.Minute)
	v.SetDefault("imports.rescue_schedule", "* * * * *")
	v.SetDefault("imports.rescue_batch", 50)
	v.SetDefault("imports.max_attempts", 20)
	v.SetDefault("imports.min_backoff", time.Second)
	v.SetDefault("imports.max_backoff", 15*time.Minute)
	v.SetDefault("imports.record_retention", 30*24*time.Hour)
	v.SetDefault("imports.max_attachment_bytes", int64(25<<20))
	v.SetDefault("imports.max_upload_bytes", int64(4<<20))

	v.SetDefault("linear.endpoint", "https://api.linear.app/graphql")
	v.SetDefault("linear.request_timeout", 30*time.Second)
	v.SetDefault("linear.max_response_size", int64(32<<20))
	v.SetDefault("linear.page_size", 100)

	v.SetDefault("source_control.github_endpoint", "https://api.github.com")
	v.SetDefault("source_control.gitlab_endpoint", "https://gitlab.com")
	v.SetDefault("source_control.request_timeout", 30*time.Second)
	v.SetDefault("source_control.dial_timeout", 5*time.Second)
	v.SetDefault("source_control.max_response_size", int64(8<<20))
	v.SetDefault("source_control.max_delivery_bytes", int64(4<<20))
	v.SetDefault("source_control.page_size", 100)
	v.SetDefault("source_control.reconcile_schedule", "*/5 * * * *")
	v.SetDefault("source_control.reconcile_batch", 20)
	v.SetDefault("source_control.calls_per_cycle", 30)
	v.SetDefault("source_control.max_catch_up", 168*time.Hour)
	v.SetDefault("source_control.max_attempts", 5)
	v.SetDefault("source_control.min_backoff", 30*time.Second)
	v.SetDefault("source_control.max_backoff", time.Hour)
	v.SetDefault("source_control.delivery_retention", 720*time.Hour)
	v.SetDefault("source_control.allowed_destinations", []string{})

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
