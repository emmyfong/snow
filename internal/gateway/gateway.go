// Package gateway starts a local service that a [provider.<name>] table
// names, such as OmniRoute, when its address does not answer.
//
// Snow starts only a tree it started, and stops that tree when the server
// exits. A gateway the user started by hand is used as it is and left
// running. The package never talks to a remote host: a gateway with start
// must answer on this machine.
package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/emmyfong/snow/internal/private"
	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/pkg/api"
)

// State is what Snow knows about a gateway.
type State int

// The states of a gateway.
const (
	// StateReady means the address answers. Snow started nothing when it
	// already answered.
	StateReady State = iota
	// StateStarting means Snow started the program and waits for the
	// address.
	StateStarting
	// StateFailed means the program exited, or the address did not answer
	// within the wait. The log says why.
	StateFailed
)

// stopGrace is how long a gateway gets to exit after SIGTERM before Snow
// kills its tree.
const stopGrace = 3 * time.Second

// Spec is a [provider.<name>] table with start set: a local service Snow
// may start when its address answers nothing.
type Spec struct {
	Name    string   // the provider's name, and the log file's name
	BaseURL string   // the API's address, such as http://localhost:20128/v1
	Start   []string // the program and its arguments
}

// Status is a gateway's state and, for a gateway Snow started, its log.
type Status struct {
	Name    string
	State   State
	LogPath string // where the program's output goes; "" before a start
	Err     error  // why it failed; nil otherwise
}

// Word is the short state the model picker shows: "ready", "starting", or
// "failed".
func (s Status) Word() string {
	switch s.State {
	case StateReady:
		return api.GatewayReady
	case StateStarting:
		return api.GatewayStarting
	}
	return api.GatewayFailed
}

// Detail is what the user reads in the model picker: "starting", or
// "failed, see <log>".
func (s Status) Detail() string {
	if s.State != StateFailed {
		return s.Word()
	}
	if s.LogPath != "" {
		return "failed, see " + s.LogPath
	}
	if s.Err != nil {
		return "failed: " + s.Err.Error()
	}
	return "failed"
}

// Options tunes a Manager. The zero value works.
type Options struct {
	// Wait is how long a started gateway gets to answer. 0 means
	// DefaultWait.
	Wait time.Duration
	// Poll is how often Snow asks a starting gateway. 0 means
	// DefaultPoll.
	Poll time.Duration
	// LogDir holds <name>.log. Empty means ~/.snow/logs.
	LogDir string
	// Log receives a line when a gateway fails. Nil discards it.
	Log *slog.Logger
	// Probe asks whether an address answers. Nil means Probe.
	Probe func(ctx context.Context, baseURL string) error
}

// DefaultWait is how long a started gateway gets to answer.
const DefaultWait = 120 * time.Second

// DefaultPoll is how often Snow asks a starting gateway.
const DefaultPoll = 250 * time.Millisecond

// Manager starts the configured gateways and reports their state. Its
// methods are safe to use from several goroutines.
type Manager struct {
	opts    Options
	entries []*entry

	mu       sync.Mutex
	idle     *sync.Cond // wakes a Stop when the last start goroutine returns
	running  int        // start goroutines that have not returned
	started  bool
	stopping bool
}

// entry is one gateway. Its fields other than spec change only under the
// manager's lock.
type entry struct {
	spec    Spec
	state   State
	logPath string
	err     error
	proc    *shell.Process // the tree Snow started, or nil
}

// runningGateway is a tree Snow started, on its way to Stop.
type runningGateway struct {
	name string
	proc *shell.Process
}

// New returns a manager for specs, which Start starts. A spec without a
// name or a program is ignored, as the config rejects it first.
func New(specs []Spec, opts Options) *Manager {
	if opts.Wait <= 0 {
		opts.Wait = DefaultWait
	}
	if opts.Poll <= 0 {
		opts.Poll = DefaultPoll
	}
	m := &Manager{opts: opts}
	m.idle = sync.NewCond(&m.mu)
	for _, s := range specs {
		if s.Name == "" || strings.ContainsAny(s.Name, `/\`) || len(s.Start) == 0 {
			continue
		}
		s.Start = slices.Clone(s.Start)
		m.entries = append(m.entries, &entry{spec: s, state: StateStarting})
	}
	slices.SortFunc(m.entries, func(a, b *entry) int { return strings.Compare(a.spec.Name, b.spec.Name) })
	return m
}

// Start asks each gateway whether it answers, and starts the ones that do
// not. It returns at once; each gateway is ready, starting, or failed
// later, as Statuses reports. A Start after Stop does nothing.
func (m *Manager) Start() {
	m.mu.Lock()
	if m.started || m.stopping {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.running = len(m.entries)
	m.mu.Unlock()
	for _, e := range m.entries {
		go func(e *entry) {
			defer m.finished()
			m.startOne(e)
		}(e)
	}
}

// finished records that one start goroutine returned, and wakes a waiting
// Stop when it was the last.
func (m *Manager) finished() {
	m.mu.Lock()
	m.running--
	if m.running == 0 {
		m.idle.Broadcast()
	}
	m.mu.Unlock()
}

// waitForStarts waits until every start goroutine returned.
func (m *Manager) waitForStarts() {
	m.mu.Lock()
	for m.running > 0 {
		m.idle.Wait()
	}
	m.mu.Unlock()
}

// Statuses returns every gateway's state, in name order.
func (m *Manager) Statuses() []Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Status, len(m.entries))
	for i, e := range m.entries {
		out[i] = Status{Name: e.spec.Name, State: e.state, LogPath: e.logPath, Err: e.err}
	}
	return out
}

// Stop stops every gateway Snow started and waits for it. A gateway that
// already answered when Snow looked is not touched. A second call returns
// at once.
func (m *Manager) Stop() {
	m.mu.Lock()
	if m.stopping {
		m.mu.Unlock()
		return
	}
	m.stopping = true
	var running []runningGateway
	for _, e := range m.entries {
		if e.proc != nil {
			running = append(running, runningGateway{name: e.spec.Name, proc: e.proc})
		}
	}
	m.mu.Unlock()
	var wg sync.WaitGroup
	for _, r := range running {
		wg.Add(1)
		go func(r runningGateway) {
			defer wg.Done()
			m.stopProc(r)
		}(r)
	}
	wg.Wait()
	m.waitForStarts()
}

// stopProc stops one gateway's tree and notes a failure in the log.
func (m *Manager) stopProc(r runningGateway) {
	if err := r.proc.Stop(stopGrace); err != nil {
		m.warn("stop a gateway", r.name, "", err)
	}
}

// isStopping reports whether Stop has begun.
func (m *Manager) isStopping() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopping
}

// startOne decides whether e answers already, or starts it.
func (m *Manager) startOne(e *entry) {
	ctx, cancel := context.WithTimeout(context.Background(), m.opts.Wait)
	defer cancel()
	if err := m.probe(ctx, e.spec.BaseURL); err == nil {
		m.set(e, StateReady, "", nil)
		return
	}
	logPath, logFile, err := m.openLog(e.spec.Name)
	if err != nil {
		m.set(e, StateFailed, "", err)
		return
	}
	defer func() { _ = logFile.Close() }()
	profile, err := shell.Program(e.spec.Name, e.spec.Start)
	if err != nil {
		m.set(e, StateFailed, logPath, err)
		return
	}
	proc, err := shell.StartBackground(shell.Launch{
		Path: profile.Path, Args: profile.Args, CmdLine: profile.CmdLine, Dir: homeDir(),
	}, logFile)
	if err != nil {
		m.set(e, StateFailed, logPath, err)
		return
	}
	m.mu.Lock()
	e.proc = proc
	stopping := m.stopping
	m.mu.Unlock()
	if stopping {
		m.stopProc(runningGateway{name: e.spec.Name, proc: proc})
		return
	}
	m.set(e, StateStarting, logPath, nil)

	tick := time.NewTicker(m.opts.Poll)
	defer tick.Stop()
	for {
		select {
		case <-proc.Done():
			// Stop ends this tree on purpose, so its exit is not a failure.
			if m.isStopping() {
				return
			}
			err := proc.Err()
			if err == nil {
				err = errors.New("the program exited before the gateway answered")
			}
			m.set(e, StateFailed, logPath, err)
			return
		case <-tick.C:
			ask, cancel := context.WithTimeout(ctx, probeTimeout)
			err := m.probe(ask, e.spec.BaseURL)
			cancel()
			if err == nil {
				m.set(e, StateReady, logPath, nil)
				return
			}
		case <-ctx.Done():
			m.set(e, StateFailed, logPath, errors.New("the address did not answer in time; the program keeps running"))
			return
		}
	}
}

// probe asks whether baseURL answers.
func (m *Manager) probe(ctx context.Context, baseURL string) error {
	if m.opts.Probe != nil {
		return m.opts.Probe(ctx, baseURL)
	}
	return Probe(ctx, baseURL)
}

// set records e's state. A failure goes to the log once.
func (m *Manager) set(e *entry, state State, logPath string, err error) {
	m.mu.Lock()
	e.state, e.logPath, e.err = state, logPath, err
	m.mu.Unlock()
	if state == StateFailed {
		m.warn("gateway failed", e.spec.Name, logPath, err)
	}
}

// warn writes a line about gateway name when a logger is set.
func (m *Manager) warn(msg, name, logPath string, err error) {
	if m.opts.Log == nil {
		return
	}
	if logPath == "" {
		m.opts.Log.Warn(msg, "gateway", name, "err", err)
		return
	}
	m.opts.Log.Warn(msg, "gateway", name, "log", logPath, "err", err)
}

// openLog opens the gateway's log file in the log folder, private to the
// user. The file appends, so an earlier run's reason stays until the user
// removes the file.
func (m *Manager) openLog(name string) (string, *os.File, error) {
	dir := m.opts.LogDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", nil, fmt.Errorf("find home folder for the gateway log: %w", err)
		}
		dir = filepath.Join(home, ".snow", "logs")
	}
	if err := private.Dir(dir); err != nil {
		return "", nil, fmt.Errorf("open the gateway log folder: %w", err)
	}
	path := filepath.Join(dir, name+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", nil, fmt.Errorf("open the gateway log: %w", err)
	}
	return path, f, nil
}

// homeDir is the folder a gateway starts in: the user's home, so a program
// finds its settings whatever folder the server started in.
func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}
