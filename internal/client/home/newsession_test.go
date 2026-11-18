package home

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/newsession"
	"github.com/emmyfong/snow/pkg/api"
)

func TestOpenNewSessionFromEnter(t *testing.T) {
	m := screen(80, 24)
	m.SetFolder("/work/api")
	cmd := m.Update(press(tea.KeyEnter))
	if cmd == nil || m.dialog == nil {
		t.Fatalf("empty Enter: dialog open %v, command %v; want dialog and runtime request", m.dialog != nil, cmd)
	}
	sent, ok := cmd().(SendMsg)
	request, requestOK := sent.Msg.(*api.RuntimeRequest)
	if !ok || !requestOK || request.Pane != 0 || request.Agent != api.AgentClaude || request.Action != api.RuntimeList || request.Folder != "/work/api" {
		t.Fatalf("new-session request = %#v, want a Claude list request for /work/api", sent)
	}
	if got := m.dialog.Choice().Folder; got != "/work/api" {
		t.Fatalf("dialog folder = %q, want /work/api", got)
	}
	typeText(m, "work")
	if cmd := m.Update(press(tea.KeyEnter)); cmd == nil {
		t.Fatal("dialog Enter returned no OpenMsg command")
	} else if got, ok := cmd().(OpenMsg); !ok || got != (OpenMsg{Create: true, Dir: "/work/apiwork", Runtime: newsession.StartShell}) {
		t.Fatalf("dialog Enter produced %#v, want numbered session in edited folder", got)
	}
}

func TestNewSessionEscCloses(t *testing.T) {
	m := screen(80, 24)
	before := m.View().Content
	m.openNewSession()
	if cmd := m.Update(press(tea.KeyEscape)); cmd != nil || m.dialog != nil {
		t.Fatalf("Esc: dialog open %v, command %v; want closed without quitting", m.dialog != nil, cmd)
	}
	if got := m.View().Content; got != before {
		t.Fatal("closing the dialog changed the home page underneath it")
	}
}

func TestRuntimeReadinessRefusalDoesNotChangeHome(t *testing.T) {
	m := screen(80, 24)
	m.SetFolder("/work")
	before := m.View().Content
	m.openNewSession()
	m.Update(&api.Error{Code: api.CodeBadCommand, Message: api.ErrInvalidRuntime.Error()})
	if m.dialog == nil || m.notice != "" {
		t.Fatalf("readiness refusal changed dialog=%v notice=%q", m.dialog != nil, m.notice)
	}
	m.Update(press(tea.KeyEscape))
	if got := m.View().Content; got != before {
		t.Fatal("closing the dialog after a readiness refusal changed the home page")
	}
}

func TestHomeReceivesClaudeReadiness(t *testing.T) {
	m := screen(80, 24)
	m.SetFolder("/work")
	request := m.openNewSession()().(SendMsg).Msg.(*api.RuntimeRequest)
	m.Update(&api.RuntimeConnections{Pane: 0, Agent: api.AgentClaude, RequestID: request.RequestID,
		Connections: []api.RuntimeConnection{{ID: "conn", Status: api.RuntimeStatusSignedIn, Ready: true}}})
	box, _, _ := m.dialog.View(m.theme, 80, 24)
	if !strings.Contains(ansi.Strip(box), "Claude Code") {
		t.Fatalf("home did not pass ready connection to the dialog:\n%s", ansi.Strip(box))
	}
}

func TestHomeRechecksClaudeReadinessForEachDialog(t *testing.T) {
	m := screen(80, 24)
	m.runtimes = []api.RuntimeConnection{{ID: "previous", Status: api.RuntimeStatusSignedIn, Ready: true}}
	m.openNewSession()
	box, _, _ := m.dialog.View(m.theme, 80, 24)
	if strings.Contains(ansi.Strip(box), "Claude Code") {
		t.Fatal("new dialog showed stale Claude readiness before its current check")
	}
}

func TestNewSessionTooSmallShowsNotice(t *testing.T) {
	m := screen(20, 10)
	m.openNewSession()
	if m.dialog != nil || m.notice == "" {
		t.Fatalf("too-small window: dialog %v, notice %q", m.dialog != nil, m.notice)
	}
}

func TestNewSessionOverHomeGolden80x24(t *testing.T)  { newSessionOverHomeGolden(t, 80, 24) }
func TestNewSessionOverHomeGolden120x40(t *testing.T) { newSessionOverHomeGolden(t, 120, 40) }

func newSessionOverHomeGolden(t *testing.T, cols, rows int) {
	m := screen(cols, rows)
	m.SetFolder("/work/api")
	m.Update(press(tea.KeyEnter))
	golden.RequireEqual(t, m.View().Content)
}

func TestNewSessionCommand(t *testing.T) {
	m := screen(80, 24, session("work", 0, 0))
	typeText(m, "/new")
	if cmd := m.Update(press(tea.KeyEnter)); cmd == nil || m.dialog == nil {
		t.Fatalf("/new: dialog open %v, command %v; want dialog and runtime request", m.dialog != nil, cmd)
	}

	m = screen(80, 24)
	typeText(m, "/new work")
	if got := open(t, m.Update(press(tea.KeyEnter))); got != (OpenMsg{Session: "work", Create: true}) {
		t.Fatalf("/new work opened %+v, want named session", got)
	}
}

func TestNewSessionClickAcceptsChoice(t *testing.T) {
	m := screen(80, 24)
	m.SetFolder("/work")
	m.dialog = nil
	m.openNewSession()
	box, x, y := m.dialog.View(m.theme, 80, 24)
	if box == "" {
		t.Fatal("new-session dialog did not render")
	}
	// Enter in the footer accepts by mouse, matching the keyboard action.
	plain := strings.Split(ansi.Strip(box), "\n")
	var clickX, clickY int
	for row, line := range plain {
		if col := strings.Index(line, "Enter start"); col >= 0 {
			clickX, clickY = x+col+2, y+row
			break
		}
	}
	if clickY == 0 {
		t.Fatal("dialog has no Enter hint")
	}
	if cmd := m.Update(tea.MouseClickMsg{X: clickX, Y: clickY, Button: tea.MouseLeft}); cmd == nil {
		t.Fatal("clicking the Enter hint returned no OpenMsg")
	} else if got, ok := cmd().(OpenMsg); !ok || got != (OpenMsg{Create: true, Dir: "/work", Runtime: newsession.StartShell}) {
		t.Fatalf("click produced %#v, want numbered session in /work", got)
	}
}
