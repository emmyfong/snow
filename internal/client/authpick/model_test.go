package authpick

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

func key(code rune) tea.Key { return tea.Key{Code: code} }

func text(s string) tea.Key { return tea.Key{Code: []rune(s)[0], Text: s} }

func typeText(m *Model, s string) {
	for _, r := range s {
		m.Key(text(string(r)))
	}
}

// providers is one connected provider and one without a key.
func providers() []api.ProviderAuth {
	return []api.ProviderAuth{
		{Name: "anthropic", Title: "Anthropic", KeyURL: "https://console.anthropic.com/settings/keys",
			Source: "OS credential store", Connected: true},
		{Name: "openai", Title: "OpenAI"},
	}
}

func open(t *testing.T) *Model {
	t.Helper()
	m := New()
	m.SetList(providers())
	return m
}

// TestEnterOpensKeyAndSubmits: Enter opens the key field, typing fills it,
// and Enter sends it for a check.
func TestEnterOpensKeyAndSubmits(t *testing.T) {
	m := open(t)
	m.Key(key(tea.KeyEnter))
	if m.stage != stageKey {
		t.Fatalf("Enter left stage %v, want the key field", m.stage)
	}
	typeText(m, "sk-test-123")
	if got := m.KeyValue(); got != "sk-test-123" {
		t.Fatalf("key %q", got)
	}
	if r := m.Key(key(tea.KeyEnter)); r != Submit {
		t.Fatalf("Enter: %v, want Submit", r)
	}
	if !m.Checking() {
		t.Fatal("the picker does not say it is checking")
	}
	if p, ok := m.Provider(); !ok || p.Name != "anthropic" {
		t.Fatalf("provider %+v, %v", p, ok)
	}
}

// TestFailKeepsTheKey: a refused key returns to the field with the key and
// the reason, so the user can correct it.
func TestFailKeepsTheKey(t *testing.T) {
	m := open(t)
	m.Key(key(tea.KeyEnter))
	typeText(m, "sk-bad")
	m.Key(key(tea.KeyEnter))
	m.Fail("the provider rejected the key")
	if m.stage != stageKey {
		t.Fatalf("stage %v, want the key field", m.stage)
	}
	if got := m.KeyValue(); got != "sk-bad" {
		t.Fatalf("key %q after a failure", got)
	}
	if !strings.Contains(m.note, "rejected") {
		t.Fatalf("note %q", m.note)
	}
}

// TestSavedReturnsToList: a saved key returns to the provider list, selects
// the provider, and says where the key went.
func TestSavedReturnsToList(t *testing.T) {
	m := open(t)
	m.Key(key(tea.KeyEnter))
	typeText(m, "sk-good")
	m.Key(key(tea.KeyEnter))
	m.Saved(&api.Login{Provider: "openai", LoginResult: api.LoginResult{
		Source: "OS credential store", Shadowed: true, Notice: "The fallback file is in use.",
	}})
	if m.stage != stageList {
		t.Fatalf("stage %v, want the provider list", m.stage)
	}
	if m.KeyValue() != "" {
		t.Fatalf("the key stayed: %q", m.KeyValue())
	}
	if p, _ := m.Provider(); p.Name != "openai" {
		t.Fatalf("selected %q, want openai", p.Name)
	}
	for _, want := range []string{"Saved to OS credential store.", "environment variable", "fallback file"} {
		if !strings.Contains(m.note, want) {
			t.Errorf("note %q does not hold %q", m.note, want)
		}
	}
}

// TestEsc: Esc closes the list, leaves the key field for the list, and
// closes while the server checks.
func TestEsc(t *testing.T) {
	m := open(t)
	if r := m.Key(key(tea.KeyEscape)); r != Cancelled {
		t.Fatalf("Esc on the list: %v, want Cancelled", r)
	}
	m = open(t)
	m.Key(key(tea.KeyEnter))
	if r := m.Key(key(tea.KeyEscape)); r != Open || m.stage != stageList {
		t.Fatalf("Esc in the key field: %v stage %v", r, m.stage)
	}
	m = open(t)
	m.Key(key(tea.KeyEnter))
	typeText(m, "sk-test")
	m.Key(key(tea.KeyEnter))
	if r := m.Key(key(tea.KeyEscape)); r != Cancelled {
		t.Fatalf("Esc while checking: %v, want Cancelled", r)
	}
}

// TestPaste: a pasted key lands in the field, never in the list.
func TestPaste(t *testing.T) {
	m := open(t)
	m.Paste("ignored")
	if m.KeyValue() != "" {
		t.Fatalf("a paste on the list filled the key: %q", m.KeyValue())
	}
	m.Key(key(tea.KeyEnter))
	m.Paste("sk-pasted\n")
	if got := m.KeyValue(); got != "sk-pasted" {
		t.Fatalf("pasted key %q", got)
	}
}

// TestSubmitRejects: an empty key and a key with spaces stay in the field
// with a note, and send nothing.
func TestSubmitRejects(t *testing.T) {
	for _, tt := range []struct {
		name string
		key  string
		want string
	}{
		{"empty", "", "Type the API key."},
		{"spaces", "sk two", "one word with no spaces"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := open(t)
			m.Key(key(tea.KeyEnter))
			typeText(m, tt.key)
			if r := m.Key(key(tea.KeyEnter)); r != Open {
				t.Fatalf("Enter: %v, want Open", r)
			}
			if m.stage != stageKey || !strings.Contains(m.note, tt.want) {
				t.Fatalf("stage %v note %q", m.stage, m.note)
			}
		})
	}
}

// TestSetListKeepsProvider: a fresh list keeps the provider the user chose.
func TestSetListKeepsProvider(t *testing.T) {
	m := open(t)
	m.Key(key(tea.KeyDown))
	if p, _ := m.Provider(); p.Name != "openai" {
		t.Fatalf("selected %q", p.Name)
	}
	m.SetList(append([]api.ProviderAuth{{Name: "other"}}, providers()...))
	if p, _ := m.Provider(); p.Name != "openai" {
		t.Fatalf("after a fresh list: %q", p.Name)
	}
}

// TestNoProviders: a picker with no providers says so and cannot ask for a
// key.
func TestNoProviders(t *testing.T) {
	m := New()
	box, _, _ := m.View(theme.Default(), 80, 24)
	if box == "" || !strings.Contains(box, "Loading providers.") {
		t.Fatalf("view:\n%s", box)
	}
	m.Key(key(tea.KeyEnter))
	if m.stage != stageList || m.note != "No providers." {
		t.Fatalf("stage %v note %q", m.stage, m.note)
	}
	m.SetList(nil)
	box, _, _ = m.View(theme.Default(), 80, 24)
	if !strings.Contains(box, "No providers.") {
		t.Fatalf("view:\n%s", box)
	}
}

// TestClick: a click on a provider opens its key field; a click outside
// closes the picker.
func TestClick(t *testing.T) {
	m := open(t)
	th := theme.Default()
	l, ok := m.layout(th, 80, 24)
	if !ok {
		t.Fatal("no layout")
	}
	if r := m.Click(th, 80, 24, l.x+2, l.listTop+1); r != Open {
		t.Fatalf("provider click: %v", r)
	}
	if m.stage != stageKey {
		t.Fatalf("stage %v, want the key field", m.stage)
	}
	if p, _ := m.Provider(); p.Name != "openai" {
		t.Fatalf("clicked provider %q", p.Name)
	}
	if r := m.Click(th, 80, 24, 0, 0); r != Cancelled {
		t.Fatalf("outside click: %v, want Cancelled", r)
	}
}

// TestMaskedKey: the picker draws one mask glyph per key rune and never the
// key itself.
func TestMaskedKey(t *testing.T) {
	m := open(t)
	m.Key(key(tea.KeyEnter))
	const secret = "sk-secret-123"
	typeText(m, secret)
	box, _, _ := m.View(theme.Default(), 80, 24)
	if strings.Contains(box, secret) || strings.Contains(ansi.Strip(box), secret) {
		t.Fatalf("the key is on screen:\n%s", ansi.Strip(box))
	}
	if !strings.Contains(ansi.Strip(box), strings.Repeat(theme.Default().Glyphs.Mask, len([]rune(secret)))) {
		t.Fatalf("the key is not masked:\n%s", ansi.Strip(box))
	}
}

// TestKeyStageShowsTheEnvironmentKey: the key field says which environment
// variable Snow reads first and where a saved key comes from.
func TestKeyStageShowsTheEnvironmentKey(t *testing.T) {
	m := New()
	m.SetList([]api.ProviderAuth{{Name: "anthropic", Title: "Anthropic",
		Source: "environment", EnvVar: "ANTHROPIC_API_KEY", Connected: true}})
	m.Key(key(tea.KeyEnter))
	box, _, _ := m.View(theme.Default(), 80, 24)
	s := ansi.Strip(box)
	for _, want := range []string{"A key is already saved from environment.", "Snow uses ANTHROPIC_API_KEY first."} {
		if !strings.Contains(s, want) {
			t.Errorf("the key field does not say %q:\n%s", want, s)
		}
	}
}

// TestLoginPickerGolden draws the provider list, the key field, and a
// refused key.
func TestLoginPickerGolden(t *testing.T) {
	th := theme.Default()
	t.Run("List", func(t *testing.T) {
		m := open(t)
		box, _, _ := m.View(th, 80, 24)
		golden.RequireEqual(t, box)
	})
	t.Run("Key", func(t *testing.T) {
		m := open(t)
		m.Key(key(tea.KeyEnter))
		typeText(m, "sk-test-123")
		box, _, _ := m.View(th, 80, 24)
		golden.RequireEqual(t, box)
	})
	t.Run("Error", func(t *testing.T) {
		m := open(t)
		m.Key(key(tea.KeyEnter))
		typeText(m, "sk-bad")
		m.Key(key(tea.KeyEnter))
		m.Fail("anthropic rejected the key (401)")
		box, _, _ := m.View(th, 80, 24)
		golden.RequireEqual(t, box)
	})
}

// TestViewFitsEveryWindow: the box fits its window exactly at several
// sizes, and is empty when the window is too small.
func TestViewFitsEveryWindow(t *testing.T) {
	m := open(t)
	for _, size := range [][2]int{{80, 24}, {120, 40}, {40, 24}, {36, 16}} {
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
