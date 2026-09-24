package entity

import (
	"errors"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	AgentMCPServersPerOwner     = 25
	AgentMCPCommandMaxLen       = 1024
	AgentMCPArgsMax             = 64
	AgentMCPArgMaxLen           = 4096
	AgentMCPURLMaxLen           = 2048
	AgentMCPVariablesMax        = 50
	AgentMCPVariableValueMaxLen = 8192
	AgentMCPClientIDMaxLen      = 512
	AgentMCPReservedName        = "norn"
	AgentMCPAuthorizationHeader = "Authorization"
	AgentMCPRegistrySearchMax   = 30
	agentMCPBearerPrefix        = "Bearer "
)

var (
	ErrAgentMCPServerNotFound      = errors.New("mcp server not found")
	ErrAgentMCPServerNameTaken     = errors.New("this agent already has an mcp server with that name")
	ErrAgentMCPServerLimitReached  = errors.New("no more mcp servers can be added here")
	ErrAgentMCPOAuthNotConfigured  = errors.New("this mcp server does not sign in with oauth")
	ErrAgentMCPOAuthUnsupported    = errors.New("this mcp server does not publish how to sign in to it")
	ErrAgentMCPOAuthStateNotFound  = errors.New("this sign-in has already been used, or it took too long")
	ErrAgentMCPOAuthRefused        = errors.New("the authorization server refused the sign-in")
	ErrAgentMCPServerUnreachable   = errors.New("the mcp server could not be reached")
	ErrAgentMCPConnectionNotFound  = errors.New("this mcp server is not signed in")
	ErrAgentMCPRegistryUnreachable = errors.New("the mcp registry could not be reached")
	ErrAgentMCPDestinationRefused  = errors.New("this instance will not open a connection to that address")

	agentMCPEnvKey     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	agentMCPHeaderName = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")
)

type AgentMCPTransport string

const (
	AgentMCPStdio AgentMCPTransport = "stdio"
	AgentMCPHTTP  AgentMCPTransport = "http"
	AgentMCPSSE   AgentMCPTransport = "sse"
)

func (t AgentMCPTransport) Valid() bool {
	return t == AgentMCPStdio || t == AgentMCPHTTP || t == AgentMCPSSE
}

func (t AgentMCPTransport) Remote() bool {
	return t == AgentMCPHTTP || t == AgentMCPSSE
}

type AgentMCPAuth string

const (
	AgentMCPAuthNone    AgentMCPAuth = "none"
	AgentMCPAuthHeaders AgentMCPAuth = "headers"
	AgentMCPAuthOAuth   AgentMCPAuth = "oauth"
)

func (a AgentMCPAuth) Valid() bool {
	return a == AgentMCPAuthNone || a == AgentMCPAuthHeaders || a == AgentMCPAuthOAuth
}

type AgentMCPRegistryRef struct {
	Name    string
	Version string
}

type AgentMCPServer struct {
	ID            uuid.UUID
	WorkspaceID   uuid.UUID
	AgentID       *uuid.UUID
	Name          string
	Transport     AgentMCPTransport
	Command       string
	Args          []string
	URL           string
	Auth          AgentMCPAuth
	EnvKeys       []string
	HeaderKeys    []string
	OAuthClientID string
	Registry      AgentMCPRegistryRef
	CreatedBy     uuid.UUID
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (s AgentMCPServer) InLibrary() bool {
	return s.AgentID == nil
}

type AgentMCPSecrets struct {
	Env               map[string]string
	Headers           map[string]string
	OAuthClientSecret string
}

func (s AgentMCPSecrets) Empty() bool {
	return len(s.Env) == 0 && len(s.Headers) == 0 && s.OAuthClientSecret == ""
}

type AgentMCPServerInput struct {
	Name              string
	Transport         AgentMCPTransport
	Command           string
	Args              []string
	URL               string
	Auth              AgentMCPAuth
	Env               map[string]string
	Headers           map[string]string
	OAuthClientID     string
	OAuthClientSecret string
	Registry          AgentMCPRegistryRef
}

func (in AgentMCPServerInput) Normalized() AgentMCPServerInput {
	out := in
	out.Name = strings.TrimSpace(in.Name)
	out.Command = strings.TrimSpace(in.Command)
	out.URL = strings.TrimSpace(in.URL)
	out.OAuthClientID = strings.TrimSpace(in.OAuthClientID)
	out.OAuthClientSecret = strings.TrimSpace(in.OAuthClientSecret)

	if out.Auth == "" {
		out.Auth = AgentMCPAuthNone
	}

	out.Args = make([]string, 0, len(in.Args))
	for _, argument := range in.Args {
		if trimmed := strings.TrimSpace(argument); trimmed != "" {
			out.Args = append(out.Args, trimmed)
		}
	}

	out.Env = trimmedVariables(in.Env)
	out.Headers = trimmedVariables(in.Headers)

	if !out.Transport.Remote() {
		out.URL, out.Headers, out.Auth, out.OAuthClientID, out.OAuthClientSecret = "", nil, AgentMCPAuthNone, "", ""
	} else {
		out.Command, out.Args, out.Env = "", nil, nil
	}

	if out.Auth != AgentMCPAuthOAuth {
		out.OAuthClientID, out.OAuthClientSecret = "", ""
	}

	if out.Auth != AgentMCPAuthHeaders {
		out.Headers = nil
	}

	return out
}

func trimmedVariables(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}

	trimmed := make(map[string]string, len(values))
	for key, value := range values {
		trimmed[strings.TrimSpace(key)] = value
	}

	return trimmed
}

func (in AgentMCPServerInput) Validate() error {
	fields := []FieldError{ValidateAgentCapabilityName("name", in.Name)}

	if in.Name == AgentMCPReservedName {
		fields = append(fields, FieldError{Field: "name", Code: ValidationCodeTaken})
	}

	if !in.Transport.Valid() {
		fields = append(fields, FieldError{Field: "transport", Code: ValidationCodeUnsupportedValue})
	}

	if !in.Auth.Valid() {
		fields = append(fields, FieldError{Field: "auth", Code: ValidationCodeUnsupportedValue})
	}

	if in.Transport == AgentMCPStdio {
		fields = append(fields, validateMCPCommand(in.Command, in.Args)...)
	}

	if in.Transport.Remote() {
		fields = append(fields, validateMCPURL(in.URL))
	}

	fields = append(fields, validateVariables("env", in.Env, agentMCPEnvKey)...)
	fields = append(fields, validateVariables("headers", in.Headers, agentMCPHeaderName)...)

	if in.Auth == AgentMCPAuthHeaders && len(in.Headers) == 0 {
		fields = append(fields, FieldError{Field: "headers", Code: ValidationCodeRequired})
	}

	if len(in.OAuthClientID) > AgentMCPClientIDMaxLen {
		fields = append(fields, FieldError{Field: "oauthClientId", Code: ValidationCodeTooLong})
	}

	if in.OAuthClientSecret != "" && in.OAuthClientID == "" {
		fields = append(fields, FieldError{Field: "oauthClientId", Code: ValidationCodeRequired})
	}

	return NewValidationError(fields...)
}

func validateMCPCommand(command string, args []string) []FieldError {
	var fields []FieldError

	switch {
	case command == "":
		fields = append(fields, FieldError{Field: "command", Code: ValidationCodeRequired})
	case len(command) > AgentMCPCommandMaxLen || strings.ContainsAny(command, "\x00\n"):
		fields = append(fields, FieldError{Field: "command", Code: ValidationCodeMalformed})
	}

	if len(args) > AgentMCPArgsMax {
		fields = append(fields, FieldError{Field: "args", Code: ValidationCodeTooLong})
	}

	for _, argument := range args {
		if len(argument) > AgentMCPArgMaxLen || strings.ContainsRune(argument, 0) {
			fields = append(fields, FieldError{Field: "args", Code: ValidationCodeMalformed})

			break
		}
	}

	return fields
}

func validateMCPURL(raw string) FieldError {
	if raw == "" {
		return FieldError{Field: "url", Code: ValidationCodeRequired}
	}

	if len(raw) > AgentMCPURLMaxLen {
		return FieldError{Field: "url", Code: ValidationCodeTooLong}
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") ||
		parsed.User != nil || parsed.Fragment != "" {
		return FieldError{Field: "url", Code: ValidationCodeMalformed}
	}

	return FieldError{}
}

func validateVariables(field string, values map[string]string, key *regexp.Regexp) []FieldError {
	if len(values) > AgentMCPVariablesMax {
		return []FieldError{{Field: field, Code: ValidationCodeTooLong}}
	}

	for name, value := range values {
		if !key.MatchString(name) {
			return []FieldError{{Field: field, Code: ValidationCodeMalformed}}
		}

		if len(value) > AgentMCPVariableValueMaxLen || strings.ContainsAny(value, "\x00\r\n") {
			return []FieldError{{Field: field, Code: ValidationCodeMalformed}}
		}
	}

	return nil
}

func AgentMCPSecretRequired(field string) error {
	return NewValidationError(FieldError{Field: field, Code: ValidationCodeRequired})
}

func AgentMCPVariableKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	return keys
}

func AgentMCPBearer(accessToken string) string {
	return agentMCPBearerPrefix + accessToken
}

type AgentMCPConnectionStatus string

const (
	AgentMCPConnected AgentMCPConnectionStatus = "connected"
	AgentMCPExpired   AgentMCPConnectionStatus = "expired"
	AgentMCPFailed    AgentMCPConnectionStatus = "failed"
)

type AgentMCPConnectionFailure string

const (
	AgentMCPFailureRefreshRejected AgentMCPConnectionFailure = "refresh_rejected"
	AgentMCPFailureUnreachable     AgentMCPConnectionFailure = "unreachable"
)

type AgentMCPConnection struct {
	ServerID      uuid.UUID
	WorkspaceID   uuid.UUID
	Status        AgentMCPConnectionStatus
	Issuer        string
	TokenEndpoint string
	ClientID      string
	Scopes        []string
	ExpiresAt     *time.Time
	Failure       AgentMCPConnectionFailure
	ConnectedBy   uuid.UUID
	ConnectedAt   time.Time
	UpdatedAt     time.Time
}

func (c AgentMCPConnection) Current(now time.Time) AgentMCPConnectionStatus {
	if c.Status == AgentMCPConnected && c.ExpiresAt != nil && !c.ExpiresAt.After(now) {
		return AgentMCPExpired
	}

	return c.Status
}

func (c AgentMCPConnection) DueForRefresh(now time.Time, lead time.Duration) bool {
	return c.ExpiresAt != nil && !c.ExpiresAt.After(now.Add(lead))
}

type AgentMCPTokens struct {
	ClientSecret string
	AccessToken  string
	RefreshToken string
	ExpiresAt    *time.Time
}

type AgentMCPAuthServer struct {
	Issuer                string
	AuthorizationEndpoint string
	TokenEndpoint         string
	RegistrationEndpoint  string
	Resource              string
	Scopes                []string
}

type AgentMCPClient struct {
	ID     string
	Secret string
}

type AgentMCPOAuthState struct {
	WorkspaceID   uuid.UUID
	ServerID      uuid.UUID
	AccountID     uuid.UUID
	Verifier      string
	Issuer        string
	TokenEndpoint string
	Resource      string
	Scopes        []string
	Client        AgentMCPClient
	ReturnTo      string
	CreatedAt     time.Time
}

type AgentMCPVariableSpec struct {
	Key         string
	Description string
	Required    bool
	Secret      bool
	Default     string
}

type AgentMCPServerTemplate struct {
	Transport AgentMCPTransport
	Command   string
	Args      []string
	URL       string
	Env       []AgentMCPVariableSpec
	Headers   []AgentMCPVariableSpec
}

type AgentMCPRegistryEntry struct {
	Name        string
	Title       string
	Description string
	Version     string
	WebsiteURL  string
	Repository  string
	Templates   []AgentMCPServerTemplate
}

type AgentToolkitSkill struct {
	Name        string
	ContentHash string
	DownloadURL string
}

type AgentToolkitServer struct {
	Name      string
	Transport AgentMCPTransport
	Command   string
	Args      []string
	Env       map[string]string
	URL       string
	Headers   map[string]string
}

type AgentToolkit struct {
	Skills     []AgentToolkitSkill
	MCPServers []AgentToolkitServer
}

func ValidAgentMCPReturnTo(target string) bool {
	return strings.HasPrefix(target, "/") && !strings.HasPrefix(target, "//") &&
		!strings.ContainsAny(target, "\\\r\n") && utf8.ValidString(target)
}
