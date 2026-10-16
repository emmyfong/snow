package theme

import (
	"errors"
	"image/color"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
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
				"status bar": {th.BarFG, th.BarBG},
				"active tab": {th.TabActive, th.TabActiveBG},
				"question":   {th.PromptFG, th.PromptBG},
				"error":      {th.ErrorFG, th.Error},
				"position":   {th.IndicatorFG, th.IndicatorBG},
				"match":      {th.MatchFG, th.MatchBG},
				"current":    {th.CurrentFG, th.CurrentBG},
				"selected":   {th.SelectedFG, th.SelectedBG},
			}
			for what, pair := range pairs {
				if r := contrast(p.Convert(pair[0]), p.Convert(pair[1])); r < minContrast {
					t.Errorf("%s at %s: %s contrast %.2f, want at least %.0f", th.Name, p, what, r, minContrast)
				}
			}
		}
	}
}

// TestTrueColorKeepsThePalette: a full-color terminal sees each theme
// exactly as designed.
func TestTrueColorKeepsThePalette(t *testing.T) {
	for _, th := range Builtin() {
		if got := th.For(colorprofile.TrueColor); got.BarFG != th.BarFG || got.PromptFG != th.PromptFG || got.TabActive != th.TabActive {
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
