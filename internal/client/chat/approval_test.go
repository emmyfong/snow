package chat

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

// sgr matches the style sequences Snow itself draws.
var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

var writeCall = api.ChatTool{ID: "w1", Name: "write", Input: json.RawMessage(`{"path":"internal/rename/rename.go","content":"x"}`)}

var writeApproval = api.ChatApproval{
	ID: "w1", Tool: "write", Input: writeCall.Input,
	Diff: &api.ChatDiff{Path: "internal/rename/rename.go", Old: editOld, New: editNew},
}

// waiting returns a chat whose turn waits for an answer about a write.
func waiting(t *testing.T, w, h int) *Model {
	t.Helper()
	m, _ := newChat(w, h)
	m.Event(ev(api.ChatEvent{Kind: api.EventTurnStarted, Turn: 1, Text: "Trim names in rename.go"}))
	call := writeCall
	m.Event(ev(api.ChatEvent{Kind: api.EventToolCall, Tool: &call}))
	a := writeApproval
	m.Event(ev(api.ChatEvent{Kind: api.EventApproval, Approval: &a}))
	m.Event(ev(api.ChatEvent{Kind: api.EventState, State: api.ChatStateApproval}))
	return m
}

func typeKeys(m *Model, ks ...tea.Key) *api.ChatInput {
	var last *api.ChatInput
	for _, k := range ks {
		if in := m.Key(k); in != nil {
			last = in
		}
	}
	return last
}

var (
	enter = key(tea.KeyEnter, 0, "")
	right = key(tea.KeyRight, 0, "")
	left  = key(tea.KeyLeft, 0, "")
)

func letter(r rune) tea.Key { return key(r, 0, string(r)) }

// TestApprovalBoxKeys: each choice sends the right answer; "no" takes a
// reason; the diff toggles; other typing goes nowhere.
func TestApprovalBoxKeys(t *testing.T) {
	reason := []tea.Key{letter('n')}
	for _, r := range "too risky" {
		reason = append(reason, letter(r))
	}
	reason = append(reason, enter)
	tests := []struct {
		name     string
		keys     []tea.Key
		decision string
		text     string
	}{
		{"y", []tea.Key{letter('y')}, api.DecisionYes, ""},
		{"Enter on the first choice", []tea.Key{enter}, api.DecisionYes, ""},
		{"a", []tea.Key{letter('a')}, api.DecisionAlways, ""},
		{"Right then Enter", []tea.Key{right, enter}, api.DecisionAlways, ""},
		{"Left wraps to the diff, back to no", []tea.Key{left, left, enter, enter}, api.DecisionNo, ""},
		{"n with a reason", reason, api.DecisionNo, "too risky"},
		{"Esc leaves the reason", []tea.Key{letter('n'), key(tea.KeyEscape, 0, ""), letter('y')}, api.DecisionYes, ""},
		{"typing waits", []tea.Key{letter('q'), letter('y')}, api.DecisionYes, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := waiting(t, 100, 30)
			in := typeKeys(m, tt.keys...)
			if in == nil || in.Kind != api.InputAnswer || in.ID != "w1" || in.Decision != tt.decision || in.Text != tt.text || in.Validate() != nil {
				t.Fatalf("sent %+v", in)
			}
			if len(m.input) != 0 {
				t.Fatalf("keys reached the input: %q", string(m.input))
			}
		})
	}
	m := waiting(t, 100, 30)
	m.Key(letter('d'))
	if !strings.Contains(screen(m), "Hide diff") || !strings.Contains(screen(m), "+ import \"strings\"") {
		t.Fatalf("d did not show the diff:\n%s", screen(m))
	}
	if in := m.Key(key(tea.KeyEscape, 0, "")); in == nil || in.Kind != api.InputInterrupt {
		t.Fatalf("Esc sent %+v", in)
	}
}

// TestApprovalBoxClick: a click on a choice picks it.
func TestApprovalBoxClick(t *testing.T) {
	m := waiting(t, 100, 30)
	for y, r := range m.View() {
		if x := strings.Index(ansi.Strip(r), "Always for write"); x >= 0 {
			in := m.Click(x+1, y)
			if in == nil || in.Decision != api.DecisionAlways {
				t.Fatalf("click sent %+v", in)
			}
			return
		}
	}
	t.Fatalf("no button:\n%s", screen(m))
}

// TestApprovalSurvivesDetach: a snapshot that holds a waiting request, as
// a client gets on attach, shows the box, and the answer goes out.
func TestApprovalSurvivesDetach(t *testing.T) {
	a := writeApproval
	call := writeCall
	m := New(&api.ChatSnapshot{
		Pane: 2, Agent: "c1", Model: "fake/model-1", Mode: "ask", State: api.ChatStateApproval, Running: true, Turn: 1,
		Entries: []api.ChatEntry{{Kind: api.EntryUser, Turn: 1, Text: "Trim names"}, {Kind: api.EntryTool, Turn: 1, Tool: &call}},
		Pending: &a,
	}, waiting(t, 1, 1).th, nil)
	m.SetSize(100, 30)
	if s := screen(m); !strings.Contains(s, "Allow write?") || !strings.Contains(s, "waiting for you") {
		t.Fatalf("screen:\n%s", s)
	}
	if _, _, ok := m.Cursor(); ok {
		t.Fatal("the input kept the cursor while the box waits")
	}
	if in := m.Key(letter('y')); in == nil || in.ID != "w1" || in.Decision != api.DecisionYes {
		t.Fatalf("sent %+v", in)
	}
	m.Event(ev(api.ChatEvent{Kind: api.EventToolResult, Tool: &api.ChatTool{ID: "w1", Result: "wrote 7 lines"}}))
	if strings.Contains(screen(m), "Allow write?") {
		t.Fatal("the box stayed after the result")
	}
}

// TestApprovalBoxGolden: the box with its choices, with the diff shown,
// and asking for a reason, plus a request outside the folder.
func TestApprovalBoxGolden(t *testing.T) {
	states := map[string]func(m *Model){
		"Choices": func(*Model) {},
		"Diff":    func(m *Model) { m.Key(letter('d')) },
		"Reason": func(m *Model) {
			m.Key(letter('n'))
			for _, r := range "use TrimSpace only" {
				m.Key(letter(r))
			}
		},
		"Outside": func(m *Model) {
			a := writeApproval
			a.ID, a.Diff, a.Outside = "w2", nil, []string{"/etc/hosts"}
			a.Input = json.RawMessage(`{"path":"/etc/hosts","content":"x"}`)
			m.Event(ev(api.ChatEvent{Kind: api.EventApproval, Approval: &a}))
		},
	}
	for name, state := range states {
		for _, size := range [][2]int{{80, 24}, {120, 40}} {
			t.Run(fmt.Sprintf("%s%dx%d", name, size[0], size[1]), func(t *testing.T) {
				m := waiting(t, size[0], size[1])
				state(m)
				golden.RequireEqual(t, strings.Join(m.View(), "\n"))
			})
		}
	}
}

// runWaiting returns a chat whose turn waits for an answer about a run
// of command.
func runWaiting(t *testing.T, w, h int, command string) *Model {
	t.Helper()
	m, _ := newChat(w, h)
	in, _ := json.Marshal(map[string]string{"command": command})
	m.Event(ev(api.ChatEvent{Kind: api.EventTurnStarted, Turn: 1, Text: "clean up"}))
	m.Event(ev(api.ChatEvent{Kind: api.EventToolCall, Tool: &api.ChatTool{ID: "r1", Name: "run", Input: in}}))
	m.Event(ev(api.ChatEvent{Kind: api.EventApproval, Approval: &api.ChatApproval{ID: "r1", Tool: "run", Input: in}}))
	return m
}

// TestApprovalShowsWholeCommand: every line of the command the user
// approves shows, a long line wraps instead of being cut, and a command
// too long for the box says how much is hidden until d shows it all.
func TestApprovalShowsWholeCommand(t *testing.T) {
	m := runWaiting(t, 80, 30, "echo hello\nrm -rf ~/important")
	s := screen(m)
	if !strings.Contains(s, "echo hello") || !strings.Contains(s, "rm -rf ~/important") {
		t.Fatalf("a line of the command is hidden:\n%s", s)
	}
	if !strings.Contains(s, "(+1 line)") {
		t.Fatalf("the cell does not mark the hidden line:\n%s", s)
	}

	long := "find . -name '*.tmp' -newer stamp -exec rm -- {} + && echo removed-every-temporary-file-under-the-folder END"
	s = screen(runWaiting(t, 60, 30, long))
	if !strings.Contains(strings.ReplaceAll(strings.ReplaceAll(s, "│", ""), " ", ""), "END") {
		t.Fatalf("the end of a long command is cut:\n%s", s)
	}

	var lines []string
	for i := range 30 {
		lines = append(lines, fmt.Sprintf("step %d", i))
	}
	m = runWaiting(t, 80, 60, strings.Join(lines, "\n"))
	s = screen(m)
	if strings.Contains(s, "step 29") || !strings.Contains(s, "more lines") {
		t.Fatalf("a long command shows no marker:\n%s", s)
	}
	m.Key(letter('d'))
	if s = screen(m); !strings.Contains(s, "step 29") {
		t.Fatalf("d did not show the whole command:\n%s", s)
	}
}

// TestApprovalBoxCleansText: escape sequences in the input and in paths
// outside the folder do not reach the screen.
func TestApprovalBoxCleansText(t *testing.T) {
	m := runWaiting(t, 80, 30, "echo a\x1b[2Jb\x1b]52;c;QUJD\x07\x1b_Gx\x1b\\\x1bPq\x1b\\\rc")
	a := *m.snap.Pending
	a.ID, a.Outside = "r2", []string{"/etc/\x1b[8mpass\x1b[0mwd"}
	m.Event(ev(api.ChatEvent{Kind: api.EventApproval, Approval: &a}))
	assertNoForeignEscapes(t, m.View())
}

// assertNoForeignEscapes fails when a row holds an escape sequence other
// than the SGR styles Snow draws with.
func assertNoForeignEscapes(t *testing.T, rows []string) {
	t.Helper()
	for _, r := range rows {
		rest := sgr.ReplaceAllString(r, "")
		if strings.ContainsAny(rest, "\x1b\x07\r\x08") {
			t.Fatalf("row holds a foreign control sequence: %q", r)
		}
	}
}

// TestApprovalWaitsWhileTyping: a box that comes while the user types
// takes no letters and no Enter, so a word with a or y in it never
// answers it. The keys edit the message instead.
func TestApprovalWaitsWhileTyping(t *testing.T) {
	m := waiting(t, 100, 30)
	m.input = []rune("and ad")
	m.cursor = len(m.input)
	if in := m.Key(letter('a')); in != nil {
		t.Fatalf("a sent %+v", in)
	}
	if in := m.Key(letter('y')); in != nil {
		t.Fatalf("y sent %+v", in)
	}
	in := m.Key(key(tea.KeyEnter, 0, ""))
	if in == nil || in.Kind != api.InputMessage || in.Text != "and aday" {
		t.Fatalf("Enter sent %+v, want the message", in)
	}
	if m.pending() == nil {
		t.Fatal("the box is gone")
	}
	if in := m.Key(letter('y')); in == nil || in.Kind != api.InputAnswer || in.Decision != api.DecisionYes {
		t.Fatalf("y on an empty line sent %+v", in)
	}
}

// TestLongReasonStillAnswers: a reason past MaxChatMessage is cut, so the
// answer is still valid.
func TestLongReasonStillAnswers(t *testing.T) {
	m := waiting(t, 100, 30)
	m.Key(letter('n'))
	m.box.text = strings.Repeat("é", api.MaxChatMessage)
	in := m.Key(key(tea.KeyEnter, 0, ""))
	if in == nil || in.Decision != api.DecisionNo || in.Validate() != nil || !utf8.ValidString(in.Text) {
		t.Fatalf("answer %d bytes, %v", len(in.Text), in.Validate())
	}
}
