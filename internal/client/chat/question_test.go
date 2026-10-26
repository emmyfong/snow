package chat

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

var storeQuestion = api.ChatQuestion{ID: "q1", Text: "Which store should the cache use?", Options: []api.ChatOption{
	{Label: "SQLite", Description: "one file, no server"},
	{Label: "Postgres", Description: "the team's server", Preview: "host: db.internal\nport: 5432"},
	{Label: "Memory", Description: "lost on restart"},
}}

// asking returns a chat whose turn waits for an answer to the store
// question.
func asking(t *testing.T, w, h int, multi bool) *Model {
	t.Helper()
	m, _ := newChat(w, h)
	m.Event(ev(api.ChatEvent{Kind: api.ChatEventTurnStarted, Turn: 1, Text: "Add a cache"}))
	m.Event(ev(api.ChatEvent{Kind: api.ChatEventToolCall, Tool: &api.ChatTool{ID: "q1", Name: "ask", Input: []byte(`{"question":"Which store should the cache use?"}`)}}))
	q := storeQuestion
	q.Multi = multi
	m.Event(ev(api.ChatEvent{Kind: api.ChatEventQuestion, Question: &q}))
	m.Event(ev(api.ChatEvent{Kind: api.ChatEventState, State: api.ChatStateApproval}))
	return m
}

var (
	down  = key(tea.KeyDown, 0, "")
	up    = key(tea.KeyUp, 0, "")
	space = key(tea.KeySpace, 0, " ")
	esc   = key(tea.KeyEscape, 0, "")
)

func typed(s string) []tea.Key {
	var ks []tea.Key
	for _, r := range s {
		ks = append(ks, letter(r))
	}
	return ks
}

func seq(groups ...[]tea.Key) []tea.Key {
	var out []tea.Key
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// TestQuestionKeys: each way to answer sends the right reply, and no key
// reaches the message being typed.
func TestQuestionKeys(t *testing.T) {
	tests := []struct {
		name   string
		multi  bool
		keys   []tea.Key
		picked []string
		text   string
	}{
		{"Enter on the first", false, []tea.Key{enter}, []string{"SQLite"}, ""},
		{"Down then Enter", false, []tea.Key{down, enter}, []string{"Postgres"}, ""},
		{"Up wraps to Other", false, seq([]tea.Key{up, enter}, typed("Redis"), []tea.Key{enter}), nil, "Redis"},
		{"a digit", false, []tea.Key{letter('3')}, []string{"Memory"}, ""},
		{"j and k", false, []tea.Key{letter('j'), letter('j'), letter('k'), enter}, []string{"Postgres"}, ""},
		{"other letters wait", false, []tea.Key{letter('x'), enter}, []string{"SQLite"}, ""},
		{"Esc leaves the field", false, seq([]tea.Key{up, enter}, typed("x"), []tea.Key{esc, down, enter}), []string{"SQLite"}, ""},
		{"blank answer waits", false, []tea.Key{up, enter, letter(' '), enter, esc, letter('1')}, []string{"SQLite"}, ""},
		{"multi checks with Space", true, []tea.Key{space, down, down, space, enter}, []string{"SQLite", "Memory"}, ""},
		{"multi checks with digits", true, []tea.Key{letter('3'), letter('2'), enter}, []string{"Postgres", "Memory"}, ""},
		{"multi unchecks", true, []tea.Key{letter('1'), letter('2'), letter('1'), enter}, []string{"Postgres"}, ""},
		{"multi with none checked sends the highlighted", true, []tea.Key{down, enter}, []string{"Postgres"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := asking(t, 100, 30, tt.multi)
			in := typeKeys(m, tt.keys...)
			if in == nil || in.Kind != api.ChatInputReply || in.ID != "q1" || in.Text != tt.text ||
				fmt.Sprint(in.Picked) != fmt.Sprint(tt.picked) || in.Validate() != nil {
				t.Fatalf("sent %+v, want picked %v text %q", in, tt.picked, tt.text)
			}
			if len(m.input) != 0 {
				t.Fatalf("keys reached the input: %q", string(m.input))
			}
		})
	}
	m := asking(t, 100, 30, false)
	if in := m.Key(esc); in == nil || in.Kind != api.ChatInputInterrupt {
		t.Fatalf("Esc sent %+v", in)
	}
	if _, _, ok := m.Cursor(); ok {
		t.Fatal("the input has a cursor while the question waits")
	}
}

// TestQuestionAfterInterrupt: once the turn stopped, typing goes to the
// message, and the box takes only the arrows, Space, and Enter while the
// message is empty.
func TestQuestionAfterInterrupt(t *testing.T) {
	m := asking(t, 100, 30, false)
	m.Event(ev(api.ChatEvent{Kind: api.ChatEventTurnEnded, Turn: 1, Reason: api.ChatEndInterrupted}))
	if !strings.Contains(screen(m), "An answer starts a new one") {
		t.Fatalf("no note that the turn stopped:\n%s", screen(m))
	}
	if in := typeKeys(m, down, enter); in == nil || in.Kind != api.ChatInputReply || in.Picked[0] != "Postgres" {
		t.Fatalf("Down, Enter sent %+v", in)
	}
	if in := typeKeys(m, typed("2 more")...); in != nil || string(m.input) != "2 more" {
		t.Fatalf("typing sent %+v; input %q", in, string(m.input))
	}
	if in := m.Key(enter); in == nil || in.Kind != api.ChatInputMessage || in.Text != "2 more" {
		t.Fatalf("Enter with a message sent %+v", in)
	}
	m.Event(ev(api.ChatEvent{Kind: api.ChatEventTurnStarted, Turn: 2, Text: "2 more"}))
	if strings.Contains(screen(m), "Question") {
		t.Fatalf("the box outlived the next turn:\n%s", screen(m))
	}
}

// TestQuestionClick: a click on a row answers, or checks it in a
// multi-select question, and Send sends the checked options.
func TestQuestionClick(t *testing.T) {
	click := func(m *Model, text string) *api.ChatInput {
		t.Helper()
		for y, r := range m.View() {
			if x := strings.Index(ansi.Strip(r), text); x >= 0 {
				return m.Click(ansi.StringWidth(ansi.Strip(r)[:x])+1, y)
			}
		}
		t.Fatalf("%q is not on screen:\n%s", text, screen(m))
		return nil
	}
	m := asking(t, 100, 30, false)
	if in := click(m, "Memory"); in == nil || in.Picked[0] != "Memory" {
		t.Fatalf("click sent %+v", in)
	}
	m = asking(t, 100, 30, false)
	if in := click(m, "Other: type"); in != nil || !m.ask.typing {
		t.Fatalf("click on Other sent %+v, typing %v", in, m.ask.typing)
	}
	m = asking(t, 100, 30, true)
	if in := click(m, "SQLite"); in != nil {
		t.Fatalf("click in a multi-select question sent %+v", in)
	}
	click(m, "Memory")
	if in := click(m, " Send "); in == nil || fmt.Sprint(in.Picked) != "[SQLite Memory]" {
		t.Fatalf("Send sent %+v", in)
	}
}

func TestQuestionGolden(t *testing.T) {
	states := map[string]struct {
		multi bool
		state func(m *Model)
	}{
		"Single":  {false, func(*Model) {}},
		"Preview": {false, func(m *Model) { m.Key(down) }},
		"Multi":   {true, func(m *Model) { typeKeys(m, letter('1'), letter('3')) }},
		"Other":   {false, func(m *Model) { typeKeys(m, seq([]tea.Key{up, enter}, typed("Redis, with a TTL"))...) }},
		"Stopped": {false, func(m *Model) {
			m.Event(ev(api.ChatEvent{Kind: api.ChatEventTurnEnded, Turn: 1, Reason: api.ChatEndInterrupted}))
		}},
	}
	for name, s := range states {
		for _, th := range []struct {
			name  string
			theme theme.Theme
		}{{"Dark", theme.Default()}, {"Light", lightTheme()}} {
			for _, size := range [][2]int{{80, 24}, {120, 40}} {
				t.Run(fmt.Sprintf("%s%s%dx%d", name, th.name, size[0], size[1]), func(t *testing.T) {
					m := asking(t, size[0], size[1], s.multi)
					m.SetTheme(th.theme)
					s.state(m)
					golden.RequireEqual(t, strings.Join(m.View(), "\n"))
				})
			}
		}
	}
}
