package clauderuntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/emmyfong/snow/internal/shell"
)

var (
	// ErrClosed means the runtime transport no longer accepts input.
	ErrClosed = errors.New("runtime is closed")
	// ErrProtocol means the runtime sent invalid structured output.
	ErrProtocol = errors.New("invalid Claude runtime protocol")
	// ErrBackpressure means the consumer did not keep up with runtime events.
	ErrBackpressure = errors.New("runtime event queue is full")
	// ErrDelivery means a submitted write may have reached the runtime. The
	// caller must preserve the input and must not automatically submit it again.
	ErrDelivery = errors.New("runtime input delivery is uncertain")
	// ErrStop means process-tree cleanup failed. A protocol failure alone does
	// not mean usage can continue after cleanup was confirmed.
	ErrStop = errors.New("runtime process cleanup failed")
)

// MaxFrameBytes bounds a single protocol message, including large tool results.
const MaxFrameBytes = 4 << 20

// Frame is one validated runtime envelope. Raw retains the original fields for
// the conversation controller; Type alone never establishes run completion.
type Frame struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	RequestID string          `json:"request_id,omitempty"`
	Raw       json.RawMessage `json:"-"`
}

type writeRequest struct {
	ctx  context.Context
	data []byte
	done chan error
}

// Stream owns a persistent Claude process and its bounded NDJSON transport.
// Writes run on one worker, never under a lock or on the server owner goroutine.
// Stop terminates the whole owned process tree and waits for pipe cleanup.
type Stream struct {
	process  *shell.Process
	input    *os.File
	output   *os.File
	frames   chan Frame
	writes   chan writeRequest
	stopping chan struct{}
	done     chan struct{}
	readDone chan struct{}
	once     sync.Once
	err      error // read only after done closes
}

// StartStream launches a resolved, profile-configured Claude command. Diagnostic
// output is not mixed into the protocol or retained in a login transcript.
func StartStream(launch shell.Launch) (*Stream, error) {
	in, input, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("open Claude input: %w", err)
	}
	output, out, err := os.Pipe()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("open Claude output: %w", err), in.Close(), input.Close())
	}
	p, err := shell.StartProcess(launch, in, out, io.Discard)
	closeErr := errors.Join(in.Close(), out.Close())
	if err != nil {
		return nil, errors.Join(err, closeErr, input.Close(), output.Close())
	}
	s := &Stream{process: p, input: input, output: output,
		frames: make(chan Frame, 64), writes: make(chan writeRequest, 16),
		stopping: make(chan struct{}), done: make(chan struct{}), readDone: make(chan struct{})}
	go s.read()
	go s.write()
	if closeErr != nil {
		s.finish(closeErr)
		<-s.done
		return nil, s.err
	}
	return s, nil
}

// Frames yields ordered runtime envelopes. A full bounded queue pauses this
// stream's reader instead of dropping requests or growing indefinitely. Stop
// bypasses the queue and releases the reader even if nobody consumes frames.
func (s *Stream) Frames() <-chan Frame { return s.frames }

// Done closes after the process and its descendants have been stopped.
func (s *Stream) Done() <-chan struct{} { return s.done }

// Err returns the terminal transport error. Call it only after Done closes.
func (s *Stream) Err() error { return s.err }

// Send writes a single JSON message. A cancellation after enqueueing can leave
// delivery ambiguous, reported as ErrDelivery. It never retries the write.
func (s *Stream) Send(ctx context.Context, message any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode Claude input: %w", err)
	}
	if len(data) > MaxFrameBytes {
		return fmt.Errorf("%w: input exceeds frame limit", ErrProtocol)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	req := writeRequest{ctx: ctx, data: append(data, '\n'), done: make(chan error, 1)}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.stopping:
		return ErrClosed
	case s.writes <- req:
	}
	select {
	case err := <-req.done:
		return err
	case <-ctx.Done():
		return errors.Join(ErrDelivery, ctx.Err())
	case <-s.stopping:
		return errors.Join(ErrDelivery, ErrClosed)
	}
}

func (s *Stream) write() {
	for {
		select {
		case <-s.stopping:
			return
		case req := <-s.writes:
			if err := req.ctx.Err(); err != nil {
				req.done <- err
				continue
			}
			_, err := s.input.Write(req.data)
			if err != nil {
				req.done <- errors.Join(ErrDelivery, ErrClosed)
				s.finish(fmt.Errorf("write Claude input: %w", err))
				return
			}
			req.done <- nil
		}
	}
}

// Stop closes input and terminates the runtime tree. It is safe to call more than
// once. A transport failure remains visible even when cleanup succeeds.
func (s *Stream) Stop() error {
	s.finish(nil)
	<-s.done
	return s.err
}

func (s *Stream) finish(cause error) {
	s.once.Do(func() {
		close(s.stopping)
		go func() {
			closeErr := errors.Join(s.input.Close(), s.output.Close())
			stopErr := s.process.Stop(time.Second)
			if stopErr != nil {
				stopErr = errors.Join(ErrStop, stopErr)
			}
			<-s.readDone
			s.err = errors.Join(cause, closeErr, stopErr)
			close(s.frames)
			close(s.done)
		}()
	})
}
