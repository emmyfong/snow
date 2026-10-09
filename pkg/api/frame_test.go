package api

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// everyMessage holds one populated value of each message type.
func everyMessage() []Message {
	return []Message{
		&Hello{ClientVersion: "0.1.0", Protocol: ProtocolVersion, Cols: 120, Rows: 40},
		&Welcome{ServerVersion: "0.1.0", Protocol: ProtocolVersion},
		&Attach{Session: "api", Create: true},
		&Input{Pane: 3, Key: &Key{Code: KeyUp}},
		&Input{Pane: 3, Paste: "echo hi\n"},
		&Mouse{Pane: 1, X: 4, Y: 2, Button: "left", Action: "press"},
		&Resize{Cols: 80, Rows: 24},
		&Command{Op: "split", Target: 2, Args: map[string]string{"dir": "right"}},
		&Layout{Session: "api", Active: 0, Windows: []WindowInfo{{Index: 0, Name: "dev", Panes: []PaneInfo{{ID: 1, Rect: Rect{W: 80, H: 23}, Profile: "Ubuntu", Focused: true}}}}},
		&PaneUpdate{Pane: 1, Lines: []LineUpdate{{Row: 0, Text: "\x1b[31mred\x1b[0m"}}, Cursor: &Cursor{X: 3, Y: 0, Visible: true}},
		&SessionList{Sessions: []SessionInfo{{Name: "api", Windows: 2, Attached: 1, State: StateRunning, LastUsed: time.Date(2026, 10, 9, 9, 14, 0, 0, time.UTC)}}},
		&Bell{Pane: 2},
		&Error{Code: "no-session", Message: "session api does not exist"},
	}
}

func TestFrameRoundTrip(t *testing.T) {
	for _, m := range everyMessage() {
		t.Run(reflect.TypeOf(m).Elem().Name(), func(t *testing.T) {
			var buf bytes.Buffer
			if err := WriteFrame(&buf, m); err != nil {
				t.Fatal(err)
			}
			got, err := ReadFrame(&buf)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, m) {
				t.Fatalf("round trip = %#v, want %#v", got, m)
			}
		})
	}
}

// oneByteReader returns at most one byte per Read, like a slow socket.
type oneByteReader struct{ r io.Reader }

func (o oneByteReader) Read(p []byte) (int, error) { return o.r.Read(p[:min(1, len(p))]) }

func TestFrameSplitReads(t *testing.T) {
	var buf bytes.Buffer
	want := &Input{Pane: 7, Key: &Key{Code: "a", Text: "a"}}
	if err := WriteFrame(&buf, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFrame(oneByteReader{&buf})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func rawFrame(length uint32, typ byte, payload []byte) []byte {
	b := binary.BigEndian.AppendUint32(nil, length)
	b = append(b, typ)
	return append(b, payload...)
}

func TestUnknownTypeIsSkipped(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(rawFrame(4, 250, []byte(`{"a":1}`)[:4]))
	if err := WriteFrame(&buf, &Bell{Pane: 9}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFrame(&buf); !errors.Is(err, ErrUnknownType) {
		t.Fatalf("first read error = %v, want ErrUnknownType", err)
	}
	m, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("second read: %v (the unknown payload was not skipped)", err)
	}
	if b, ok := m.(*Bell); !ok || b.Pane != 9 {
		t.Fatalf("second read = %#v, want Bell{9}", m)
	}
}

func TestWriteFrameRejects(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteFrame(&buf, nil); !errors.Is(err, ErrNilMessage) {
		t.Fatalf("nil message error = %v, want ErrNilMessage", err)
	}
	if err := WriteFrame(&buf, (*Bell)(nil)); !errors.Is(err, ErrNilMessage) {
		t.Fatalf("typed nil error = %v, want ErrNilMessage", err)
	}
	big := &Input{Pane: 1, Paste: strings.Repeat("x", MaxFrameSize)}
	if err := WriteFrame(&buf, big); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("oversize error = %v, want ErrFrameTooLarge", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("rejected frames wrote %d bytes", buf.Len())
	}
}

func TestFrameErrors(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want error
	}{
		{"too large", rawFrame(MaxFrameSize+1, byte(TypeBell), nil), ErrFrameTooLarge},
		{"unknown type", rawFrame(2, 250, []byte("{}")), ErrUnknownType},
		{"bad json", rawFrame(5, byte(TypeBell), []byte("{nope")), ErrBadPayload},
		{"null payload", rawFrame(4, byte(TypeBell), []byte("null")), ErrBadPayload},
		{"truncated payload", rawFrame(10, byte(TypeBell), []byte("{}")), io.ErrUnexpectedEOF},
		{"clean end of stream", nil, io.EOF},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReadFrame(bytes.NewReader(tt.in))
			if !errors.Is(err, tt.want) {
				t.Fatalf("ReadFrame error = %v, want %v", err, tt.want)
			}
		})
	}
}

// splitWriter writes each frame in two pieces with a yield between them, like
// a buffered or TLS writer. Without Conn's lock, frames from different
// goroutines interleave. io.Pipe alone serializes whole writes and would hide
// a missing lock.
type splitWriter struct{ w io.Writer }

func (s splitWriter) Write(p []byte) (int, error) {
	half := len(p) / 2
	n, err := s.w.Write(p[:half])
	if err != nil {
		return n, err
	}
	runtime.Gosched()
	m, err := s.w.Write(p[half:])
	return n + m, err
}

func TestConnConcurrentSends(t *testing.T) {
	r, w := io.Pipe()
	c := NewConn(struct {
		io.Reader
		io.Writer
	}{r, splitWriter{w}})
	const senders, each = 50, 20
	var wg sync.WaitGroup
	for i := range senders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range each {
				if err := c.Send(&Bell{Pane: i*1000 + j}); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	go func() { wg.Wait(); _ = w.Close() }()
	seen := 0
	for {
		m, err := c.Receive()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("frame %d corrupted: %v", seen, err)
		}
		if _, ok := m.(*Bell); !ok {
			t.Fatalf("frame %d is %T", seen, m)
		}
		seen++
	}
	if seen != senders*each {
		t.Fatalf("received %d frames, want %d", seen, senders*each)
	}
}
