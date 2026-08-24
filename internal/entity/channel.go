package entity

import (
	"errors"
	"time"

	"github.com/google/uuid"

	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

const (
	ChannelVersion = channelv1.Version

	ChannelHeartbeat        = channelv1.Heartbeat
	ChannelLeaseTTL         = channelv1.LeaseTTL
	ChannelReadBlock        = channelv1.ReadBlock
	ChannelHandshakeTimeout = channelv1.HandshakeTimeout
	ChannelWriteTimeout     = channelv1.WriteTimeout

	ChannelSpoolWindow     = channelv1.SpoolWindow
	ChannelSpoolTTL        = channelv1.SpoolTTL
	ChannelSeenTTL         = channelv1.SeenTTL
	ChannelMaxMessageBytes = channelv1.MaxMessageBytes
)

var (
	ErrChannelTicketInvalid = errors.New("runner channel ticket is not valid")
	ErrChannelDisplaced     = errors.New("another connection took this runner's channel")
	ErrChannelSpoolLapsed   = errors.New("the message a runner resumed from is no longer spooled")

	ErrChannelEnvelopeInvalid = channelv1.ErrEnvelopeInvalid
	ErrChannelTypeUnknown     = channelv1.ErrTypeUnknown
	ErrChannelTypeNotInbound  = channelv1.ErrTypeNotInbound
	ErrChannelTypeNotOutbound = channelv1.ErrTypeNotOutbound
)

type ChannelMessageType = channelv1.MessageType

const ChannelAck = channelv1.Ack

const (
	ChannelSync             = channelv1.Sync
	ChannelExecutionOffer   = channelv1.ExecutionOffer
	ChannelExecutionStart   = channelv1.ExecutionStart
	ChannelExecutionResume  = channelv1.ExecutionResume
	ChannelExecutionCancel  = channelv1.ExecutionCancel
	ChannelQuestionAnswered = channelv1.QuestionAnswered
	ChannelExecutionRetain  = channelv1.ExecutionRetain
	ChannelRunnerPause      = channelv1.RunnerPause
	ChannelRunnerResume     = channelv1.RunnerResume
	ChannelRunnerConfigure  = channelv1.RunnerConfigure
)

const (
	ChannelRunnerHello       = channelv1.RunnerHello
	ChannelRunnerHeartbeat   = channelv1.RunnerHeartbeat
	ChannelExecutionAccepted = channelv1.ExecutionAccepted
	ChannelExecutionDeclined = channelv1.ExecutionDeclined
	ChannelExecutionState    = channelv1.ExecutionStateReport
	ChannelExecutionEvent    = channelv1.ExecutionEvent
	ChannelTranscriptRef     = channelv1.TranscriptRef
	ChannelServiceState      = channelv1.ServiceState
	ChannelPreviewState      = channelv1.PreviewState
	ChannelQuestionAsked     = channelv1.QuestionAsked
	ChannelChangeSetUpdated  = channelv1.ChangeSetUpdated
	ChannelExecutionResult   = channelv1.ExecutionResult
	ChannelExecutionKept     = channelv1.ExecutionRetention
)

func ChannelServerMessages() []ChannelMessageType {
	return channelv1.ServerMessages()
}

func ChannelRunnerMessages() []ChannelMessageType {
	return channelv1.RunnerMessages()
}

type ChannelMessage = channelv1.Message

type SpooledMessage struct {
	Cursor  string
	Message ChannelMessage
}

type RunnerLoad struct {
	Capacity     int
	Used         int
	Paused       bool
	DiskPressure bool
	CPUPressure  bool
}

func RunnerLoadOf(pulse channelv1.Pulse) RunnerLoad {
	return RunnerLoad{
		Capacity:     pulse.Capacity,
		Used:         pulse.Used,
		Paused:       pulse.Paused,
		DiskPressure: pulse.DiskPressure,
		CPUPressure:  pulse.CPUPressure,
	}
}

type RunnerPresence struct {
	RunnerID uuid.UUID
	Epoch    string
	SeenAt   time.Time
	Load     RunnerLoad
}

func (p RunnerPresence) Live() bool {
	return p.Epoch != ""
}

func (p RunnerPresence) Free() int {
	free := p.Load.Capacity - p.Load.Used
	if free < 0 {
		return 0
	}

	return free
}

func (p RunnerPresence) Available() bool {
	return p.Live() && !p.Load.Paused && !p.Load.DiskPressure && p.Free() > 0
}

func NewServerMessage(
	kind ChannelMessageType, executionID string, payload []byte, issuedAt time.Time,
) (ChannelMessage, error) {
	return channelv1.NewServerMessage(kind, executionID, payload, issuedAt)
}

func ValidateChannelInbound(message ChannelMessage) error {
	return channelv1.ValidateInbound(message)
}
