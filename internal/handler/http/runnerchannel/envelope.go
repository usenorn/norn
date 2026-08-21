package runnerchannel

import (
	"encoding/json"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

type envelope struct {
	V       int             `json:"v"`
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	TS      time.Time       `json:"ts"`
	ExecID  string          `json:"exec_id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	AckID   string          `json:"ack_id,omitempty"`
}

func frame(message entity.ChannelMessage) envelope {
	return envelope{
		V:       entity.ChannelVersion,
		ID:      message.ID,
		Type:    string(message.Type),
		TS:      message.IssuedAt.UTC(),
		ExecID:  message.ExecutionID,
		Payload: json.RawMessage(message.Payload),
	}
}

func acknowledgement(id string) envelope {
	return envelope{
		V:     entity.ChannelVersion,
		Type:  string(entity.ChannelAck),
		TS:    time.Now().UTC(),
		AckID: id,
	}
}

func (e envelope) message() entity.ChannelMessage {
	return entity.ChannelMessage{
		ID:          e.ID,
		Type:        entity.ChannelMessageType(e.Type),
		ExecutionID: e.ExecID,
		IssuedAt:    e.TS,
		Payload:     []byte(e.Payload),
	}
}
