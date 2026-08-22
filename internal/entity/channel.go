package entity

import (
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

const (
	ChannelVersion = 1

	ChannelHeartbeat        = 15 * time.Second
	ChannelLeaseTTL         = 60 * time.Second
	ChannelReadBlock        = 30 * time.Second
	ChannelHandshakeTimeout = 10 * time.Second
	ChannelWriteTimeout     = 10 * time.Second

	ChannelSpoolWindow     = 1000
	ChannelSpoolTTL        = 24 * time.Hour
	ChannelSeenTTL         = time.Hour
	ChannelMaxMessageBytes = 1 << 20
)

var (
	ErrChannelTicketInvalid   = errors.New("runner channel ticket is not valid")
	ErrChannelDisplaced       = errors.New("another connection took this runner's channel")
	ErrChannelEnvelopeInvalid = errors.New("runner channel envelope is malformed")
	ErrChannelTypeUnknown     = errors.New("runner channel message type is not recognised")
	ErrChannelTypeNotInbound  = errors.New("that message type is only ever sent to a runner")
	ErrChannelTypeNotOutbound = errors.New("that message type is only ever sent by a runner")
	ErrChannelSpoolLapsed     = errors.New("the message a runner resumed from is no longer spooled")
)

type ChannelMessageType string

const ChannelAck ChannelMessageType = "ack"

const (
	ChannelSync             ChannelMessageType = "channel.sync"
	ChannelExecutionOffer   ChannelMessageType = "execution.offer"
	ChannelExecutionStart   ChannelMessageType = "execution.start"
	ChannelExecutionResume  ChannelMessageType = "execution.resume"
	ChannelExecutionCancel  ChannelMessageType = "execution.cancel"
	ChannelQuestionAnswered ChannelMessageType = "question.answered"
	ChannelRunnerPause      ChannelMessageType = "runner.pause"
	ChannelRunnerResume     ChannelMessageType = "runner.resume"
	ChannelRunnerConfigure  ChannelMessageType = "runner.update_config"
)

const (
	ChannelRunnerHello       ChannelMessageType = "runner.hello"
	ChannelRunnerHeartbeat   ChannelMessageType = "runner.heartbeat"
	ChannelExecutionAccepted ChannelMessageType = "execution.accepted"
	ChannelExecutionDeclined ChannelMessageType = "execution.declined"
	ChannelExecutionState    ChannelMessageType = "execution.state"
	ChannelExecutionEvent    ChannelMessageType = "execution.event"
	ChannelTranscriptRef     ChannelMessageType = "agent.transcript_ref"
	ChannelServiceState      ChannelMessageType = "service.state"
	ChannelPreviewState      ChannelMessageType = "preview.state"
	ChannelQuestionAsked     ChannelMessageType = "question.asked"
	ChannelChangeSetUpdated  ChannelMessageType = "changeset.updated"
	ChannelExecutionResult   ChannelMessageType = "execution.result"
)

func ChannelServerMessages() []ChannelMessageType {
	return []ChannelMessageType{
		ChannelSync, ChannelExecutionOffer, ChannelExecutionStart, ChannelExecutionResume,
		ChannelExecutionCancel, ChannelQuestionAnswered, ChannelRunnerPause, ChannelRunnerResume,
		ChannelRunnerConfigure,
	}
}

func ChannelRunnerMessages() []ChannelMessageType {
	return []ChannelMessageType{
		ChannelRunnerHello, ChannelRunnerHeartbeat, ChannelExecutionAccepted,
		ChannelExecutionDeclined, ChannelExecutionState, ChannelExecutionEvent,
		ChannelTranscriptRef, ChannelServiceState, ChannelPreviewState, ChannelQuestionAsked,
		ChannelChangeSetUpdated, ChannelExecutionResult,
	}
}

func ChannelHandledMessages() []ChannelMessageType {
	return []ChannelMessageType{ChannelAck, ChannelRunnerHello, ChannelRunnerHeartbeat}
}

func (t ChannelMessageType) FromServer() bool {
	return slices.Contains(ChannelServerMessages(), t)
}

func (t ChannelMessageType) FromRunner() bool {
	return slices.Contains(ChannelRunnerMessages(), t)
}

func (t ChannelMessageType) Valid() bool {
	return t == ChannelAck || t.FromServer() || t.FromRunner()
}

func (t ChannelMessageType) Handled() bool {
	return slices.Contains(ChannelHandledMessages(), t)
}

type ChannelMessage struct {
	ID          string
	Type        ChannelMessageType
	ExecutionID string
	IssuedAt    time.Time
	Payload     []byte
}

type SpooledMessage struct {
	Cursor  string
	Message ChannelMessage
}

type RunnerPresence struct {
	RunnerID uuid.UUID
	Epoch    string
	SeenAt   time.Time
}

func (p RunnerPresence) Live() bool {
	return p.Epoch != ""
}

func NewServerMessage(
	kind ChannelMessageType,
	executionID string,
	payload []byte,
	issuedAt time.Time,
) (ChannelMessage, error) {
	if !kind.FromServer() {
		return ChannelMessage{}, ErrChannelTypeNotOutbound
	}

	return ChannelMessage{
		ID:          ulid.Make().String(),
		Type:        kind,
		ExecutionID: executionID,
		IssuedAt:    issuedAt,
		Payload:     payload,
	}, nil
}

func ValidateChannelInbound(message ChannelMessage) error {
	switch {
	case message.ID == "":
		return ErrChannelEnvelopeInvalid
	case !message.Type.Valid():
		return ErrChannelTypeUnknown
	case message.Type.FromServer():
		return ErrChannelTypeNotInbound
	default:
		return nil
	}
}
