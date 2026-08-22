package config

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

type Config struct {
	App           App           `mapstructure:"app"`
	Instance      Instance      `mapstructure:"instance"`
	Licence       Licence       `mapstructure:"licence"`
	Audit         Audit         `mapstructure:"audit"`
	HTTP          HTTP          `mapstructure:"http"`
	Postgres      Postgres      `mapstructure:"postgres"`
	Valkey        Valkey        `mapstructure:"valkey"`
	Asynq         Asynq         `mapstructure:"asynq"`
	SMTP          SMTP          `mapstructure:"smtp"`
	Storage       Storage       `mapstructure:"storage"`
	Attachments   Attachments   `mapstructure:"attachments"`
	Notifications Notifications `mapstructure:"notifications"`
	Realtime      Realtime      `mapstructure:"realtime"`
	APITokens     APITokens     `mapstructure:"api_tokens"`
	Worker        Worker        `mapstructure:"worker"`
	MCP           MCP           `mapstructure:"mcp"`
	Webhooks      Webhooks      `mapstructure:"webhooks"`
	Imports       Imports       `mapstructure:"imports"`
	Linear        Linear        `mapstructure:"linear"`
	SourceControl SourceControl `mapstructure:"source_control"`
	Session       Session       `mapstructure:"session"`
	Casbin        Casbin        `mapstructure:"casbin"`
	GeoIP         GeoIP         `mapstructure:"geoip"`
	Password      Password      `mapstructure:"password"`
	Workspace     Workspace     `mapstructure:"workspace"`
	Security      Security      `mapstructure:"security"`
	OIDC          OIDC          `mapstructure:"oidc"`
	SAML          SAML          `mapstructure:"saml"`
	Cycles        Cycles        `mapstructure:"cycles"`
	Runner        Runner        `mapstructure:"runner"`
	Executions    Executions    `mapstructure:"executions"`
}

type Licence struct {
	Key   string        `mapstructure:"key"`
	Grace time.Duration `mapstructure:"grace"`
}

type Audit struct {
	Retention     time.Duration `mapstructure:"retention"`
	SweepSchedule string        `mapstructure:"sweep_schedule"`
	SweepBatch    int           `mapstructure:"sweep_batch"`
}

func (c Audit) RetentionDays() int {
	return int(c.Retention.Hours() / 24)
}

type Security struct {
	EncryptionKey string `mapstructure:"encryption_key"`
}

type OIDC struct {
	RequestTimeout  time.Duration `mapstructure:"request_timeout"`
	MaxResponseSize int64         `mapstructure:"max_response_size"`
	StateTTL        time.Duration `mapstructure:"state_ttl"`
}

type Executions struct {
	LeaseSweepSchedule string `mapstructure:"lease_sweep_schedule"`
}

type Runner struct {
	ChannelEnabled bool          `mapstructure:"channel_enabled"`
	AccessTTL      time.Duration `mapstructure:"access_ttl"`
	TicketTTL      time.Duration `mapstructure:"ticket_ttl"`
	NonceTTL       time.Duration `mapstructure:"nonce_ttl"`
	MaxClockSkew   time.Duration `mapstructure:"max_clock_skew"`
}

type SAML struct {
	CertificateSweepSchedule string        `mapstructure:"certificate_sweep_schedule"`
	RequestTimeout           time.Duration `mapstructure:"request_timeout"`
	MaxResponseSize          int64         `mapstructure:"max_response_size"`
	StateTTL                 time.Duration `mapstructure:"state_ttl"`
	ReplayTTL                time.Duration `mapstructure:"replay_ttl"`
	MaxClockSkew             time.Duration `mapstructure:"max_clock_skew"`
	MaxIssueDelay            time.Duration `mapstructure:"max_issue_delay"`
}

type Cycles struct {
	GenerationSchedule string `mapstructure:"generation_schedule"`
}

type Workspace struct {
	DeletionGracePeriod time.Duration `mapstructure:"deletion_grace_period"`
}

type App struct {
	Name     string `mapstructure:"name"`
	Env      string `mapstructure:"env"`
	Version  string `mapstructure:"version"`
	BaseURL  string `mapstructure:"base_url"`
	LogLevel string `mapstructure:"log_level"`
}

func (c App) Host() string {
	parsed, err := url.Parse(c.BaseURL)
	if err != nil {
		return ""
	}

	return parsed.Host
}

type Instance struct {
	SignupsOpen  bool `mapstructure:"signups_open"`
	PasswordAuth bool `mapstructure:"password_auth"`
	SelfHosted   bool `mapstructure:"self_hosted"`
}

type HTTP struct {
	Addr              string        `mapstructure:"addr"`
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"`
	IdleTimeout       time.Duration `mapstructure:"idle_timeout"`
	RequestTimeout    time.Duration `mapstructure:"request_timeout"`
	ShutdownTimeout   time.Duration `mapstructure:"shutdown_timeout"`
	MaxRequestBytes   int64         `mapstructure:"max_request_bytes"`
	ClientIPHeader    string        `mapstructure:"client_ip_header"`
	TrustedProxies    []string      `mapstructure:"trusted_proxies"`
}

func (c HTTP) TrustedPrefixes() ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(c.TrustedProxies))

	for _, entry := range c.TrustedProxies {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}

		prefix, err := parsePrefix(trimmed)
		if err != nil {
			return nil, fmt.Errorf("http.trusted_proxies entry %q: %w", trimmed, err)
		}

		prefixes = append(prefixes, prefix)
	}

	return prefixes, nil
}

func parsePrefix(entry string) (netip.Prefix, error) {
	if prefix, err := netip.ParsePrefix(entry); err == nil {
		return prefix.Masked(), nil
	}

	addr, err := netip.ParseAddr(entry)
	if err != nil {
		return netip.Prefix{}, errors.New("must be an IP address or a CIDR block")
	}

	unmapped := addr.Unmap()

	return netip.PrefixFrom(unmapped, unmapped.BitLen()), nil
}

type GeoIP struct {
	DatabasePath string `mapstructure:"database_path"`
}

type Postgres struct {
	DSN             string        `mapstructure:"dsn"`
	MaxConns        int32         `mapstructure:"max_conns"`
	MinConns        int32         `mapstructure:"min_conns"`
	MaxConnLifetime time.Duration `mapstructure:"max_conn_lifetime"`
	MaxConnIdleTime time.Duration `mapstructure:"max_conn_idle_time"`
	ConnectTimeout  time.Duration `mapstructure:"connect_timeout"`
}

type Valkey struct {
	Addr         string        `mapstructure:"addr"`
	Username     string        `mapstructure:"username"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	PoolSize     int           `mapstructure:"pool_size"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type Asynq struct {
	Addr            string         `mapstructure:"addr"`
	Username        string         `mapstructure:"username"`
	Password        string         `mapstructure:"password"`
	DB              int            `mapstructure:"db"`
	Concurrency     int            `mapstructure:"concurrency"`
	Queues          map[string]int `mapstructure:"queues"`
	ShutdownTimeout time.Duration  `mapstructure:"shutdown_timeout"`
	MaxRetry        int            `mapstructure:"max_retry"`
}

type SMTP struct {
	Host        string        `mapstructure:"host"`
	Port        int           `mapstructure:"port"`
	Username    string        `mapstructure:"username"`
	Password    string        `mapstructure:"password"`
	AuthType    string        `mapstructure:"auth_type"`
	TLSPolicy   string        `mapstructure:"tls_policy"`
	FromAddress string        `mapstructure:"from_address"`
	FromName    string        `mapstructure:"from_name"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

func (c SMTP) Configured() bool {
	return c.Host != "" && c.FromAddress != ""
}

type Password struct {
	BreachCheckEnabled  bool          `mapstructure:"breach_check_enabled"`
	BreachCheckEndpoint string        `mapstructure:"breach_check_endpoint"`
	BreachCheckTimeout  time.Duration `mapstructure:"breach_check_timeout"`
}

const (
	StorageBackendFilesystem = "filesystem"
	StorageBackendS3         = "s3"
)

type Storage struct {
	Backend         string        `mapstructure:"backend"`
	Root            string        `mapstructure:"root"`
	Endpoint        string        `mapstructure:"endpoint"`
	Region          string        `mapstructure:"region"`
	Bucket          string        `mapstructure:"bucket"`
	AccessKeyID     string        `mapstructure:"access_key_id"`
	SecretAccessKey string        `mapstructure:"secret_access_key"`
	UsePathStyle    bool          `mapstructure:"use_path_style"`
	PublicBaseURL   string        `mapstructure:"public_base_url"`
	Timeout         time.Duration `mapstructure:"timeout"`
}

type Attachments struct {
	MaxFileBytes      int64         `mapstructure:"max_file_bytes"`
	MaxWorkspaceBytes int64         `mapstructure:"max_workspace_bytes"`
	UploadTTL         time.Duration `mapstructure:"upload_ttl"`
	LinkTTL           time.Duration `mapstructure:"link_ttl"`
	TransferTimeout   time.Duration `mapstructure:"transfer_timeout"`
	ReclaimSchedule   string        `mapstructure:"reclaim_schedule"`
	ReclaimBatch      int           `mapstructure:"reclaim_batch"`
}

type Notifications struct {
	FanOutSchedule string `mapstructure:"fanout_schedule"`
	DigestSchedule string `mapstructure:"digest_schedule"`
}

type Realtime struct {
	Enabled       bool `mapstructure:"enabled"`
	MaxPerAccount int  `mapstructure:"max_per_account"`
}

type APITokens struct {
	ExpirySweepSchedule string `mapstructure:"expiry_sweep_schedule"`
}

type Webhooks struct {
	FanOutSchedule      string        `mapstructure:"fan_out_schedule"`
	FanOutBatch         int           `mapstructure:"fan_out_batch"`
	RequestTimeout      time.Duration `mapstructure:"request_timeout"`
	DialTimeout         time.Duration `mapstructure:"dial_timeout"`
	MaxResponseSize     int64         `mapstructure:"max_response_size"`
	SecretGrace         time.Duration `mapstructure:"secret_grace"`
	Retention           time.Duration `mapstructure:"retention"`
	SweepSchedule       string        `mapstructure:"sweep_schedule"`
	SweepBatch          int           `mapstructure:"sweep_batch"`
	AllowedDestinations []string      `mapstructure:"allowed_destinations"`
}

func (c Webhooks) RetentionDays() int { return int(c.Retention.Hours() / 24) }

type Worker struct {
	HealthAddr      string        `mapstructure:"health_addr"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type Imports struct {
	ChunkSize          int           `mapstructure:"chunk_size"`
	PageSize           int           `mapstructure:"page_size"`
	SliceBudget        time.Duration `mapstructure:"slice_budget"`
	LeaseTTL           time.Duration `mapstructure:"lease_ttl"`
	RescueSchedule     string        `mapstructure:"rescue_schedule"`
	RescueBatch        int           `mapstructure:"rescue_batch"`
	MaxAttempts        int           `mapstructure:"max_attempts"`
	MinBackoff         time.Duration `mapstructure:"min_backoff"`
	MaxBackoff         time.Duration `mapstructure:"max_backoff"`
	RecordRetention    time.Duration `mapstructure:"record_retention"`
	MaxAttachmentBytes int64         `mapstructure:"max_attachment_bytes"`
	MaxUploadBytes     int64         `mapstructure:"max_upload_bytes"`
}

type Linear struct {
	Endpoint        string        `mapstructure:"endpoint"`
	RequestTimeout  time.Duration `mapstructure:"request_timeout"`
	MaxResponseSize int64         `mapstructure:"max_response_size"`
	PageSize        int           `mapstructure:"page_size"`
}

type SourceControl struct {
	GitHubEndpoint      string        `mapstructure:"github_endpoint"`
	GitLabEndpoint      string        `mapstructure:"gitlab_endpoint"`
	RequestTimeout      time.Duration `mapstructure:"request_timeout"`
	DialTimeout         time.Duration `mapstructure:"dial_timeout"`
	MaxResponseSize     int64         `mapstructure:"max_response_size"`
	MaxDeliveryBytes    int64         `mapstructure:"max_delivery_bytes"`
	PageSize            int           `mapstructure:"page_size"`
	ReconcileSchedule   string        `mapstructure:"reconcile_schedule"`
	ReconcileBatch      int           `mapstructure:"reconcile_batch"`
	CallsPerCycle       int           `mapstructure:"calls_per_cycle"`
	MaxCatchUp          time.Duration `mapstructure:"max_catch_up"`
	MaxAttempts         int           `mapstructure:"max_attempts"`
	MinBackoff          time.Duration `mapstructure:"min_backoff"`
	MaxBackoff          time.Duration `mapstructure:"max_backoff"`
	DeliveryRetention   time.Duration `mapstructure:"delivery_retention"`
	AllowedDestinations []string      `mapstructure:"allowed_destinations"`

	AppStateTTL            time.Duration `mapstructure:"app_state_ttl"`
	GitHubAppID            string        `mapstructure:"github_app_id"`
	GitHubAppSlug          string        `mapstructure:"github_app_slug"`
	GitHubAppClientID      string        `mapstructure:"github_app_client_id"`
	GitHubAppClientSecret  string        `mapstructure:"github_app_client_secret"`
	GitHubAppPrivateKey    string        `mapstructure:"github_app_private_key"`
	GitHubAppWebhookSecret string        `mapstructure:"github_app_webhook_secret"`
}

func (c SourceControl) GitHubAppConfigured() bool {
	return c.GitHubAppID != "" && c.GitHubAppPrivateKey != "" && c.GitHubAppWebhookSecret != ""
}

type MCP struct {
	Enabled           bool          `mapstructure:"enabled"`
	RequestsPerWindow int           `mapstructure:"requests_per_window"`
	RateWindow        time.Duration `mapstructure:"rate_window"`
}

type Session struct {
	CookieName       string        `mapstructure:"cookie_name"`
	CookiePath       string        `mapstructure:"cookie_path"`
	Domain           string        `mapstructure:"domain"`
	Secure           bool          `mapstructure:"secure"`
	SameSite         string        `mapstructure:"same_site"`
	KeyPrefix        string        `mapstructure:"key_prefix"`
	IdleTimeout      time.Duration `mapstructure:"idle_timeout"`
	AbsoluteLifetime time.Duration `mapstructure:"absolute_lifetime"`
	RefreshInterval  time.Duration `mapstructure:"refresh_interval"`
	MaxPerAccount    int           `mapstructure:"max_per_account"`
}

type Casbin struct {
	TableName      string `mapstructure:"table_name"`
	WatcherChannel string `mapstructure:"watcher_channel"`
}
