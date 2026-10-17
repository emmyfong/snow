package home

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// TestThemePickerPreviews: /theme lists the themes; moving previews each
// on the page, Esc puts the first one back, and Enter keeps the one shown.
func TestThemePickerPreviews(t *testing.T) {
	m := screen(80, 24, session("api", time.Minute, 0))
	typeText(m, "/theme")
	if got := enter(m); got != nil {
		t.Fatalf("/theme gave %#v, want the picker", got)
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "Tokyo Night") {
		t.Fatal("the picker does not list the themes")
	}
	m.Update(press(tea.KeyDown))
	if m.theme.Name != "Campbell" {
		t.Fatalf("theme %q after Down, want the preview of Campbell", m.theme.Name)
	}
	if got := msgOf(m.Update(press(tea.KeyEscape))); got != nil || m.theme.Name != "Frost" {
		t.Fatalf("Esc: sent %#v, theme %q; want nothing and Frost back", got, m.theme.Name)
	}
	typeText(m, "/theme")
	enter(m)
	m.Update(press(tea.KeyDown))
	m.Update(press(tea.KeyDown))
	if got := enter(m); got != (ThemeMsg{Name: "One Half Dark"}) {
		t.Fatalf("Enter gave %#v, want ThemeMsg{One Half Dark}", got)
	}
	if m.theme.Name != "One Half Dark" {
		t.Fatalf("theme %q after Enter, want One Half Dark kept", m.theme.Name)
	}
}

func TestThemeByName(t *testing.T) {
	m := screen(80, 24)
	typeText(m, "/theme tokyo night")
	if got := enter(m); got != (ThemeMsg{Name: "Tokyo Night"}) {
		t.Fatalf("/theme tokyo night gave %#v", got)
	}
	m = screen(80, 24)
	typeText(m, "/theme neon")
	if got := enter(m); got != nil {
		t.Fatalf("/theme neon gave %#v, want a refusal", got)
	}
	if !strings.Contains(m.notice, "Nord") {
		t.Fatalf("notice %q, want the list of themes", m.notice)
	}
}

func TestThemePickerGolden80x24(t *testing.T)  { goldenTyped(t, 80, 24, "/theme\r") }
func TestThemePickerGolden120x40(t *testing.T) { goldenTyped(t, 120, 40, "/theme\r") }
