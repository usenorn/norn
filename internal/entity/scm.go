package entity

import (
	"crypto/rand"
	"encoding/base64"
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
	SCMSecretPrefix     = "nrnscm_"
	SCMSecretBytes      = 32
	SCMTokenHintLen     = 4
	SCMRepositoryMaxLen = 200
	SCMMirrorLabelMax   = 80
	SCMLabelMaxLen      = 80
	SCMBaseURLMaxLen    = 300
	SCMTokenMaxLen      = 500
	SCMPathPrefixMaxLen = 300
)

var (
	ErrSCMConnectionNotFound      = errors.New("source control connection not found")
	ErrSCMConnectionExists        = errors.New("that forge is already connected")
	ErrSCMProviderUnsupported     = errors.New("source control provider is not supported")
	ErrSCMEncryptionKeyMissing    = errors.New("source control credentials cannot be sealed")
	ErrSCMConnectionBroken        = errors.New("source control connection is broken")
	ErrSCMDeliveryDuplicate       = errors.New("delivery has already been received")
	ErrSCMSignatureInvalid        = errors.New("delivery signature did not verify")
	ErrSCMRepositoryNotFound      = errors.New("repository is not connected")
	ErrSCMRepositoryExists        = errors.New("that repository is already connected")
	ErrSCMRouteNotFound           = errors.New("route not found")
	ErrSCMRouteExists             = errors.New("that team already has a route for this path")
	ErrSCMRouteUnreachable        = errors.New("no team owns this path")
	ErrCodeLinkNotFound           = errors.New("linked change not found")
	ErrIssueMirrorNotFound        = errors.New("issue is not mirrored")
	ErrIssueMirrorExists          = errors.New("issue is already mirrored")
	ErrSCMTransitionRuleNotFound  = errors.New("team does not act on this change")
	ErrSCMRepositoryUnrecognised  = errors.New("repository is not owner and name")
	ErrSCMTeamOutsideConnection   = errors.New("issue is outside the connection's reach")
	ErrSCMMirrorLabelNotSupported = errors.New("mirror label is not usable")
)

type SCMProvider string

const (
	SCMProviderGitHub SCMProvider = "github"
	SCMProviderGitLab SCMProvider = "gitlab"
	// SCMProviderGitea covers Forgejo as well. Forgejo is a fork of Gitea and serves the same
	// api under the same paths, so one adapter reaches both and a second would be a copy.
	SCMProviderGitea SCMProvider = "gitea"
)

func SCMProviders() []SCMProvider {
	return []SCMProvider{SCMProviderGitHub, SCMProviderGitLab, SCMProviderGitea}
}

func (p SCMProvider) Valid() bool {
	return slices.Contains(SCMProviders(), p)
}

func (p SCMProvider) Label() string {
	switch p {
	case SCMProviderGitHub:
		return "GitHub"
	case SCMProviderGitLab:
		return "GitLab"
	case SCMProviderGitea:
		return "Gitea or Forgejo"
	default:
		return string(p)
	}
}

type SCMConnectionStatus string

const (
	SCMConnectionConnected SCMConnectionStatus = "connected"
	SCMConnectionBroken    SCMConnectionStatus = "broken"
)

type SCMBrokenReason string

const (
	SCMBrokenNone                  SCMBrokenReason = ""
	SCMBrokenCredentialsRejected   SCMBrokenReason = "credentials_rejected"
	SCMBrokenRepositoryUnreachable SCMBrokenReason = "repository_unreachable"
	SCMBrokenHookMissing           SCMBrokenReason = "hook_missing"
)

func SCMBrokenReasons() []SCMBrokenReason {
	return []SCMBrokenReason{
		SCMBrokenCredentialsRejected,
		SCMBrokenRepositoryUnreachable,
		SCMBrokenHookMissing,
	}
}

func (r SCMBrokenReason) Valid() bool {
	return r == SCMBrokenNone || slices.Contains(SCMBrokenReasons(), r)
}

// SelfHosted reports a connection to somebody's own instance rather than the public service.
// Gitea has no hosted service this integration knows, so it is always somebody's own.
func (c SCMConnection) SelfHosted() bool {
	return c.Provider == SCMProviderGitea || strings.TrimSpace(c.BaseURL) != ""
}

// SCMConnection is one credential against one forge address. A token already reaches every
// repository its owner can see, so repositories hang off the connection rather than each
// carrying its own copy of the secret.
type SCMConnection struct {
	ID                   uuid.UUID
	WorkspaceID          uuid.UUID
	Provider             SCMProvider
	BaseURL              string
	Label                string
	TokenHint            string
	TokenSet             bool
	IdentityLogin        string
	IntegrationAccountID uuid.UUID
	IntegrationName      string
	OwnerAccountID       uuid.UUID
	OwnerActorKind       ActorKind
	OwnerAuthMethod      SessionAuthMethod
	Status               SCMConnectionStatus
	BrokenReason         SCMBrokenReason
	BrokenDetail         string
	BrokenAt             *time.Time
	VerifiedAt           *time.Time
	Trust                SCMTrust
	Capabilities         SCMCapabilitySet
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (c SCMConnection) Broken() bool {
	return c.Status == SCMConnectionBroken
}

func (c SCMConnection) Verified() bool {
	return c.VerifiedAt != nil
}

// Wrote reports content on the forge that this connection's own token authored. A digest
// tells us a value came back unchanged; this tells us who sent it, which is the only guard
// that still holds when a delivery arrives before we have recorded what we just pushed.
func (c SCMConnection) Wrote(login string) bool {
	return c.IdentityLogin != "" && strings.EqualFold(c.IdentityLogin, login)
}

func (c SCMConnection) Actor() Actor {
	kind := c.OwnerActorKind
	if kind == "" {
		kind = ActorKindToken
	}

	id := c.ID

	return Actor{
		Kind:           kind,
		AccountID:      c.IntegrationAccountID,
		OwnerAccountID: c.OwnerAccountID,
		ConnectionID:   &id,
		ConnectionName: c.IntegrationName,
		AuthMethod:     c.OwnerAuthMethod,
	}
}

func (c SCMConnection) DisplayName() string {
	switch {
	case c.Label != "":
		return c.Label
	case c.IdentityLogin != "":
		return c.IdentityLogin
	case c.BaseURL != "":
		return c.BaseURL
	default:
		return c.Provider.Label()
	}
}

type SCMTarget struct {
	Provider   SCMProvider
	BaseURL    string
	Repository string
	Token      string
	Trust      SCMTrust
}

func (c SCMConnection) Target(repository, token string) SCMTarget {
	return SCMTarget{
		Provider:   c.Provider,
		BaseURL:    c.BaseURL,
		Repository: repository,
		Token:      token,
		Trust:      c.Trust,
	}
}

// SCMRemoteRepository is what the forge says about a repository, as read at connect time.
// It is never stored whole; SCMRepository is the row Norn keeps.
type SCMRemoteRepository struct {
	ExternalID    string
	FullName      string
	URL           string
	DefaultBranch string
	Private       bool
	CanAdmin      bool
}

// SCMDeliveryOutcome answers the question a delivery log exists for: not "did this
// arrive" but "why did no link appear". Applied and ignored are both successes and read
// identically without this — one made a link, the other decided there was nothing to make.
type SCMDeliveryOutcome string

const (
	SCMDeliveryPending SCMDeliveryOutcome = ""
	SCMDeliveryApplied SCMDeliveryOutcome = "applied"
	SCMDeliveryIgnored SCMDeliveryOutcome = "ignored"
	SCMDeliveryFailed  SCMDeliveryOutcome = "failed"
)

func SCMDeliveryOutcomes() []SCMDeliveryOutcome {
	return []SCMDeliveryOutcome{SCMDeliveryApplied, SCMDeliveryIgnored, SCMDeliveryFailed}
}

func (o SCMDeliveryOutcome) Valid() bool {
	return o == SCMDeliveryPending || slices.Contains(SCMDeliveryOutcomes(), o)
}

func (o SCMDeliveryOutcome) Settled() bool {
	return o != SCMDeliveryPending
}

type SCMDelivery struct {
	ID           uuid.UUID
	RepositoryID uuid.UUID
	WorkspaceID  uuid.UUID
	ExternalID   string
	Event        string
	Payload      []byte
	Attempt      int
	RetryAfter   *time.Time
	Outcome      SCMDeliveryOutcome
	Detail       string
	Failure      string
	ReceivedAt   time.Time
	ProcessedAt  *time.Time
}

func NewSCMWebhookSecret() (string, error) {
	buffer := make([]byte, SCMSecretBytes)

	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}

	return SCMSecretPrefix + base64.RawURLEncoding.EncodeToString(buffer), nil
}

func SCMTokenHint(token string) string {
	trimmed := strings.TrimSpace(token)

	if utf8.RuneCountInString(trimmed) <= SCMTokenHintLen {
		return ""
	}

	runes := []rune(trimmed)

	return string(runes[len(runes)-SCMTokenHintLen:])
}

func IntegrationAccountName(provider SCMProvider, label string) string {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" {
		return provider.Label()
	}

	return provider.Label() + " · " + trimmed
}

var scmRepositoryPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)+$`)

func ValidateSCMRepository(field, repository string) FieldError {
	trimmed := strings.Trim(strings.TrimSpace(repository), "/")

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case len(trimmed) > SCMRepositoryMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	case !scmRepositoryPattern.MatchString(trimmed):
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	default:
		return FieldError{}
	}
}

func NormalizeSCMRepository(repository string) string {
	return strings.Trim(strings.TrimSpace(repository), "/")
}

func ValidateSCMBaseURL(field, raw string) FieldError {
	trimmed := strings.TrimSpace(raw)

	if trimmed == "" {
		return FieldError{}
	}

	if len(trimmed) > SCMBaseURLMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	}

	return FieldError{}
}

func ValidateSCMToken(field, token string) FieldError {
	trimmed := strings.TrimSpace(token)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case len(trimmed) > SCMTokenMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateSCMLabel(field, label string) FieldError {
	trimmed := strings.TrimSpace(label)

	if utf8.RuneCountInString(trimmed) > SCMLabelMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}

func ValidateSCMMirrorLabel(field, label string) FieldError {
	trimmed := strings.TrimSpace(label)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case utf8.RuneCountInString(trimmed) > SCMMirrorLabelMax:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}
