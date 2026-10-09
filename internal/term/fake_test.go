package term

import (
	"bytes"
	"context"
	"io"
	"sync"
)

// fakePTY stands in for a real PTY in unit tests. The test plays the shell:
// it writes output with shellWrites and reads what the pane sent with sent.
type fakePTY struct {
	out    *io.PipeReader
	outW   *io.PipeWriter
	mu     sync.Mutex
	in     bytes.Buffer
	exited chan struct{}
	once   sync.Once
	cols   int
	rows   int
}

func newFakePTY() *fakePTY {
	r, w := io.Pipe()
	return &fakePTY{out: r, outW: w, exited: make(chan struct{})}
}

func (f *fakePTY) Read(p []byte) (int, error) { return f.out.Read(p) }

func (f *fakePTY) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.in.Write(p)
}

func (f *fakePTY) Resize(cols, rows int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cols, f.rows = cols, rows
	return nil
}

func (f *fakePTY) Wait(ctx context.Context) error {
	select {
	case <-f.exited:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (f *fakePTY) Close() error {
	f.exit()
	return f.outW.Close()
}

func (f *fakePTY) exit() { f.once.Do(func() { close(f.exited) }) }

func (f *fakePTY) shellWrites(s string) { _, _ = f.outW.Write([]byte(s)) }

func (f *fakePTY) sent() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.in.String()
}
