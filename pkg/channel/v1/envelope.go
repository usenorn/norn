package channelv1

import (
	"encoding/json"
	"errors"
	"time"
)

const Version = 1

const (
	Heartbeat        = 15 * time.Second
	LeaseTTL         = 60 * time.Second
	ReadBlock        = 30 * time.Second
	HandshakeTimeout = 10 * time.Second
	WriteTimeout     = 10 * time.Second

	SpoolWindow     = 1000
	SpoolTTL        = 24 * time.Hour
	SeenTTL         = time.Hour
	MaxMessageBytes = 1 << 20
)

var ErrEnvelopeInvalid = errors.New("runner channel envelope is malformed")

type Envelope struct {
	V       int             `json:"v"`
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	TS      time.Time       `json:"ts"`
	ExecID  string          `json:"exec_id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	AckID   string          `json:"ack_id,omitempty"`
}

func Frame(message Message) Envelope {
	return Envelope{
		V:       Version,
		ID:      message.ID,
		Type:    string(message.Type),
		TS:      message.IssuedAt.UTC(),
		ExecID:  message.ExecutionID,
		Payload: json.RawMessage(message.Payload),
	}
}

func Acknowledgement(id string, now time.Time) Envelope {
	return Envelope{
		V:     Version,
		Type:  string(Ack),
		TS:    now.UTC(),
		AckID: id,
	}
}

func (e Envelope) Message() Message {
	return Message{
		ID:          e.ID,
		Type:        MessageType(e.Type),
		ExecutionID: e.ExecID,
		IssuedAt:    e.TS,
		Payload:     []byte(e.Payload),
	}
}

func (e Envelope) Acknowledging() bool {
	return MessageType(e.Type) == Ack
}

func ValidateInbound(message Message) error {
	switch {
	case message.ID == "":
		return ErrEnvelopeInvalid
	case !message.Type.Valid():
		return ErrTypeUnknown
	case message.Type.FromServer():
		return ErrTypeNotInbound
	default:
		return nil
	}
}

func ValidateOutbound(message Message) error {
	switch {
	case message.ID == "":
		return ErrEnvelopeInvalid
	case !message.Type.Valid():
		return ErrTypeUnknown
	case message.Type.FromRunner():
		return ErrTypeNotOutbound
	default:
		return nil
	}
}
