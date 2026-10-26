package chat

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// commandFixtures are the commands a command list golden shows.
var commandFixtures = []command.Command{
	{Name: "model", Args: "[id]", Desc: "Pick a model", Group: command.GroupModel},
	{Name: "effort", Args: "[level]", Desc: "Set reasoning effort", Group: command.GroupModel},
	{Name: "login", Desc: "Save a provider key", Group: command.GroupModel},
	{Name: "new", Desc: "Start a new conversation", Group: command.GroupConversation},
	{Name: "resume", Desc: "Return to a past conversation", Group: command.GroupConversation},
	{Name: "rename", Args: "[title]", Desc: "Rename this conversation", Group: command.GroupConversation},
	{Name: "mode", Args: "[ask|auto]", Desc: "Set the approval mode", Group: command.GroupConversation},
	{Name: "open", Args: "[path]", Desc: "Open a file", Group: command.GroupWorkspace},
	{Name: "help", Desc: "Show commands and keys", Group: command.GroupSnow},
}

// commandChat returns a chat with the fixture commands and their test
// behavior: every command sends its name and rest.
func commandChat(t *testing.T, w, h int) *Model {
	t.Helper()
	m, _ := newChat(w, h)
	m.reg = command.NewRegistry()
	m.runs = map[string]RunFunc{}
	for _, c := range commandFixtures {
		c.Where = command.Chat
		if err := m.reg.Register(c); err != nil {
			t.Fatalf("register %s: %v", c.Name, err)
		}
		name := c.Name
		m.runs[name] = func(h Host, rest string) { h.Send(api.ChatInputMessage, name+" "+rest) }
	}
	return m
}

func TestChatSlashOpensList(t *testing.T) {
	m := commandChat(t, 120, 40)
	typeText(m, "/")
	if !m.cmdOpen {
		t.Fatal("/ did not open the command list")
	}
	s := screen(m)
	for _, want := range []string{"Model", "Conversation", "/model [id]", "/help"} {
		if !strings.Contains(s, want) {
			t.Fatalf("the list does not show %q:\n%s", want, s)
		}
	}
}

func TestChatSlashAfterText(t *testing.T) {
	m := commandChat(t, 80, 24)
	typeText(m, "hello /")
	if m.cmdOpen {
		t.Fatal("/ after other text opened the command list")
	}
	if s := screen(m); strings.Contains(s, "/model [id]") {
		t.Fatalf("the command list shows after text:\n%s", s)
	}
	m.Key(key('u', tea.ModCtrl, ""))
	typeText(m, "/")
	if !m.cmdOpen {
		t.Fatal("typing / again did not open the list")
	}
}

func TestChatSlashAfterArgument(t *testing.T) {
	m := commandChat(t, 80, 24)
	typeText(m, "/rename ")
	if m.cmdOpen {
		t.Fatal("a space after the command name kept the list open")
	}
}

func TestCommandRunsFromChat(t *testing.T) {
	m := commandChat(t, 80, 24)
	m.reg = command.NewRegistry()
	m.runs = map[string]RunFunc{
		"fake": func(h Host, rest string) { h.Send(api.ChatInputMessage, "ran "+rest) },
	}
	if err := m.reg.Register(command.Command{
		Name: "fake", Args: "<name>", Desc: "A fake command", Group: command.GroupConversation, Where: command.Chat,
	}); err != nil {
		t.Fatal(err)
	}
	typeText(m, "/fak")
	if !m.cmdOpen {
		t.Fatal("the list did not open")
	}
	m.Key(key(tea.KeyTab, 0, ""))
	if got := string(m.input); got != "/fake " {
		t.Fatalf("Tab completed to %q, want %q", got, "/fake ")
	}
	if m.cmdOpen {
		t.Fatal("the list stayed open after completing an argument")
	}
	typeText(m, "arg")
	in := m.Key(key(tea.KeyEnter, 0, ""))
	if in == nil || in.Kind != api.ChatInputMessage || in.Text != "ran arg" || in.Pane != 2 || in.Agent != "c1" {
		t.Fatalf("Enter sent %+v", in)
	}
	if len(m.input) != 0 {
		t.Fatalf("the input kept %q", string(m.input))
	}
}

func TestCommandEnterRunsHighlighted(t *testing.T) {
	m := commandChat(t, 80, 24)
	typeText(m, "/ren")
	in := m.Key(key(tea.KeyEnter, 0, ""))
	if in == nil || in.Text != "rename " {
		t.Fatalf("Enter sent %+v, want the highlighted rename", in)
	}
}

func TestUnknownCommandIsNotSent(t *testing.T) {
	m := commandChat(t, 80, 24)
	typeText(m, "/nope")
	if in := m.Key(key(tea.KeyEnter, 0, "")); in != nil {
		t.Fatalf("an unknown command sent %+v", in)
	}
	if !strings.Contains(m.Note(), "No command matches") {
		t.Fatalf("note = %q", m.Note())
	}
}

func TestCommandClickCompletesOrRuns(t *testing.T) {
	m := commandChat(t, 80, 24)
	typeText(m, "/")
	row := rowOf(t, m, "/rename [title]")
	if in := m.Click(1, row); in != nil {
		t.Fatalf("clicking a command with arguments sent %+v", in)
	}
	if got := string(m.input); got != "/rename " {
		t.Fatalf("click completed to %q, want %q", got, "/rename ")
	}

	m = commandChat(t, 80, 24)
	typeText(m, "/")
	row = rowOf(t, m, "/login")
	in := m.Click(1, row)
	if in == nil || in.Text != "login " {
		t.Fatalf("clicking a command without arguments sent %+v", in)
	}
}

func rowOf(t *testing.T, m *Model, text string) int {
	t.Helper()
	for i, line := range m.View() {
		if strings.Contains(ansi.Strip(line), text) {
			return i
		}
	}
	t.Fatalf("%q is not on screen:\n%s", text, screen(m))
	return 0
}

func TestCommandListGolden(t *testing.T) {
	light := lightTheme()
	for _, th := range []struct {
		name  string
		theme theme.Theme
	}{{"Dark", theme.Default()}, {"Light", light}} {
		for _, size := range [][2]int{{120, 40}, {80, 24}, {40, 24}} {
			t.Run(fmt.Sprintf("%s%dx%d", th.name, size[0], size[1]), func(t *testing.T) {
				m := commandChat(t, size[0], size[1])
				m.SetTheme(th.theme)
				typeText(m, "/")
				golden.RequireEqual(t, strings.Join(m.View(), "\n"))
			})
		}
	}
}

// lightTheme is a light-terminal variant of the default tokens, so the
// goldens cover both terminal backgrounds. Snow ships only dark palettes.
func lightTheme() theme.Theme {
	th := theme.Default()
	th.Name = "Light"
	th.Border = color.RGBA{R: 0x99, G: 0x99, B: 0x99, A: 0xff}
	th.FocusBorder = color.RGBA{R: 0x22, G: 0x66, B: 0xcc, A: 0xff}
	th.Accent = color.RGBA{R: 0x22, G: 0x66, B: 0xcc, A: 0xff}
	th.Muted = color.RGBA{R: 0x66, G: 0x66, B: 0x66, A: 0xff}
	th.Selection = color.RGBA{R: 0xbb, G: 0xd6, B: 0xff, A: 0xff}
	th.SelectionFG = color.RGBA{R: 0, G: 0, B: 0, A: 0xff}
	th.UserFG = color.RGBA{R: 0x22, G: 0x66, B: 0xcc, A: 0xff}
	th.UserBG = color.RGBA{R: 0xdd, G: 0xee, B: 0xff, A: 0xff}
	th.AgentFG = color.RGBA{R: 0x22, G: 0x66, B: 0xcc, A: 0xff}
	th.AgentBG = color.RGBA{R: 0xee, G: 0xee, B: 0xee, A: 0xff}
	return th
}
