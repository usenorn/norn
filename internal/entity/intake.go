package entity

import (
	"errors"
	"fmt"
	"html"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	IntakeLabelMaxLen     = 24
	IntakeTokenBytes      = 6
	IntakeLocalPartMaxLen = 64
	IntakeSubjectFallback = "(no subject)"
	IntakeLabelFallback   = "team"
)

var (
	ErrIntakeDisabled          = errors.New("team does not take issues by email")
	ErrIntakeAddressUnknown    = errors.New("no team takes mail at this address")
	ErrIntakeAddressTaken      = errors.New("address is already in use")
	ErrIntakeDeliveryDuplicate = errors.New("inbound message has already been received")
	ErrIntakeDeliveryNotFound  = errors.New("inbound message not found")
	ErrIntakeSignatureInvalid  = errors.New("inbound mail delivery did not verify")
	ErrIntakeUnroutable        = errors.New("inbound message reached no team address")
	ErrIntakeDomainUnset       = errors.New("this instance has no domain for incoming mail")
)

type IntakeAddress struct {
	TeamID      uuid.UUID
	WorkspaceID uuid.UUID
	LocalPart   string
	Domain      string
	EnabledBy   uuid.UUID
	CreatedAt   time.Time
	RotatedAt   *time.Time
}

func (a IntakeAddress) Email() string {
	return a.LocalPart + "@" + a.Domain
}

func (a IntakeAddress) Rotated() bool {
	return a.RotatedAt != nil
}

var intakeLabelUnsafe = regexp.MustCompile(`[^a-z0-9]+`)

func IntakeLabel(team Team) string {
	label := intakeLabelUnsafe.ReplaceAllString(strings.ToLower(latinise(team.Name)), "-")
	label = strings.Trim(label, "-")

	if label == "" {
		label = intakeLabelUnsafe.ReplaceAllString(strings.ToLower(team.Key), "-")
		label = strings.Trim(label, "-")
	}

	if label == "" {
		return IntakeLabelFallback
	}

	if len(label) > IntakeLabelMaxLen {
		label = strings.Trim(label[:IntakeLabelMaxLen], "-")
	}

	return label
}

func IntakeLocalPart(label, token string) string {
	return label + "-" + token
}

func IntakeRecipient(recipients []string, domain string) (string, error) {
	suffix := "@" + strings.ToLower(domain)

	for _, recipient := range recipients {
		address := strings.ToLower(strings.TrimSpace(EmailAddressOf(recipient)))
		if local, found := strings.CutSuffix(address, suffix); found && local != "" {
			return local, nil
		}
	}

	return "", ErrIntakeUnroutable
}

var intakeAngled = regexp.MustCompile(`<([^<>]+)>`)

func EmailAddressOf(mailbox string) string {
	if match := intakeAngled.FindStringSubmatch(mailbox); match != nil {
		return strings.TrimSpace(match[1])
	}

	return strings.TrimSpace(mailbox)
}

type IntakeDeliveryOutcome string

const (
	IntakeDeliveryPending IntakeDeliveryOutcome = ""
	IntakeDeliveryFiled   IntakeDeliveryOutcome = "filed"
	IntakeDeliveryIgnored IntakeDeliveryOutcome = "ignored"
	IntakeDeliveryFailed  IntakeDeliveryOutcome = "failed"
)

func IntakeDeliveryOutcomes() []IntakeDeliveryOutcome {
	return []IntakeDeliveryOutcome{IntakeDeliveryFiled, IntakeDeliveryIgnored, IntakeDeliveryFailed}
}

func (o IntakeDeliveryOutcome) Valid() bool {
	return o == IntakeDeliveryPending || slices.Contains(IntakeDeliveryOutcomes(), o)
}

func (o IntakeDeliveryOutcome) Settled() bool {
	return o != IntakeDeliveryPending
}

type IntakeDelivery struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	TeamID      uuid.UUID
	ExternalID  string
	Recipient   string
	Sender      string
	Subject     string
	ReceivedAt  time.Time
	ProcessedAt *time.Time
	Outcome     IntakeDeliveryOutcome
	IssueID     uuid.UUID
	Failure     string
}

type InboundNotice struct {
	DeliveryID string
	MessageID  string
}

func (n InboundNotice) Carries() bool {
	return n.MessageID != ""
}

type InboundMessage struct {
	ExternalID  string
	Sender      string
	Recipients  []string
	Subject     string
	Text        string
	HTML        string
	ReceivedAt  time.Time
	Attachments []string
}

func IntakeTitle(subject string) string {
	title := strings.TrimSpace(collapseIntakeSpace(subject))
	if title == "" {
		return IntakeSubjectFallback
	}

	if utf8.RuneCountInString(title) > IssueTitleMaxLen {
		return string([]rune(title)[:IssueTitleMaxLen])
	}

	return title
}

func IntakeDescription(message InboundMessage) string {
	body := strings.TrimSpace(message.Text)
	if body == "" {
		body = strings.TrimSpace(TextFromHTML(message.HTML))
	}

	lead := fmt.Sprintf("From: %s", strings.TrimSpace(message.Sender))

	if len(message.Attachments) > 0 {
		lead += "\nAttached: " + strings.Join(message.Attachments, ", ")
	}

	if body == "" {
		return lead
	}

	return lead + "\n\n" + body
}

var (
	intakeStripped   = regexp.MustCompile(`(?is)<(script|style)\b[^>]*>.*?</(script|style)>`)
	intakeBreaks     = regexp.MustCompile(`(?i)<(br|/p|/div|/tr|/li|/h[1-6])\s*/?>`)
	intakeTags       = regexp.MustCompile(`(?s)<[^>]*>`)
	intakeBlankLines = regexp.MustCompile(`\n{3,}`)
	intakeSpaces     = regexp.MustCompile(`[ \t\x{00a0}]+`)
)

func TextFromHTML(markup string) string {
	if markup == "" {
		return ""
	}

	text := intakeStripped.ReplaceAllString(markup, "")
	text = intakeBreaks.ReplaceAllString(text, "\n")
	text = intakeTags.ReplaceAllString(text, "")
	text = html.UnescapeString(text)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = intakeSpaces.ReplaceAllString(text, " ")

	lines := strings.Split(text, "\n")
	for index, line := range lines {
		lines[index] = strings.TrimSpace(line)
	}

	return strings.TrimSpace(intakeBlankLines.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

func collapseIntakeSpace(value string) string {
	return intakeSpaces.ReplaceAllString(strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " "), " ")
}
