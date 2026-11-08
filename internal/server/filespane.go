package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/emmyfong/snow/internal/files"
	"github.com/emmyfong/snow/pkg/api"
)

// ErrFilesBusy is returned for a files pane action while earlier ones still
// wait for the disk.
var ErrFilesBusy = errors.New("the files pane is still busy with earlier actions")

// filesJobs bounds the actions a files pane holds while it reads.
const filesJobs = 8

// filesPoll is how often the pane reads the expanded folders again, so a
// change an agent makes shows without a key.
var filesPoll = 3 * time.Second

// filesStateBudget is the most a files pane's state may take as JSON, so it
// fits in one frame with room for the envelope.
const filesStateBudget = api.MaxFrameSize * 3 / 4

// filesKind is the files pane: a lazy tree of the folder, with git's marks
// on the files it changed. Reads run on the pane's own goroutine.
func filesKind() sideKind {
	return sideKind{Name: api.PaneFiles, Title: "Files", Open: openFiles}
}

// filesPane is the model goroutine's half of a files pane. Its worker reads
// folders and hands each new state over through Apply, so state needs no
// lock.
type filesPane struct {
	state   api.FilesState // owned by the model goroutine
	pending int            // jobs taken and not yet answered
	jobs    chan filesJob
	cancel  context.CancelFunc
	done    chan struct{}
}

// filesJob is one action for the worker.
type filesJob struct {
	action string
	args   map[string]string
}

func openFiles(ctx sideContext) (sidePane, error) {
	runCtx, cancel := context.WithCancel(context.Background())
	p := &filesPane{
		state:  api.FilesState{Folder: ctx.Folder, Loading: true},
		jobs:   make(chan filesJob, filesJobs),
		cancel: cancel,
		done:   make(chan struct{}),
	}
	w := &filesWorker{
		root:     ctx.Folder,
		osRoot:   files.OSPath(ctx.Folder, ctx.Distro),
		distro:   ctx.Distro,
		expanded: map[string]bool{"": true},
		cur:      api.FilesState{Folder: ctx.Folder},
		apply:    func(st api.FilesState, done bool) { ctx.Apply(func() { p.take(st, done) }) },
	}
	go func() {
		defer close(p.done)
		w.run(runCtx, p.jobs)
	}()
	return p, nil
}

// take makes st the pane's state, on the model goroutine. done is set when
// st answers a job. The pane shows loading while any job waits.
func (p *filesPane) take(st api.FilesState, done bool) {
	if done && p.pending > 0 {
		p.pending--
	}
	st.Loading = p.pending > 0
	p.state = st
}

// State implements sidePane.
func (p *filesPane) State() api.SideState {
	st := p.state
	return &st
}

// Act implements sidePane. It checks the action and hands it to the worker;
// the outcome comes back as a new state.
func (p *filesPane) Act(action string, args map[string]string) error {
	if err := checkFilesAction(action, args); err != nil {
		return err
	}
	select {
	case p.jobs <- filesJob{action: action, args: args}:
	default:
		return ErrFilesBusy
	}
	p.pending++
	p.state.Loading = true
	return nil
}

// Close implements sidePane. It stops the reads and waits for the worker.
func (p *filesPane) Close() error {
	p.cancel()
	<-p.done
	return nil
}

// checkFilesAction checks action and its arguments before the disk is read.
func checkFilesAction(action string, args map[string]string) error {
	switch action {
	case api.FilesActRefresh:
		return nil
	case api.FilesActExpand, api.FilesActCollapse, api.FilesActSelect:
		p := args[api.ArgPath]
		if !validTreePath(p) {
			return fmt.Errorf("files %s: %q is not a path below the folder", action, p)
		}
		return nil
	}
	return fmt.Errorf("the files pane has no action %q", action)
}

// validTreePath reports whether p is a clean relative path with at least
// one part.
func validTreePath(p string) bool {
	if p == "" || path.IsAbs(p) || strings.Contains(p, `\`) {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

// filesWorker reads a files pane's folders on its own goroutine. cur is the
// state it last built; only the worker touches it.
type filesWorker struct {
	root     string // the folder in its own form, as the pane names it
	osRoot   string // the same folder in the form the OS reads
	distro   string // the WSL distribution of root, if any
	expanded map[string]bool
	selected string
	cur      api.FilesState
	apply    func(api.FilesState, bool)
}

// run reads the root, then does each job, and reads the expanded folders
// again on each tick, until ctx ends.
func (w *filesWorker) run(ctx context.Context, jobs <-chan filesJob) {
	w.publish(w.read(ctx), true, true)
	tick := time.NewTicker(filesPoll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-jobs:
			w.do(ctx, j)
		case <-tick.C:
			next := w.read(ctx)
			// A read that failed shows its own error; otherwise the last
			// action's outcome stays until the next action.
			if next.Error == "" {
				next.Error = w.cur.Error
			}
			w.publish(next, false, false)
		}
	}
}

// do runs one job, then reads the expanded folders and hands the state
// over, with the job's outcome in Error.
func (w *filesWorker) do(ctx context.Context, j filesJob) {
	err := w.action(j)
	if ctx.Err() != nil {
		return
	}
	next := w.read(ctx)
	if err != nil {
		next.Error = err.Error()
	}
	w.publish(next, true, true)
}

// action applies j to the tree's expanded set and selection.
func (w *filesWorker) action(j filesJob) error {
	p := j.args[api.ArgPath]
	switch j.action {
	case api.FilesActExpand:
		w.expanded[p] = true
	case api.FilesActCollapse:
		if p == "" {
			return errors.New("the root folder stays open")
		}
		for q := range w.expanded {
			if q == p || strings.HasPrefix(q, p+"/") {
				delete(w.expanded, q)
			}
		}
		if w.selected == p || strings.HasPrefix(w.selected, p+"/") {
			w.selected = ""
		}
	case api.FilesActSelect:
		for q := files.Parent(p); ; q = files.Parent(q) {
			w.expanded[q] = true
			if q == "" {
				break
			}
		}
		w.selected = p
	}
	return nil
}

// read builds a state from the expanded folders and git's marks. A folder
// that cannot be read keeps the rest of the tree, and its failure is the
// state's Error.
func (w *filesWorker) read(ctx context.Context) api.FilesState {
	next := api.FilesState{Folder: w.root, Selected: w.selected}
	for p := range w.expanded {
		next.Expanded = append(next.Expanded, p)
	}
	slices.Sort(next.Expanded)
	if len(w.expanded) == 0 {
		w.expanded[""] = true
		next.Expanded = []string{""}
	}
	marks, err := files.GitMarks(ctx, w.root, w.distro)
	if err != nil {
		next.Error = err.Error()
	}
	for _, folder := range next.Expanded {
		if len(next.Folders) == api.MaxFilesFolders {
			next.Omitted++
			continue
		}
		entries, omitted, err := files.List(ctx, w.osRoot, folder)
		if err != nil {
			if next.Error == "" {
				next.Error = err.Error()
			}
			continue
		}
		next.Omitted += omitted
		f := api.FilesFolder{Path: folder}
		for _, e := range entries {
			fe := api.FilesEntry{Name: e.Name, Path: e.Path, Dir: e.Dir, Mark: marks[e.Path]}
			f.Entries = append(f.Entries, fe)
		}
		f.Entries = w.addGone(f.Entries, folder, marks)
		next.Folders = append(next.Folders, f)
	}
	return next
}

// addGone adds the files git still lists as changed but the folder no
// longer has, such as a deletion, so they show with their mark.
func (w *filesWorker) addGone(entries []api.FilesEntry, folder string, marks map[string]string) []api.FilesEntry {
	have := make(map[string]bool, len(entries))
	for _, e := range entries {
		have[e.Path] = true
	}
	for p, m := range marks {
		if have[p] || files.Parent(p) != folder {
			continue
		}
		entries = append(entries, api.FilesEntry{Name: path.Base(p), Path: p, Mark: m, Deleted: m == "D"})
	}
	slices.SortStableFunc(entries, func(a, b api.FilesEntry) int {
		if a.Dir != b.Dir {
			if a.Dir {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return entries
}

// publish hands next over when it differs from the last state, or always
// when force is set: an action's answer clears the pane's Loading mark.
func (w *filesWorker) publish(next api.FilesState, force, done bool) {
	fitFilesState(&next)
	if !force && reflect.DeepEqual(next, w.cur) {
		return
	}
	w.cur = next
	w.apply(next, done)
}

// fitFilesState cuts st until its JSON fits filesStateBudget: entries from
// the last folders first, so the top of the tree survives.
func fitFilesState(st *api.FilesState) {
	for sizeFiles(st) > filesStateBudget {
		if len(st.Folders) == 0 {
			return
		}
		last := &st.Folders[len(st.Folders)-1]
		if len(last.Entries) > 0 {
			half := len(last.Entries) / 2
			st.Omitted += len(last.Entries) - half
			last.Entries = last.Entries[:half]
			continue
		}
		st.Folders = st.Folders[:len(st.Folders)-1]
		st.Omitted++
	}
}

func sizeFiles(st *api.FilesState) int {
	b, err := json.Marshal(st)
	if err != nil {
		return 0
	}
	return len(b)
}
