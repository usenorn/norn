package entity

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	MCPTokenPrefix            = "nmcp_"
	MCPTokenBytes             = 32
	MCPAuthCodeBytes          = 32
	MCPClientNameMaxLen       = 80
	MCPClientMaxRedirect      = 10
	MCPRegistrationsPerWindow = 10
)

var (
	ErrMCPClientNotFound      = errors.New("mcp client not found")
	ErrMCPAuthRequestNotFound = errors.New("mcp authorization request not found or expired")
	ErrMCPCodeInvalid         = errors.New("mcp authorization code is invalid or already used")

	ErrMCPNoWorkspaceChosen = errors.New(
		"choose at least one workspace for this client to reach",
	)
	ErrMCPWorkspaceUnreachable = errors.New(
		"that workspace is not one you can grant a client access to",
	)
	ErrMCPTokenNotFound       = errors.New("mcp token not found")
	ErrMCPConnectionNotFound  = errors.New("mcp connection not found")
	ErrMCPConnectionRevoked   = errors.New("mcp connection has been revoked")
	ErrMCPScopeInvalid        = errors.New("mcp scope is not recognised")
	ErrMCPRequestInvalid      = errors.New("mcp authorization request is malformed")
	ErrMCPResourceInvalid     = errors.New("mcp resource indicator does not name this server")
	ErrMCPGrantInvalid        = errors.New("mcp connection narrowing must shrink its current reach")
	ErrMCPRedirectInvalid     = errors.New("mcp redirect uri is not registered for this client")
	ErrMCPManageForbidden     = errors.New("mcp connections are managed by signed-in people only")
)

type MCPCapability string

const (
	MCPCapabilityRead  MCPCapability = "read"
	MCPCapabilityWrite MCPCapability = "write"
)

func (c MCPCapability) Valid() bool {
	return c == MCPCapabilityRead || c == MCPCapabilityWrite
}

var mcpReadScopes = APIScopeSet{
	NewAPIScope(ResourceWorkspace, ActionRead),
	NewAPIScope(ResourceMembership, ActionRead),
	NewAPIScope(ResourceTeam, ActionRead),
	NewAPIScope(ResourceTeamMembership, ActionRead),
	NewAPIScope(ResourceIssue, ActionRead),
	NewAPIScope(ResourceCycle, ActionRead),
	NewAPIScope(ResourceLabel, ActionRead),
	NewAPIScope(ResourceProject, ActionRead),
	NewAPIScope(ResourceComment, ActionRead),
}

var mcpWriteScopes = APIScopeSet{
	NewAPIScope(ResourceIssue, ActionManage),
	NewAPIScope(ResourceComment, ActionManage),
}

func MCPScopesFor(capability MCPCapability) APIScopeSet {
	scopes := slices.Clone(mcpReadScopes)
	if capability == MCPCapabilityWrite {
		scopes = append(scopes, mcpWriteScopes...)
	}

	return scopes.Normalized()
}

func CapabilityOf(scopes APIScopeSet) MCPCapability {
	for _, scope := range mcpWriteScopes {
		if slices.Contains(scopes, scope) {
			return MCPCapabilityWrite
		}
	}

	return MCPCapabilityRead
}

func ParseMCPScopes(value string) (MCPCapability, error) {
	capability := MCPCapabilityRead

	for _, field := range strings.Fields(value) {
		parsed := MCPCapability(field)
		if !parsed.Valid() {
			return "", ErrMCPScopeInvalid
		}

		if parsed == MCPCapabilityWrite {
			capability = MCPCapabilityWrite
		}
	}

	return capability, nil
}

type MCPClient struct {
	ID           uuid.UUID
	Name         string
	RedirectURIs []string
	CreatedAt    time.Time
}

func (c MCPClient) PermitsRedirect(uri string) bool {
	if slices.Contains(c.RedirectURIs, uri) {
		return true
	}

	requested, err := url.Parse(uri)
	if err != nil || !loopbackRedirect(requested) {
		return false
	}

	for _, registered := range c.RedirectURIs {
		parsed, err := url.Parse(registered)
		if err != nil || !loopbackRedirect(parsed) {
			continue
		}

		if parsed.Hostname() == requested.Hostname() &&
			parsed.Path == requested.Path &&
			parsed.RawQuery == requested.RawQuery {
			return true
		}
	}

	return false
}

func loopbackRedirect(parsed *url.URL) bool {
	if parsed.Scheme != "http" {
		return false
	}

	switch parsed.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func ValidateMCPClientName(field, name string) FieldError {
	trimmed := strings.TrimSpace(name)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case len(trimmed) > MCPClientNameMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

var deniedRedirectSchemes = []string{"javascript", "data", "vbscript", "blob", "file", "about"}

func ValidMCPRedirectURI(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Fragment != "" {
		return false
	}

	switch parsed.Scheme {
	case "https":
		return parsed.Host != ""
	case "http":
		return loopbackRedirect(parsed)
	default:
		return privateSchemeRedirect(parsed)
	}
}

func privateSchemeRedirect(parsed *url.URL) bool {
	if slices.Contains(deniedRedirectSchemes, strings.ToLower(parsed.Scheme)) {
		return false
	}

	if !privateSchemeName(parsed.Scheme) {
		return false
	}

	if parsed.Opaque != "" {
		return strings.HasPrefix(parsed.Opaque, "/")
	}

	return parsed.Host != "" || strings.HasPrefix(parsed.Path, "/")
}

func privateSchemeName(scheme string) bool {
	for index, char := range scheme {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= '0' && char <= '9', char == '+', char == '-', char == '.':
			if index == 0 {
				return false
			}
		default:
			return false
		}
	}

	return scheme != ""
}

type MCPConnection struct {
	ID         uuid.UUID
	AccountID  uuid.UUID
	ClientID   uuid.UUID
	ClientName string
	Scopes     APIScopeSet
	Grants     APITokenGrants
	RevokedAt  *time.Time
	LastUsedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (c MCPConnection) Revoked() bool {
	return c.RevokedAt != nil
}

func (c MCPConnection) Usable() bool {
	return !c.Revoked()
}

func (c MCPConnection) NeedsUsageStamp(now time.Time, interval time.Duration) bool {
	return c.LastUsedAt == nil || now.Sub(*c.LastUsedAt) >= interval
}

func (c MCPConnection) Capability() MCPCapability {
	return CapabilityOf(c.Scopes)
}

func (c MCPConnection) FollowsMembership() bool {
	return c.Grants == nil
}

type MCPTokenKind string

const (
	MCPTokenKindAccess  MCPTokenKind = "access"
	MCPTokenKindRefresh MCPTokenKind = "refresh"
)

func (k MCPTokenKind) Valid() bool {
	return k == MCPTokenKindAccess || k == MCPTokenKindRefresh
}

type MCPToken struct {
	ID           uuid.UUID
	ConnectionID uuid.UUID
	Kind         MCPTokenKind
	TokenHash    []byte
	ExpiresAt    time.Time
	ConsumedAt   *time.Time
	CreatedAt    time.Time
}

func (t MCPToken) ExpiredAt(now time.Time) bool {
	return !now.Before(t.ExpiresAt)
}

func (t MCPToken) Consumed() bool {
	return t.ConsumedAt != nil
}

func NewMCPTokenValue() (string, []byte, error) {
	raw := make([]byte, MCPTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate mcp token: %w", err)
	}

	token := MCPTokenPrefix + base64.RawURLEncoding.EncodeToString(raw)

	return token, HashMCPToken(token), nil
}

func HashMCPToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))

	return sum[:]
}

func LooksLikeMCPToken(token string) bool {
	return strings.HasPrefix(token, MCPTokenPrefix)
}

func NewMCPAuthCode() (string, error) {
	raw := make([]byte, MCPAuthCodeBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate mcp authorization code: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func VerifyPKCE(challenge, verifier string) bool {
	sum := sha256.Sum256([]byte(verifier))
	derived := base64.RawURLEncoding.EncodeToString(sum[:])

	return subtle.ConstantTimeCompare([]byte(derived), []byte(challenge)) == 1
}

type MCPAuthRequest struct {
	ClientID      uuid.UUID
	ClientName    string
	RedirectURI   string
	Capability    MCPCapability
	State         string
	CodeChallenge string
	Resource      string
	CreatedAt     time.Time
}

type MCPAuthCode struct {
	ClientID      uuid.UUID
	AccountID     uuid.UUID
	RedirectURI   string
	Capability    MCPCapability
	CodeChallenge string
	Grants        APITokenGrants
}

func MCPGrantsFor(chosen []uuid.UUID, reachable []Workspace) (APITokenGrants, error) {
	if len(chosen) == 0 {
		return nil, ErrMCPNoWorkspaceChosen
	}

	grants := make(APITokenGrants, 0, len(chosen))

	for _, workspaceID := range chosen {
		if !slices.ContainsFunc(reachable, func(w Workspace) bool { return w.ID == workspaceID }) {
			return nil, ErrMCPWorkspaceUnreachable
		}

		grants = append(grants, APITokenGrant{WorkspaceID: workspaceID, AllTeams: true})
	}

	return grants, nil
}
