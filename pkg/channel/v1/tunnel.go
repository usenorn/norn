package channelv1

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
)

const (
	TunnelPath           = "/v1/runners/tunnel"
	TunnelMaxHeaderBytes = 4 << 10
)

type StreamOpen struct {
	Execution string `json:"execution"`
	Preview   string `json:"preview"`
}

type StreamReady struct {
	Open   bool   `json:"open"`
	Reason string `json:"reason,omitempty"`
}

type Stream struct {
	net.Conn

	buffered *bufio.Reader
}

func NewStream(conn net.Conn) *Stream {
	return &Stream{Conn: conn, buffered: bufio.NewReader(conn)}
}

func (s *Stream) Read(into []byte) (int, error) {
	return s.buffered.Read(into)
}

func (s *Stream) WriteFrame(frame any) error {
	return WriteFrame(s.Conn, frame)
}

func (s *Stream) ReadFrame(frame any) error {
	return ReadFrame(s.buffered, frame)
}

func WriteFrame(w io.Writer, frame any) error {
	payload, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("encode tunnel frame: %w", err)
	}

	if len(payload) > TunnelMaxHeaderBytes {
		return fmt.Errorf(
			"tunnel frame is %d bytes, over the %d allowed", len(payload), TunnelMaxHeaderBytes,
		)
	}

	if _, err := w.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("write tunnel frame: %w", err)
	}

	return nil
}

func ReadFrame(r *bufio.Reader, frame any) error {
	line, err := r.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read tunnel frame: %w", err)
	}

	if len(line) > TunnelMaxHeaderBytes {
		return fmt.Errorf(
			"tunnel frame is %d bytes, over the %d allowed", len(line), TunnelMaxHeaderBytes,
		)
	}

	if err := json.Unmarshal([]byte(line), frame); err != nil {
		return fmt.Errorf("decode tunnel frame: %w", err)
	}

	return nil
}
