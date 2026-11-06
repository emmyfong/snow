package runtimepick

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

var (
	ready   = api.RuntimeConnection{ID: "k3j9qa7c2m", Name: "Claude Code", Auth: "subscription", Environment: "This machine", Status: "signed-in", Ready: true}
	keyed   = api.RuntimeConnection{ID: "p0x7d3lw8e", Name: "Claude Code", Auth: "api-key", Environment: "This machine", Status: "key-saved"}
	wsl     = api.RuntimeConnection{ID: "u2vb6t1yno", Name: "Claude Code", Auth: "subscription", Environment: "WSL Ubuntu", Status: "unknown"}
	out     = api.RuntimeConnection{ID: "r5s8w2ehzk", Name: "Claude Code", Auth: "subscription", Environment: "WSL Ubuntu", Status: "signed-out"}
	blocked = api.RuntimeConnection{ID: "b7c4f9q1ad", Name: "Claude Code", Auth: "api-key", Environment: "This machine", Status: "blocked",
		Reason: "Your organization requires Claude subscription sign-in, and Claude Code refuses API keys."}
)

func picked(connections ...api.RuntimeConnection) *Model {
	m := New()
	m.SetList(&api.RuntimeConnections{Connections: connections})
	return m
}

// open selects the connection on row i and opens its choices.
func open(t *testing.T, m *Model, i int) {
	t.Helper()
	m.list.Select(i)
	if got := m.Key(tea.Key{Code: tea.KeyEnter}); got != picker.Accepted || m.Action().Kind != api.RuntimeStatus {
		t.Fatalf("opening a connection did not check its status: %v", got)
	}
}

func labels(m *Model) []string {
	var out []string
	for _, c := range m.choices {
		out = append(out, c.label)
	}
	return out
}

// TestStatusWords: each state reads as itself. Ready is separate from
// signed in, and unknown never reads as signed out.
func TestStatusWords(t *testing.T) {
	tests := []struct {
		c    api.RuntimeConnection
		want string
	}{
		{ready, "Signed in, ready"},
		{api.RuntimeConnection{Auth: "subscription", Status: "signed-in"}, "Signed in"},
		{keyed, "Key saved"},
		{api.RuntimeConnection{Auth: "api-key", Status: "signed-out"}, "No key saved"},
		{out, "Signed out"},
		{blocked, "Blocked by policy"},
		{wsl, "Not checked yet"},
		{api.RuntimeConnection{Status: "unknown", Reason: "claude did not answer"}, "Status unknown"},
	}
	for _, tt := range tests {
		if got := statusText(tt.c); got != tt.want {
			t.Errorf("statusText(%+v) = %q, want %q", tt.c, got, tt.want)
		}
	}
}

// TestBlockedOffersNoSignIn: a connection the policy blocks offers no way to
// use it or sign in, and shows why.
func TestBlockedOffersNoSignIn(t *testing.T) {
	m := picked(blocked)
	open(t, m, 0)
	for _, label := range labels(m) {
		if label == "Use for this chat" || label == "Enter an API key" || label == "Sign in" {
			t.Fatalf("blocked connection offers %q", label)
		}
	}
	if !strings.Contains(viewText(m, 120, 40), "requires Claude subscription") {
		t.Fatal("policy reason not shown")
	}
}

// TestRouteChoices: a subscription connection offers sign-in in Snow; an
// API-key connection offers a key, saved only for it.
func TestRouteChoices(t *testing.T) {
	m := picked(out, keyed)
	open(t, m, 0)
	if got := labels(m); got[1] != "Sign in" {
		t.Fatalf("subscription choices: %q", got)
	}
	m.Key(tea.Key{Code: tea.KeyEscape})
	m = picked(out, keyed)
	open(t, m, 1)
	if got := labels(m); got[1] != "Enter an API key" {
		t.Fatalf("API-key choices: %q", got)
	}
}

// TestKeyNeverFormatted: an action that holds a pasted key never shows it.
func TestKeyNeverFormatted(t *testing.T) {
	a := Action{Kind: api.RuntimeSaveKey, Connection: "c", Key: "sk-ant-secret"}
	if s := fmt.Sprintf("%v %+v %#v %s", a, a, a, a); strings.Contains(s, "sk-ant-secret") {
		t.Fatalf("formatted action shows the key: %s", s)
	}
}

func viewText(m *Model, cols, rows int) string {
	box, _, _ := m.View(theme.Default(), cols, rows)
	return ansi.Strip(box)
}

// TestPickerGolden covers each state at both sizes: the first run, the list
// with every kind of state, one connection signed out, a blocked one, and a
// refused operation.
func TestPickerGolden(t *testing.T) {
	first := func() *Model {
		m := New()
		m.SetList(&api.RuntimeConnections{Connections: []api.RuntimeConnection{}, Ignored: []string{"ANTHROPIC_API_KEY"}})
		return m
	}
	list := func() *Model { return picked(ready, keyed, wsl, blocked) }
	signedOut := func() *Model {
		m := picked(out)
		m.list.Select(0)
		m.Key(tea.Key{Code: tea.KeyEnter})
		return m
	}
	block := func() *Model {
		m := picked(blocked)
		m.list.Select(0)
		m.Key(tea.Key{Code: tea.KeyEnter})
		return m
	}
	refused := func() *Model {
		m := signedOut()
		m.SetList(&api.RuntimeConnections{Connections: []api.RuntimeConnection{out}, State: "error",
			Message: "claude is not installed in this WSL distribution. Install Claude Code in WSL Ubuntu."})
		return m
	}
	for _, tt := range []struct {
		name string
		m    func() *Model
	}{{"first", first}, {"list", list}, {"signedout", signedOut}, {"blocked", block}, {"refused", refused}} {
		for _, size := range []struct{ cols, rows int }{{80, 24}, {120, 40}} {
			t.Run(fmt.Sprintf("%s_%dx%d", tt.name, size.cols, size.rows), func(t *testing.T) {
				box, _, _ := tt.m().View(theme.Default(), size.cols, size.rows)
				golden.RequireEqual(t, box)
			})
		}
	}
}

// TestFailureKeepsReason: after a refused operation the failure shows, and
// the connection's own reason stays visible beside it.
func TestFailureKeepsReason(t *testing.T) {
	unknown := api.RuntimeConnection{ID: "u2vb6t1yno", Name: "Claude Code", Auth: "subscription", Environment: "This machine",
		Status: "unknown", Reason: "claude did not answer"}
	m := picked(unknown)
	open(t, m, 0)
	m.SetList(&api.RuntimeConnections{Connections: []api.RuntimeConnection{unknown}, State: "error", Message: "Sign-in refused."})
	if text := viewText(m, 120, 40); !strings.Contains(text, "Sign-in refused.") || !strings.Contains(text, "did not answer") {
		t.Fatalf("view lost the failure or the reason:\n%s", text)
	}
}
