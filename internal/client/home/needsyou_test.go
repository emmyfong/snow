package home

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// TestHomeNeedsYouGolden: one row marked "needs you" and one plain row.
// No built-in theme is light; the mark draws its own prompt colors, so it
// reads on a light or a dark terminal. Two themes cover two palettes.
func TestHomeNeedsYouGolden(t *testing.T) {
	waiting := session("api-server", 3*time.Minute, 1)
	waiting.NeedsYou = 2
	plain := session("dotfiles", time.Hour, 0)
	for _, name := range []string{"Frost", "Campbell"} {
		for _, size := range []struct{ cols, rows int }{{80, 24}, {120, 40}} {
			t.Run(fmt.Sprintf("%s_%dx%d", name, size.cols, size.rows), func(t *testing.T) {
				th, _ := theme.ByName(name)
				m := New("v0.1.0", th)
				m.now = func() time.Time { return now }
				m.SetSize(size.cols, size.rows)
				m.Update(&api.SessionList{Sessions: []api.SessionInfo{waiting, plain}})
				view := m.View().Content
				if !strings.Contains(ansi.Strip(view), "● needs you (2)") {
					t.Fatalf("no mark:\n%s", ansi.Strip(view))
				}
				golden.RequireEqual(t, view)
			})
		}
	}
}
