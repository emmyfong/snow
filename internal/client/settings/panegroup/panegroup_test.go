package panegroup

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/settings"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/config"
)

func TestPaneGroupShowsConfigValues(t *testing.T) {
	tests := []struct {
		name   string
		group  settings.Group
		values []string
	}{
		{
			name:  "first detected profile",
			group: NewProfiles(config.Defaults()),
			values: []string{
				"Default profile", "first detected", "new panes, after the server restarts",
				"~/.snow/config.toml", "[profile] default",
			},
		},
		{
			name:  "named profile",
			group: NewProfiles(config.Config{Profile: config.Profile{Default: "Ubuntu"}}),
			values: []string{
				"Default profile", "Ubuntu", "new panes, after the server restarts",
				"~/.snow/config.toml", "[profile] default",
			},
		},
		{
			name:  "scrollback",
			group: NewScrollback(config.Config{Scrollback: config.Scrollback{Lines: 2000}}),
			values: []string{
				"Scrollback", "2000 lines", "each pane, after the server restarts",
				"~/.snow/config.toml", "[scrollback] lines",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := tt.group.View(theme.Default(), 52, 12)
			for _, want := range tt.values {
				if !strings.Contains(view, want) {
					t.Errorf("view does not show %q:\n%s", want, view)
				}
			}
		})
	}
}

func TestPaneGroupReadOnly(t *testing.T) {
	tests := []struct {
		name  string
		group settings.Group
	}{
		{name: "profiles", group: NewProfiles(config.Defaults())},
		{name: "scrollback", group: NewScrollback(config.Defaults())},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := tt.group.View(theme.Default(), 52, 12)
			for _, msg := range []tea.Msg{
				tea.KeyPressMsg{Code: tea.KeyEnter},
				tea.KeyPressMsg{Text: "change"},
				tea.KeyPressMsg{Code: tea.KeyDelete},
			} {
				if cmd := tt.group.Update(msg); cmd != nil {
					t.Fatalf("Update(%T) returned a command; want read-only", msg)
				}
			}
			if after := tt.group.View(theme.Default(), 52, 12); after != before {
				t.Fatalf("input changed the read-only view:\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}
}

func TestPaneGroupNoRuntimeControls(t *testing.T) {
	for _, group := range []settings.Group{NewProfiles(config.Defaults()), NewScrollback(config.Defaults())} {
		view := group.View(theme.Default(), 52, 12)
		if !strings.Contains(view, "No runtime reports these controls yet.") {
			t.Errorf("%s group has no unavailable runtime note:\n%s", group.Title(), view)
		}
		if strings.Contains(view, "[runtime]") || strings.Contains(view, "Runtime control") {
			t.Errorf("%s group shows an editable runtime control:\n%s", group.Title(), view)
		}
	}
}

func TestPaneGroupGolden80x24(t *testing.T)  { paneGroupGolden(t, 80, 24) }
func TestPaneGroupGolden120x40(t *testing.T) { paneGroupGolden(t, 120, 40) }

func paneGroupGolden(t *testing.T, cols, rows int) {
	t.Helper()
	m := settings.New([]settings.Group{NewProfiles(config.Defaults())})
	m.SetTheme(theme.Default())
	m.SetSize(cols, rows)
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	golden.RequireEqual(t, m.View().Content)
}
