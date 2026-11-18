package newsession

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

func TestNewSessionKeyboard(t *testing.T) {
	m := New("/work/api")
	for range 2 {
		if got := m.Key(tea.Key{Code: tea.KeyTab}); got != picker.Open {
			t.Fatalf("Tab returned %v, want open", got)
		}
	}
	if got := m.Key(tea.Key{Code: tea.KeyDown}); got != picker.Open {
		t.Fatalf("Down returned %v, want open", got)
	}
	if got := m.Choice(); got.Layout != api.LayoutPresets[0].Name {
		t.Fatalf("layout = %q, want %q", got.Layout, api.LayoutPresets[0].Name)
	}
	m.Key(tea.Key{Code: tea.KeyTab})
	if got := m.Key(tea.Key{Code: 'x', Text: "x"}); got != picker.Open {
		t.Fatalf("typing returned %v, want open", got)
	}
	if got := m.Choice().Folder; got != "/work/apix" {
		t.Fatalf("folder = %q, want %q", got, "/work/apix")
	}
	if got := m.Key(tea.Key{Code: tea.KeyEnter}); got != picker.Accepted {
		t.Fatalf("Enter returned %v, want accepted", got)
	}
	if got := New("/work").Key(tea.Key{Code: tea.KeyEscape}); got != picker.Cancelled {
		t.Fatalf("Esc returned %v, want cancelled", got)
	}
}

func TestNewSessionMouseParity(t *testing.T) {
	byKey := New("/work/api")
	byKey.Key(tea.Key{Code: tea.KeyTab})
	byKey.Key(tea.Key{Code: tea.KeyTab})
	byKey.Key(tea.Key{Code: tea.KeyDown})

	byMouse := New("/work/api")
	box, x, y := byMouse.View(theme.Default(), 80, 24)
	plain := ansi.Strip(box)
	row := strings.Index(plain, api.LayoutPresets[0].Name)
	if row < 0 {
		t.Fatalf("no layout row in dialog:\n%s", plain)
	}
	rowY := strings.Count(plain[:row], "\n")
	lines := strings.Split(plain, "\n")
	rowX := ansi.StringWidth(strings.Split(lines[rowY], api.LayoutPresets[0].Name)[0])
	if got := byMouse.Click(theme.Default(), 80, 24, x+rowX, y+rowY); got != picker.Open {
		t.Fatalf("layout click returned %v, want open", got)
	}
	if byMouse.Choice() != byKey.Choice() {
		t.Fatalf("mouse choice %+v differs from keyboard choice %+v", byMouse.Choice(), byKey.Choice())
	}
	if got := byMouse.Click(theme.Default(), 80, 24, 0, 0); got != picker.Cancelled {
		t.Fatalf("outside click returned %v, want cancelled", got)
	}
}

func TestNewSessionTitle(t *testing.T) {
	m := New("/work/api")
	box, _, _ := m.View(theme.Default(), 80, 24)
	if !strings.Contains(ansi.Strip(box), "New session") {
		t.Fatalf("dialog title missing:\n%s", ansi.Strip(box))
	}
}

func TestNewSessionFolderPrefilled(t *testing.T) {
	m := New("/work/api")
	box, _, _ := m.View(theme.Default(), 80, 24)
	if !strings.Contains(ansi.Strip(box), "/work/api") {
		t.Fatalf("folder not shown:\n%s", ansi.Strip(box))
	}
	if got := m.Choice().Folder; got != "/work/api" {
		t.Fatalf("folder choice = %q, want %q", got, "/work/api")
	}
}

func TestNewSessionLayoutRows(t *testing.T) {
	m := New("")
	box, _, _ := m.View(theme.Default(), 80, 24)
	got := ansi.Strip(box)
	for i, preset := range api.LayoutPresets {
		if !strings.Contains(got, preset.Name) {
			t.Errorf("layout row %q missing:\n%s", preset.Name, got)
		}
		if i > 0 && strings.Index(got, preset.Name) < strings.Index(got, api.LayoutPresets[i-1].Name) {
			t.Errorf("layout rows are out of order:\n%s", got)
		}
	}
	if !strings.Contains(got, "No layout") || !strings.Contains(got, "Plain shell") {
		t.Fatalf("default rows missing:\n%s", got)
	}
}

func TestNewSessionGolden80x24(t *testing.T)  { newSessionGolden(t, 80, 24) }
func TestNewSessionGolden120x40(t *testing.T) { newSessionGolden(t, 120, 40) }

func newSessionGolden(t *testing.T, cols, rows int) {
	m := New("/work/api")
	box, _, _ := m.View(theme.Default(), cols, rows)
	golden.RequireEqual(t, box)
}
