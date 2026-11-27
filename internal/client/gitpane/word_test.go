package gitpane

import (
	"charm.land/lipgloss/v2"
	"github.com/emmyfong/snow/internal/client/theme"
	"image/color"
	"strings"
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

func TestChangeWords(t *testing.T) {
	th := testTheme()
	for _, tt := range []struct {
		code, word string
		fg         color.Color
	}{
		{"A", "added", th.Added}, {"M", "modified", th.Modified},
		{"T", "modified", th.Modified}, {"D", "deleted", th.Removed},
		{"R", "renamed", th.Accent}, {"C", "copied", th.Accent},
		{"U", "conflict", th.ErrorFG}, {"?", "untracked", th.Muted},
		{"X", "changed", th.Muted}, {"", "changed", th.Muted},
	} {
		for _, staged := range []bool{false, true} {
			for _, conflict := range []bool{false, true} {
				e := entry{change: api.GitChange{Index: tt.code, Work: tt.code, Conflict: conflict}, staged: staged}
				if staged {
					e.change.Work = "D"
				} else {
					e.change.Index = "A"
				}
				word, fg, bg := changeWord(th, e)
				want, wantFG := tt.word, tt.fg
				var wantBG color.Color
				if conflict || tt.code == "U" {
					want, wantFG, wantBG = "conflict", th.ErrorFG, th.Error
				}
				if word != want || fg != wantFG || bg != wantBG {
					t.Errorf("code %q staged %v conflict %v: (%q, %v, %v), want (%q, %v, %v)", tt.code, staged, conflict, word, fg, bg, want, wantFG, wantBG)
				}
			}
		}
	}
}

func TestSelectedChangeWordContrast(t *testing.T) {
	for _, th := range theme.Builtin() {
		for _, code := range []string{"A", "M", "T", "D", "R", "C", "U", "?", "X"} {
			for _, conflict := range []bool{false, true} {
				m := model(api.GitState{Changes: []api.GitChange{{Path: "file.go", Work: code, Conflict: conflict}}}, 80, 24)
				m.SetTheme(th)
				e := m.entries()[0]
				word, normalFG, normalBG := changeWord(th, e)
				fg, bg := th.SelectionFG, th.Selection
				if code == "U" || conflict {
					fg, bg = th.ErrorFG, th.Error
				}
				want := lipgloss.NewStyle().Foreground(fg).Background(bg).Render(word)
				if cell := m.changeCell(e, 40); !strings.Contains(cell, want) {
					t.Errorf("%s %s selected word lacks readable colors: %q", th.Name, code, cell)
				}
				m.selKey = ""
				style := lipgloss.NewStyle().Foreground(normalFG)
				if normalBG != nil {
					style = style.Background(normalBG)
				}
				if cell := m.changeCell(e, 40); !strings.Contains(cell, style.Render(word)) {
					t.Errorf("%s %s changed unselected status colors", th.Name, code)
				}
			}
		}
	}
}
