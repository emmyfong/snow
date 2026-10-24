// Package providertest has a scripted provider.Provider for tests of the
// packages that use one.
package providertest

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/emmyfong/snow/internal/provider"
)

// Reply scripts one call to Stream: Stream returns StartErr, or a stream
// that yields Events and then ends with Err, or io.EOF when Err is nil.
// With Wait, the stream waits after its events until its context ends, as
// a reply that is still streaming does.
type Reply struct {
	Events   []provider.Event
	Err      error
	StartErr error
	Wait     bool
}

// ErrNoReply means Stream was called more times than replies were scripted.
var ErrNoReply = errors.New("providertest: no reply scripted")

// Fake is a provider.Provider that plays scripted replies in order. It is
// safe for use from several goroutines.
type Fake struct {
	name      string
	models    []provider.Model
	modelsErr error

	mu       sync.Mutex
	replies  []Reply
	requests []provider.Request
}

// New returns a Fake called name that plays replies, one per Stream call.
func New(name string, replies ...Reply) *Fake {
	return &Fake{name: name, replies: replies}
}

// SetModels sets what Models returns.
func (f *Fake) SetModels(models []provider.Model, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.models, f.modelsErr = models, err
}

// Requests returns the requests Stream received, in order.
func (f *Fake) Requests() []provider.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]provider.Request(nil), f.requests...)
}

// Name implements provider.Provider.
func (f *Fake) Name() string { return f.name }

// Models implements provider.Provider.
func (f *Fake) Models(ctx context.Context) ([]provider.Model, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]provider.Model(nil), f.models...), f.modelsErr
}

// Stream implements provider.Provider.
func (f *Fake) Stream(ctx context.Context, req provider.Request) (provider.Stream, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)
	if len(f.replies) == 0 {
		return nil, ErrNoReply
	}
	r := f.replies[0]
	f.replies = f.replies[1:]
	if r.StartErr != nil {
		return nil, r.StartErr
	}
	return &stream{ctx: ctx, reply: r}, nil
}

type stream struct {
	ctx    context.Context
	reply  Reply
	next   int
	closed bool
}

func (s *stream) Next() (provider.Event, error) {
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	if s.closed {
		return nil, io.EOF
	}
	if s.next < len(s.reply.Events) {
		s.next++
		return s.reply.Events[s.next-1], nil
	}
	if s.reply.Wait {
		<-s.ctx.Done()
		return nil, s.ctx.Err()
	}
	if s.reply.Err != nil {
		return nil, s.reply.Err
	}
	return nil, io.EOF
}

func (s *stream) Close() error {
	s.closed = true
	return nil
}
