package entity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	WebhookPageDefaultSize  = 50
	WebhookPageMaxSize      = 200
	WebhookNameMaxLen       = 100
	WebhookURLMaxLen        = 2048
	WebhookMaxEvents        = 32
	WebhookSecretPrefix     = "nrnwhs_"
	WebhookSecretBytes      = 32
	WebhookSecretHintLen    = 4
	WebhookMaxAttempts      = 6
	WebhookFailureLimit     = 20
	WebhookResponseExcerpt  = 512
	WebhookSignatureVersion = "v1"
	WebhookTimestampSkew    = 5 * time.Minute
)

var (
	ErrWebhookNotFound              = errors.New("webhook not found")
	ErrWebhookDeliveryNotFound      = errors.New("webhook delivery not found")
	ErrWebhookNotPermitted          = errors.New("webhooks are registered by workspace administrators")
	ErrWebhookCursorInvalid         = errors.New("webhook delivery cursor is invalid")
	ErrWebhookDestinationRefused    = errors.New("that destination is not reachable from this instance")
	ErrWebhookEncryptionKeyMissing  = errors.New("this instance has no encryption key, so a webhook cannot be signed")
	ErrWebhookDeliveryNotReplayable = errors.New("a delivery still in flight cannot be replayed")
)

type WebhookEvent string

const (
	WebhookIssueCreated       WebhookEvent = "issue.created"
	WebhookIssueUpdated       WebhookEvent = "issue.updated"
	WebhookIssueStatusChanged WebhookEvent = "issue.status_changed"
	WebhookIssueDeleted       WebhookEvent = "issue.deleted"
	WebhookIssueTriaged       WebhookEvent = "issue.triaged"
	WebhookIssueMerged        WebhookEvent = "issue.merged"
	WebhookIssueDelegated     WebhookEvent = "issue.delegated"

	WebhookCommentPosted  WebhookEvent = "comment.posted"
	WebhookCommentEdited  WebhookEvent = "comment.edited"
	WebhookCommentDeleted WebhookEvent = "comment.deleted"

	WebhookProjectCreated       WebhookEvent = "project.created"
	WebhookProjectUpdated       WebhookEvent = "project.updated"
	WebhookProjectDeleted       WebhookEvent = "project.deleted"
	WebhookProjectMembersEdited WebhookEvent = "project.members_changed"
	WebhookProjectStatusPosted  WebhookEvent = "project.status_posted"

	WebhookCycleStarted        WebhookEvent = "cycle.started"
	WebhookCycleClosed         WebhookEvent = "cycle.closed"
	WebhookCycleCadenceChanged WebhookEvent = "cycle.cadence_changed"

	WebhookMembershipAdded   WebhookEvent = "membership.added"
	WebhookMembershipChanged WebhookEvent = "membership.changed"
	WebhookMembershipRemoved WebhookEvent = "membership.removed"

	WebhookTeamMembershipAdded   WebhookEvent = "team_membership.added"
	WebhookTeamMembershipRemoved WebhookEvent = "team_membership.removed"

	WebhookTest WebhookEvent = "webhook.test"
)

func WebhookEvents() []WebhookEvent {
	return []WebhookEvent{
		WebhookIssueCreated, WebhookIssueUpdated, WebhookIssueStatusChanged,
		WebhookIssueDeleted, WebhookIssueTriaged, WebhookIssueMerged,
		WebhookIssueDelegated,
		WebhookCommentPosted, WebhookCommentEdited, WebhookCommentDeleted,
		WebhookProjectCreated, WebhookProjectUpdated, WebhookProjectDeleted,
		WebhookProjectMembersEdited, WebhookProjectStatusPosted,
		WebhookCycleStarted, WebhookCycleClosed, WebhookCycleCadenceChanged,
		WebhookMembershipAdded, WebhookMembershipChanged, WebhookMembershipRemoved,
		WebhookTeamMembershipAdded, WebhookTeamMembershipRemoved,
	}
}

func (e WebhookEvent) Valid() bool {
	return slices.Contains(WebhookEvents(), e)
}

type WebhookScopeRule string

const (
	WebhookScopeTeam      WebhookScopeRule = "team"
	WebhookScopeWorkspace WebhookScopeRule = "workspace"
	WebhookScopeAdmin     WebhookScopeRule = "admin"
	WebhookScopeAdminTeam WebhookScopeRule = "admin_team"
)

func WebhookEventScope(event WebhookEvent) WebhookScopeRule {
	subject, _, _ := strings.Cut(string(event), ".")

	switch subject {
	case "project":
		return WebhookScopeWorkspace
	case "membership":
		return WebhookScopeAdmin
	case "team_membership":
		return WebhookScopeAdminTeam
	default:
		return WebhookScopeTeam
	}
}

func WebhookEventResource(event WebhookEvent) Resource {
	subject, _, _ := strings.Cut(string(event), ".")

	switch subject {
	case "comment":
		return ResourceComment
	case "project":
		return ResourceProject
	case "cycle":
		return ResourceCycle
	case "membership":
		return ResourceMembership
	case "team_membership":
		return ResourceTeamMembership
	default:
		return ResourceIssue
	}
}

type WebhookDisableReason string

const (
	WebhookDisabledByOwner        WebhookDisableReason = "by_owner"
	WebhookDisabledSustainedFault WebhookDisableReason = "sustained_failure"
)

type WebhookDeliveryState string

const (
	WebhookDeliveryPending   WebhookDeliveryState = "pending"
	WebhookDeliverySucceeded WebhookDeliveryState = "succeeded"
	WebhookDeliveryFailed    WebhookDeliveryState = "failed"
)

func (s WebhookDeliveryState) Valid() bool {
	switch s {
	case WebhookDeliveryPending, WebhookDeliverySucceeded, WebhookDeliveryFailed:
		return true
	default:
		return false
	}
}

func (s WebhookDeliveryState) Settled() bool {
	return s == WebhookDeliverySucceeded || s == WebhookDeliveryFailed
}

type WebhookAttemptOutcome string

const (
	WebhookAttemptSucceeded WebhookAttemptOutcome = "succeeded"
	WebhookAttemptRejected  WebhookAttemptOutcome = "rejected"
	WebhookAttemptRefused   WebhookAttemptOutcome = "refused"
	WebhookAttemptTimedOut  WebhookAttemptOutcome = "timed_out"
	WebhookAttemptFailed    WebhookAttemptOutcome = "failed"
)

func (o WebhookAttemptOutcome) Valid() bool {
	switch o {
	case WebhookAttemptSucceeded, WebhookAttemptRejected,
		WebhookAttemptRefused, WebhookAttemptTimedOut, WebhookAttemptFailed:
		return true
	default:
		return false
	}
}

type Webhook struct {
	ID              uuid.UUID
	WorkspaceID     uuid.UUID
	OwnerAccountID  uuid.UUID
	Name            string
	URL             string
	Events          []WebhookEvent
	Secret          string
	PreviousSecret  string
	SecretHint      string
	SecretExpiresAt *time.Time
	Enabled         bool
	DisabledAt      *time.Time
	DisabledReason  WebhookDisableReason
	FailureStreak   int
	LastDeliveredAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (w Webhook) Subscribes(event WebhookEvent) bool {
	return slices.Contains(w.Events, event)
}

func (w Webhook) SigningSecrets(now time.Time) []string {
	secrets := make([]string, 0, 2)

	if w.Secret != "" {
		secrets = append(secrets, w.Secret)
	}

	if w.PreviousSecret != "" && w.SecretExpiresAt != nil && now.Before(*w.SecretExpiresAt) {
		secrets = append(secrets, w.PreviousSecret)
	}

	return secrets
}

type WebhookRequest struct {
	URL     string
	Headers map[string]string
	Body    []byte
}

type WebhookResponse struct {
	Outcome         WebhookAttemptOutcome
	StatusCode      int
	Excerpt         string
	Error           string
	ResolvedAddress string
	StartedAt       time.Time
	FinishedAt      time.Time
}

type WebhookOutboxEntry struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Event       WebhookEvent
	SubjectKind string
	SubjectID   uuid.UUID
	TeamID      uuid.UUID
	ActorID     uuid.UUID
	ActorKind   ActorKind
	ActorName   string
	Body        []byte
	OccurredAt  time.Time
}

type WebhookDelivery struct {
	ID          uuid.UUID
	WebhookID   uuid.UUID
	WorkspaceID uuid.UUID
	OutboxID    uuid.UUID
	ReplayOf    uuid.UUID
	Event       WebhookEvent
	SubjectKind string
	SubjectID   uuid.UUID
	TeamID      uuid.UUID
	Body        []byte
	State       WebhookDeliveryState
	Attempt     int
	NextAttempt time.Time
	SettledAt   *time.Time
	CreatedAt   time.Time
	Attempts    []WebhookAttempt
}

func (d WebhookDelivery) Replayable() bool {
	return d.State.Settled()
}

type WebhookAttempt struct {
	ID              uuid.UUID
	DeliveryID      uuid.UUID
	Attempt         int
	RequestURL      string
	ResolvedAddress string
	Outcome         WebhookAttemptOutcome
	StatusCode      int
	ResponseExcerpt string
	Error           string
	StartedAt       time.Time
	FinishedAt      time.Time
}

func (a WebhookAttempt) Elapsed() time.Duration {
	return a.FinishedAt.Sub(a.StartedAt)
}

type WebhookDeliveryFilter struct {
	WebhookID uuid.UUID
	Events    []WebhookEvent
	States    []WebhookDeliveryState
	Cursor    *WebhookDeliveryCursor
	Limit     int
}

func (f WebhookDeliveryFilter) Normalized() WebhookDeliveryFilter {
	if f.Limit <= 0 {
		f.Limit = WebhookPageDefaultSize
	}

	if f.Limit > WebhookPageMaxSize {
		f.Limit = WebhookPageMaxSize
	}

	return f
}

func (f WebhookDeliveryFilter) Lookahead() WebhookDeliveryFilter {
	f.Limit++

	return f
}

type WebhookDeliveryCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

func (c WebhookDeliveryCursor) Encode() string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte(c.ID.String() + c.CreatedAt.UTC().Format(time.RFC3339Nano)),
	)
}

func DecodeWebhookDeliveryCursor(raw string) (WebhookDeliveryCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) < ActivityCursorIDLen {
		return WebhookDeliveryCursor{}, ErrWebhookCursorInvalid
	}

	id, err := uuid.Parse(string(decoded[:ActivityCursorIDLen]))
	if err != nil {
		return WebhookDeliveryCursor{}, ErrWebhookCursorInvalid
	}

	createdAt, err := time.Parse(time.RFC3339Nano, string(decoded[ActivityCursorIDLen:]))
	if err != nil {
		return WebhookDeliveryCursor{}, ErrWebhookCursorInvalid
	}

	return WebhookDeliveryCursor{CreatedAt: createdAt, ID: id}, nil
}

func (d WebhookDelivery) Cursor() WebhookDeliveryCursor {
	return WebhookDeliveryCursor{CreatedAt: d.CreatedAt, ID: d.ID}
}

func NewWebhookSecret() (string, error) {
	raw := make([]byte, WebhookSecretBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate webhook secret: %w", err)
	}

	return WebhookSecretPrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func WebhookSecretHint(secret string) string {
	if len(secret) <= WebhookSecretHintLen {
		return secret
	}

	return secret[len(secret)-WebhookSecretHintLen:]
}

func SignWebhook(secret string, timestamp time.Time, deliveryID uuid.UUID, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(WebhookSignatureVersion + ":" +
		strconv.FormatInt(timestamp.UTC().Unix(), 10) + ":" +
		deliveryID.String() + ":"))
	mac.Write(body)

	return WebhookSignatureVersion + "=" + hex.EncodeToString(mac.Sum(nil))
}

var webhookRetryLadder = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
	time.Hour,
	6 * time.Hour,
}

func WebhookRetryDelay(attempt int) (time.Duration, bool) {
	if attempt < 1 || attempt > len(webhookRetryLadder) || attempt >= WebhookMaxAttempts {
		return 0, false
	}

	return webhookRetryLadder[attempt-1], true
}

func ValidateWebhookName(field, name string) FieldError {
	trimmed := strings.TrimSpace(name)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case len(trimmed) > WebhookNameMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateWebhookURL(field, raw string) FieldError {
	trimmed := strings.TrimSpace(raw)

	if trimmed == "" {
		return FieldError{Field: field, Code: ValidationCodeRequired}
	}

	if len(trimmed) > WebhookURLMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	}

	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	}

	if parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	}

	return FieldError{}
}

func ValidateWebhookEvents(field string, events []WebhookEvent) FieldError {
	if len(events) == 0 {
		return FieldError{Field: field, Code: ValidationCodeRequired}
	}

	if len(events) > WebhookMaxEvents {
		return FieldError{Field: field, Code: ValidationCodeOutOfRange}
	}

	for _, event := range events {
		if !event.Valid() {
			return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
		}
	}

	return FieldError{}
}

func NormalizeWebhookEvents(events []WebhookEvent) []WebhookEvent {
	normalized := slices.Clone(events)
	slices.Sort(normalized)

	return slices.Compact(normalized)
}

func WebhookEventStrings(events []WebhookEvent) []string {
	values := make([]string, 0, len(events))

	for _, event := range events {
		values = append(values, string(event))
	}

	return values
}

func NewWebhookEvents(values []string) []WebhookEvent {
	events := make([]WebhookEvent, 0, len(values))

	for _, value := range values {
		events = append(events, WebhookEvent(value))
	}

	return events
}
