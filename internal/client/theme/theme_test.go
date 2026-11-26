package theme

import (
	"errors"
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

func TestDefaultIsFrost(t *testing.T) {
	if got := Default().Name; got != "Frost" {
		t.Fatalf("default theme %q, want Frost (chosen by the user in #33)", got)
	}
}

func TestBuiltinThemesAreComplete(t *testing.T) {
	themes := Builtin()
	if len(themes) != 12 {
		t.Fatalf("%d built-in themes, want 12", len(themes))
	}
	seen := map[string]bool{}
	for _, th := range themes {
		if th.Name == "" || seen[strings.ToLower(th.Name)] {
			t.Fatalf("theme name %q is empty or repeated", th.Name)
		}
		seen[strings.ToLower(th.Name)] = true
		for name, c := range th.colors() {
			if c == nil {
				t.Errorf("%s: token %s has no color", th.Name, name)
			}
		}
	}
}

func TestByName(t *testing.T) {
	for _, name := range []string{"nord", "Tokyo Night", "CLAY"} {
		if _, ok := ByName(name); !ok {
			t.Errorf("ByName(%q) found nothing", name)
		}
	}
	if _, ok := ByName("no-such-theme"); ok {
		t.Fatal("ByName found a theme that does not exist")
	}
}

func TestResolveFillsMissingTokens(t *testing.T) {
	partial := Theme{Name: "mine", Border: Default().Error}
	got := partial.Resolve()
	if got.Border != Default().Error {
		t.Fatal("Resolve replaced a token the theme set")
	}
	for name, c := range got.colors() {
		if c == nil {
			t.Errorf("Resolve left token %s empty", name)
		}
	}
	if got.Glyphs.Vertical == "" {
		t.Error("Resolve left the glyphs empty")
	}
}

// TestTokensDownsample: every text-on-background pair stays readable when a
// terminal can show only 256 or 16 colors. Readable means a contrast ratio of
// at least 3:1, the WCAG minimum for bold interface text. For adjusts a
// theme to the terminal's colors.
func TestTokensDownsample(t *testing.T) {
	profiles := []colorprofile.Profile{colorprofile.TrueColor, colorprofile.ANSI256, colorprofile.ANSI}
	for _, base := range Builtin() {
		for _, p := range profiles {
			th := base.For(p)
			pairs := map[string][2]color.Color{
				"default text":   {th.Text, th.Background},
				"added word":     {th.Added, th.Background},
				"modified word":  {th.Modified, th.Background},
				"deleted word":   {th.Removed, th.Background},
				"renamed word":   {th.Accent, th.Background},
				"untracked word": {th.Muted, th.Background},
				"status bar":     {th.BarFG, th.BarBG},
				"active tab":     {th.TabActive, th.TabActiveBG},
				"question":       {th.PromptFG, th.PromptBG},
				"error":          {th.ErrorFG, th.Error},
				"position":       {th.IndicatorFG, th.IndicatorBG},
				"match":          {th.MatchFG, th.MatchBG},
				"current":        {th.CurrentFG, th.CurrentBG},
				"selected":       {th.SelectedFG, th.SelectedBG},
				"list row":       {th.SelectionFG, th.Selection},
				"you label":      {th.UserFG, th.UserBG},
				"snow label":     {th.AgentFG, th.AgentBG},
				"activity":       {th.ActivityFG, th.ActivityBG},
			}
			for what, pair := range pairs {
				if r := contrast(p.Convert(pair[0]), p.Convert(pair[1])); r < minContrast {
					t.Errorf("%s at %s: %s contrast %.2f, want at least %.0f", th.Name, p, what, r, minContrast)
				}
			}
			// The shown window's block must stand out from the bar around it.
			if r := contrast(p.Convert(th.TabActiveBG), p.Convert(th.BarBG)); r < minSeparation {
				t.Errorf("%s at %s: tab block vs bar contrast %.2f, want at least %.0f", th.Name, p, r, minSeparation)
			}
		}
	}
}

// TestTrueColorKeepsThePalette: a full-color terminal sees each theme
// exactly as designed.
func TestTrueColorKeepsThePalette(t *testing.T) {
	for _, th := range Builtin() {
		if got := th.For(colorprofile.TrueColor); got.Background != th.Background || got.Text != th.Text || got.BarFG != th.BarFG || got.PromptFG != th.PromptFG || got.TabActive != th.TabActive || got.TabActiveBG != th.TabActiveBG || got.Added != th.Added || got.Modified != th.Modified || got.Removed != th.Removed || got.Accent != th.Accent || got.Muted != th.Muted {
			t.Errorf("%s: For(TrueColor) changed the palette", th.Name)
		}
	}
}

func TestNamed(t *testing.T) {
	for _, name := range []string{"default", "Frost", "tokyo night"} {
		if _, err := Named(name); err != nil {
			t.Errorf("Named(%q) error = %v", name, err)
		}
	}
	_, err := Named("neon")
	if !errors.Is(err, ErrUnknownTheme) || !strings.Contains(err.Error(), "Nord") {
		t.Fatalf("Named(neon) error = %v, want ErrUnknownTheme listing the themes", err)
	}
}

// TestForTerminalsWithoutColor: with no colors to show (no terminal, or an
// ASCII-only one), there is nothing to adapt, and For must not fail.
func TestForTerminalsWithoutColor(t *testing.T) {
	for _, p := range []colorprofile.Profile{colorprofile.NoTTY, colorprofile.ASCII} {
		if got := Default().For(p); got.BarFG != Default().BarFG {
			t.Errorf("For(%s) changed the theme", p)
		}
	}
}

// TestAgentGlyphsFallBackWithoutColor: a terminal with no colors may also
// lack the drawn marks, so agent states and the driven mark turn into ASCII
// there.
func TestAgentGlyphsFallBackWithoutColor(t *testing.T) {
	d := Default()
	if d.Glyphs.AgentIdle != "○" || d.Glyphs.AgentWorking != "◐" || d.Glyphs.AgentNeedsYou != "●" {
		t.Fatalf("default agent glyphs %q %q %q", d.Glyphs.AgentIdle, d.Glyphs.AgentWorking, d.Glyphs.AgentNeedsYou)
	}
	if d.Glyphs.Driven != "✎" {
		t.Fatalf("default driven glyph %q, want ✎", d.Glyphs.Driven)
	}
	for _, p := range []colorprofile.Profile{colorprofile.NoTTY, colorprofile.ASCII} {
		g := d.For(p).Glyphs
		if g.AgentIdle != "-" || g.AgentWorking != "~" || g.AgentNeedsYou != "!" {
			t.Errorf("For(%s) agent glyphs %q %q %q, want - ~ !", p, g.AgentIdle, g.AgentWorking, g.AgentNeedsYou)
		}
		if g.Driven != "*" {
			t.Errorf("For(%s) driven glyph %q, want *", p, g.Driven)
		}
	}
	if g := d.For(colorprofile.ANSI).Glyphs; g.AgentNeedsYou != "●" || g.Driven != "✎" {
		t.Errorf("For(ANSI) changed the glyphs to %q and %q", g.AgentNeedsYou, g.Driven)
	}
}

func TestPaneButtonGlyphsOneCell(t *testing.T) {
	for _, th := range Builtin() {
		for name, glyph := range map[string]string{"Model": th.Glyphs.Model, "Zoom": th.Glyphs.Zoom, "Close": th.Glyphs.Close} {
			if width := ansi.StringWidth(glyph); width != 1 {
				t.Errorf("%s %s width = %d, want 1", th.Name, name, width)
			}
		}
	}
}

func TestChromeGlyphsFallBackWithoutColor(t *testing.T) {
	for _, p := range []colorprofile.Profile{colorprofile.NoTTY, colorprofile.ASCII} {
		g := Default().For(p).Glyphs
		for name, tt := range map[string]struct{ got, want string }{
			"Dropdown": {g.Dropdown, "v"}, "Menu": {g.Menu, "="}, "Model": {g.Model, "v"}, "Zoom": {g.Zoom, "+"}, "Close": {g.Close, "x"},
		} {
			if tt.got != tt.want {
				t.Errorf("For(%s) %s=%q,want %q", p, name, tt.got, tt.want)
			}
		}
	}
}

func TestNoColorDropsBackground(t *testing.T) {
	for _, p := range []colorprofile.Profile{colorprofile.NoTTY, colorprofile.ASCII} {
		if got := Default().For(p); got.Background != nil || got.Text != nil {
			t.Errorf("For(%s) kept default colors", p)
		}
	}
}

func TestBarColorsFollowADR0016(t *testing.T) {
	barBG := map[string]string{
		"Frost": "#1e2a36", "Campbell": "#13a10e", "One Half Dark": "#3e4451",
		"Dark+": "#007acc", "Solarized Dark": "#073642", "Dracula": "#44475a",
		"Nord": "#3b4252", "Gruvbox Dark": "#3c3836", "Tokyo Night": "#24283b",
		"Catppuccin Mocha": "#313244", "OpenCode": "#141414", "Clay": "#2b2a27",
	}
	for _, th := range Builtin() {
		tabBG, tabFG := th.Accent, ink(colorprofile.TrueColor, th.Accent)
		switch th.Name {
		case "Campbell":
			tabBG, tabFG = lipgloss.Color("#0c0c0c"), lipgloss.Color("#f2f2f2")
		case "Dark+":
			tabBG, tabFG = lipgloss.Color("#ffffff"), lipgloss.Color("#007acc")
		}
		if th.BarBG != lipgloss.Color(barBG[th.Name]) || th.BarFG != th.IndicatorFG ||
			th.TabActiveBG != tabBG || th.TabActive != tabFG {
			t.Errorf("%s: wrong bar or tab colors", th.Name)
		}
	}
}

func TestMutedOnPaintedBackground(t *testing.T) {
	for _, th := range Builtin() {
		for _, p := range []colorprofile.Profile{colorprofile.ANSI, colorprofile.ANSI256} {
			painted := th.ForBackground(p, th.Background)
			for _, fg := range []color.Color{painted.Muted, painted.Queued} {
				if got := contrast(p.Convert(fg), painted.Background); got < minContrast {
					t.Errorf("%s %s: muted contrast %.2f, want 3:1", th.Name, p, got)
				}
			}
			if plain := th.For(p); plain.Queued != th.Queued {
				t.Errorf("%s %s: changed queued text in terminal mode", th.Name, p)
			}
		}
		if painted := th.ForBackground(colorprofile.TrueColor, th.Background); painted.Muted != th.Muted || painted.Queued != th.Queued {
			t.Errorf("%s: changed exact true-color palette", th.Name)
		}
	}
}
