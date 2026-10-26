package helppanel

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
)

var cmds = []command.Command{
	{Name: "alpha", Args: "[x]", Desc: "does alpha"},
	{Name: "beta", Desc: "does beta"},
}

var keys = []Key{{Keys: "Enter", Desc: "go"}, {Keys: "Esc", Desc: "stop"}}

func TestPanelListsCommandsThenKeys(t *testing.T) {
	box, x, y := Panel(theme.Default(), 80, 24, cmds, keys)
	plain := ansi.Strip(box)
	last := -1
	for _, want := range []string{"/alpha [x]", "does alpha", "/beta", "Enter", "stop", closeLine} {
		i := strings.Index(plain, want)
		if i < last {
			t.Errorf("%q comes before the row above it:\n%s", want, plain)
		}
		last = max(i, last)
		if i < 0 {
			t.Errorf("panel lacks %q:\n%s", want, plain)
		}
	}
	w, h := ui.Size(box)
	if x != (80-w)/2 || y != (24-h)/2 {
		t.Fatalf("panel at %d,%d, size %dx%d: not centered", x, y, w, h)
	}
}

// TestPanelKeepsTheCloseLineOnAShortScreen: rows that do not fit are cut
// from the end, and the line that says how to close stays.
func TestPanelKeepsTheCloseLineOnAShortScreen(t *testing.T) {
	box, _, _ := Panel(theme.Default(), 80, 7, cmds, keys)
	_, h := ui.Size(box)
	if h > 7 {
		t.Fatalf("panel is %d rows in a 7-row screen", h)
	}
	if !strings.Contains(ansi.Strip(box), closeLine) {
		t.Fatalf("the close line was cut:\n%s", ansi.Strip(box))
	}
}
