package client

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/client/home"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// TestCheckThemeSeesAChangedFile: another Snow window saved a theme, or the
// user edited the file; the check reports the new theme once.
func TestCheckThemeSeesAChangedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if got := checkTheme(path, time.Time{}); got.th != nil {
		t.Fatalf("no file: got theme %v, want none", got.th.Name)
	}
	if err := os.WriteFile(path, []byte("[theme]\nname = \"Nord\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := checkTheme(path, time.Time{})
	if got.th == nil || got.th.Name != "Nord" {
		t.Fatalf("got %+v, want Nord", got)
	}
	if again := checkTheme(path, got.mod); again.th != nil {
		t.Fatal("an unchanged file reported a theme again")
	}
	if err := os.WriteFile(path, []byte("[theme]\nname = \"neon\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	if bad := checkTheme(path, got.mod); bad.th != nil {
		t.Fatal("an unknown theme name was applied")
	}
}

// TestConfigThemeReachesTheScreen: the app applies a theme from the file to
// the screen on show and to screens it opens later.
func TestConfigThemeReachesTheScreen(t *testing.T) {
	a := newHomeApp(func(api.Message) {}, "v0.1.0", "", theme.Default())
	nord, _ := theme.ByName("Nord")
	a.Update(configCheckedMsg{th: &nord, mod: time.Now()})
	if a.theme.Name != "Nord" {
		t.Fatalf("app theme %q, want Nord", a.theme.Name)
	}
	if h, ok := a.screen.(*home.Model); !ok || h.Theme().Name != "Nord" {
		t.Fatal("the home page did not switch to Nord")
	}
}
