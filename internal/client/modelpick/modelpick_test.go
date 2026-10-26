package modelpick

import (
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

func key(code rune) tea.Key { return tea.Key{Code: code} }

func text(s string) tea.Key { return tea.Key{Code: []rune(s)[0], Text: s} }

// providers is two connected providers with models, and one without a key.
func providers() []api.ProviderModels {
	all := []string{"low", "medium", "high", "xhigh", "max"}
	return []api.ProviderModels{
		{
			Name: "anthropic", Title: "Anthropic (Claude)", Connected: true,
			Models: []api.ModelInfo{
				{ID: "claude-opus-5-5", DisplayName: "Claude Opus 5.5", ContextWindow: 200_000, Efforts: all},
				{ID: "claude-haiku-5-5", DisplayName: "Claude Haiku 5.5", ContextWindow: 200_000},
			},
		},
		{
			Name: "openai", Title: "OpenAI", Connected: true,
			Models: []api.ModelInfo{
				{ID: "gpt-6.1-sol", DisplayName: "GPT-6.1 Sol", ContextWindow: 400_000, Efforts: []string{"low", "medium", "high"}},
			},
		},
		{Name: "other", Title: "Other", Connected: false},
	}
}

func open(t *testing.T, model, effort string) *Model {
	t.Helper()
	m := New(model, effort)
	m.SetList(providers())
	return m
}

// TestSelectsCurrent: the chat's provider, model, and effort are selected
// when the list arrives.
func TestSelectsCurrent(t *testing.T) {
	m := open(t, "openai/gpt-6.1-sol", "medium")
	if got, effort := m.Choice(); got != "openai/gpt-6.1-sol" || effort != "medium" {
		t.Fatalf("Choice = %q %q, want the chat's model", got, effort)
	}
}

// TestEnterChooses: Enter accepts the selected model; ctrl+d also saves it
// as the default.
func TestEnterChooses(t *testing.T) {
	m := open(t, "anthropic/claude-opus-5-5", "high")
	if r := m.Key(key(tea.KeyEnter)); r != picker.Accepted {
		t.Fatalf("Enter: %v, want Accepted", r)
	}
	if r := m.Key(tea.Key{Code: 'd', Mod: tea.ModCtrl}); r != picker.Saved {
		t.Fatalf("ctrl+d: %v, want Saved", r)
	}
}

// TestMoveModels: up and down move the model selection and wrap.
func TestMoveModels(t *testing.T) {
	m := open(t, "anthropic/claude-opus-5-5", "")
	m.Key(key(tea.KeyDown))
	if got, _ := m.Choice(); got != "anthropic/claude-haiku-5-5" {
		t.Fatalf("down: %q", got)
	}
	m.Key(key(tea.KeyDown))
	if got, _ := m.Choice(); got != "anthropic/claude-opus-5-5" {
		t.Fatalf("down at the end: %q, want wrap to the first", got)
	}
	m.Key(key(tea.KeyUp))
	if got, _ := m.Choice(); got != "anthropic/claude-haiku-5-5" {
		t.Fatalf("up: %q", got)
	}
}

// TestMoveProviders: Tab reaches the provider column, where up and down
// change which provider's models show.
func TestMoveProviders(t *testing.T) {
	m := open(t, "anthropic/claude-opus-5-5", "")
	m.Key(key(tea.KeyTab))
	if m.focus != focusEffort {
		t.Fatalf("first Tab goes to %v, want the effort row", m.focus)
	}
	m.Key(key(tea.KeyTab))
	m.Key(key(tea.KeyDown))
	if got, _ := m.Choice(); got != "openai/gpt-6.1-sol" {
		t.Fatalf("after moving providers: %q", got)
	}
	m.Key(key(tea.KeyDown))
	if got, _ := m.Choice(); got != "" {
		t.Fatalf("provider without models: %q", got)
	}
}

// TestTabCyclesColumns: Tab and shift+Tab visit the columns in order.
func TestTabCyclesColumns(t *testing.T) {
	m := open(t, "", "")
	if m.focus != focusModels {
		t.Fatalf("opens on %v, want the model column", m.focus)
	}
	for _, want := range []focus{focusEffort, focusProviders, focusModels} {
		m.Key(key(tea.KeyTab))
		if m.focus != want {
			t.Fatalf("Tab: %v, want %v", m.focus, want)
		}
	}
	m.Key(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.focus != focusProviders {
		t.Fatalf("shift+Tab: %v, want the provider column", m.focus)
	}
}

// TestEffortSkipsUnsupported: the model takes low and high only, so the
// effort row moves between those and the default.
func TestEffortSkipsUnsupported(t *testing.T) {
	m := New("", "")
	m.SetList([]api.ProviderModels{{
		Name: "p", Connected: true,
		Models: []api.ModelInfo{{ID: "m", Efforts: []string{"low", "high"}}},
	}})
	m.focus = focusEffort
	m.Key(key(tea.KeyRight))
	if got := m.levelEffort(); got != "low" {
		t.Fatalf("right: %q, want low", got)
	}
	m.Key(key(tea.KeyRight))
	if got := m.levelEffort(); got != "high" {
		t.Fatalf("right again: %q, want high", got)
	}
	m.Key(key(tea.KeyRight))
	if got := m.levelEffort(); got != "" {
		t.Fatalf("right at high: %q, want the default", got)
	}
	m.Key(key(tea.KeyLeft))
	if got := m.levelEffort(); got != "high" {
		t.Fatalf("left: %q, want high", got)
	}
}

// TestChangeModelResetsUnsupportedEffort: a model that takes no effort
// returns the choice to the provider's default.
func TestChangeModelResetsUnsupportedEffort(t *testing.T) {
	m := open(t, "anthropic/claude-opus-5-5", "high")
	m.Key(key(tea.KeyDown))
	if _, effort := m.Choice(); effort != "" {
		t.Fatalf("effort with a model without efforts: %q, want the default", effort)
	}
	m.focus = focusEffort
	m.Key(key(tea.KeyRight))
	if got := m.levelEffort(); got != "" {
		t.Fatalf("moving effort on a model without efforts: %q", got)
	}
}

// TestFilter: typing narrows the models; a filter that matches nothing
// leaves nothing to choose.
func TestFilter(t *testing.T) {
	m := open(t, "anthropic/claude-opus-5-5", "")
	for _, r := range "haiku" {
		m.Key(text(string(r)))
	}
	if got, _ := m.Choice(); got != "anthropic/claude-haiku-5-5" {
		t.Fatalf("filtered: %q", got)
	}
	for range "haiku" {
		m.Key(key(tea.KeyBackspace))
	}
	if got, _ := m.Choice(); got != "anthropic/claude-opus-5-5" {
		t.Fatalf("after backspace: %q", got)
	}
	m.Key(text("z"))
	m.Key(text("z"))
	if r := m.Key(key(tea.KeyEnter)); r != picker.Open || m.note == "" {
		t.Fatalf("Enter with no match: %v note %q, want it to stay open", r, m.note)
	}
}

// TestLoadingListKeepsTheCurrentModel: the empty cached list comes first,
// then the full one, which selects the chat's model.
func TestLoadingListKeepsTheCurrentModel(t *testing.T) {
	m := New("anthropic/claude-opus-5-5", "high")
	m.SetList([]api.ProviderModels{{Name: "anthropic", Connected: true, Refreshing: true}})
	if _, ok := m.selected(); ok {
		t.Fatal("a model is selected before the list arrives")
	}
	m.SetList(providers())
	if got, effort := m.Choice(); got != "anthropic/claude-opus-5-5" || effort != "high" {
		t.Fatalf("after the list: %q %q", got, effort)
	}
}

// TestRefreshKeepsTheUserPlace: after the user moves, a new list does not
// jump back to the chat's model.
func TestRefreshKeepsTheUserPlace(t *testing.T) {
	m := open(t, "anthropic/claude-opus-5-5", "")
	m.Key(key(tea.KeyDown))
	m.SetList(providers())
	if got, _ := m.Choice(); got != "anthropic/claude-haiku-5-5" {
		t.Fatalf("after a refresh: %q", got)
	}
}

// TestNoModel: a picker with no model stays open and says so, and the
// view still fits.
func TestNoModel(t *testing.T) {
	m := New("", "")
	m.SetList([]api.ProviderModels{{Name: "p", Title: "P", Connected: false}})
	if r := m.Key(key(tea.KeyEnter)); r != picker.Open {
		t.Fatalf("Enter: %v, want Open", r)
	}
	if m.note != "No model to choose." {
		t.Fatalf("note %q", m.note)
	}
	box, _, _ := m.View(theme.Default(), 80, 24)
	if box == "" || !strings.Contains(box, "No model to choose.") || !strings.Contains(box, "No key.") {
		t.Fatalf("view:\n%s", box)
	}
}

// TestModelPickerKeys: every key the design names does what it says.
func TestModelPickerKeys(t *testing.T) {
	tests := []struct {
		name    string
		keys    []tea.Key
		want    picker.Result
		model   string
		effort  string
		focus   focus
		focused bool
		note    bool
	}{
		{name: "Enter uses the model", keys: []tea.Key{key(tea.KeyEnter)}, want: picker.Accepted, model: "anthropic/claude-opus-5-5", effort: "high"},
		{name: "ctrl+d saves it", keys: []tea.Key{{Code: 'd', Mod: tea.ModCtrl}}, want: picker.Saved, model: "anthropic/claude-opus-5-5", effort: "high"},
		{name: "Esc cancels", keys: []tea.Key{key(tea.KeyEscape)}, want: picker.Cancelled},
		{name: "down moves", keys: []tea.Key{key(tea.KeyDown)}, want: picker.Open, model: "anthropic/claude-haiku-5-5", effort: ""},
		{name: "up wraps", keys: []tea.Key{key(tea.KeyUp)}, want: picker.Open, model: "anthropic/claude-haiku-5-5", effort: ""},
		{name: "Tab goes to effort", keys: []tea.Key{key(tea.KeyTab)}, want: picker.Open, model: "anthropic/claude-opus-5-5", effort: "high", focus: focusEffort, focused: true},
		{name: "shift+Tab goes to providers", keys: []tea.Key{{Code: tea.KeyTab, Mod: tea.ModShift}}, want: picker.Open, model: "anthropic/claude-opus-5-5", effort: "high", focus: focusProviders, focused: true},
		{name: "typing filters", keys: []tea.Key{text("h"), text("a"), text("i")}, want: picker.Open, model: "anthropic/claude-haiku-5-5", effort: ""},
		{name: "a filter with no match", keys: []tea.Key{text("z"), key(tea.KeyEnter)}, want: picker.Open, note: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := open(t, "anthropic/claude-opus-5-5", "high")
			var got picker.Result
			for _, k := range tt.keys {
				got = m.Key(k)
			}
			if got != tt.want {
				t.Fatalf("result %v, want %v", got, tt.want)
			}
			if tt.model != "" {
				if model, effort := m.Choice(); model != tt.model || effort != tt.effort {
					t.Fatalf("choice %q %q, want %q %q", model, effort, tt.model, tt.effort)
				}
			}
			if tt.focused && m.focus != tt.focus {
				t.Fatalf("focus %v, want %v", m.focus, tt.focus)
			}
			if tt.note != (m.note != "") {
				t.Fatalf("note %q, want note %v", m.note, tt.note)
			}
		})
	}
}

// TestEffortLimitedToModel: a model that supports fewer levels disables the
// others, and the choice moves only between the default and supported ones.
func TestEffortLimitedToModel(t *testing.T) {
	m := New("", "")
	m.SetList([]api.ProviderModels{{
		Name: "p", Connected: true,
		Models: []api.ModelInfo{
			{ID: "none"},
			{ID: "low-high", Efforts: []string{"low", "high"}},
			{ID: "all", Efforts: []string{"low", "medium", "high", "xhigh", "max"}},
		},
	}})
	tests := []struct {
		model string
		level int
		want  []bool // default plus each effort level
	}{
		{"none", 0, []bool{true, false, false, false, false, false}},
		{"low-high", 1, []bool{true, true, false, true, false, false}},
		{"all", 3, []bool{true, true, true, true, true, true}},
	}
	for _, tt := range tests {
		i := m.findModel(tt.model)
		m.setModel(i)
		m.level = tt.level
		m.clampLevel()
		for level, want := range tt.want {
			if got := m.levelEnabled(level); got != want {
				t.Errorf("%s level %d enabled = %v, want %v", tt.model, level, got, want)
			}
		}
	}
}

// TestModelPickerGolden draws both columns, a filter, and the effort row,
// in a dark and a light theme.
func TestModelPickerGolden(t *testing.T) {
	for _, tt := range []struct {
		name string
		th   theme.Theme
	}{{"Dark", theme.Default()}, {"Light", lightTheme()}} {
		t.Run(tt.name, func(t *testing.T) {
			m := open(t, "anthropic/claude-opus-5-5", "high")
			for _, r := range "opus" {
				m.Key(text(string(r)))
			}
			box, _, _ := m.View(tt.th, 80, 24)
			golden.RequireEqual(t, box)
		})
	}
}

// lightTheme is a theme whose colors read on a light terminal.
func lightTheme() theme.Theme {
	th := theme.Default()
	th.Name = "Light"
	th.Border = color.RGBA{R: 0x99, G: 0x99, B: 0x99, A: 0xff}
	th.FocusBorder = color.RGBA{R: 0x22, G: 0x66, B: 0xcc, A: 0xff}
	th.Accent = color.RGBA{R: 0x22, G: 0x66, B: 0xcc, A: 0xff}
	th.Muted = color.RGBA{R: 0x66, G: 0x66, B: 0x66, A: 0xff}
	th.Selection = color.RGBA{R: 0xbb, G: 0xd6, B: 0xff, A: 0xff}
	th.SelectionFG = color.RGBA{R: 0, G: 0, B: 0, A: 0xff}
	return th
}

// TestViewFitsEveryWindow: the box fits its window exactly at several
// sizes, and is empty when the window is too small.
func TestViewFitsEveryWindow(t *testing.T) {
	m := open(t, "anthropic/claude-opus-5-5", "high")
	for _, size := range [][2]int{{80, 24}, {120, 40}, {40, 24}, {36, 12}, {76, 8}} {
		box, x, y := m.View(theme.Default(), size[0], size[1])
		if box == "" {
			t.Fatalf("%v: no view", size)
		}
		lines := strings.Split(box, "\n")
		if len(lines) > size[1] || x < 0 || y < 0 {
			t.Fatalf("%v: box %d rows at %d,%d", size, len(lines), x, y)
		}
		for _, l := range lines {
			if w := ansi.StringWidth(l); w > size[0] {
				t.Fatalf("%v: a row is %d wide", size, w)
			}
		}
	}
	if box, _, _ := m.View(theme.Default(), 20, 6); box != "" {
		t.Fatalf("small window:\n%s", box)
	}
}

// TestClick: a provider click changes the models, a model click chooses,
// an effort click selects a level, and a click outside closes.
func TestClick(t *testing.T) {
	m := open(t, "anthropic/claude-opus-5-5", "")
	th := theme.Default()
	l, ok := m.layout(th, 80, 24)
	if !ok {
		t.Fatal("no layout")
	}
	if r := m.Click(th, 80, 24, l.leftX+1, l.listTop+1); r != picker.Open || m.providerName() != "openai" {
		t.Fatalf("provider click: %v %q", r, m.providerName())
	}
	if r := m.Click(th, 80, 24, l.rightX+1, l.listTop); r != picker.Accepted || m.modelID() != "gpt-6.1-sol" {
		t.Fatalf("model click: %v %q", r, m.modelID())
	}
	l, ok = m.layout(th, 80, 24)
	if !ok {
		t.Fatal("no layout after the model click")
	}
	var high chip
	for _, c := range l.chips {
		if c.index == 3 {
			high = c
		}
	}
	if r := m.Click(th, 80, 24, l.x+1+high.x, l.effortTop+high.line); r != picker.Open || m.level != 3 {
		t.Fatalf("effort click: %v level %d", r, m.level)
	}
	if r := m.Click(th, 80, 24, 0, 0); r != picker.Cancelled {
		t.Fatalf("outside click: %v, want Cancelled", r)
	}
}
