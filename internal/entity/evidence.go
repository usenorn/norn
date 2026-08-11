package entity

import (
	"errors"
	"fmt"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	EvidenceOutputMaxBytes  = 64 * 1024
	EvidenceOutputHeadBytes = 48 * 1024
	EvidenceOutputTailBytes = 15 * 1024
	EvidenceCommandMaxLen   = 2000
	EvidenceElision         = "\n[norn dropped %d bytes of output here]\n"
)

var (
	ErrEvidenceNotFound = errors.New("evidence not found")
	ErrEvidenceEmpty    = errors.New("evidence needs the output it was drawn from")
)

type EvidenceVerdict string

const (
	EvidencePassed         EvidenceVerdict = "passed"
	EvidenceFailed         EvidenceVerdict = "failed"
	EvidenceAbsentNegative EvidenceVerdict = "absent_negative"
	EvidenceInconclusive   EvidenceVerdict = "inconclusive"
)

func EvidenceVerdicts() []EvidenceVerdict {
	return []EvidenceVerdict{
		EvidencePassed,
		EvidenceFailed,
		EvidenceAbsentNegative,
		EvidenceInconclusive,
	}
}

func (v EvidenceVerdict) Valid() bool {
	return slices.Contains(EvidenceVerdicts(), v)
}

func (v EvidenceVerdict) Proves() bool {
	return v == EvidencePassed
}

func (v EvidenceVerdict) Disproves() bool {
	return v == EvidenceFailed
}

func (v EvidenceVerdict) RestsOnAbsence() bool {
	return v == EvidenceAbsentNegative
}

type EvidenceChannel string

const (
	EvidenceChannelCommand    EvidenceChannel = "command"
	EvidenceChannelHTTP       EvidenceChannel = "http"
	EvidenceChannelLog        EvidenceChannel = "log"
	EvidenceChannelScreenshot EvidenceChannel = "screenshot"
	EvidenceChannelDatabase   EvidenceChannel = "database"
	EvidenceChannelHuman      EvidenceChannel = "human"
)

func EvidenceChannels() []EvidenceChannel {
	return []EvidenceChannel{
		EvidenceChannelCommand,
		EvidenceChannelHTTP,
		EvidenceChannelLog,
		EvidenceChannelScreenshot,
		EvidenceChannelDatabase,
		EvidenceChannelHuman,
	}
}

func (c EvidenceChannel) Valid() bool {
	return slices.Contains(EvidenceChannels(), c)
}

type Evidence struct {
	ID                  uuid.UUID
	WorkspaceID         uuid.UUID
	IssueID             uuid.UUID
	CheckID             uuid.UUID
	Verdict             EvidenceVerdict
	Channel             EvidenceChannel
	Command             string
	Output              string
	Truncated           bool
	Redactions          int
	ExitCode            *int
	ObservedAt          time.Time
	ReceivedAt          time.Time
	Actor               ActivityAttribution
	ActorName           string
	CodeLinkID          uuid.UUID
	CommitSHA           string
	ScrubbedByAccountID uuid.UUID
	ScrubbedAt          *time.Time
}

func (e Evidence) Attested() bool {
	return e.Actor.Kind == ActorKindUser
}

func EvidenceLink(links []CodeLink) (CodeLink, bool) {
	var newest CodeLink

	found := false

	for _, link := range links {
		if link.Kind != CodeLinkChange || link.HeadSHA == "" || link.State == CodeChangeClosed {
			continue
		}

		if !found || link.CreatedAt.After(newest.CreatedAt) {
			newest, found = link, true
		}
	}

	return newest, found
}

func ObservationTime(claimed, received time.Time) time.Time {
	if claimed.IsZero() || claimed.After(received) {
		return received
	}

	return claimed
}

func TruncateEvidenceOutput(output string) (string, bool) {
	if len(output) <= EvidenceOutputMaxBytes {
		return output, false
	}

	head := trimPartialRune(output[:EvidenceOutputHeadBytes])
	tail := skipPartialRune(output[len(output)-EvidenceOutputTailBytes:])

	return head + fmt.Sprintf(EvidenceElision, len(output)-len(head)-len(tail)) + tail, true
}

func trimPartialRune(text string) string {
	for len(text) > 0 {
		decoded, width := utf8.DecodeLastRuneInString(text)
		if decoded != utf8.RuneError || width > 1 {
			return text
		}

		text = text[:len(text)-1]
	}

	return text
}

func skipPartialRune(text string) string {
	for len(text) > 0 && !utf8.RuneStart(text[0]) {
		text = text[1:]
	}

	return text
}

func ValidateEvidenceVerdict(field string, verdict EvidenceVerdict) FieldError {
	if !verdict.Valid() {
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	}

	return FieldError{}
}

func ValidateEvidenceChannel(field string, channel EvidenceChannel) FieldError {
	if !channel.Valid() {
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	}

	return FieldError{}
}

func ValidateEvidenceCommand(field, command string) FieldError {
	if utf8.RuneCountInString(command) > EvidenceCommandMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}
