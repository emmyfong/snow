// Package chat is a chat pane on the client: it keeps the pane's
// transcript from the server's ChatSnapshot and ChatEvent messages, edits
// the message being typed, scrolls, and draws the pane. It does not import
// the session screen. The session gives it the pane's size, its keys, and
// the server's messages, and sends the ChatInput it returns.
package chat

import (
	"time"

	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// Model is one chat pane. Like every Bubble Tea model, it is used from one
// goroutine.
type Model struct {
	th            theme.Theme
	now           func() time.Time
	snap          api.ChatSnapshot
	input         []rune // the message being typed
	cursor        int    // where typing goes, in runes
	scroll        int    // transcript rows above the latest; 0 follows new text
	width, height int
	started       time.Time // when the running turn started, on this client's clock
	frame         int       // the spinner's frame
	selected      string    // the tool call whose cell has the keyboard, or ""
	expanded      map[string]bool
	box           approval
	drawn         rendered
	usage         bool   // the top bar shows the context gauge and the cost
	errAt         int    // the entry of the failure whose choices take keys
	errChoice     int    // its highlighted choice
	asked         string // a request for the session; see Asked
	loaded        bool   // the first snapshot came
	note          string // a note for the status bar; see Note
	diffs         diffCache
}

// New returns the chat pane for snap. now is the clock for the elapsed
// time of a turn; nil means time.Now.
func New(snap *api.ChatSnapshot, th theme.Theme, now func() time.Time) *Model {
	if now == nil {
		now = time.Now
	}
	m := &Model{th: th, now: now, expanded: map[string]bool{}, usage: true, errAt: -1}
	m.Snapshot(snap)
	return m
}

// Waiting returns the chat pane id before its first snapshot comes: it
// draws no transcript and sends nothing until Snapshot.
func Waiting(id int, th theme.Theme, now func() time.Time) *Model {
	m := New(&api.ChatSnapshot{Pane: id}, th, now)
	m.loaded = false
	return m
}

// SetTheme sets the colors.
func (m *Model) SetTheme(th theme.Theme) { m.th = th; m.invalidate() }

// SetUsage shows or hides the context gauge and the cost in the top bar,
// from the [chat] usage setting. The model always shows.
func (m *Model) SetUsage(show bool) { m.usage = show }

// SetSize sets the pane's size in cells.
func (m *Model) SetSize(width, height int) { m.width, m.height = max(width, 0), max(height, 0) }

// Snapshot replaces the transcript with snap, as the server sends it on
// attach or after this client fell behind. What the user is typing stays.
func (m *Model) Snapshot(snap *api.ChatSnapshot) {
	m.keepPlace(func() {
		m.snap = *snap
		m.loaded = true
	})
	if snap.Running && m.started.IsZero() {
		m.started = m.now()
	}
	if !snap.Running {
		m.started = time.Time{}
	}
}

// Event applies one change from the server.
func (m *Model) Event(e *api.ChatEvent) {
	m.keepPlace(func() { m.snap.Apply(e) })
	switch e.Kind {
	case api.EventTurnStarted:
		m.started = m.now()
	case api.EventTurnEnded:
		m.started = time.Time{}
	}
}

// keepPlace runs change, which changes the transcript. While the user is
// scrolled up, the rows it adds or removes move the scroll offset too, so
// the rows the user reads stay where they are.
func (m *Model) keepPlace(change func()) {
	if m.scroll == 0 {
		change()
		m.invalidate()
		return
	}
	before := len(m.transcriptRows())
	change()
	m.invalidate()
	m.scroll += len(m.transcriptRows()) - before
	m.Scroll(0)
}

// Chat returns the transcript as this pane knows it.
func (m *Model) Chat() *api.ChatSnapshot { return &m.snap }

// Running reports whether a turn runs, so the session keeps the spinner
// turning.
func (m *Model) Running() bool { return m.snap.Running }

// Tick moves the spinner on by one frame.
func (m *Model) Tick() { m.frame++ }
