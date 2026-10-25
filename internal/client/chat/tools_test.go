package chat

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

func readResult(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "%d\tline %d of rename.go\n", i, i)
	}
	return b.String()
}

const editOld = "package rename\n\nfunc Rename(s string) string {\n\treturn s\n}\n"
const editNew = "package rename\n\nimport \"strings\"\n\nfunc Rename(s string) string {\n\treturn strings.TrimSpace(s)\n}\n"

// toolTurn plays a turn with a read, an edit with a diff, and a failed run.
func toolTurn(m *Model) {
	m.Event(ev(api.ChatEvent{Kind: api.EventTurnStarted, Turn: 1, Text: "Trim names in rename.go"}))
	m.Event(ev(api.ChatEvent{Kind: api.EventText, Text: "Reading it first."}))
	calls := []api.ChatTool{
		{ID: "t1", Name: "read", Input: json.RawMessage(`{"path":"internal/rename/rename.go"}`)},
		{ID: "t2", Name: "edit", Input: json.RawMessage(`{"path":"internal/rename/rename.go","old":"x","new":"y"}`),
			Diff: &api.ChatDiff{Path: "internal/rename/rename.go", Old: editOld, New: editNew}},
		{ID: "t3", Name: "run", Input: json.RawMessage(`{"command":"go test ./internal/rename"}`)},
	}
	results := []api.ChatTool{
		{ID: "t1", Result: readResult(212)},
		{ID: "t2", Result: "edited internal/rename/rename.go"},
		{ID: "t3", Result: "exit status 1\n--- FAIL: TestRename\n    rename_test.go:12: got \"a \"", IsError: true},
	}
	for i := range calls {
		m.Event(ev(api.ChatEvent{Kind: api.EventToolCall, Tool: &calls[i]}))
		m.Event(ev(api.ChatEvent{Kind: api.EventToolResult, Tool: &results[i]}))
	}
	m.Event(ev(api.ChatEvent{Kind: api.EventText, Text: "The test still fails."}))
	m.Event(ev(api.ChatEvent{Kind: api.EventTurnEnded, Turn: 1, Reason: api.EndDone}))
}

// TestToolCellGolden: collapsed, expanded, failed, and edit with diff.
func TestToolCellGolden(t *testing.T) {
	states := map[string]func(m *Model){
		"Collapsed": func(*Model) {},
		"ExpandedRead": func(m *Model) {
			m.expanded["t1"] = true
		},
		"EditDiff": func(m *Model) {
			m.expanded["t2"] = true
		},
		"Failed": func(m *Model) {
			m.Key(key(tea.KeyTab, 0, ""))
			m.Key(key(tea.KeyEnter, 0, ""))
		},
	}
	for name, state := range states {
		for _, size := range [][2]int{{80, 24}, {120, 40}} {
			t.Run(fmt.Sprintf("%s%dx%d", name, size[0], size[1]), func(t *testing.T) {
				m, _ := newChat(size[0], size[1])
				toolTurn(m)
				state(m)
				m.invalidate()
				golden.RequireEqual(t, strings.Join(m.View(), "\n"))
			})
		}
	}
}

// TestToolCellSummary: the collapsed rows say what each call did.
func TestToolCellSummary(t *testing.T) {
	m, _ := newChat(100, 30)
	toolTurn(m)
	s := screen(m)
	for _, want := range []string{"read internal/rename/rename.go  212 lines", "edit internal/rename/rename.go  +3 -1", "run go test ./internal/rename  failed exit status 1"} {
		if !strings.Contains(s, want) {
			t.Fatalf("no %q in\n%s", want, s)
		}
	}
}

// TestToolCellExpand: Tab selects the latest cell and walks back, Enter
// toggles it, and a click toggles the cell it lands on.
func TestToolCellExpand(t *testing.T) {
	m, _ := newChat(100, 40)
	toolTurn(m)
	m.Key(key(tea.KeyTab, 0, ""))
	m.Key(key(tea.KeyTab, 0, ""))
	if m.selected != "t2" {
		t.Fatalf("selected %q", m.selected)
	}
	if in := m.Key(key(tea.KeyEnter, 0, "")); in != nil || !m.expanded["t2"] {
		t.Fatalf("Enter sent %+v, expanded %v", in, m.expanded)
	}
	if !strings.Contains(screen(m), "+ import \"strings\"") {
		t.Fatalf("no diff:\n%s", screen(m))
	}
	m.Key(key(tea.KeyEscape, 0, ""))
	if m.selected != "" {
		t.Fatal("Esc kept the selection")
	}
	m.Key(key(tea.KeyEnter, 0, ""))
	if !m.expanded["t2"] {
		t.Fatal("Enter with no selection toggled a cell")
	}

	rows := m.View()
	for y, r := range rows {
		if strings.Contains(stripped(r), "read internal/rename") {
			m.Click(10, y)
			break
		}
	}
	if !m.expanded["t1"] {
		t.Fatalf("click did not expand the read cell:\n%s", screen(m))
	}
}

func stripped(s string) string { return ansi.Strip(s) }

// TestToolAndErrorTextCleaned: escape sequences in a tool's input and
// result, and in a vendor's error message, do not reach the screen.
func TestToolAndErrorTextCleaned(t *testing.T) {
	const bad = "a\x1b[2Jb\x1b]52;c;QUJD\x07\x1b_Gx\x1b\\\x1bPq\x1b\\\rc\x08"
	m, _ := newChat(80, 40)
	in, _ := json.Marshal(map[string]string{"path": bad, "k" + bad: "v"})
	m.Event(ev(api.ChatEvent{Kind: api.EventTurnStarted, Turn: 1, Text: "read"}))
	m.Event(ev(api.ChatEvent{Kind: api.EventToolCall, Tool: &api.ChatTool{ID: "t1", Name: "read", Input: in}}))
	m.Event(ev(api.ChatEvent{Kind: api.EventToolResult, Tool: &api.ChatTool{ID: "t1", Done: true, IsError: true, Result: bad + "\n" + bad}}))
	m.expanded["t1"] = true
	m.invalidate()
	assertNoForeignEscapes(t, m.View())

	m.Event(ev(api.ChatEvent{Kind: api.EventTurnEnded, Turn: 1, Reason: api.EndFailed, Error: &api.ChatError{Code: api.ChatErrOther, Message: bad}}))
	assertNoForeignEscapes(t, m.View())
}
