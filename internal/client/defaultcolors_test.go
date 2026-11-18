package client

import (
	"image/color"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/home"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/config"
	"github.com/emmyfong/snow/pkg/api"
)

func TestDefaultColorsFollowTheme(t *testing.T) {
	th := theme.Default()
	a := newHomeApp(func(api.Message) {}, "dev", "", th)
	if v := a.View(); v.BackgroundColor != nil || v.ForegroundColor != nil {
		t.Fatal("applied colors before the terminal profile was known")
	}
	a.Update(tea.ColorProfileMsg{Profile: colorprofile.TrueColor})
	a.Update(tea.ForegroundColorMsg{Color: color.White})
	if v := a.View(); v.BackgroundColor != nil || v.ForegroundColor != nil {
		t.Fatal("applied colors without a response for both terminal defaults")
	}
	a.Update(tea.BackgroundColorMsg{Color: color.Black})
	for _, profile := range []colorprofile.Profile{colorprofile.TrueColor, colorprofile.ANSI256, colorprofile.ANSI, colorprofile.ASCII} {
		a.Update(tea.ColorProfileMsg{Profile: profile})
		want := th.For(profile)
		if v := a.View(); v.BackgroundColor != want.Background || v.ForegroundColor != want.Text {
			t.Errorf("terminal defaults for %s = (%v, %v), want (%v, %v)", profile, v.BackgroundColor, v.ForegroundColor, want.Background, want.Text)
		}
	}
}

func TestThemeChangeRepaints(t *testing.T) {
	a := newHomeApp(func(api.Message) {}, "dev", "", theme.Default())
	a.Update(tea.ColorProfileMsg{Profile: colorprofile.ANSI256})
	a.Update(tea.ForegroundColorMsg{Color: color.White})
	a.Update(tea.BackgroundColorMsg{Color: color.Black})
	a.Update(command.ThemeMsg{Name: "Nord"})
	nord, ok := theme.ByName("Nord")
	if !ok {
		t.Fatal("Nord theme missing")
	}
	want := nord.For(colorprofile.ANSI256)
	if v := a.View(); v.BackgroundColor != want.Background || v.ForegroundColor != want.Text {
		t.Fatalf("Nord did not repaint the default colors: %v, %v", v.BackgroundColor, v.ForegroundColor)
	}
}

func TestConfigCheckSwitchesBackgroundMode(t *testing.T) {
	a := newHomeApp(func(api.Message) {}, "dev", "", theme.Default())
	a.terminalBackground = true
	fg, bg := color.RGBA{R: 241, G: 222, B: 194, A: 255}, color.RGBA{R: 46, G: 38, B: 45, A: 255}
	a.Update(tea.ColorProfileMsg{Profile: colorprofile.TrueColor})
	a.Update(tea.ForegroundColorMsg{Color: fg})
	a.Update(tea.BackgroundColorMsg{Color: bg})
	path := filepath.Join(t.TempDir(), "config.toml")
	for _, tt := range []struct {
		body   string
		bg, fg color.Color
	}{
		{"[theme]\nbackground = \"terminal\"\n", nil, nil},
		{"[theme]\nbackground = \"invalid\"\n", nil, nil},
		{"[theme]\nbackground = \"theme\"\n", theme.Default().Background, theme.Default().Text},
		{"[theme]\nbackground = \"terminal\"\n", bg, fg},
	} {
		if err := os.WriteFile(path, []byte(tt.body), 0o600); err != nil {
			t.Fatal(err)
		}
		a.Update(checkTheme(path, time.Time{}))
		v := a.View()
		if v.BackgroundColor != tt.bg || v.ForegroundColor != tt.fg || a.theme.Name != theme.Default().Name {
			t.Fatalf("unexpected terminal defaults after reading %q: %v, %v (%s)", tt.body, v.BackgroundColor, v.ForegroundColor, a.theme.Name)
		}
	}
}

func TestReducedColorHomeTracksPaintedBackground(t *testing.T) {
	nord, ok := theme.ByName("Nord")
	if !ok {
		t.Fatal("Nord theme missing")
	}
	a := newHomeApp(func(api.Message) {}, "dev", "", nord)
	h, ok := a.screen.(*home.Model)
	if !ok {
		t.Fatalf("screen = %T, want home", a.screen)
	}
	p := colorprofile.ANSI256
	a.Update(tea.ColorProfileMsg{Profile: p})
	if got := h.Theme().Muted; got != nord.For(p).Muted {
		t.Fatalf("before paint: muted = %v", got)
	}
	a.Update(tea.BackgroundColorMsg{Color: color.Black})
	a.Update(tea.ForegroundColorMsg{Color: color.White})
	if got := h.Theme().Muted; got != nord.ForBackground(p, nord.Background).Muted {
		t.Fatalf("while painting: muted = %v", got)
	}
	a.Update(configCheckedMsg{background: config.BackgroundTerminal})
	if got := h.Theme().Muted; got != nord.For(p).Muted {
		t.Fatalf("after restoring terminal mode: muted = %v", got)
	}
}
