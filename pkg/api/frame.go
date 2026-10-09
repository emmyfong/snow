package api

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
)

// MaxFrameSize bounds a payload so a corrupt length cannot exhaust memory.
const MaxFrameSize = 1 << 20

// Frame errors. Callers match them with errors.Is.
var (
	ErrFrameTooLarge = errors.New("api: frame too large")
	ErrUnknownType   = errors.New("api: unknown message type")
	ErrBadPayload    = errors.New("api: malformed payload")
	ErrNilMessage    = errors.New("api: nil message")
)

const headerSize = 5 // 4-byte length + 1-byte type

// WriteFrame writes m as one frame, in one Write call.
func WriteFrame(w io.Writer, m Message) error {
	if m == nil || reflect.ValueOf(m).IsNil() {
		return ErrNilMessage
	}
	payload, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("encode %T: %w", m, err)
	}
	if len(payload) > MaxFrameSize {
		return fmt.Errorf("%w: %d bytes", ErrFrameTooLarge, len(payload))
	}
	frame := make([]byte, headerSize, headerSize+len(payload))
	binary.BigEndian.PutUint32(frame, uint32(len(payload))) //nolint:gosec // bounded by MaxFrameSize
	frame[4] = byte(m.Type())
	if _, err := w.Write(append(frame, payload...)); err != nil {
		return fmt.Errorf("write frame: %w", err)
	}
	return nil
}

// ReadFrame reads one frame. It returns io.EOF only when the stream ends
// cleanly between frames.
//
// ErrUnknownType and ErrBadPayload leave the stream usable: the payload has
// been consumed, so the caller may ignore the frame and read the next one. A
// newer peer can therefore add message types. Any other error leaves the
// stream in an unknown state; close the connection.
func ReadFrame(r io.Reader) (Message, error) {
	var header [headerSize]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("read frame header: %w", err)
	}
	size := binary.BigEndian.Uint32(header[:4])
	if size > MaxFrameSize {
		return nil, fmt.Errorf("%w: %d bytes", ErrFrameTooLarge, size)
	}
	m := newMessage(MsgType(header[4]))
	if m == nil {
		// size is already bounded, so skipping the payload is safe.
		if _, err := io.CopyN(io.Discard, r, int64(size)); err != nil {
			return nil, fmt.Errorf("skip unknown frame: %w", err)
		}
		return nil, fmt.Errorf("%w: %d", ErrUnknownType, header[4])
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF
		}
		return nil, fmt.Errorf("read frame payload: %w", err)
	}
	if bytes.Equal(bytes.TrimSpace(payload), []byte("null")) {
		return nil, fmt.Errorf("%w: null", ErrBadPayload)
	}
	if err := json.Unmarshal(payload, m); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadPayload, err)
	}
	return m, nil
}
