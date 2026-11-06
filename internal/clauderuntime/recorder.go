package clauderuntime

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/pkg/api"
)

// recorder keeps disk writes off the conversation controller. Its bounded queue
// and error channel let the controller stop a run if progress cannot be saved.
type recorder struct {
	w      *convstore.Writer
	queue  chan convstore.RuntimeEvent
	failed chan error
	done   chan struct{}
	err    error // read after done closes
}

func newRecorder(w *convstore.Writer) *recorder {
	r := &recorder{w: w, queue: make(chan convstore.RuntimeEvent, 256), failed: make(chan error, 1), done: make(chan struct{})}
	go r.run()
	return r
}

func (r *recorder) append(event api.ChatEvent, session string) error {
	display, err := json.Marshal(event)
	if err != nil {
		return err
	}
	record := convstore.RuntimeEvent{At: time.Now(), Session: session, Display: display}
	if event.Kind == api.ChatEventTurnStarted {
		record.User = event.Text
	}
	if event.Kind == api.ChatEventTitle && event.Error == nil {
		record.Title = event.Text
	}
	if event.Kind == api.ChatEventModel && event.Error == nil {
		record.Model = event.Model
	}
	select {
	case <-r.done:
		return errors.Join(ErrClosed, r.err)
	case r.queue <- record:
		return nil
	default:
		return ErrBackpressure
	}
}

func (r *recorder) run() {
	defer close(r.done)
	for event := range r.queue {
		batch := []convstore.RuntimeEvent{event}
	drain:
		for len(batch) < 32 {
			select {
			case event, ok := <-r.queue:
				if !ok {
					break drain
				}
				batch = append(batch, event)
			default:
				break drain
			}
		}
		if err := r.w.AppendRuntime(batch); err != nil {
			r.err = err
			r.failed <- err
			return
		}
	}
}

func (r *recorder) close() error {
	close(r.queue)
	<-r.done
	return errors.Join(r.err, r.w.Close())
}
