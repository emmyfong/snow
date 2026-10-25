package chat

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

var start = time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)

// clock is a test clock that the test moves.
type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newChat(w, h int) (*Model, *clock) {
	c := &clock{t: start}
	m := New(&api.ChatSnapshot{Pane: 2, Agent: "c1", Conversation: "c1", Model: "anthropic/claude-sonnet-5-5", Mode: "ask", State: api.ChatStateIdle}, theme.Default(), c.now)
	m.SetSize(w, h)
	return m, c
}

func ev(e api.ChatEvent) *api.ChatEvent { e.Pane, e.Agent = 2, "c1"; return &e }

func key(code rune, mod tea.KeyMod, text string) tea.Key {
	return tea.Key{Code: code, Mod: mod, Text: text}
}

func typeText(m *Model, s string) {
	for _, r := range s {
		m.Key(key(r, 0, string(r)))
	}
}

func screen(m *Model) string { return ansi.Strip(strings.Join(m.View(), "\n")) }

// oneTurn plays a whole turn into m.
func oneTurn(m *Model, n int, user, reply string) {
	m.Event(ev(api.ChatEvent{Kind: api.EventTurnStarted, Turn: n, Text: user}))
	m.Event(ev(api.ChatEvent{Kind: api.EventText, Text: reply}))
	m.Event(ev(api.ChatEvent{Kind: api.EventTurnEnded, Turn: n, Reason: api.EndDone}))
}

// TestChatStreamingAppends: deltas fed as messages append in order, and
// the transcript follows them.
func TestChatStreamingAppends(t *testing.T) {
	m, _ := newChat(60, 12)
	m.Event(ev(api.ChatEvent{Kind: api.EventTurnStarted, Turn: 1, Text: "hi"}))
	for _, d := range []string{"Hel", "lo, ", "wor", "ld."} {
		m.Event(ev(api.ChatEvent{Kind: api.EventText, Text: d}))
	}
	if got := m.Chat().Entries[1].Text; got != "Hello, world." {
		t.Fatalf("reply %q", got)
	}
	if s := screen(m); !strings.Contains(s, "Hello, world.") || !strings.Contains(s, "working") {
		t.Fatalf("screen:\n%s", s)
	}
}

// TestChatSendsMessage: Enter sends what was typed and clears the input;
// a blank message is not sent.
func TestChatSendsMessage(t *testing.T) {
	m, _ := newChat(60, 12)
	if in := m.Key(key(tea.KeyEnter, 0, "")); in != nil {
		t.Fatalf("blank message sent: %+v", in)
	}
	typeText(m, "fix it")
	in := m.Key(key(tea.KeyEnter, 0, ""))
	if in == nil || in.Kind != api.InputMessage || in.Text != "fix it" || in.Pane != 2 || in.Agent != "c1" || in.Validate() != nil {
		t.Fatalf("sent %+v", in)
	}
	if len(m.input) != 0 {
		t.Fatalf("input kept %q", string(m.input))
	}
}

// TestChatShiftEnterNewLine: Shift+Enter, Alt+Enter, and Ctrl+J add a line
// instead of sending.
func TestChatShiftEnterNewLine(t *testing.T) {
	for _, k := range []tea.Key{key(tea.KeyEnter, tea.ModShift, ""), key(tea.KeyEnter, tea.ModAlt, ""), key('j', tea.ModCtrl, "")} {
		m, _ := newChat(60, 12)
		typeText(m, "a")
		if in := m.Key(k); in != nil {
			t.Fatalf("%v sent %+v", k, in)
		}
		typeText(m, "b")
		if in := m.Key(key(tea.KeyEnter, 0, "")); in == nil || in.Text != "a\nb" {
			t.Fatalf("%v: sent %+v", k, in)
		}
	}
}

// TestChatEscSendsInterrupt: Esc during a turn interrupts it; Esc while
// idle sends nothing.
func TestChatEscSendsInterrupt(t *testing.T) {
	m, _ := newChat(60, 12)
	if in := m.Key(key(tea.KeyEscape, 0, "")); in != nil {
		t.Fatalf("idle Esc sent %+v", in)
	}
	m.Event(ev(api.ChatEvent{Kind: api.EventTurnStarted, Turn: 1, Text: "hi"}))
	in := m.Key(key(tea.KeyEscape, 0, ""))
	if in == nil || in.Kind != api.InputInterrupt || in.Validate() != nil {
		t.Fatalf("sent %+v", in)
	}
}

// TestChatEditing: the cursor moves, and Backspace and Delete remove
// around it.
func TestChatEditing(t *testing.T) {
	m, _ := newChat(60, 12)
	typeText(m, "helo")
	m.Key(key(tea.KeyLeft, 0, ""))
	typeText(m, "l")
	m.Key(key(tea.KeyHome, 0, ""))
	m.Key(key(tea.KeyDelete, 0, ""))
	m.Key(key(tea.KeyEnd, 0, ""))
	m.Key(key(tea.KeyBackspace, 0, ""))
	m.Paste("p\r\nq")
	if got := string(m.input); got != "ellp\nq" {
		t.Fatalf("input %q", got)
	}
}

// TestChatScroll: PgUp leaves the latest text, new text does not pull the
// view back, and typing returns to it.
func TestChatScroll(t *testing.T) {
	m, _ := newChat(40, 10)
	for i := 1; i <= 8; i++ {
		oneTurn(m, i, fmt.Sprintf("question %d", i), fmt.Sprintf("answer %d", i))
	}
	if !strings.Contains(screen(m), "answer 8") {
		t.Fatalf("not following:\n%s", screen(m))
	}
	m.Key(key(tea.KeyPgUp, 0, ""))
	s := screen(m)
	if strings.Contains(s, "answer 8") || !strings.Contains(s, "newer text below") {
		t.Fatalf("PgUp:\n%s", s)
	}
	oneTurn(m, 9, "question 9", "answer 9")
	if strings.Contains(screen(m), "answer 9") {
		t.Fatal("new text pulled a scrolled view down")
	}
	for range 100 {
		m.Scroll(1)
	}
	if !strings.Contains(screen(m), "question 1") {
		t.Fatalf("top:\n%s", screen(m))
	}
	typeText(m, "x")
	if !strings.Contains(screen(m), "answer 9") {
		t.Fatalf("typing did not return to the latest text:\n%s", screen(m))
	}
}

// TestChatResize: wide characters and a resize keep every row inside the
// pane, and the view is exactly the pane's height.
func TestChatResize(t *testing.T) {
	m, _ := newChat(80, 20)
	oneTurn(m, 1, "名前を直して", strings.Repeat("名前名前 ", 30))
	typeText(m, strings.Repeat("長い入力 ", 20))
	for _, size := range [][2]int{{80, 20}, {33, 9}, {7, 4}, {1, 1}, {120, 40}} {
		m.SetSize(size[0], size[1])
		rows := m.View()
		if len(rows) != size[1] {
			t.Fatalf("%dx%d: %d rows", size[0], size[1], len(rows))
		}
		for i, r := range rows {
			if w := ansi.StringWidth(r); w > size[0] {
				t.Fatalf("%dx%d: row %d is %d wide: %q", size[0], size[1], i, w, ansi.Strip(r))
			}
		}
		if x, y, ok := m.Cursor(); !ok || x >= size[0] || y >= size[1] || y < 0 {
			t.Fatalf("%dx%d: cursor %d,%d", size[0], size[1], x, y)
		}
	}
	m.SetSize(0, 0)
	if m.View() != nil {
		t.Fatal("a zero-size pane should draw nothing")
	}
}

// TestChatSnapshotKeepsInput: a fresh snapshot replaces the transcript but
// not what the user is typing.
func TestChatSnapshotKeepsInput(t *testing.T) {
	m, _ := newChat(60, 12)
	typeText(m, "draft")
	m.Snapshot(&api.ChatSnapshot{Pane: 2, Agent: "c1", Entries: []api.ChatEntry{{Kind: api.EntryUser, Text: "old"}}})
	if string(m.input) != "draft" || len(m.Chat().Entries) != 1 {
		t.Fatalf("input %q, entries %+v", string(m.input), m.Chat().Entries)
	}
}

// goldenStates are the chat states the goldens show.
var goldenStates = map[string]func(m *Model, c *clock){
	"Empty": func(*Model, *clock) {},
	"OneTurn": func(m *Model, _ *clock) {
		oneTurn(m, 1, "What does the rename command do?", "It renames the session on screen. With a name it renames at once; without one it asks for the new name in the status bar.")
	},
	"Streaming": func(m *Model, c *clock) {
		oneTurn(m, 1, "What does the rename command do?", "It renames the session on screen.")
		m.Event(ev(api.ChatEvent{Kind: api.EventTurnStarted, Turn: 2, Text: "And kill?"}))
		m.Event(ev(api.ChatEvent{Kind: api.EventState, State: api.ChatStateStreaming}))
		m.Event(ev(api.ChatEvent{Kind: api.EventText, Text: "Kill ends a session and every program in it. Snow asks"}))
		c.t = c.t.Add(12 * time.Second)
		typeText(m, "and the tests")
	},
	"Queued": func(m *Model, c *clock) {
		m.Event(ev(api.ChatEvent{Kind: api.EventTurnStarted, Turn: 1, Text: "Fix the failing test in rename_test.go"}))
		m.Event(ev(api.ChatEvent{Kind: api.EventText, Text: "Looking at rename_test.go."}))
		m.Event(ev(api.ChatEvent{Kind: api.EventQueue, Queue: []string{"then run go vet"}}))
		c.t = c.t.Add(75 * time.Second)
	},
	"Scrolled": func(m *Model, _ *clock) {
		for i := 1; i <= 12; i++ {
			oneTurn(m, i, fmt.Sprintf("Question number %d?", i), fmt.Sprintf("Answer number %d, in a sentence long enough to wrap in a narrow pane.", i))
		}
		m.Key(key(tea.KeyPgUp, 0, ""))
	},
}

// TestChatGolden draws each state at 80x24 and 120x40.
func TestChatGolden(t *testing.T) {
	for name, state := range goldenStates {
		for _, size := range [][2]int{{80, 24}, {120, 40}} {
			t.Run(fmt.Sprintf("%s%dx%d", name, size[0], size[1]), func(t *testing.T) {
				m, c := newChat(size[0], size[1])
				state(m, c)
				golden.RequireEqual(t, strings.Join(m.View(), "\n"))
			})
		}
	}
}
