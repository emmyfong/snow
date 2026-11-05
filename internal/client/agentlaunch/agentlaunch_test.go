package agentlaunch

import (
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

var profiles = []string{"Claude Code", "Ubuntu Claude Code"}

func press(m *Model, keys ...tea.Key) picker.Result {
	r := picker.Open
	for _, k := range keys {
		r = m.Key(k)
	}
	return r
}

func text(s string) tea.Key { return tea.Key{Text: s, Code: []rune(s)[0]} }

// TestAgentLaunchCommand: the form sends the folder, the branch it was
// given, and the picked profile. An empty branch is left to the server.
func TestAgentLaunchCommand(t *testing.T) {
	m := New("/src/snow", profiles, nil)
	m.Paste("fix-login")
	r := press(m, tea.Key{Code: tea.KeyTab}, tea.Key{Code: tea.KeyDown}, tea.Key{Code: tea.KeyEnter})
	if r != picker.Accepted {
		t.Fatalf("Enter gave %v, want Accepted", r)
	}
	got := m.Command(4)
	want := map[string]string{api.ArgFolder: "/src/snow", api.ArgBranch: "fix-login", api.ArgProfile: "Ubuntu Claude Code"}
	if got.Op != api.OpAgentNew || got.Target != 4 || len(got.Args) != len(want) {
		t.Fatalf("command %+v", got)
	}
	for k, v := range want {
		if got.Args[k] != v {
			t.Fatalf("arg %s = %q, want %q", k, got.Args[k], v)
		}
	}
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, set := New("/src", profiles, nil).Command(1).Args[api.ArgBranch]; set {
		t.Fatal("an empty branch was sent")
	}
}

// TestAgentLaunchEditsFields: Tab moves between the fields, typing edits
// the focused one, and Backspace removes the last character.
func TestAgentLaunchEditsFields(t *testing.T) {
	m := New("/src", profiles, nil)
	press(m, tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}, text("/"), text("x"), tea.Key{Code: tea.KeyBackspace})
	if string(m.folder) != "/src/" {
		t.Fatalf("folder %q, want /src/", string(m.folder))
	}
	press(m, tea.Key{Code: tea.KeyTab}, text("a"))
	if string(m.branch) != "a" {
		t.Fatalf("branch %q, want a", string(m.branch))
	}
	if press(m, tea.Key{Code: tea.KeyEscape}) != picker.Cancelled {
		t.Fatal("Escape did not cancel")
	}
}

// TestAgentLaunchRefusesEmpty: with no profile, or no folder, Enter stays
// in the form and says what it needs.
func TestAgentLaunchRefusesEmpty(t *testing.T) {
	m := New("/src", nil, nil)
	if press(m, tea.Key{Code: tea.KeyEnter}) != picker.Open || !strings.Contains(m.note, "No Claude Code profile") {
		t.Fatalf("no profiles: note %q", m.note)
	}
	m = New("", profiles, nil)
	if press(m, tea.Key{Code: tea.KeyEnter}) != picker.Open || m.focus != fieldFolder {
		t.Fatalf("no folder: focus %v, note %q", m.focus, m.note)
	}
}

// TestAgentLaunchClick: a click on a profile picks it, a click on a field
// focuses it, and a click outside cancels.
func TestAgentLaunchClick(t *testing.T) {
	th := theme.Default()
	m := New("/src", profiles, nil)
	box, x, y := m.View(th, 80, 24)
	rows := strings.Split(box, "\n")
	find := func(s string) int {
		for i, r := range rows {
			if strings.Contains(ansi.Strip(r), s) {
				return y + i
			}
		}
		t.Fatalf("no row with %q", s)
		return 0
	}
	if m.Click(th, 80, 24, x+4, find("Ubuntu Claude Code")) != picker.Open || m.selected != 1 || m.focus != fieldProfile {
		t.Fatalf("after click on profile: selected %d, focus %v", m.selected, m.focus)
	}
	m.Click(th, 80, 24, x+4, find("Folder"))
	if m.focus != fieldFolder {
		t.Fatalf("focus %v after click on Folder", m.focus)
	}
	if m.Click(th, 80, 24, 0, 0) != picker.Cancelled {
		t.Fatal("a click outside did not cancel")
	}
}

// TestAgentLaunchGolden: the form at both sizes, with the branch field
// focused, the profile list focused, an error, and no profile.
func TestAgentLaunchGolden(t *testing.T) {
	th := theme.Default()
	cases := map[string]func() *Model{
		"branch": func() *Model { return New("/home/u/src/snow", profiles, nil) },
		"profile": func() *Model {
			m := New("/home/u/src/snow", profiles, nil)
			m.Paste("fix-login")
			press(m, tea.Key{Code: tea.KeyTab}, tea.Key{Code: tea.KeyDown})
			return m
		},
		"no-folder": func() *Model {
			m := New("", profiles, nil)
			press(m, tea.Key{Code: tea.KeyEnter})
			return m
		},
		"no-profile": func() *Model { return New("/home/u/src/snow", nil, nil) },
	}
	for name, mk := range cases {
		for _, size := range []struct{ w, h int }{{80, 24}, {120, 40}} {
			t.Run(name+"/"+sizeName(size.w, size.h), func(t *testing.T) {
				box, _, _ := mk().View(th, size.w, size.h)
				if box == "" {
					t.Fatal("no box")
				}
				golden.RequireEqual(t, box)
			})
		}
	}
}

func sizeName(w, h int) string { return strconv.Itoa(w) + "x" + strconv.Itoa(h) }

// TestAgentLaunchTiny: a window too small draws nothing and still takes
// keys.
func TestAgentLaunchTiny(t *testing.T) {
	m := New("/src", profiles, nil)
	if box, _, _ := m.View(theme.Default(), 20, 5); box != "" {
		t.Fatalf("drew %q in a tiny window", box)
	}
	if press(m, tea.Key{Code: tea.KeyEscape}) != picker.Cancelled {
		t.Fatal("Escape did not cancel")
	}
}

var roles = []api.RoleInfo{
	{Name: "reviewer", Title: "reviewer", PermissionMode: "plan", HasInstructions: true, ReadOnly: true},
	{Name: "coder", Title: "coder", HasInstructions: true},
}

// TestAgentRoleCommand: Tab reaches the role list, Down picks a role, and
// the command names it. With no role picked, the command has none.
func TestAgentRoleCommand(t *testing.T) {
	m := New("/src", profiles, roles)
	if _, set := m.Command(1).Args[api.ArgRole]; set {
		t.Fatal("a role is set before the user picks one")
	}
	press(m, tea.Key{Code: tea.KeyTab}, tea.Key{Code: tea.KeyTab}, tea.Key{Code: tea.KeyDown}, tea.Key{Code: tea.KeyDown})
	if got := m.Command(1).Args[api.ArgRole]; got != "coder" {
		t.Fatalf("role %q, want coder", got)
	}
	press(m, tea.Key{Code: tea.KeyUp}, tea.Key{Code: tea.KeyUp}, tea.Key{Code: tea.KeyUp})
	if _, set := m.Command(1).Args[api.ArgRole]; set {
		t.Fatal("No role still sends a role")
	}
}

// TestAgentRoleTabSkipsMissingList: with no roles, Tab goes from the
// profile list back to the folder.
func TestAgentRoleTabSkipsMissingList(t *testing.T) {
	m := New("/src", profiles, nil)
	press(m, tea.Key{Code: tea.KeyTab}, tea.Key{Code: tea.KeyTab})
	if m.focus != fieldFolder {
		t.Fatalf("focus %d, want the folder", m.focus)
	}
}

// TestAgentRoleClick: a click on a role row picks that role.
func TestAgentRoleClick(t *testing.T) {
	th := theme.Default()
	m := New("/src", profiles, roles)
	l, ok := m.layout(th, 120, 40)
	if !ok {
		t.Fatal("no layout")
	}
	m.Click(th, 120, 40, l.x+4, l.roleTop+1)
	if got := m.Command(1).Args[api.ArgRole]; got != "reviewer" {
		t.Fatalf("role %q, want reviewer", got)
	}
}

// TestAgentRoleGolden: the role list with a read-only role picked, and an
// instruction-only role, which the form shows as not enforced.
func TestAgentRoleGolden(t *testing.T) {
	th := theme.Default()
	cases := map[string]int{"read-only": 1, "not-enforced": 2}
	for name, downs := range cases {
		for _, size := range []struct{ w, h int }{{80, 24}, {120, 40}} {
			t.Run(name+"/"+sizeName(size.w, size.h), func(t *testing.T) {
				m := New("/home/u/src/snow", profiles, roles)
				press(m, tea.Key{Code: tea.KeyTab}, tea.Key{Code: tea.KeyTab})
				for range downs {
					press(m, tea.Key{Code: tea.KeyDown})
				}
				box, _, _ := m.View(th, size.w, size.h)
				if box == "" {
					t.Fatal("no box")
				}
				plain := ansi.Strip(box)
				want := map[string]string{"read-only": "read-only: plan mode, enforced", "not-enforced": "instructions only, not enforced"}[name]
				if !strings.Contains(plain, want) {
					t.Fatalf("form does not say %q:\n%s", want, plain)
				}
				if name == "not-enforced" && strings.Contains(plain, "read-only:") {
					t.Fatalf("an instruction-only role reads as read-only:\n%s", plain)
				}
				golden.RequireEqual(t, box)
			})
		}
	}
}
