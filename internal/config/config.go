package config

import (
	"net/url"
	"time"
)

type Config struct {
	App       App       `mapstructure:"app"`
	Instance  Instance  `mapstructure:"instance"`
	HTTP      HTTP      `mapstructure:"http"`
	Postgres  Postgres  `mapstructure:"postgres"`
	Valkey    Valkey    `mapstructure:"valkey"`
	Asynq     Asynq     `mapstructure:"asynq"`
	SMTP      SMTP      `mapstructure:"smtp"`
	Storage   Storage   `mapstructure:"storage"`
	Session   Session   `mapstructure:"session"`
	Casbin    Casbin    `mapstructure:"casbin"`
	GeoIP     GeoIP     `mapstructure:"geoip"`
	Password  Password  `mapstructure:"password"`
	Workspace Workspace `mapstructure:"workspace"`
	Security  Security  `mapstructure:"security"`
	OIDC      OIDC      `mapstructure:"oidc"`
	SAML      SAML      `mapstructure:"saml"`
}

type Security struct {
	EncryptionKey string `mapstructure:"encryption_key"`
}

type OIDC struct {
	RequestTimeout  time.Duration `mapstructure:"request_timeout"`
	MaxResponseSize int64         `mapstructure:"max_response_size"`
	StateTTL        time.Duration `mapstructure:"state_ttl"`
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

type Storage struct {
	Endpoint        string        `mapstructure:"endpoint"`
	Region          string        `mapstructure:"region"`
	Bucket          string        `mapstructure:"bucket"`
	AccessKeyID     string        `mapstructure:"access_key_id"`
	SecretAccessKey string        `mapstructure:"secret_access_key"`
	UsePathStyle    bool          `mapstructure:"use_path_style"`
	PublicBaseURL   string        `mapstructure:"public_base_url"`
	Timeout         time.Duration `mapstructure:"timeout"`
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
