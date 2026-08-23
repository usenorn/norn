package config

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"

	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
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

	if strings.EqualFold(cfg.Session.SameSite, "none") {
		return fmt.Errorf(
			"session.same_site cannot be none; that sends the session cookie on requests any " +
				"other site makes, which is the whole shape of a cross-site request forgery",
		)
	}

	if strings.HasPrefix(cfg.App.BaseURL, "https://") && !cfg.Session.Secure {
		return fmt.Errorf(
			"session.secure must be set when app.base_url (%s) is https, or the session cookie "+
				"travels over any plain-http request to the same host",
			cfg.App.BaseURL,
		)
	}

	if cfg.SMTP.Host == "" || cfg.SMTP.FromAddress == "" {
		return fmt.Errorf(
			"smtp.host and smtp.from_address are both required; signing in, confirming an " +
				"address, recovering a password and inviting somebody all send mail, and an " +
				"instance that cannot send it cannot let anybody in",
		)
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

	if err := validateRunner(cfg.Runner); err != nil {
		return err
	}

	if err := validateExecutions(cfg); err != nil {
		return err
	}

	if err := validatePreviews(cfg.App, cfg.Previews); err != nil {
		return err
	}

	if err := validateGateway(cfg.Gateway); err != nil {
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

func validateExecutions(cfg Config) error {
	if cfg.Executions.UploadRetention <= 0 {
		return fmt.Errorf(
			"executions.upload_retention (%s) must be positive; a run's logs and transcript always "+
				"age out and there is no setting for keeping them forever",
			cfg.Executions.UploadRetention,
		)
	}

	if cfg.Executions.RetentionBatch < 1 {
		return fmt.Errorf("executions.retention_batch (%d) must be at least 1", cfg.Executions.RetentionBatch)
	}

	if cfg.Executions.MaxUploadBytes < 1 {
		return fmt.Errorf(
			"executions.max_upload_bytes (%d) must be positive; it is how much one run may store "+
				"before the server starts turning its uploads down",
			cfg.Executions.MaxUploadBytes,
		)
	}

	if cfg.Executions.MaxChunkBytes < 1 {
		return fmt.Errorf("executions.max_chunk_bytes (%d) must be positive", cfg.Executions.MaxChunkBytes)
	}

	if cfg.Executions.MaxChunkBytes > cfg.HTTP.MaxRequestBytes {
		return fmt.Errorf(
			"executions.max_chunk_bytes (%d) is above http.max_request_bytes (%d). A runner posts "+
				"batches over the same routes as the dashboard, so the second cuts the first off, "+
				"and a machine told its batch may be that big would watch it refused at the "+
				"smaller number with nothing naming which limit did it. Raise "+
				"http.max_request_bytes to accept larger batches",
			cfg.Executions.MaxChunkBytes, cfg.HTTP.MaxRequestBytes,
		)
	}

	if cfg.Executions.MaxArtifactBytes < 1 {
		return fmt.Errorf(
			"executions.max_artifact_bytes (%d) must be positive", cfg.Executions.MaxArtifactBytes,
		)
	}

	if cfg.Executions.MaxArtifactBytes >= cfg.HTTP.MaxRequestBytes {
		return fmt.Errorf(
			"executions.max_artifact_bytes (%d) must be below http.max_request_bytes (%d), not "+
				"merely within it. An artifact arrives inside a multipart body, so the envelope "+
				"needs room above the file; at or above the request cap it is the transport that "+
				"cuts the upload off, and a transport has no way to say which file was too big. "+
				"Raise http.max_request_bytes to accept larger artifacts",
			cfg.Executions.MaxArtifactBytes, cfg.HTTP.MaxRequestBytes,
		)
	}

	return nil
}

func validateRunner(cfg Runner) error {
	if !channelv1.Released(cfg.MinimumVersion) {
		return fmt.Errorf(
			"runner.minimum_version (%q) must be a semantic version such as 1.2.0. It is the oldest "+
				"runner this server will open a channel to, and a value it cannot compare would "+
				"either let every machine in or lock every machine out",
			cfg.MinimumVersion,
		)
	}

	if cfg.AccessTTL <= 0 || cfg.TicketTTL <= 0 {
		return fmt.Errorf("runner.access_ttl and runner.ticket_ttl must be positive")
	}

	if cfg.MaxClockSkew <= 0 {
		return fmt.Errorf("runner.max_clock_skew must be positive")
	}

	if cfg.NonceTTL < 2*cfg.MaxClockSkew {
		return fmt.Errorf(
			"runner.nonce_ttl (%s) must be at least twice runner.max_clock_skew (%s). A runner's "+
				"assertion is accepted anywhere inside the skew window in either direction, so a "+
				"nonce that stops being remembered sooner than that window is wide leaves a gap in "+
				"which the same signed assertion can be presented a second time and accepted",
			cfg.NonceTTL, cfg.MaxClockSkew,
		)
	}

	return nil
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

func validatePreviews(app App, cfg Previews) error {
	if cfg.SessionTTL <= 0 || cfg.TicketTTL <= 0 || cfg.GatewayAccessTTL <= 0 {
		return fmt.Errorf(
			"previews.session_ttl, previews.ticket_ttl and previews.gateway_access_ttl must be " +
				"positive",
		)
	}

	if cfg.ShareDefaultTTL <= 0 || cfg.ShareMaxTTL <= 0 {
		return fmt.Errorf("previews.share_default_ttl and previews.share_max_ttl must be positive")
	}

	if cfg.ShareDefaultTTL > cfg.ShareMaxTTL {
		return fmt.Errorf(
			"previews.share_default_ttl (%s) is longer than previews.share_max_ttl (%s), so the "+
				"link a person gets without naming a lifetime would already be past the longest "+
				"one they are allowed to ask for",
			cfg.ShareDefaultTTL, cfg.ShareMaxTTL,
		)
	}

	if cfg.AuditWindow <= 0 {
		return fmt.Errorf("previews.audit_window must be positive")
	}

	if !cfg.Routable() {
		return nil
	}

	if cfg.Scheme != "https" && cfg.Scheme != "http" {
		return fmt.Errorf("previews.scheme (%q) must be http or https", cfg.Scheme)
	}

	return separateDomains(app.Host(), cfg.BaseDomain)
}

func separateDomains(app, previews string) error {
	host := bareHost(app)
	previews = bareHost(previews)

	if host == "" || previews == "" {
		return nil
	}

	if host == previews ||
		strings.HasSuffix(host, "."+previews) ||
		strings.HasSuffix(previews, "."+host) {
		return fmt.Errorf(
			"previews.base_domain (%s) must not share a domain with app.base_url (%s); a preview "+
				"runs code norn did not write, and a browser sends the session cookie to any host "+
				"underneath the domain that set it",
			previews, host,
		)
	}

	return nil
}

func validateGateway(cfg Gateway) error {
	if cfg.Listen == "" {
		return fmt.Errorf("gateway.listen must be set")
	}

	if cfg.MaxStreamsPerRunner <= 0 {
		return fmt.Errorf("gateway.max_streams_per_runner must be positive")
	}

	positive := map[string]time.Duration{
		"gateway.request_timeout":     cfg.RequestTimeout,
		"gateway.stream_open_timeout": cfg.StreamOpenTimeout,
		"gateway.handshake_timeout":   cfg.HandshakeTimeout,
		"gateway.heartbeat":           cfg.Heartbeat,
		"gateway.read_header_timeout": cfg.ReadHeaderTimeout,
		"gateway.shutdown_timeout":    cfg.ShutdownTimeout,
		"gateway.refresh_lead":        cfg.RefreshLead,
		"gateway.retry_min":           cfg.RetryMin,
		"gateway.retry_max":           cfg.RetryMax,
	}

	for name, value := range positive {
		if value <= 0 {
			return fmt.Errorf("%s must be positive", name)
		}
	}

	if cfg.RetryMin > cfg.RetryMax {
		return fmt.Errorf(
			"gateway.retry_min (%s) is longer than gateway.retry_max (%s)",
			cfg.RetryMin, cfg.RetryMax,
		)
	}

	if cfg.Server == "" {
		return nil
	}

	parsed, err := url.Parse(cfg.Server)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf(
			"gateway.server (%q) must be an absolute url such as https://app.example.com",
			cfg.Server,
		)
	}

	return nil
}

func bareHost(address string) string {
	if host, _, err := net.SplitHostPort(address); err == nil {
		return host
	}

	return strings.Trim(address, "[]")
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

	v.SetDefault("runner.channel_enabled", true)
	v.SetDefault("runner.minimum_version", channelv1.MinimumRunner)
	v.SetDefault("runner.access_ttl", 15*time.Minute)
	v.SetDefault("runner.ticket_ttl", time.Minute)
	v.SetDefault("runner.nonce_ttl", 10*time.Minute)
	v.SetDefault("runner.max_clock_skew", 3*time.Minute)
	v.SetDefault("executions.lease_sweep_schedule", "@every 30s")
	v.SetDefault("executions.max_chunk_bytes", int64(1<<20))
	v.SetDefault("executions.max_artifact_bytes", int64(3<<20))
	v.SetDefault("executions.max_upload_bytes", int64(512<<20))
	v.SetDefault("executions.upload_retention", 90*24*time.Hour)
	v.SetDefault("executions.retention_schedule", "0 3 * * *")
	v.SetDefault("questions.expiry_sweep_schedule", "*/15 * * * *")

	v.SetDefault("previews.base_domain", "")
	v.SetDefault("previews.scheme", "https")
	v.SetDefault("previews.session_ttl", 15*time.Minute)
	v.SetDefault("previews.ticket_ttl", time.Minute)
	v.SetDefault("previews.gateway_access_ttl", 15*time.Minute)
	v.SetDefault("previews.share_default_ttl", 24*time.Hour)
	v.SetDefault("previews.share_max_ttl", 7*24*time.Hour)
	v.SetDefault("previews.audit_window", time.Hour)
	v.SetDefault("gateway.listen", ":8091")
	v.SetDefault("gateway.server", "")
	v.SetDefault("gateway.secret", "")
	v.SetDefault("gateway.tunnel_host", "")
	v.SetDefault("gateway.max_streams_per_runner", 256)
	v.SetDefault("gateway.request_timeout", 30*time.Second)
	v.SetDefault("gateway.stream_open_timeout", 10*time.Second)
	v.SetDefault("gateway.handshake_timeout", 10*time.Second)
	v.SetDefault("gateway.heartbeat", 15*time.Second)
	v.SetDefault("gateway.read_header_timeout", 10*time.Second)
	v.SetDefault("gateway.shutdown_timeout", 30*time.Second)
	v.SetDefault("gateway.refresh_lead", 2*time.Minute)
	v.SetDefault("gateway.retry_min", 2*time.Second)
	v.SetDefault("gateway.retry_max", time.Minute)
	v.SetDefault("executions.retention_batch", 200)
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
	v.SetDefault("realtime.max_per_account", 12)
	v.SetDefault("notifications.fanout_schedule", "* * * * *")
	v.SetDefault("notifications.digest_schedule", "*/15 * * * *")
	v.SetDefault("api_tokens.expiry_sweep_schedule", "0 9 * * *")

	v.SetDefault("worker.health_addr", "127.0.0.1:8090")
	v.SetDefault("worker.shutdown_timeout", 10*time.Second)
	v.SetDefault("mcp.enabled", false)
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

	v.SetDefault("source_control.github_app_id", "")
	v.SetDefault("source_control.github_app_slug", "")
	v.SetDefault("source_control.github_app_client_id", "")
	v.SetDefault("source_control.github_app_client_secret", "")
	v.SetDefault("source_control.github_app_private_key", "")
	v.SetDefault("source_control.github_app_webhook_secret", "")
	v.SetDefault("source_control.github_endpoint", "https://api.github.com")
	v.SetDefault("source_control.app_state_ttl", 10*time.Minute)
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
