package config

func NewApp(cfg Config) App { return cfg.App }

func NewInstance(cfg Config) Instance { return cfg.Instance }

func NewHTTP(cfg Config) HTTP { return cfg.HTTP }

func NewPostgres(cfg Config) Postgres { return cfg.Postgres }

func NewValkey(cfg Config) Valkey { return cfg.Valkey }

func NewAsynq(cfg Config) Asynq { return cfg.Asynq }

func NewSMTP(cfg Config) SMTP { return cfg.SMTP }

func NewStorage(cfg Config) Storage { return cfg.Storage }

func NewSession(cfg Config) Session { return cfg.Session }

func NewCasbin(cfg Config) Casbin { return cfg.Casbin }

func NewGeoIP(cfg Config) GeoIP { return cfg.GeoIP }

func NewPassword(cfg Config) Password { return cfg.Password }

func NewWorkspace(cfg Config) Workspace { return cfg.Workspace }
