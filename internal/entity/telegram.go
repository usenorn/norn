package entity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	TelegramLinkCodeBytes      = 24
	TelegramWebhookSecretBytes = 32
	TelegramTokenHintLen       = 4
	TelegramMessageMaxLen      = 4096
	TelegramCallbackAnswer     = "a:"
	TelegramCallbackApprove    = "approve"
	TelegramCallbackChanges    = "changes"
	TelegramCallbackRetry      = "retry"
	TelegramCallbackAbandon    = "abandon"
	TelegramConnectionName     = "Telegram"
	TelegramTypingAction       = "typing"
	TelegramUpdateSweepBatch   = 500
	TelegramChatTurnMaxLen     = AgentTurnMaxLen
)

var TelegramAllowedUpdates = []string{"message", "callback_query", "my_chat_member"}

var telegramBotToken = regexp.MustCompile(`^[0-9]{1,20}:[A-Za-z0-9_-]{30,64}$`)

var (
	ErrTelegramBotNotFound          = errors.New("this agent has no telegram bot")
	ErrTelegramTokenRejected        = errors.New("telegram rejected this bot token")
	ErrTelegramBotTaken             = errors.New("this telegram bot already serves another agent")
	ErrTelegramOriginInsecure       = errors.New("telegram only delivers updates to an https origin")
	ErrTelegramUnreachable          = errors.New("telegram could not be reached")
	ErrTelegramChatUnavailable      = errors.New("telegram will not deliver to this chat")
	ErrTelegramEncryptionKeyMissing = errors.New("this instance has no encryption key, so a bot token cannot be stored")
	ErrTelegramSecretInvalid        = errors.New("telegram update secret is invalid")
	ErrTelegramUpdateDuplicate      = errors.New("telegram update already received")
	ErrTelegramUpdateNotFound       = errors.New("telegram update not found")
	ErrTelegramLinkCodeInvalid      = errors.New("telegram link code is invalid or expired")
	ErrTelegramAccountNotLinked     = errors.New("telegram account is not linked")
	ErrTelegramGroupNotFound        = errors.New("telegram group not found")
	ErrTelegramDecisionNotFound     = errors.New("telegram decision message not found")
)

type TelegramLinkPurpose string

const (
	TelegramLinkPrivate TelegramLinkPurpose = "private"
	TelegramLinkGroup   TelegramLinkPurpose = "group"
)

func (p TelegramLinkPurpose) Valid() bool {
	return p == TelegramLinkPrivate || p == TelegramLinkGroup
}

type TelegramChatType string

const (
	TelegramChatPrivate    TelegramChatType = "private"
	TelegramChatGroup      TelegramChatType = "group"
	TelegramChatSupergroup TelegramChatType = "supergroup"
	TelegramChatChannel    TelegramChatType = "channel"
)

func (t TelegramChatType) Grouped() bool {
	return t == TelegramChatGroup || t == TelegramChatSupergroup
}

type TelegramUpdateOutcome string

const (
	TelegramUpdateApplied TelegramUpdateOutcome = "applied"
	TelegramUpdateIgnored TelegramUpdateOutcome = "ignored"
	TelegramUpdateFailed  TelegramUpdateOutcome = "failed"
)

type TelegramBot struct {
	ID              uuid.UUID
	WorkspaceID     uuid.UUID
	AgentID         uuid.UUID
	BotUserID       int64
	Username        string
	Name            string
	TokenHint       string
	ConnectedBy     uuid.UUID
	ConnectedByName string
	ConnectedAt     time.Time
	UpdatedAt       time.Time
}

func (b TelegramBot) DeepLink(purpose TelegramLinkPurpose, code string) string {
	parameter := "start"
	if purpose == TelegramLinkGroup {
		parameter = "startgroup"
	}

	return fmt.Sprintf("https://t.me/%s?%s=%s", b.Username, parameter, code)
}

type TelegramIdentity struct {
	BotUserID int64
	Username  string
	Name      string
}

type TelegramAccount struct {
	BotID          uuid.UUID
	AccountID      uuid.UUID
	AccountName    string
	TelegramUserID int64
	ChatID         int64
	Username       string
	LinkedAt       time.Time
}

func (a TelegramAccount) Actor() Actor {
	bot := a.BotID

	return Actor{
		Kind:           ActorKindToken,
		AccountID:      a.AccountID,
		OwnerAccountID: a.AccountID,
		ConnectionID:   &bot,
		ConnectionName: TelegramConnectionName,
	}
}

type TelegramGroup struct {
	ID          uuid.UUID
	BotID       uuid.UUID
	ChatID      int64
	Title       string
	BoundBy     uuid.UUID
	BoundByName string
	BoundAt     time.Time
}

type TelegramLinkCode struct {
	BotID     uuid.UUID
	AccountID uuid.UUID
	Purpose   TelegramLinkPurpose
	ExpiresAt time.Time
}

type TelegramUpdate struct {
	ID          uuid.UUID
	BotID       uuid.UUID
	UpdateID    int64
	Payload     []byte
	Outcome     TelegramUpdateOutcome
	ReceivedAt  time.Time
	ProcessedAt *time.Time
}

func (u TelegramUpdate) Processed() bool {
	return u.ProcessedAt != nil
}

type TelegramSender struct {
	ID        int64
	IsBot     bool
	Anonymous bool
	Username  string
}

func (s TelegramSender) Person() bool {
	return s.ID != 0 && !s.IsBot && !s.Anonymous
}

type TelegramMessage struct {
	ChatID          int64
	ChatType        TelegramChatType
	ChatTitle       string
	MessageID       int64
	Sender          TelegramSender
	Text            string
	Mentions        []string
	ReplyToID       int64
	ReplyToSenderID int64
	MigratedTo      int64
}

func (m TelegramMessage) Start(botUsername string) (string, bool) {
	fields := strings.Fields(m.Text)
	if len(fields) != 2 {
		return "", false
	}

	command, addressee, addressed := strings.Cut(fields[0], "@")
	if command != "/start" {
		return "", false
	}

	if addressed && !strings.EqualFold(addressee, botUsername) {
		return "", false
	}

	return fields[1], true
}

func (m TelegramMessage) AddressedTo(bot TelegramBot) bool {
	if m.ChatType == TelegramChatPrivate {
		return true
	}

	if m.ReplyToSenderID == bot.BotUserID {
		return true
	}

	for _, mention := range m.Mentions {
		if strings.EqualFold(mention, bot.Username) {
			return true
		}
	}

	return false
}

func (m TelegramMessage) Prompt(botUsername string) string {
	if botUsername == "" {
		return strings.TrimSpace(m.Text)
	}

	mention := regexp.MustCompile(`(?i)@` + regexp.QuoteMeta(botUsername) + `\b`)

	return strings.TrimSpace(mention.ReplaceAllString(m.Text, ""))
}

type TelegramCallback struct {
	ID        string
	Sender    TelegramSender
	ChatID    int64
	MessageID int64
	Data      string
}

func (c TelegramCallback) Option() (int, bool) {
	raw, ok := strings.CutPrefix(c.Data, TelegramCallbackAnswer)
	if !ok {
		return 0, false
	}

	index, err := strconv.Atoi(raw)
	if err != nil || index < 0 {
		return 0, false
	}

	return index, true
}

func TelegramOptionData(index int) string {
	return TelegramCallbackAnswer + strconv.Itoa(index)
}

func TelegramOptionButtons(options []string) []TelegramButton {
	buttons := make([]TelegramButton, 0, len(options))

	for index, option := range options {
		buttons = append(buttons, TelegramButton{Label: option, Data: TelegramOptionData(index)})
	}

	return buttons
}

type TelegramMembership struct {
	ChatID   int64
	ChatType TelegramChatType
	Removed  bool
}

type TelegramIncoming struct {
	UpdateID   int64
	Message    *TelegramMessage
	Callback   *TelegramCallback
	Membership *TelegramMembership
}

type TelegramButton struct {
	Label string
	Data  string
}

type TelegramOutgoing struct {
	ChatID  int64
	Text    string
	ReplyTo int64
	Buttons []TelegramButton
}

type TelegramDecisionKind string

const (
	TelegramDecisionQuestion    TelegramDecisionKind = "question"
	TelegramDecisionPlan        TelegramDecisionKind = "plan"
	TelegramDecisionReview      TelegramDecisionKind = "review"
	TelegramDecisionPublication TelegramDecisionKind = "publication"
)

func (k TelegramDecisionKind) Valid() bool {
	switch k {
	case TelegramDecisionQuestion, TelegramDecisionPlan, TelegramDecisionReview, TelegramDecisionPublication:
		return true
	default:
		return false
	}
}

type TelegramDecision struct {
	WorkspaceID uuid.UUID
	Kind        TelegramDecisionKind
	QuestionID  uuid.UUID
	ExecutionID string
	Round       string
}

func TelegramQuestionDecision(question IssueQuestion) TelegramDecision {
	return TelegramDecision{
		WorkspaceID: question.WorkspaceID,
		Kind:        TelegramDecisionQuestion,
		QuestionID:  question.ID,
	}
}

func TelegramPlanDecision(execution Execution, revision int) TelegramDecision {
	return TelegramDecision{
		WorkspaceID: execution.WorkspaceID,
		Kind:        TelegramDecisionPlan,
		ExecutionID: execution.ID,
		Round:       strconv.Itoa(revision),
	}
}

func TelegramReviewDecision(execution Execution, heads ReviewHeads) TelegramDecision {
	return TelegramDecision{
		WorkspaceID: execution.WorkspaceID,
		Kind:        TelegramDecisionReview,
		ExecutionID: execution.ID,
		Round:       heads.Digest(),
	}
}

func TelegramPublicationDecision(execution Execution, revision, attempt int) TelegramDecision {
	return TelegramDecision{
		WorkspaceID: execution.WorkspaceID,
		Kind:        TelegramDecisionPublication,
		ExecutionID: execution.ID,
		Round:       strconv.Itoa(revision) + "." + strconv.Itoa(attempt),
	}
}

func TelegramDecisionLeft(execution Execution, from ExecutionState, at time.Time) (TelegramDecision, bool) {
	kind := TelegramDecisionPlan

	switch from {
	case ExecutionAwaitingPlan:
	case ExecutionAwaitingReview:
		kind = TelegramDecisionReview
	case ExecutionApproved:
		kind = TelegramDecisionPublication
	default:
		return TelegramDecision{}, false
	}

	return TelegramDecision{
		WorkspaceID: execution.WorkspaceID,
		Kind:        kind,
		ExecutionID: execution.ID,
		Round:       strconv.FormatInt(at.UnixNano(), 10),
	}, true
}

func (d TelegramDecision) Subject() string {
	if d.Kind == TelegramDecisionQuestion {
		return d.QuestionID.String()
	}

	return d.ExecutionID
}

func (d TelegramDecision) Complete() bool {
	if d.WorkspaceID == uuid.Nil || !d.Kind.Valid() {
		return false
	}

	if d.Kind == TelegramDecisionQuestion {
		return d.QuestionID != uuid.Nil
	}

	return d.ExecutionID != ""
}

type TelegramDecisionMessage struct {
	BotID        uuid.UUID
	ChatID       int64
	MessageID    int64
	Kind         TelegramDecisionKind
	QuestionID   uuid.UUID
	ExecutionID  string
	PlanRevision int
	ReviewHeads  ReviewHeads
	Round        string
	Settled      bool
}

func TelegramTokenHint(token string) string {
	if len(token) <= TelegramTokenHintLen {
		return ""
	}

	return token[len(token)-TelegramTokenHintLen:]
}

func ValidateTelegramBotToken(field, token string) FieldError {
	trimmed := strings.TrimSpace(token)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case !telegramBotToken.MatchString(trimmed):
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	default:
		return FieldError{}
	}
}

func ValidateTelegramLinkPurpose(field string, purpose TelegramLinkPurpose) FieldError {
	if !purpose.Valid() {
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	}

	return FieldError{}
}

func NewTelegramLinkCode() (string, []byte, error) {
	return telegramSecret(TelegramLinkCodeBytes)
}

func NewTelegramWebhookSecret() (string, []byte, error) {
	return telegramSecret(TelegramWebhookSecretBytes)
}

func HashTelegramSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))

	return sum[:]
}

func telegramSecret(size int) (string, []byte, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate telegram secret: %w", err)
	}

	secret := base64.RawURLEncoding.EncodeToString(raw)

	return secret, HashTelegramSecret(secret), nil
}
