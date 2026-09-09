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
	IntakeContentScheme   = "cid:"
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
	ErrIntakeMessageTooLarge   = errors.New("message is larger than this instance reads")
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

type InboundAttachment struct {
	FileName    string
	ContentType string
	ContentID   string
	Content     []byte
}

func (a InboundAttachment) Embedded() bool {
	return a.ContentID != ""
}

type InboundMessage struct {
	ExternalID  string
	Sender      string
	Recipients  []string
	Subject     string
	Text        string
	HTML        string
	ReceivedAt  time.Time
	Attachments []InboundAttachment
}

func (m InboundMessage) Embeds() bool {
	for _, attachment := range m.Attachments {
		if attachment.Embedded() {
			return true
		}
	}

	return false
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

// IntakeDescription writes the message as markdown. A picture the sender put in the body is
// left as a reference to the part of the message it came from, because the file has nowhere to
// live until the issue exists; IntakeEmbed swaps those references for the stored files.
func IntakeDescription(message InboundMessage) string {
	body := strings.TrimSpace(message.Text)

	// The markup carries where the pictures sit, and the plain text alternative does not, so a
	// message with pictures is read from the markup even when both are offered.
	if body == "" || (message.Embeds() && message.HTML != "") {
		body = strings.TrimSpace(TextFromHTML(message.HTML))
	}

	lead := fmt.Sprintf("From: %s", strings.TrimSpace(message.Sender))

	if body == "" {
		return lead
	}

	return lead + "\n\n" + body
}

// IntakeAttachmentFallback names a file the sender's client did not name. A picture written
// into a message often arrives nameless, and a file with no name cannot be stored or listed.
func IntakeAttachmentFallback(contentType string) string {
	extension := ""

	if kind, _, found := strings.Cut(contentType, "/"); found && kind == "image" {
		_, extension, _ = strings.Cut(contentType, "/")
		extension = "." + strings.Split(extension, "+")[0]
	}

	return "attachment" + extension
}

func IntakeContentReference(contentID string) string {
	return IntakeContentScheme + strings.Trim(strings.TrimSpace(contentID), "<>")
}

// IntakeEmbed points every reference at the file it now names, and says plainly which files
// could not be kept rather than leaving somebody to notice the absence.
func IntakeEmbed(description string, stored map[string]string, refused []string) string {
	embedded := description

	for reference, path := range stored {
		embedded = strings.ReplaceAll(embedded, "("+reference+")", "("+path+")")
	}

	// A reference with no file behind it renders as a broken image, so what is left over is
	// removed rather than shown.
	embedded = intakeLeftoverEmbed.ReplaceAllString(embedded, "")
	embedded = strings.TrimSpace(intakeBlankLines.ReplaceAllString(embedded, "\n\n"))

	if len(refused) > 0 {
		embedded += "\n\nNot kept: " + strings.Join(refused, ", ")
	}

	return embedded
}

var (
	intakeStripped      = regexp.MustCompile(`(?is)<(script|style)\b[^>]*>.*?</(script|style)>`)
	intakeBreaks        = regexp.MustCompile(`(?i)<(br|/p|/div|/tr|/li|/h[1-6])\s*/?>`)
	intakeTags          = regexp.MustCompile(`(?s)<[^>]*>`)
	intakeBlankLines    = regexp.MustCompile(`\n{3,}`)
	intakeSpaces        = regexp.MustCompile(`[ \t\x{00a0}]+`)
	intakeImages        = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	intakeAttribute     = regexp.MustCompile(`(?is)\b(src|alt)\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	intakeLeftoverEmbed = regexp.MustCompile(`!\[[^\]]*\]\(` + IntakeContentScheme + `[^)]*\)`)
)

func TextFromHTML(markup string) string {
	if markup == "" {
		return ""
	}

	text := intakeStripped.ReplaceAllString(markup, "")
	text = intakeImages.ReplaceAllStringFunc(text, markdownImage)
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

// markdownImage keeps a picture where the sender put it. The source is left exactly as written
// so a reference into the message survives to be swapped for the stored file, and a picture
// hosted elsewhere keeps pointing there.
func markdownImage(tag string) string {
	source, description := "", ""

	for _, attribute := range intakeAttribute.FindAllStringSubmatch(tag, -1) {
		value := strings.Trim(attribute[2], `"'`)

		switch strings.ToLower(attribute[1]) {
		case "src":
			source = html.UnescapeString(strings.TrimSpace(value))
		case "alt":
			description = collapseIntakeSpace(html.UnescapeString(value))
		}
	}

	if source == "" {
		return ""
	}

	return "\n![" + strings.TrimSpace(description) + "](" + source + ")\n"
}

func collapseIntakeSpace(value string) string {
	return intakeSpaces.ReplaceAllString(strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " "), " ")
}
