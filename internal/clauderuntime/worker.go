package clauderuntime

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"time"
)

// operation is processed in order by one I/O worker. The conversation actor
// stays available for stop, queue updates and arriving runtime events.
type operation struct {
	kind, id, text, session string
	model, effort           string // for a prompt that starts Claude
	allow                   bool
	input                   json.RawMessage
	ids                     []string
}

type operationResult struct {
	operation operation
	err       error
}

type stopTaskRef struct{ id, run string }

type runWorker struct {
	ctx         context.Context
	cancel      context.CancelFunc
	client      atomic.Pointer[Client]
	operations  chan operation
	ready       chan *Client
	results     chan operationResult
	done        chan struct{}
	stopQueries chan chan []stopTaskRef
	start       func(context.Context, StartRequest) (*Client, error)
}

func newRunWorker(start func(context.Context, StartRequest) (*Client, error)) *runWorker {
	ctx, cancel := context.WithCancel(context.Background())
	w := &runWorker{ctx: ctx, cancel: cancel, operations: make(chan operation, 16),
		ready: make(chan *Client, 1), results: make(chan operationResult, 16), done: make(chan struct{}),
		stopQueries: make(chan chan []stopTaskRef), start: start}
	go w.run()
	return w
}

func (w *runWorker) run() {
	defer close(w.done)
	for {
		select {
		case <-w.ctx.Done():
			return
		case op := <-w.operations:
			ctx, cancel := context.WithTimeout(w.ctx, time.Minute)
			err := w.perform(ctx, op)
			cancel()
			select {
			case w.results <- operationResult{operation: op, err: err}:
			case <-w.ctx.Done():
				return
			}
		}
	}
}

func (w *runWorker) perform(ctx context.Context, op operation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	client := w.client.Load()
	if client == nil {
		if op.kind != "prompt" {
			return ErrClosed
		}
		var err error
		client, err = w.start(ctx, StartRequest{Session: op.session, Model: op.model, Effort: op.effort})
		if err != nil {
			return err
		}
		if client == nil {
			return ErrClosed
		}
		w.client.Store(client)
		select {
		case w.ready <- client:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	switch op.kind {
	case "prompt":
		return client.Prompt(ctx, op.session, op.text)
	case "answer":
		return client.Answer(ctx, op.id, op.allow, op.input, op.text)
	case "interrupt":
		return client.Interrupt(ctx)
	case "stop-task":
		if len(op.ids) > 0 {
			for _, id := range op.ids {
				if err := client.StopTask(ctx, id); err != nil {
					return err
				}
			}
			return nil
		}
		return client.StopTask(ctx, op.id)
	default:
		return ErrRequest
	}
}

func (w *runWorker) submit(op operation) error {
	select {
	case <-w.ctx.Done():
		return ErrClosed
	case w.operations <- op:
		return nil
	default:
		return ErrBackpressure
	}
}

// stop bypasses the I/O queue. An unresponsive write or initialize must never
// prevent the user from terminating the process and its descendants.
func (w *runWorker) stop() <-chan error {
	w.cancel()
	result := make(chan error, 1)
	go func() {
		var interruptErr, stopErr error
		client := w.client.Load()
		if client != nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			interruptErr = client.Interrupt(ctx)
			cancel()
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			interruptErr = errors.Join(interruptErr, w.stopOwnedTasks(ctx, client))
			cancel()
			stopErr = client.Stop()
		}
		<-w.done
		if late := w.client.Load(); late != nil && late != client {
			stopErr = errors.Join(stopErr, late.Stop())
		}
		result <- errors.Join(interruptErr, stopErr)
	}()
	return result
}

func (w *runWorker) stopOwnedTasks(ctx context.Context, client *Client) error {
	requested := map[stopTaskRef]bool{}
	var errs []error
	for {
		reply := make(chan []stopTaskRef, 1)
		select {
		case w.stopQueries <- reply:
		case <-ctx.Done():
			return errors.Join(append(errs, ctx.Err())...)
		}
		var tasks []stopTaskRef
		select {
		case tasks = <-reply:
		case <-ctx.Done():
			return errors.Join(append(errs, ctx.Err())...)
		}
		if len(tasks) == 0 {
			return errors.Join(errs...)
		}
		for _, task := range tasks {
			if !requested[task] {
				requested[task] = true
				if err := client.StopTask(ctx, task.id); err != nil {
					errs = append(errs, err)
				}
			}
		}
		select {
		case <-ctx.Done():
			return errors.Join(append(errs, ctx.Err())...)
		case <-time.After(10 * time.Millisecond):
		}
	}
}
