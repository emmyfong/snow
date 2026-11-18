package home

import (
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

func TestThemeHomeGolden80x24(t *testing.T)  { themeHomeGolden(t, 80, 24) }
func TestThemeHomeGolden120x40(t *testing.T) { themeHomeGolden(t, 120, 40) }

func themeHomeGolden(t *testing.T, cols, rows int) {
	t.Helper()
	saved := session("dotfiles", 26*time.Hour, 0)
	saved.State = api.StateSaved
	for _, th := range theme.Builtin() {
		t.Run(th.Name, func(t *testing.T) {
			m := screen(cols, rows, saved)
			m.SetTheme(th)
			golden.RequireEqual(t, m.View().Content)
		})
	}
}
