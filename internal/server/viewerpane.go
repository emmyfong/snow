package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"time"

	"github.com/emmyfong/snow/internal/files"
	"github.com/emmyfong/snow/internal/viewer"
	"github.com/emmyfong/snow/pkg/api"
)

// ErrViewerBusy is returned for a viewer action while an earlier one still
// waits for the disk.
var ErrViewerBusy = errors.New("the viewer is still busy with an earlier action")

// viewerJobs bounds the actions a viewer pane holds while it reads.
const viewerJobs = 8

// viewerPoll is how often the viewer reads the file again, so an agent's
// change shows without a key.
var viewerPoll = 2 * time.Second

// viewerStateBudget is the most a viewer's state may take as JSON, so it
// fits in one frame with room for the envelope.
const viewerStateBudget = api.MaxFrameSize * 3 / 4

// viewerKind is the read-only viewer: a bounded, highlighted file with the
// lines a recorded change still explains marked.
func viewerKind() sideKind {
	return sideKind{Name: api.PaneViewer, Title: "Viewer", Open: openViewer}
}

// viewerPane is the model goroutine's half of a viewer pane. Its worker
// reads the file and hands each new state over through Apply.
type viewerPane struct {
	state   api.ViewerState
	pending int
	jobs    chan viewerJob
	cancel  context.CancelFunc
	done    chan struct{}
}

// viewerJob is one file to read and the evidence that explains its marks.
type viewerJob struct {
	path     string
	line     int
	evidence []viewer.Evidence
	refresh  bool
}

func openViewer(ctx sideContext) (sidePane, error) {
	runCtx, cancel := context.WithCancel(context.Background())
	p := &viewerPane{
		state:  api.ViewerState{Folder: ctx.Folder, Loading: true},
		jobs:   make(chan viewerJob, viewerJobs),
		cancel: cancel,
		done:   make(chan struct{}),
	}
	w := &viewerWorker{
		folder:   ctx.Folder,
		osFolder: files.OSPath(ctx.Folder, ctx.Distro),
		distro:   ctx.Distro,
		apply:    func(st api.ViewerState, done bool) { ctx.Apply(func() { p.take(st, done) }) },
	}
	go func() {
		defer close(p.done)
		w.run(runCtx, p.jobs)
	}()
	return p, nil
}

// take makes st the pane's state, on the model goroutine.
func (p *viewerPane) take(st api.ViewerState, done bool) {
	if done && p.pending > 0 {
		p.pending--
	}
	st.Loading = p.pending > 0
	p.state = st
}

// State implements sidePane.
func (p *viewerPane) State() api.SideState {
	st := p.state
	return &st
}

// Act implements sidePane. A client may only ask for a fresh read; opening
// a file comes from the server's OpOpenFile.
func (p *viewerPane) Act(action string, _ map[string]string) error {
	if action != api.ViewerActRefresh {
		return fmt.Errorf("the viewer has no action %q", action)
	}
	return p.refresh()
}

// refresh reads the current file again.
func (p *viewerPane) refresh() error {
	return p.start(viewerJob{refresh: true})
}

// open shows path, at line, with evidence, on the model goroutine.
func (p *viewerPane) open(path string, line int, evidence []viewer.Evidence) error {
	return p.start(viewerJob{path: path, line: line, evidence: evidence})
}

func (p *viewerPane) start(j viewerJob) error {
	select {
	case p.jobs <- j:
	default:
		return ErrViewerBusy
	}
	p.pending++
	p.state.Loading = true
	return nil
}

// Close implements sidePane.
func (p *viewerPane) Close() error {
	p.cancel()
	<-p.done
	return nil
}

// viewerWorker reads a viewer pane's file on its own goroutine.
type viewerWorker struct {
	folder   string
	osFolder string
	distro   string
	path     string
	line     int
	evidence []viewer.Evidence
	cur      api.ViewerState
	apply    func(api.ViewerState, bool)
}

// run reads the empty viewer, then each job, and reads the file again on
// each tick, until ctx ends.
func (w *viewerWorker) run(ctx context.Context, jobs <-chan viewerJob) {
	w.publish(w.read(), true, true)
	tick := time.NewTicker(viewerPoll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-jobs:
			if j.refresh {
				w.publish(w.read(), true, true)
				continue
			}
			w.path, w.line, w.evidence = j.path, j.line, j.evidence
			w.publish(w.read(), true, true)
		case <-tick.C:
			w.publish(w.read(), false, false)
		}
	}
}

// read builds a state from the file. A failure is the state's Error, so
// the last good text stays visible only until the file is read again.
func (w *viewerWorker) read() api.ViewerState {
	next := api.ViewerState{Folder: w.folder, Path: w.path, Reveal: w.line}
	if w.path == "" {
		return next
	}
	osPath := filepath.Join(w.osFolder, filepath.FromSlash(w.path))
	d, err := viewer.Read(osPath)
	if err != nil {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			next.Error = w.path + ": " + pathErr.Err.Error()
		} else {
			next.Error = err.Error()
		}
		return next
	}
	next.Binary, next.Truncated, next.Limit = d.Binary, d.Truncated, d.Limit
	if !d.Binary {
		lines, attribution, stale := viewer.Apply(d.Lines, w.evidence)
		next.Attribution, next.Stale = attribution, stale
		next.Lines = make([]api.ViewerLine, 0, len(lines))
		for _, l := range lines {
			next.Lines = append(next.Lines, api.ViewerLine{Number: l.Number, Text: l.Text, Mark: l.Mark})
		}
	}
	return next
}

// publish hands next over when it differs from the last state, or always
// when force is set.
func (w *viewerWorker) publish(next api.ViewerState, force, done bool) {
	fitViewerState(&next)
	if !force && reflect.DeepEqual(next, w.cur) {
		return
	}
	w.cur = next
	w.apply(next, done)
}

// fitViewerState cuts st until its JSON fits viewerStateBudget, from the
// end, so the start of the file survives.
func fitViewerState(st *api.ViewerState) {
	for sizeViewer(st) > viewerStateBudget && len(st.Lines) > 1 {
		keep := len(st.Lines) * 3 / 4
		if keep < 1 {
			keep = 1
		}
		st.Lines = st.Lines[:keep]
		st.Truncated = true
		if st.Limit == "" {
			st.Limit = "The viewer cuts this file to fit its pane."
		}
	}
}

func sizeViewer(st *api.ViewerState) int {
	b, err := json.Marshal(st)
	if err != nil {
		return 0
	}
	return len(b)
}
