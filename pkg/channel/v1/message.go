package channelv1

import (
	"errors"
	"slices"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	ErrTypeUnknown     = errors.New("runner channel message type is not recognised")
	ErrTypeNotInbound  = errors.New("that message type is only ever sent to a runner")
	ErrTypeNotOutbound = errors.New("that message type is only ever sent by a runner")
)

type MessageType string

const Ack MessageType = "ack"

const (
	Sync             MessageType = "channel.sync"
	ExecutionOffer   MessageType = "execution.offer"
	ExecutionStart   MessageType = "execution.start"
	ExecutionResume  MessageType = "execution.resume"
	ExecutionCancel  MessageType = "execution.cancel"
	QuestionAnswered MessageType = "question.answered"
	ExecutionRetain  MessageType = "execution.retain"
	RunnerPause      MessageType = "runner.pause"
	RunnerResume     MessageType = "runner.resume"
	RunnerConfigure  MessageType = "runner.update_config"
)

const (
	RunnerHello          MessageType = "runner.hello"
	RunnerHeartbeat      MessageType = "runner.heartbeat"
	ExecutionAccepted    MessageType = "execution.accepted"
	ExecutionDeclined    MessageType = "execution.declined"
	ExecutionStateReport MessageType = "execution.state"
	ExecutionEvent       MessageType = "execution.event"
	TranscriptRef        MessageType = "agent.transcript_ref"
	ServiceState         MessageType = "service.state"
	PreviewState         MessageType = "preview.state"
	QuestionAsked        MessageType = "question.asked"
	ChangeSetUpdated     MessageType = "changeset.updated"
	ExecutionRetention   MessageType = "execution.retention"
	ExecutionResult      MessageType = "execution.result"
)

func ServerMessages() []MessageType {
	return []MessageType{
		Sync, ExecutionOffer, ExecutionStart, ExecutionResume, ExecutionCancel, QuestionAnswered,
		ExecutionRetain, RunnerPause, RunnerResume, RunnerConfigure,
	}
}

func RunnerMessages() []MessageType {
	return []MessageType{
		RunnerHello, RunnerHeartbeat, ExecutionAccepted, ExecutionDeclined, ExecutionStateReport,
		ExecutionEvent, TranscriptRef, ServiceState, PreviewState, QuestionAsked, ChangeSetUpdated,
		ExecutionResult, ExecutionRetention,
	}
}

func (t MessageType) FromServer() bool {
	return slices.Contains(ServerMessages(), t)
}

func (t MessageType) FromRunner() bool {
	return slices.Contains(RunnerMessages(), t)
}

func (t MessageType) Valid() bool {
	return t == Ack || t.FromServer() || t.FromRunner()
}

type Message struct {
	ID          string
	Type        MessageType
	ExecutionID string
	IssuedAt    time.Time
	Payload     []byte
}

func NewServerMessage(
	kind MessageType, executionID string, payload []byte, issuedAt time.Time,
) (Message, error) {
	if !kind.FromServer() {
		return Message{}, ErrTypeNotOutbound
	}

	return newMessage(kind, executionID, payload, issuedAt), nil
}

func NewRunnerMessage(
	kind MessageType, executionID string, payload []byte, issuedAt time.Time,
) (Message, error) {
	if !kind.FromRunner() {
		return Message{}, ErrTypeNotInbound
	}

	return newMessage(kind, executionID, payload, issuedAt), nil
}

func newMessage(kind MessageType, executionID string, payload []byte, issuedAt time.Time) Message {
	return Message{
		ID:          ulid.Make().String(),
		Type:        kind,
		ExecutionID: executionID,
		IssuedAt:    issuedAt,
		Payload:     payload,
	}
}
