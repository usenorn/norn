package config

func NewApp(cfg Config) App { return cfg.App }

func NewSecurity(cfg Config) Security { return cfg.Security }

func NewOIDC(cfg Config) OIDC { return cfg.OIDC }

func NewSAML(cfg Config) SAML { return cfg.SAML }

func NewCycles(cfg Config) Cycles { return cfg.Cycles }

func NewInstance(cfg Config) Instance { return cfg.Instance }

func NewLicence(cfg Config) Licence { return cfg.Licence }

func NewAudit(cfg Config) Audit { return cfg.Audit }

func NewHTTP(cfg Config) HTTP { return cfg.HTTP }

func NewPostgres(cfg Config) Postgres { return cfg.Postgres }

func NewValkey(cfg Config) Valkey { return cfg.Valkey }

func NewAsynq(cfg Config) Asynq { return cfg.Asynq }

func NewSMTP(cfg Config) SMTP { return cfg.SMTP }

func NewStorage(cfg Config) Storage { return cfg.Storage }

func NewAttachments(cfg Config) Attachments { return cfg.Attachments }

func NewNotifications(cfg Config) Notifications { return cfg.Notifications }

func NewRealtime(cfg Config) Realtime { return cfg.Realtime }

func NewAPITokens(cfg Config) APITokens { return cfg.APITokens }

func NewMCP(cfg Config) MCP { return cfg.MCP }

func NewSession(cfg Config) Session { return cfg.Session }

func NewCasbin(cfg Config) Casbin { return cfg.Casbin }

func NewGeoIP(cfg Config) GeoIP { return cfg.GeoIP }

func NewPassword(cfg Config) Password { return cfg.Password }

func NewWorkspace(cfg Config) Workspace { return cfg.Workspace }
